// guis — gallery 统一壳层：一套 UI 跑桌面 / 浏览器 / 安卓
//
// 三端共用本文件的 View()：同一份「自绘标题栏 + 左侧导航 + 内容区 + 浮层」。
// 终端差异全部收在两条自适应规则里（组件、条目、顺序、文案完全一致）：
//
//	① 平台能力：窗口按钮 / 标题栏拖动 / 全局快捷键只在桌面端生效，
//	   安卓与浏览器上这些调用本身是 no-op，这里顺手把按钮隐藏掉；
//	② 窗口宽度：窄屏（手机竖屏、被拖窄的桌面窗口、窄浏览器）侧栏按断点收窄、
//	   隐藏标题栏里的菜单栏（菜单项在窄屏放不下），宽屏恢复标准宽度。
//
// 入口：三端都调本文件的 View()（桌面 cmd/gallery/main.go → ui.Main → Run），
// 因此改一处布局三端同步生效。

package pages

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"guis/internal/version"
	"guis/ui"
)

const (
	// TitleBarH 自绘标题栏高度（内容区顶部用等高占位撑开）。
	TitleBarH float32 = 40
	// NavBaseW 左侧导航的标准宽度（宽屏）。
	NavBaseW float32 = 232
	// NarrowWidth 窄屏断点（逻辑像素）：小于它按手机 / 窄窗口布局。
	NarrowWidth float32 = 900
)

// IsMobile 当前是否移动端（Android）。
//
// 用 runtime.GOOS 而不是构建标签：本文件要三端同时编译，
// 构建标签会把代码筛掉，运行时判断才能让一套源码覆盖全部终端。
func IsMobile() bool { return runtime.GOOS == "android" }

// IsDesktop 当前是否桌面平台（有系统窗口，可拖动 / 最小化 / 关闭）。
func IsDesktop() bool {
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		return true
	}
	return false
}

// IsNarrow 当前是否窄屏：手机竖屏、拖窄的桌面窗口、窄浏览器都算。
//
// 读的是最近一次布局的视口宽度（ui.ViewportW）：窗口尺寸变化后
// 下一帧自动切到对应形态，不需要自己监听 resize。
func IsNarrow() bool { return ui.ViewportW() < NarrowWidth }

// NavWidth 左侧导航宽度。
//
// 导航条目 / 顺序 / 选中态三端一致，只有宽度随窗口收窄，
// 保证手机竖屏下内容区仍然有可用的宽度。
func NavWidth() float32 {
	switch w := ui.ViewportW(); {
	case w < 600:
		return 150
	case w < NarrowWidth:
		return 176
	default:
		return NavBaseW
	}
}

// navPad 侧栏内边距（窄屏收紧）。
func navPad() float32 {
	if IsNarrow() {
		return 12
	}
	return 18
}

// ---- 全局壳层状态 ----

var (
	// bindGlobalKeysOnce 全局快捷键只需绑定一次（View 每帧都会调用）。
	bindGlobalKeysOnce sync.Once
	// wantFullscreen 桌面启动参数 --fullscreen：首帧后进全屏。
	wantFullscreen bool
	fsApplied      bool
)

// RequestFullscreen 请求首帧后进入全屏（桌面入口 --fullscreen 用）。
func RequestFullscreen() { wantFullscreen = true }

// ---- 构建耗时埋点（GUIS_FPS=1 时输出，定位每帧构建热点）----

var (
	profView = os.Getenv("GUIS_FPS") != ""
	profT    = time.Now()
)

func shellMark(name string) {
	if !profView {
		return
	}
	now := time.Now()
	fmt.Fprintf(os.Stderr, "[view] %-10s %6.2fms\n", name, float64(now.Sub(profT).Microseconds())/1000)
	profT = now
}

// View gallery 的唯一整页入口（桌面 / 浏览器 / 安卓共用）。
func View() *ui.Node {
	profT = time.Now()
	normalizeCurrent()

	// 全局快捷键只绑定一次：菜单里的「全屏切换」提示为 F11，这里真正接上。
	// 移动端 / 浏览器没有全局热键，ui.OnKeyGlobal 在那边是空实现，调用无害。
	bindGlobalKeysOnce.Do(func() {
		ui.OnKeyGlobal(func(k uint32) {
			switch k {
			case ui.VK_F11:
				ToggleFullscreen()
			case 0x70: // VK_F1：打开 API 文档（帮助菜单里提示为 F1）
				SwitchCase("api")
			}
		})
	})
	// --fullscreen：首帧后请求全屏（必须在窗口建好后调用）。
	if wantFullscreen && !fsApplied {
		fsApplied = true
		if a := ui.Current(); a != nil {
			a.SetFullscreen(true)
			// 播放器是懒创建的，可能还是 nil；写之前判空（旧代码直接写 → panic 黑屏）。
			SetVideoFullscreen(true)
			ui.RequestRedraw()
		}
	}

	// API 文档页自带「左侧目录 + 右侧正文」双栏，壳层不再叠加左侧导航，
	// 否则窄屏下会挤出三栏（正文可读宽度被压到 300 以内）。
	var body *ui.Node
	if Current == "api" {
		body = ui.Row(Content()).St(ui.St().GrowTo(1).BgC(ui.ColBg))
	} else {
		body = ui.Row(
			ui.Column(navItems()...).St(
				ui.St().W(NavWidth()).Hp(100).Padding(navPad()).BgC(ui.ColBgSoft).GapOf(4),
			).K("nav"),
			Content(),
		).St(ui.St().GrowTo(1).BgC(ui.ColBg))
	}

	shellMark("body")
	// 标题栏（菜单栏）单独作为最上层浮层：
	// 先用等高占位撑出内容区，再把标题栏放到 Stack 靠后的位置绘制，
	// 这样无论内容怎么滚动/溢出，顶部菜单栏永远压在它们之上（不会被组件遮挡）。
	shell := ui.Column(
		ui.Box().St(ui.St().H(TitleBarH+1)), // 占位：标题栏 + 下方分隔线
		body,
	).St(ui.St().Hp(100).BgC(ui.ColBg))

	titleLayer := titleBar().St(ui.St().AbsTL(0, 0).Wp(100))
	shellMark("titleBar")

	// 播放器全屏：在同一窗口根 Stack 里铺满，不改系统窗口（见 case_video.go）
	videoLayer := VideoLayer()
	menuPanelLayer := ui.If(CtrlMenu, func() *ui.Node {
		return ui.MenuPanel(690, 150, []ui.MenuItem{
			{Label: "新建文件", Hint: "Ctrl+N", OnPick: func() { CtrlMenu = false; Toast("新建文件") }},
			{Label: "打开项目", Hint: "Ctrl+O", OnPick: func() { CtrlMenu = false; Toast("打开项目") }},
			{Label: "导出为", Children: []ui.MenuItem{
				{Label: "PNG 图片", OnPick: func() { CtrlMenu = false; Toast("已导出 PNG") }},
				{Label: "PDF 文档", Hint: "Beta", OnPick: func() { CtrlMenu = false; Toast("已导出 PDF") }},
			}},
			{Separator: true},
			{Label: "更多设置…", Disabled: true},
		}, 180)
	})
	shellMark("panels")

	// 顶部标题栏菜单的下拉浮层（含多级子菜单）
	menuLayer := ui.MenuLayer(TitleMenu, titleMenus(), titleMenuX(), TitleBarH)
	shellMark("menuLayer")
	tipLayer := ui.TooltipLayer()
	selectLayer := ui.SelectLayers()
	dateLayer := ui.DateLayers()
	// 表单案例的弹窗：Alert / Confirm / Prompt / 自定义 Modal / Drawer
	formLayer := FormOverlays()
	// Markdown 图片放大层（点击 markdown 中的图片后全屏预览）
	mdLayer := ui.MarkdownLightbox()
	// 组件套件页浮层（命令面板 / 自动完成 / 时间 / 级联 / 取色 / 引导 / 通知）
	suiteLayer := SuiteLayers()
	toastLayer := ui.ToastLayer()
	shellMark("layers")

	// 浮层顺序：后声明的画在上层，三端保持一致，
	// 否则手机上的弹窗 / 下拉 / 轻提示会被内容区压住。
	return ui.Stack(shell, titleLayer, videoLayer, menuPanelLayer,
		menuLayer, tipLayer,
		selectLayer, dateLayer, formLayer, mdLayer, suiteLayer, toastLayer,
	).St(ui.St().Hp(100))
}

// ---- 左侧导航（三端同一份：条目 / 顺序 / 选中态完全一致）----

// navItems 左侧导航条目：标题 + 说明 + 各分类入口 + 底部当前页描述。
func navItems() []*ui.Node {
	nav := []*ui.Node{
		ui.Text(ui.TOr("g.nav.title", "界面案例")).St(ui.St().Text(19, 700, "#e8ecff")),
		ui.Text(fmt.Sprintf(ui.TOr("g.nav.subtitle", "%d 个界面骨架 + 布局实验室"), len(Cases))).
			St(ui.St().Text(11.5, 400, ui.ColTextMuted).PadOnly(0, 0, 18, 0).WrapOn()),
	}

	itemW := NavWidth() - navPad()*2
	for i, c := range Cases {
		id := c.ID
		labelColor := ui.ColTextDim
		if Current == id {
			labelColor = ui.ColText
		}
		// 用色块代替字符图标：避免字体缺字形导致列表观感异常
		dotColor := [...]string{"#818cf8", "#22d3ee", "#f472b6", "#34d399", "#fbbf24"}[i%5]
		row := ui.Row(
			ui.Box().St(ui.St().W(6).H(6).R(2).BgC(dotColor)),
			ui.Text(CaseLabel(c)).St(ui.St().Text(13, 500, labelColor)),
		).St(ui.St().GapOf(10).PadXY(12, 9).R(9).W(itemW))
		if Current == id {
			row.St(ui.St().BgC(ui.ColBgSoft).Stroke(1, ui.ColStroke))
		}
		nav = append(nav, row.K("case-"+id).Tap(func() {
			SwitchCase(id)
		}).AlignTo(ui.AlignCenter))
	}

	// 底部描述：窄屏高度紧张，留白也比空文案好看，但保持条目存在（三端结构一致）。
	return append(nav, ui.Spacer(),
		ui.Text(CaseDesc()).St(ui.St().Text(11, 400, ui.ColTextMuted).WrapOn()))
}

// ---- 顶部标题栏（自绘：品牌 + 菜单栏 + 窗口按钮 + 拖动）----

// brandWidth 品牌区宽度（与 Row 内实际布局一致，用于菜单浮层定位）。
func brandWidth() float32 { return 18 + 8 + ui.TextWidth("GUIS", 12.5, 600) }

// titleMenuX 菜单栏左边缘的窗口坐标（标题栏左内边距 10 + 品牌区 + 间距 14）。
func titleMenuX() float32 { return 10 + brandWidth() + 14 }

func titleBar() *ui.Node {
	// dragTitle 按下标题栏空白/品牌区时交给系统拖动窗口（移动端为 no-op）
	dragTitle := func() {
		TitleMenu.Close()
		if a := ui.Current(); a != nil {
			a.BeginDragMove()
		}
	}

	brand := ui.Row(
		ui.Box().St(ui.St().W(18).H(18).R(5).BgGrad("#6366f1", "#8b5cf6", 120)),
		ui.Text("GUIS").St(ui.St().Text(12.5, 600, ui.ColText)),
	).St(ui.St().GapOf(8)).AlignTo(ui.AlignCenter)
	// 品牌区也可以拖窗口（里面没有可交互控件）
	brand.OnPressEvt(dragTitle)

	bar := ui.Row(
		brand,
		// 菜单栏：窄屏放不下（手机竖屏 / 拖窄的浏览器窗口），隐藏；
		// 页面切换走左侧导航，功能不缺。
		ui.If(!IsNarrow(), func() *ui.Node {
			return ui.MenuBar(TitleMenu, titleMenus())
		}),
		// 拖拽区：只有标题栏空白处才交给系统拖动窗口。
		// 以前是把 OnPress 挂在整条 Column 上，按菜单/按钮时一旦命中的是空白或间隙
		// 就会进入系统移动模态循环（SC_MOVE），Windows 会在窗口四周画一圈拖动边标，
		// 按住不放就一直存在，松开才消失——这正是“点顶部菜单冒出一圈边标”的来源。
		// 注意：无子节点的 Box 高度会塔陷为 0，必须显式给高，否则命中不到、拖不动窗口。
		ui.Box().St(ui.St().GrowTo(1).H(TitleBarH)).OnPressEvt(dragTitle),
		// 语言切换胶囊：窄屏会隐藏菜单栏，这里始终可用（点击轮换语言）。
		// 放在最小化 / 最大化 / 关闭之前，紧挨着右侧窗口按钮。
		localeChip(),
		// 窗口按钮只有桌面端有意义（移动端全屏、浏览器没有系统窗口）。
		ui.If(IsDesktop(), func() *ui.Node { return ui.WindowButtons() }),
	).St(ui.St().H(TitleBarH).PadOnly(0, 10, 0, 8).GapOf(14).BgC(ui.ColBgSoft)).
		AlignTo(ui.AlignCenter)

	return ui.Column(bar, ui.Divider()).K("titlebar")
}

func titleMenus() []ui.MenuDef {
	return []ui.MenuDef{
		{Label: ui.TOr("g.menu.file", "文件"), Items: []ui.MenuItem{
			{Label: ui.TOr("g.menu.file.open", "打开"), Children: []ui.MenuItem{
				{Label: ui.TOr("g.menu.file.openImage", "图片…"), OnPick: func() { Toast(ui.T("g.toast.image")) }},
				{Label: ui.TOr("g.menu.file.openVideo", "视频…"), OnPick: func() { SwitchCase("video") }},
				{Label: ui.TOr("g.menu.file.openAudio", "音频…"), OnPick: func() { SwitchCase("music") }},
			}},
			{Label: ui.TOr("g.menu.file.newWindow", "新建窗口"), Hint: "Ctrl+N", OnPick: openChildWindow},
			{Label: ui.TOr("g.menu.file.closeWindow", "关闭当前窗口"), Hint: "Alt+F4", OnPick: closeWindow},
			{Separator: true},
			{Label: ui.TOr("g.menu.file.exit", "退出"), OnPick: closeWindow},
		}},
		{Label: ui.TOr("g.menu.edit", "编辑"), Items: []ui.MenuItem{
			{Label: ui.TOr("g.menu.edit.copyPage", "复制当前页面名"), Hint: "Ctrl+C", OnPick: func() { Toast(ui.T("g.toast.copied") + Current) }},
			{Label: ui.TOr("g.menu.edit.paste", "粘贴"), Hint: "Ctrl+V", Disabled: true},
			{Separator: true},
			{Label: ui.TOr("g.menu.edit.find", "查找"), Children: []ui.MenuItem{
				{Label: ui.TOr("g.menu.edit.findInPage", "当前页面内查找…"), Hint: "Ctrl+F", OnPick: func() { Toast(ui.T("g.toast.find")) }},
				{Label: ui.TOr("g.menu.edit.findGlobal", "全局搜索…"), Hint: "Ctrl+Shift+F", OnPick: func() { Toast(ui.T("g.toast.search")) }},
			}},
		}},
		{Label: ui.TOr("g.menu.view", "视图"), Items: []ui.MenuItem{
			{Label: ui.TOr("g.menu.view.cases", "界面案例"), Children: caseMenuItems()},
			{Separator: true},
			{Label: ui.TOr("g.menu.view.fullscreen", "全屏切换"), Hint: "F11", OnPick: ToggleFullscreen},
			{Label: ui.TOr("g.menu.view.theme", "主题"), Children: []ui.MenuItem{
				{Label: ui.TOr("g.menu.view.themeDark", "深色"), Checked: Theme == "深色", OnPick: func() { SetTheme("深色") }},
				{Label: ui.TOr("g.menu.view.themeLight", "浅色"), Checked: Theme == "浅色", OnPick: func() { SetTheme("浅色") }},
				{Label: ui.TOr("g.menu.view.themeAuto", "跟随系统"), Checked: Theme == "跟随系统", OnPick: func() { SetTheme("跟随系统") }},
			}},
		}},
		{Label: ui.TOr("g.menu.custom", "自定义"), Items: []ui.MenuItem{
			{Label: ui.TOr("g.menu.custom.demo2", "二级菜单演示"), Children: []ui.MenuItem{
				{Label: ui.TOr("g.menu.custom.direct", "直接执行项"), Hint: "Enter", OnPick: func() { Toast(ui.T("g.menu.custom.picked2")) }},
				{Label: ui.TOr("g.menu.custom.demo3", "三级菜单演示"), Children: []ui.MenuItem{
					{Label: ui.TOr("g.menu.custom.itemA", "三级项 A"), OnPick: func() { Toast(ui.T("g.menu.custom.picked") + " A") }},
					{Label: ui.TOr("g.menu.custom.itemB", "三级项 B"), OnPick: func() { Toast(ui.T("g.menu.custom.picked") + " B") }},
					{Separator: true},
					{Label: ui.TOr("g.menu.custom.disabled", "禁用项"), Disabled: true},
				}},
				{Separator: true},
				{Label: ui.TOr("g.menu.custom.shortcut", "带快捷键的项"), Hint: "Ctrl+K", OnPick: func() { Toast(ui.T("g.menu.custom.picked")) }},
			}},
			{Separator: true},
			{Label: ui.T("g.menu.custom.current") + Current, Disabled: true},
		}},
		{Label: ui.TOr("g.menu.code", "代码编辑器"), Items: []ui.MenuItem{
			{Label: ui.TOr("g.menu.code.open", "打开编辑器示例"), OnPick: func() { SwitchCase("code") }},
			{Label: ui.TOr("g.menu.code.stress", "载入十万行压力测试"), OnPick: LoadCodeStress},
			{Separator: true},
			{Label: ui.TOr("g.menu.code.lsp", "连接 gopls（LSP）"), OnPick: ToggleCodeLSP},
			{Label: ui.TOr("g.menu.code.format", "格式化文档"), OnPick: FormatCode},
			{Separator: true},
			{Label: ui.TOr("g.menu.code.shortcuts", "快捷键一览"), OnPick: func() {
				SwitchCase("code")
				codeHelp = true
			}},
		}},
		{Label: ui.TOr("g.menu.help", "帮助"), Items: []ui.MenuItem{
			// API 文档：整页 Markdown 手册（正文见 docs/api，左目录右正文）
			{Label: ui.TOr("g.menu.help.api", "API 文档"), Hint: "F1", OnPick: func() { SwitchCase("api") }},
			{Separator: true},
			{Label: ui.TOr("g.menu.help.usage", "菜单用法"), Children: []ui.MenuItem{
				{Label: "MenuBar / MenuLayer", OnPick: func() { SwitchCase("controls") }},
				{Label: ui.TOr("g.menu.help.nested", "MenuItem.Children 嵌套"), OnPick: func() { Toast(ui.T("g.toast.nested")) }},
			}},
			{Label: ui.TOr("g.menu.help.about", "关于 GUIS"), OnPick: func() { Toast(version.Full() + " · " + ui.T("g.toast.about")) }},
		}},
		// 语言：每种可用语言一项（当前语言打勾），最后一项「跟随系统」
		{Label: ui.TOr("g.menu.language", "语言"), Items: languageMenuItems()},
	}
}

// caseMenuItems 把示例列表映射成菜单项（演示动态生成的子菜单）。
func caseMenuItems() []ui.MenuItem {
	out := make([]ui.MenuItem, 0, len(Cases))
	for _, c := range Cases {
		id := c.ID
		out = append(out, ui.MenuItem{
			Label:   CaseLabel(c),
			Checked: Current == id,
			OnPick:  func() { SwitchCase(id) },
		})
	}
	return out
}

// ToggleFullscreen 全屏切换（菜单 F11 / 播放器全屏共用）。
func ToggleFullscreen() {
	a := ui.Current()
	if a == nil {
		return
	}
	on := !a.IsFullscreen()
	// 播放器懒创建：未打开过视频时为 nil，直接赋值会 panic（曾导致「全屏即黑屏」）
	SetVideoFullscreen(on)
	a.SetFullscreen(on)
	ui.RequestRedraw()
}

func closeWindow() {
	if a := ui.Current(); a != nil {
		a.Close()
	}
}

func openChildWindow() {
	if _, err := ui.NewWindow(ui.Options{Title: ui.T("g.child.title") + " · GUIS", Width: 520, Height: 360}, simpleChildView); err != nil {
		Toast(ui.T("g.toast.childFail") + err.Error())
		return
	}
	Toast(ui.T("g.toast.childOpen"))
}

func simpleChildView() *ui.Node {
	return ui.Center(ui.Column(
		ui.Text(ui.T("g.child.title")).St(ui.St().Text(20, 700, ui.ColText)),
		ui.Text(ui.T("g.child.hint")).St(ui.St().Text(12, 400, ui.ColTextMuted).WrapOn()),
	).St(ui.St().GapOf(8)))
}
