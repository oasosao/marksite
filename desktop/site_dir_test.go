package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeSiteDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()

	cases := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "空", in: "  ", wantErr: "请先选择站点目录"},
		{name: "相对路径", in: "blog", wantErr: "请填写绝对路径"},
		{name: "根目录", in: "/", wantErr: "磁盘根目录"},
		{name: "用户主目录", in: home, wantErr: "用户主目录"},
		{name: "已存在目录带尾斜杠", in: base + "/", want: base},
		{name: "多级路径自动创建", in: filepath.Join(base, "a", "b"), want: filepath.Join(base, "a", "b")},
	}

	for _, c := range cases {
		got, err := normalizeSiteDir(c.in)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("%s: 期望错误包含 %q，实际 got=%q err=%v", c.name, c.wantErr, got, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: 意外错误 %v", c.name, err)
		}
		if got != c.want {
			t.Fatalf("%s: 期望 %q，实际 %q", c.name, c.want, got)
		}
		if st, statErr := os.Stat(got); statErr != nil || !st.IsDir() {
			t.Fatalf("%s: 目录未创建: %v", c.name, statErr)
		}
	}
}

func TestCleanSiteDir(t *testing.T) {
	if got := cleanSiteDir("  "); got != "" {
		t.Fatalf("空白应返回空串，实际 %q", got)
	}
	if got := cleanSiteDir(" ./a/../b/ "); got != "b" {
		t.Fatalf("应清理多余层级与空白，实际 %q", got)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cleanSiteDir("~/Sites/blog"), filepath.Join(home, "Sites", "blog"); got != want {
		t.Fatalf("~ 未展开：期望 %q，实际 %q", want, got)
	}
}

func TestSiteName(t *testing.T) {
	if got := siteName("", "/tmp/ms/blog"); got != "blog" {
		t.Fatalf("空名称应回退目录名，实际 %q", got)
	}
	if got := siteName("  我的站点 ", "/tmp/ms/blog"); got != "我的站点" {
		t.Fatalf("名称应去掉首尾空白，实际 %q", got)
	}
}
