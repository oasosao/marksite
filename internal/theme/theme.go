package theme

import (
	"archive/zip"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"marksite/internal/model"
	"marksite/internal/pkg"
)

//go:embed default dark ink paper vibrant
var builtinFS embed.FS

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-_]{1,31}$`)

var requiredTemplates = []string{"index.html", "post.html", "page.html", "archive.html"}

// builtinNames 内置主题列表（顺序即显示顺序）
var builtinNames = []string{"default", "dark", "ink", "paper", "vibrant"}

// isBuiltinName 判断是否为内置主题名
func isBuiltinName(name string) bool {
	return slices.Contains(builtinNames, name)
}

// Meta 主题元信息
type Meta struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
}

// Theme 已加载的主题
type Theme struct {
	Meta
	Builtin bool
	FS      fs.FS
}

// Manager 主题管理：内置 default + 用户上传主题
type Manager struct {
	UserDir string
}

func NewManager(userDir string) *Manager {
	return &Manager{UserDir: userDir}
}

func builtinRoot(name string) (fs.FS, error) {
	return fs.Sub(builtinFS, name)
}

// Get 加载指定主题
func (m *Manager) Get(name string) (*Theme, error) {
	if name == "" {
		name = "default"
	}
	for _, n := range builtinNames {
		if name == n {
			root, err := builtinRoot(name)
			if err != nil {
				return nil, err
			}
			meta, err := readMeta(root)
			if err != nil {
				return nil, err
			}
			meta.Name = name
			return &Theme{Meta: meta, Builtin: true, FS: root}, nil
		}
	}
	if !nameRe.MatchString(name) {
		return nil, pkg.BadRequest("主题名称非法")
	}
	dir := filepath.Join(m.UserDir, name)
	root := os.DirFS(dir)
	meta, err := readMeta(root)
	if err != nil {
		return nil, pkg.NotFound("主题不存在或无效")
	}
	if err := validate(root); err != nil {
		return nil, err
	}
	meta.Name = name
	return &Theme{Meta: meta, FS: root}, nil
}

// List 列出全部主题
func (m *Manager) List(active string) ([]model.ThemeInfo, error) {
	out := []model.ThemeInfo{}
	for _, name := range builtinNames {
		root, err := builtinRoot(name)
		if err != nil {
			continue
		}
		meta, err := readMeta(root)
		if err != nil {
			continue
		}
		out = append(out, model.ThemeInfo{
			Name: name, Version: meta.Version, Author: meta.Author,
			Description: meta.Description, Builtin: true, Active: active == name, Valid: true,
		})
	}
	entries, err := os.ReadDir(m.UserDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || !nameRe.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(m.UserDir, e.Name())
		r := os.DirFS(dir)
		info := model.ThemeInfo{Name: e.Name(), Active: active == e.Name()}
		meta, err := readMeta(r)
		if err == nil {
			info.Version = meta.Version
			info.Author = meta.Author
			info.Description = meta.Description
			info.Valid = validate(r) == nil
		}
		out = append(out, info)
	}
	return out, nil
}

func readMeta(root fs.FS) (Meta, error) {
	var meta Meta
	data, err := fs.ReadFile(root, "theme.json")
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, err
	}
	if meta.Version == "" {
		meta.Version = "1.0.0"
	}
	return meta, nil
}

func validate(root fs.FS) error {
	for _, t := range requiredTemplates {
		if _, err := fs.Stat(root, "templates/"+t); err != nil {
			return fmt.Errorf("主题缺少模板 templates/%s", t)
		}
	}
	return nil
}

// Install 从 zip 安装用户主题
func (m *Manager) Install(zipPath string) (model.ThemeInfo, error) {
	tmp, err := os.MkdirTemp(m.UserDir, ".install-")
	if err != nil {
		return model.ThemeInfo{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	if err := extractZip(zipPath, tmp); err != nil {
		return model.ThemeInfo{}, err
	}
	root := tmp
	if _, err := os.Stat(filepath.Join(tmp, "theme.json")); err != nil {
		// 根目录无 theme.json，在子目录中查找
		entries, _ := os.ReadDir(tmp)
		var found string
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if _, err := os.Stat(filepath.Join(tmp, e.Name(), "theme.json")); err == nil {
				if found != "" {
					return model.ThemeInfo{}, pkg.BadRequest("主题包包含多个主题目录，请只保留一个")
				}
				found = filepath.Join(tmp, e.Name())
			}
		}
		if found == "" {
			return model.ThemeInfo{}, pkg.BadRequest("主题包缺少 theme.json")
		}
		root = found
	}
	meta, err := readMeta(os.DirFS(root))
	if err != nil {
		return model.ThemeInfo{}, pkg.BadRequest("theme.json 解析失败")
	}
	if !nameRe.MatchString(meta.Name) || isBuiltinName(meta.Name) {
		return model.ThemeInfo{}, pkg.BadRequest("主题名称非法或与内置主题重名")
	}
	if err := validate(os.DirFS(root)); err != nil {
		return model.ThemeInfo{}, pkg.BadRequest(err.Error())
	}
	dest := filepath.Join(m.UserDir, meta.Name)
	if _, err := os.Stat(dest); err == nil {
		return model.ThemeInfo{}, pkg.Conflict("主题已存在，请先删除")
	}
	if err := os.Rename(root, dest); err != nil {
		return model.ThemeInfo{}, err
	}
	return model.ThemeInfo{
		Name: meta.Name, Version: meta.Version, Author: meta.Author,
		Description: meta.Description, Valid: true,
	}, nil
}

// Delete 删除用户主题
func (m *Manager) Delete(name string, active string) error {
	if isBuiltinName(name) {
		return pkg.BadRequest("内置主题不可删除")
	}
	if name == active {
		return pkg.BadRequest("当前激活的主题不可删除")
	}
	if !nameRe.MatchString(name) {
		return pkg.BadRequest("主题名称非法")
	}
	dir := filepath.Join(m.UserDir, name)
	if _, err := os.Stat(dir); err != nil {
		return pkg.NotFound("主题不存在")
	}
	return os.RemoveAll(dir)
}

// extractZip 安全解压（限制条目数、大小、路径穿越）
func extractZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return pkg.BadRequest("zip 文件无效")
	}
	defer func() { _ = r.Close() }()
	if len(r.File) > 2000 {
		return pkg.BadRequest("zip 条目过多")
	}
	const maxTotal = 100 * 1024 * 1024
	const maxFile = 20 * 1024 * 1024
	var total int64
	for _, f := range r.File {
		if !pkg.SafeZipEntry(f.Name) {
			return pkg.BadRequest("zip 包含非法路径")
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return pkg.BadRequest("zip 包含符号链接")
		}
		name := filepath.FromSlash(f.Name)
		target := filepath.Join(dest, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = rc.Close()
			return err
		}
		n, copyErr := io.Copy(out, io.LimitReader(rc, maxFile))
		_ = out.Close()
		_ = rc.Close()
		if copyErr != nil {
			return copyErr
		}
		// 用实际读取字节数累加，防止声明大小伪造的 zip 炸弹
		total += n
		if total > maxTotal {
			return pkg.BadRequest("zip 解压后体积过大")
		}
	}
	return nil
}
