// MarkSite 桌面客户端入口：用 Wails 承载内嵌前端，进程内直接复用 HTTP 路由。
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"marksite/internal/bootstrap"
)

// WorkDirInfo 当前站点的工作目录状态，供设置页展示
type WorkDirInfo struct {
	State     string `json:"state"` // need_setup：未打开站点；ready：已就绪
	WorkDir   string `json:"work_dir"`
	DataDir   string `json:"data_dir"`
	OutputDir string `json:"output_dir"`
	Error     string `json:"error"`
}

// SiteInfo 站点注册信息：每个站点对应一个工作目录
type SiteInfo struct {
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	Exists bool   `json:"exists"`
	Last   bool   `json:"last"` // 上次打开的站点
}

// Desktop 暴露给前端调用的桌面能力（保存文件、系统浏览器、站点列表）
type Desktop struct {
	mu      sync.Mutex
	app     *application.App
	baseURL string
	info    WorkDirInfo

	// life 串行化站点切换与返回列表，避免并发点击时重复启停服务
	life sync.Mutex

	// 由 run 注入的服务生命周期操作
	start  func(dir string) error
	stop   func()
	reload func()
}

// settingsMu 串行化站点配置的读改写，避免并发请求互相覆盖
var settingsMu sync.Mutex

func (d *Desktop) setApp(app *application.App) {
	d.mu.Lock()
	d.app = app
	d.mu.Unlock()
}

func (d *Desktop) getApp() *application.App {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.app
}

func (d *Desktop) setBaseURL(u string) {
	d.mu.Lock()
	d.baseURL = u
	d.mu.Unlock()
}

func (d *Desktop) setInfo(info WorkDirInfo) {
	d.mu.Lock()
	d.info = info
	d.mu.Unlock()
}

// SaveFile 弹出系统保存对话框并写入前端传来的 base64 内容，返回保存路径
func (d *Desktop) SaveFile(filename string, base64Data string) (string, error) {
	app := d.getApp()
	if app == nil {
		return "", fmt.Errorf("桌面环境尚未就绪")
	}
	path, err := app.Dialog.SaveFile().
		SetMessage("保存文件").
		SetFilename(filename).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// OpenInBrowser 在系统浏览器打开本地服务下的指定路径（如站点预览）
func (d *Desktop) OpenInBrowser(path string) {
	d.mu.Lock()
	app, base := d.app, d.baseURL
	d.mu.Unlock()
	if app == nil || base == "" {
		return
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if err := app.Browser.OpenURL(base + path); err != nil {
		slog.Warn("打开系统浏览器失败", "err", err)
	}
}

// WorkDirInfo 返回当前工作目录与初始化状态
func (d *Desktop) WorkDirInfo() WorkDirInfo {
	d.mu.Lock()
	info := d.info
	d.mu.Unlock()
	if info.WorkDir != "" {
		paths := bootstrap.PathsFor(info.WorkDir)
		info.DataDir, info.OutputDir = paths.DataDir, paths.OutputDir
	}
	return info
}

// ListSites 返回已注册的站点列表（附带目录是否可用、是否为上次打开的站点）
func (d *Desktop) ListSites() []SiteInfo {
	settingsMu.Lock()
	st := loadSettings()
	settingsMu.Unlock()
	list := make([]SiteInfo, 0, len(st.Sites))
	for _, s := range st.Sites {
		// 只有目录存在才算可用；同名文件或无法访问都视为不可打开
		fi, err := os.Stat(s.Dir)
		list = append(list, SiteInfo{
			Name:   s.Name,
			Dir:    s.Dir,
			Exists: err == nil && fi.IsDir(),
			Last:   s.Dir == st.LastSite,
		})
	}
	return list
}

// AddSite 新建站点：校验并创建目录后登记到站点列表
func (d *Desktop) AddSite(dir, name string) error {
	abs, err := normalizeSiteDir(dir)
	if err != nil {
		return err
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	st := loadSettings()
	for _, s := range st.Sites {
		if s.Dir == abs {
			return fmt.Errorf("该目录已在站点列表中，可直接点击「打开」")
		}
	}
	st.Sites = append(st.Sites, siteEntry{Name: siteName(name, abs), Dir: abs})
	return saveSettings(st)
}

// RenameSite 修改站点名称（只改列表显示名，不影响目录）
func (d *Desktop) RenameSite(dir, name string) error {
	dir = cleanSiteDir(dir)
	if dir == "" {
		return fmt.Errorf("站点不存在")
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	st := loadSettings()
	for i, s := range st.Sites {
		if s.Dir == dir {
			st.Sites[i].Name = siteName(name, dir)
			return saveSettings(st)
		}
	}
	return fmt.Errorf("站点不存在")
}

// RemoveSite 从站点列表移除（不删除磁盘上的目录与数据）
func (d *Desktop) RemoveSite(dir string) error {
	dir = cleanSiteDir(dir)
	if dir == "" {
		return fmt.Errorf("站点不存在")
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	st := loadSettings()
	sites := make([]siteEntry, 0, len(st.Sites))
	found := false
	for _, s := range st.Sites {
		if s.Dir == dir {
			found = true
			continue
		}
		sites = append(sites, s)
	}
	if !found {
		return fmt.Errorf("站点不存在")
	}
	st.Sites = sites
	if st.LastSite == dir {
		st.LastSite = ""
	}
	return saveSettings(st)
}

// OpenSite 打开指定站点；成功后该目录会自动登记到站点列表
func (d *Desktop) OpenSite(dir string) error {
	abs, err := normalizeSiteDir(dir)
	if err != nil {
		return err
	}
	return d.SetWorkDir(abs)
}

// ShowSites 关闭当前站点服务并回到站点管理页（窗口会重新加载）
func (d *Desktop) ShowSites() error {
	d.life.Lock()
	defer d.life.Unlock()
	d.mu.Lock()
	stop, reload := d.stop, d.reload
	d.mu.Unlock()
	if stop == nil {
		return fmt.Errorf("桌面环境尚未就绪")
	}
	stop()
	d.setInfo(WorkDirInfo{State: "need_setup"})
	if reload != nil {
		go func() {
			time.Sleep(300 * time.Millisecond)
			reload()
		}()
	}
	return nil
}

// PickWorkDir 弹出目录选择对话框，返回用户选定的工作目录（取消时为空）
func (d *Desktop) PickWorkDir() (string, error) {
	app := d.getApp()
	if app == nil {
		return "", fmt.Errorf("桌面环境尚未就绪")
	}
	d.mu.Lock()
	cur := d.info.WorkDir
	d.mu.Unlock()

	def := cur
	if def == "" {
		// 站点管理页没有当前站点，沿用上次打开的目录，用户更容易找到自己的站点
		def = loadSettings().LastSite
	}
	if def == "" {
		def = suggestedWorkDir()
	}
	// 对话框要求初始目录必须存在，向上找到最近一个已存在的目录，避免为此创建新目录
	for def != "" && def != string(filepath.Separator) {
		if st, err := os.Stat(def); err == nil && st.IsDir() {
			break
		}
		def = filepath.Dir(def)
	}
	return app.Dialog.OpenFile().
		SetTitle("选择工作目录").
		SetDirectory(def).
		CanChooseFiles(false).
		CanChooseDirectories(true).
		PromptForSingleSelection()
}

// SetWorkDir 切换工作目录：停止当前服务 → 初始化新目录 → 保存偏好 → 重新加载界面
func (d *Desktop) SetWorkDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("请先选择工作目录")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("工作目录无效: %v", err)
	}
	d.life.Lock()
	defer d.life.Unlock()
	d.mu.Lock()
	start, stop, reload := d.start, d.stop, d.reload
	prev, prevState := d.info.WorkDir, d.info.State
	d.mu.Unlock()
	if start == nil {
		return fmt.Errorf("桌面环境尚未就绪")
	}
	// 目录未变且服务已就绪时无需重启
	if abs == prev && prevState == "ready" {
		return nil
	}

	if stop != nil {
		stop()
	}
	if startErr := start(abs); startErr != nil {
		if prev != "" && prevState == "ready" {
			// 新目录不可用时恢复原目录，避免应用停留在不可用状态
			if restoreErr := start(prev); restoreErr != nil {
				slog.Error("恢复原工作目录失败", "dir", prev, "err", restoreErr)
			}
		} else {
			d.setInfo(WorkDirInfo{State: "need_setup", WorkDir: abs, Error: startErr.Error()})
		}
		return startErr
	}
	// 成功切换后登记站点并记住最近打开的一个
	if err := rememberSite(abs); err != nil {
		slog.Warn("保存桌面设置失败", "err", err)
	}
	if reload != nil {
		// 稍等片刻再刷新，保证本次调用的响应能先回到前端
		go func() {
			time.Sleep(300 * time.Millisecond)
			reload()
		}()
	}
	return nil
}

// siteEntry 站点列表中的一项
type siteEntry struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

// desktopSettings 桌面端本地偏好：站点列表与最近打开的站点（固定放在系统配置目录）
type desktopSettings struct {
	Sites    []siteEntry `json:"sites"`
	LastSite string      `json:"last_site"`
	WorkDir  string      `json:"work_dir,omitempty"` // 旧版单目录配置，读取时迁移到站点列表
}

// expandHome 展开 ~ 与 ~/xxx，避免把 "~" 当成字面目录名
func expandHome(dir string) string {
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, dir[1:])
		}
	}
	return dir
}

// cleanSiteDir 归一化站点路径：去空白、展开 ~、清理多余的 / 与 ..
// 站点列表中保存的路径统一经过这里，保证比较与展示一致
func cleanSiteDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	return filepath.Clean(expandHome(dir))
}

// normalizeSiteDir 校验站点目录：要求绝对路径（支持 ~），不存在时创建
func normalizeSiteDir(dir string) (string, error) {
	dir = cleanSiteDir(dir)
	if dir == "" {
		return "", fmt.Errorf("请先选择站点目录")
	}
	// 相对路径的基准目录取决于应用启动位置，容易建到意料之外的位置，直接拒绝
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("请填写绝对路径，例如 /Users/you/Sites/blog")
	}
	abs := dir
	if filepath.Dir(abs) == abs {
		return "", fmt.Errorf("不能使用磁盘根目录作为站点目录")
	}
	if home, err := os.UserHomeDir(); err == nil && abs == home {
		return "", fmt.Errorf("不能使用用户主目录作为站点目录")
	}
	st, statErr := os.Stat(abs)
	switch {
	case statErr == nil:
		if !st.IsDir() {
			return "", fmt.Errorf("该路径不是目录")
		}
	case os.IsNotExist(statErr):
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return "", fmt.Errorf("创建目录失败: %v", err)
		}
	default:
		return "", fmt.Errorf("目录不可用: %v", statErr)
	}
	return abs, nil
}

// siteName 名称留空时使用目录名
func siteName(name, dir string) string {
	if name = strings.TrimSpace(name); name == "" {
		return filepath.Base(dir)
	}
	return name
}

// rememberSite 成功打开站点后登记并记录为最近打开
func rememberSite(dir string) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	st := loadSettings()
	found := false
	for _, s := range st.Sites {
		if s.Dir == dir {
			found = true
			break
		}
	}
	if !found {
		st.Sites = append(st.Sites, siteEntry{Name: siteName("", dir), Dir: dir})
	}
	st.LastSite = dir
	return saveSettings(st)
}

func settingsPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "MarkSite", "desktop.json"), nil
}

func loadSettings() desktopSettings {
	var s desktopSettings
	path, err := settingsPath()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)

	// 旧版只记录了一个工作目录，迁移为站点列表中的一项
	if s.WorkDir != "" {
		dir := s.WorkDir
		registered := false
		for _, site := range s.Sites {
			if site.Dir == dir {
				registered = true
				break
			}
		}
		if !registered {
			s.Sites = append(s.Sites, siteEntry{Name: siteName("", dir), Dir: dir})
		}
		if s.LastSite == "" {
			s.LastSite = dir
		}
		s.WorkDir = ""
	}

	// 归一化历史配置里的路径，避免尾随 / 或 ./ 造成重复条目、删除时匹配不到
	seen := make(map[string]bool, len(s.Sites))
	sites := make([]siteEntry, 0, len(s.Sites))
	for _, site := range s.Sites {
		dir := cleanSiteDir(site.Dir)
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		sites = append(sites, siteEntry{Name: site.Name, Dir: dir})
	}
	s.Sites = sites
	s.LastSite = cleanSiteDir(s.LastSite)
	return s
}

func saveSettings(s desktopSettings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	// 先写临时文件再改名，避免读取方看到只写了一半的配置
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// suggestedWorkDir 目录选择框的默认位置
func suggestedWorkDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Documents", "MarkSite")
}

// resolveWorkDir 只识别 -workdir 参数与环境变量；默认返回空串，启动后展示站点管理页
func resolveWorkDir(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("MARKSITE_WORKDIR")
}

func main() {
	if err := run(); err != nil {
		slog.Error("桌面端启动失败", "err", err)
		os.Exit(1)
	}
}
