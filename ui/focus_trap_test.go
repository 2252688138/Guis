// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// focusableText 造一个「能聚焦的纯文本」节点：Text 自带 Opacity 1，
// 这里只补 Focusable（用 St() 会覆盖 Opacity 导致节点被判为不可见）。
func focusableText(s, key string) *Node {
	n := Text(s).K(key)
	n.Style.Focusable = true
	return n
}

// trapFixture 建树 + 布局 + 安装成当前 App，退出时清干净全部包级状态。
func trapFixture(t *testing.T, root *Node) *App {
	t.Helper()
	fc := newTestCache()
	Layout(root, 600, 400, fc)
	a := &App{fonts: fc, scrolls: map[string]float32{}}
	a.root = root
	prev := currentApp.Swap(a)
	t.Cleanup(func() {
		currentApp.Store(prev)
		ResetFocusScopes()
		ResetRoveState()
		trapUsed.Store(false)
		SetFocusVisible(false)
	})
	return a
}

// TestFocusTrapTabStaysInsideSubtree 焦点陷阱内 Tab 只在子树内循环。
func TestFocusTrapTabStaysInsideSubtree(t *testing.T) {
	root := Column(
		Column(focusableText("一", "trap-1"), focusableText("二", "trap-2")).Trap(),
		focusableText("外", "trap-out"),
	)
	a := trapFixture(t, root)

	if got := a.focusables(); len(got) != 2 {
		t.Fatalf("陷阱内应只有 2 个可聚焦节点，实际 %d 个", len(got))
	}
	for i := 0; i < 6; i++ {
		FocusNext()
		if a.focusKey != "trap-1" && a.focusKey != "trap-2" {
			t.Fatalf("第 %d 次 Tab 后焦点跑出陷阱：%q", i+1, a.focusKey)
		}
	}
	// 反向同理
	for i := 0; i < 3; i++ {
		FocusPrev()
		if a.focusKey != "trap-1" && a.focusKey != "trap-2" {
			t.Fatalf("第 %d 次 Shift+Tab 后焦点跑出陷阱：%q", i+1, a.focusKey)
		}
	}
}

// TestFocusTrapAutoFocusesFirst 陷阱出现时把焦点从背景收进子树；
// 焦点已在子树内则保持不动（用户点到哪就停在哪）。
func TestFocusTrapAutoFocusesFirst(t *testing.T) {
	root := Column(
		Column(focusableText("一", "auto-1")).Trap(),
		focusableText("外", "auto-out"),
	)
	a := trapFixture(t, root)

	SetFocus("auto-out")
	a.syncFocusTrap()
	if a.focusKey != "auto-1" {
		t.Fatalf("陷阱出现后焦点应收进子树，实际 %q", a.focusKey)
	}

	a.syncFocusTrap()
	if a.focusKey != "auto-1" {
		t.Fatalf("焦点已在陷阱内不应被改动，实际 %q", a.focusKey)
	}
}

// TestDeepestTrapWins 嵌套陷阱取最内层。
func TestDeepestTrapWins(t *testing.T) {
	inner := Column(focusableText("内", "deep-in")).Trap()
	outer := Column(focusableText("外", "deep-out"), inner).Trap()
	a := trapFixture(t, Column(outer))

	if f := a.focusables(); len(f) != 1 || f[0].Key != "deep-in" {
		t.Fatalf("应只看到最内层陷阱的可聚焦节点，实际 %v", trapKeys(f))
	}
}

// TestRovingGroupSingleTabStop 一组只占一个 Tab 位，方向键在组内移动并循环。
func TestRovingGroupSingleTabStop(t *testing.T) {
	root := Row(
		focusableText("一", "rove-1"),
		focusableText("二", "rove-2"),
		focusableText("三", "rove-3"),
	).Rove("test-group")
	a := trapFixture(t, root)

	if f := a.focusables(); len(f) != 1 || f[0].Key != "rove-1" {
		t.Fatalf("roving 组应只出一个 Tab 站点（首项），实际 %v", trapKeys(f))
	}

	SetFocus("rove-1")
	if !rovingHandleKey(VK_RIGHT) {
		t.Fatal("组内方向键应被消费")
	}
	if a.focusKey != "rove-2" {
		t.Fatalf("右键应移到第二项，实际 %q", a.focusKey)
	}
	if RoveActive("test-group") != "rove-2" {
		t.Fatalf("活动项未记录，实际 %q", RoveActive("test-group"))
	}
	if f := a.focusables(); len(f) != 1 || f[0].Key != "rove-2" {
		t.Fatalf("Tab 站点应跟随活动项，实际 %v", trapKeys(f))
	}

	// 反向循环：rove-2 → rove-1 → rove-3
	rovingHandleKey(VK_LEFT)
	rovingHandleKey(VK_LEFT)
	if a.focusKey != "rove-3" {
		t.Fatalf("左键应循环到最后一项，实际 %q", a.focusKey)
	}
}

// TestRovingSkipsTextEdit 文本编辑控件里方向键要留给光标，不能被 roving 吞掉。
func TestRovingSkipsTextEdit(t *testing.T) {
	root := Row(
		Input("rove-in", "", "", nil),
		focusableText("按钮", "rove-btn"),
	).Rove("test-group2")
	a := trapFixture(t, root)

	RegisterTextEdit("rove-in", func(TextEditAction) bool { return false })
	defer delete(textEditHandlers, "rove-in")

		a.focusKey = "rove-in"
	if rovingHandleKey(VK_RIGHT) {
		t.Fatal("输入框内的方向键不应被 roving 组消费")
	}
}

// TestRoveActiveFallsBackWhenItemGone 活动项被删掉后回落到首项，不残留死 Key。
func TestRoveActiveFallsBackWhenItemGone(t *testing.T) {
	group := Row(focusableText("一", "fb-1"), focusableText("二", "fb-2")).Rove("test-group3")
	a := trapFixture(t, Column(group))

	SetFocus("fb-2")
	rovingHandleKey(VK_RIGHT) // fb-2 → fb-1（循环）
	if RoveActive("test-group3") != "fb-1" {
		t.Fatalf("活动项应为 fb-1，实际 %q", RoveActive("test-group3"))
	}

	// 删掉 fb-1，只剩 fb-2：活动项失效后应回落
	group.Children = []*Node{focusableText("二", "fb-2")}
	if f := a.focusables(); len(f) != 1 || f[0].Key != "fb-2" {
		t.Fatalf("活动项被删后应回落到剩余项，实际 %v", trapKeys(f))
	}
}

func trapKeys(ns []*Node) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.Key)
	}
	return out
}
