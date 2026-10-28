-- MarkSite 数据库结构（user_version = 6）
-- v1 首次建表；v2：directories 升级为分类/独立页面，新增 tags + posts_tags + pages_tags
-- v4：posts/snapshots 移除 category 自由文本字段（分类统一由 directories 承担）
-- v5：posts/pages/snapshots 新增 gallery 图集字段（JSON 数组）
-- v6：移除未使用的托盘开关 tray_enabled
CREATE TABLE IF NOT EXISTS app_config (
  id                         INTEGER PRIMARY KEY CHECK (id = 1),
  listen_addr                TEXT    NOT NULL DEFAULT '127.0.0.1:8080',
  log_level                  TEXT    NOT NULL DEFAULT 'info',
  snapshot_limit             INTEGER NOT NULL DEFAULT 20,
  auto_backup_enabled        INTEGER NOT NULL DEFAULT 0,
  auto_backup_interval_hours INTEGER NOT NULL DEFAULT 24,
  backup_keep_count          INTEGER NOT NULL DEFAULT 7,
  auto_open_browser          INTEGER NOT NULL DEFAULT 0,
  output_dir                 TEXT    NOT NULL DEFAULT '',
  created_at                 TEXT    NOT NULL,
  updated_at                 TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS site_config (
  id              INTEGER PRIMARY KEY CHECK (id = 1),
  site_name       TEXT    NOT NULL DEFAULT 'MarkSite',
  site_desc       TEXT    NOT NULL DEFAULT '',
  site_url        TEXT    NOT NULL DEFAULT 'http://localhost:8080',
  logo            TEXT    NOT NULL DEFAULT '',
  favicon         TEXT    NOT NULL DEFAULT '',
  copyright       TEXT    NOT NULL DEFAULT '',
  rss_enabled     INTEGER NOT NULL DEFAULT 1,
  sitemap_enabled INTEGER NOT NULL DEFAULT 1,
  robots          TEXT    NOT NULL DEFAULT '',
  custom_head     TEXT    NOT NULL DEFAULT '',
  custom_footer   TEXT    NOT NULL DEFAULT '',
  page_size       INTEGER NOT NULL DEFAULT 10,
  path_mode       TEXT    NOT NULL DEFAULT 'fixed',
  theme           TEXT    NOT NULL DEFAULT 'default',
  created_at      TEXT    NOT NULL,
  updated_at      TEXT    NOT NULL
);

-- v2: directories 升级为分类（可独立页面）
CREATE TABLE IF NOT EXISTS directories (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  parent_id     INTEGER NOT NULL DEFAULT 0,
  name          TEXT    NOT NULL,
  slug          TEXT    NOT NULL DEFAULT '',
  page_title    TEXT    NOT NULL DEFAULT '',
  page_content  TEXT    NOT NULL DEFAULT '',
  page_summary  TEXT    NOT NULL DEFAULT '',
  cover         TEXT    NOT NULL DEFAULT '',
  is_page       INTEGER NOT NULL DEFAULT 0,
  is_published  INTEGER NOT NULL DEFAULT 1,
  menu_visible  INTEGER NOT NULL DEFAULT 1,
  sort          INTEGER NOT NULL DEFAULT 0,
  created_at    TEXT    NOT NULL,
  updated_at    TEXT    NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_directories_parent_name ON directories(parent_id, name);
CREATE UNIQUE INDEX IF NOT EXISTS uk_directories_slug ON directories(slug);
CREATE INDEX IF NOT EXISTS idx_directories_parent ON directories(parent_id);

CREATE TABLE IF NOT EXISTS posts (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  dir_id       INTEGER NOT NULL DEFAULT 0,
  title        TEXT    NOT NULL,
  slug         TEXT    NOT NULL,
  content      TEXT    NOT NULL DEFAULT '',
  summary      TEXT    NOT NULL DEFAULT '',
  cover        TEXT    NOT NULL DEFAULT '',
  gallery      TEXT    NOT NULL DEFAULT '[]',
  tags         TEXT    NOT NULL DEFAULT '[]',
  sort         INTEGER NOT NULL DEFAULT 0,
  is_published INTEGER NOT NULL DEFAULT 0,
  published_at TEXT    NOT NULL DEFAULT '',
  created_at   TEXT    NOT NULL,
  updated_at   TEXT    NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_posts_slug ON posts(slug);
CREATE INDEX IF NOT EXISTS idx_posts_dir ON posts(dir_id);
CREATE INDEX IF NOT EXISTS idx_posts_published ON posts(is_published, published_at);

-- v2: pages 补 tags 列（与 posts 对齐）
CREATE TABLE IF NOT EXISTS pages (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  dir_id       INTEGER NOT NULL DEFAULT 0,
  title        TEXT    NOT NULL,
  slug         TEXT    NOT NULL,
  content      TEXT    NOT NULL DEFAULT '',
  summary      TEXT    NOT NULL DEFAULT '',
  cover        TEXT    NOT NULL DEFAULT '',
  gallery      TEXT    NOT NULL DEFAULT '[]',
  tags         TEXT    NOT NULL DEFAULT '[]',
  sort         INTEGER NOT NULL DEFAULT 0,
  is_published INTEGER NOT NULL DEFAULT 0,
  published_at TEXT    NOT NULL DEFAULT '',
  created_at   TEXT    NOT NULL,
  updated_at   TEXT    NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_pages_slug ON pages(slug);
CREATE INDEX IF NOT EXISTS idx_pages_dir ON pages(dir_id);

CREATE TABLE IF NOT EXISTS snapshots (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  target_type TEXT    NOT NULL,
  target_id   INTEGER NOT NULL,
  dir_id      INTEGER NOT NULL DEFAULT 0,
  title       TEXT    NOT NULL,
  slug        TEXT    NOT NULL,
  content     TEXT    NOT NULL,
  summary     TEXT    NOT NULL DEFAULT '',
  cover       TEXT    NOT NULL DEFAULT '',
  gallery     TEXT    NOT NULL DEFAULT '[]',
  tags        TEXT    NOT NULL DEFAULT '[]',
  created_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_snapshots_target ON snapshots(target_type, target_id, id);

CREATE TABLE IF NOT EXISTS assets (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  path       TEXT    NOT NULL UNIQUE,
  filename   TEXT    NOT NULL,
  size       INTEGER NOT NULL,
  mime       TEXT    NOT NULL,
  ref_count  INTEGER NOT NULL DEFAULT 0,
  created_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_assets_created ON assets(created_at);

CREATE TABLE IF NOT EXISTS generate_tasks (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  type        TEXT    NOT NULL,
  status      TEXT    NOT NULL,
  progress    INTEGER NOT NULL DEFAULT 0,
  total       INTEGER NOT NULL DEFAULT 0,
  done        INTEGER NOT NULL DEFAULT 0,
  log         TEXT    NOT NULL DEFAULT '',
  error       TEXT    NOT NULL DEFAULT '',
  created_at  TEXT    NOT NULL,
  started_at  TEXT    NOT NULL DEFAULT '',
  finished_at TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_tasks_created ON generate_tasks(created_at);

-- v2 新增：标签主表
CREATE TABLE IF NOT EXISTS tags (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL UNIQUE,
  slug       TEXT NOT NULL UNIQUE,
  sort       INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);

-- v2 新增：文章-标签多对多
CREATE TABLE IF NOT EXISTS posts_tags (
  post_id INTEGER NOT NULL,
  tag_id  INTEGER NOT NULL,
  PRIMARY KEY(post_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_posts_tags_tag ON posts_tags(tag_id);

-- v2 新增：页面-标签多对多
CREATE TABLE IF NOT EXISTS pages_tags (
  page_id INTEGER NOT NULL,
  tag_id  INTEGER NOT NULL,
  PRIMARY KEY(page_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_pages_tags_tag ON pages_tags(tag_id);
