// Package bootstrap 统一初始化 MarkSite 运行时：数据目录、数据库、全文索引、主题与业务服务。
// CLI 与桌面端共用，避免两套入口出现偏差。
package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"marksite/internal/db"
	"marksite/internal/fts"
	"marksite/internal/router"
	"marksite/internal/service"
	"marksite/internal/theme"
)

// PathsFor 根据工作目录生成运行路径：源数据放 data/，生成的静态站点放 output/
func PathsFor(workDir string) service.Paths {
	dataDir := filepath.Join(workDir, "data")
	return service.Paths{
		WorkDir:    workDir,
		DataDir:    dataDir,
		DBPath:     filepath.Join(dataDir, "marksite.db"),
		AssetsDir:  filepath.Join(dataDir, "assets"),
		ThemesDir:  filepath.Join(dataDir, "themes"),
		OutputDir:  filepath.Join(workDir, "output"),
		FTSDir:     filepath.Join(dataDir, "ftsindex"),
		BackupsDir: filepath.Join(dataDir, "backups"),
	}
}

// Runtime 持有运行期依赖，Close 时释放数据库连接
type Runtime struct {
	App   *service.App
	Paths service.Paths
	Level *slog.LevelVar

	database *sql.DB
}

// Init 初始化日志、工作目录与业务服务
func Init(workDir string) (*Runtime, error) {
	level := new(slog.LevelVar)
	level.Set(slog.LevelInfo)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	paths := PathsFor(workDir)
	if err := db.EnsureDataDir(paths.DataDir, paths.AssetsDir, paths.ThemesDir, paths.OutputDir, paths.FTSDir, paths.BackupsDir); err != nil {
		return nil, err
	}
	database, err := db.Open(paths.DBPath)
	if err != nil {
		return nil, err
	}
	index := fts.New(paths.FTSDir)
	themes := theme.NewManager(paths.ThemesDir)
	app := service.NewApp(database, paths, index, themes, level)

	ctx := context.Background()
	app.CleanupStaleTasks(ctx)
	app.ApplyLogLevel(ctx)

	return &Runtime{App: app, Paths: paths, Level: level, database: database}, nil
}

// Handler 构建 HTTP 路由（API + 内嵌前端 + 资源 + 预览）
func (r *Runtime) Handler() http.Handler {
	return router.New(r.App)
}

// Listen 创建监听；addr 传 "127.0.0.1:0" 时由系统分配空闲端口
func (r *Runtime) Listen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// StartBackupScheduler 启动每 10 分钟检查一次的自动备份
func (r *Runtime) StartBackupScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.App.AutoBackupIfDue(context.Background())
			}
		}
	}()
}

// Close 关闭数据库
func (r *Runtime) Close() {
	if err := r.database.Close(); err != nil {
		slog.Warn("关闭数据库失败", "err", err)
	}
}
