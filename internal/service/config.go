package service

import (
	"context"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"marksite/internal/model"
	"marksite/internal/pkg"
)

// GetAppConfig 读取程序设置
func (a *App) GetAppConfig(ctx context.Context) (*model.AppConfig, error) {
	return a.Repo.GetAppConfig(ctx)
}

// UpdateAppConfig 全量更新程序设置
func (a *App) UpdateAppConfig(ctx context.Context, in model.AppConfig) (*model.AppConfig, error) {
	in.ListenAddr = strings.TrimSpace(in.ListenAddr)
	host, port, err := net.SplitHostPort(in.ListenAddr)
	if err != nil || port == "" || host == "" {
		return nil, pkg.BadRequest("监听地址格式应为 host:port")
	}
	if _, err := net.LookupPort("tcp", port); err != nil {
		return nil, pkg.BadRequest("监听端口非法")
	}
	in.LogLevel = strings.ToLower(strings.TrimSpace(in.LogLevel))
	switch in.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return nil, pkg.BadRequest("日志级别必须为 debug/info/warn/error")
	}
	if in.SnapshotLimit < 1 || in.SnapshotLimit > 200 {
		return nil, pkg.BadRequest("快照上限必须为 1–200")
	}
	if in.AutoBackupIntervalHours < 1 || in.AutoBackupIntervalHours > 168 {
		return nil, pkg.BadRequest("自动备份间隔必须为 1–168 小时")
	}
	if in.BackupKeepCount < 1 || in.BackupKeepCount > 50 {
		return nil, pkg.BadRequest("备份保留数量必须为 1–50")
	}
	in.OutputDir = strings.TrimSpace(in.OutputDir)
	// 留空表示使用工作目录下的 output 目录
	if in.OutputDir != "" {
		absOut, err := filepath.Abs(in.OutputDir)
		if err != nil {
			return nil, pkg.BadRequest("输出目录非法")
		}
		if absOut == string(filepath.Separator) {
			return nil, pkg.BadRequest("输出目录不能为根目录")
		}
		for _, self := range selfDirs() {
			if absOut == self {
				return nil, pkg.BadRequest("输出目录不能为程序目录本身")
			}
		}
	}
	in.AutoBackupEnabled = flag01(in.AutoBackupEnabled)
	in.AutoOpenBrowser = flag01(in.AutoOpenBrowser)
	in.UpdatedAt = pkg.NowStr()
	if err := a.Repo.UpdateAppConfig(ctx, &in); err != nil {
		return nil, err
	}
	// 日志级别立即生效
	if a.Level != nil {
		a.Level.Set(parseLevel(in.LogLevel))
	}
	return a.Repo.GetAppConfig(ctx)
}

// GetSiteConfig 读取站点设置
func (a *App) GetSiteConfig(ctx context.Context) (*model.SiteConfig, error) {
	return a.Repo.GetSiteConfig(ctx)
}

// UpdateSiteConfig 全量更新站点设置
func (a *App) UpdateSiteConfig(ctx context.Context, in model.SiteConfig) (*model.SiteConfig, error) {
	in.SiteName = strings.TrimSpace(in.SiteName)
	if in.SiteName == "" || utf8.RuneCountInString(in.SiteName) > 100 {
		return nil, pkg.BadRequest("站点名称不能为空且不超过 100 字符")
	}
	in.SiteDesc = strings.TrimSpace(in.SiteDesc)
	if utf8.RuneCountInString(in.SiteDesc) > 500 {
		return nil, pkg.BadRequest("站点描述不能超过 500 字符")
	}
	in.SiteURL = strings.TrimRight(strings.TrimSpace(in.SiteURL), "/")
	u, err := url.Parse(in.SiteURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, pkg.BadRequest("站点 URL 必须为合法的 http(s) 地址")
	}
	in.Logo = normalizeAssetPath(in.Logo)
	in.Favicon = normalizeAssetPath(in.Favicon)
	in.Copyright = strings.TrimSpace(in.Copyright)
	if utf8.RuneCountInString(in.Copyright) > 500 {
		return nil, pkg.BadRequest("版权信息不能超过 500 字符")
	}
	if utf8.RuneCountInString(in.Robots) > 5000 {
		return nil, pkg.BadRequest("robots 内容不能超过 5000 字符")
	}
	if utf8.RuneCountInString(in.CustomHead) > 20000 || utf8.RuneCountInString(in.CustomFooter) > 20000 {
		return nil, pkg.BadRequest("自定义 head/footer 不能超过 20000 字符")
	}
	if in.PageSize < 1 || in.PageSize > 100 {
		return nil, pkg.BadRequest("每页数量必须为 1–100")
	}
	// path_mode 已废弃（生成器固定 URL 规则），仅保留字段兼容旧库
	switch in.PathMode {
	case "slug", "type", "dir", "fixed":
	default:
		return nil, pkg.BadRequest("URL 模式必须为 slug/type/dir/fixed")
	}
	if in.Theme == "" {
		in.Theme = "default"
	}
	if _, err := a.Themes.Get(in.Theme); err != nil {
		return nil, pkg.BadRequest("主题不存在或无效")
	}
	in.RSSEnabled = flag01(in.RSSEnabled)
	in.SitemapEnabled = flag01(in.SitemapEnabled)
	in.UpdatedAt = pkg.NowStr()
	if err := a.Repo.UpdateSiteConfig(ctx, &in); err != nil {
		return nil, err
	}
	return a.Repo.GetSiteConfig(ctx)
}

func flag01(v int) int {
	if v == 1 {
		return 1
	}
	return 0
}

func normalizeAssetPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "media/")
	return p
}

// ApplyLogLevel 按数据库配置设置日志级别（启动时调用）
func (a *App) ApplyLogLevel(ctx context.Context) {
	cfg, err := a.Repo.GetAppConfig(ctx)
	if err != nil || a.Level == nil {
		return
	}
	a.Level.Set(parseLevel(cfg.LogLevel))
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// selfDirs 程序工作目录与可执行文件目录
func selfDirs() []string {
	out := []string{}
	if wd, err := os.Getwd(); err == nil {
		out = append(out, wd)
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		out = append(out, filepath.Dir(exe))
	}
	return out
}
