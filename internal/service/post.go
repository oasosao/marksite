package service

import (
	"context"
	"log/slog"

	"marksite/internal/model"
	"marksite/internal/pkg"
	"marksite/internal/repo"
)

// ListPosts 文章分页列表（批量填充 tags + 展开目录递归子目录 + 填充 DirName/DirPath）
func (a *App) ListPosts(ctx context.Context, f repo.PostFilter) ([]model.Post, int64, error) {
	// 若只指定了单个 DirID（非 DirIDs），自动展开为自身 + 所有后代目录
	if len(f.DirIDs) == 0 && f.DirID != nil {
		desc, err := a.Repo.CollectDescendantIDs(ctx, *f.DirID)
		if err == nil && len(desc) > 1 {
			f.DirIDs = desc
			f.DirID = nil
		}
	}

	list, total, err := a.Repo.ListPosts(ctx, f)
	if err != nil {
		return nil, 0, err
	}

	// 一次性批量填充 DirName / DirPath / Tags
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
		tagsMap, _ := a.Repo.Tag().GetPostsTagsMap(ctx, ids)
		for i := range list {
			list[i].Tags = tagsMap[list[i].ID]
		}
	}
	return list, total, nil
}

// GetPost 文章详情
func (a *App) GetPost(ctx context.Context, id int64) (*model.Post, error) {
	p, err := a.Repo.GetPost(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("文章不存在")
	}
	p.Tags, _ = a.Repo.Tag().GetPostTags(ctx, id)
	if dirMap, err := a.Repo.BatchDirMetaMap(ctx); err == nil {
		if meta, ok := dirMap[p.DirID]; ok {
			p.DirName = meta.Name
			p.DirPath = meta.Path
		}
	}
	return p, nil
}

// CreatePost 新建文章（事务包裹：正文 + 标签原子写入）
func (a *App) CreatePost(ctx context.Context, in model.PostInput) (*model.Post, error) {
	p, err := buildPost(in)
	if err != nil {
		return nil, err
	}
	if err := a.ensureDir(ctx, p.DirID); err != nil {
		return nil, err
	}
	taken, err := a.slugTaken(ctx, p.Slug, true, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, pkg.Conflict("slug 已被占用")
	}
	now := pkg.NowStr()
	p.CreatedAt = now
	p.UpdatedAt = now

	var created *model.Post
	if err := a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		id, err := tx.CreatePost(ctx, p)
		if err != nil {
			return err
		}
		tagIDs, err := a.resolveTagNames(ctx, tx, in.Tags)
		if err != nil {
			return err
		}
		if err := tx.Tag().SetPostTags(ctx, id, tagIDs); err != nil {
			return err
		}
		post, err := tx.GetPost(ctx, id)
		if err != nil {
			return err
		}
		post.Tags, err = tx.Tag().GetPostTags(ctx, id)
		if err != nil {
			return err
		}
		created = post
		return nil
	}); err != nil {
		return nil, err
	}
	if err := a.indexPost(*created); err != nil {
		slog.Warn("文章索引更新失败", "id", created.ID, "err", err)
	}
	return created, nil
}

// UpdatePost 全量更新文章（事务包裹：快照 + 正文 + 标签原子写入）
func (a *App) UpdatePost(ctx context.Context, id int64, in model.PostInput) (*model.Post, error) {
	old, err := a.Repo.GetPost(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("文章不存在")
	}
	next, err := buildPost(in)
	if err != nil {
		return nil, err
	}
	if err := a.ensureDir(ctx, next.DirID); err != nil {
		return nil, err
	}
	taken, err := a.slugTaken(ctx, next.Slug, true, id)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, pkg.Conflict("slug 已被占用")
	}
	next.ID = id
	next.CreatedAt = old.CreatedAt
	next.UpdatedAt = pkg.NowStr()
	// 内容或元信息变化时保存旧版本快照（失败仅告警，不阻断主流程）
	a.snapshotPost(ctx, old)

	var updated *model.Post
	if err := a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		if err := tx.UpdatePost(ctx, next); err != nil {
			return err
		}
		tagIDs, err := a.resolveTagNames(ctx, tx, in.Tags)
		if err != nil {
			return err
		}
		if err := tx.Tag().SetPostTags(ctx, id, tagIDs); err != nil {
			return err
		}
		post, err := tx.GetPost(ctx, id)
		if err != nil {
			return err
		}
		post.Tags, err = tx.Tag().GetPostTags(ctx, id)
		if err != nil {
			return err
		}
		updated = post
		return nil
	}); err != nil {
		return nil, err
	}
	if err := a.indexPost(*updated); err != nil {
		slog.Warn("文章索引更新失败", "id", updated.ID, "err", err)
	}
	return updated, nil
}

// DeletePost 删除文章（含快照与索引）
func (a *App) DeletePost(ctx context.Context, id int64) error {
	if _, err := a.Repo.GetPost(ctx, id); err != nil {
		return pkg.NotFound("文章不存在")
	}
	if err := a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		if err := tx.DeleteSnapshotsByTarget(ctx, "post", id); err != nil {
			return err
		}
		return tx.DeletePost(ctx, id)
	}); err != nil {
		return err
	}
	if err := a.FTS.Delete("post", id); err != nil {
		slog.Warn("文章索引删除失败", "id", id, "err", err)
	}
	return nil
}

// PostSnapshots 文章快照列表（不含正文）
func (a *App) PostSnapshots(ctx context.Context, id int64) ([]model.SnapshotMeta, error) {
	if _, err := a.Repo.GetPost(ctx, id); err != nil {
		return nil, pkg.NotFound("文章不存在")
	}
	return a.Repo.ListSnapshotMeta(ctx, "post", id)
}

// RollbackPost 回滚到指定快照（事务包裹：快照 + 正文 + 标签原子写入）
func (a *App) RollbackPost(ctx context.Context, id, snapshotID int64) (*model.Post, error) {
	cur, err := a.Repo.GetPost(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("文章不存在")
	}
	sn, err := a.Repo.GetSnapshot(ctx, snapshotID)
	if err != nil || sn.TargetType != "post" || sn.TargetID != id {
		return nil, pkg.BadRequest("快照不存在或不属于该文章")
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
	a.snapshotPost(ctx, cur)

	var updated *model.Post
	if err := a.Repo.WithTx(ctx, func(tx *repo.Repo) error {
		if err := tx.UpdatePost(ctx, &next); err != nil {
			return err
		}
		// 从快照 tag name 列表重建标签关联
		tagIDs, err := a.resolveTagNames(ctx, tx, sn.Tags)
		if err != nil {
			return err
		}
		if err := tx.Tag().SetPostTags(ctx, id, tagIDs); err != nil {
			return err
		}
		post, err := tx.GetPost(ctx, id)
		if err != nil {
			return err
		}
		post.Tags, err = tx.Tag().GetPostTags(ctx, id)
		if err != nil {
			return err
		}
		updated = post
		return nil
	}); err != nil {
		return nil, err
	}
	if err := a.indexPost(*updated); err != nil {
		slog.Warn("文章索引更新失败", "id", updated.ID, "err", err)
	}
	return updated, nil
}
