package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"marksite/internal/model"
)

// TagRepo 标签相关查询
type TagRepo struct{ q querier }

// NewTagRepo 从 Repo 派生（便于事务）
func (r *Repo) Tag() *TagRepo { return &TagRepo{q: r.q} }

func (t *TagRepo) Create(ctx context.Context, name, slug string, sort int, now string) (int64, error) {
	res, err := t.q.ExecContext(ctx, `INSERT INTO tags (name, slug, sort, created_at) VALUES (?, ?, ?, ?)`, name, slug, sort, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (t *TagRepo) GetByID(ctx context.Context, id int64) (*model.Tag, error) {
	var tag model.Tag
	err := t.q.QueryRowContext(ctx, `SELECT id, name, slug, sort, created_at FROM tags WHERE id = ?`, id).Scan(
		&tag.ID, &tag.Name, &tag.Slug, &tag.Sort, &tag.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &tag, err
}

func (t *TagRepo) GetByName(ctx context.Context, name string) (*model.Tag, error) {
	var tag model.Tag
	err := t.q.QueryRowContext(ctx, `SELECT id, name, slug, sort, created_at FROM tags WHERE name = ?`, name).Scan(
		&tag.ID, &tag.Name, &tag.Slug, &tag.Sort, &tag.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &tag, err
}

func (t *TagRepo) List(ctx context.Context) ([]model.Tag, error) {
	rows, err := t.q.QueryContext(ctx, `SELECT id, name, slug, sort, created_at FROM tags ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	// 显式初始化空切片，确保无数据时 JSON 输出 [] 而不是 null
	out := make([]model.Tag, 0)
	for rows.Next() {
		var tag model.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Sort, &tag.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

// ListWithCount 带引用计数
func (t *TagRepo) ListWithCount(ctx context.Context) ([]model.TagCount, error) {
	rows, err := t.q.QueryContext(ctx, `
		SELECT t.id, t.name, t.slug, t.sort, t.created_at,
		       (SELECT COUNT(1) FROM posts_tags pt WHERE pt.tag_id = t.id) +
		       (SELECT COUNT(1) FROM pages_tags pt2 WHERE pt2.tag_id = t.id) AS use_count
		FROM tags t ORDER BY t.sort, t.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]model.TagCount, 0)
	for rows.Next() {
		var c model.TagCount
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Sort, &c.CreatedAt, &c.UseCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (t *TagRepo) Update(ctx context.Context, id int64, name, slug *string, sort *int) error {
	sets := []string{}
	args := []any{}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if slug != nil {
		sets = append(sets, "slug = ?")
		args = append(args, *slug)
	}
	if sort != nil {
		sets = append(sets, "sort = ?")
		args = append(args, *sort)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := t.q.ExecContext(ctx, `UPDATE tags SET `+joinComma(sets)+` WHERE id = ?`, args...)
	return err
}

func (t *TagRepo) Delete(ctx context.Context, id int64) error {
	if _, err := t.q.ExecContext(ctx, `DELETE FROM posts_tags WHERE tag_id = ?`, id); err != nil {
		return err
	}
	if _, err := t.q.ExecContext(ctx, `DELETE FROM pages_tags WHERE tag_id = ?`, id); err != nil {
		return err
	}
	_, err := t.q.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, id)
	return err
}

func (t *TagRepo) Count(ctx context.Context) (int64, error) {
	var c int64
	err := t.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM tags`).Scan(&c)
	return c, err
}

// --- 关联表操作 ---

// SetPostTags 覆盖写入文章的标签关联（先删所有再批量插）
func (t *TagRepo) SetPostTags(ctx context.Context, postID int64, tagIDs []int64) error {
	if _, err := t.q.ExecContext(ctx, `DELETE FROM posts_tags WHERE post_id = ?`, postID); err != nil {
		return err
	}
	for _, tid := range tagIDs {
		if _, err := t.q.ExecContext(ctx, `INSERT OR IGNORE INTO posts_tags (post_id, tag_id) VALUES (?, ?)`, postID, tid); err != nil {
			return err
		}
	}
	return nil
}

// SetPageTags 覆盖写入页面的标签关联
func (t *TagRepo) SetPageTags(ctx context.Context, pageID int64, tagIDs []int64) error {
	if _, err := t.q.ExecContext(ctx, `DELETE FROM pages_tags WHERE page_id = ?`, pageID); err != nil {
		return err
	}
	for _, tid := range tagIDs {
		if _, err := t.q.ExecContext(ctx, `INSERT OR IGNORE INTO pages_tags (page_id, tag_id) VALUES (?, ?)`, pageID, tid); err != nil {
			return err
		}
	}
	return nil
}

// GetPostTags 文章标签
func (t *TagRepo) GetPostTags(ctx context.Context, postID int64) ([]model.Tag, error) {
	rows, err := t.q.QueryContext(ctx, `
		SELECT t.id, t.name, t.slug, t.sort, t.created_at
		FROM tags t JOIN posts_tags pt ON pt.tag_id = t.id
		WHERE pt.post_id = ? ORDER BY t.sort, t.id`, postID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	// 显式初始化空切片，确保无数据时 JSON 输出 [] 而不是 null
	out := make([]model.Tag, 0)
	for rows.Next() {
		var tag model.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Sort, &tag.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

// GetPageTags 页面标签
func (t *TagRepo) GetPageTags(ctx context.Context, pageID int64) ([]model.Tag, error) {
	rows, err := t.q.QueryContext(ctx, `
		SELECT t.id, t.name, t.slug, t.sort, t.created_at
		FROM tags t JOIN pages_tags pt ON pt.tag_id = t.id
		WHERE pt.page_id = ? ORDER BY t.sort, t.id`, pageID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	// 显式初始化空切片，确保无数据时 JSON 输出 [] 而不是 null
	out := make([]model.Tag, 0)
	for rows.Next() {
		var tag model.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Sort, &tag.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

// GetPostsTagsMap 批量查询多篇文章的标签，返回 post_id -> []Tag（避免 N+1）
func (t *TagRepo) GetPostsTagsMap(ctx context.Context, postIDs []int64) (map[int64][]model.Tag, error) {
	result := map[int64][]model.Tag{}
	if len(postIDs) == 0 {
		return result, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(postIDs)), ",")
	args := make([]any, len(postIDs))
	for i, id := range postIDs {
		args[i] = id
	}
	rows, err := t.q.QueryContext(ctx, `
		SELECT pt.post_id, t.id, t.name, t.slug, t.sort, t.created_at
		FROM tags t JOIN posts_tags pt ON pt.tag_id = t.id
		WHERE pt.post_id IN (`+ph+`) ORDER BY pt.post_id, t.sort, t.id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var postID int64
		var tag model.Tag
		if err := rows.Scan(&postID, &tag.ID, &tag.Name, &tag.Slug, &tag.Sort, &tag.CreatedAt); err != nil {
			return nil, err
		}
		result[postID] = append(result[postID], tag)
	}
	return result, rows.Err()
}

// GetPagesTagsMap 批量查询多个页面的标签，返回 page_id -> []Tag（避免 N+1）
func (t *TagRepo) GetPagesTagsMap(ctx context.Context, pageIDs []int64) (map[int64][]model.Tag, error) {
	result := map[int64][]model.Tag{}
	if len(pageIDs) == 0 {
		return result, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(pageIDs)), ",")
	args := make([]any, len(pageIDs))
	for i, id := range pageIDs {
		args[i] = id
	}
	rows, err := t.q.QueryContext(ctx, `
		SELECT pt.page_id, t.id, t.name, t.slug, t.sort, t.created_at
		FROM tags t JOIN pages_tags pt ON pt.tag_id = t.id
		WHERE pt.page_id IN (`+ph+`) ORDER BY pt.page_id, t.sort, t.id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var pageID int64
		var tag model.Tag
		if err := rows.Scan(&pageID, &tag.ID, &tag.Name, &tag.Slug, &tag.Sort, &tag.CreatedAt); err != nil {
			return nil, err
		}
		result[pageID] = append(result[pageID], tag)
	}
	return result, rows.Err()
}

// SlugExists 检查 slug 是否已被其他标签使用
func (t *TagRepo) SlugExists(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var count int
	err := t.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM tags WHERE slug = ? AND id <> ?`, slug, excludeID).Scan(&count)
	return count > 0, err
}

// NameExists 检查 name 是否已存在
func (t *TagRepo) NameExists(ctx context.Context, name string, excludeID int64) (bool, error) {
	var count int
	err := t.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM tags WHERE name = ? AND id <> ?`, name, excludeID).Scan(&count)
	return count > 0, err
}
