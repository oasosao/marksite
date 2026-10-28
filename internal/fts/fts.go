package fts

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/blugelabs/bluge"
	"github.com/blugelabs/bluge/analysis"

	"marksite/internal/model"
)

// ErrCanceled 任务被取消
var ErrCanceled = errors.New("任务已取消")

// Doc 索引文档（数据来源为 SQLite）
type Doc struct {
	Kind      string
	ID        int64
	DirID     int64
	Title     string
	Slug      string
	Summary   string
	Content   string
	Tags      []string
	Published int
	UpdatedAt string
}

// Index Bluge 索引封装，目录 ./ftsindex
type Index struct {
	dir      string
	mu       sync.Mutex
	analyzer *analysis.Analyzer
}

// New 创建索引对象（不立即打开，按需操作）
func New(dir string) *Index {
	return &Index{dir: dir, analyzer: cjkAnalyzer}
}

func (i *Index) Path() string { return i.dir }

func (i *Index) Exists() bool {
	st, err := os.Stat(i.dir)
	return err == nil && st.IsDir()
}

// Size 索引目录占用字节
func (i *Index) Size() int64 {
	var total int64
	_ = filepath.WalkDir(i.dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// Count 文档数量
func (i *Index) Count() uint64 {
	if !i.Exists() {
		return 0
	}
	reader, err := bluge.OpenReader(bluge.DefaultConfig(i.dir))
	if err != nil {
		return 0
	}
	defer func() { _ = reader.Close() }()
	n, err := reader.Count()
	if err != nil {
		return 0
	}
	return n
}

func docID(kind string, id int64) string {
	return kind + "-" + itoa(id)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

func (i *Index) buildDocument(d Doc) *bluge.Document {
	doc := bluge.NewDocument(docID(d.Kind, d.ID))
	doc.AddField(bluge.NewKeywordField("kind", d.Kind).StoreValue())
	doc.AddField(bluge.NewKeywordField("id", itoa(d.ID)).StoreValue())
	doc.AddField(bluge.NewKeywordField("dir_id", itoa(d.DirID)).StoreValue())
	doc.AddField(bluge.NewTextField("title", d.Title).WithAnalyzer(i.analyzer).StoreValue())
	doc.AddField(bluge.NewKeywordField("slug", d.Slug).StoreValue())
	doc.AddField(bluge.NewTextField("summary", d.Summary).WithAnalyzer(i.analyzer).StoreValue())
	doc.AddField(bluge.NewTextField("content", d.Content).WithAnalyzer(i.analyzer))
	doc.AddField(bluge.NewStoredOnlyField("plain", []byte(plainText(d.Content))))
	doc.AddField(bluge.NewTextField("tags", strings.Join(d.Tags, " ")).WithAnalyzer(i.analyzer).StoreValue())
	pub := "0"
	if d.Published == 1 {
		pub = "1"
	}
	doc.AddField(bluge.NewKeywordField("published", pub).StoreValue())
	doc.AddField(bluge.NewKeywordField("updated_at", d.UpdatedAt).StoreValue())
	return doc
}

// Upsert 写入/更新单篇文档
func (i *Index) Upsert(d Doc) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := os.MkdirAll(i.dir, 0o755); err != nil {
		return err
	}
	writer, err := bluge.OpenWriter(bluge.DefaultConfig(i.dir))
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()
	doc := i.buildDocument(d)
	return writer.Update(doc.ID(), doc)
}

// Delete 删除文档
func (i *Index) Delete(kind string, id int64) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.Exists() {
		return nil
	}
	writer, err := bluge.OpenWriter(bluge.DefaultConfig(i.dir))
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()
	termDoc := bluge.NewDocument(docID(kind, id))
	return writer.Delete(termDoc.ID())
}

// Search 多字段检索
func (i *Index) Search(q string, limit int) ([]model.SearchHit, uint64, error) {
	out := []model.SearchHit{}
	if strings.TrimSpace(q) == "" || !i.Exists() {
		return out, 0, nil
	}
	if limit <= 0 {
		limit = 20
	}
	reader, err := bluge.OpenReader(bluge.DefaultConfig(i.dir))
	if err != nil {
		return out, 0, nil
	}
	defer func() { _ = reader.Close() }()

	fieldQuery := func(field string, boost float64) *bluge.MatchQuery {
		return bluge.NewMatchQuery(q).
			SetField(field).
			SetAnalyzer(i.analyzer).
			SetOperator(bluge.MatchQueryOperatorAnd).
			SetBoost(boost)
	}
	query := bluge.NewBooleanQuery().
		AddShould(
			fieldQuery("title", 3),
			fieldQuery("tags", 2),
			fieldQuery("summary", 1.5),
			fieldQuery("content", 1),
		).
		SetMinShould(1)

	iterator, err := reader.Search(context.Background(),
		bluge.NewTopNSearch(limit, query).WithStandardAggregations())
	if err != nil {
		return out, 0, nil
	}
	for {
		match, err := iterator.Next()
		if err != nil {
			break
		}
		if match == nil {
			break
		}
		hit := model.SearchHit{}
		var plain string
		_ = match.VisitStoredFields(func(field string, value []byte) bool {
			switch field {
			case "kind":
				hit.Type = string(value)
			case "id":
				hit.ID = atoi(string(value))
			case "title":
				hit.Title = string(value)
			case "slug":
				hit.Slug = string(value)
			case "published":
				if string(value) == "1" {
					hit.Published = 1
				}
			case "updated_at":
				hit.UpdatedAt = string(value)
			case "plain":
				plain = string(value)
			}
			return true
		})
		hit.Snippet = snippet(plain, q)
		out = append(out, hit)
	}
	return out, iterator.Aggregations().Count(), nil
}

// ReplaceAll 全量重建（临时目录构建后原子替换）
func (i *Index) ReplaceAll(docs []Doc, progress func(done, total int), canceled func() bool) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	tmp := i.dir + ".building"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	writer, err := bluge.OpenWriter(bluge.DefaultConfig(tmp))
	if err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	for idx, d := range docs {
		if canceled != nil && canceled() {
			_ = writer.Close()
			_ = os.RemoveAll(tmp)
			return ErrCanceled
		}
		doc := i.buildDocument(d)
		if err := writer.Update(doc.ID(), doc); err != nil {
			_ = writer.Close()
			_ = os.RemoveAll(tmp)
			return err
		}
		if progress != nil {
			progress(idx+1, len(docs))
		}
	}
	if err := writer.Close(); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(i.dir); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, i.dir); err != nil {
		return err
	}
	return nil
}

func atoi(s string) int64 {
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int64(r-'0')
	}
	return n
}

// plainText 粗略去除 Markdown 标记，供摘要展示
func plainText(s string) string {
	replacer := strings.NewReplacer("#", " ", "*", "", "`", "", ">", "", "[", "", "]", "", "(", "", ")", "", "!", "")
	t := replacer.Replace(s)
	t = strings.Join(strings.Fields(t), " ")
	if len(t) > 8000 {
		t = t[:8000]
	}
	return t
}

func snippet(plain, query string) string {
	runes := []rune(plain)
	if len(runes) == 0 {
		return ""
	}
	lower := strings.ToLower(plain)
	bytePos := findSnippetAnchor(lower, query)
	// strings.Index 返回字节偏移，需要转换为 rune 下标才能在 []rune 上切片
	runePos := utf8.RuneCountInString(plain[:bytePos])
	start := runePos - 40
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	end := start + 120
	if end > len(runes) {
		end = len(runes)
	}
	prefix := ""
	if start > 0 {
		prefix = "..."
	}
	suffix := ""
	if end < len(runes) {
		suffix = "..."
	}
	return prefix + string(runes[start:end]) + suffix
}

// findSnippetAnchor 在 lowerPlain 中找到最匹配 query 的字节偏移
// 先尝试精确子串匹配，失败则用 analyzer 分词后找命中 token 最密集的位置
func findSnippetAnchor(lowerPlain, query string) int {
	if pos := strings.Index(lowerPlain, strings.ToLower(query)); pos >= 0 {
		return pos
	}
	tokens := cjkAnalyzer.Analyze([]byte(query))
	if len(tokens) == 0 {
		return 0
	}
	// 收集所有 token 在正文中的出现位置
	type hit struct{ bytePos int }
	var hits []hit
	for _, t := range tokens {
		term := strings.ToLower(string(t.Term))
		if term == "" {
			continue
		}
		from := 0
		for {
			p := strings.Index(lowerPlain[from:], term)
			if p < 0 {
				break
			}
			hits = append(hits, hit{bytePos: from + p})
			from += p + len(term)
			if from >= len(lowerPlain) {
				break
			}
		}
	}
	if len(hits) == 0 {
		return 0
	}
	// 按 bytePos 排序后，用滑动窗口（200 字节）找命中最密集的位置
	sort.Slice(hits, func(i, j int) bool { return hits[i].bytePos < hits[j].bytePos })
	bestPos, bestCount := hits[0].bytePos, 0
	for i, h := range hits {
		count := 1
		for j := i + 1; j < len(hits); j++ {
			if hits[j].bytePos-h.bytePos <= 200 {
				count++
			} else {
				break
			}
		}
		if count > bestCount {
			bestCount = count
			bestPos = h.bytePos
		}
	}
	return bestPos
}
