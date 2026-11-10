// guis — 推进量缓存（Advance）回归

package text

import (
	"math"
	"testing"
)

// TestAdvanceMatchesGlyphFor 推进量缓存必须与 GlyphFor 结果一致（含缩放）。
func TestAdvanceMatchesGlyphFor(t *testing.T) {
	f := NewFont("Microsoft YaHei UI", 14, 400, 1.5)
	for _, r := range []rune("abc XYZ 0123 中文测试「」，。…🙂") {
		want := f.GlyphFor(r).Advance / f.Scale()
		got := f.Advance(r)
		if math.Abs(float64(got-want)) > 0.01 {
			t.Fatalf("字符 %q 推进量 = %.3f，期望 %.3f", r, got, want)
		}
		if got2 := f.Advance(r); got2 != got {
			t.Fatalf("字符 %q 二次取值 = %.3f，首次 %.3f", r, got2, got)
		}
	}
}

// TestAdvanceCacheResetOnScaleChange 缩放变化后推进量缓存必须清空。
// （逻辑推进量与缩放无关，但像素尺寸 / hinting 变了，必须按新 face 重算。）
func TestAdvanceCacheResetOnScaleChange(t *testing.T) {
	f := NewFont("Microsoft YaHei UI", 14, 400, 1)
	if v := f.Advance('测'); v <= 0 {
		t.Fatalf("推进量应为正：%v", v)
	}
	if v := f.Advance('A'); v <= 0 {
		t.Fatalf("ASCII 推进量应为正：%v", v)
	}
	if f.advSnap.Load() == nil || f.advASCII.Load() == nil {
		t.Fatal("推进量缓存未建立")
	}
	f.SetScale(2)
	if f.advSnap.Load() != nil || f.advASCII.Load() != nil {
		t.Fatal("缩放变化后推进量缓存应被清空")
	}
	if v := f.Advance('测'); v <= 0 {
		t.Fatalf("重建后推进量应为正：%v", v)
	}
}

// TestAdvanceGenChangesOnRebuild 字体代次必须在 face 重建后变化（折行缓存靠它失效）。
func TestAdvanceGenChangesOnRebuild(t *testing.T) {
	a := NewFont("Microsoft YaHei UI", 14, 400, 1)
	b := NewFont("Microsoft YaHei UI", 14, 400, 1)
	if a.Gen() == b.Gen() {
		t.Fatal("不同字体实例的代次不应相同")
	}
	g := a.Gen()
	a.SetScale(2)
	if a.Gen() == g {
		t.Fatal("缩放重建后代次应变化")
	}
}
