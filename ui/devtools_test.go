// guis — 内置 DevTools 回归

package ui

import (
	"testing"

	"guis/internal/geom"
)

// buildDevTestView 一个带百分比尺寸与 Grow 的普通页面，用于验证包装不改变布局。
func buildDevTestView() *Node {
	return Column(
		Box().St(St().Hp(50).Wp(100)),
		Row(
			Box().St(St().Wp(50).H(40)),
			Box().St(St().GrowTo(1).H(40)),
		).St(St().GapOf(8)),
	).St(St().Hp(100).GapOf(10).Padding(10))
}

// TestDevToolsWrapKeepsLayout：面板挂在根 Stack 最上层，主视图的几何必须一帧不变。
//
// 这是 DevTools 最大的风险点：根被包进 Stack 后会收到 loose 约束，
// 若主视图的百分比尺寸 / Grow 因此变化，等于「打开调试面板就改了界面」。
func TestDevToolsWrapKeepsLayout(t *testing.T) {
	withTestFonts(t)
	devOnFlag.Store(true)
	defer devOnFlag.Store(false)

	plain := buildDevTestView()
	Layout(plain, 800, 600, newTestCache())

	root := devtoolsWrap(buildDevTestView(), 800, 600)
	if root == nil || len(root.Children) < 2 {
		t.Fatalf("devtoolsWrap 应返回含主视图与面板的 Stack，实际 %v", root)
	}
	Layout(root, 800, 600, newTestCache())

	view := root.Children[0]
	if view == nil {
		t.Fatal("Stack 的第一个子节点应是主视图")
	}
	if view.Frame.W != 800 || view.Frame.H != 600 {
		t.Fatalf("主视图应铺满整窗，实际 %.1fx%.1f", view.Frame.W, view.Frame.H)
	}
	if len(view.Children) != len(plain.Children) {
		t.Fatalf("主视图子节点数变了：%d != %d", len(view.Children), len(plain.Children))
	}
	for i := range plain.Children {
		want, got := plain.Children[i].Frame, view.Children[i].Frame
		if want != got {
			t.Fatalf("第 %d 个子节点布局被 DevTools 改变：%v != %v", i, got, want)
		}
	}
}

// TestDevToolsPanelTabs：三个页签都能构建并布局，不 panic、不空树。
func TestDevToolsPanelTabs(t *testing.T) {
	withTestFonts(t)
	prevTab, prevSel := devTab, devSelIdx
	defer func() { devTab, devSelIdx = prevTab, prevSel }()

	// 造一份最小采集数据供节点树页签使用
	root := Column(
		Text("标题").K("title"),
		Button("确定", func() {}).K("ok"),
	).St(St().Padding(12).GapOf(8))
	Layout(root, 400, 300, newTestCache())
	devtoolsSample(&App{fonts: newTestCache()}, root, 400, 300, 1.5)
	if len(devNodeList) == 0 {
		t.Fatal("采集后节点清单不应为空")
	}

	for _, tab := range []string{"overview", "tree", "repaint"} {
		devTab = tab
		panel := devtoolsPanel(800, 600)
		if panel == nil {
			t.Fatalf("%s 页签返回空面板", tab)
		}
		Layout(panel, 800, 600, newTestCache())
		if panel.Frame.W <= 0 || panel.Frame.H <= 0 {
			t.Fatalf("%s 页签布局后尺寸异常：%.1fx%.1f", tab, panel.Frame.W, panel.Frame.H)
		}
	}
}

// TestDevToolsSampleCollects：采集要给出正确节点数，且绝对坐标沿父链累加。
func TestDevToolsSampleCollects(t *testing.T) {
	withTestFonts(t)
	root := Column(
		Box().St(St().W(100).H(50)).K("inner"),
	).St(St().Padding(20))
	Layout(root, 400, 300, newTestCache())

	// 手动给子节点一个偏移，验证绝对坐标 = 父 + 自身
	root.Frame = geom.Rect{X: 0, Y: 0, W: 400, H: 300}
	if len(root.Children) == 0 {
		t.Fatal("测试视图缺少子节点")
	}
	child := root.Children[0]
	child.Frame = geom.Rect{X: 20, Y: 20, W: 100, H: 50}

	devtoolsSample(&App{fonts: newTestCache()}, root, 400, 300, 2.0)
	if len(devNodeList) != 2 {
		t.Fatalf("应采集到 2 个节点，实际 %d", len(devNodeList))
	}
	if devStatsCur.nodes != 2 {
		t.Fatalf("节点计数应为 2，实际 %d", devStatsCur.nodes)
	}
	got := devNodeList[1].abs
	if got.X != 20 || got.Y != 20 || got.W != 100 || got.H != 50 {
		t.Fatalf("子节点绝对矩形应为 20,20 100x50，实际 %v", got)
	}
	if devNodeList[1].depth != 1 {
		t.Fatalf("子节点深度应为 1，实际 %d", devNodeList[1].depth)
	}
}

// TestDevToolsSparklineKeepsOrder：采样环按时间先后出图，未满时不应包含空洞。
func TestDevToolsSparklineKeepsOrder(t *testing.T) {
	prevCount, prevNext := devSampleCount, devSampleNext
	defer func() { devSampleCount, devSampleNext = prevCount, prevNext }()
	devSampleCount, devSampleNext = 0, 0
	for i := 0; i < 5; i++ {
		devSamples[devSampleNext] = devSample{build: float64(i), paint: 1}
		devSampleNext = (devSampleNext + 1) % devSampleN
		devSampleCount++
	}
	ss := devSamplesInOrder()
	if len(ss) != 5 {
		t.Fatalf("应返回 5 个采样，实际 %d", len(ss))
	}
	for i, s := range ss {
		if s.build != float64(i) {
			t.Fatalf("采样顺序错乱：第 %d 个 build=%.1f", i, s.build)
		}
	}
}
