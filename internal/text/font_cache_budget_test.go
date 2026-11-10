// guis — 文本缓存字节预算回归
//
// 旧实现只按「条数」限流（measure 8192 条 / runs 1024 条 + 8192 字形），
// 而聊天/日志类界面每条都是唯一长字符串：条数远没到上限，内存已经涨到几 MB。
// 这里验证改成字节预算后：既不超预算，也不像旧实现那样「一满就整体清空」。

package text

import (
	"fmt"
	"strings"
	"testing"
)

// TestMeasureCacheByteBudget 行宽缓存按字节预算淘汰，且不整体清空。
func TestMeasureCacheByteBudget(t *testing.T) {
	f := NewFont("Microsoft YaHei UI", 14, 400, 1)
	defer f.Close()

	line := strings.Repeat("行宽缓存预算测试", 32) // ≈256 字，约 700B/条
	for i := 0; i < 2000; i++ {
		f.Measure(fmt.Sprintf("%s#%06d", line, i))
	}

	f.mu.RLock()
	n, bytes := len(f.measure), f.measureBytes
	f.mu.RUnlock()

	if bytes > measureByteBudget*3/2 {
		t.Fatalf("行宽缓存 %d 字节，超过预算 %d 的 1.5 倍（%d 条）", bytes, measureByteBudget, n)
	}
	if n == 0 {
		t.Fatal("行宽缓存被整体清空：应淘汰约 1/8 并保留大部分")
	}
	if n > measureCacheMax {
		t.Fatalf("行宽缓存 %d 条超过条数上限 %d", n, measureCacheMax)
	}

	// 淘汰只是丢条目，不能把测量结果写坏：重算一次必须一致。
	s := fmt.Sprintf("%s#%06d", line, 1999)
	want := f.MeasureRunes([]rune(s))
	if got := f.Measure(s); got != want {
		t.Fatalf("淘汰后测量结果错误：got %.3f want %.3f", got, want)
	}
}

// TestRunCacheByteBudget 字形序列缓存按字节预算淘汰。
func TestRunCacheByteBudget(t *testing.T) {
	f := NewFont("Microsoft YaHei UI", 14, 400, 1)
	defer f.Close()

	// ASCII 长串：只光栅化 ~100 个字形，但每条缓存按字形数记账（≈6.5KB/条）。
	base := strings.Repeat("abcdefghij", 10) // 100 字符
	for i := 0; i < 400; i++ {
		f.GlyphsFor(fmt.Sprintf("%s%04d", base, i))
	}

	f.mu.RLock()
	n, glyphs, bytes := len(f.runs), f.runGlyphs, f.runBytes
	f.mu.RUnlock()

	if bytes > runByteBudget*3/2 {
		t.Fatalf("字形序列缓存 %d 字节，超过预算 %d 的 1.5 倍（%d 条 / %d 字形）",
			bytes, runByteBudget, n, glyphs)
	}
	if n == 0 {
		t.Fatal("字形序列缓存被整体清空：应淘汰约 1/8")
	}
	if n > runCacheMax || glyphs > runGlyphMax {
		t.Fatalf("字形序列缓存超上限：%d 条 / %d 字形", n, glyphs)
	}
}
