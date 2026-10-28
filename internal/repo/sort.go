package repo

import "context"

func (r *Repo) UpdateDirSort(ctx context.Context, id int64, sort int, now string) error {
	_, err := r.q.ExecContext(ctx, `UPDATE directories SET sort = ?, updated_at = ? WHERE id = ?`, sort, now, id)
	return err
}

func (r *Repo) UpdatePostSort(ctx context.Context, id int64, sort int, now string) error {
	_, err := r.q.ExecContext(ctx, `UPDATE posts SET sort = ?, updated_at = ? WHERE id = ?`, sort, now, id)
	return err
}

func (r *Repo) UpdatePageSort(ctx context.Context, id int64, sort int, now string) error {
	_, err := r.q.ExecContext(ctx, `UPDATE pages SET sort = ?, updated_at = ? WHERE id = ?`, sort, now, id)
	return err
}
