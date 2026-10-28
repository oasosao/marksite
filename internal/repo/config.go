package repo

import (
	"context"
	"database/sql"
	"errors"

	"marksite/internal/model"
)

const appConfigCols = `id, listen_addr, log_level, snapshot_limit, auto_backup_enabled, auto_backup_interval_hours,
 backup_keep_count, auto_open_browser, output_dir, created_at, updated_at`

func (r *Repo) GetAppConfig(ctx context.Context) (*model.AppConfig, error) {
	var c model.AppConfig
	err := r.q.QueryRowContext(ctx, `SELECT `+appConfigCols+` FROM app_config WHERE id = 1`).
		Scan(&c.ID, &c.ListenAddr, &c.LogLevel, &c.SnapshotLimit, &c.AutoBackupEnabled, &c.AutoBackupIntervalHours,
			&c.BackupKeepCount, &c.AutoOpenBrowser, &c.OutputDir, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repo) UpdateAppConfig(ctx context.Context, c *model.AppConfig) error {
	_, err := r.q.ExecContext(ctx, `UPDATE app_config SET
		listen_addr = ?, log_level = ?, snapshot_limit = ?, auto_backup_enabled = ?,
		auto_backup_interval_hours = ?, backup_keep_count = ?,
		auto_open_browser = ?, output_dir = ?, updated_at = ? WHERE id = 1`,
		c.ListenAddr, c.LogLevel, c.SnapshotLimit, c.AutoBackupEnabled, c.AutoBackupIntervalHours,
		c.BackupKeepCount, c.AutoOpenBrowser, c.OutputDir, c.UpdatedAt)
	return err
}

const siteConfigCols = `id, site_name, site_desc, site_url, logo, favicon, copyright, rss_enabled, sitemap_enabled,
 robots, custom_head, custom_footer, page_size, path_mode, theme, created_at, updated_at`

func (r *Repo) GetSiteConfig(ctx context.Context) (*model.SiteConfig, error) {
	var c model.SiteConfig
	err := r.q.QueryRowContext(ctx, `SELECT `+siteConfigCols+` FROM site_config WHERE id = 1`).
		Scan(&c.ID, &c.SiteName, &c.SiteDesc, &c.SiteURL, &c.Logo, &c.Favicon, &c.Copyright,
			&c.RSSEnabled, &c.SitemapEnabled, &c.Robots, &c.CustomHead, &c.CustomFooter,
			&c.PageSize, &c.PathMode, &c.Theme, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repo) UpdateSiteConfig(ctx context.Context, c *model.SiteConfig) error {
	_, err := r.q.ExecContext(ctx, `UPDATE site_config SET
		site_name = ?, site_desc = ?, site_url = ?, logo = ?, favicon = ?, copyright = ?,
		rss_enabled = ?, sitemap_enabled = ?, robots = ?, custom_head = ?, custom_footer = ?,
		page_size = ?, path_mode = ?, theme = ?, updated_at = ? WHERE id = 1`,
		c.SiteName, c.SiteDesc, c.SiteURL, c.Logo, c.Favicon, c.Copyright,
		c.RSSEnabled, c.SitemapEnabled, c.Robots, c.CustomHead, c.CustomFooter,
		c.PageSize, c.PathMode, c.Theme, c.UpdatedAt)
	return err
}
