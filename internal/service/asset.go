package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"marksite/internal/model"
	"marksite/internal/pkg"
)

var allowedImageExt = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".ico": "image/x-icon",
}

var allowedImageMime = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "image/x-icon": true,
}

// assetURL 管理端访问地址
func assetURL(p string) string { return "/media/" + strings.TrimPrefix(p, "/") }

// UploadAsset 上传图片资源
func (a *App) UploadAsset(ctx context.Context, filename string, r io.Reader, size int64) (*model.Asset, error) {
	if size <= 0 {
		return nil, pkg.BadRequest("文件为空")
	}
	if size > 20<<20 {
		return nil, pkg.BadRequest("单文件不能超过 20MB")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	mime, ok := allowedImageExt[ext]
	if !ok {
		return nil, pkg.BadRequest("仅支持 png/jpg/jpeg/gif/webp/ico")
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	head = head[:n]
	detected := http.DetectContentType(head)
	if !allowedImageMime[strings.Split(detected, ";")[0]] {
		return nil, pkg.BadRequest("文件内容不是受支持的图片格式")
	}
	// 读取完整内容并压缩
	rest, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	raw := append(head, rest...)
	if int64(len(raw)) > 20<<20 {
		return nil, pkg.BadRequest("单文件不能超过 20MB")
	}
	compressed := compressImage(raw, ext)
	now := time.Now().UTC()
	relDir := now.Format("2006/01")
	name := itoa(now.UnixNano()) + ext
	relPath := relDir + "/" + name
	absDir, err := pkg.SafeJoin(a.Paths.AssetsDir, relDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, err
	}
	absPath := filepath.Join(absDir, name)
	f, err := os.Create(absPath)
	if err != nil {
		return nil, err
	}
	written, copyErr := io.Copy(f, bytes.NewReader(compressed))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(absPath)
		if copyErr != nil {
			return nil, copyErr
		}
		return nil, closeErr
	}
	asset := &model.Asset{
		Path: relPath, Filename: filepath.Base(filename), Size: written,
		Mime: mime, CreatedAt: pkg.NowStr(),
	}
	id, err := a.Repo.InsertAsset(ctx, asset)
	if err != nil {
		_ = os.Remove(absPath)
		return nil, err
	}
	created, err := a.Repo.GetAsset(ctx, id)
	if err != nil {
		return nil, err
	}
	created.URL = assetURL(created.Path)
	return created, nil
}

// ListAssets 资源列表（先重算引用计数）
func (a *App) ListAssets(ctx context.Context, keyword string, page, size int) ([]model.Asset, int64, error) {
	if _, err := a.recalcAssetRefs(ctx); err != nil {
		return nil, 0, err
	}
	list, total, err := a.Repo.ListAssets(ctx, keyword, page, size)
	if err != nil {
		return nil, 0, err
	}
	for i := range list {
		list[i].URL = assetURL(list[i].Path)
	}
	return list, total, nil
}

// DeleteAsset 删除资源（有引用时拒绝）
func (a *App) DeleteAsset(ctx context.Context, id int64) error {
	asset, err := a.Repo.GetAsset(ctx, id)
	if err != nil {
		return pkg.NotFound("资源不存在")
	}
	if _, err := a.recalcAssetRefs(ctx); err != nil {
		return err
	}
	if fresh, err := a.Repo.GetAsset(ctx, id); err == nil && fresh.RefCount > 0 {
		return pkg.Conflict("资源正在被引用，请先解除引用")
	}
	abs, err := pkg.SafeJoin(a.Paths.AssetsDir, asset.Path)
	if err != nil {
		return pkg.BadRequest("资源路径非法")
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return err
	}
	return a.Repo.DeleteAsset(ctx, id)
}

// CleanUnusedResult 清理结果
type CleanUnusedResult struct {
	Scanned      int   `json:"scanned"`
	DeletedCount int   `json:"deleted_count"`
	FreedBytes   int64 `json:"freed_bytes"`
}

// CleanUnusedAssets 清理未被引用的资源
func (a *App) CleanUnusedAssets(ctx context.Context) (*CleanUnusedResult, error) {
	refs, err := a.recalcAssetRefs(ctx)
	if err != nil {
		return nil, err
	}
	list, err := a.Repo.ListAllAssets(ctx)
	if err != nil {
		return nil, err
	}
	res := &CleanUnusedResult{Scanned: len(list)}
	for _, asset := range list {
		if refs[asset.ID] > 0 {
			continue
		}
		abs, err := pkg.SafeJoin(a.Paths.AssetsDir, asset.Path)
		if err == nil {
			_ = os.Remove(abs)
		}
		if err := a.Repo.DeleteAsset(ctx, asset.ID); err != nil {
			continue
		}
		res.DeletedCount++
		res.FreedBytes += asset.Size
	}
	return res, nil
}

// recalcAssetRefs 扫描内容与站点配置，回写引用计数
func (a *App) recalcAssetRefs(ctx context.Context) (map[int64]int, error) {
	assets, err := a.Repo.ListAllAssets(ctx)
	if err != nil {
		return nil, err
	}
	posts, err := a.Repo.ListAllPosts(ctx)
	if err != nil {
		return nil, err
	}
	pages, err := a.Repo.ListAllPages(ctx)
	if err != nil {
		return nil, err
	}
	site, err := a.Repo.GetSiteConfig(ctx)
	if err != nil {
		return nil, err
	}
	refs := make(map[int64]int, len(assets))
	for _, asset := range assets {
		count := 0
		for _, p := range posts {
			if contentUsesAsset(p.Content, p.Cover, asset.Path) || galleryUsesAsset(p.Gallery, asset.Path) {
				count++
			}
		}
		for _, p := range pages {
			if contentUsesAsset(p.Content, p.Cover, asset.Path) || galleryUsesAsset(p.Gallery, asset.Path) {
				count++
			}
		}
		if contentUsesAsset("", site.Logo, asset.Path) || contentUsesAsset("", site.Favicon, asset.Path) {
			count++
		}
		refs[asset.ID] = count
		if asset.RefCount != count {
			_ = a.Repo.UpdateAssetRef(ctx, asset.ID, count)
		}
	}
	return refs, nil
}

// galleryUsesAsset 判断图集是否引用资源
func galleryUsesAsset(gallery []string, assetPath string) bool {
	assetPath = strings.TrimPrefix(assetPath, "/")
	if assetPath == "" {
		return false
	}
	for _, g := range gallery {
		gp := strings.TrimPrefix(strings.TrimSpace(g), "/")
		gp = strings.TrimPrefix(gp, "media/")
		gp = strings.TrimPrefix(gp, "assets/")
		if gp == assetPath {
			return true
		}
	}
	return false
}

// contentUsesAsset 判断正文或封面是否引用资源
func contentUsesAsset(content, cover, assetPath string) bool {
	assetPath = strings.TrimPrefix(assetPath, "/")
	if assetPath == "" {
		return false
	}
	cov := strings.TrimPrefix(strings.TrimSpace(cover), "/")
	cov = strings.TrimPrefix(cov, "media/")
	if cov == assetPath {
		return true
	}
	if content == "" {
		return false
	}
	if strings.Contains(content, "/media/"+assetPath) || strings.Contains(content, assetPath) {
		return true
	}
	return false
}
