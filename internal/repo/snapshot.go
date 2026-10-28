package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"marksite/internal/model"
)

const snapshotCols = `id, target_type, target_id, dir_id, title, slug, content, summary, cover, gallery, tags, created_at`

func scanSnapshot(s rowScanner) (model.Snapshot, error) {
	var sn model.Snapshot
	var tags, gallery string
	err := s.Scan(&sn.ID, &sn.TargetType, &sn.TargetID, &sn.DirID, &sn.Title, &sn.Slug, &sn.Content,
		&sn.Summary, &sn.Cover, &gallery, &tags, &sn.CreatedAt)
	if err != nil {
		return sn, err
	}
	sn.Tags = tagsFromJSON(tags)
	sn.Gallery = tagsFromJSON(gallery)
	return sn, nil
}

func (r *Repo) InsertSnapshot(ctx context.Context, s *model.Snapshot) (int64, error) {
	res, err := r.q.ExecContext(ctx, `INSERT INTO snapshots
		(target_type, target_id, dir_id, title, slug, content, summary, cover, gallery, tags, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.TargetType, s.TargetID, s.DirID, s.Title, s.Slug, s.Content, s.Summary, s.Cover,
		tagsToJSON(s.Gallery), tagsToJSON(s.Tags), s.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repo) ListSnapshotMeta(ctx context.Context, targetType string, targetID int64) ([]model.SnapshotMeta, error) {
	rows, err := r.q.QueryContext(ctx,
		`SELECT id, title, created_at FROM snapshots WHERE target_type = ? AND target_id = ? ORDER BY id DESC`,
		targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.SnapshotMeta{}
	for rows.Next() {
		var m model.SnapshotMeta
		if err := rows.Scan(&m.ID, &m.Title, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repo) GetSnapshot(ctx context.Context, id int64) (*model.Snapshot, error) {
	sn, err := scanSnapshot(r.q.QueryRowContext(ctx, `SELECT `+snapshotCols+` FROM snapshots WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sn, nil
}

// TrimSnapshots 保留最新 limit 条快照
func (r *Repo) TrimSnapshots(ctx context.Context, targetType string, targetID int64, limit int) error {
	if limit < 1 {
		return nil
	}
	_, err := r.q.ExecContext(ctx, `DELETE FROM snapshots WHERE target_type = ? AND target_id = ? AND id NOT IN (
		SELECT id FROM snapshots WHERE target_type = ? AND target_id = ? ORDER BY id DESC LIMIT ?)`,
		targetType, targetID, targetType, targetID, limit)
	return err
}

func (r *Repo) DeleteSnapshotsByTargets(ctx context.Context, targetType string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, targetType)
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := r.q.ExecContext(ctx, `DELETE FROM snapshots WHERE target_type = ? AND target_id IN (`+ph+`)`, args...)
	return err
}

func (r *Repo) DeleteSnapshotsByTarget(ctx context.Context, targetType string, targetID int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM snapshots WHERE target_type = ? AND target_id = ?`, targetType, targetID)
	return err
}
