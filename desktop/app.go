package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"marksite/internal/bootstrap"
	"marksite/internal/pkg"
)

// handlerBox 在没有打开站点时返回站点管理页，打开站点后切换为真实路由
type handlerBox struct {
	mu  sync.RWMutex
	h   http.Handler
	aux http.Handler
}

func (b *handlerBox) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.RLock()
	h, aux := b.h, b.aux
	b.mu.RUnlock()
	if h != nil {
		h.ServeHTTP(w, r)
		return
	}
	if aux != nil && strings.HasPrefix(r.URL.Path, "/__desktop/") {
		aux.ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"code":50300,"message":"请先打开一个站点"}`)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, sitesPageHTML)
}

func (b *handlerBox) Set(h http.Handler) {
	b.mu.Lock()
	b.h = h
	b.mu.Unlock()
}

// sitesAPI 站点管理页专用的内部接口：此时真实路由尚未就绪，页面用 fetch 调用
type sitesAPI struct {
	d *Desktop
}

func (s *sitesAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 管理页数据每次都要求最新，避免 WebView 缓存
	w.Header().Set("Cache-Control", "no-store")
	type siteForm struct {
		Dir  string `json:"dir"`
		Name string `json:"name"`
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/__desktop/sites":
		// error 用于展示启动时直接打开指定目录失败的原因
		pkg.OK(w, map[string]any{
			"sites": s.d.ListSites(),
			"error": s.d.WorkDirInfo().Error,
		})

	case r.Method == http.MethodPost && r.URL.Path == "/__desktop/pick":
		dir, err := s.d.PickWorkDir()
		if err != nil {
			pkg.WriteErr(w, r, pkg.BadRequest(err.Error()))
			return
		}
		pkg.OK(w, map[string]string{"dir": dir})

	case r.Method == http.MethodPost && r.URL.Path == "/__desktop/sites/open":
		var in siteForm
		if !pkg.BindJSON(w, r, &in) {
			return
		}
		if err := s.d.OpenSite(in.Dir); err != nil {
			pkg.WriteErr(w, r, pkg.BadRequest(err.Error()))
			return
		}
		pkg.OK(w, nil)

	// 新建站点后直接打开，省去再点一次
	case r.Method == http.MethodPost && r.URL.Path == "/__desktop/sites/create":
		var in siteForm
		if !pkg.BindJSON(w, r, &in) {
			return
		}
		if err := s.d.AddSite(in.Dir, in.Name); err != nil {
			pkg.WriteErr(w, r, pkg.BadRequest(err.Error()))
			return
		}
		if err := s.d.OpenSite(in.Dir); err != nil {
			pkg.WriteErr(w, r, pkg.BadRequest(err.Error()))
			return
		}
		pkg.OK(w, nil)

	case r.Method == http.MethodPost && r.URL.Path == "/__desktop/sites/rename":
		var in siteForm
		if !pkg.BindJSON(w, r, &in) {
			return
		}
		if err := s.d.RenameSite(in.Dir, in.Name); err != nil {
			pkg.WriteErr(w, r, pkg.BadRequest(err.Error()))
			return
		}
		pkg.OK(w, nil)

	case r.Method == http.MethodPost && r.URL.Path == "/__desktop/sites/remove":
		var in siteForm
		if !pkg.BindJSON(w, r, &in) {
			return
		}
		if err := s.d.RemoveSite(in.Dir); err != nil {
			pkg.WriteErr(w, r, pkg.BadRequest(err.Error()))
			return
		}
		pkg.OK(w, nil)

	default:
		http.NotFound(w, r)
	}
}

// sitesPageHTML 站点管理页：启动时列出所有站点，可打开、新建、重命名、删除
const sitesPageHTML = `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><title>MarkSite 站点管理</title>
<style>
  html, body { height: 100%; margin: 0; }
  body { font-family: -apple-system, "PingFang SC", sans-serif; background: #f7f9fc; color: #303133; }
  .wrap { max-width: 760px; margin: 0 auto; padding: 44px 24px 56px; box-sizing: border-box; }
  .brand { display: flex; align-items: center; gap: 10px; }
  h1 { font-size: 20px; margin: 0; }
  .sub { color: #909399; font-size: 13px; margin: 8px 0 26px; }
  .section-title { display: flex; align-items: baseline; justify-content: space-between; margin: 0 0 10px; font-size: 13px; color: #909399; }
  .card { background: #fff; border-radius: 14px; box-shadow: 0 18px 44px -28px rgba(16,24,40,.3), 0 2px 6px -3px rgba(16,24,40,.08); overflow: hidden; }
  .site { display: flex; align-items: center; gap: 16px; padding: 15px 18px; border-bottom: 1px solid #f0f2f6; }
  .site:last-child { border-bottom: none; }
  .site-main { flex: 1 1 auto; min-width: 0; }
  .site-name { display: flex; align-items: center; gap: 8px; font-size: 15px; font-weight: 600; color: #1f2937; }
  .name-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .badge { flex: 0 0 auto; padding: 2px 7px; border-radius: 999px; background: #eef3fe; color: #2563eb; font-size: 11px; font-weight: 500; }
  .badge.warn { background: #fef0f0; color: #f56c6c; }
  .site-path { margin-top: 4px; font-family: ui-monospace, Menlo, monospace; font-size: 12px; color: #909399; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .site-actions { display: flex; gap: 8px; flex: 0 0 auto; }
  .rename-row { display: flex; gap: 8px; flex: 0 0 auto; width: 260px; }
  .confirm { display: flex; align-items: center; gap: 8px; font-size: 12px; color: #606266; }
  button { height: 34px; padding: 0 14px; border: 1px solid #dcdfe6; border-radius: 9px; background: #fff; font-size: 13px; color: #303133; cursor: pointer; }
  button:hover:not(:disabled) { border-color: #409eff; color: #409eff; }
  button:disabled { opacity: .55; cursor: default; }
  button.primary { border: none; background: linear-gradient(135deg, #3b82f6, #1d4ed8); color: #fff; }
  button.primary:hover:not(:disabled) { color: #fff; }
  button.danger:hover:not(:disabled) { border-color: #f56c6c; color: #f56c6c; }
  button.ghost { border: none; background: #f5f7fb; }
  .empty { padding: 40px 18px; text-align: center; color: #909399; font-size: 13px; line-height: 1.8; }
  .empty b { display: block; color: #606266; font-size: 14px; margin-bottom: 6px; }
  .create { margin-top: 18px; padding: 20px 18px; }
  .create h2 { font-size: 14px; margin: 0 0 14px; color: #606266; font-weight: 600; }
  .row { display: flex; gap: 8px; margin-bottom: 10px; }
  input { flex: 1 1 auto; min-width: 0; height: 38px; padding: 0 12px; border: 1px solid #dcdfe6; border-radius: 9px; font-size: 13px; color: #303133; background: #fafbfc; }
  input:focus { outline: none; border-color: #409eff; }
  .hint { color: #a8abb2; font-size: 12px; margin: 2px 0 0; }
  .err { min-height: 18px; color: #f56c6c; font-size: 12px; margin: 10px 2px 0; }
</style></head>
<body>
<div class="wrap">
  <div class="brand"><h1>MarkSite 站点管理</h1></div>
  <p class="sub">每个站点是一个独立的工作目录，各自的源数据与生成的静态站点互不影响。</p>

  <div class="section-title"><span>我的站点</span><span id="count"></span></div>
  <div class="card" id="list"></div>

  <div class="card create">
    <h2>新建站点</h2>
    <div class="row">
      <input id="new-dir" placeholder="站点目录，需绝对路径（如 ~/Sites/blog），也可点击选择">
      <button type="button" id="pick">选择…</button>
    </div>
    <div class="row">
      <input id="new-name" placeholder="站点名称（留空则使用目录名）" maxlength="40">
      <button type="button" class="primary" id="create">新建并打开</button>
    </div>
    <p class="hint">站点目录下会自动创建 data/（源数据）与 output/（生成的静态站点）。</p>
    <p class="err" id="err"></p>
  </div>
</div>
<script>
  var sites = [];
  var busy = false;
  var editingDir = null;
  var confirmDir = null;

  var listEl = document.getElementById('list');
  var countEl = document.getElementById('count');
  var errEl = document.getElementById('err');
  var dirEl = document.getElementById('new-dir');
  var nameEl = document.getElementById('new-name');
  var createBtn = document.getElementById('create');

  function esc(v) {
    return String(v == null ? '' : v)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function baseName(dir) {
    var parts = String(dir || '').replace(/[\/\\\\]+$/, '').split(/[\/\\\\]/);
    return parts[parts.length - 1] || '';
  }

  function api(path, body) {
    var opts = body === undefined ? {} : {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    };
    return fetch(path, opts).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (data) {
        if (!res.ok || (data && data.code && data.code !== 0)) {
          throw new Error((data && data.message) || '操作失败');
        }
        return data ? data.data : null;
      });
    });
  }

  function showError(msg) { errEl.textContent = msg || ''; }

  function syncCreate() {
    createBtn.disabled = busy;
  }

  function siteHTML(s) {
    var actions;
    if (confirmDir === s.dir) {
      actions = '<div class="confirm">仅从列表移除，不删除文件' +
        '<button type="button" class="danger" data-action="remove" data-dir="' + esc(s.dir) + '">移除</button>' +
        '<button type="button" class="ghost" data-action="cancel-remove">取消</button></div>';
    } else if (editingDir === s.dir) {
      actions = '<div class="rename-row">' +
        '<input id="rename-input" value="' + esc(s.name) + '" maxlength="40">' +
        '<button type="button" class="primary" data-action="save-rename" data-dir="' + esc(s.dir) + '">保存</button>' +
        '<button type="button" class="ghost" data-action="cancel-rename">取消</button></div>';
    } else {
      actions = '<div class="site-actions">' +
        '<button type="button" class="primary" data-action="open" data-dir="' + esc(s.dir) + '"' + (s.exists ? '' : ' disabled') + '>打开</button>' +
        '<button type="button" data-action="rename" data-dir="' + esc(s.dir) + '">重命名</button>' +
        '<button type="button" class="danger" data-action="ask-remove" data-dir="' + esc(s.dir) + '">删除</button></div>';
    }
    return '<div class="site"><div class="site-main">' +
      '<div class="site-name"><span class="name-text">' + esc(s.name) + '</span>' +
      (s.last ? '<span class="badge">上次打开</span>' : '') +
      (s.exists ? '' : '<span class="badge warn">目录不存在</span>') +
      '</div><div class="site-path">' + esc(s.dir) + '</div></div>' + actions + '</div>';
  }

  function render() {
    countEl.textContent = sites.length ? sites.length + ' 个站点' : '';
    listEl.innerHTML = sites.length
      ? sites.map(siteHTML).join('')
      : '<div class="empty"><b>还没有站点</b>在下方选择或输入一个目录，就可以创建并打开第一个站点。</div>';
    if (busy) {
      var btns = listEl.querySelectorAll('button');
      for (var i = 0; i < btns.length; i++) btns[i].disabled = true;
    }
    if (editingDir) {
      var input = document.getElementById('rename-input');
      if (input) { input.focus(); input.select(); }
    }
    syncCreate();
  }

  function load() {
    return api('/__desktop/sites').then(function (data) {
      sites = (data && data.sites) || [];
      if (data && data.error) showError(data.error);
      render();
    }).catch(function (e) {
      showError(e.message || String(e));
    });
  }

  function openSite(dir) {
    busy = true; showError(''); render();
    api('/__desktop/sites/open', { dir: dir }).then(function () {
      // 成功后后端会重新加载窗口，不再回到这里
    }).catch(function (e) {
      busy = false; showError(e.message || String(e)); render();
    });
  }

  function removeSite(dir) {
    busy = true; showError(''); render();
    api('/__desktop/sites/remove', { dir: dir }).then(function () {
      confirmDir = null; busy = false;
      return load();
    }).catch(function (e) {
      busy = false; showError(e.message || String(e)); render();
    });
  }

  function saveRename(dir) {
    var input = document.getElementById('rename-input');
    var name = input ? input.value.trim() : '';
    if (!name) { showError('站点名称不能为空'); return; }
    busy = true; showError('');
    api('/__desktop/sites/rename', { dir: dir, name: name }).then(function () {
      editingDir = null; busy = false;
      return load();
    }).catch(function (e) {
      busy = false; showError(e.message || String(e)); render();
    });
  }

  listEl.addEventListener('click', function (ev) {
    var el = ev.target && ev.target.closest ? ev.target.closest('[data-action]') : null;
    if (!el || busy) return;
    var action = el.getAttribute('data-action');
    var dir = el.getAttribute('data-dir') || '';
    if (action === 'open') { openSite(dir); return; }
    if (action === 'rename') { editingDir = dir; confirmDir = null; showError(''); render(); return; }
    if (action === 'save-rename') { saveRename(dir); return; }
    if (action === 'cancel-rename') { editingDir = null; showError(''); render(); return; }
    if (action === 'ask-remove') { confirmDir = dir; editingDir = null; showError(''); render(); return; }
    if (action === 'remove') { removeSite(dir); return; }
    if (action === 'cancel-remove') { confirmDir = null; showError(''); render(); }
  });

  document.getElementById('pick').onclick = function () {
    api('/__desktop/pick', {}).then(function (data) {
      if (!data || !data.dir) return;
      dirEl.value = data.dir;
      if (!nameEl.value.trim()) nameEl.value = baseName(data.dir);
      showError('');
    }).catch(function (e) { showError(e.message || String(e)); });
  };

  dirEl.addEventListener('blur', function () {
    if (!nameEl.value.trim() && dirEl.value.trim()) nameEl.value = baseName(dirEl.value);
  });

  // 输入框内回车等同点击「新建并打开」
  function onEnterCreate(ev) {
    if (ev.key === 'Enter') { ev.preventDefault(); createBtn.click(); }
  }
  dirEl.addEventListener('keydown', onEnterCreate);
  nameEl.addEventListener('keydown', onEnterCreate);

  createBtn.onclick = function () {
    var dir = dirEl.value.trim();
    var name = nameEl.value.trim();
    if (!dir) { showError('请先选择或输入站点目录'); return; }
    busy = true; showError('');
    createBtn.textContent = '正在创建…';
    render();
    api('/__desktop/sites/create', { dir: dir, name: name }).then(function () {
      // 成功后后端会重新加载窗口
    }).catch(function (e) {
      busy = false;
      createBtn.textContent = '新建并打开';
      render();
      // 可能是「已登记但打开失败」，刷新列表让新站点可见
      load();
      showError(e.message || String(e));
    });
  };

  load();
</script>
</body>
</html>`

func run() error {
	workFlag := flag.String("workdir", "", "启动时直接打开的站点目录（留空则展示站点管理页）")
	flag.Parse()

	box := &handlerBox{}
	desktop := &Desktop{}

	var (
		rt     *bootstrap.Runtime
		srv    *http.Server
		cancel context.CancelFunc
	)

	// stop 关闭调度、HTTP 服务与数据库，回到站点管理页状态
	stop := func() {
		if cancel != nil {
			cancel()
			cancel = nil
		}
		if srv != nil {
			_ = srv.Shutdown(context.Background())
			srv = nil
		}
		if rt != nil {
			rt.Close()
			rt = nil
		}
		box.Set(nil)
		desktop.setBaseURL("")
	}

	// start 初始化站点目录、启动本地服务并切换路由
	start := func(dir string) error {
		r, err := bootstrap.Init(dir)
		if err != nil {
			return err
		}
		ln, err := r.Listen(desktopListenAddr())
		if err != nil {
			r.Close()
			return err
		}
		s := &http.Server{Handler: r.Handler()}
		ctx, c := context.WithCancel(context.Background())
		r.StartBackupScheduler(ctx)

		box.Set(r.Handler())
		srv, cancel, rt = s, c, r
		go func() {
			if err := s.Serve(ln); err != nil && err != http.ErrServerClosed {
				slog.Error("HTTP 服务退出", "err", err)
			}
		}()
		desktop.setBaseURL("http://" + ln.Addr().String() + "/")
		desktop.setInfo(WorkDirInfo{State: "ready", WorkDir: dir})
		slog.Info("MarkSite 桌面端已启动", "workdir", dir)
		return nil
	}

	desktop.start, desktop.stop = start, stop
	box.aux = &sitesAPI{d: desktop}

	// 默认展示站点管理页；带 -workdir 或 MARKSITE_WORKDIR 时直接打开该站点
	dir := resolveWorkDir(*workFlag)
	if dir != "" {
		// 统一存绝对路径，避免登记进站点列表后因启动位置不同而失效
		if abs, absErr := filepath.Abs(expandHome(dir)); absErr == nil {
			dir = abs
		}
	}
	if dir == "" {
		desktop.setInfo(WorkDirInfo{State: "need_setup"})
	} else if startErr := start(dir); startErr != nil {
		slog.Error("站点目录初始化失败", "dir", dir, "err", startErr)
		desktop.setInfo(WorkDirInfo{
			State: "need_setup",
			Error: fmt.Sprintf("「%s」无法使用：%v", dir, startErr),
		})
	} else if saveErr := rememberSite(dir); saveErr != nil {
		slog.Warn("保存站点列表失败", "err", saveErr)
	}

	app := application.New(application.Options{
		Name:        "MarkSite",
		Description: "单二进制、离线可用的静态站点管理工具",
		Services: []application.Service{
			application.NewService(desktop),
		},
		Assets: application.AssetOptions{
			// 未打开站点时返回站点管理页，打开后复用现有路由：
			// 静态资源、/api、/media、/preview 全部走同一套逻辑
			Handler: box,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.marksite.desktop",
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() {
			stop()
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "main",
		Title:  "MarkSite",
		Width:  1280,
		Height: 860,
		// 侧边目录树 + 编辑器的最小可用尺寸
		MinWidth:         1024,
		MinHeight:        640,
		URL:              "/",
		BackgroundColour: application.NewRGB(247, 249, 252),
	})

	desktop.setApp(app)
	desktop.reload = func() {
		win.Reload()
	}

	// 替换 Wails 默认菜单（File / Edit / View / Window / Help），只保留必要项
	app.Menu.Set(buildAppMenu())

	return app.Run()
}
