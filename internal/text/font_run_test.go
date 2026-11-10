// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package text

import (
	"fmt"
	"testing"
)

// TestGlyphsForMatchesGlyphFor 字形序列缓存必须与逐字查表结果完全一致。
// 绘制路径改用 GlyphsFor 后，任何偏差都会直接表现为错字/串位。
func TestGlyphsForMatchesGlyphFor(t *testing.T) {
	f := NewFont("Microsoft YaHei UI", 16, 400, 1)
	defer f.Close()

	const s = "guis 渲染性能 Glyph 123"
	got := f.GlyphsFor(s)
	rs := []rune(s)
	if len(got) != len(rs) {
		t.Fatalf("字形数 = %d，期望 %d", len(got), len(rs))
	}
	for i, r := range rs {
		want := f.GlyphFor(r)
		if got[i] != *want {
			t.Fatalf("第 %d 个字符 %q 字形不一致:\n got=%+v\nwant=%+v", i, r, got[i], *want)
		}
	}

	// 第二次必须命中缓存并返回同一份数据（绘制热路径依赖零额外分配）
	again := f.GlyphsFor(s)
	if len(again) != len(got) || &again[0] != &got[0] {
		t.Fatal("第二次 GlyphsFor 应直接返回缓存切片")
	}
}

// TestGlyphsForInvalidatedOnAtlasGrow 图集扩容后 UV 全部失效，缓存必须丢弃。
// 旧字形在图集中的位置会变，若继续用旧 UV 就会画出错字/乱码。
func TestGlyphsForInvalidatedOnAtlasGrow(t *testing.T) {
	f := NewFont("Microsoft YaHei UI", 16, 400, 1)
	defer f.Close()

	cached := f.GlyphsFor("Aa")
	if len(cached) == 0 {
		t.Fatal("未生成字形序列")
	}

	// 模拟图集扩容（与 allocCell 的扩容路径一致：重建图集并把一切缓存作废）
	f.mu.Lock()
	f.growAtlas(minAtlasDim * 2)
	f.mu.Unlock()

	after := f.GlyphsFor("Aa")
	if len(after) == 0 {
		t.Fatal("扩容后未重新生成字形序列")
	}
	if &after[0] == &cached[0] {
		t.Fatal("扩容后不应复用旧缓存（UV 已失效）")
	}
	if want := f.GlyphFor('A'); after[0] != *want {
		t.Fatalf("扩容后字形应与重新光栅化的结果一致:\n got=%+v\nwant=%+v", after[0], *want)
	}
}

// TestGlyphsForCacheBounded 缓存必须有上限，长列表/日志类界面不能无界增长。
func TestGlyphsForCacheBounded(t *testing.T) {
	f := NewFont("Microsoft YaHei UI", 14, 400, 1)
	defer f.Close()

	for i := 0; i < runCacheMax+64; i++ {
		f.GlyphsFor(fmt.Sprintf("行 %d 的文本内容", i))
	}
	f.mu.RLock()
	n := len(f.runs)
	f.mu.RUnlock()
	if n > runCacheMax {
		t.Fatalf("字形序列缓存条数 = %d，超过上限 %d", n, runCacheMax)
	}
}

// BenchmarkGlyphPerRune 旧绘制路径：每个字符一次 GlyphFor（读锁 + map 哈希）。
func BenchmarkGlyphPerRune(b *testing.B) {
	f := NewFont("Microsoft YaHei UI", 13, 400, 1)
	defer f.Close()
	lines := make([]string, 0, 300)
	for i := 0; i < 300; i++ {
		lines = append(lines, fmt.Sprintf("第 %d 行：这是一段典型的中文说明文字，包含数字 %d 与符号。", i, i*13))
	}
	// 预热：先把字形全部光栅化，只测查找开销
	for _, ln := range lines {
		for _, r := range ln {
			f.GlyphFor(r)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		acc := float32(0)
		for _, ln := range lines {
			for _, r := range ln {
				acc += f.GlyphFor(r).Advance
			}
		}
		_ = acc
	}
}

// BenchmarkGlyphRun 新绘制路径：按字符串取字形序列（一次 map 查找 + 顺序读）。
func BenchmarkGlyphRun(b *testing.B) {
	f := NewFont("Microsoft YaHei UI", 13, 400, 1)
	defer f.Close()
	lines := make([]string, 0, 300)
	for i := 0; i < 300; i++ {
		lines = append(lines, fmt.Sprintf("第 %d 行：这是一段典型的中文说明文字，包含数字 %d 与符号。", i, i*13))
	}
	for _, ln := range lines {
		f.GlyphsFor(ln)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		acc := float32(0)
		for _, ln := range lines {
			for _, g := range f.GlyphsFor(ln) {
				acc += g.Advance
			}
		}
		_ = acc
	}
}
