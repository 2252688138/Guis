// guis — gallery 页面：Iconfont 图标库
//
// 演示 ui.Icon / ui.IconSized / ui.IconColored / ui.IconStroked / ui.IconLabel /
// ui.IconAction 以及运行时 ui.RegisterIcon 注册自定义矢量图标。
// 图标全部来自内置路径表（ui/iconfont_icons.go），不依赖系统字体或 .ttf 文件。

package pages

import (
	"fmt"

	"guis/ui"
)

// ---- 页面状态 ----

var (
	// iconQuery 图标搜索关键字（按名称 / 中文标签过滤）。
	iconQuery string
	// iconSize 尺寸演示的当前档位（默认 22）。
	iconSize float32 = 22
	// iconStroke 线宽演示的当前档位（设计网格单位）。
	iconStroke float32 = 2
	// iconPicked 最近点击的图标名（顶部状态条显示）。
	iconPicked = "search"
)

// iconPalette 图标网格的循环配色（品牌色系，深色主题下辨识度高）。
var iconPalette = []string{
	"#818cf8", "#22d3ee", "#f472b6", "#34d399",
	"#fbbf24", "#a78bfa", "#fb7185", "#94a3b8",
}

// iconPaletteColor 按序号取循环配色。
func iconPaletteColor(i int) string { return iconPalette[i%len(iconPalette)] }

// iconMatches 关键字过滤（空关键字命中全部）。
func iconMatches(name, label, query string) bool {
	if query == "" {
		return true
	}
	return containsFold(name, query) || containsFold(label, query)
}

// containsFold 忽略 ASCII 大小写的子串匹配（关键字通常很短，直接扫一遍即可）。
func containsFold(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	lower := func(b byte) byte {
		if b >= 'A' && b <= 'Z' {
			return b + ('a' - 'A')
		}
		return b
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		ok := true
		for j := 0; j < len(sub); j++ {
			if lower(s[i+j]) != lower(sub[j]) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// iconFilteredNames 当前关键字下命中的图标名。
func iconFilteredNames() []string {
	all := ui.IconNames()
	out := make([]string, 0, len(all))
	for _, name := range all {
		def, ok := ui.IconDefOf(name)
		label := ""
		if ok {
			label = def.Label
		}
		if iconMatches(name, label, iconQuery) {
			out = append(out, name)
		}
	}
	return out
}

// ensureCustomIcon 注册演示用的自定义图标（已存在则跳过，避免每帧重复注册）。
func ensureCustomIcon() {
	if ui.IconExists("guis-logo") {
		return
	}
	ui.RegisterIcon(ui.IconDef{
		Name:  "guis-logo",
		Label: "品牌徽标",
		Paths: []string{
			"M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z",
			"M15.5 9.5a4 4 0 1 0 0 5h.5v-3h-3",
		},
	})
}

// ---- 页面 ----

// iconsCase Iconfont 图标库整页（自带滚动容器，见 state.go 路由）。
func iconsCase() *ui.Node {
	ensureCustomIcon()
	names := iconFilteredNames()

	return ui.Column(
		pageHead("Iconfont 图标库",
			fmt.Sprintf("%d 个内置矢量图标 · 任意尺寸清晰渲染 · 可换色/调线宽 · 支持运行时注册自定义图标",
				ui.IconCount())),

		// 1. 能力概览：尺寸 / 颜色 / 线宽 / 选中状态
		ui.Card(
			ui.Row(
				ui.Title("尺寸", "同一路径按需光栅化，16px 与 64px 一样锐利"),
				ui.Spacer(),
				ui.Tag(fmt.Sprintf("当前选中：%s", iconPicked), "#a5b4fc"),
			).AlignTo(ui.AlignCenter),
			ui.Row(
				iconSizeCell("plus", 14), iconSizeCell("plus", 18), iconSizeCell("plus", 22),
				iconSizeCell("plus", 28), iconSizeCell("plus", 36), iconSizeCell("plus", 48),
				iconSizeCell("plus", 64),
			).St(ui.St().GapOf(14)).AlignTo(ui.AlignCenter),
			ui.Divider(),
			ui.Row(
				ui.Title("颜色", "纹理是白色遮罩，绘制时着色"),
				ui.Spacer(),
				ui.IconColored("palette", 18, ui.ColInfo),
			).AlignTo(ui.AlignCenter),
			ui.Row(ui.Each(iconPalette, func(color string, i int) *ui.Node {
				return ui.Row(
					ui.IconColored([]string{"heart", "star", "bell", "shield", "leaf", "zap", "tag", "globe"}[i%8], 22, color),
					ui.Text(color).St(ui.St().Text(10.5, 500, ui.ColTextMuted)),
				).St(ui.St().GapOf(7).PadXY(10, 7).R(9).BgC(ui.ColBgSoft)).AlignTo(ui.AlignCenter)
			})...).St(ui.St().GapOf(8)),
			ui.Divider(),
			ui.Row(
				ui.Title("线宽", "24×24 设计网格单位，粗线更适合大图标"),
				ui.Spacer(),
				ui.IconStroked("settings", 18, 2.5),
			).AlignTo(ui.AlignCenter),
			ui.Row(
				iconStrokeCell(1.25), iconStrokeCell(1.5), iconStrokeCell(2),
				iconStrokeCell(2.5), iconStrokeCell(3),
			).St(ui.St().GapOf(16)).AlignTo(ui.AlignCenter),
		),

		// 2. 全部图标：搜索 + 网格
		ui.Card(
			ui.Row(
				ui.Title("全部图标", "点击任意图标可选中（顶部状态条显示名称）"),
				ui.Spacer(),
				ui.Box(
					ui.Input("icon-search", iconQuery, "搜索图标名 / 中文名", func(v string) {
						iconQuery = v
						ui.RequestRedraw()
					}),
				).St(ui.St().W(240)),
			).AlignTo(ui.AlignCenter),
			ui.If(len(names) == 0, func() *ui.Node {
				return ui.Empty("没有匹配的图标", "换个关键字试试，例如 file / 文件 / play")
			}),
			ui.Wrap(ui.Each(names, func(name string, i int) *ui.Node {
				def, _ := ui.IconDefOf(name)
				color := iconPaletteColor(i)
				if iconPicked == name {
					color = ui.ColText
				}
				card := ui.Column(
					ui.IconColored(name, 22, color),
					ui.Text(name).St(ui.St().Text(10.5, 600, ui.ColTextDim)),
					ui.Text(def.Label).St(ui.St().Text(10, 400, ui.ColTextMuted)),
				).St(ui.St().W(104).PadXY(6, 10).GapOf(5).R(11).BgC(ui.ColBgSoft)).AlignTo(ui.AlignCenter)
				if iconPicked == name {
					card.St(ui.St().Stroke(1, "#6366f1"))
				} else {
					card.St(ui.St().Stroke(1, ui.ColLine))
				}
				id := name
				return card.K("icon-cell-" + id).Tap(func() {
					iconPicked = id
					toastShow("已选中图标：" + id)
					ui.RequestRedraw()
				})
			})...).St(ui.St().GapOf(8)),
		),

		// 3. 组合场景：按钮 / 工具栏 / 输入框 / 列表 / 空状态
		ui.Card(
			ui.Title("组合场景", "按钮图文、工具栏、输入框前缀、列表条目、空状态"),
			ui.Row(
				iconTextButton("plus", "新建", "#6366f1", func() { toastShow("新建项目") }),
				iconTextButton("download", "导出", "#0ea5e9", func() { toastShow("导出数据") }),
				iconTextButton("trash", "删除", "#ef4444", func() { toastShow("删除记录") }),
				iconTextButton("share", "分享", "#10b981", func() { toastShow("分享链接") }),
			).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),

			ui.Row(
				ui.Text("工具栏").St(ui.St().Text(11.5, 500, ui.ColTextMuted).W(52)),
				ui.IconAction("search", 18, func() { toastShow("搜索") }),
				ui.IconAction("filter", 18, func() { toastShow("筛选") }),
				ui.IconAction("refresh", 18, func() { toastShow("刷新") }),
				ui.IconAction("settings", 18, func() { toastShow("设置") }),
				ui.IconAction("more-h", 18, func() { toastShow("更多操作") }),
				ui.Spacer(),
				ui.IconColored("cloud", 16, ui.ColTextMuted),
				ui.Text("已同步 · 2 分钟前").St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
			).St(ui.St().GapOf(8)).AlignTo(ui.AlignCenter),

			ui.Row(
				ui.Row(
					ui.IconColored("search", 16, ui.ColTextMuted),
					ui.Box(ui.Input("icon-demo-search", "", "搜索项目 / 文件 / 成员", nil)).Grow(1),
				).St(ui.St().GapOf(8).PadXY(10, 0).W(300).H(38).R(10).
					BgC(ui.ColInput).Stroke(1, ui.ColStroke)).AlignTo(ui.AlignCenter),
				ui.Row(
					ui.IconColored("mail", 16, ui.ColTextMuted),
					ui.Text("tang@example.com").St(ui.St().Text(12.5, 500, ui.ColTextDim)),
					ui.Spacer(),
					ui.IconColored("check-circle", 16, ui.ColSuccess),
				).St(ui.St().GapOf(8).PadXY(12, 9).R(10).W(280).
					BgC(ui.ColBgSoft).Stroke(1, ui.ColStroke)).AlignTo(ui.AlignCenter),
			).St(ui.St().GapOf(12)).AlignTo(ui.AlignCenter),

			ui.Column(
				iconListRow("folder", "设计资源", "128 个文件 · 今天更新", "#6366f1"),
				iconListRow("image", "产品截图.png", "2.4 MB · 1920×1080", "#f472b6"),
				iconListRow("code", "main.go", "12 KB · Go", "#22d3ee"),
				iconListRow("lock", "私密文档.pdf", "需要密码访问", "#fbbf24"),
			).St(ui.St().GapOf(6)),

			ui.Row(
				ui.Empty("暂无通知", "所有消息都已读完了"),
				ui.Column(
					ui.Text("空状态也可以直接用图标拼").St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
					ui.Row(
						ui.IconColored("bell", 28, ui.ColTextMuted),
						ui.IconColored("message", 28, ui.ColPlaceholder),
						ui.IconColored("clock", 28, ui.ColPlaceholder),
					).St(ui.St().GapOf(10)),
				).St(ui.St().GapOf(6)).AlignTo(ui.AlignCenter),
			).St(ui.St().GapOf(20)).AlignTo(ui.AlignCenter),
		),

		// 4. 自定义图标 + API 用法
		ui.Card(
			ui.Row(
				ui.Title("自定义图标", "ui.RegisterIcon 注册 SVG path，立即进入图标库"),
				ui.Spacer(),
				ui.Tag("SVG path · 24×24", "#34d399"),
			).AlignTo(ui.AlignCenter),
			ui.Row(
				ui.Column(
					ui.IconColored("guis-logo", 56, "#8b5cf6"),
					ui.Text("guis-logo").St(ui.St().Text(10.5, 500, ui.ColTextMuted)),
				).St(ui.St().GapOf(6).W(120)).AlignTo(ui.AlignCenter),
				ui.Column(
					ui.Text(`ui.RegisterIcon(ui.IconDef{`).St(codeLine()),
					ui.Text(`    Name: "guis-logo", Label: "品牌徽标",`).St(codeLine()),
					ui.Text(`    Paths: []string{"M5 3h14a2 2 0 0 1 2 2v14…"},`).St(codeLine()),
					ui.Text(`})`).St(codeLine()),
					ui.Text(`ui.IconColored("guis-logo", 56, "#8b5cf6")`).St(codeLine()),
				).Grow(1).St(ui.St().GapOf(2).PadXY(14, 10).R(10).
					BgC("rgba(0,0,0,0.25)").Stroke(1, ui.ColLine)),
			).St(ui.St().GapOf(16)).AlignTo(ui.AlignCenter),
		),
	).St(ui.St().Padding(26).GapOf(16))
}

// codeLine 代码片段的等宽小字样式。
func codeLine() *ui.Style {
	return ui.St().Text(11.5, 400, "#a5b4fc").Font("Consolas")
}

// iconSizeCell 尺寸阶梯的一格：图标 + 尺寸文字。
func iconSizeCell(name string, size float32) *ui.Node {
	return ui.Column(
		ui.IconColored(name, size, ui.ColTextDim),
		ui.Text(fmt.Sprintf("%.0fpx", size)).St(ui.St().Text(10, 400, ui.ColTextMuted)),
	).St(ui.St().GapOf(4).W(60).PadXY(0, 6)).AlignTo(ui.AlignCenter)
}

// iconStrokeCell 线宽阶梯的一格。
func iconStrokeCell(stroke float32) *ui.Node {
	return ui.Column(
		ui.IconStroked("settings", 34, stroke),
		ui.Text(fmt.Sprintf("%.2g", stroke)).St(ui.St().Text(10, 400, ui.ColTextMuted)),
	).St(ui.St().GapOf(4).W(56)).AlignTo(ui.AlignCenter)
}

// iconTextButton 图标 + 文字按钮（渐变底）。
func iconTextButton(name, label, color string, onClick func()) *ui.Node {
	n := ui.Row(
		ui.IconColored(name, 15, "#ffffff"),
		ui.Text(label).St(ui.St().Text(12.5, 600, "#ffffff")),
	).St(ui.St().GapOf(7).PadXY(14, 8).R(9).BgGrad(color, "rgba(255,255,255,0.18)", 90)).
		AlignTo(ui.AlignCenter)
	n.Tap(func() {
		if onClick != nil {
			onClick()
		}
		ui.RequestRedraw()
	})
	return n
}

// iconListRow 带图标的列表条目。
func iconListRow(icon, title, sub, color string) *ui.Node {
	row := ui.Row(
		ui.Stack(
			ui.Box().St(ui.St().AbsFill().R(9).BgC(color).Fade(0.16)),
			ui.IconColored(icon, 17, color).St(ui.St().AbsCenter()),
		).St(ui.St().W(32).H(32)),
		ui.Column(
			ui.Text(title).St(ui.St().Text(13, 500, ui.ColText)),
			ui.Text(sub).St(ui.St().Text(11, 400, ui.ColTextMuted)),
		).Grow(1).St(ui.St().GapOf(2)),
		ui.IconColored("chevron-right", 14, ui.ColPlaceholder),
	).St(ui.St().GapOf(10).PadXY(12, 9).R(10).BgC(ui.ColBgSoft)).
		AlignTo(ui.AlignCenter)
	row.Tap(func() {
		toastShow("打开：" + title)
		ui.RequestRedraw()
	})
	return row
}
