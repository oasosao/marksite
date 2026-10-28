package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS 前端构建产物
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
