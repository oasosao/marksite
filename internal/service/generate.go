package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"marksite/internal/generator"
	"marksite/internal/model"
	"marksite/internal/pkg"
)

// StartGenerate 创建生成任务并后台执行（同一时刻仅允许一个）
func (a *App) StartGenerate(ctx context.Context, mode string) (int64, error) {
	if mode != "full" && mode != "increment" {
		return 0, pkg.BadRequest("生成模式必须为 full 或 increment")
	}
	a.taskMu.Lock()
	if a.genRunning {
		a.taskMu.Unlock()
		return 0, pkg.Busy("已有生成任务正在运行")
	}
	a.genRunning = true
	a.taskMu.Unlock()

	id, err := a.Repo.CreateTask(ctx, mode, pkg.NowStr())
	if err != nil {
		a.taskMu.Lock()
		a.genRunning = false
		a.taskMu.Unlock()
		return 0, err
	}
	go a.runGenerate(id, mode)
	return id, nil
}

func (a *App) runGenerate(taskID int64, mode string) {
	defer func() {
		a.taskMu.Lock()
		a.genRunning = false
		a.taskMu.Unlock()
		a.clearCancel(taskID)
	}()

	ctx := context.Background()
	fail := func(err error) {
		slog.Error("静态站点生成失败", "task", taskID, "err", err)
		_ = a.Repo.FinishTask(ctx, taskID, "failed", err.Error(), pkg.NowStr())
	}
	if err := a.Repo.StartTask(ctx, taskID, pkg.NowStr()); err != nil {
		fail(err)
		return
	}

	site, err := a.Repo.GetSiteConfig(ctx)
	if err != nil {
		fail(err)
		return
	}
	cfg, err := a.Repo.GetAppConfig(ctx)
	if err != nil {
		fail(err)
		return
	}
	outDir := cfg.OutputDir
	if outDir == "" {
		outDir = a.Paths.OutputDir
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		fail(err)
		return
	}
	if absOut == string(filepath.Separator) {
		fail(pkg.BadRequest("输出目录不能为根目录"))
		return
	}
	for _, self := range selfDirs() {
		if absOut == self {
			fail(pkg.BadRequest("输出目录不能为程序目录本身"))
			return
		}
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		fail(err)
		return
	}

	th, err := a.Themes.Get(site.Theme)
	if err != nil {
		fail(pkg.BadRequest("主题不存在或无效"))
		return
	}
	posts, err := a.Repo.ListAllPosts(ctx)
	if err != nil {
		fail(err)
		return
	}
	pages, err := a.Repo.ListAllPages(ctx)
	if err != nil {
		fail(err)
		return
	}
	dirs, err := a.Repo.ListDirs(ctx)
	if err != nil {
		fail(err)
		return
	}
	tags, err := a.Repo.Tag().List(ctx)
	if err != nil {
		fail(err)
		return
	}
	// 填充 posts/pages 的 Tags 关联（批量查询，避免 N+1）
	postIDs := make([]int64, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
	}
	postTagsMap, err := a.Repo.Tag().GetPostsTagsMap(ctx, postIDs)
	if err != nil {
		fail(err)
		return
	}
	for i := range posts {
		if tags, ok := postTagsMap[posts[i].ID]; ok {
			posts[i].Tags = tags
		}
	}
	pageIDs := make([]int64, len(pages))
	for i, p := range pages {
		pageIDs[i] = p.ID
	}
	pageTagsMap, err := a.Repo.Tag().GetPagesTagsMap(ctx, pageIDs)
	if err != nil {
		fail(err)
		return
	}
	for i := range pages {
		if tags, ok := pageTagsMap[pages[i].ID]; ok {
			pages[i].Tags = tags
		}
	}

	full := mode == "full"
	changedPosts := map[int64]bool{}
	changedPages := map[int64]bool{}
	noItemChange := false
	if !full {
		baseline, _ := a.Repo.LastSuccessfulGenerateStart(ctx)
		if baseline == "" || site.UpdatedAt > baseline {
			// 无基线或站点配置变更，增量升格为全量
			full = true
		} else {
			for _, p := range posts {
				if p.UpdatedAt > baseline {
					changedPosts[p.ID] = true
				}
			}
			for _, p := range pages {
				if p.UpdatedAt > baseline {
					changedPages[p.ID] = true
				}
			}
			noItemChange = len(changedPosts) == 0 && len(changedPages) == 0
		}
	}
	if full {
		_ = a.Repo.AppendTaskLog(ctx, taskID, "模式：全量生成\n")
	} else {
		_ = a.Repo.AppendTaskLog(ctx, taskID, "模式：增量生成\n")
	}

	err = generator.Render(generator.Options{
		Site:         *site,
		Theme:        th,
		Posts:        posts,
		Pages:        pages,
		Dirs:         dirs,
		Tags:         tags,
		AssetsDir:    a.Paths.AssetsDir,
		OutDir:       absOut,
		Full:         full,
		ChangedPosts: changedPosts,
		ChangedPages: changedPages,
		NoItemChange: noItemChange,
		Progress: func(done, total int, msg string) {
			progress := 0
			if total > 0 {
				progress = done * 100 / total
			}
			_ = a.Repo.UpdateTaskProgress(ctx, taskID, done, total, progress)
			_ = a.Repo.AppendTaskLog(ctx, taskID, msg+"\n")
		},
		Canceled: func() bool { return a.isCanceled(taskID) },
	})
	if errors.Is(err, generator.ErrCanceled) {
		_ = a.Repo.FinishTask(ctx, taskID, "canceled", "", pkg.NowStr())
		return
	}
	if err != nil {
		fail(err)
		return
	}
	_ = a.Repo.FinishTask(ctx, taskID, "success", "", pkg.NowStr())
	_ = a.Repo.UpdateTaskProgress(ctx, taskID, 1, 1, 100)
}

// ResolveOutputDir 解析最终输出目录（配置优先，默认程序路径）
func (a *App) ResolveOutputDir(ctx context.Context) (string, error) {
	cfg, err := a.Repo.GetAppConfig(ctx)
	if err != nil {
		return "", err
	}
	outDir := cfg.OutputDir
	if outDir == "" {
		outDir = a.Paths.OutputDir
	}
	return filepath.Abs(outDir)
}

// CancelTask 取消任务
func (a *App) CancelTask(ctx context.Context, id int64) (map[string]any, error) {
	t, err := a.Repo.GetTask(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("任务不存在")
	}
	switch t.Status {
	case "pending", "running":
	default:
		return map[string]any{"id": id, "status": t.Status}, nil
	}
	a.markCanceled(id)
	status := "running"
	if t.Status == "pending" {
		_ = a.Repo.FinishTask(ctx, id, "canceled", "", pkg.NowStr())
		status = "canceled"
	}
	return map[string]any{"id": id, "status": status}, nil
}

// GetTask 任务详情
func (a *App) GetTask(ctx context.Context, id int64) (*model.Task, error) {
	t, err := a.Repo.GetTask(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("任务不存在")
	}
	return t, nil
}

// ListTasks 任务列表
func (a *App) ListTasks(ctx context.Context, limit int) ([]model.Task, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return a.Repo.ListTasks(ctx, limit)
}

// CleanupStaleTasks 启动时清理上次异常退出的任务
func (a *App) CleanupStaleTasks(ctx context.Context) {
	if err := a.Repo.AbortRunningTasks(ctx, "程序重启导致任务中断", pkg.NowStr()); err != nil {
		slog.Warn("清理中断任务失败", "err", err)
	}
}

// Busy 是否有生成/索引任务在运行（备份恢复前置校验）
func (a *App) Busy() bool {
	a.taskMu.Lock()
	defer a.taskMu.Unlock()
	return a.genRunning || a.ftsRunning
}
