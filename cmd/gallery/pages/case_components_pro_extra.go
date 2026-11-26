// guis — gallery 页面拆分：迷你趋势图 / JSON 查看器 / 就地确认

package pages

import (
	"strconv"

	"guis/ui"
)

// ---- 迷你趋势图 ----

func kpiCard(title, value, delta string, series []float64, up bool) *ui.Node {
	color := ui.ColDanger
	if up {
		color = ui.ColSuccess
	}
	return ui.Column(
		ui.Text(title).St(ui.St().Text(11.5, 500, ui.ColTextMuted)),
		ui.Row(
			ui.Text(value).St(ui.St().Text(20, 700, ui.ColText)),
			ui.Spacer(),
			ui.Text(delta).St(ui.St().Text(11, 600, color)),
		).St(ui.St().PadOnly(4, 0, 6, 0)).AlignTo(ui.AlignEnd),
		ui.Sparkline(series, ui.SparkOptions{Width: 150, Height: 34, HighlightLast: true, Baseline: true}),
	).St(ui.St().W(190).Padding(14).R(12).BgC(ui.ColPanel).Stroke(1, ui.ColStroke).GapOf(2))
}

func sparklineSection() *ui.Node {
	return ui.Column(
		pageHead("迷你趋势图", "KPI 卡片内嵌折柱，不占图表管线；可高亮末柱、可选基线"),
		ui.Row(
			kpiCard("请求量", "24.6k", "+12%", []float64{3, 5, 4, 7, 6, 9, 8, 11}, true),
			kpiCard("平均延迟", "86ms", "-8%", []float64{9, 8, 9, 6, 7, 5, 6, 4}, false),
			kpiCard("错误率", "0.4%", "+0.1%", []float64{2, 1, 2, 3, 2, 4, 3, 5}, true),
		).St(ui.St().GapOf(12)),
	).St(ui.St().Padding(20).GapOf(12))
}

// ---- JSON 查看器 ----

func jsonDemoValue() map[string]any {
	return map[string]any{
		"name":    "guis",
		"version": float64(0.9),
		"stable":  true,
		"modules": []any{"ui", "game", "httpc", "agent"},
		"limits": map[string]any{
			"maxWindows": float64(4),
			"maxFPS":     float64(240),
			"gpu":        map[string]any{"backend": "WGL", "vsync": true},
		},
		"changelog": "新增树形表格 / 穿梭框 / 无限滚动 / 迷你趋势图 / JSON 查看器",
	}
}

func jsonViewerSection() *ui.Node {
	return ui.Column(
		pageHead("JSON 查看器", "任意值转可折叠树表，键名排序稳定；适合调试面板与接口预览"),
		ui.JsonViewer(jsonDemoValue(), ui.JSONOptions{
			Key:         "gallery-json",
			RootLabel:   "response",
			DefaultOpen: true,
			OnPick: func(path []string, preview string) {
				toastShow("选中 " + strconv.Itoa(len(path)) + " 层：" + preview)
			},
		}),
	).St(ui.St().Padding(20).GapOf(12))
}

// ---- 就地确认 ----

var proDeleted bool

func popconfirmSection() *ui.Node {
	return ui.Column(
		pageHead("就地确认", "点击后在原位展开确定/取消，不依赖浮层定位；危险操作用 BtnDanger"),
		ui.Row(
			ui.Popconfirm("gallery-pc-del", "删除记录", "删除后不可恢复，确定继续？", ui.BtnDanger, func() {
				proDeleted = true
				toastShow("已删除")
			}),
			ui.Popconfirm("gallery-pc-reset", "重置设置", "将恢复全部默认值，确定？", ui.BtnPrimary, func() {
				toastShow("已重置")
			}),
			ui.Spacer(),
			ui.Text("（行内展开，不遮挡布局）").St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
		).St(ui.St().GapOf(12)).AlignTo(ui.AlignCenter),
	).St(ui.St().Padding(20).GapOf(12))
}
