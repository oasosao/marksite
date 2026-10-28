package handler

import (
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"marksite/internal/model"
	"marksite/internal/pkg"
	"marksite/internal/repo"
	"marksite/internal/service"
)

// Handler 聚合全部 HTTP 接口
type Handler struct {
	A *service.App
}

func New(a *service.App) *Handler {
	return &Handler{A: a}
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		pkg.WriteErr(w, r, pkg.BadRequest("ID 非法"))
		return 0, false
	}
	return id, true
}

// setDownloadHeader 设置下载响应头，兼容中文文件名（RFC 5987）
func setDownloadHeader(w http.ResponseWriter, filename string) {
	encoded := url.PathEscape(filename)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+encoded+"\"; filename*=UTF-8''"+encoded)
}

// Stats 仪表盘
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.A.Stats(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, stats)
}

// ---------------- 内容树 ----------------

// Tree 混合树
func (h *Handler) Tree(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.A.Tree(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, nodes)
}

// CreateDir 新建分类
func (h *Handler) CreateDir(w http.ResponseWriter, r *http.Request) {
	var in model.DirInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	dir, err := h.A.CreateDir(r.Context(), in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, dir)
}

// GetDir 分类详情
func (h *Handler) GetDir(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	dir, err := h.A.Repo.GetDir(r.Context(), id)
	if err != nil {
		pkg.WriteErr(w, r, pkg.NotFound("分类不存在"))
		return
	}
	pkg.OK(w, dir)
}

// UpdateDir 更新分类
func (h *Handler) UpdateDir(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in model.DirUpdateInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	dir, err := h.A.UpdateDir(r.Context(), id, in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, dir)
}

// DeleteDir 删除分类
func (h *Handler) DeleteDir(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	mode := pkg.Query(r, "mode")
	res, err := h.A.DeleteDir(r.Context(), id, mode)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, res)
}

// MovePost 移动文章
func (h *Handler) MovePost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in model.MoveInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	post, err := h.A.MovePost(r.Context(), id, in.DirID)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, post)
}

// MovePage 移动页面
func (h *Handler) MovePage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in model.MoveInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	page, err := h.A.MovePage(r.Context(), id, in.DirID)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, page)
}

// SortTree 同层排序
func (h *Handler) SortTree(w http.ResponseWriter, r *http.Request) {
	var in model.TreeSortInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	if err := h.A.SortTree(r.Context(), in); err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, nil)
}

// ---------------- 文章 ----------------

func postFilter(w http.ResponseWriter, r *http.Request) (repo.PostFilter, int, int) {
	page, size := pkg.ParsePagination(r)
	f := repo.PostFilter{
		Keyword: pkg.Query(r, "keyword"),
		Page:    page,
		Size:    size,
	}
	if v := pkg.Query(r, "dir_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.DirID = &id
		}
	}
	if v := pkg.Query(r, "is_published"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Published = &n
		}
	}
	return f, page, size
}

// ListPosts 文章列表
func (h *Handler) ListPosts(w http.ResponseWriter, r *http.Request) {
	f, page, size := postFilter(w, r)
	list, total, err := h.A.ListPosts(r.Context(), f)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, map[string]any{"list": list, "total": total, "page": page, "page_size": size})
}

// GetPost 文章详情
func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	post, err := h.A.GetPost(r.Context(), id)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, post)
}

// CreatePost 新建文章
func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	var in model.PostInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	post, err := h.A.CreatePost(r.Context(), in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, post)
}

// UpdatePost 更新文章
func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in model.PostInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	post, err := h.A.UpdatePost(r.Context(), id, in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, post)
}

// DeletePost 删除文章
func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.A.DeletePost(r.Context(), id); err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, nil)
}

// ExportPost 导出单篇
func (h *Handler) ExportPost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	name, data, err := h.A.ExportPost(r.Context(), id)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	setDownloadHeader(w, name)
	pkg.Data(w, http.StatusOK, "text/markdown; charset=utf-8", data)
}

// BatchExportPosts 批量导出
func (h *Handler) BatchExportPosts(w http.ResponseWriter, r *http.Request) {
	var in model.BatchIDsInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	name, data, err := h.A.BatchExportPosts(r.Context(), in.IDs)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	setDownloadHeader(w, name)
	pkg.Data(w, http.StatusOK, "application/zip", data)
}

// ImportPosts 导入文章
func (h *Handler) ImportPosts(w http.ResponseWriter, r *http.Request) {
	dirID, filename, data, err := readUpload(w, r)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	res, err := h.A.ImportPosts(r.Context(), filename, data, dirID)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, res)
}

// PostSnapshots 快照列表
func (h *Handler) PostSnapshots(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	list, err := h.A.PostSnapshots(r.Context(), id)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, list)
}

// RollbackPost 回滚
func (h *Handler) RollbackPost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in model.RollbackInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	post, err := h.A.RollbackPost(r.Context(), id, in.SnapshotID)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, post)
}

// ---------------- 独立页面 ----------------

func pageFilter(w http.ResponseWriter, r *http.Request) (repo.PageFilter, int, int) {
	page, size := pkg.ParsePagination(r)
	f := repo.PageFilter{Keyword: pkg.Query(r, "keyword"), Page: page, Size: size}
	if v := pkg.Query(r, "dir_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.DirID = &id
		}
	}
	if v := pkg.Query(r, "is_published"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Published = &n
		}
	}
	return f, page, size
}

// ListPages 页面列表
func (h *Handler) ListPages(w http.ResponseWriter, r *http.Request) {
	f, page, size := pageFilter(w, r)
	list, total, err := h.A.ListPages(r.Context(), f)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, map[string]any{"list": list, "total": total, "page": page, "page_size": size})
}

// GetPage 页面详情
func (h *Handler) GetPage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	page, err := h.A.GetPage(r.Context(), id)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, page)
}

// CreatePage 新建页面
func (h *Handler) CreatePage(w http.ResponseWriter, r *http.Request) {
	var in model.PageInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	page, err := h.A.CreatePage(r.Context(), in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, page)
}

// UpdatePage 更新页面
func (h *Handler) UpdatePage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in model.PageInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	page, err := h.A.UpdatePage(r.Context(), id, in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, page)
}

// DeletePage 删除页面
func (h *Handler) DeletePage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.A.DeletePage(r.Context(), id); err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, nil)
}

// ExportPage 导出单页
func (h *Handler) ExportPage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	name, data, err := h.A.ExportPage(r.Context(), id)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	setDownloadHeader(w, name)
	pkg.Data(w, http.StatusOK, "text/markdown; charset=utf-8", data)
}

// BatchExportPages 批量导出页面
func (h *Handler) BatchExportPages(w http.ResponseWriter, r *http.Request) {
	var in model.BatchIDsInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	name, data, err := h.A.BatchExportPages(r.Context(), in.IDs)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	setDownloadHeader(w, name)
	pkg.Data(w, http.StatusOK, "application/zip", data)
}

// ImportPages 导入页面
func (h *Handler) ImportPages(w http.ResponseWriter, r *http.Request) {
	dirID, filename, data, err := readUpload(w, r)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	res, err := h.A.ImportPages(r.Context(), filename, data, dirID)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, res)
}

// PageSnapshots 页面快照列表
func (h *Handler) PageSnapshots(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	list, err := h.A.PageSnapshots(r.Context(), id)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, list)
}

// RollbackPage 页面回滚
func (h *Handler) RollbackPage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in model.RollbackInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	page, err := h.A.RollbackPage(r.Context(), id, in.SnapshotID)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, page)
}

// UploadAsset 上传资源
func (h *Handler) UploadAsset(w http.ResponseWriter, r *http.Request) {
	file, err := pkg.FormFile(w, r, "file")
	if err != nil {
		pkg.WriteErr(w, r, pkg.BadRequest("请选择要上传的图片"))
		return
	}
	src, err := file.Open()
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	defer func() { _ = src.Close() }()
	asset, err := h.A.UploadAsset(r.Context(), file.Filename, src, file.Size)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, asset)
}

// ListAssets 资源列表
func (h *Handler) ListAssets(w http.ResponseWriter, r *http.Request) {
	page, size := pkg.ParsePagination(r)
	list, total, err := h.A.ListAssets(r.Context(), pkg.Query(r, "keyword"), page, size)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, map[string]any{"list": list, "total": total, "page": page, "page_size": size})
}

// DeleteAsset 删除资源
func (h *Handler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.A.DeleteAsset(r.Context(), id); err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, nil)
}

// CleanUnusedAssets 清理未引用资源
func (h *Handler) CleanUnusedAssets(w http.ResponseWriter, r *http.Request) {
	res, err := h.A.CleanUnusedAssets(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, res)
}

// ---------------- 主题 ----------------

// ListThemes 主题列表
func (h *Handler) ListThemes(w http.ResponseWriter, r *http.Request) {
	list, err := h.A.ListThemes(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, list)
}

// UploadTheme 上传主题
func (h *Handler) UploadTheme(w http.ResponseWriter, r *http.Request) {
	file, err := pkg.FormFile(w, r, "file")
	if err != nil {
		pkg.WriteErr(w, r, pkg.BadRequest("请选择主题 zip 包"))
		return
	}
	src, err := file.Open()
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	defer func() { _ = src.Close() }()
	tmp, err := os.CreateTemp("", "marksite-theme-*.zip")
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	// 用实际读取字节数判断大小，防止 multipart header 伪造
	n, err := tmp.ReadFrom(src)
	_ = tmp.Close()
	if err != nil {
		pkg.WriteErr(w, r, pkg.BadRequest("读取主题包失败"))
		return
	}
	if n > 20<<20 {
		pkg.WriteErr(w, r, pkg.BadRequest("主题包不能超过 20MB"))
		return
	}
	info, err := h.A.InstallTheme(r.Context(), tmpName)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, info)
}

// ActiveTheme 激活主题
func (h *Handler) ActiveTheme(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	res, err := h.A.SetActiveTheme(r.Context(), in.Name)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, res)
}

// DeleteTheme 删除主题
func (h *Handler) DeleteTheme(w http.ResponseWriter, r *http.Request) {
	if err := h.A.DeleteTheme(r.Context(), r.PathValue("name")); err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, nil)
}

// ---------------- 配置 ----------------

// GetSiteConfig 站点配置
func (h *Handler) GetSiteConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.A.GetSiteConfig(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, cfg)
}

// UpdateSiteConfig 更新站点配置
func (h *Handler) UpdateSiteConfig(w http.ResponseWriter, r *http.Request) {
	var in model.SiteConfig
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	cfg, err := h.A.UpdateSiteConfig(r.Context(), in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, cfg)
}

// GetAppConfig 程序配置
func (h *Handler) GetAppConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.A.GetAppConfig(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, cfg)
}

// UpdateAppConfig 更新程序配置
func (h *Handler) UpdateAppConfig(w http.ResponseWriter, r *http.Request) {
	var in model.AppConfig
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	cfg, err := h.A.UpdateAppConfig(r.Context(), in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, cfg)
}

// ---------------- 生成 ----------------

// GenerateFull 全量生成
func (h *Handler) GenerateFull(w http.ResponseWriter, r *http.Request) {
	id, err := h.A.StartGenerate(r.Context(), "full")
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, map[string]any{"task_id": id})
}

// GenerateIncrement 增量生成
func (h *Handler) GenerateIncrement(w http.ResponseWriter, r *http.Request) {
	id, err := h.A.StartGenerate(r.Context(), "increment")
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, map[string]any{"task_id": id})
}

// GenerateTasks 任务查询
func (h *Handler) GenerateTasks(w http.ResponseWriter, r *http.Request) {
	if v := pkg.Query(r, "task_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			pkg.WriteErr(w, r, pkg.BadRequest("task_id 非法"))
			return
		}
		t, err := h.A.GetTask(r.Context(), id)
		if err != nil {
			pkg.WriteErr(w, r, err)
			return
		}
		pkg.OK(w, t)
		return
	}
	limit, _ := strconv.Atoi(pkg.QueryDefault(r, "limit", "20"))
	list, err := h.A.ListTasks(r.Context(), limit)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, list)
}

// CancelTask 取消任务
func (h *Handler) CancelTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	res, err := h.A.CancelTask(r.Context(), id)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, res)
}

// ---------------- 备份 ----------------

// ListBackups 备份列表
func (h *Handler) ListBackups(w http.ResponseWriter, r *http.Request) {
	list, err := h.A.ListBackups()
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, list)
}

// CreateBackup 创建备份
func (h *Handler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	res, err := h.A.CreateBackup(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, res)
}

// RestoreBackup 恢复备份
func (h *Handler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	res, err := h.A.RestoreBackup(r.Context(), r.PathValue("name"))
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, res)
}

// DownloadBackup 下载备份
func (h *Handler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	full, err := h.A.BackupFile(name)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.Attachment(w, r, full, name)
}

// DeleteBackup 删除备份
func (h *Handler) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	if err := h.A.DeleteBackup(r.PathValue("name")); err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, nil)
}

// ---------------- 检索 ----------------

// Search 全文检索
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := pkg.Query(r, "q")
	limit, _ := strconv.Atoi(pkg.QueryDefault(r, "limit", "20"))
	res, err := h.A.Search(q, limit)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, res)
}

// RebuildFTS 重建索引
func (h *Handler) RebuildFTS(w http.ResponseWriter, r *http.Request) {
	id, err := h.A.StartFTSRebuild(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, map[string]any{"task_id": id})
}

// FTSStatus 索引状态
func (h *Handler) FTSStatus(w http.ResponseWriter, r *http.Request) {
	pkg.OK(w, h.A.FTSStatus())
}

// --- 标签 CRUD ---

func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.A.ListTags(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, tags)
}

func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var in model.TagInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	tag, err := h.A.CreateTag(r.Context(), in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, tag)
}

func (h *Handler) UpdateTag(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in model.TagUpdateInput
	if !pkg.BindJSON(w, r, &in) {
		return
	}
	tag, err := h.A.UpdateTag(r.Context(), id, in)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, tag)
}

func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	tag, err := h.A.DeleteTag(r.Context(), id)
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	pkg.OK(w, tag)
}

// readUpload 解析 multipart 上传：读 dir_id 参数 + file 文件，限制 100MB
func readUpload(w http.ResponseWriter, r *http.Request) (dirID int64, filename string, data []byte, err error) {
	dirID, _ = strconv.ParseInt(pkg.PostForm(w, r, "dir_id"), 10, 64)
	file, err := pkg.FormFile(w, r, "file")
	if err != nil {
		return 0, "", nil, pkg.BadRequest("请选择要导入的文件")
	}
	if file.Size > 100<<20 {
		return 0, "", nil, pkg.BadRequest("文件不能超过 100MB")
	}
	src, err := file.Open()
	if err != nil {
		return 0, "", nil, err
	}
	defer func() { _ = src.Close() }()
	data, err = readLimited(src, 100<<20)
	if err != nil {
		return 0, "", nil, err
	}
	return dirID, file.Filename, data, nil
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, pkg.BadRequest("读取文件失败")
	}
	if int64(len(b)) > limit {
		return nil, pkg.BadRequest("文件过大")
	}
	return b, nil
}

// ---------------- 静态站点预览 ----------------

// PreviewSite 预览已生成的静态站点：服务 output 目录，HTML 内以 / 开头的绝对链接改写为 /preview/ 前缀，
// 使 iframe 内嵌预览时资源与跳转不脱离预览域。
func (h *Handler) PreviewSite(w http.ResponseWriter, r *http.Request) {
	outDir, err := h.A.ResolveOutputDir(r.Context())
	if err != nil {
		pkg.WriteErr(w, r, err)
		return
	}
	rel := strings.TrimPrefix(r.PathValue("path"), "/")
	if rel == "" {
		rel = "index.html"
	}
	abs, err := pkg.SafeJoin(outDir, rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	st, err := os.Stat(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// 目录自动回退到 index.html
	if st.IsDir() {
		abs = filepath.Join(abs, "index.html")
		if _, err := os.Stat(abs); err != nil {
			http.NotFound(w, r)
			return
		}
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ext := strings.ToLower(filepath.Ext(abs))
	if ext == ".html" || ext == ".htm" {
		// 改写绝对路径链接与资源，使其落在 /preview/ 前缀下（资源鉴权由 preview cookie 承接）
		body := strings.ReplaceAll(string(data), `href="/`, `href="/preview/`)
		body = strings.ReplaceAll(body, `src="/`, `src="/preview/`)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(body))
		return
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}
