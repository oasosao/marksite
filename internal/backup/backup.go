package backup

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"marksite/internal/pkg"
)

// SchemaVersion 当前数据库结构版本
const SchemaVersion = 1

// Paths 备份相关路径
type Paths struct {
	DBPath     string
	AssetsDir  string
	FTSDir     string
	BackupsDir string
}

// Info 备份文件信息
type Info struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
	Auto      bool   `json:"auto"`
}

// Manifest 备份包元信息
type Manifest struct {
	AppVersion    string   `json:"app_version"`
	SchemaVersion int      `json:"schema_version"`
	CreatedAt     string   `json:"created_at"`
	Includes      []string `json:"includes"`
}

// Create 创建备份包（db + assets + ftsindex）
func Create(db *sql.DB, p Paths, name string) (Info, error) {
	if !pkg.SafeBackupName(name) {
		return Info{}, fmt.Errorf("备份文件名非法")
	}
	if err := os.MkdirAll(p.BackupsDir, 0o755); err != nil {
		return Info{}, err
	}
	tmpDir, err := os.MkdirTemp(p.BackupsDir, ".tmp-")
	if err != nil {
		return Info{}, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dbCopy := filepath.Join(tmpDir, "marksite.db")
	if _, err := db.Exec(`VACUUM INTO ?`, dbCopy); err != nil {
		return Info{}, err
	}
	manifest := Manifest{
		AppVersion: "1.0.0", SchemaVersion: SchemaVersion,
		CreatedAt: pkg.NowStr(), Includes: []string{"db", "assets", "ftsindex"},
	}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "manifest.json"), mb, 0o644); err != nil {
		return Info{}, err
	}

	zipTmp := filepath.Join(tmpDir, "backup.zip")
	f, err := os.Create(zipTmp)
	if err != nil {
		return Info{}, err
	}
	zw := zip.NewWriter(f)
	defer func() { _ = zw.Close(); _ = f.Close() }()
	addDir := func(base, prefix string) error {
		info, err := os.Stat(base)
		if err != nil || !info.IsDir() {
			return nil
		}
		return filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				return nil
			}
			w, err := zw.Create(prefix + "/" + rel)
			if err != nil {
				return err
			}
			src, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() { _ = src.Close() }()
			_, err = io.Copy(w, src)
			return err
		})
	}
	if err := writeFileToZip(zw, "marksite.db", dbCopy); err != nil {
		return Info{}, err
	}
	if err := writeFileToZip(zw, "manifest.json", filepath.Join(tmpDir, "manifest.json")); err != nil {
		return Info{}, err
	}
	if err := addDir(p.AssetsDir, "assets"); err != nil {
		return Info{}, err
	}
	if err := addDir(p.FTSDir, "ftsindex"); err != nil {
		return Info{}, err
	}
	if err := zw.Close(); err != nil {
		return Info{}, err
	}
	if err := f.Close(); err != nil {
		return Info{}, err
	}
	dest := filepath.Join(p.BackupsDir, name)
	if err := os.Rename(zipTmp, dest); err != nil {
		return Info{}, err
	}
	st, err := os.Stat(dest)
	if err != nil {
		return Info{}, err
	}
	return Info{Name: name, Size: st.Size(), CreatedAt: pkg.NowStr(), Auto: strings.HasPrefix(name, "auto-")}, nil
}

func writeFileToZip(zw *zip.Writer, name, src string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(w, f)
	return err
}

// List 列出备份文件（时间倒序）
func List(p Paths) ([]Info, error) {
	entries, err := os.ReadDir(p.BackupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Info{}, nil
		}
		return nil, err
	}
	out := []Info{}
	for _, e := range entries {
		if e.IsDir() || !pkg.SafeBackupName(e.Name()) {
			continue
		}
		st, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Info{
			Name: e.Name(), Size: st.Size(),
			CreatedAt: st.ModTime().UTC().Format(time.RFC3339),
			Auto:      strings.HasPrefix(e.Name(), "auto-"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// Path 返回备份文件路径
func Path(p Paths, name string) (string, error) {
	if !pkg.SafeBackupName(name) {
		return "", fmt.Errorf("备份文件名非法")
	}
	full := filepath.Join(p.BackupsDir, name)
	if _, err := os.Stat(full); err != nil {
		return "", err
	}
	return full, nil
}

// Delete 删除备份文件
func Delete(p Paths, name string) error {
	full, err := Path(p, name)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

// Extract 解压备份包到目标目录（限制条目数/总大小/路径穿越）
func Extract(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("备份包无效")
	}
	defer func() { _ = r.Close() }()
	if len(r.File) > 5000 {
		return fmt.Errorf("备份包条目过多")
	}
	const maxTotalBytes = 500 * 1024 * 1024
	var written int64
	for _, f := range r.File {
		if !pkg.SafeZipEntry(f.Name) {
			return fmt.Errorf("备份包包含非法路径")
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("备份包包含符号链接")
		}
		target := filepath.Join(dest, filepath.FromSlash(f.Name))
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
		remaining := maxTotalBytes - written
		if remaining <= 0 {
			_ = out.Close()
			_ = rc.Close()
			return fmt.Errorf("备份包解压后体积超过 500MB")
		}
		n, copyErr := io.Copy(out, io.LimitReader(rc, remaining+1)) // +1 用于检测超限
		_ = out.Close()
		_ = rc.Close()
		if n > remaining {
			return fmt.Errorf("备份包解压后体积超过 500MB")
		}
		written += n
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

// ValidateDB 校验备份数据库可读且结构版本不高于当前版本
func ValidateDB(dbPath string) error {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("备份数据库无法打开")
	}
	defer func() { _ = db.Close() }()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("备份数据库无法读取结构版本")
	}
	if version > SchemaVersion {
		return fmt.Errorf("备份数据库结构版本过高（%d）", version)
	}
	var tables int
	if err := db.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name IN ('site_config', 'directories', 'posts', 'pages')`).Scan(&tables); err != nil || tables < 4 {
		return fmt.Errorf("备份数据库缺少必要的数据表")
	}
	return nil
}
