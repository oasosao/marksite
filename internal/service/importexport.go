package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"marksite/internal/model"
	"marksite/internal/pkg"
)

// ImportError 导入失败项
type ImportError struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// ImportResult 导入结果
type ImportResult struct {
	Imported int           `json:"imported"`
	Skipped  int           `json:"skipped"`
	Errors   []ImportError `json:"errors"`
}

// ManifestItem 批量导出清单项
type ManifestItem struct {
	File        string   `json:"file"`
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Summary     string   `json:"summary"`
	Cover       string   `json:"cover"`
	Gallery     []string `json:"gallery,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Sort        int      `json:"sort"`
	IsPublished int      `json:"is_published"`
	PublishedAt string   `json:"published_at"`
}

type importItem struct {
	file string
	in   ManifestItem
	body string
}

// ExportPost 单篇导出纯 Markdown
func (a *App) ExportPost(ctx context.Context, id int64) (string, []byte, error) {
	p, err := a.Repo.GetPost(ctx, id)
	if err != nil {
		return "", nil, pkg.NotFound("文章不存在")
	}
	return p.Slug + ".md", []byte(p.Content), nil
}

// ExportPage 单页导出纯 Markdown
func (a *App) ExportPage(ctx context.Context, id int64) (string, []byte, error) {
	p, err := a.Repo.GetPage(ctx, id)
	if err != nil {
		return "", nil, pkg.NotFound("页面不存在")
	}
	return p.Slug + ".md", []byte(p.Content), nil
}

// BatchExportPosts 批量导出文章 zip（md + manifest.json）
func (a *App) BatchExportPosts(ctx context.Context, ids []int64) (string, []byte, error) {
	items := make([]importItem, 0, len(ids))
	for _, id := range ids {
		p, err := a.Repo.GetPost(ctx, id)
		if err != nil {
			continue
		}
		items = append(items, importItem{
			file: p.Slug + ".md",
			body: p.Content,
			in: ManifestItem{
				File: p.Slug + ".md", Title: p.Title, Slug: p.Slug, Summary: p.Summary,
				Cover: p.Cover, Gallery: p.Gallery, Tags: tagNames(p.Tags), Sort: p.Sort,
				IsPublished: p.IsPublished, PublishedAt: p.PublishedAt,
			},
		})
	}
	return buildZip("posts", items)
}

// BatchExportPages 批量导出页面 zip（md + manifest.json）
func (a *App) BatchExportPages(ctx context.Context, ids []int64) (string, []byte, error) {
	items := make([]importItem, 0, len(ids))
	for _, id := range ids {
		p, err := a.Repo.GetPage(ctx, id)
		if err != nil {
			continue
		}
		items = append(items, importItem{
			file: p.Slug + ".md",
			body: p.Content,
			in: ManifestItem{
				File: p.Slug + ".md", Title: p.Title, Slug: p.Slug, Summary: p.Summary,
				Cover: p.Cover, Gallery: p.Gallery, Sort: p.Sort, IsPublished: p.IsPublished, PublishedAt: p.PublishedAt,
			},
		})
	}
	return buildZip("pages", items)
}

func buildZip(prefix string, items []importItem) (string, []byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	defer func() { _ = zw.Close() }()
	manifest := make([]ManifestItem, 0, len(items))
	for _, it := range items {
		w, err := zw.Create(it.file)
		if err != nil {
			return "", nil, err
		}
		if _, err := w.Write([]byte(it.body)); err != nil {
			return "", nil, err
		}
		manifest = append(manifest, it.in)
	}
	w, err := zw.Create("manifest.json")
	if err != nil {
		return "", nil, err
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", nil, err
	}
	if _, err := w.Write(b); err != nil {
		return "", nil, err
	}
	if err := zw.Close(); err != nil {
		return "", nil, err
	}
	return prefix + "-export-" + time.Now().UTC().Format("20060102-150405") + ".zip", buf.Bytes(), nil
}

// ImportPosts 导入文章（zip 或单篇 md），统一归入 dirID
func (a *App) ImportPosts(ctx context.Context, filename string, data []byte, dirID int64) (*ImportResult, error) {
	if err := a.ensureDir(ctx, dirID); err != nil {
		return nil, err
	}
	items, parseErrs, err := readImportData(filename, data)
	if err != nil {
		return nil, pkg.BadRequest(err.Error())
	}
	res := &ImportResult{Errors: append([]ImportError{}, parseErrs...)}
	res.Skipped += len(parseErrs)
	for _, it := range items {
		in := model.PostInput{
			DirID: dirID, Title: it.in.Title, Slug: it.in.Slug, Content: it.body,
			Summary: it.in.Summary, Cover: it.in.Cover, Gallery: it.in.Gallery, Tags: it.in.Tags,
			Sort: it.in.Sort, IsPublished: it.in.IsPublished, PublishedAt: it.in.PublishedAt,
		}
		if _, err := a.CreatePost(ctx, in); err != nil {
			res.Skipped++
			res.Errors = append(res.Errors, ImportError{File: it.file, Reason: err.Error()})
			continue
		}
		res.Imported++
	}
	return res, nil
}

// ImportPages 导入独立页面（zip 或单篇 md），统一归入 dirID
func (a *App) ImportPages(ctx context.Context, filename string, data []byte, dirID int64) (*ImportResult, error) {
	if err := a.ensureDir(ctx, dirID); err != nil {
		return nil, err
	}
	items, parseErrs, err := readImportData(filename, data)
	if err != nil {
		return nil, pkg.BadRequest(err.Error())
	}
	res := &ImportResult{Errors: append([]ImportError{}, parseErrs...)}
	res.Skipped += len(parseErrs)
	for _, it := range items {
		in := model.PageInput{
			DirID: dirID, Title: it.in.Title, Slug: it.in.Slug, Content: it.body,
			Summary: it.in.Summary, Cover: it.in.Cover, Gallery: it.in.Gallery, Sort: it.in.Sort,
			IsPublished: it.in.IsPublished, PublishedAt: it.in.PublishedAt,
		}
		if _, err := a.CreatePage(ctx, in); err != nil {
			res.Skipped++
			res.Errors = append(res.Errors, ImportError{File: it.file, Reason: err.Error()})
			continue
		}
		res.Imported++
	}
	return res, nil
}

// readImportData 解析 zip 或单个 md
func readImportData(filename string, data []byte) ([]importItem, []ImportError, error) {
	if len(data) == 0 {
		return nil, nil, &pkg.AppError{Status: 400, Code: pkg.CodeBadRequest, Msg: "文件为空"}
	}
	isZip := len(data) >= 4 && bytes.Equal(data[:4], []byte{'P', 'K', 3, 4})
	if !isZip {
		if !strings.HasSuffix(strings.ToLower(filename), ".md") {
			return nil, nil, &pkg.AppError{Status: 400, Code: pkg.CodeBadRequest, Msg: "仅支持 .md 或 .zip 文件"}
		}
		if len(data) > 10<<20 {
			return nil, nil, &pkg.AppError{Status: 400, Code: pkg.CodeBadRequest, Msg: "单篇 Markdown 不能超过 10MB"}
		}
		base := strings.TrimSuffix(path.Base(strings.ReplaceAll(filename, "\\", "/")), ".md")
		base = strings.TrimSuffix(base, ".MD")
		return []importItem{{
			file: path.Base(filename),
			in:   ManifestItem{File: path.Base(filename), Title: base, Slug: deriveSlug(base)},
			body: string(data),
		}}, nil, nil
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, &pkg.AppError{Status: 400, Code: pkg.CodeBadRequest, Msg: "zip 文件损坏"}
	}
	if len(zr.File) > 2000 {
		return nil, nil, &pkg.AppError{Status: 400, Code: pkg.CodeBadRequest, Msg: "zip 条目过多"}
	}
	var total int64
	manifestByFile := map[string]ManifestItem{}
	manifestBySlug := map[string]ManifestItem{}
	items := []importItem{}
	errs := []ImportError{}
	names := make([]string, 0, len(zr.File))
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !pkg.SafeZipEntry(f.Name) || strings.HasPrefix(f.Name, "__MACOSX/") {
			continue
		}
		names = append(names, f.Name)
		files[f.Name] = f
	}
	sort.Strings(names)
	for _, name := range names {
		if strings.EqualFold(path.Base(name), "manifest.json") && !strings.Contains(name, "/") {
			rc, err := files[name].Open()
			if err != nil {
				continue
			}
			buf, _ := io.ReadAll(io.LimitReader(rc, 4<<20))
			_ = rc.Close()
			parseManifest(buf, manifestByFile, manifestBySlug)
		}
	}
	for _, name := range names {
		f := files[name]
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		if f.UncompressedSize64 > 10<<20 {
			errs = append(errs, ImportError{File: name, Reason: "单篇文件超过 10MB"})
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > 100<<20 {
			errs = append(errs, ImportError{File: name, Reason: "压缩包解压总量超过 100MB"})
			break
		}
		rc, err := f.Open()
		if err != nil {
			errs = append(errs, ImportError{File: name, Reason: "读取失败"})
			continue
		}
		buf, err := io.ReadAll(io.LimitReader(rc, 10<<20))
		_ = rc.Close()
		if err != nil {
			errs = append(errs, ImportError{File: name, Reason: "读取失败"})
			continue
		}
		base := strings.TrimSuffix(path.Base(name), path.Ext(name))
		meta, ok := manifestByFile[path.Base(name)]
		if !ok {
			meta, ok = manifestByFile[name]
		}
		if !ok {
			if m, found := manifestBySlug[deriveSlug(base)]; found {
				meta = m
			}
		}
		if meta.Title == "" {
			meta.Title = base
		}
		if meta.Slug == "" {
			meta.Slug = deriveSlug(base)
		}
		meta.File = name
		items = append(items, importItem{file: name, in: meta, body: string(buf)})
	}
	if len(items) == 0 && len(errs) == 0 {
		return nil, nil, &pkg.AppError{Status: 400, Code: pkg.CodeBadRequest, Msg: "压缩包内未找到 Markdown 文件"}
	}
	return items, errs, nil
}

func parseManifest(buf []byte, byFile, bySlug map[string]ManifestItem) {
	var list []ManifestItem
	if err := json.Unmarshal(buf, &list); err != nil {
		var wrap struct {
			Items []ManifestItem `json:"items"`
		}
		if err := json.Unmarshal(buf, &wrap); err != nil {
			return
		}
		list = wrap.Items
	}
	for _, m := range list {
		if m.File != "" {
			byFile[m.File] = m
			byFile[path.Base(m.File)] = m
		}
		if m.Slug != "" {
			bySlug[m.Slug] = m
		}
	}
}
