package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"marksite/internal/pkg"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Open 打开 SQLite 连接并完成迁移与种子数据
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := migrate(context.Background(), conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := seed(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// migrate 基于 user_version 执行建表 + 增量升级
func migrate(ctx context.Context, conn *sql.DB) error {
	var version int
	if err := conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version == 0 {
		if _, err := conn.ExecContext(ctx, schemaSQL); err != nil {
			return fmt.Errorf("执行建表脚本失败: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `PRAGMA user_version = 6`); err != nil {
			return err
		}
		slog.Info("数据库结构已初始化", "version", 6)
		return nil
	}
	if version < 2 {
		slog.Info("升级数据库 schema", "from", version, "to", 2)
		if err := migrateV1ToV2(ctx, conn); err != nil {
			return fmt.Errorf("升级到 v2 失败: %w", err)
		}
	}
	if version < 3 {
		if err := migrateV2ToV3(ctx, conn); err != nil {
			return fmt.Errorf("升级到 v3 失败: %w", err)
		}
	}
	if version < 4 {
		if err := migrateV3ToV4(ctx, conn); err != nil {
			return fmt.Errorf("升级到 v4 失败: %w", err)
		}
	}
	if version < 5 {
		if err := migrateV4ToV5(ctx, conn); err != nil {
			return fmt.Errorf("升级到 v5 失败: %w", err)
		}
	}
	if version < 6 {
		if err := migrateV5ToV6(ctx, conn); err != nil {
			return fmt.Errorf("升级到 v6 失败: %w", err)
		}
	}
	return nil
}

// seed 首次启动初始化配置
func seed(conn *sql.DB) error {
	now := pkg.NowStr()
	if _, err := conn.Exec(`INSERT OR IGNORE INTO app_config (id, created_at, updated_at) VALUES (1, ?, ?)`, now, now); err != nil {
		return err
	}
	robots := "User-agent: *\nAllow: /"
	if _, err := conn.Exec(`INSERT OR IGNORE INTO site_config
		(id, site_name, site_desc, site_url, robots, page_size, path_mode, theme, created_at, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"MarkSite", "我的静态站点", "http://localhost:8080", robots, 10, "fixed", "default", now, now); err != nil {
		return err
	}
	return nil
}

// EnsureDataDir 创建运行期目录
func EnsureDataDir(dirs ...string) error {
	for _, d := range dirs {
		if strings.TrimSpace(d) == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
