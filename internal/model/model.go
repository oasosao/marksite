package model

// AppConfig 程序配置（单行）
type AppConfig struct {
	ID                      int64  `json:"-"`
	ListenAddr              string `json:"listen_addr"`
	LogLevel                string `json:"log_level"`
	SnapshotLimit           int    `json:"snapshot_limit"`
	AutoBackupEnabled       int    `json:"auto_backup_enabled"`
	AutoBackupIntervalHours int    `json:"auto_backup_interval_hours"`
	BackupKeepCount         int    `json:"backup_keep_count"`
	AutoOpenBrowser         int    `json:"auto_open_browser"`
	OutputDir               string `json:"output_dir"`
	CreatedAt               string `json:"created_at"`
	UpdatedAt               string `json:"updated_at"`
}

// SiteConfig 站点生成配置（单行）
type SiteConfig struct {
	ID             int64  `json:"-"`
	SiteName       string `json:"site_name"`
	SiteDesc       string `json:"site_desc"`
	SiteURL        string `json:"site_url"`
	Logo           string `json:"logo"`
	Favicon        string `json:"favicon"`
	Copyright      string `json:"copyright"`
	RSSEnabled     int    `json:"rss_enabled"`
	SitemapEnabled int    `json:"sitemap_enabled"`
	Robots         string `json:"robots"`
	CustomHead     string `json:"custom_head"`
	CustomFooter   string `json:"custom_footer"`
	PageSize       int    `json:"page_size"`
	PathMode       string `json:"path_mode"`
	Theme          string `json:"theme"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// Directory 分类（邻接表）
type Directory struct {
	ID          int64  `json:"id"`
	ParentID    int64  `json:"parent_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	PageTitle   string `json:"page_title"`
	PageContent string `json:"page_content"`
	PageSummary string `json:"page_summary"`
	Cover       string `json:"cover"`
	IsPage      int    `json:"is_page"`
	IsPublished int    `json:"is_published"`
	MenuVisible int    `json:"menu_visible"`
	Sort        int    `json:"sort"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Tag 标签
type Tag struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Sort      int    `json:"sort"`
	CreatedAt string `json:"created_at"`
}

// TagCount 带引用计数
type TagCount struct {
	Tag
	UseCount int64 `json:"use_count"`
}

// Post 文章
type Post struct {
	ID          int64    `json:"id"`
	DirID       int64    `json:"dir_id"`
	DirName     string   `json:"dir_name"` // 所属目录名（service 层填充）
	DirPath     string   `json:"dir_path"` // 完整层级路径 "技术博客 / Go 语言"（service 层填充）
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Content     string   `json:"content"`
	Summary     string   `json:"summary"`
	Cover       string   `json:"cover"`
	Gallery     []string `json:"gallery"`
	Tags        []Tag    `json:"tags"`
	Sort        int      `json:"sort"`
	IsPublished int      `json:"is_published"`
	PublishedAt string   `json:"published_at"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// Page 独立页面
type Page struct {
	ID          int64    `json:"id"`
	DirID       int64    `json:"dir_id"`
	DirName     string   `json:"dir_name"`
	DirPath     string   `json:"dir_path"`
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Content     string   `json:"content"`
	Summary     string   `json:"summary"`
	Cover       string   `json:"cover"`
	Gallery     []string `json:"gallery"`
	Tags        []Tag    `json:"tags"`
	Sort        int      `json:"sort"`
	IsPublished int      `json:"is_published"`
	PublishedAt string   `json:"published_at"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// Snapshot 历史快照
type Snapshot struct {
	ID         int64    `json:"id"`
	TargetType string   `json:"target_type"`
	TargetID   int64    `json:"target_id"`
	DirID      int64    `json:"dir_id"`
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	Content    string   `json:"content"`
	Summary    string   `json:"summary"`
	Cover      string   `json:"cover"`
	Gallery    []string `json:"gallery"`
	Tags       []string `json:"tags"`
	CreatedAt  string   `json:"created_at"`
}

// SnapshotMeta 快照列表项（不含正文）
type SnapshotMeta struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

// Asset 资源
type Asset struct {
	ID        int64  `json:"id"`
	Path      string `json:"path"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	Mime      string `json:"mime"`
	RefCount  int    `json:"ref_count"`
	CreatedAt string `json:"created_at"`
	URL       string `json:"url"`
}

// Task 任务（生成 / 重建索引）
type Task struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Progress   int    `json:"progress"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
	Log        string `json:"log"`
	Error      string `json:"error"`
	CreatedAt  string `json:"created_at"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

// TreeNode 混合内容树节点
type TreeNode struct {
	Key         string      `json:"key"`
	Type        string      `json:"type"`
	ID          int64       `json:"id"`
	ParentID    int64       `json:"parent_id"`
	Label       string      `json:"label"`
	Slug        string      `json:"slug,omitempty"`
	Sort        int         `json:"sort"`
	IsPublished int         `json:"is_published,omitempty"`
	IsPage      int         `json:"is_page,omitempty"`
	MenuVisible int         `json:"menu_visible,omitempty"`
	Children    []*TreeNode `json:"children"`
}

// NavItem 导航菜单项（生成引擎输出）
type NavItem struct {
	Label    string    `json:"label"`
	Slug     string    `json:"slug"`
	URL      string    `json:"url"`
	Children []NavItem `json:"children,omitempty"`
	IsPage   bool      `json:"is_page"`
}

// 请求 DTO

type DirInput struct {
	ParentID    int64  `json:"parent_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	PageTitle   string `json:"page_title"`
	PageContent string `json:"page_content"`
	PageSummary string `json:"page_summary"`
	Cover       string `json:"cover"`
	IsPage      int    `json:"is_page"`
	IsPublished int    `json:"is_published"`
	MenuVisible int    `json:"menu_visible"`
}

// DirectoryInput repo.CreateDir 用的完整分类输入
type DirectoryInput struct {
	ParentID    int64
	Name        string
	Slug        string
	PageTitle   string
	PageContent string
	PageSummary string
	Cover       string
	IsPage      int
	IsPublished int
	MenuVisible int
	Sort        int
}

type DirUpdateInput struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	ParentID    *int64  `json:"parent_id"`
	Sort        *int    `json:"sort"`
	PageTitle   *string `json:"page_title"`
	PageContent *string `json:"page_content"`
	PageSummary *string `json:"page_summary"`
	Cover       *string `json:"cover"`
	IsPage      *int    `json:"is_page"`
	IsPublished *int    `json:"is_published"`
	MenuVisible *int    `json:"menu_visible"`
}

// DirectoryPatch repo.UpdateDir 用的完整可选 patch
type DirectoryPatch struct {
	ParentID    *int64
	Name        *string
	Slug        *string
	PageTitle   *string
	PageContent *string
	PageSummary *string
	Cover       *string
	IsPage      *int
	IsPublished *int
	MenuVisible *int
	Sort        *int
}

type TagInput struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Sort int    `json:"sort"`
}

type TagUpdateInput struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
	Sort *int    `json:"sort"`
}

type PostInput struct {
	DirID       int64    `json:"dir_id"`
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Content     string   `json:"content"`
	Summary     string   `json:"summary"`
	Cover       string   `json:"cover"`
	Gallery     []string `json:"gallery"`
	Tags        []string `json:"tags"`
	Sort        int      `json:"sort"`
	IsPublished int      `json:"is_published"`
	PublishedAt string   `json:"published_at"`
}

type PageInput struct {
	DirID       int64    `json:"dir_id"`
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Content     string   `json:"content"`
	Summary     string   `json:"summary"`
	Cover       string   `json:"cover"`
	Gallery     []string `json:"gallery"`
	Tags        []string `json:"tags"`
	Sort        int      `json:"sort"`
	IsPublished int      `json:"is_published"`
	PublishedAt string   `json:"published_at"`
}

type MoveInput struct {
	DirID int64 `json:"dir_id"`
}

type SortItem struct {
	Type string `json:"type"`
	ID   int64  `json:"id"`
	Sort int    `json:"sort"`
}

type TreeSortInput struct {
	ParentID int64      `json:"parent_id"`
	Items    []SortItem `json:"items"`
}

type RollbackInput struct {
	SnapshotID int64 `json:"snapshot_id"`
}

type BatchIDsInput struct {
	IDs []int64 `json:"ids"`
}

// Stats 仪表盘统计
type Stats struct {
	Posts        CountPair   `json:"posts"`
	Pages        CountPair   `json:"pages"`
	Dirs         int64       `json:"dirs"`
	Tags         int64       `json:"tags"`
	Assets       AssetStats  `json:"assets"`
	FTS          FTSStats    `json:"fts"`
	Backups      BackupStats `json:"backups"`
	DBSize       int64       `json:"db_size"`
	LastGenerate *Task       `json:"last_generate"`
	RecentTasks  []Task      `json:"recent_tasks"`
}

type CountPair struct {
	Total     int64 `json:"total"`
	Published int64 `json:"published"`
}

type AssetStats struct {
	Count int64 `json:"count"`
	Size  int64 `json:"size"`
}

type FTSStats struct {
	Docs   uint64 `json:"docs"`
	Exists bool   `json:"exists"`
}

type BackupStats struct {
	Count int `json:"count"`
}

// SearchHit 检索结果
type SearchHit struct {
	Type      string `json:"type"`
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Published int    `json:"published"`
	UpdatedAt string `json:"updated_at"`
	Snippet   string `json:"snippet"`
}

// ThemeInfo 主题信息
type ThemeInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Builtin     bool   `json:"builtin"`
	Active      bool   `json:"active"`
	Valid       bool   `json:"valid"`
}
