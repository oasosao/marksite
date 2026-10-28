package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSitesAPI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // 让站点配置写到隔离目录

	base := t.TempDir()
	d := &Desktop{}
	var started string
	d.start = func(dir string) error { started = dir; return nil }
	d.stop = func() {}
	d.reload = func() {}
	d.setInfo(WorkDirInfo{State: "need_setup"})
	api := &sitesAPI{d: d}

	call := func(method, path, body string) (int, map[string]any, http.Header) {
		var r *http.Request
		if body == "" {
			r = httptest.NewRequest(method, path, nil)
		} else {
			r = httptest.NewRequest(method, path, bytes.NewBufferString(body))
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out, w.Header()
	}
	create := func(dir, name string) (int, map[string]any) {
		b, _ := json.Marshal(map[string]string{"dir": dir, "name": name})
		code, out, _ := call(http.MethodPost, "/__desktop/sites/create", string(b))
		return code, out
	}
	remove := func(dir string) (int, map[string]any) {
		b, _ := json.Marshal(map[string]string{"dir": dir})
		code, out, _ := call(http.MethodPost, "/__desktop/sites/remove", string(b))
		return code, out
	}
	list := func() []any {
		code, out, h := call(http.MethodGet, "/__desktop/sites", "")
		if code != http.StatusOK {
			t.Fatalf("列表接口状态 %d: %v", code, out)
		}
		if h.Get("Cache-Control") != "no-store" {
			t.Fatalf("列表接口缺少 no-store 缓存头: %q", h.Get("Cache-Control"))
		}
		data, _ := out["data"].(map[string]any)
		arr, _ := data["sites"].([]any)
		return arr
	}

	// 1. 相对路径拒绝
	if code, out := create("rel-site", ""); code != http.StatusBadRequest || !strings.Contains(out["message"].(string), "绝对路径") {
		t.Fatalf("相对路径应被拒绝，实际 %d %v", code, out)
	}
	if got := list(); len(got) != 0 {
		t.Fatalf("失败的新建不应登记站点: %v", got)
	}

	// 2. 正常新建并打开
	dir1 := filepath.Join(base, "site1")
	if code, out := create(dir1, ""); code != http.StatusOK {
		t.Fatalf("新建失败: %d %v", code, out)
	}
	if started != dir1 {
		t.Fatalf("新建后应打开该站点，实际 %q", started)
	}
	sites := list()
	if len(sites) != 1 {
		t.Fatalf("应有 1 个站点，实际 %v", sites)
	}
	first := sites[0].(map[string]any)
	if first["dir"] != dir1 || first["name"] != "site1" || first["exists"] != true || first["last"] != true {
		t.Fatalf("站点信息不符: %v", first)
	}

	// 3. 重复目录被拒绝
	if code, out := create(dir1, "另一个"); code != http.StatusBadRequest || !strings.Contains(out["message"].(string), "已在站点列表") {
		t.Fatalf("重复目录应被拒绝，实际 %d %v", code, out)
	}
	if got := list(); len(got) != 1 {
		t.Fatalf("重复新建不应新增条目: %v", got)
	}

	// 4. 重命名
	b, _ := json.Marshal(map[string]string{"dir": dir1, "name": "博客"})
	if code, out, _ := call(http.MethodPost, "/__desktop/sites/rename", string(b)); code != http.StatusOK {
		t.Fatalf("重命名失败: %d %v", code, out)
	}
	if got := list()[0].(map[string]any)["name"]; got != "博客" {
		t.Fatalf("重命名未生效: %v", got)
	}

	// 5. 带尾斜杠也能删除，且只移除登记
	if code, out := remove(dir1 + "/"); code != http.StatusOK {
		t.Fatalf("删除失败: %d %v", code, out)
	}
	if got := list(); len(got) != 0 {
		t.Fatalf("删除后列表应为空: %v", got)
	}
	if _, err := filepath.Glob(dir1); err != nil {
		t.Fatal(err)
	}

	// 6. 重复删除报「站点不存在」
	if code, out := remove(dir1); code != http.StatusBadRequest || !strings.Contains(out["message"].(string), "站点不存在") {
		t.Fatalf("重复删除应提示站点不存在，实际 %d %v", code, out)
	}

	// 7. 根目录 / 主目录拒绝
	if code, out := create("/", ""); code != http.StatusBadRequest || !strings.Contains(out["message"].(string), "根目录") {
		t.Fatalf("根目录应被拒绝，实际 %d %v", code, out)
	}
	if code, out := create(home, ""); code != http.StatusBadRequest || !strings.Contains(out["message"].(string), "主目录") {
		t.Fatalf("主目录应被拒绝，实际 %d %v", code, out)
	}

	// 8. ~ 展开
	if code, out := create("~/site2", ""); code != http.StatusOK {
		t.Fatalf("~/ 路径新建失败: %d %v", code, out)
	}
	want := filepath.Join(home, "site2")
	if started != want {
		t.Fatalf("~/ 未展开为绝对路径: 期望 %q 实际 %q", want, started)
	}
}
