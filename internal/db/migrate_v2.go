package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"marksite/internal/pkg"
)

// migrateV1ToV2 将 user_version=1 的数据库升级到 2
func migrateV1ToV2(ctx context.Context, conn *sql.DB) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 1) directories 加列 + pages 补 tags 列（v1 pages 表原本没有 tags）
	alterSQL := []string{
		"ALTER TABLE directories ADD COLUMN slug TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE directories ADD COLUMN page_title TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE directories ADD COLUMN page_content TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE directories ADD COLUMN page_summary TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE directories ADD COLUMN cover TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE directories ADD COLUMN is_page INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE directories ADD COLUMN is_published INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE directories ADD COLUMN menu_visible INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE pages ADD COLUMN tags TEXT NOT NULL DEFAULT '[]'",
	}
	for _, s := range alterSQL {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("ALTER 失败: %w (SQL=%s)", err, s)
		}
	}

	// 2) 先回填分类 slug（在 UNIQUE INDEX 之前）
	dirs, err := fetchDirsForMigrate(ctx, tx)
	if err != nil {
		return err
	}
	used := map[string]bool{}
	for _, d := range dirs {
		if d.Slug != "" {
			used[d.Slug] = true
		}
	}
	for _, d := range dirs {
		if d.Slug != "" {
			continue
		}
		slug := deriveSlug(d.Name, used)
		if _, err := tx.ExecContext(ctx, `UPDATE directories SET slug = ? WHERE id = ?`, slug, d.ID); err != nil {
			return fmt.Errorf("分类 slug 回填失败: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS uk_directories_slug ON directories(slug)`); err != nil {
		return fmt.Errorf("uk_directories_slug 失败: %w", err)
	}

	// 3) 新增 tags / posts_tags / pages_tags
	newTables := []string{
		`CREATE TABLE IF NOT EXISTS tags (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL UNIQUE,
			slug       TEXT NOT NULL UNIQUE,
			sort       INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS posts_tags (
			post_id INTEGER NOT NULL,
			tag_id  INTEGER NOT NULL,
			PRIMARY KEY(post_id, tag_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_posts_tags_tag ON posts_tags(tag_id)`,
		`CREATE TABLE IF NOT EXISTS pages_tags (
			page_id INTEGER NOT NULL,
			tag_id  INTEGER NOT NULL,
			PRIMARY KEY(page_id, tag_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pages_tags_tag ON pages_tags(tag_id)`,
	}
	for _, s := range newTables {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("新表创建失败: %w", err)
		}
	}

	// 4) 从 posts.tags / pages.tags JSON 展开入 tags 表 + 关联表
	if err := migrateJSONTags(ctx, tx, "posts", "posts_tags"); err != nil {
		return err
	}
	if err := migrateJSONTags(ctx, tx, "pages", "pages_tags"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 2`); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	slog.Info("数据库已升级到 v2（分类/标签）")
	return nil
}

type migrateDirRow struct {
	ID   int64
	Name string
	Slug string
}

func fetchDirsForMigrate(ctx context.Context, tx *sql.Tx) ([]migrateDirRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, name, slug FROM directories`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []migrateDirRow
	for rows.Next() {
		var r migrateDirRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Slug); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// deriveSlug 从 name 派生一个合法 slug（中文转拼音），冲突时自动加 -1/-2 后缀
func deriveSlug(name string, used map[string]bool) string {
	if used == nil {
		used = map[string]bool{}
	}
	base := pkg.Slugify(name, "untitled")
	candidate := base
	for i := 1; used[candidate]; i++ {
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	used[candidate] = true
	return candidate
}

// migrateJSONTags 读取 contentTable 的 tags JSON，去重后写入 tags 表与 joinTable
func migrateJSONTags(ctx context.Context, tx *sql.Tx, contentTable, joinTable string) error {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`SELECT id, tags FROM %s WHERE tags != '[]' AND tags != ''`, contentTable))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	now := pkg.NowStr()
	tagNameToID := map[string]int64{}
	ensureTag := func(name string) (int64, error) {
		if id, ok := tagNameToID[name]; ok {
			return id, nil
		}
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = ?`, name).Scan(&id)
		if err == nil {
			tagNameToID[name] = id
			return id, nil
		}
		slug := deriveSlug(name, nil)
		for {
			var existing int64
			if err2 := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE slug = ?`, slug).Scan(&existing); err2 != nil {
				break
			}
			// 冲突，换一个
			var i int
			_, _ = fmt.Sscanf(slug, "%s-%d", new(string), &i)
			if i == 0 {
				i = 1
			}
			slug = fmt.Sprintf("%s-%d", slug, i+1)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO tags (name, slug, sort, created_at) VALUES (?, ?, 0, ?)`, name, slug, now)
		if err != nil {
			return 0, err
		}
		id, _ = res.LastInsertId()
		tagNameToID[name] = id
		return id, nil
	}

	var linkRows []string
	for rows.Next() {
		var contentID int64
		var raw string
		if err := rows.Scan(&contentID, &raw); err != nil {
			return err
		}
		var names []string
		if err := json.Unmarshal([]byte(raw), &names); err != nil {
			continue
		}
		seen := map[int64]bool{}
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			id, err := ensureTag(n)
			if err != nil {
				return err
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			linkRows = append(linkRows, fmt.Sprintf("(%d,%d)", contentID, id))
		}
	}
	if len(linkRows) == 0 {
		return rows.Err()
	}
	// 批量 INSERT
	contentCol := contentTable
	var pkCol string
	switch contentCol {
	case "posts":
		pkCol = "post_id"
	case "pages":
		pkCol = "page_id"
	default:
		return fmt.Errorf("未知表 %s", contentCol)
	}
	stmt := fmt.Sprintf(`INSERT OR IGNORE INTO %s (%s, tag_id) VALUES `, joinTable, pkCol)
	_, err = tx.ExecContext(ctx, stmt+strings.Join(linkRows, ","))
	if err != nil {
		return err
	}
	return rows.Err()
}
