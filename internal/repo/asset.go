package repo

import (
	"context"
	"database/sql"
	"errors"

	"marksite/internal/model"
)

const assetCols = `id, path, filename, size, mime, ref_count, created_at`

func scanAsset(s rowScanner) (model.Asset, error) {
	var a model.Asset
	err := s.Scan(&a.ID, &a.Path, &a.Filename, &a.Size, &a.Mime, &a.RefCount, &a.CreatedAt)
	return a, err
}

func (r *Repo) InsertAsset(ctx context.Context, a *model.Asset) (int64, error) {
	res, err := r.q.ExecContext(ctx,
		`INSERT INTO assets (path, filename, size, mime, ref_count, created_at) VALUES (?, ?, ?, ?, 0, ?)`,
		a.Path, a.Filename, a.Size, a.Mime, a.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repo) ListAssets(ctx context.Context, keyword string, page, size int) ([]model.Asset, int64, error) {
	where := "1=1"
	args := []any{}
	if keyword != "" {
		where = "(filename LIKE ? OR path LIKE ?)"
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}
	var total int64
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(1) FROM assets WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := r.q.QueryContext(ctx,
		`SELECT `+assetCols+` FROM assets WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Asset{}
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (r *Repo) ListAllAssets(ctx context.Context) ([]model.Asset, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+assetCols+` FROM assets ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Asset{}
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repo) GetAsset(ctx context.Context, id int64) (*model.Asset, error) {
	a, err := scanAsset(r.q.QueryRowContext(ctx, `SELECT `+assetCols+` FROM assets WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repo) DeleteAsset(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM assets WHERE id = ?`, id)
	return err
}

func (r *Repo) UpdateAssetRef(ctx context.Context, id int64, ref int) error {
	_, err := r.q.ExecContext(ctx, `UPDATE assets SET ref_count = ? WHERE id = ?`, ref, id)
	return err
}

func (r *Repo) AssetTotals(ctx context.Context) (count, size int64, err error) {
	err = r.q.QueryRowContext(ctx, `SELECT COUNT(1), COALESCE(SUM(size),0) FROM assets`).Scan(&count, &size)
	return
}
