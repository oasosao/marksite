package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"marksite/internal/fts"
	"marksite/internal/model"
	"marksite/internal/pkg"
	"marksite/internal/repo"
)

// getAppConfig 读取程序配置
func (a *App) getAppConfig(ctx context.Context) (*model.AppConfig, error) {
	return a.Repo.GetAppConfig(ctx)
}

// snapshotLimit 快照保留上限
func (a *App) snapshotLimit(ctx context.Context) int {
	cfg, err := a.getAppConfig(ctx)
	if err != nil || cfg.SnapshotLimit < 1 {
		return 20
	}
	return cfg.SnapshotLimit
}

// ensureDir 校验分类归属（0 或已存在）
func (a *App) ensureDir(ctx context.Context, dirID int64) error {
	if dirID == 0 {
		return nil
	}
	if _, err := a.Repo.GetDir(ctx, dirID); err != nil {
		return pkg.BadRequest("分类不存在")
	}
	return nil
}

// deriveSlug 由标题生成默认 slug（小写、空格转 -、非 ASCII 剔除）
func deriveSlug(title string) string {
	return pkg.Slugify(title, "untitled")
}

// validateTitle slug 与标题的公共校验
func validateTitleSlug(title, slug string) (string, string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", "", pkg.BadRequest("标题不能为空")
	}
	if utf8.RuneCountInString(title) > 200 {
		return "", "", pkg.BadRequest("标题长度不能超过 200 字符")
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		slug = deriveSlug(title)
	}
	if slug == "" {
		return "", "", pkg.BadRequest("slug 不能为空，请手动填写")
	}
	if !pkg.ValidSlug(slug) || len(slug) > 120 {
		return "", "", pkg.BadRequest("slug 仅允许小写字母、数字与中划线，长度 1–120")
	}
	return title, slug, nil
}

// slugTaken 跨 posts + pages 全局唯一检查
func (a *App) slugTaken(ctx context.Context, slug string, isPost bool, id int64) (bool, error) {
	postExclude, pageExclude := int64(0), int64(0)
	if isPost {
		postExclude = id
	} else {
		pageExclude = id
	}
	taken, err := a.Repo.PostSlugExists(ctx, slug, postExclude)
	if err != nil || taken {
		return taken, err
	}
	return a.Repo.PageSlugExists(ctx, slug, pageExclude)
}

// applyPublish 发布状态与发布时间处理
func applyPublish(isPublished int, publishedAt string) (int, string, error) {
	if isPublished != 1 {
		isPublished = 0
	}
	publishedAt = strings.TrimSpace(publishedAt)
	if publishedAt != "" {
		t, err := pkg.NormalizeTime(publishedAt)
		if err != nil {
			return 0, "", pkg.BadRequest("发布时间格式非法，应为 RFC3339")
		}
		publishedAt = t
	}
	if isPublished == 1 && publishedAt == "" {
		publishedAt = pkg.NowStr()
	}
	return isPublished, publishedAt, nil
}

// buildPost 将请求 DTO 校验并转换为模型
func buildPost(in model.PostInput) (*model.Post, error) {
	title, slug, err := validateTitleSlug(in.Title, in.Slug)
	if err != nil {
		return nil, err
	}
	isPublished, publishedAt, err := applyPublish(in.IsPublished, in.PublishedAt)
	if err != nil {
		return nil, err
	}
	sortVal := in.Sort
	if sortVal < 0 {
		sortVal = 0
	}
	return &model.Post{
		DirID: in.DirID, Title: title, Slug: slug, Content: in.Content,
		Summary: strings.TrimSpace(in.Summary), Cover: strings.TrimSpace(in.Cover),
		Gallery: in.Gallery, Tags: nil, Sort: sortVal,
		IsPublished: isPublished, PublishedAt: publishedAt,
	}, nil
}

// buildPage 将请求 DTO 校验并转换为模型
func buildPage(in model.PageInput) (*model.Page, error) {
	title, slug, err := validateTitleSlug(in.Title, in.Slug)
	if err != nil {
		return nil, err
	}
	isPublished, publishedAt, err := applyPublish(in.IsPublished, in.PublishedAt)
	if err != nil {
		return nil, err
	}
	sortVal := in.Sort
	if sortVal < 0 {
		sortVal = 0
	}
	return &model.Page{
		DirID: in.DirID, Title: title, Slug: slug, Content: in.Content,
		Summary: strings.TrimSpace(in.Summary), Cover: strings.TrimSpace(in.Cover),
		Gallery: in.Gallery, Sort: sortVal, IsPublished: isPublished, PublishedAt: publishedAt,
	}, nil
}

// indexPost 同步文章索引
func (a *App) indexPost(p model.Post) error {
	return a.FTS.Upsert(fts.Doc{
		Kind: "post", ID: p.ID, DirID: p.DirID, Title: p.Title, Slug: p.Slug,
		Summary: p.Summary, Content: p.Content, Tags: tagNames(p.Tags),
		Published: p.IsPublished, UpdatedAt: p.UpdatedAt,
	})
}

// tagNames 从 []model.Tag 提取名字列表
func tagNames(tags []model.Tag) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, t.Name)
	}
	return out
}

// resolveTagNames 从 tag name 列表解析/创建 tag，返回 tag id 列表
// r 可以是主 repo 或事务 repo，保证标签操作与调用方在同一事务内
func (a *App) resolveTagNames(ctx context.Context, r *repo.Repo, names []string) ([]int64, error) {
	if len(names) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(names))
	seen := map[int64]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		existing, err := r.Tag().GetByName(ctx, n)
		if err == nil {
			if !seen[existing.ID] {
				ids = append(ids, existing.ID)
				seen[existing.ID] = true
			}
			continue
		}
		// 不存在则创建（中文名派生的 fallback slug 可能冲突，自动追加序号）
		base := pkg.Slugify(n, "tag")
		slug := base
		for i := 1; ; i++ {
			used, err := r.Tag().SlugExists(ctx, slug, 0)
			if err != nil {
				return nil, err
			}
			if !used {
				break
			}
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		now := pkg.NowStr()
		id, err := r.Tag().Create(ctx, n, slug, 0, now)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

// indexPage 同步页面索引
func (a *App) indexPage(p model.Page) error {
	return a.FTS.Upsert(fts.Doc{
		Kind: "page", ID: p.ID, DirID: p.DirID, Title: p.Title, Slug: p.Slug,
		Summary: p.Summary, Content: p.Content, Published: p.IsPublished, UpdatedAt: p.UpdatedAt,
	})
}

// snapshotPost 保存文章旧版本快照并裁剪
func (a *App) snapshotPost(ctx context.Context, p *model.Post) {
	if p.ID == 0 {
		return
	}
	sn := &model.Snapshot{
		TargetType: "post", TargetID: p.ID, DirID: p.DirID, Title: p.Title, Slug: p.Slug,
		Content: p.Content, Summary: p.Summary, Cover: p.Cover, Gallery: p.Gallery, Tags: tagNames(p.Tags),
		CreatedAt: pkg.NowStr(),
	}
	if _, err := a.Repo.InsertSnapshot(ctx, sn); err != nil {
		slog.Warn("文章快照保存失败", "id", p.ID, "err", err)
	}
	if err := a.Repo.TrimSnapshots(ctx, "post", p.ID, a.snapshotLimit(ctx)); err != nil {
		slog.Warn("文章快照裁剪失败", "id", p.ID, "err", err)
	}
}

// snapshotPage 保存页面旧版本快照并裁剪
func (a *App) snapshotPage(ctx context.Context, p *model.Page) {
	if p.ID == 0 {
		return
	}
	sn := &model.Snapshot{
		TargetType: "page", TargetID: p.ID, DirID: p.DirID, Title: p.Title, Slug: p.Slug,
		Content: p.Content, Summary: p.Summary, Cover: p.Cover, Gallery: p.Gallery, Tags: tagNames(p.Tags),
		CreatedAt: pkg.NowStr(),
	}
	if _, err := a.Repo.InsertSnapshot(ctx, sn); err != nil {
		slog.Warn("页面快照保存失败", "id", p.ID, "err", err)
	}
	if err := a.Repo.TrimSnapshots(ctx, "page", p.ID, a.snapshotLimit(ctx)); err != nil {
		slog.Warn("页面快照裁剪失败", "id", p.ID, "err", err)
	}
}
