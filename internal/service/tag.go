package service

import (
	"context"
	"strings"

	"marksite/internal/model"
	"marksite/internal/pkg"
)

// ListTags 标签列表（带引用计数）
func (a *App) ListTags(ctx context.Context) ([]model.TagCount, error) {
	return a.Repo.Tag().ListWithCount(ctx)
}

// CreateTag 新建标签（含 name/slug 唯一性校验）
func (a *App) CreateTag(ctx context.Context, in model.TagInput) (*model.Tag, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, pkg.BadRequest("标签名不能为空")
	}
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = pkg.Slugify(name, "tag")
	}
	if used, err := a.Repo.Tag().SlugExists(ctx, slug, 0); err != nil {
		return nil, err
	} else if used {
		return nil, pkg.Conflict("slug 已存在")
	}
	if used, err := a.Repo.Tag().NameExists(ctx, name, 0); err != nil {
		return nil, err
	} else if used {
		return nil, pkg.Conflict("标签名已存在")
	}
	id, err := a.Repo.Tag().Create(ctx, name, slug, in.Sort, pkg.NowStr())
	if err != nil {
		return nil, err
	}
	return a.Repo.Tag().GetByID(ctx, id)
}

// UpdateTag 更新标签（含唯一性校验）
func (a *App) UpdateTag(ctx context.Context, id int64, in model.TagUpdateInput) (*model.Tag, error) {
	if _, err := a.Repo.Tag().GetByID(ctx, id); err != nil {
		return nil, pkg.NotFound("标签不存在")
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if used, err := a.Repo.Tag().NameExists(ctx, name, id); err != nil {
			return nil, err
		} else if used {
			return nil, pkg.Conflict("标签名已存在")
		}
	}
	if in.Slug != nil {
		slug := strings.TrimSpace(*in.Slug)
		if used, err := a.Repo.Tag().SlugExists(ctx, slug, id); err != nil {
			return nil, err
		} else if used {
			return nil, pkg.Conflict("slug 已存在")
		}
	}
	if err := a.Repo.Tag().Update(ctx, id, in.Name, in.Slug, in.Sort); err != nil {
		return nil, err
	}
	return a.Repo.Tag().GetByID(ctx, id)
}

// DeleteTag 删除标签（同时清理文章/页面关联）
func (a *App) DeleteTag(ctx context.Context, id int64) (*model.Tag, error) {
	tag, err := a.Repo.Tag().GetByID(ctx, id)
	if err != nil {
		return nil, pkg.NotFound("标签不存在")
	}
	if err := a.Repo.Tag().Delete(ctx, id); err != nil {
		return nil, err
	}
	return tag, nil
}
