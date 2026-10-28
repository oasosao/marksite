package fts

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestFindSnippetAnchor 验证锚点定位逻辑（精确匹配 + analyzer 分词回退）
func TestFindSnippetAnchor(t *testing.T) {
	plain := "MarkSite 是一个用 Go 开发的静态网站生成管理系统，支持快速地安装和部署，内置多种主题可以使用。"
	lower := strings.ToLower(plain)

	tests := []struct {
		name  string
		query string
		// anchor 必须落在预期子串附近（允许少量偏移，因为锚点取的是 token 首次命中）
		anchorAround string
	}{
		{
			name:         "精确子串匹配 快路径",
			query:        "Go 开发",
			anchorAround: "Go 开发",
		},
		{
			name:         "CJK 分词回退: 查询词被其他字隔开",
			query:        "快速安装",
			anchorAround: "快速地安装",
		},
		{
			name:         "CJK 单字 token",
			query:        "主题",
			anchorAround: "主题可以使用",
		},
		{
			name:         "英文分词回退",
			query:        "static site",
			anchorAround: "静态网站",
		},
		{
			name:         "完全不命中 → 兜底首个 token 出现位置",
			query:        "𠮷𠮸𠮹xyz",
			anchorAround: "MarkSite", // 兜底在正文开头（首个 token 命中位置或 0）
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pos := findSnippetAnchor(lower, tc.query)
			if pos < 0 || pos > len(plain) {
				t.Fatalf("返回的字节偏移 %d 越界 (len=%d)", pos, len(plain))
			}
			if !strings.Contains(lower[pos:], strings.ToLower(tc.anchorAround)) {
				t.Errorf("锚点 pos=%d 附近未找到 %q, 上下文: ...%s...",
					pos, tc.anchorAround, truncate(plain[pos:], 60))
			}
		})
	}
}

// TestSnippet 验证完整 snippet 函数（含 rune 偏移转换）
func TestSnippet(t *testing.T) {
	// 含大量多字节字符的正文，用来测试 byte → rune 偏移转换
	plain := strings.Repeat("你好世界Hello", 20) + "MarkSite静态网站生成管理系统支持快速安装和部署" + strings.Repeat("测试数据", 30)

	tests := []struct {
		name  string
		query string
	}{
		{"CJK 精确匹配", "快速安装"},
		{"CJK 部分匹配", "MarkSite"},
		{"全英文", "Hello"},
		{"混合查询", "你好 Hello"},
		{"空查询", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := snippet(plain, tc.query)
			if s == "" {
				if tc.query == "" {
					return // 空查询返回空是预期
				}
				t.Fatal("snippet 返回空串")
			}
			if !utf8.ValidString(s) {
				t.Errorf("snippet 截断了 UTF-8: %q", s)
			}
		})
	}
}

// TestSnippetByteRuneRegression 原始 bug 回归：字节偏移 vs rune 偏移错位
func TestSnippetByteRuneRegression(t *testing.T) {
	// 正文前半段全是多字节中文，确保字节偏移与 rune 偏移差异大
	plain := strings.Repeat("中", 50) + "这是一段测试文本hello world" + strings.Repeat("后", 50)

	// 搜索在中文字符之后的位置
	s := snippet(plain, "hello")
	if !strings.Contains(s, "hello") {
		t.Errorf("snippet 应包含 'hello' 关键词, 实际: %q", s)
	}
	// 关键：之前的 bug 会把字节偏移直接当 rune 偏移用，导致中文区被错误截断
	// 检查 snippet 内的 rune 完整性
	if !utf8.ValidString(s) {
		t.Errorf("snippet 包含无效 UTF-8 (byte/rune 偏移错位): %q", s)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
