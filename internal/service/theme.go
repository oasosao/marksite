package service

import (
	"context"

	"marksite/internal/model"
	"marksite/internal/pkg"
)

// ListThemes 主题列表（标记当前激活）
func (a *App) ListThemes(ctx context.Context) ([]model.ThemeInfo, error) {
	site, err := a.Repo.GetSiteConfig(ctx)
	if err != nil {
		return nil, err
	}
	return a.Themes.List(site.Theme)
}

// InstallTheme 安装主题包
func (a *App) InstallTheme(ctx context.Context, zipPath string) (model.ThemeInfo, error) {
	info, err := a.Themes.Install(zipPath)
	if err != nil {
		return info, err
	}
	site, err := a.Repo.GetSiteConfig(ctx)
	if err == nil {
		info.Active = site.Theme == info.Name
	}
	return info, nil
}

// SetActiveTheme 激活主题
func (a *App) SetActiveTheme(ctx context.Context, name string) (map[string]string, error) {
	if _, err := a.Themes.Get(name); err != nil {
		return nil, pkg.BadRequest("主题不存在或无效")
	}
	site, err := a.Repo.GetSiteConfig(ctx)
	if err != nil {
		return nil, err
	}
	site.Theme = name
	site.UpdatedAt = pkg.NowStr()
	if err := a.Repo.UpdateSiteConfig(ctx, site); err != nil {
		return nil, err
	}
	return map[string]string{"active": name}, nil
}

// DeleteTheme 删除用户主题
func (a *App) DeleteTheme(ctx context.Context, name string) error {
	site, err := a.Repo.GetSiteConfig(ctx)
	if err != nil {
		return err
	}
	return a.Themes.Delete(name, site.Theme)
}
