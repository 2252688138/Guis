// guis — 折行缓存的近似 LRU 淘汰回归
//
// 旧实现按 map 随机顺序删条目：热点段落（每帧都折的）和一次性段落被删概率相同，
// 长文档里刚折好行的段落下一轮就可能被丢掉。改成「采样最冷」后，热点应长期保活。

package ui

import (
	"fmt"
	"testing"
)

func TestWrapCacheEvictsColdest(t *testing.T) {
	clearWrapMemo()

	fc := newFontCache("Microsoft YaHei UI", 1)
	f := fc.get(14, 400)
	if f == nil {
		t.Skip("测试环境没有可用字体")
	}

	hot := "热点段落：每帧都要折行的标题"
	hotKey := wrapKey{gen: f.Gen(), s: hot, w: 800} // 400px * 2（与 wrapLinesCached 的量化一致）

	// 模拟真实场景：一边滚动不断跑出新段落，一边每帧都在画热点段落。
	// 近似 LRU 下热点应长期存活；旧实现（随机删）里它迟早会被随手删掉。
	for i := 0; i < wrapMemoMax+600; i++ {
		_ = wrapLinesCached(f, fmt.Sprintf("冷门段落 %d：一次性文本", i), 400)
		if i%8 == 0 {
			_ = wrapLinesCached(f, hot, 400)
		}
	}

	wrapMu.RLock()
	_, alive := wrapMemo[hotKey]
	total := len(wrapMemo)
	wrapMu.RUnlock()

	if !alive {
		t.Fatalf("被持续访问的热点段落被淘汰了（缓存 %d 条）", total)
	}
	if total > wrapMemoMax {
		t.Fatalf("缓存条数超上限：%d > %d", total, wrapMemoMax)
	}
}

// 清空后必须真的空（缩放/锐度变化时会调用）。
func TestWrapCacheClear(t *testing.T) {
	fc := newFontCache("Microsoft YaHei UI", 1)
	f := fc.get(14, 400)
	if f == nil {
		t.Skip("测试环境没有可用字体")
	}
	_ = wrapLinesCached(f, "一些文本", 300)
	clearWrapMemo()

	wrapMu.RLock()
	n := len(wrapMemo)
	lineBudget := wrapLineCount
	wrapMu.RUnlock()

	if n != 0 || lineBudget != 0 {
		t.Fatalf("清空后仍有 %d 条 / %d 行", n, lineBudget)
	}
}

// 同一文本不同宽度必须各自独立（缓存键带宽度，换宽度不能吃到旧结果）。
func TestWrapCacheWidthKeyed(t *testing.T) {
	clearWrapMemo()

	fc := newFontCache("Microsoft YaHei UI", 1)
	f := fc.get(14, 400)
	if f == nil {
		t.Skip("测试环境没有可用字体")
	}
	text := "宽度不同时折行结果必须不同，这一段要足够长以触发多次折行。"

	narrow := wrapLinesCached(f, text, 120)
	wide := wrapLinesCached(f, text, 900)

	if len(wide) > len(narrow) {
		t.Fatalf("宽容器行数不应多于窄容器：narrow=%d wide=%d", len(narrow), len(wide))
	}
	// 再取一次窄宽度，必须与第一次一致（缓存命中不串味）
	again := wrapLinesCached(f, text, 120)
	if len(again) != len(narrow) {
		t.Fatalf("同一宽度两次结果不一致：%d vs %d", len(again), len(narrow))
	}
}
