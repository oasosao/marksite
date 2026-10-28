package db

import (
	"context"
	"database/sql"
	"log/slog"
)

// migrateV2ToV3 默认关闭"启动自动打开浏览器"
func migrateV2ToV3(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, `UPDATE app_config SET auto_open_browser = 0`); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA user_version = 3`); err != nil {
		return err
	}
	slog.Info("数据库已升级到 v3（默认不自动打开浏览器）")
	return nil
}
