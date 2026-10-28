package pkg

import (
	"regexp"
	"strings"

	"github.com/mozillazg/go-pinyin"
)

var slugSep = regexp.MustCompile(`[\s_]+`)

// Slugify 把任意名称（含中文）转换成小写、用中划线分隔的 ASCII slug。
// 中文名用 go-pinyin 转为无声调拼音，如 "技术博客" → "ji-zhu-bo-ke"。
// 完全非 ASCII 时返回 fallback（默认 "untitled"）。
func Slugify(name string, fallback ...string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	if base == "" {
		return pickFallback(fallback, "untitled")
	}

	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		case r > 0x7f: // 中文等非 ASCII 走拼音
			p := pinyin.Pinyin(string(r), pinyin.NewArgs())
			if len(p) > 0 && len(p[0]) > 0 {
				b.WriteString(p[0][0])
			}
		}
	}

	out := b.String()
	out = slugSep.ReplaceAllString(out, "-")
	out = strings.Trim(out, "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if out == "" {
		return pickFallback(fallback, "untitled")
	}
	return out
}

func pickFallback(fallback []string, def string) string {
	if len(fallback) > 0 && fallback[0] != "" {
		return fallback[0]
	}
	return def
}
