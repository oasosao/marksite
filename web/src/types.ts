export interface TreeNode {
  key: string
  type: 'dir' | 'post' | 'page'
  id: number
  parent_id: number
  label: string
  slug?: string
  sort: number
  is_published?: number
  is_page?: number
  menu_visible?: number
  children: TreeNode[]
}

export interface Tag {
  id: number
  name: string
  slug: string
  sort: number
  created_at: string
}

export interface TagCount extends Tag {
  use_count: number
}

export interface Directory {
  id: number
  parent_id: number
  name: string
  slug: string
  page_title: string
  page_content: string
  page_summary: string
  cover: string
  is_page: number
  is_published: number
  menu_visible: number
  sort: number
  created_at: string
  updated_at: string
}

export interface Post {
  id: number
  dir_id: number
  dir_name?: string   // 所属目录名（后端 service 层填充）
  dir_path?: string   // 完整层级路径 "技术博客 / Go 语言"
  title: string
  slug: string
  content: string
  summary: string
  cover: string
  gallery: string[]
  tags: Tag[]
  sort: number
  is_published: number
  published_at: string
  created_at: string
  updated_at: string
}

export interface Page {
  id: number
  dir_id: number
  dir_name?: string
  dir_path?: string
  title: string
  slug: string
  content: string
  summary: string
  cover: string
  gallery: string[]
  tags: Tag[]
  sort: number
  is_published: number
  published_at: string
  created_at: string
  updated_at: string
}

export interface Paged<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export interface Asset {
  id: number
  path: string
  url: string
  filename: string
  size: number
  mime: string
  ref_count: number
  created_at: string
}

export interface SnapshotMeta {
  id: number
  title: string
  created_at: string
}

export interface ThemeInfo {
  name: string
  version: string
  author: string
  description: string
  builtin: boolean
  active: boolean
  valid: boolean
}

export interface Task {
  id: number
  type: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'canceled'
  progress: number
  total: number
  done: number
  log: string
  error: string
  created_at: string
  started_at: string
  finished_at: string
}

export interface SiteConfig {
  id: number
  site_name: string
  site_desc: string
  site_url: string
  logo: string
  favicon: string
  copyright: string
  page_size: number
  path_mode: string
  theme: string
  rss_enabled: number
  sitemap_enabled: number
  robots: string
  custom_head: string
  custom_footer: string
  updated_at: string
}

export interface AppConfig {
  id: number
  listen_addr: string
  log_level: 'debug' | 'info' | 'warn' | 'error'
  output_dir: string
  auto_open_browser: number
  snapshot_limit: number
  auto_backup_enabled: number
  auto_backup_interval_hours: number
  backup_keep_count: number
  updated_at: string
}

export interface Stats {
  posts: { total: number; published: number }
  pages: { total: number; published: number }
  dirs: number
  tags: number
  assets: { count: number; size: number }
  fts: { docs: number; exists: boolean }
  backups: { count: number }
  db_size: number
  last_generate: Task | null
  recent_tasks: Task[]
}

export interface SearchHit {
  type: string
  id: number
  title: string
  slug: string
  published: number
  snippet: string
  updated_at: string
}

export interface SearchResult {
  list: SearchHit[]
  total: number
  took_ms: number
  query: string
}

export interface BackupInfo {
  name: string
  size: number
  created_at: string
  auto: boolean
}
