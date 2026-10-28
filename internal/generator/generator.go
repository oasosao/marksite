package generator

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"marksite/internal/model"
	"marksite/internal/theme"
)

// ErrCanceled 任务被取消
var ErrCanceled = errors.New("任务已取消")

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// SiteView 模板站点信息
type SiteView struct {
	Name         string
	Desc         string
	URL          string
	Logo         string
	Favicon      string
	Copyright    string
	CustomHead   template.HTML
	CustomFooter template.HTML
	HomeURL      string
	ArchiveURL   string
	RSSURL       string
	AssetBase    string
	GeneratedAt  string
	PageSize     int
	NavMenu      []model.NavItem
}

// PostCard 列表项
type PostCard struct {
	ID          int64
	Title       string
	Slug        string
	Summary     string
	Cover       string
	Tags        []model.Tag
	URL         string
	PublishDate string
	DirID       int64
}

// Pagination 分页
type Pagination struct {
	Page       int
	TotalPages int
	Total      int
	PrevURL    string
	NextURL    string
}

// IndexData 首页/分页
type IndexData struct {
	Site       SiteView
	Posts      []PostCard
	Pagination Pagination
	// Heading 列表页标题：分类名/标签名，首页留空
	Heading string
	// Groups 目录聚合页专用：按子目录分组的文章（只有当 Posts 跨多个子目录时才填）
	Groups []DirGroup
}

// DirGroup 目录聚合页的子分类分组
type DirGroup struct {
	DirID   int64
	DirName string
	DirSlug string
	URL     string // 子分类独立页链接
	Posts   []PostCard
}

// PostView 文章详情
type PostView struct {
	ID          int64
	Title       string
	Slug        string
	Summary     string
	Cover       string
	Gallery     []string
	Tags        []model.Tag
	URL         string
	PublishDate string
	ContentHTML template.HTML
}

// PageView 页面详情
type PageView struct {
	ID          int64
	Title       string
	Slug        string
	Summary     string
	Cover       string
	Gallery     []string
	URL         string
	ContentHTML template.HTML
}

// PostData 文章模板数据
type PostData struct {
	Site SiteView
	Post PostView
}

// PageData 页面模板数据
type PageData struct {
	Site SiteView
	Page PageView
}

// YearGroup 归档年份分组
type YearGroup struct {
	Year  int
	Posts []PostCard
}

// ArchiveData 归档模板数据
type ArchiveData struct {
	Site  SiteView
	Years []YearGroup
}

// Options 渲染参数
type Options struct {
	Site         model.SiteConfig
	Theme        *theme.Theme
	Posts        []model.Post
	Pages        []model.Page
	Dirs         []model.Directory
	Tags         []model.Tag
	AssetsDir    string
	OutDir       string
	Full         bool
	ChangedPosts map[int64]bool
	ChangedPages map[int64]bool
	// NoChange 为 true 时只重建聚合产物（增量且配置未变）
	NoItemChange bool
	Progress     func(done, total int, msg string)
	Canceled     func() bool
}

// Render 执行静态站点渲染
func Render(in Options) error {
	if in.Theme == nil {
		return fmt.Errorf("未加载主题")
	}
	outAbs, err := filepath.Abs(in.OutDir)
	if err != nil {
		return err
	}
	if in.Full {
		if err := os.RemoveAll(outAbs); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(outAbs, 0o755); err != nil {
		return err
	}

	tmpl, err := template.ParseFS(in.Theme.FS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("主题模板解析失败: %w", err)
	}

	ub := newURLBuilder(in.Dirs)

	site := SiteView{
		Name:         in.Site.SiteName,
		Desc:         in.Site.SiteDesc,
		URL:          strings.TrimRight(in.Site.SiteURL, "/"),
		Logo:         in.Site.Logo,
		Favicon:      in.Site.Favicon,
		Copyright:    in.Site.Copyright,
		CustomHead:   template.HTML(in.Site.CustomHead),
		CustomFooter: template.HTML(in.Site.CustomFooter),
		HomeURL:      "/",
		ArchiveURL:   "/archive.html",
		AssetBase:    "/assets",
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		PageSize:     in.Site.PageSize,
	}
	if in.Site.RSSEnabled == 1 {
		site.RSSURL = "/feed.xml"
	}

	// 构建导航菜单（仅已发布且 menu_visible=1 的分类）
	site.NavMenu = buildNavMenu(in.Dirs, ub)

	posts := make([]model.Post, 0, len(in.Posts))
	for _, p := range in.Posts {
		if p.IsPublished == 1 {
			posts = append(posts, p)
		}
	}
	sort.SliceStable(posts, func(i, j int) bool {
		return posts[i].PublishedAt > posts[j].PublishedAt
	})
	pages := make([]model.Page, 0, len(in.Pages))
	for _, p := range in.Pages {
		if p.IsPublished == 1 {
			pages = append(pages, p)
		}
	}

	expected := map[string]bool{}
	addExpected := func(url string) { expected[strings.TrimPrefix(url, "/")] = true }
	for _, p := range posts {
		addExpected(ub.postURL(p))
	}
	for _, p := range pages {
		addExpected(ub.pageURL(p))
	}
	addExpected("/index.html")
	addExpected("/archive.html")
	if in.Site.RSSEnabled == 1 {
		addExpected("/feed.xml")
	}
	if in.Site.SitemapEnabled == 1 {
		addExpected("/sitemap.xml")
	}
	addExpected("/robots.txt")
	pageSize := in.Site.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	totalPages := (len(posts) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	for n := 2; n <= totalPages; n++ {
		addExpected(fmt.Sprintf("/page/%d/index.html", n))
	}

	// 统计进度总量
	itemTotal := 0
	if !in.NoItemChange {
		for _, p := range posts {
			if in.Full || in.ChangedPosts == nil || in.ChangedPosts[p.ID] {
				itemTotal++
			}
		}
		for _, p := range pages {
			if in.Full || in.ChangedPages == nil || in.ChangedPages[p.ID] {
				itemTotal++
			}
		}
	}
	aggTotal := 1 + totalPages + 1 + 1 + 1 // 归档 + 首页分页 + rss + sitemap + robots
	total := itemTotal + aggTotal + 2      // + static/assets
	done := 0
	tick := func(msg string) error {
		if in.Canceled != nil && in.Canceled() {
			return ErrCanceled
		}
		done++
		if in.Progress != nil {
			doneCopy := done
			totalCopy := total
			if doneCopy > totalCopy {
				doneCopy = totalCopy
			}
			in.Progress(doneCopy, totalCopy, msg)
		}
		return nil
	}

	cards := make([]PostCard, 0, len(posts))
	for _, p := range posts {
		cards = append(cards, PostCard{
			ID: p.ID, Title: p.Title, Slug: p.Slug, Summary: p.Summary, Cover: fixCoverURL(p.Cover),
			Tags: p.Tags, URL: ub.postURL(p),
			PublishDate: pubDate(p.PublishedAt, p.CreatedAt), DirID: p.DirID,
		})
	}

	// 预处理倒排索引，避免后续 dir/tag 渲染时 O(N²) 嵌套遍历
	dirCardMap := map[int64][]PostCard{}
	tagCardMap := map[int64][]PostCard{}
	for _, c := range cards {
		dirCardMap[c.DirID] = append(dirCardMap[c.DirID], c)
		for _, tc := range c.Tags {
			tagCardMap[tc.ID] = append(tagCardMap[tc.ID], c)
		}
	}

	// 把子孙目录的 cards 聚合到父目录（一级菜单能看到子目录的文章）
	dirChildren := map[int64][]int64{}
	for _, d := range in.Dirs {
		dirChildren[d.ParentID] = append(dirChildren[d.ParentID], d.ID)
	}
	var collectCards func(rootID int64) []PostCard
	collectCards = func(rootID int64) []PostCard {
		merged := append([]PostCard{}, dirCardMap[rootID]...)
		for _, cid := range dirChildren[rootID] {
			merged = append(merged, collectCards(cid)...)
		}
		return merged
	}
	mergedDirCardMap := map[int64][]PostCard{}
	for _, d := range in.Dirs {
		if d.IsPage == 1 {
			continue // 页面型目录不聚合文章
		}
		mergedDirCardMap[d.ID] = collectCards(d.ID)
	}

	if !in.NoItemChange {
		for _, p := range posts {
			if !in.Full && in.ChangedPosts != nil && !in.ChangedPosts[p.ID] {
				continue
			}
			htmlBody := renderMarkdown(p.Content)
			data := PostData{Site: site, Post: PostView{
				ID: p.ID, Title: p.Title, Slug: p.Slug, Summary: p.Summary, Cover: fixCoverURL(p.Cover),
				Gallery: fixGallery(p.Gallery),
				Tags:    p.Tags, URL: ub.postURL(p),
				PublishDate: pubDate(p.PublishedAt, p.CreatedAt), ContentHTML: template.HTML(htmlBody),
			}}
			if err := writeTemplate(tmpl, "post.html", data, outAbs, ub.postURL(p)); err != nil {
				return err
			}
			if err := tick("渲染文章 " + p.Slug); err != nil {
				return err
			}
		}
		for _, p := range pages {
			if !in.Full && in.ChangedPages != nil && !in.ChangedPages[p.ID] {
				continue
			}
			htmlBody := renderMarkdown(p.Content)
			data := PageData{Site: site, Page: PageView{
				ID: p.ID, Title: p.Title, Slug: p.Slug, Summary: p.Summary, Cover: fixCoverURL(p.Cover),
				Gallery: fixGallery(p.Gallery),
				URL:     ub.pageURL(p), ContentHTML: template.HTML(htmlBody),
			}}
			if err := writeTemplate(tmpl, "page.html", data, outAbs, ub.pageURL(p)); err != nil {
				return err
			}
			if err := tick("渲染页面 " + p.Slug); err != nil {
				return err
			}
		}
	}

	// --- 分类聚合页 & 独立页面 ---
	for _, d := range in.Dirs {
		if d.IsPublished != 1 {
			continue
		}
		if d.IsPage == 1 {
			// 独立页面，走 /{slug}.html
			url := ub.dirStandaloneURL(d.ID)
			htmlBody := renderMarkdown(d.PageContent)
			title := d.PageTitle
			if title == "" {
				title = d.Name
			}
			data := PageData{Site: site, Page: PageView{
				ID: d.ID, Title: title, Slug: d.Slug, Summary: d.PageSummary, Cover: fixCoverURL(d.Cover),
				URL: url, ContentHTML: template.HTML(htmlBody),
			}}
			if err := writeTemplate(tmpl, "page.html", data, outAbs, url); err != nil {
				return err
			}
			addExpected(url)
			if err := tick("渲染独立页面 " + d.Slug); err != nil {
				return err
			}
			continue
		}
		// 分类容器：生成文章聚合页（包含子目录的文章）
		url := ub.dirArticleURL(d.ID)
		if url != "" {
			posts := mergedDirCardMap[d.ID]
			data := IndexData{Site: site, Posts: posts, Heading: d.Name}
			// 如果聚合了子目录的文章，按子目录分组展示
			if len(posts) > 0 && len(dirChildren[d.ID]) > 0 {
				data.Groups = buildDirGroups(d, posts, dirChildren[d.ID], in.Dirs, ub)
			}
			if err := writeTemplate(tmpl, "category.html", data, outAbs, url); err != nil {
				if strings.Contains(err.Error(), "undefined") {
					// 主题没有 category.html，跳过
					continue
				}
				return err
			}
			addExpected(url)
			if err := tick("渲染分类聚合页 " + d.Slug); err != nil {
				return err
			}
		}
	}

	// --- 标签聚合页 ---
	for _, t := range in.Tags {
		url := ub.tagURL(t)
		data := IndexData{Site: site, Posts: tagCardMap[t.ID], Heading: t.Name}
		if err := writeTemplate(tmpl, "tag.html", data, outAbs, url); err != nil {
			if strings.Contains(err.Error(), "undefined") {
				continue
			}
			return err
		}
		addExpected(url)
		if err := tick("渲染标签页 " + t.Slug); err != nil {
			return err
		}
	}

	// 首页与分页
	for n := 1; n <= totalPages; n++ {
		start := (n - 1) * pageSize
		end := start + pageSize
		if start > len(cards) {
			start = len(cards)
		}
		if end > len(cards) {
			end = len(cards)
		}
		data := IndexData{
			Site:  site,
			Posts: cards[start:end],
			Pagination: Pagination{
				Page: n, TotalPages: totalPages, Total: len(cards),
				PrevURL: prevPageURL(n),
				NextURL: nextPageURL(n, totalPages),
			},
		}
		url := "/index.html"
		if n > 1 {
			url = fmt.Sprintf("/page/%d/index.html", n)
		}
		if err := writeTemplate(tmpl, "index.html", data, outAbs, url); err != nil {
			return err
		}
		if err := tick(fmt.Sprintf("渲染列表第 %d/%d 页", n, totalPages)); err != nil {
			return err
		}
	}

	// 归档
	years := groupYears(cards)
	archiveData := ArchiveData{Site: site, Years: years}
	if err := writeTemplate(tmpl, "archive.html", archiveData, outAbs, "/archive.html"); err != nil {
		return err
	}
	if err := tick("渲染归档"); err != nil {
		return err
	}

	if in.Site.RSSEnabled == 1 {
		if err := writeFeed(site, cards, outAbs); err != nil {
			return err
		}
	}
	if err := tick("生成 RSS"); err != nil {
		return err
	}
	if in.Site.SitemapEnabled == 1 {
		if err := writeSitemap(site, cards, pages, in.Dirs, in.Tags, ub, outAbs); err != nil {
			return err
		}
	}
	if err := tick("生成 Sitemap"); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outAbs, "robots.txt"), []byte(in.Site.Robots), 0o644); err != nil {
		return err
	}
	if err := tick("生成 robots.txt"); err != nil {
		return err
	}

	// 复制主题静态资源与上传资源
	if err := copyThemeStatic(in.Theme.FS, outAbs); err != nil {
		return err
	}
	if err := tick("复制主题资源"); err != nil {
		return err
	}
	if err := copyAssets(in.AssetsDir, filepath.Join(outAbs, "assets")); err != nil {
		return err
	}
	if err := tick("同步资源文件"); err != nil {
		return err
	}

	// 清理孤儿产物（跳过 static 与 assets）
	if err := cleanOrphans(outAbs, expected); err != nil {
		return err
	}
	return nil
}

func writeTemplate(tmpl *template.Template, name string, data any, outAbs, url string) error {
	rel := strings.TrimPrefix(url, "/")
	target := filepath.Join(outAbs, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("模板 %s 渲染失败: %w", name, err)
	}
	return os.WriteFile(target, buf.Bytes(), 0o644)
}

func renderMarkdown(content string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		return template.HTMLEscapeString(content)
	}
	// 管理端图片使用 /media/ 前缀，生成站点统一映射到 /assets/
	return fixAssetURL(buf.String())
}

// fixAssetURL 把管理端的 /media/ 前缀统一映射到生成站点的 /assets/
func fixAssetURL(s string) string {
	if s == "" {
		return ""
	}
	return strings.ReplaceAll(s, "/media/", "/assets/")
}

// fixCoverURL 归一化封面地址：兼容 /media/ 前缀、asset.path 相对路径、外部 URL
func fixCoverURL(s string) string {
	if s == "" {
		return ""
	}
	// 外部 URL 或协议相对路径保持不变
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "//") {
		return s
	}
	// /media/ 前缀 → /assets/
	if strings.HasPrefix(s, "/media/") {
		return "/assets/" + strings.TrimPrefix(s, "/media/")
	}
	// 已是 /assets/ 前缀，保持
	if strings.HasPrefix(s, "/assets/") {
		return s
	}
	// 其他绝对路径保持不变
	if strings.HasPrefix(s, "/") {
		return s
	}
	// 相对路径（asset.path，如 2026/10/xxx.jpg）→ /assets/xxx
	return "/assets/" + s
}

// fixGallery 归一化图集数组中每个图片地址
func fixGallery(g []string) []string {
	if len(g) == 0 {
		return nil
	}
	out := make([]string, 0, len(g))
	for _, s := range g {
		if s == "" {
			continue
		}
		out = append(out, fixCoverURL(s))
	}
	return out
}

func pubDate(publishedAt, createdAt string) string {
	v := publishedAt
	if v == "" {
		v = createdAt
	}
	if len(v) >= 10 {
		return v[:10]
	}
	return v
}

func prevPageURL(n int) string {
	if n <= 1 {
		return ""
	}
	if n == 2 {
		return "/"
	}
	return fmt.Sprintf("/page/%d/index.html", n-1)
}

func nextPageURL(n, total int) string {
	if n >= total {
		return ""
	}
	return fmt.Sprintf("/page/%d/index.html", n+1)
}

// buildDirGroups 把聚合目录的文章按子目录分组（子目录在 dirChildren 里）
// 只有有文章的子目录才出现在 Groups 里，按 Sort 排序
func buildDirGroups(parent model.Directory, posts []PostCard, childIDs []int64, allDirs []model.Directory, ub *urlBuilder) []DirGroup {
	byID := map[int64]model.Directory{}
	for _, d := range allDirs {
		byID[d.ID] = d
	}
	childSort := map[int64]int{}
	for _, cid := range childIDs {
		if d, ok := byID[cid]; ok {
			childSort[cid] = d.Sort
		}
	}
	byChild := map[int64][]PostCard{}
	for _, c := range posts {
		byChild[c.DirID] = append(byChild[c.DirID], c)
	}
	// 收集有文章的子目录，按 Sort 排序
	type sortItem struct {
		id   int64
		sort int
	}
	var items []sortItem
	for cid := range byChild {
		if cid == parent.ID {
			continue // 父目录自身的文章不进分组（理论上聚合页父目录自身不会有文章）
		}
		items = append(items, sortItem{cid, childSort[cid]})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].sort != items[j].sort {
			return items[i].sort < items[j].sort
		}
		return items[i].id < items[j].id
	})
	groups := make([]DirGroup, 0, len(items))
	for _, it := range items {
		d := byID[it.id]
		groups = append(groups, DirGroup{
			DirID:   d.ID,
			DirName: d.Name,
			DirSlug: d.Slug,
			URL:     ub.dirArticleURL(d.ID),
			Posts:   byChild[d.ID],
		})
	}
	return groups
}

func groupYears(cards []PostCard) []YearGroup {
	byYear := map[int][]PostCard{}
	years := []int{}
	for _, c := range cards {
		y := 0
		if len(c.PublishDate) >= 4 {
			y, _ = strconv.Atoi(c.PublishDate[:4])
		}
		if _, ok := byYear[y]; !ok {
			years = append(years, y)
		}
		byYear[y] = append(byYear[y], c)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	out := make([]YearGroup, 0, len(years))
	for _, y := range years {
		out = append(out, YearGroup{Year: y, Posts: byYear[y]})
	}
	return out
}

// urlBuilder 固定路径模式（Q4A 废弃 path_mode）
type urlBuilder struct {
	dirs map[int64]model.Directory
}

func newURLBuilder(dirs []model.Directory) *urlBuilder {
	m := map[int64]model.Directory{}
	for _, d := range dirs {
		m[d.ID] = d
	}
	return &urlBuilder{dirs: m}
}

func (u *urlBuilder) dirSlug(dirID int64) string {
	d, ok := u.dirs[dirID]
	if !ok {
		return ""
	}
	return d.Slug
}

// dirArticleURL 分类文章聚合页
func (u *urlBuilder) dirArticleURL(dirID int64) string {
	slug := u.dirSlug(dirID)
	if slug == "" {
		return ""
	}
	return "/" + slug + "/index.html"
}

// dirStandaloneURL 分类的独立页面（is_page=1）
func (u *urlBuilder) dirStandaloneURL(dirID int64) string {
	slug := u.dirSlug(dirID)
	if slug == "" {
		return ""
	}
	return "/" + slug + ".html"
}

// postURL 文章详情
func (u *urlBuilder) postURL(p model.Post) string {
	if p.DirID != 0 {
		if slug := u.dirSlug(p.DirID); slug != "" {
			return "/" + slug + "/" + p.Slug + ".html"
		}
	}
	return "/" + p.Slug + ".html"
}

// pageURL 页面详情
func (u *urlBuilder) pageURL(p model.Page) string {
	if p.DirID != 0 {
		if slug := u.dirSlug(p.DirID); slug != "" {
			return "/" + slug + "/" + p.Slug + ".html"
		}
	}
	return "/" + p.Slug + ".html"
}

// tagURL 标签聚合页
func (u *urlBuilder) tagURL(tag model.Tag) string {
	return "/tags/" + tag.Slug + ".html"
}

// buildNavMenu 递归构建导航菜单（仅 is_published=1 AND menu_visible=1）
func buildNavMenu(dirs []model.Directory, ub *urlBuilder) []model.NavItem {
	byID := map[int64]model.Directory{}
	for _, d := range dirs {
		byID[d.ID] = d
	}
	children := map[int64][]model.Directory{}
	for _, d := range dirs {
		if d.IsPublished == 1 && d.MenuVisible == 1 {
			children[d.ParentID] = append(children[d.ParentID], d)
		}
	}
	for pid := range children {
		sort.SliceStable(children[pid], func(i, j int) bool {
			return children[pid][i].Sort < children[pid][j].Sort
		})
	}
	var build func(parentID int64) []model.NavItem
	build = func(parentID int64) []model.NavItem {
		var out []model.NavItem
		for _, d := range children[parentID] {
			url := ub.dirArticleURL(d.ID)
			isPage := d.IsPage == 1
			if isPage {
				url = ub.dirStandaloneURL(d.ID)
			}
			label := d.Name
			if d.PageTitle != "" {
				label = d.PageTitle
			}
			item := model.NavItem{
				Label: label, Slug: d.Slug, URL: url,
				IsPage:   isPage,
				Children: build(d.ID),
			}
			out = append(out, item)
		}
		return out
	}
	return build(0)
}

func copyThemeStatic(fsys fs.FS, outAbs string) error {
	sub, err := fs.Sub(fsys, "static")
	if err != nil {
		return nil // 主题没有 static 目录
	}
	return fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(outAbs, "static", filepath.FromSlash(path)), 0o755)
		}
		data, err := fs.ReadFile(sub, path)
		if err != nil {
			return err
		}
		target := filepath.Join(outAbs, "static", filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func copyAssets(srcDir, dstDir string) error {
	if _, err := os.Stat(srcDir); err != nil {
		return nil
	}
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return nil
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if dstInfo, err := os.Stat(target); err == nil && dstInfo.Size() == info.Size() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		_ = out.Close()
		return copyErr
	})
}

func cleanOrphans(outAbs string, expected map[string]bool) error {
	var emptyDirs []string
	err := filepath.WalkDir(outAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(outAbs, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if rel == "static" || rel == "assets" {
				return filepath.SkipDir
			}
			emptyDirs = append(emptyDirs, path)
			return nil
		}
		if strings.HasSuffix(rel, ".html") || strings.HasSuffix(rel, ".xml") || strings.HasSuffix(rel, ".txt") {
			if !expected[rel] {
				_ = os.Remove(path)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 自底向上删除空目录
	sort.Sort(sort.Reverse(sort.StringSlice(emptyDirs)))
	for _, dir := range emptyDirs {
		_ = os.Remove(dir) // 非空时失败，忽略
	}
	return nil
}

func absolute(siteURL, path string) string {
	return strings.TrimRight(siteURL, "/") + path
}

func writeFeed(site SiteView, cards []PostCard, outAbs string) error {
	type item struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		GUID        string `xml:"guid"`
		PubDate     string `xml:"pubDate"`
		Description string `xml:"description"`
	}
	type channel struct {
		Title         string `xml:"title"`
		Link          string `xml:"link"`
		Description   string `xml:"description"`
		LastBuildDate string `xml:"lastBuildDate"`
		Items         []item `xml:"item"`
	}
	type rss struct {
		XMLName xml.Name `xml:"rss"`
		Version string   `xml:"version,attr"`
		Channel channel  `xml:"channel"`
	}
	r := rss{Version: "2.0"}
	r.Channel.Title = site.Name
	r.Channel.Link = site.URL + "/"
	r.Channel.Description = site.Desc
	r.Channel.LastBuildDate = time.Now().UTC().Format(time.RFC1123Z)
	limit := 20
	if len(cards) < limit {
		limit = len(cards)
	}
	for _, c := range cards[:limit] {
		link := absolute(site.URL, c.URL)
		desc := c.Summary
		if desc == "" {
			desc = c.Title
		}
		r.Channel.Items = append(r.Channel.Items, item{
			Title: c.Title, Link: link, GUID: link,
			PubDate:     parsePubRFC1123(c.PublishDate),
			Description: desc,
		})
	}
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(r); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outAbs, "feed.xml"), buf.Bytes(), 0o644)
}

func parsePubRFC1123(date string) string {
	if t, err := time.Parse("2006-01-02", date); err == nil {
		return t.UTC().Format(time.RFC1123Z)
	}
	return time.Now().UTC().Format(time.RFC1123Z)
}

func writeSitemap(site SiteView, posts []PostCard, pages []model.Page, dirs []model.Directory, tags []model.Tag, ub *urlBuilder, outAbs string) error {
	type urlEntry struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod,omitempty"`
	}
	type urlset struct {
		XMLName xml.Name   `xml:"urlset"`
		Xmlns   string     `xml:"xmlns,attr"`
		URLs    []urlEntry `xml:"url"`
	}
	set := urlset{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	set.URLs = append(set.URLs, urlEntry{Loc: absolute(site.URL, "/")})
	set.URLs = append(set.URLs, urlEntry{Loc: absolute(site.URL, "/archive.html")})
	for _, p := range posts {
		set.URLs = append(set.URLs, urlEntry{Loc: absolute(site.URL, p.URL)})
	}
	for _, p := range pages {
		lastMod := ""
		if len(p.UpdatedAt) >= 10 {
			lastMod = p.UpdatedAt[:10]
		}
		set.URLs = append(set.URLs, urlEntry{Loc: absolute(site.URL, ub.pageURL(p)), LastMod: lastMod})
	}
	for _, d := range dirs {
		if d.IsPublished != 1 || d.Slug == "" {
			continue
		}
		url := ""
		if d.IsPage == 1 {
			url = ub.dirStandaloneURL(d.ID)
		} else {
			url = ub.dirArticleURL(d.ID)
		}
		if url != "" {
			lastMod := ""
			if len(d.UpdatedAt) >= 10 {
				lastMod = d.UpdatedAt[:10]
			}
			set.URLs = append(set.URLs, urlEntry{Loc: absolute(site.URL, url), LastMod: lastMod})
		}
	}
	for _, t := range tags {
		url := ub.tagURL(t)
		lastMod := ""
		if len(t.CreatedAt) >= 10 {
			lastMod = t.CreatedAt[:10]
		}
		set.URLs = append(set.URLs, urlEntry{Loc: absolute(site.URL, url), LastMod: lastMod})
	}
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(set); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outAbs, "sitemap.xml"), buf.Bytes(), 0o644)
}
