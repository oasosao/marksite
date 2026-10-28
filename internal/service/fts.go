package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"marksite/internal/fts"
	"marksite/internal/model"
	"marksite/internal/pkg"
)

// FTSStatus 索引状态
type FTSStatus struct {
	Exists    bool   `json:"exists"`
	Docs      uint64 `json:"docs"`
	Size      int64  `json:"size"`
	IndexPath string `json:"index_path"`
}

// SearchResult 检索结果
type SearchResult struct {
	List   []model.SearchHit `json:"list"`
	Total  uint64            `json:"total"`
	TookMS int64             `json:"took_ms"`
	Query  string            `json:"query"`
}

// FTSStatus 获取索引状态
func (a *App) FTSStatus() FTSStatus {
	return FTSStatus{
		Exists:    a.FTS.Exists(),
		Docs:      a.FTS.Count(),
		Size:      a.FTS.Size(),
		IndexPath: a.FTS.Path(),
	}
}

// Search 全文检索
func (a *App) Search(q string, limit int) (*SearchResult, error) {
	if q == "" {
		return nil, pkg.BadRequest("查询关键词不能为空")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	start := time.Now()
	list, total, err := a.FTS.Search(q, limit)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		List: list, Total: total, TookMS: time.Since(start).Milliseconds(), Query: q,
	}, nil
}

// StartFTSRebuild 创建索引重建任务
func (a *App) StartFTSRebuild(ctx context.Context) (int64, error) {
	a.taskMu.Lock()
	if a.ftsRunning {
		a.taskMu.Unlock()
		return 0, pkg.Busy("索引重建任务正在运行")
	}
	a.ftsRunning = true
	a.taskMu.Unlock()

	id, err := a.Repo.CreateTask(ctx, "fts_rebuild", pkg.NowStr())
	if err != nil {
		a.taskMu.Lock()
		a.ftsRunning = false
		a.taskMu.Unlock()
		return 0, err
	}
	go a.runFTSRebuild(id)
	return id, nil
}

func (a *App) runFTSRebuild(taskID int64) {
	defer func() {
		a.taskMu.Lock()
		a.ftsRunning = false
		a.taskMu.Unlock()
		a.clearCancel(taskID)
	}()

	ctx := context.Background()
	fail := func(err error) {
		slog.Error("索引重建失败", "task", taskID, "err", err)
		_ = a.Repo.FinishTask(ctx, taskID, "failed", err.Error(), pkg.NowStr())
	}
	if err := a.Repo.StartTask(ctx, taskID, pkg.NowStr()); err != nil {
		fail(err)
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
	// ListAllPosts 不填充标签，批量查询避免 N+1
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
	docs := make([]fts.Doc, 0, len(posts)+len(pages))
	for _, p := range posts {
		docs = append(docs, fts.Doc{
			Kind: "post", ID: p.ID, DirID: p.DirID, Title: p.Title, Slug: p.Slug,
			Summary: p.Summary, Content: p.Content, Tags: tagNames(p.Tags),
			Published: p.IsPublished, UpdatedAt: p.UpdatedAt,
		})
	}
	for _, p := range pages {
		docs = append(docs, fts.Doc{
			Kind: "page", ID: p.ID, DirID: p.DirID, Title: p.Title, Slug: p.Slug,
			Summary: p.Summary, Content: p.Content, Published: p.IsPublished, UpdatedAt: p.UpdatedAt,
		})
	}

	err = a.FTS.ReplaceAll(docs, func(done, total int) {
		progress := 0
		if total > 0 {
			progress = done * 100 / total
		}
		_ = a.Repo.UpdateTaskProgress(ctx, taskID, done, total, progress)
		if done == total || done%20 == 0 {
			_ = a.Repo.AppendTaskLog(ctx, taskID, fmt.Sprintf("索引 %d/%d\n", done, total))
		}
	}, func() bool { return a.isCanceled(taskID) })
	if errors.Is(err, fts.ErrCanceled) {
		_ = a.Repo.FinishTask(ctx, taskID, "canceled", "", pkg.NowStr())
		return
	}
	if err != nil {
		fail(err)
		return
	}
	_ = a.Repo.AppendTaskLog(ctx, taskID, fmt.Sprintf("索引重建完成，共 %d 条\n", len(docs)))
	_ = a.Repo.FinishTask(ctx, taskID, "success", "", pkg.NowStr())
	_ = a.Repo.UpdateTaskProgress(ctx, taskID, 1, 1, 100)
}
