package db

import (
	"context"
	"database/sql"
	"log/slog"
)

// migrateV3ToV4 移除 posts/snapshots 的 category 自由文本字段（分类统一由 directories 承担）
func migrateV3ToV4(ctx context.Context, conn *sql.DB) error {
	stmts := []string{
		`ALTER TABLE posts DROP COLUMN category`,
		`ALTER TABLE snapshots DROP COLUMN category`,
		`PRAGMA user_version = 4`,
	}
	for _, s := range stmts {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	slog.Info("数据库已升级到 v4（移除文章展示分类字段）")
	return nil
}
