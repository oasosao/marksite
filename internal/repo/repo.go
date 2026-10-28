package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

var ErrNotFound = errors.New("记录不存在")

type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Repo 数据访问层，所有 SQL 集中于此
type Repo struct {
	db *sql.DB
	q  querier
}

func New(db *sql.DB) *Repo {
	return &Repo{db: db, q: db}
}

func NewTx(tx *sql.Tx) *Repo {
	return &Repo{q: tx}
}

// WithTx 在事务内执行
func (r *Repo) WithTx(ctx context.Context, fn func(*Repo) error) error {
	if r.db == nil {
		return fn(r)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(NewTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func tagsToJSON(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func tagsFromJSON(s string) []string {
	out := []string{}
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}
