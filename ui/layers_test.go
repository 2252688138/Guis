// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"testing"

	"guis/internal/geom"
	"guis/internal/gpu"
)

// cacheableCard 构造一个「适合栅格缓存」的节点：不透明底色、无圆角/描边/阴影。
func cacheableCard(txt string) *Node {
	return Column(
		Text("标题").St(St().Text(14, 600, "#e8ecff")),
		Text(txt).St(St().Text(12, 400, "#6b7694")),
	).K("card").St(St().Padding(16).BgC("#1b2333").CacheOn())
}

// TestLayerFingerprintStable 同样的内容 → 同样的指纹；内容变了 → 指纹变。
// 指纹是缓存命中判定的唯一依据，漏字段会导致「内容变了还贴旧图」。
func TestLayerFingerprintStable(t *testing.T) {
	a := testApp()
	root := cacheableCard("内容 A")
	Layout(root, 400, 300, newTestCache())
	fp1 := a.layerFingerprint(root)

	same := cacheableCard("内容 A")
	Layout(same, 400, 300, newTestCache())
	if got := a.layerFingerprint(same); got != fp1 {
		t.Fatalf("同内容指纹应一致：%x != %x", got, fp1)
	}

	// 文本变化
	other := cacheableCard("内容 B")
	Layout(other, 400, 300, newTestCache())
	if a.layerFingerprint(other) == fp1 {
		t.Fatal("文本变化后指纹必须变")
	}

	// 颜色变化
	tinted := cacheableCard("内容 A")
	tinted.Style.Bg = C("#ff0000")
	if a.layerFingerprint(tinted) == fp1 {
		t.Fatal("背景色变化后指纹必须变")
	}

	// 几何变化（可用宽度变了 → 卡片尺寸/换行都可能变）
	moved := cacheableCard("内容 A")
	Layout(moved, 500, 300, newTestCache())
	if a.layerFingerprint(moved) == fp1 {
		t.Fatal("尺寸变化后指纹必须变")
	}
}

// TestLayerFingerprintIgnoresCallbacks 回调变化不影响画面，不应让缓存失效。
func TestLayerFingerprintIgnoresCallbacks(t *testing.T) {
	a := testApp()
	n1 := cacheableCard("内容 A")
	n1.OnClick = func() {}
	Layout(n1, 400, 300, newTestCache())
	fp1 := a.layerFingerprint(n1)

	n2 := cacheableCard("内容 A")
	n2.OnClick = func() {}
	n2.OnHover = func(bool) {}
	n2.OnDrag = func(x, y float32) {}
	Layout(n2, 400, 300, newTestCache())
	if got := a.layerFingerprint(n2); got != fp1 {
		t.Fatalf("回调变化不应影响指纹：%x != %x", got, fp1)
	}
}

// TestLayerCacheableRules 缓存适用性判定：不满足条件必须拒绝（宁可多画，不可画错）。
func TestLayerCacheableRules(t *testing.T) {
	a := testApp()
	a.winW, a.winH = 800, 600
	full := geom.Rect{X: 0, Y: 0, W: 800, H: 600}

	ok := cacheableCard("A")
	Layout(ok, 800, 600, newTestCache())
	if !a.layerCacheable(ok, ok.Frame, false, full) {
		t.Fatal("不透明无圆角卡片应可缓存")
	}

	type tc struct {
		name   string
		mutate func(*Node)
	}
	cases := []tc{
		{"半透明背景", func(n *Node) { n.Style.Bg.A = 0.5 }},
		{"无背景", func(n *Node) { n.Style.Bg = gpu.Paint{} }},
		{"阴影", func(n *Node) { n.Style.ShadowColor = C("#00000080") }},
		{"动画", func(n *Node) { n.Style.Animated = true }},
		{"毛玻璃", func(n *Node) { n.Style.BackdropBlur = 12 }},
		{"无 Key", func(n *Node) { n.Key = "" }},
		{"含视频", func(n *Node) { n.Children = append(n.Children, Video(nil)) }},
		{"含图表", func(n *Node) { n.Children = append(n.Children, &Node{Kind: KChart}) }},
		{"含图片", func(n *Node) { n.Children = append(n.Children, Image("x.png")) }},
		{"含焦点", func(n *Node) { n.Children[0].Focus = true }},
	}
	for _, c := range cases {
		n := cacheableCard("A")
		Layout(n, 800, 600, newTestCache())
		c.mutate(n)
		if a.layerCacheable(n, n.Frame, false, full) {
			t.Fatalf("%s 的节点不应被缓存", c.name)
		}
	}

	// 圆角可以缓存：贴回时用 SDF 圆角遮罩，圆角外的旧像素不会露出
	rounded := cacheableCard("A")
	rounded.Style.Radius = [4]float32{12, 12, 12, 12}
	Layout(rounded, 800, 600, newTestCache())
	if !a.layerCacheable(rounded, rounded.Frame, false, full) {
		t.Fatal("带圆角的不透明卡片应可缓存（贴回时按 SDF 圆角遮罩）")
	}

	// 描边是向内画的，不超出矩形，因此也能缓存
	bordered := cacheableCard("A")
	bordered.Style.Border = 1
	bordered.Style.BorderColor = C("#ffffff")
	Layout(bordered, 800, 600, newTestCache())
	if !a.layerCacheable(bordered, bordered.Frame, false, full) {
		t.Fatal("描边在矩形内绘制，应可缓存")
	}

	// 不完全落在裁剪区内
	big := cacheableCard("A")
	Layout(big, 800, 600, newTestCache())
	half := geom.Rect{X: 0, Y: 0, W: 40, H: 40}
	if a.layerCacheable(big, big.Frame, true, half) {
		t.Fatal("不完全落在裁剪区内时不应缓存")
	}
}

// TestDropLayerReleases 丢弃 / 全量释放只清记账（无 GL 时纹理为 nil，不应 panic）。
func TestDropLayerReleases(t *testing.T) {
	a := testApp()
	a.layers = map[uint64]*layerEntry{
		1: {w: 10, h: 10},
	}
	a.dropLayer(1)
	if len(a.layers) != 0 {
		t.Fatal("dropLayer 应删除条目")
	}
	a.layers[2] = &layerEntry{}
	a.layerReleaseAll()
	if a.layers != nil {
		t.Fatal("layerReleaseAll 应清空映射")
	}
}

// TestLayerEvictByBudget 超出预算时按最近使用淘汰。
func TestLayerEvictByBudget(t *testing.T) {
	a := testApp()
	// 用一个很小的预算跑淘汰逻辑（无 GL 时 DeleteTexture 对 nil 纹理是空操作）
	a.layers = map[uint64]*layerEntry{}
	for i := 0; i < 8; i++ {
		e := &layerEntry{w: 4, h: 4, stamp: int64(i + 1)}
		a.layers[uint64(i+1)] = e
	}
	// 直接调用淘汰：need 取一个把预算撑爆的值
	a.layerEvict(64 << 20)
	if len(a.layers) > 1 {
		t.Fatalf("超额时应淘汰到只剩 1 条（最近使用），实际 %d 条", len(a.layers))
	}
}

// TestLayerCacheableSkipsFocusRing 获得焦点的节点不缓存（焦点环画在内容之上）。
func TestLayerCacheableSkipsFocusRing(t *testing.T) {
	a := testApp()
	a.winW, a.winH = 800, 600
	n := cacheableCard("A")
	Layout(n, 800, 600, newTestCache())
	n.Focus = true
	if a.layerCacheable(n, n.Frame, false, geom.Rect{X: 0, Y: 0, W: 800, H: 600}) {
		t.Fatal("获得焦点的节点不应缓存（焦点环会被快照漏掉）")
	}
}
