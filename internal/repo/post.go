package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"marksite/internal/model"
)

const postCols = `id, dir_id, title, slug, content, summary, cover, gallery, sort,
 is_published, published_at, created_at, updated_at`

func scanPost(s rowScanner) (model.Post, error) {
	var p model.Post
	var gallery string
	err := s.Scan(&p.ID, &p.DirID, &p.Title, &p.Slug, &p.Content, &p.Summary, &p.Cover, &gallery,
		&p.Sort, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	p.Gallery = tagsFromJSON(gallery)
	return p, nil
}

// PostFilter 文章列表筛选条件
type PostFilter struct {
	Keyword   string
	DirID     *int64  // 单目录精确匹配
	DirIDs    []int64 // 多目录 IN 查询（优先于 DirID）
	Published *int
	Page      int
	Size      int
}

func (r *Repo) ListPosts(ctx context.Context, f PostFilter) ([]model.Post, int64, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.Keyword != "" {
		where = append(where, "(title LIKE ? OR slug LIKE ?)")
		kw := "%" + f.Keyword + "%"
		args = append(args, kw, kw)
	}
	if len(f.DirIDs) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(f.DirIDs)), ",")
		where = append(where, "dir_id IN ("+ph+")")
		for _, id := range f.DirIDs {
			args = append(args, id)
		}
	} else if f.DirID != nil {
		where = append(where, "dir_id = ?")
		args = append(args, *f.DirID)
	}
	if f.Published != nil {
		where = append(where, "is_published = ?")
		args = append(args, *f.Published)
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM posts WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), f.Size, (f.Page-1)*f.Size)
	rows, err := r.q.QueryContext(ctx,
		`SELECT `+postCols+` FROM posts WHERE `+cond+` ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *Repo) ListAllPosts(ctx context.Context) ([]model.Post, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+postCols+` FROM posts ORDER BY published_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) ListPostsByDirIDs(ctx context.Context, dirIDs []int64) ([]model.Post, error) {
	if len(dirIDs) == 0 {
		return []model.Post{}, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(dirIDs)), ",")
	args := make([]any, len(dirIDs))
	for i, id := range dirIDs {
		args[i] = id
	}
	rows, err := r.q.QueryContext(ctx, `SELECT `+postCols+` FROM posts WHERE dir_id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) GetPost(ctx context.Context, id int64) (*model.Post, error) {
	p, err := scanPost(r.q.QueryRowContext(ctx, `SELECT `+postCols+` FROM posts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repo) CreatePost(ctx context.Context, p *model.Post) (int64, error) {
	res, err := r.q.ExecContext(ctx, `INSERT INTO posts
		(dir_id, title, slug, content, summary, cover, gallery, sort, is_published, published_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.DirID, p.Title, p.Slug, p.Content, p.Summary, p.Cover, tagsToJSON(p.Gallery),
		p.Sort, p.IsPublished, p.PublishedAt, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repo) UpdatePost(ctx context.Context, p *model.Post) error {
	_, err := r.q.ExecContext(ctx, `UPDATE posts SET
		dir_id = ?, title = ?, slug = ?, content = ?, summary = ?, cover = ?, gallery = ?,
		sort = ?, is_published = ?, published_at = ?, updated_at = ? WHERE id = ?`,
		p.DirID, p.Title, p.Slug, p.Content, p.Summary, p.Cover, tagsToJSON(p.Gallery),
		p.Sort, p.IsPublished, p.PublishedAt, p.UpdatedAt, p.ID)
	return err
}

func (r *Repo) DeletePost(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM posts WHERE id = ?`, id)
	return err
}

func (r *Repo) DeletePostsByDirIDs(ctx context.Context, dirIDs []int64) error {
	if len(dirIDs) == 0 {
		return nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(dirIDs)), ",")
	args := make([]any, len(dirIDs))
	for i, id := range dirIDs {
		args[i] = id
	}
	_, err := r.q.ExecContext(ctx, `DELETE FROM posts WHERE dir_id IN (`+ph+`)`, args...)
	return err
}

func (r *Repo) MovePost(ctx context.Context, id, dirID int64, now string) error {
	_, err := r.q.ExecContext(ctx, `UPDATE posts SET dir_id = ?, updated_at = ? WHERE id = ?`, dirID, now, id)
	return err
}

func (r *Repo) PostSlugExists(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var count int
	err := r.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM posts WHERE slug = ? AND id <> ?`, slug, excludeID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repo) CountPosts(ctx context.Context) (total, published int64, err error) {
	err = r.q.QueryRowContext(ctx, `SELECT COUNT(1), COALESCE(SUM(is_published), 0) FROM posts`).Scan(&total, &published)
	return
}
