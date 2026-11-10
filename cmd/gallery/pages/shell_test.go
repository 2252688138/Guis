// guis — gallery 统一壳层（pages/shell.go）的响应式冒烟测试
//
// 三端跑的是同一个 View()：宽屏保留完整标题栏 + 232px 左侧导航，
// 窄屏（手机竖屏 / 拖窄的窗口）侧栏收窄、隐藏菜单栏，但条目、顺序、
// 选中态与内容区完全一致。这里用无窗口布局锁定这两种形态。

package pages

import (
	"testing"

	"guis/ui"
)

// TestShellWideLayout 宽屏（1180×900，桌面默认窗口）：侧栏标准宽度、标题栏与条目齐全。
func TestShellWideLayout(t *testing.T) {
	oldCurrent, oldTheme := Current, Theme
	defer func() { Current, Theme = oldCurrent, oldTheme }()
	Current, Theme = "basic", "深色"

	// 先排一帧让视口宽度同步（构建期读的是上一帧结果）
	ui.LayoutForTest(View(), 1180, 900, "", 1)
	root := View()
	ui.LayoutForTest(root, 1180, 900, "", 1)

	nav := findKeyNode(root, "nav")
	if nav == nil {
		t.Fatal("缺少左侧导航（nav）")
	}
	if nav.Frame.W != NavBaseW {
		t.Fatalf("宽屏侧栏宽度 %.1f，期望 %.0f（不改桌面观感）", nav.Frame.W, NavBaseW)
	}
	if n := findKeyNode(root, "titlebar"); n == nil || n.Frame.H <= 0 {
		t.Fatal("宽屏缺少自绘标题栏")
	}
	// 每个分类入口都要在（三端同一份导航）
	for _, c := range Cases {
		n := findKeyNode(root, "case-"+c.ID)
		if n == nil || n.Frame.W <= 0 {
			t.Fatalf("左侧导航缺少条目 %s", c.ID)
		}
	}
	if n := findKeyNode(root, "scroll-basic"); n == nil || n.Frame.H <= 0 {
		t.Fatal("内容区未撑开（scroll-basic 高度为 0）")
	}
}

// TestShellNarrowLayout 窄屏（480×853 ≈ 手机竖屏）：侧栏收窄但保留全部条目，
// 内容区仍有可用宽度，标题栏与内容区一个不少。
func TestShellNarrowLayout(t *testing.T) {
	oldCurrent, oldTheme := Current, Theme
	defer func() { Current, Theme = oldCurrent, oldTheme }()
	Current, Theme = "basic", "深色"

	ui.LayoutForTest(View(), 480, 853, "", 1)
	root := View()
	ui.LayoutForTest(root, 480, 853, "", 1)

	nav := findKeyNode(root, "nav")
	if nav == nil {
		t.Fatal("窄屏缺少左侧导航（nav）")
	}
	if nav.Frame.W != 150 {
		t.Fatalf("窄屏侧栏宽度 %.1f，期望 150（自适应收窄）", nav.Frame.W)
	}
	if nav.Frame.W >= 480*0.5 {
		t.Fatalf("侧栏 %.1f 占了窄屏的一半以上，内容区没空间了", nav.Frame.W)
	}
	for _, c := range Cases {
		if n := findKeyNode(root, "case-"+c.ID); n == nil {
			t.Fatalf("窄屏左侧导航缺少条目 %s（三端必须同一份菜单）", c.ID)
		}
	}
	if n := findKeyNode(root, "titlebar"); n == nil || n.Frame.H <= 0 {
		t.Fatal("窄屏缺少标题栏")
	}
	if n := findKeyNode(root, "scroll-basic"); n == nil || n.Frame.W <= 0 || n.Frame.H <= 0 {
		t.Fatal("窄屏内容区未撑开")
	}
}

// TestShellSwitchesByViewport 同一份 View 在宽窄两种视口下选择不同的侧栏宽度。
func TestShellSwitchesByViewport(t *testing.T) {
	oldCurrent := Current
	defer func() { Current = oldCurrent }()
	Current = "basic"

	type tcase struct {
		w    float32
		want float32
	}
	cases := []tcase{
		{1400, NavBaseW}, // 宽屏：232
		{1000, NavBaseW}, // 仍算宽屏（>= NarrowWidth）
		{880, 176},       // 窄屏（< 900）：收窄
		{480, 150},       // 手机竖屏：进一步收窄
	}
	for _, c := range cases {
		ui.LayoutForTest(View(), c.w, 800, "", 1)
		root := View()
		ui.LayoutForTest(root, c.w, 800, "", 1)
		nav := findKeyNode(root, "nav")
		if nav == nil {
			t.Fatalf("宽度 %.0f：缺少左侧导航", c.w)
		}
		if nav.Frame.W != c.want {
			t.Fatalf("宽度 %.0f：侧栏 %.1f，期望 %.0f", c.w, nav.Frame.W, c.want)
		}
	}
}
