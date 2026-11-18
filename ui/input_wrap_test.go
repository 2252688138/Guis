// guis — 输入框换行 / 横向滚动回归测试。
//
// 规则：单行输入框（Input）不折行，文本超出可视宽度时横向滚动，光标始终留在框内；
// WrapInput 折行，框高随行数增长，不再裁掉尾部文字。

package ui

import (
	"strings"
	"testing"
)

// withTestFonts 临时挂一个带字体的 App（文本度量依赖 currentApp.fonts）。
func withTestFonts(tb testing.TB) {
	tb.Helper()
	fc := newTestCache()
	prev := currentApp.Swap(&App{fonts: fc})
	tb.Cleanup(func() { currentApp.Store(prev) })
}

// TestInputScrollStart 光标超出可视宽度时返回滚动起点，且滚出后的可见部分能装进宽度。
func TestInputScrollStart(t *testing.T) {
	withTestFonts(t)
	const s = "abcdefghijklmnopqrstuvwxyz0123456789"
	runes := []rune(s)
	const avail = float32(80)

	start := inputScrollStart(runes, len(runes), avail)
	if start <= 0 {
		t.Fatalf("超宽文本应横向滚动（start = %d）", start)
	}
	if w := TextWidth(string(runes[start:]), 13, 400); w > avail {
		t.Fatalf("滚动后可见宽度 %.1f 超出可视宽度 %.1f", w, avail)
	}
	// 起点再往前一格就装不下，说明没有多滚
	if start > 0 {
		if w := TextWidth(string(runes[start-1:]), 13, 400); w <= avail-6 {
			t.Fatalf("滚动起点偏左（再往前一格仍宽 %.1f <= %.1f）", w, avail-6)
		}
	}

	// 放得下 / 宽度未知：不滚动
	if got := inputScrollStart(runes, 3, 500); got != 0 {
		t.Fatalf("放得下的文本不应滚动（start = %d）", got)
	}
	if got := inputScrollStart(runes, len(runes), 0); got != 0 {
		t.Fatalf("可视宽度未知时不应滚动（start = %d）", got)
	}
}

// TestInputWrapRanges 折行区间首尾相接、覆盖全文，且每行宽度不超出可用宽度。
func TestInputWrapRanges(t *testing.T) {
	withTestFonts(t)
	const s = "这是一段很长的单行文本，用来验证折行输入框在窄容器里会自动换行，不会把尾巴裁掉。"
	runes := []rune(s)
	const avail = float32(120)

	rows := WrapRanges(runes, avail, 13, 400)
	if len(rows) < 2 {
		t.Fatalf("超宽文本应折成多行（rows = %d）", len(rows))
	}
	if rows[0][0] != 0 || rows[len(rows)-1][1] != len(runes) {
		t.Fatalf("折行区间未覆盖全文：%v", rows)
	}
	for i, rg := range rows {
		if rg[1] < rg[0] {
			t.Fatalf("第 %d 行区间非法：%v", i, rg)
		}
		if w := TextWidth(string(runes[rg[0]:rg[1]]), 13, 400); w > avail {
			t.Fatalf("第 %d 行宽度 %.1f 超出可用宽度 %.1f", i, w, avail)
		}
		if i > 0 && rg[0] != rows[i-1][1] {
			t.Fatalf("第 %d 行与上一行不连续：%v / %v", i, rows[i-1], rg)
		}
	}
}

// TestWrapInputLayoutGrows 折行输入框：窄容器里框高随行数增长，空文本保持最小高度。
func TestWrapInputLayoutGrows(t *testing.T) {
	withTestFonts(t)
	const long = "这是一段很长的单行文本，用来验证折行输入框在窄容器里会自动换行并且框高随之增长，不会把文字裁掉。"
	a := currentApp.Load()

	build := func(val string) *Node {
		root := Column(WrapInput("wi", val, "占位", 0, nil)).St(St().W(200).Padding(10))
		a.root = root
		Layout(root, 220, 600, a.fonts)
		// 第二帧：首帧拿不到宽度，折行要等可视宽度已知
		root = Column(WrapInput("wi", val, "占位", 0, nil)).St(St().W(200).Padding(10))
		a.root = root
		Layout(root, 220, 600, a.fonts)
		return root
	}

	n := findKey(build(long), "wi")
	if n == nil {
		t.Fatal("缺少 wi 节点")
	}
	if n.Frame.H <= 38 {
		t.Fatalf("折行输入框高度应随行数增长（H = %.1f）", n.Frame.H)
	}

	// 空内容：保持最小高度 38
	n = findKey(build(""), "wi")
	if n == nil {
		t.Fatal("缺少 wi 节点")
	}
	if n.Frame.H < 38 || n.Frame.H > 40 {
		t.Fatalf("空文本应保持最小高度 38（H = %.1f）", n.Frame.H)
	}
}

// TestInputKeepsSingleLine ui.Input 不因改动而折行（高度仍为 38）。
func TestInputKeepsSingleLine(t *testing.T) {
	withTestFonts(t)
	a := currentApp.Load()
	const long = "这是一段很长的单行文本，用来验证普通输入框不会被改成折行显示。"
	root := Column(Input("in", long, "占位", nil)).St(St().W(200).Padding(10))
	a.root = root
	Layout(root, 220, 600, a.fonts)
	n := findKey(root, "in")
	if n == nil {
		t.Fatal("缺少 in 节点")
	}
	if !near(n.Frame.H, 38) {
		t.Fatalf("单行输入框高度应保持 38（H = %.1f）", n.Frame.H)
	}
}

// BenchmarkWrapRangesCached 折行区间缓存命中：输入框每帧重建都会走这条路。
// 加缓存前 1900 rune 的文本每次重算约 216µs / 2005 次分配。
func BenchmarkWrapRangesCached(b *testing.B) {
	withTestFonts(b)
	rs := []rune(strings.Repeat("这是一段用于测试折行与滚动性能的长文本，", 100))
	WrapRanges(rs, 700, 13, 400) // 预热缓存
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WrapRanges(rs, 700, 13, 400)
	}
}

// BenchmarkInputScrollCached 单行横向滚动起点缓存命中：光标不动时每帧都会问同一遍。
// 加缓存前 1900 rune 文本每次约 20µs / 55 次分配。
func BenchmarkInputScrollCached(b *testing.B) {
	withTestFonts(b)
	rs := []rune(strings.Repeat("这是一段用于测试折行与滚动性能的长文本，", 100))
	caret := len(rs)
	inputScrollStart(rs, caret, 700) // 预热缓存
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = inputScrollStart(rs, caret, 700)
	}
}
