// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// TestContextMenuLayerState 未展开不渲染，展开后渲染浮层。
func TestContextMenuLayerState(t *testing.T) {
	st := NewContextMenu("cm")
	if got := ContextMenuLayer(st); got != nil {
		t.Fatal("未展开时不应渲染右键菜单")
	}
	st.At(20, 30, []MenuItem{{Label: "复制"}})
	if !st.Open {
		t.Fatal("At 后应展开")
	}
	if got := ContextMenuLayer(st); got == nil {
		t.Fatal("展开后应渲染浮层")
	}
	st.Close()
	if st.Open || ContextMenuLayer(st) != nil {
		t.Fatal("Close 后应停止渲染")
	}
}

// TestWrapMenuItemsClosesFirst 选中菜单项时先关菜单再执行回调。
func TestWrapMenuItemsClosesFirst(t *testing.T) {
	st := NewContextMenu("cm2")
	st.At(0, 0, nil)
	picked, closedWhenPicked := false, false
	items := wrapMenuItems([]MenuItem{{Label: "删除", OnPick: func() {
		picked = true
		closedWhenPicked = !st.Open
	}}}, st.Close)
	items[0].OnPick()
	if !picked || !closedWhenPicked {
		t.Fatalf("应先关菜单再回调：picked=%v closedFirst=%v", picked, closedWhenPicked)
	}
}

// TestWithContextMenuPosition 右键位置 = 节点布局坐标 + 节点内偏移。
func TestWithContextMenuPosition(t *testing.T) {
	st := NewContextMenu("cm3")
	target := Column(Text("行")).St(St().W(200).H(100))
	target.WithContextMenu(st, []MenuItem{{Label: "菜单项"}})
	root := Stack(target).St(St().W(400).H(300))
	Layout(root, 400, 300, newTestCache())

	if target.OnRightClickAt == nil {
		t.Fatal("应挂上右键回调")
	}
	target.OnRightClickAt(10, 20)
	if !st.Open {
		t.Fatal("右键后菜单应展开")
	}
	if st.X != target.Frame.X+10 || st.Y != target.Frame.Y+20 {
		t.Fatalf("菜单位置错误：got (%.1f,%.1f) want (%.1f,%.1f)",
			st.X, st.Y, target.Frame.X+10, target.Frame.Y+20)
	}
}

// TestPopoverFlipsUp 下方空间不足时翻到锚点上方，且不越出窗口。
func TestPopoverFlipsUp(t *testing.T) {
	Layout(Stack(Text("x")).St(St().W(400).H(300)), 400, 300, newTestCache())
	st := NewPopover("pop")
	st.W, st.H = 100, 80

	st.ShowAt(10, 10) // 上方有位置，默认在下方
	if _, y := popoverPos(st); y != 16 {
		t.Fatalf("默认应在锚点下方 6px，实际 y=%.1f", y)
	}

	st.ShowAt(300, 250) // 下方放不下 → 翻上方
	x, y := popoverPos(st)
	if y != 164 {
		t.Fatalf("应翻到锚点上方，实际 y=%.1f", y)
	}
	if x != 292 {
		t.Fatalf("应夹取到窗口右边界内，实际 x=%.1f", x)
	}
}

// TestPopoverLayerToggle 展开渲染、关闭后不渲染。
func TestPopoverLayerToggle(t *testing.T) {
	Layout(Stack(Text("x")).St(St().W(400).H(300)), 400, 300, newTestCache())
	st := NewPopover("pop2")
	if PopoverLayer(st, Text("内容")) != nil {
		t.Fatal("未展开时不应渲染")
	}
	st.ShowAt(20, 20)
	if PopoverLayer(st, Text("内容")) == nil {
		t.Fatal("展开后应渲染")
	}
	st.Toggle(20, 20)
	if st.Open {
		t.Fatal("Toggle 应收起")
	}
}

// TestDateRangeShowsHint 区间倒置时多渲染一行提示。
func TestDateRangeShowsHint(t *testing.T) {
	st := NewDateRange("dr")
	ok := DateRangeBox(st, "2026-04-01", "2026-05-01", nil)
	bad := DateRangeBox(st, "2026-05-01", "2026-04-01", nil)
	if len(ok.Children) != 3 {
		t.Fatalf("合法区间应是单行（3 个子节点），实际 %d", len(ok.Children))
	}
	if len(bad.Children) != 2 {
		t.Fatalf("倒置区间应是 Row + 提示（2 个子节点），实际 %d", len(bad.Children))
	}
	if !DateRangeValid("2026-04-01", "2026-05-01") || DateRangeValid("2026-05-01", "2026-04-01") {
		t.Fatal("DateRangeValid 判定错误")
	}
}
