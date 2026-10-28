package db

import (
	"context"
	"database/sql"
	"log/slog"
)

// migrateV5ToV6 移除未使用的托盘开关字段
func migrateV5ToV6(ctx context.Context, conn *sql.DB) error {
	stmts := []string{
		`ALTER TABLE app_config DROP COLUMN tray_enabled`,
		`PRAGMA user_version = 6`,
	}
	for _, s := range stmts {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	slog.Info("数据库已升级到 v6（移除未使用的托盘开关）")
	return nil
}
