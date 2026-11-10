// guis — 文本折行缓存基准（字体缓存复用，模拟真实运行时）
//
// 旧实现 wrapMemo 上限 512 且超出整体清空：长页面一屏段落数就能超过上限，
// 每帧「清空 → 全量重新折行」。这里用 600 段互不相同的文本量化两种行为。

package ui

import (
	"fmt"
	"strings"
	"testing"
)

// wrapParagraphs 造 n 段互不相同的文本（模拟长会话 / 长文档）。
func wrapParagraphs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("第 %d 段：%s", i, strings.Repeat("中文折行测试文本，包含标点与英文 mixed text。", 4))
	}
	return out
}

// BenchmarkWrapCacheHit 600 段全部命中缓存（新实现：LRU 4096，不整体清空）。
func BenchmarkWrapCacheHit(b *testing.B) {
	fc := newFontCache("Microsoft YaHei UI", 1)
	f := fc.get(14, 400)
	paras := wrapParagraphs(600)
	for _, s := range paras { // 预热
		_ = wrapLinesCached(f, s, 420)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range paras {
			_ = wrapLinesCached(f, s, 420)
		}
	}
}

// BenchmarkWrapCacheThrash 每轮清空缓存（等价旧实现「超出 512 条整体清空」）。
func BenchmarkWrapCacheThrash(b *testing.B) {
	fc := newFontCache("Microsoft YaHei UI", 1)
	f := fc.get(14, 400)
	paras := wrapParagraphs(600)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clearWrapMemo()
		for _, s := range paras {
			_ = wrapLinesCached(f, s, 420)
		}
	}
}
