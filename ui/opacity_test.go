// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"testing"

	"guis/internal/gpu"
)

// fadedPaint 按不透明度调制颜色 alpha：0.16 的底板才不会盖成实心色块。
func TestFadedPaint(t *testing.T) {
	c := gpu.Paint{R: 1, G: 0.4, B: 0.3, A: 1}
	if got := fadedPaint(c, 0.16); got.A < 0.159 || got.A > 0.161 {
		t.Fatalf("alpha 应为 0.16，实际 %v", got.A)
	}
	// op 越界或颜色本就透明时原样返回，避免把全透明色「提亮」
	if got := fadedPaint(c, 1); got.A != 1 {
		t.Fatalf("op>=1 应保持 alpha=1，实际 %v", got.A)
	}
	if got := fadedPaint(gpu.Paint{}, 0.5); got.A != 0 {
		t.Fatalf("全透明色应保持 alpha=0，实际 %v", got.A)
	}
}

// 列表行图标块：底板 Fade(0.16) 要真的变淡，同色图标保持不透明 —— 否则图标整块看不见。
func TestTileFadeKeepsIconVisible(t *testing.T) {
	tile := fadedStyle(St().AbsFill().R(9).BgC("#6366f1").Fade(0.16))
	if tile.Bg.A < 0.159 || tile.Bg.A > 0.161 {
		t.Fatalf("底板 alpha 应为 0.16，实际 %v", tile.Bg.A)
	}
	icon := fadedStyle(St().Text(0, 0, "#6366f1"))
	if icon.Color.A < 0.99 {
		t.Fatalf("未设 Fade 的图标应保持不透明，实际 %v", icon.Color.A)
	}
}
func TestFadeSetsOpacity(t *testing.T) {
	n := Box().St(St().BgC("#6366f1").Fade(0.16))
	if n.Style.Opacity < 0.159 || n.Style.Opacity > 0.161 {
		t.Fatalf("Fade(0.16) 应写入 Opacity，实际 %v", n.Style.Opacity)
	}
	if got := fadedPaint(n.Style.Bg, n.Style.Opacity); got.A < 0.159 || got.A > 0.161 {
		t.Fatalf("底板 alpha 应为 0.16，实际 %v", got.A)
	}
}
