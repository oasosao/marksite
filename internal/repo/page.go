package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"marksite/internal/model"
)

const pageCols = `id, dir_id, title, slug, content, summary, cover, gallery, sort, is_published, published_at, created_at, updated_at`

func scanPage(s rowScanner) (model.Page, error) {
	var p model.Page
	var gallery string
	err := s.Scan(&p.ID, &p.DirID, &p.Title, &p.Slug, &p.Content, &p.Summary, &p.Cover, &gallery,
		&p.Sort, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	p.Gallery = tagsFromJSON(gallery)
	return p, nil
}

// PageFilter 页面列表筛选条件
type PageFilter struct {
	Keyword   string
	DirID     *int64
	DirIDs    []int64
	Published *int
	Page      int
	Size      int
}

func (r *Repo) ListPages(ctx context.Context, f PageFilter) ([]model.Page, int64, error) {
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
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM pages WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), f.Size, (f.Page-1)*f.Size)
	rows, err := r.q.QueryContext(ctx,
		`SELECT `+pageCols+` FROM pages WHERE `+cond+` ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Page{}
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *Repo) ListAllPages(ctx context.Context) ([]model.Page, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+pageCols+` FROM pages ORDER BY published_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Page{}
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) ListPagesByDirIDs(ctx context.Context, dirIDs []int64) ([]model.Page, error) {
	if len(dirIDs) == 0 {
		return []model.Page{}, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(dirIDs)), ",")
	args := make([]any, len(dirIDs))
	for i, id := range dirIDs {
		args[i] = id
	}
	rows, err := r.q.QueryContext(ctx, `SELECT `+pageCols+` FROM pages WHERE dir_id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Page{}
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) GetPage(ctx context.Context, id int64) (*model.Page, error) {
	p, err := scanPage(r.q.QueryRowContext(ctx, `SELECT `+pageCols+` FROM pages WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repo) CreatePage(ctx context.Context, p *model.Page) (int64, error) {
	res, err := r.q.ExecContext(ctx, `INSERT INTO pages
		(dir_id, title, slug, content, summary, cover, gallery, sort, is_published, published_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.DirID, p.Title, p.Slug, p.Content, p.Summary, p.Cover, tagsToJSON(p.Gallery), p.Sort, p.IsPublished, p.PublishedAt,
		p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repo) UpdatePage(ctx context.Context, p *model.Page) error {
	_, err := r.q.ExecContext(ctx, `UPDATE pages SET
		dir_id = ?, title = ?, slug = ?, content = ?, summary = ?, cover = ?, gallery = ?, sort = ?,
		is_published = ?, published_at = ?, updated_at = ? WHERE id = ?`,
		p.DirID, p.Title, p.Slug, p.Content, p.Summary, p.Cover, tagsToJSON(p.Gallery), p.Sort,
		p.IsPublished, p.PublishedAt, p.UpdatedAt, p.ID)
	return err
}

func (r *Repo) DeletePage(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM pages WHERE id = ?`, id)
	return err
}

func (r *Repo) DeletePagesByDirIDs(ctx context.Context, dirIDs []int64) error {
	if len(dirIDs) == 0 {
		return nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(dirIDs)), ",")
	args := make([]any, len(dirIDs))
	for i, id := range dirIDs {
		args[i] = id
	}
	_, err := r.q.ExecContext(ctx, `DELETE FROM pages WHERE dir_id IN (`+ph+`)`, args...)
	return err
}

func (r *Repo) MovePage(ctx context.Context, id, dirID int64, now string) error {
	_, err := r.q.ExecContext(ctx, `UPDATE pages SET dir_id = ?, updated_at = ? WHERE id = ?`, dirID, now, id)
	return err
}

func (r *Repo) PageSlugExists(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var count int
	err := r.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM pages WHERE slug = ? AND id <> ?`, slug, excludeID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repo) CountPages(ctx context.Context) (total, published int64, err error) {
	err = r.q.QueryRowContext(ctx, `SELECT COUNT(1), COALESCE(SUM(is_published), 0) FROM pages`).Scan(&total, &published)
	return
}
