package service

import (
	"context"
	"log/slog"

	"marksite/internal/model"
	"marksite/internal/pkg"
	"marksite/internal/repo"
)

// ListPages 页面分页列表（批量填充 tags + 展开目录递归子目录 + 填充 DirName/DirPath）
func (a *App) ListPages(ctx context.Context, f repo.PageFilter) ([]model.Page, int64, error) {
	if len(f.DirIDs) == 0 && f.DirID != nil {
		desc, err := a.Repo.CollectDescendantIDs(ctx, *f.DirID)
		if err == nil && len(desc) > 1 {
			f.DirIDs = desc
			f.DirID = nil
		}
	}

	list, total, err := a.Repo.ListPages(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	if len(list) > 0 {
		dirMap, _ := a.Repo.BatchDirMetaMap(ctx)
		ids := make([]int64, len(list))
		for i, p := range list {
			if meta, ok := dirMap[p.DirID]; ok {
				list[i].DirName = meta.Name
				list[i].DirPath = meta.Path
			}
			ids[i] = p.ID
		}
		tagsMap, _ := a.Repo.Tag().GetPagesTagsMap(ctx, ids)
		for i := range list {
			list[i].Tags = tagsMap[list[i].ID]
		}
	}
	return list, total, nil
}

// GetPage 页面详情
func (a *App) GetPage(ctx context.Context, id int64) (*model.Page, error) {
	p, err := a.Repo.GetPage(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("页面不存在")
	}
	p.Tags, _ = a.Repo.Tag().GetPageTags(ctx, id)
	if dirMap, err := a.Repo.BatchDirMetaMap(ctx); err == nil {
		if meta, ok := dirMap[p.DirID]; ok {
			p.DirName = meta.Name
			p.DirPath = meta.Path
		}
	}
	return p, nil
}

// CreatePage 新建独立页面（事务包裹：正文 + 标签原子写入）
func (a *App) CreatePage(ctx context.Context, in model.PageInput) (*model.Page, error) {
	p, err := buildPage(in)
	if err != nil {
		return nil, err
	}
	if err := a.ensureDir(ctx, p.DirID); err != nil {
		return nil, err
	}
	taken, err := a.slugTaken(ctx, p.Slug, false, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, pkg.Conflict("slug 已被占用")
	}
	now := pkg.NowStr()
	p.CreatedAt = now
	p.UpdatedAt = now

	var created *model.Page
	if err := a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		id, err := tx.CreatePage(ctx, p)
		if err != nil {
			return err
		}
		tagIDs, err := a.resolveTagNames(ctx, tx, in.Tags)
		if err != nil {
			return err
		}
		if err := tx.Tag().SetPageTags(ctx, id, tagIDs); err != nil {
			return err
		}
		page, err := tx.GetPage(ctx, id)
		if err != nil {
			return err
		}
		page.Tags, err = tx.Tag().GetPageTags(ctx, id)
		if err != nil {
			return err
		}
		created = page
		return nil
	}); err != nil {
		return nil, err
	}
	if err := a.indexPage(*created); err != nil {
		slog.Warn("页面索引更新失败", "id", created.ID, "err", err)
	}
	return created, nil
}

// UpdatePage 全量更新页面（事务包裹：快照 + 正文 + 标签原子写入）
func (a *App) UpdatePage(ctx context.Context, id int64, in model.PageInput) (*model.Page, error) {
	old, err := a.Repo.GetPage(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("页面不存在")
	}
	next, err := buildPage(in)
	if err != nil {
		return nil, err
	}
	if err := a.ensureDir(ctx, next.DirID); err != nil {
		return nil, err
	}
	taken, err := a.slugTaken(ctx, next.Slug, false, id)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, pkg.Conflict("slug 已被占用")
	}
	next.ID = id
	next.CreatedAt = old.CreatedAt
	next.UpdatedAt = pkg.NowStr()
	a.snapshotPage(ctx, old)

	var updated *model.Page
	if err := a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		if err := tx.UpdatePage(ctx, next); err != nil {
			return err
		}
		tagIDs, err := a.resolveTagNames(ctx, tx, in.Tags)
		if err != nil {
			return err
		}
		if err := tx.Tag().SetPageTags(ctx, id, tagIDs); err != nil {
			return err
		}
		page, err := tx.GetPage(ctx, id)
		if err != nil {
			return err
		}
		page.Tags, err = tx.Tag().GetPageTags(ctx, id)
		if err != nil {
			return err
		}
		updated = page
		return nil
	}); err != nil {
		return nil, err
	}
	if err := a.indexPage(*updated); err != nil {
		slog.Warn("页面索引更新失败", "id", updated.ID, "err", err)
	}
	return updated, nil
}

// DeletePage 删除页面（含快照与索引）
func (a *App) DeletePage(ctx context.Context, id int64) error {
	if _, err := a.Repo.GetPage(ctx, id); err != nil {
		return pkg.NotFound("页面不存在")
	}
	if err := a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		if err := tx.DeleteSnapshotsByTarget(ctx, "page", id); err != nil {
			return err
		}
		return tx.DeletePage(ctx, id)
	}); err != nil {
		return err
	}
	if err := a.FTS.Delete("page", id); err != nil {
		slog.Warn("页面索引删除失败", "id", id, "err", err)
	}
	return nil
}

// PageSnapshots 页面快照列表（不含正文）
func (a *App) PageSnapshots(ctx context.Context, id int64) ([]model.SnapshotMeta, error) {
	if _, err := a.Repo.GetPage(ctx, id); err != nil {
		return nil, pkg.NotFound("页面不存在")
	}
	return a.Repo.ListSnapshotMeta(ctx, "page", id)
}

// RollbackPage 回滚页面到指定快照（事务包裹：快照 + 正文 + 标签原子写入）
func (a *App) RollbackPage(ctx context.Context, id, snapshotID int64) (*model.Page, error) {
	cur, err := a.Repo.GetPage(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("页面不存在")
	}
	sn, err := a.Repo.GetSnapshot(ctx, snapshotID)
	if err != nil || sn.TargetType != "page" || sn.TargetID != id {
		return nil, pkg.BadRequest("快照不存在或不属于该页面")
	}
	next := *cur
	next.DirID = sn.DirID
	next.Title = sn.Title
	next.Slug = sn.Slug
	next.Content = sn.Content
	next.Summary = sn.Summary
	next.Cover = sn.Cover
	next.Gallery = sn.Gallery
	next.UpdatedAt = pkg.NowStr()
	a.snapshotPage(ctx, cur)

	var updated *model.Page
	if err := a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		if err := tx.UpdatePage(ctx, &next); err != nil {
			return err
		}
		tagIDs, err := a.resolveTagNames(ctx, tx, sn.Tags)
		if err != nil {
			return err
		}
		if err := tx.Tag().SetPageTags(ctx, id, tagIDs); err != nil {
			return err
		}
		page, err := tx.GetPage(ctx, id)
		if err != nil {
			return err
		}
		page.Tags, err = tx.Tag().GetPageTags(ctx, id)
		if err != nil {
			return err
		}
		updated = page
		return nil
	}); err != nil {
		return nil, err
	}
	if err := a.indexPage(*updated); err != nil {
		slog.Warn("页面索引更新失败", "id", updated.ID, "err", err)
	}
	return updated, nil
}
