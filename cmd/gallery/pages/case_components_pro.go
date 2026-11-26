// guis — gallery 页面拆分：树形表格 / 穿梭框 / 无限滚动（新增组件演示）

package pages

import (
	"strconv"

	"guis/ui"
)

// ---- 树形表格 ----

func treeTableRows() []ui.TreeTableRow {
	return []ui.TreeTableRow{
		{Key: "ui", Texts: []string{"guis/ui", "1326 文件", "核心"}, Children: []ui.TreeTableRow{
			{Key: "ui-node", Texts: []string{"node.go", "组件基座", "稳定"}},
			{Key: "ui-form", Texts: []string{"form.go", "表单控件", "稳定"}},
			{Key: "ui-chart", Texts: []string{"chart.go", "图表", "实验中"}, Children: []ui.TreeTableRow{
				{Key: "ui-chart-draw", Texts: []string{"chart_draw.go", "绘制层", "实验中"}},
				{Key: "ui-chart-hit", Texts: []string{"chart_interact.go", "交互层", "实验中"}},
			}},
		}},
		{Key: "game", Texts: []string{"guis/game", "3D 引擎", "稳定"}, Children: []ui.TreeTableRow{
			{Key: "game-core", Texts: []string{"场景 / 相机", "内置", "稳定"}},
		}},
		{Key: "docs", Texts: []string{"docs", "文档", "维护中"}},
	}
}

func treeTableSection() *ui.Node {
	rows := treeTableRows()
	return ui.Column(
		pageHead("树形表格", "首列自动缩进与展开箭头，父行折叠整棵子树；展开/选中状态按 Key 保存"),
		ui.TreeTable(rows, ui.TreeTableOptions{
			Key:         "gallery-tree-table",
			Headers:     []string{"名称", "说明", "状态"},
			Widths:      []float32{240, 0, 90},
			DefaultOpen: true,
			OnClick: func(path []string, row ui.TreeTableRow) {
				toastShow("选中行：" + row.Texts[0])
			},
		}),
		ui.Row(
			ui.Btn("展开全部", ui.BtnGhost, func() {
				ui.TreeTableSetAllOpen("gallery-tree-table", rows, true)
			}),
			ui.Btn("收起全部", ui.BtnGhost, func() {
				ui.TreeTableSetAllOpen("gallery-tree-table", rows, false)
			}),
		).St(ui.St().GapOf(8).PadOnly(10, 0, 0, 0)),
	).St(ui.St().Padding(20).GapOf(12))
}

// ---- 穿梭框 ----

var transferSeeded bool

func transferItems() []ui.TransferItem {
	return []ui.TransferItem{
		{Key: "read", Label: "查看", Hint: "基础"},
		{Key: "write", Label: "编辑", Hint: "基础"},
		{Key: "delete", Label: "删除", Hint: "高危"},
		{Key: "export", Label: "导出", Hint: "基础"},
		{Key: "admin", Label: "系统管理", Hint: "高危"},
		{Key: "audit", Label: "审计日志", Hint: "只读"},
	}
}

func transferSection() *ui.Node {
	items := transferItems()
	s := ui.TransferOf("gallery-transfer")
	if !transferSeeded {
		transferSeeded = true
		s.Right["read"] = true
		s.Right["export"] = true
	}
	return ui.Column(
		pageHead("穿梭框", "左右候选列表 + 勾选转移，支持搜索过滤；右侧结果按原顺序回填"),
		ui.Transfer(s, items, ui.TransferOptions{
			Key:        "gallery-transfer",
			TitleLeft:  "可授权限",
			TitleRight: "已授权限",
			Height:     240,
			PanelWidth: 180,
			Filterable: true,
			OnChange: func(left, right []string) {
				toastShow("已选：" + strconv.Itoa(len(s.RightKeys(items))) + " 项")
			},
		}),
	).St(ui.St().Padding(20).GapOf(12))
}

// ---- 无限滚动 ----

var infCount = 20

func infiniteRows(n int) []*ui.Node {
	out := make([]*ui.Node, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, ui.Row(
			ui.Text("动态 #"+strconv.Itoa(i)).St(ui.St().Text(12.5, 500, ui.ColText)),
			ui.Spacer(),
			ui.Text(strconv.Itoa(i*7)+" 次浏览").St(ui.St().Text(11, 400, ui.ColTextMuted)),
		).St(ui.St().PadXY(14, 9)))
	}
	return out
}

func infiniteSection() *ui.Node {
	return ui.Column(
		pageHead("无限滚动", "滚到距底部 120px 自动触发加载；底部显示「加载中 / 没有更多」"),
		ui.InfiniteScroll(infiniteRows(infCount), ui.InfiniteScrollOptions{
			Key:       "gallery-infinite",
			Height:    260,
			Threshold: 120,
			HasMore:   infCount < 80,
			OnLoadMore: func() {
				if infCount >= 80 {
					return
				}
				infCount += 10
				if infCount > 80 {
					infCount = 80
				}
				ui.RequestRedraw()
			},
		}).St(ui.St().R(10).Stroke(1, ui.ColStroke).Clipped()),
		ui.Row(
			ui.Text("当前 "+strconv.Itoa(infCount)+" 条").St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
			ui.Spacer(),
			ui.Btn("重置", ui.BtnGhost, func() {
				infCount = 20
				ui.InfiniteScrollReset("gallery-infinite")
				ui.ScrollTo("gallery-infinite", 0)
				ui.RequestRedraw()
			}),
		).St(ui.St().GapOf(8)),
	).St(ui.St().Padding(20).GapOf(12))
}
