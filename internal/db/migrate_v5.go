package db

import (
	"context"
	"database/sql"
	"log/slog"
)

// migrateV4ToV5 为 posts/pages/snapshots 新增 gallery 图集字段（JSON 数组）
func migrateV4ToV5(ctx context.Context, conn *sql.DB) error {
	stmts := []string{
		`ALTER TABLE posts ADD COLUMN gallery TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE pages ADD COLUMN gallery TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE snapshots ADD COLUMN gallery TEXT NOT NULL DEFAULT '[]'`,
		`PRAGMA user_version = 5`,
	}
	for _, s := range stmts {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	slog.Info("数据库已升级到 v5（新增 gallery 图集字段）")
	return nil
}
