// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"testing"

	"guis/internal/geom"
)

// TestAutoLayerRoots 自动模式：只把「最外层纯静态的不透明容器」标成缓存根。
func TestAutoLayerRoots(t *testing.T) {
	a := testApp()
	inner := Column(Text("内容").St(St().Text(12, 400, "#ffffff"))).K("inner").
		St(St().Padding(10).BgC("#101010"))
	outer := Column(inner).K("outer").St(St().Padding(20).BgC("#202020"))
	Layout(outer, 400, 300, newTestCache())

	if !markStaticTree(outer) || outer.staticOK != 1 {
		t.Fatal("纯静态容器应标记为 staticOK=1")
	}
	a.markCacheRoots(outer, 0xCBF29CE484222325, false)
	if !outer.cacheRoot {
		t.Fatal("最外层合格容器应成为缓存根")
	}
	if inner.cacheRoot {
		t.Fatal("内层不应重复成为缓存根（嵌套缓存的收益为零，只会多一次拷贝）")
	}
	if !layerRootEligible(outer) {
		t.Fatal("不透明、无阴影、尺寸足够的容器应满足静态条件")
	}
}

// TestAutoLayerAllowsInteractive 交互控件不再阻挡缓存（外观变化靠指纹检测）。
func TestAutoLayerAllowsInteractive(t *testing.T) {
	btn := Text("按钮").St(St().Text(12, 500, "#ffffff"))
	btn.OnClick = func() {}
	outer := Column(btn).K("outer").St(St().Padding(20).BgC("#202020"))
	Layout(outer, 400, 300, newTestCache())

	if !markStaticTree(outer) {
		t.Fatal("带 OnClick 的子节点仍应视为静态（会经过指纹检测）")
	}
	testApp().markCacheRoots(outer, 0xCBF29CE484222325, false)
	if !outer.cacheRoot {
		t.Fatal("带按钮的不透明卡片应能成为缓存根")
	}
}

// TestAutoLayerExcludesDynamicKinds 图片 / 图表 / 视频 / 动画 / 毛玻璃都不参与缓存。
func TestAutoLayerExcludesDynamicKinds(t *testing.T) {
	cases := map[string]func() *Node{
		"图片":  func() *Node { return Column(Image("x.png")).K("r").St(St().Padding(8).BgC("#202020")) },
		"图表":  func() *Node { return Column(&Node{Kind: KChart}).K("r").St(St().Padding(8).BgC("#202020")) },
		"动画":  func() *Node { return Column(Box().St(St().AnimOn())).K("r").St(St().Padding(8).BgC("#202020")) },
		"毛玻璃": func() *Node { return Column(Box()).K("r").St(St().Padding(8).BgC("#202020").Backdrop(12, "#00000000")) },
	}
	for name, build := range cases {
		n := build()
		Layout(n, 400, 300, newTestCache())
		if markStaticTree(n) {
			t.Fatalf("%s 子树不应视为纯静态", name)
		}
	}
}

// TestAutoLayerExcludesScrollRoot 滚动容器不当缓存根（内容随偏移变化）。
func TestAutoLayerExcludesScrollRoot(t *testing.T) {
	n := Scroll(Column(Text("行").St(St().Text(12, 400, "#fff")))).K("sc").
		St(St().Padding(8).BgC("#202020"))
	Layout(n, 400, 300, newTestCache())
	markStaticTree(n)
	testApp().markCacheRoots(n, 0xCBF29CE484222325, false)
	if n.cacheRoot {
		t.Fatal("滚动容器不应成为缓存根（滚动期间会每帧失效重拍）")
	}
	if !layerRootEligible(&Node{Style: Style{Bg: C("#202020")}, Key: "k", staticOK: 1, layerKey: 1,
		Frame: geom.Rect{X: 0, Y: 0, W: 400, H: 300}}) {
		t.Fatal("普通不透明容器应满足静态条件")
	}
}

// TestAutoLayerExcludesSmall 小于 64×64 的块不做缓存（拷贝成本抵不过收益）。
func TestAutoLayerExcludesSmall(t *testing.T) {
	n := Column(Text("x").St(St().Text(11, 400, "#fff"))).K("small").
		St(St().Padding(2).BgC("#202020"))
	Layout(n, 40, 40, newTestCache())
	markStaticTree(n)
	testApp().markCacheRoots(n, 0xCBF29CE484222325, false)
	if n.cacheRoot {
		t.Fatal("过小的块不应成为缓存根")
	}
}
