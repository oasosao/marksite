package repo

import (
	"context"
	"database/sql"
	"errors"

	"marksite/internal/model"
)

const taskCols = `id, type, status, progress, total, done, log, error, created_at, started_at, finished_at`

func scanTask(s rowScanner) (model.Task, error) {
	var t model.Task
	err := s.Scan(&t.ID, &t.Type, &t.Status, &t.Progress, &t.Total, &t.Done, &t.Log, &t.Error,
		&t.CreatedAt, &t.StartedAt, &t.FinishedAt)
	return t, err
}

func (r *Repo) CreateTask(ctx context.Context, taskType, now string) (int64, error) {
	res, err := r.q.ExecContext(ctx,
		`INSERT INTO generate_tasks (type, status, progress, total, done, log, error, created_at, started_at, finished_at)
		 VALUES (?, 'pending', 0, 0, 0, '', '', ?, '', '')`, taskType, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repo) StartTask(ctx context.Context, id int64, now string) error {
	_, err := r.q.ExecContext(ctx, `UPDATE generate_tasks SET status = 'running', started_at = ? WHERE id = ?`, now, id)
	return err
}

func (r *Repo) UpdateTaskProgress(ctx context.Context, id int64, done, total, progress int) error {
	_, err := r.q.ExecContext(ctx,
		`UPDATE generate_tasks SET done = ?, total = ?, progress = ? WHERE id = ?`, done, total, progress, id)
	return err
}

// AppendTaskLog 追加日志并保留最近 200KB
func (r *Repo) AppendTaskLog(ctx context.Context, id int64, line string) error {
	_, err := r.q.ExecContext(ctx,
		`UPDATE generate_tasks SET log = substr(log || ?, -200000) WHERE id = ?`, line, id)
	return err
}

func (r *Repo) FinishTask(ctx context.Context, id int64, status, errMsg, now string) error {
	_, err := r.q.ExecContext(ctx,
		`UPDATE generate_tasks SET status = ?, error = ?, finished_at = ?, progress = 100 WHERE id = ?`,
		status, errMsg, now, id)
	return err
}

func (r *Repo) GetTask(ctx context.Context, id int64) (*model.Task, error) {
	t, err := scanTask(r.q.QueryRowContext(ctx, `SELECT `+taskCols+` FROM generate_tasks WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repo) ListTasks(ctx context.Context, limit int) ([]model.Task, error) {
	rows, err := r.q.QueryContext(ctx,
		`SELECT `+taskCols+` FROM generate_tasks ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []model.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		// 列表只返回截断日志
		if len(t.Log) > 2048 {
			t.Log = t.Log[len(t.Log)-2048:]
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repo) LastSuccessfulGenerateStart(ctx context.Context) (string, error) {
	var v sql.NullString
	err := r.q.QueryRowContext(ctx,
		`SELECT started_at FROM generate_tasks WHERE status = 'success' AND type IN ('full','increment')
		 ORDER BY id DESC LIMIT 1`).Scan(&v)
	return v.String, err
}

func (r *Repo) LastSuccessfulTask(ctx context.Context) (*model.Task, error) {
	t, err := scanTask(r.q.QueryRowContext(ctx,
		`SELECT `+taskCols+` FROM generate_tasks WHERE status = 'success' AND type IN ('full','increment')
		 ORDER BY id DESC LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// AbortRunningTasks 将中断的任务标记为失败（程序重启时调用）
func (r *Repo) AbortRunningTasks(ctx context.Context, errMsg, now string) error {
	_, err := r.q.ExecContext(ctx,
		`UPDATE generate_tasks SET status = 'failed', error = ?, finished_at = ? WHERE status IN ('pending','running')`,
		errMsg, now)
	return err
}

func (r *Repo) HasRunningTask(ctx context.Context) (bool, error) {
	var count int
	err := r.q.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM generate_tasks WHERE status IN ('pending','running')`).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
