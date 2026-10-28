package service

import (
	"database/sql"
	"log/slog"
	"sync"

	"marksite/internal/fts"
	"marksite/internal/repo"
	"marksite/internal/theme"
)

// Paths 运行期路径
type Paths struct {
	// WorkDir 工作目录：源数据与站点产物都放在其下
	WorkDir    string
	DataDir    string
	DBPath     string
	AssetsDir  string
	ThemesDir  string
	OutputDir  string
	FTSDir     string
	BackupsDir string
}

// App 业务服务集合（handler -> service -> repo）
type App struct {
	DB     *sql.DB
	Repo   *repo.Repo
	FTS    *fts.Index
	Themes *theme.Manager
	Paths  Paths
	Level  *slog.LevelVar

	taskMu      sync.Mutex
	genRunning  bool
	ftsRunning  bool
	cancelFlags map[int64]bool

	backupMu sync.Mutex
}

func NewApp(db *sql.DB, paths Paths, index *fts.Index, themes *theme.Manager, level *slog.LevelVar) *App {
	return &App{
		DB: db, Repo: repo.New(db), FTS: index, Themes: themes, Paths: paths, Level: level,
		cancelFlags: map[int64]bool{},
	}
}

func (a *App) isCanceled(id int64) bool {
	a.taskMu.Lock()
	defer a.taskMu.Unlock()
	return a.cancelFlags[id]
}

func (a *App) clearCancel(id int64) {
	a.taskMu.Lock()
	delete(a.cancelFlags, id)
	a.taskMu.Unlock()
}

func (a *App) markCanceled(id int64) {
	a.taskMu.Lock()
	a.cancelFlags[id] = true
	a.taskMu.Unlock()
}
