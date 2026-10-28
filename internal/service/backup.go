package service

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"marksite/internal/backup"
	"marksite/internal/db"
	"marksite/internal/pkg"
	"marksite/internal/repo"
)

// backupPaths 备份使用的路径集合
func (a *App) backupPaths() backup.Paths {
	return backup.Paths{
		DBPath:     a.Paths.DBPath,
		AssetsDir:  a.Paths.AssetsDir,
		FTSDir:     a.Paths.FTSDir,
		BackupsDir: a.Paths.BackupsDir,
	}
}

// ListBackups 备份列表
func (a *App) ListBackups() ([]backup.Info, error) {
	return backup.List(a.backupPaths())
}

// CreateBackup 手动创建备份
func (a *App) CreateBackup(ctx context.Context) (map[string]any, error) {
	a.backupMu.Lock()
	defer a.backupMu.Unlock()
	name := "marksite-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	info, err := backup.Create(a.DB, a.backupPaths(), name)
	if err != nil {
		return nil, err
	}
	return map[string]any{"name": info.Name, "size": info.Size}, nil
}

// DeleteBackup 删除备份
func (a *App) DeleteBackup(name string) error {
	return backup.Delete(a.backupPaths(), name)
}

// BackupFile 备份文件绝对路径
func (a *App) BackupFile(name string) (string, error) {
	p, err := backup.Path(a.backupPaths(), name)
	if err != nil {
		return "", pkg.NotFound("备份文件不存在")
	}
	return p, nil
}

// RestoreBackup 恢复备份
func (a *App) RestoreBackup(ctx context.Context, name string) (map[string]any, error) {
	if a.Busy() {
		return nil, pkg.Busy("请先等待生成/索引任务结束")
	}
	if running, _ := a.Repo.HasRunningTask(ctx); running {
		return nil, pkg.Busy("存在未完成的任务，无法恢复备份")
	}
	zipPath, err := backup.Path(a.backupPaths(), name)
	if err != nil {
		return nil, pkg.NotFound("备份文件不存在")
	}

	a.backupMu.Lock()
	defer a.backupMu.Unlock()

	workDir := filepath.Join(a.Paths.BackupsDir, ".restore-"+pkg.RandomHex(6))
	_ = os.RemoveAll(workDir)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	if err := backup.Extract(zipPath, workDir); err != nil {
		return nil, pkg.BadRequest(err.Error())
	}
	newDB := filepath.Join(workDir, "marksite.db")
	if _, err := os.Stat(newDB); err != nil {
		return nil, pkg.BadRequest("备份包缺少 marksite.db")
	}
	if err := backup.ValidateDB(newDB); err != nil {
		return nil, pkg.BadRequest(err.Error())
	}

	safetyName := "pre-restore-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	if _, err := backup.Create(a.DB, a.backupPaths(), safetyName); err != nil {
		return nil, err
	}

	// 关闭数据库后替换文件
	if err := a.DB.Close(); err != nil {
		return nil, err
	}
	currentDir := filepath.Join(workDir, "current")
	_ = os.MkdirAll(currentDir, 0o755)

	movedOut := []string{}
	moveOut := func(src, dst string) error {
		if _, err := os.Stat(src); err != nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			return err
		}
		movedOut = append(movedOut, dst)
		return nil
	}

	rollback := func() {
		_ = os.Remove(a.Paths.DBPath)
		_ = os.Remove(a.Paths.DBPath + "-wal")
		_ = os.Remove(a.Paths.DBPath + "-shm")
		_ = os.RemoveAll(a.Paths.AssetsDir)
		_ = os.RemoveAll(a.Paths.FTSDir)
		_ = moveBack(filepath.Join(currentDir, "marksite.db"), a.Paths.DBPath)
		_ = moveBack(filepath.Join(currentDir, "assets"), a.Paths.AssetsDir)
		_ = moveBack(filepath.Join(currentDir, "ftsindex"), a.Paths.FTSDir)
		if reopened, err := db.Open(a.Paths.DBPath); err == nil {
			a.DB = reopened
			a.Repo = repo.New(reopened)
		}
	}

	if err := moveOut(a.Paths.DBPath, filepath.Join(currentDir, "marksite.db")); err != nil {
		rollback()
		return nil, pkg.Internal("恢复失败，已回滚")
	}
	if err := moveOut(a.Paths.AssetsDir, filepath.Join(currentDir, "assets")); err != nil {
		rollback()
		return nil, pkg.Internal("恢复失败，已回滚")
	}
	if err := moveOut(a.Paths.FTSDir, filepath.Join(currentDir, "ftsindex")); err != nil {
		rollback()
		return nil, pkg.Internal("恢复失败，已回滚")
	}

	install := func(src, dst string) error {
		if _, err := os.Stat(src); err != nil {
			if strings.HasSuffix(dst, "marksite.db") {
				return err
			}
			return os.MkdirAll(dst, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.Rename(src, dst)
	}
	if err := install(newDB, a.Paths.DBPath); err != nil {
		rollback()
		return nil, pkg.Internal("恢复失败，已回滚")
	}
	if err := install(filepath.Join(workDir, "assets"), a.Paths.AssetsDir); err != nil {
		rollback()
		return nil, pkg.Internal("恢复失败，已回滚")
	}
	if err := install(filepath.Join(workDir, "ftsindex"), a.Paths.FTSDir); err != nil {
		rollback()
		return nil, pkg.Internal("恢复失败，已回滚")
	}

	reopened, err := db.Open(a.Paths.DBPath)
	if err != nil {
		rollback()
		return nil, pkg.Internal("恢复后数据库打开失败，已回滚")
	}
	a.DB = reopened
	a.Repo = repo.New(reopened)
	slog.Info("备份恢复完成", "backup", name, "safety", safetyName)
	return map[string]any{"restored": true, "safety_backup": safetyName}, nil
}

func moveBack(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// AutoBackupIfDue 自动备份调度（每 10 分钟调用一次）
func (a *App) AutoBackupIfDue(ctx context.Context) {
	cfg, err := a.Repo.GetAppConfig(ctx)
	if err != nil || cfg.AutoBackupEnabled != 1 {
		return
	}
	list, err := a.ListBackups()
	if err != nil {
		slog.Warn("自动备份检查失败", "err", err)
		return
	}
	var latest time.Time
	for _, item := range list {
		if !item.Auto {
			continue
		}
		if t, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil && t.After(latest) {
			latest = t
		}
	}
	interval := time.Duration(cfg.AutoBackupIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if !latest.IsZero() && time.Since(latest) < interval {
		return
	}
	a.backupMu.Lock()
	name := "auto-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	_, err = backup.Create(a.DB, a.backupPaths(), name)
	a.backupMu.Unlock()
	if err != nil {
		slog.Warn("自动备份创建失败", "err", err)
		return
	}
	slog.Info("自动备份已创建", "name", name)
	// 仅保留最新 keep 个自动备份（按创建时间倒序，删除超出的旧备份）
	fresh, err := a.ListBackups()
	if err != nil {
		return
	}
	keep := cfg.BackupKeepCount
	if keep < 1 {
		keep = 7
	}
	// 过滤自动备份并按 CreatedAt 倒序排序
	var autos []backup.Info
	for _, item := range fresh {
		if item.Auto {
			autos = append(autos, item)
		}
	}
	sort.Slice(autos, func(i, j int) bool {
		return autos[i].CreatedAt > autos[j].CreatedAt
	})
	for i := keep; i < len(autos); i++ {
		_ = backup.Delete(a.backupPaths(), autos[i].Name)
	}
}
