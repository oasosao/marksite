package main

import "github.com/wailsapp/wails/v3/pkg/application"

// buildAppMenu 精简原生菜单：只保留应用菜单、编辑菜单与视图菜单。
// 默认菜单里的 File / Window / Help 已去掉；想增减项时在这里增删 AddRole 即可，
// 可用角色见 application.Role 常量（About、Quit、Reload、ToggleFullscreen……）。
func buildAppMenu() *application.Menu {
	menu := application.NewMenu()

	// macOS 的第一个子菜单即应用菜单，标题会由系统替换为应用名
	appMenu := menu.AddSubmenu("MarkSite")
	appMenu.AddRole(application.About)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Hide)
	appMenu.AddRole(application.HideOthers)
	appMenu.AddRole(application.UnHide)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Quit)

	// 编辑菜单保留系统标准的撤销/剪切/复制/粘贴，输入框快捷键依赖这些菜单项
	menu.AddRole(application.EditMenu)

	// 视图菜单提供重新加载、缩放与全屏；开发版构建还会自动带上「开发者工具」
	menu.AddRole(application.ViewMenu)

	return menu
}
