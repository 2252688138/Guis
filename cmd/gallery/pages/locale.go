// guis — gallery 的多语言文案与语言切换入口
//
// 顶部菜单的「语言 / Language」、标题栏右侧的语言胶囊、左侧导航的标题与案例名
// 都走 ui.TOr(key, 中文原文)：key 有译文就用译文，没有就原样显示中文，
// 因此可以一层层渐进接入，不会出现「可读化的 key」或大片空白。
//
// 语言包直接内置在 pages 包里（gallery 是示例工程，不依赖外部文件、
// 浏览器 / 安卓同样可用）；业务工程一般放在独立 JSON 里用 go:embed 载入。

package pages

import (
	"strings"

	"guis/ui"
)

// galleryZH / galleryEN gallery 自己的语言包（键统一以 g. 开头）。
const (
	galleryZH = `{
  "g.menu.file": "文件",
  "g.menu.edit": "编辑",
  "g.menu.view": "视图",
  "g.menu.custom": "自定义",
  "g.menu.code": "代码编辑器",
  "g.menu.help": "帮助",
  "g.menu.language": "语言",

  "g.menu.file.open": "打开",
  "g.menu.file.openImage": "图片…",
  "g.menu.file.openVideo": "视频…",
  "g.menu.file.openAudio": "音频…",
  "g.menu.file.newWindow": "新建窗口",
  "g.menu.file.closeWindow": "关闭当前窗口",
  "g.menu.file.exit": "退出",

  "g.menu.edit.copyPage": "复制当前页面名",
  "g.menu.edit.paste": "粘贴",
  "g.menu.edit.find": "查找",
  "g.menu.edit.findInPage": "当前页面内查找…",
  "g.menu.edit.findGlobal": "全局搜索…",

  "g.menu.view.cases": "界面案例",
  "g.menu.view.fullscreen": "全屏切换",
  "g.menu.view.theme": "主题",
  "g.menu.view.themeDark": "深色",
  "g.menu.view.themeLight": "浅色",
  "g.menu.view.themeAuto": "跟随系统",

  "g.menu.custom.demo2": "二级菜单演示",
  "g.menu.custom.direct": "直接执行项",
  "g.menu.custom.demo3": "三级菜单演示",
  "g.menu.custom.itemA": "三级项 A",
  "g.menu.custom.itemB": "三级项 B",
  "g.menu.custom.disabled": "禁用项",
  "g.menu.custom.shortcut": "带快捷键的项",
  "g.menu.custom.current": "当前页面：",
  "g.menu.custom.picked": "自定义菜单项",
  "g.menu.custom.picked2": "二级菜单项",

  "g.menu.code.open": "打开编辑器示例",
  "g.menu.code.stress": "载入十万行压力测试",
  "g.menu.code.lsp": "连接 gopls（LSP）",
  "g.menu.code.format": "格式化文档",
  "g.menu.code.shortcuts": "快捷键一览",

  "g.menu.help.api": "API 文档",
  "g.menu.help.usage": "菜单用法",
  "g.menu.help.nested": "MenuItem.Children 嵌套",
  "g.menu.help.about": "关于 GUIS",

  "g.nav.title": "界面案例",
  "g.nav.subtitle": "%d 个界面骨架 + 布局实验室",
  "g.lang.auto": "跟随系统",
  "g.lang.switch": "切换语言",

  "g.toast.image": "打开图片（示例）",
  "g.toast.copied": "已复制：",
  "g.toast.find": "查找（示例）",
  "g.toast.search": "全局搜索（示例）",
  "g.toast.childOpen": "已打开子窗口",
  "g.toast.childFail": "打开子窗口失败：",
  "g.toast.theme": "主题已切换：",
  "g.toast.lang": "语言已切换：{name}",
  "g.toast.about": "自绘标题栏与多级菜单示例",
  "g.toast.nested": "Children 非空即为子菜单，可多层嵌套",

  "g.child.title": "子窗口",
  "g.child.hint": "从顶部标题栏菜单「文件 → 新建窗口」打开。",

  "g.case.basic": "基础组件",
  "g.case.basic.desc": "按钮 / 输入 / 下拉框 / 多行输入 / 滑块 / 开关 / 导航 / 提示",
  "g.case.form": "表单与弹窗",
  "g.case.form.desc": "表单校验 / 下拉 / 日期 / 上传 / 四类弹窗 / 抽屉",
  "g.case.list": "列表与表格",
  "g.case.list.desc": "文件树 / 图文卡片 / 列表 / 社区帖子 / 分页 / 数据表格（页内页签切换）",
  "g.case.media": "媒体播放",
  "g.case.media.desc": "视频 / 音频 / 频谱 / 播放列表",
  "g.case.layout": "布局与仪表盘",
  "g.case.layout.desc": "Flex / 网格 / 换行 / 瀑布流 / 叠层 / 响应式 / 统计卡",
  "g.case.charts": "图表",
  "g.case.charts.desc": "折线 / 面积 / 柱状 / 饼 / 环形 / 雷达 / 散点",
  "g.case.text": "文本与富文本",
  "g.case.text.desc": "Markdown 渲染 / 富文本 / 代码高亮",
  "g.case.system": "系统操作",
  "g.case.system.desc": "文件与拖入拖出 / 剪贴板 / 热键 / 托盘 / 任务栏 / 窗口特效 / WebView / 打印",
  "g.case.office": "办公文档",
  "g.case.office.desc": "Excel / Word / PDF 读写与渲染 · 打印预览 · 导出 PDF",
  "g.case.icons": "图标库",
  "g.case.icons.desc": "Iconfont 矢量图标：内置 80+ 图标 / 任意尺寸清晰 / 换色与线宽 / 运行时注册自定义图标",
  "g.case.code": "代码编辑器",
  "g.case.code.desc": "十万行虚拟渲染 / 语法高亮 / 行号 + 小地图 / 右键菜单 / LSP 语法解析",
  "g.case.term": "伪终端",
  "g.case.term.desc": "VT/ANSI 终端：256/真彩色 / 备用屏 / 回看 / 选区复制 / 内置 ConPTY 本地 shell"
}`

	galleryEN = `{
  "g.menu.file": "File",
  "g.menu.edit": "Edit",
  "g.menu.view": "View",
  "g.menu.custom": "Custom",
  "g.menu.code": "Editor",
  "g.menu.help": "Help",
  "g.menu.language": "Language",

  "g.menu.file.open": "Open",
  "g.menu.file.openImage": "Image…",
  "g.menu.file.openVideo": "Video…",
  "g.menu.file.openAudio": "Audio…",
  "g.menu.file.newWindow": "New window",
  "g.menu.file.closeWindow": "Close window",
  "g.menu.file.exit": "Quit",

  "g.menu.edit.copyPage": "Copy page name",
  "g.menu.edit.paste": "Paste",
  "g.menu.edit.find": "Find",
  "g.menu.edit.findInPage": "Find in page…",
  "g.menu.edit.findGlobal": "Search everywhere…",

  "g.menu.view.cases": "Examples",
  "g.menu.view.fullscreen": "Toggle fullscreen",
  "g.menu.view.theme": "Theme",
  "g.menu.view.themeDark": "Dark",
  "g.menu.view.themeLight": "Light",
  "g.menu.view.themeAuto": "Follow system",

  "g.menu.custom.demo2": "Nested menu demo",
  "g.menu.custom.direct": "Run directly",
  "g.menu.custom.demo3": "Third-level demo",
  "g.menu.custom.itemA": "Third-level A",
  "g.menu.custom.itemB": "Third-level B",
  "g.menu.custom.disabled": "Disabled item",
  "g.menu.custom.shortcut": "Item with shortcut",
  "g.menu.custom.current": "Current page: ",
  "g.menu.custom.picked": "Custom menu item",
  "g.menu.custom.picked2": "Nested menu item",

  "g.menu.code.open": "Open editor demo",
  "g.menu.code.stress": "Load 100k-line stress test",
  "g.menu.code.lsp": "Connect gopls (LSP)",
  "g.menu.code.format": "Format document",
  "g.menu.code.shortcuts": "Keyboard shortcuts",

  "g.menu.help.api": "API docs",
  "g.menu.help.usage": "Menu usage",
  "g.menu.help.nested": "Nested MenuItem.Children",
  "g.menu.help.about": "About GUIS",

  "g.nav.title": "UI examples",
  "g.nav.subtitle": "%d skeletons + layout lab",
  "g.lang.auto": "Follow system",
  "g.lang.switch": "Switch language",

  "g.toast.image": "Open image (demo)",
  "g.toast.copied": "Copied: ",
  "g.toast.find": "Find (demo)",
  "g.toast.search": "Search everywhere (demo)",
  "g.toast.childOpen": "Child window opened",
  "g.toast.childFail": "Failed to open child window: ",
  "g.toast.theme": "Theme: ",
  "g.toast.lang": "Language: {name}",
  "g.toast.about": "Custom title bar and multi-level menus",
  "g.toast.nested": "Non-empty Children means a submenu; nesting is unlimited",

  "g.child.title": "Child window",
  "g.child.hint": "Opened from the title bar menu: File → New window.",

  "g.case.basic": "Basic components",
  "g.case.basic.desc": "Buttons / inputs / selects / textarea / slider / switch / navigation / hints",
  "g.case.form": "Forms & dialogs",
  "g.case.form.desc": "Validation / select / date / upload / four dialog types / drawer",
  "g.case.list": "Lists & tables",
  "g.case.list.desc": "File tree / media cards / list rows / posts / pager / data table (tabs inside)",
  "g.case.media": "Media",
  "g.case.media.desc": "Video / audio / spectrum / playlist",
  "g.case.layout": "Layout & dashboard",
  "g.case.layout.desc": "Flex / grid / wrap / masonry / stack / responsive / stat cards",
  "g.case.charts": "Charts",
  "g.case.charts.desc": "Line / area / bar / pie / donut / radar / scatter",
  "g.case.text": "Text & rich text",
  "g.case.text.desc": "Markdown rendering / rich text / code highlighting",
  "g.case.system": "System",
  "g.case.system.desc": "Files & drag-drop / clipboard / hotkeys / tray / taskbar / window effects / WebView / print",
  "g.case.office": "Office documents",
  "g.case.office.desc": "Excel / Word / PDF read, render, print preview and export",
  "g.case.icons": "Icons",
  "g.case.icons.desc": "Iconfont vector icons: 80+ built in, crisp at any size, recolor, runtime registration",
  "g.case.code": "Code editor",
  "g.case.code.desc": "100k-line virtualization / syntax highlight / gutter + minimap / context menu / LSP",
  "g.case.term": "Terminal",
  "g.case.term.desc": "VT/ANSI terminal: 256/true color, alt screen, scrollback, selection copy, built-in ConPTY shell"
}`
)

// InitLocales 载入 gallery 自己的语言包（必须在 ui.Run 之前调用一次）。
//
// 载入后由 ui.Run 的启动逻辑（AutoLocale）按「偏好 > 系统语言 > en-US」选语言；
// 想固定语言可在 ui.Options 里设置 Locale。
func InitLocales() {
	_ = ui.LoadCatalog("zh-CN", galleryZH)
	_ = ui.LoadCatalog("en-US", galleryEN)
}

// CaseLabel 案例名（有译文用译文，否则用 Cases 里的中文原文）。
func CaseLabel(c CaseItem) string { return ui.TOr("g.case."+c.ID, c.Label) }

// CaseDescOf 案例描述。
func CaseDescOf(c CaseItem) string { return ui.TOr("g.case."+c.ID+".desc", c.Desc) }

// shortLocaleLabel 语言胶囊上显示的短名（"中文（简体）" → "中文"）。
func shortLocaleLabel(code string) string {
	name := ui.LocaleLabel(code)
	if i := strings.IndexAny(name, "（("); i > 0 {
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = code
	}
	return name
}

// themeLabel 主题名本地化（Theme 变量本身仍保存中文原名，仅展示时翻译）。
func themeLabel(name string) string {
	switch name {
	case "深色":
		return ui.TOr("g.menu.view.themeDark", name)
	case "浅色":
		return ui.TOr("g.menu.view.themeLight", name)
	case "跟随系统", "系统":
		return ui.TOr("g.menu.view.themeAuto", "跟随系统")
	}
	return name
}

// SwitchLanguage 切换界面语言并记住选择（写偏好 ui.locale）。
func SwitchLanguage(code string) {
	if code == "" || code == ui.Locale() {
		return
	}
	ui.UseLocale(code)
	toastShow(ui.Tf("g.toast.lang", map[string]string{"name": ui.LocaleLabel(code)}))
	ui.RequestRedraw()
}

// NextLocale 语言列表里的下一个（标题栏语言胶囊点击时轮换）。
func NextLocale() string {
	list := ui.AvailableLocales()
	if len(list) == 0 {
		return ""
	}
	cur := ui.Locale()
	for i, in := range list {
		if in.Code == cur {
			return list[(i+1)%len(list)].Code
		}
	}
	return list[0].Code
}

// languageMenuItems 「语言 / Language」菜单的子项：每种可用语言一项，当前语言打勾。
func languageMenuItems() []ui.MenuItem {
	list := ui.AvailableLocales()
	out := make([]ui.MenuItem, 0, len(list)+2)
	cur := ui.Locale()
	for _, in := range list {
		code := in.Code
		label := in.NativeName
		if label == "" {
			label = in.Name
		}
		if label == "" {
			label = code
		}
		out = append(out, ui.MenuItem{
			Label:   label,
			Hint:    code,
			Checked: code == cur,
			OnPick:  func() { SwitchLanguage(code) },
		})
	}
	out = append(out, ui.MenuItem{Separator: true})
	// 跟随系统：清掉偏好后重新协商，等价于「下次启动按系统语言」
	out = append(out, ui.MenuItem{
		Label: ui.TOr("g.lang.auto", "跟随系统"),
		OnPick: func() {
			ui.DelPref("ui.locale")
			ui.AutoLocale()
			toastShow(ui.Tf("g.toast.lang", map[string]string{"name": ui.LocaleLabel(ui.Locale())}))
			ui.RequestRedraw()
		},
	})
	return out
}

// localeChip 标题栏右侧的语言胶囊（窄屏隐藏菜单栏时仍可切换语言）。
//
// 点击在可用语言之间轮换；完整列表在顶部菜单「语言」里，带勾选。
func localeChip() *ui.Node {
	cur := ui.Locale()
	label := shortLocaleLabel(cur)
	hover := ui.IsHovered("title-locale")
	bg := ui.C("rgba(255,255,255,0.05)")
	if hover {
		bg = ui.C(ui.ColHover)
	}
	chip := ui.Row(
		ui.IconColored("globe", 13, ui.ColTextMuted),
		ui.Text(label).St(ui.St().Text(11.5, 600, ui.ColText)),
	).St(ui.St().GapOf(5).PadXY(9, 4).R(8).BgC(bg).Stroke(1, ui.ColStroke)).
		AlignTo(ui.AlignCenter).
		K("title-locale").
		Tap(func() { SwitchLanguage(NextLocale()) })
	return ui.TooltipOf(chip, ui.TOr("g.lang.switch", "切换语言")+" · "+ui.LocaleLabel(cur))
}
