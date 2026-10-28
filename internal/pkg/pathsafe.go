package pkg

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// SafeJoin 将相对路径安全地拼接到 base 下，拒绝路径穿越
func SafeJoin(base, rel string) (string, error) {
	cleanBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", errors.New("路径为空")
	}
	if filepath.IsAbs(rel) || strings.Contains(rel, "\\") {
		return "", errors.New("非法路径")
	}
	joined := filepath.Join(cleanBase, filepath.FromSlash(rel))
	joinedAbs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if joinedAbs != cleanBase && !strings.HasPrefix(joinedAbs, cleanBase+string(os.PathSeparator)) {
		return "", errors.New("路径越界")
	}
	return joinedAbs, nil
}

// SafeZipEntry 校验 zip 条目名：禁止绝对路径与 ..
func SafeZipEntry(name string) bool {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || part == "." {
			return false
		}
	}
	return true
}

// SafeBackupName 校验备份文件名
func SafeBackupName(name string) bool {
	if name == "" || name != filepath.Base(name) {
		return false
	}
	if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return false
	}
	if !strings.HasSuffix(name, ".zip") {
		return false
	}
	for _, r := range strings.TrimSuffix(name, ".zip") {
		if r != '-' && r != '_' && r != '.' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// RandomHex 生成随机十六进制字符串
func RandomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(buf)
}
