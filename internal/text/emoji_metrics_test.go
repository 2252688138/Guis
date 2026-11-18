//go:build windows

package text

import "testing"

// TestEmojiInkInsideLineBox 彩色 Emoji 必须落在文字行框内。
//
// 回归用例：Segoe UI Emoji 的字形按「基线在字身上部」绘制（19px 的墨迹跨基线
// -16..+2.7），旧实现直接沿用它的基线，Emoji 会掉到行框底部并溢出下行——界面上
// 所有 Emoji 图标（🎬 / 🔍 / 🔀 …）看起来永远贴着底边。
func TestEmojiInkInsideLineBox(t *testing.T) {
	for _, size := range []float32{12, 13, 15, 19} {
		f := NewFont("Microsoft YaHei UI", size, 400, 1)
		if f == nil || f.LineHeight <= 0 {
			t.Skip("取不到字体")
		}
		sc := f.Scale()
		if sc <= 0 {
			sc = 1
		}
		for _, r := range []rune{'🎬', '🔍', '🔀', '🎵'} {
			g := f.GlyphFor(r)
			if !g.Valid || !g.Color {
				t.Skipf("%c 未走彩色 Emoji 路径（可能缺少 Segoe UI Emoji）", r)
			}
			top := f.Ascent + g.Top/sc
			bottom := top + g.H/sc
			if top < -1 {
				t.Errorf("size %.0f %c: 墨迹顶部 %.1f 超出行框（行高 %.1f）", size, r, top, f.LineHeight)
			}
			if bottom > f.LineHeight+1 {
				t.Errorf("size %.0f %c: 墨迹底部 %.1f 掉出行框（行高 %.1f）", size, r, bottom, f.LineHeight)
			}
		}
	}
}
