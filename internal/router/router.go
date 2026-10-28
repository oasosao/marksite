package router

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"marksite/internal/handler"
	"marksite/internal/middleware"
	"marksite/internal/pkg"
	"marksite/internal/service"
	"marksite/internal/webui"
)

// New 构建 HTTP 路由
func New(a *service.App) http.Handler {
	mux := http.NewServeMux()
	h := handler.New(a)

	// 本地单机工具，不需要登录鉴权；该别名保持下方路由注册写法不变
	handle := mux.HandleFunc

	handle("GET /api/v1/stats", h.Stats)

	handle("GET /api/v1/tree", h.Tree)
	handle("POST /api/v1/dirs", h.CreateDir)
	handle("GET /api/v1/dirs/{id}", h.GetDir)
	handle("PUT /api/v1/dirs/{id}", h.UpdateDir)
	handle("DELETE /api/v1/dirs/{id}", h.DeleteDir)
	handle("PUT /api/v1/posts/{id}/move", h.MovePost)
	handle("PUT /api/v1/pages/{id}/move", h.MovePage)
	handle("PUT /api/v1/tree/sort", h.SortTree)

	// 标签
	handle("GET /api/v1/tags", h.ListTags)
	handle("POST /api/v1/tags", h.CreateTag)
	handle("PUT /api/v1/tags/{id}", h.UpdateTag)
	handle("DELETE /api/v1/tags/{id}", h.DeleteTag)

	handle("GET /api/v1/posts", h.ListPosts)
	handle("POST /api/v1/posts", h.CreatePost)
	handle("POST /api/v1/posts/batch-export", h.BatchExportPosts)
	handle("POST /api/v1/posts/import", h.ImportPosts)
	handle("GET /api/v1/posts/{id}", h.GetPost)
	handle("PUT /api/v1/posts/{id}", h.UpdatePost)
	handle("DELETE /api/v1/posts/{id}", h.DeletePost)
	handle("GET /api/v1/posts/{id}/export", h.ExportPost)
	handle("GET /api/v1/posts/{id}/snapshots", h.PostSnapshots)
	handle("POST /api/v1/posts/{id}/rollback", h.RollbackPost)

	handle("GET /api/v1/pages", h.ListPages)
	handle("POST /api/v1/pages", h.CreatePage)
	handle("POST /api/v1/pages/batch-export", h.BatchExportPages)
	handle("POST /api/v1/pages/import", h.ImportPages)
	handle("GET /api/v1/pages/{id}", h.GetPage)
	handle("PUT /api/v1/pages/{id}", h.UpdatePage)
	handle("DELETE /api/v1/pages/{id}", h.DeletePage)
	handle("GET /api/v1/pages/{id}/export", h.ExportPage)
	handle("GET /api/v1/pages/{id}/snapshots", h.PageSnapshots)
	handle("POST /api/v1/pages/{id}/rollback", h.RollbackPage)

	handle("POST /api/v1/assets/upload", h.UploadAsset)
	handle("GET /api/v1/assets", h.ListAssets)
	handle("POST /api/v1/assets/clean-unused", h.CleanUnusedAssets)
	handle("DELETE /api/v1/assets/{id}", h.DeleteAsset)

	handle("GET /api/v1/themes", h.ListThemes)
	handle("POST /api/v1/themes/upload", h.UploadTheme)
	handle("PUT /api/v1/themes/active", h.ActiveTheme)
	handle("DELETE /api/v1/themes/{name}", h.DeleteTheme)

	handle("GET /api/v1/site-config", h.GetSiteConfig)
	handle("PUT /api/v1/site-config", h.UpdateSiteConfig)
	handle("GET /api/v1/app-config", h.GetAppConfig)
	handle("PUT /api/v1/app-config", h.UpdateAppConfig)

	handle("POST /api/v1/generate/full", h.GenerateFull)
	handle("POST /api/v1/generate/increment", h.GenerateIncrement)
	handle("GET /api/v1/generate/tasks", h.GenerateTasks)
	handle("POST /api/v1/generate/cancel/{id}", h.CancelTask)

	handle("GET /api/v1/backups", h.ListBackups)
	handle("POST /api/v1/backups/create", h.CreateBackup)
	handle("POST /api/v1/backups/restore/{name}", h.RestoreBackup)
	handle("GET /api/v1/backups/{name}/download", h.DownloadBackup)
	handle("DELETE /api/v1/backups/{name}", h.DeleteBackup)

	handle("GET /api/v1/search", h.Search)
	handle("POST /api/v1/fts/rebuild", h.RebuildFTS)
	handle("GET /api/v1/fts/status", h.FTSStatus)

	// 静态站点预览（服务 output 目录）
	handle("GET /preview/{path...}", h.PreviewSite)
	handle("GET /preview", h.PreviewSite)

	// 资源公开访问（基础设施静态路由，不属于业务 API）
	mux.HandleFunc("GET /media/{path...}", mediaHandler(a))

	dist, err := webui.FS()
	if err != nil {
		dist = nil
	}
	mux.Handle("/", spaHandler(dist))

	return middleware.Chain(mux, middleware.Recovery, middleware.AccessLog, middleware.NoCache)
}

// mediaHandler 输出 assets 目录下的资源文件
func mediaHandler(a *service.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(r.PathValue("path"), "/")
		if cleanPath == "" || strings.HasSuffix(cleanPath, "/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		abs, err := pkg.SafeJoin(a.Paths.AssetsDir, cleanPath)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if st, err := os.Stat(abs); err != nil || st.IsDir() {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, abs)
	}
}

// spaHandler 前端单页路由回退
func spaHandler(dist fs.FS) http.Handler {
	if dist == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				pkg.JSON(w, http.StatusNotFound, map[string]any{"code": pkg.CodeNotFound, "message": "接口不存在", "data": nil})
				return
			}
			http.Error(w, "前端资源未构建", http.StatusNotFound)
		})
	}
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") {
			pkg.JSON(w, http.StatusNotFound, map[string]any{"code": pkg.CodeNotFound, "message": "接口不存在", "data": nil})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		name := strings.TrimPrefix(path, "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(dist, name); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// 构建产物路径缺失视为 404，避免把 HTML 当作 JavaScript 返回
		if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/vditor/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.Error(w, "前端资源未构建", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(index); err != nil {
			return
		}
	})
}
