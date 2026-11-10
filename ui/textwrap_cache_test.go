// guis — 折行缓存行为回归

package ui

import "testing"

// TestWrapMemoKeepsBeyond512 超过 512 条后不能整体清空：
// 旧实现「满了就 make 一个新 map」，长页面每帧都要把整页文本重新折行。
func TestWrapMemoKeepsBeyond512(t *testing.T) {
	fc := newFontCache("Microsoft YaHei UI", 1)
	f := fc.get(14, 400)
	paras := wrapParagraphs(700)

	first := wrapLinesCached(f, paras[0], 420)
	if len(first) == 0 {
		t.Fatal("折行结果为空")
	}
	for _, s := range paras[1:] {
		_ = wrapLinesCached(f, s, 420)
	}
	again := wrapLinesCached(f, paras[0], 420)
	if len(again) == 0 || &first[0] != &again[0] {
		t.Fatal("超过 512 条后第 1 段不再命中缓存（说明缓存被整体清空了）")
	}
}

// TestWrapMemoInvalidatesOnFontGen 字体代次变化（缩放 / 锐度重建）后必须重算。
func TestWrapMemoInvalidatesOnFontGen(t *testing.T) {
	fc := newFontCache("Microsoft YaHei UI", 1)
	f := fc.get(14, 400)
	a := wrapLinesCached(f, "折行缓存代次测试文本", 200)
	if len(a) == 0 {
		t.Fatal("折行结果为空")
	}
	f.SetScale(2)
	b := wrapLinesCached(f, "折行缓存代次测试文本", 200)
	if len(b) == 0 || &a[0] == &b[0] {
		t.Fatal("字体代次变化后不应复用旧的折行结果")
	}
}
