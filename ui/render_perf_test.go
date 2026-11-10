// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"testing"
)

// perfCard 构造一张典型卡片（背景 + 标题 + 数值 + 折行说明）。
func perfCard(i int) *Node {
	return Column(
		Text(fmt.Sprintf("卡片标题 %d", i)).St(St().Text(14, 600, "#e8ecff")),
		Row(
			Text(fmt.Sprintf("%d", i*37)).St(St().Text(28, 700, "#f1f5ff")),
			Text("次").St(St().Text(12, 500, "#22d3ee").PadOnly(0, 0, 8, 6)),
		),
		Text("较昨日增长 12.4%，累计访问量继续走高，这里是一段用于折行的说明文本。").St(St().Text(12, 400, "#6b7694").WrapOn()),
	).St(St().Padding(16).R(14).BgC("#1b2333").Stroke(1, "#2a3550").GapOf(8))
}

// perfPage 构造一页重负载内容：侧栏 + 卡片网格 + 长列表（每帧重建的典型成本）。
func perfPage(cards int) *Node {
	grid := make([]*Node, 0, cards)
	for i := 0; i < cards; i++ {
		grid = append(grid, perfCard(i))
	}
	rows := make([]*Node, 0, 40)
	for i := 0; i < 40; i++ {
		rows = append(rows, Row(
			Text(fmt.Sprintf("%d", i+1)).St(St().Text(12, 400, "#5b6480").W(40)),
			Text(fmt.Sprintf("列表项名称 %d，带一段较长的描述文字用于折行测量", i)).St(St().Text(13, 400, "#c8d2ea").GrowTo(1)),
			Text("状态").St(St().Text(12, 500, "#22d3ee").Padding(4).R(6).BgC("#132033")),
		).St(St().Padding(12).GapOf(12).BgC("#161d2b").R(10)))
	}
	return Row(
		Column(
			Text("GUIS").St(St().Text(20, 700, "#e8ecff")),
			Text("自绘 GUI").St(St().Text(12, 400, "#5b6480")),
			Spacer(),
		).St(St().W(220).Hp(100).Padding(20).BgC("#10151f").GapOf(6)),
		Scroll(
			Column(
				Row(
					Text("控制台").St(St().Text(26, 700, "#eef2ff").GrowTo(1)),
					Button("新建", func() {}),
				),
				Grid(grid...).St(St().ColsN(4).GapOf(16)),
				Column(rows...).St(St().GapOf(8)),
			).St(St().Padding(28).GapOf(20)),
		).St(St().GrowTo(1)).K("perf-scroll"),
	).St(St().Hp(100))
}

// TestTrimIdleDropsFonts 空闲回收必须真的把不用的字体放掉。
// 旧实现的 trimTo 受「本帧在用」保护，空闲时永远淘汰不掉任何字体。
func TestTrimIdleDropsFonts(t *testing.T) {
	fc := newTestCache()
	defer fc.close()
	for f := uint64(1); f <= 30; f++ {
		fc.frame = f
		for _, sz := range []float32{11, 12, 13, 14, 16, 18, 20, 24, 28, 32} {
			fc.getFamily(sz, 400, "")
			fc.getFamily(sz, 600, "")
		}
	}
	before := len(fc.fonts)
	if before < 8 {
		t.Skipf("字体实例过少（%d），当前环境无可回收内容", before)
	}
	fc.trimIdle(idleKeepFonts)
	if got := len(fc.fonts); got > idleKeepFonts {
		t.Fatalf("空闲回收后字体数 = %d，期望 <= %d", got, idleKeepFonts)
	}
	if got := len(fc.fonts); got >= before {
		t.Fatalf("空闲回收没有效果：%d → %d", before, got)
	}
	// 回收后仍能按需重建
	if f := fc.getFamily(14, 400, ""); f == nil {
		t.Fatal("回收后应能重新创建字体")
	}
}

// BenchmarkFrameBuild 只重建节点树（声明式 UI 的固有成本，每帧一次）。
func BenchmarkFrameBuild(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = perfPage(24)
	}
}

// BenchmarkFrameLayout 重建 + 排版（不含绘制与 GL）。
func BenchmarkFrameLayout(b *testing.B) {
	fc := newTestCache()
	defer fc.close()
	// 预热字体与折行缓存，只测稳态
	Layout(perfPage(24), 1180, 760, fc)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Layout(perfPage(24), 1180, 760, fc)
	}
}

// TestFontCacheHitKeepsTick 同一帧内重复取同一字体不应刷新 LRU 计数。
// 旧实现每次命中都 tick++ 并写两次 map，而布局与绘制对每个文本节点各查一次，
// 一屏上百个节点就是几百次无意义的写入。
func TestFontCacheHitKeepsTick(t *testing.T) {
	fc := newTestCache()
	defer fc.close()

	fc.frame = 1
	f1 := fc.getFamily(14, 400, "")
	tick := fc.tick
	f2 := fc.getFamily(14, 400, "")
	if f1 != f2 {
		t.Fatal("同参数应返回同一字体实例")
	}
	if fc.tick != tick {
		t.Fatalf("同帧重复命中不应推进 LRU 计数：%d → %d", tick, fc.tick)
	}

	// 换帧后应重新记一次“本帧在用”
	fc.frame = 2
	fc.getFamily(14, 400, "")
	if fc.tick != tick+1 {
		t.Fatalf("换帧后应推进一次 LRU 计数，实际 %d，期望 %d", fc.tick, tick+1)
	}

	// 不同字号是新条目，也要推进
	fc.getFamily(18, 700, "")
	if fc.tick != tick+2 {
		t.Fatalf("新字体应推进 LRU 计数，实际 %d", fc.tick)
	}
}

// TestImageSizeCacheBounded 图片尺寸缓存必须有上限（长列表/相册会碰到成千上万个路径）。
func TestImageSizeCacheBounded(t *testing.T) {
	imageMetaMu.Lock()
	saved := imageMetas
	imageMetas = map[string]imageMeta{}
	imageMetaMu.Unlock()
	defer func() {
		imageMetaMu.Lock()
		imageMetas = saved
		imageMetaMu.Unlock()
	}()

	for i := 0; i < imageMetaMax+100; i++ {
		// 不存在的路径也会被缓存（记为失败），正好用来验证上限
		ImageSize(fmt.Sprintf("no-such-file-%d.png", i))
	}
	imageMetaMu.RLock()
	n := len(imageMetas)
	imageMetaMu.RUnlock()
	if n > imageMetaMax {
		t.Fatalf("尺寸缓存条数 = %d，超过上限 %d", n, imageMetaMax)
	}
}

// TestImageSizeConsistent 同一路径多次查询结果一致（命中缓存不改变语义）。
func TestImageSizeConsistent(t *testing.T) {
	w1, h1 := ImageSize("assets/icon.png")
	w2, h2 := ImageSize("assets/icon.png")
	if w1 != w2 || h1 != h2 {
		t.Fatalf("尺寸缓存不一致：%dx%d vs %dx%d", w1, h1, w2, h2)
	}
}
