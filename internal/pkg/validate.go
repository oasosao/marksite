package pkg

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ValidSlug 校验 slug 格式（小写字母/数字/连字符）
func ValidSlug(s string) bool {
	return len(s) > 0 && len(s) <= 120 && slugRe.MatchString(s)
}

// ValidDirName 校验分类名：非空、长度、无路径分隔符与控制字符
func ValidDirName(s string) bool {
	if s == "" || len([]rune(s)) > 100 {
		return false
	}
	if s == "." || s == ".." {
		return false
	}
	if strings.ContainsAny(s, `/\`) {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// NowStr 当前 UTC 时间（RFC3339）
func NowStr() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// NormalizeTime 校验并规范化时间字符串；空串允许
func NormalizeTime(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return "", err
	}
	return t.UTC().Format(time.RFC3339), nil
}

// ParsePagination 解析分页参数
func ParsePagination(r *http.Request) (page, size int) {
	page, _ = strconv.Atoi(QueryDefault(r, "page", "1"))
	size, _ = strconv.Atoi(QueryDefault(r, "page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
