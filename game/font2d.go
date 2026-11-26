// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"image"
	"sync"
)

// BitmapFont 位图字体（图集字形），用于 HUD / 分数 / 对话文本。
//
// 内置一套 5×7 像素字体（零外部资源），可直接用：
//
//	game.BuiltinFont().DrawText(img, "SCORE 1200", 8, 8, 2, game.RGB(255, 255, 255))
//
// 自制字体：把字形按网格排成一张纹理（白字透明底，便于染色），
// 然后用 NewBitmapFont 指定格宽、格高与列数。
type BitmapFont struct {
	// Tex 字形图集（白字 + 透明底最佳，颜色由 DrawText 的 color 乘入）。
	Tex *Texture
	// CellW, CellH 图集中每个格子的像素尺寸（含间隔）。
	CellW, CellH int
	// GlyphW, GlyphH 实际字形尺寸（0 = 等于 CellW / CellH）。
	GlyphW, GlyphH int
	// Cols 图集列数。
	Cols int
	// First, Last 字符区间（默认 32~126）。
	First, Last rune
	// Index 自定义字符→图集格序号映射（非 nil 时优先于 First/Last 区间）。
	// TTF 烘焍（LoadTTF）会给任意字符集建这张表。
	Index map[rune]int
	// Spacing 额外字间距（像素）。
	Spacing float32
	// LineGap 额外行距（像素）。
	LineGap float32
}

// NewBitmapFont 创建位图字体（first/last 为字符区间，常用 32,126）。
func NewBitmapFont(tex *Texture, cellW, cellH, cols int, first, last rune) *BitmapFont {
	if cols <= 0 {
		cols = 16
	}
	if cellW <= 0 {
		cellW = 6
	}
	if cellH <= 0 {
		cellH = 8
	}
	if last <= first {
		last = first + 1
	}
	return &BitmapFont{
		Tex: tex, CellW: cellW, CellH: cellH,
		GlyphW: cellW, GlyphH: cellH,
		Cols: cols, First: first, Last: last,
	}
}

// glyphW 实际字形宽（含默认值）。
func (f *BitmapFont) glyphW() float32 {
	if f.GlyphW > 0 {
		return float32(f.GlyphW)
	}
	return float32(f.CellW)
}

// glyphH 实际字形高（含默认值）。
func (f *BitmapFont) glyphH() float32 {
	if f.GlyphH > 0 {
		return float32(f.GlyphH)
	}
	return float32(f.CellH)
}

// advance 单字符推进量（像素，未缩放）。
func (f *BitmapFont) advance() float32 { return f.glyphW() + f.Spacing }

// lineHeight 行高（像素，未缩放）。
func (f *BitmapFont) lineHeight() float32 { return f.glyphH() + f.LineGap }

// Glyph 返回字符在图集中的源矩形（找不到时尝试大写映射）。
func (f *BitmapFont) Glyph(ch rune) (Rect2, bool) {
	if f == nil || f.Tex == nil || f.Cols <= 0 {
		return Rect2{}, false
	}
	idx := -1
	switch {
	case f.Index != nil:
		if i, ok := f.Index[ch]; ok {
			idx = i
		} else if up := upperRune(ch); up != ch {
			if i, ok := f.Index[up]; ok {
				idx = i
			}
		}
	case ch >= f.First && ch < f.Last:
		idx = int(ch - f.First)
	default:
		if up := upperRune(ch); up != ch && up >= f.First && up < f.Last {
			idx = int(up - f.First)
		}
	}
	if idx < 0 {
		return Rect2{}, false
	}
	col := idx % f.Cols
	row := idx / f.Cols
	x := float32(col * f.CellW)
	y := float32(row * f.CellH)
	if x+float32(f.CellW) > float32(f.Tex.W) || y+float32(f.CellH) > float32(f.Tex.H) {
		return Rect2{}, false
	}
	return Rect2{X: x, Y: y, W: f.glyphW(), H: f.glyphH()}, true
}

// Measure 计算文本尺寸（支持 \n 换行）。
func (f *BitmapFont) Measure(text string, scale float32) (w, h float32) {
	if f == nil {
		return 0, 0
	}
	if scale <= 0 {
		return 0, 0
	}
	var lineW float32
	lines := 1
	for _, ch := range text {
		switch ch {
		case '\n':
			if lineW > w {
				w = lineW
			}
			lineW = 0
			lines++
		default:
			lineW += f.advance()
		}
	}
	if lineW > w {
		w = lineW
	}
	return w * scale, float32(lines) * f.lineHeight() * scale
}

// DrawText 绘制文本，返回实际宽度（像素）。
//
// scale 为整体缩放倍数（1 = 原始像素，2 = 双倍），color 为乘色（A=0 按白色）。
func (f *BitmapFont) DrawText(dst *image.RGBA, text string, x, y, scale float32, color RGBA) float32 {
	if f == nil || f.Tex == nil || dst == nil {
		return 0
	}
	if scale <= 0 {
		scale = 1
	}
	startX := x
	cx, cy := x, y
	gw, gh := f.glyphW()*scale, f.glyphH()*scale
	adv := f.advance() * scale
	lh := f.lineHeight() * scale
	for _, ch := range text {
		switch ch {
		case '\n':
			cx = startX
			cy += lh
			continue
		case '\t':
			cx += adv * 4
			continue
		}
		if ch == ' ' {
			cx += adv
			continue
		}
		src, ok := f.Glyph(ch)
		if !ok {
			cx += adv
			continue
		}
		DrawSprite(dst, Sprite{
			Pos:     V2(cx, cy),
			Size:    V2(gw, gh),
			Texture: f.Tex,
			Src:     src,
			Color:   color,
		})
		cx += adv
	}
	return cx - startX
}

// DrawTextCentered 以 cx 为中心水平居中绘制文本。
func (f *BitmapFont) DrawTextCentered(dst *image.RGBA, text string, cx, y, scale float32, color RGBA) {
	if f == nil {
		return
	}
	w, _ := f.Measure(text, scale)
	f.DrawText(dst, text, cx-w*0.5, y, scale, color)
}

// DrawTextShadow 带 1 像素（按 scale 缩放）硬阴影的文本，适合叠加在亮背景上。
func (f *BitmapFont) DrawTextShadow(dst *image.RGBA, text string, x, y, scale float32, color RGBA, shadow RGBA) float32 {
	if f == nil {
		return 0
	}
	if scale <= 0 {
		scale = 1
	}
	f.DrawText(dst, text, x+scale, y+scale, scale, shadow)
	return f.DrawText(dst, text, x, y, scale, color)
}

// upperRune ASCII 小写转大写（其他字符原样返回）。
func upperRune(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

var (
	builtinFontOnce sync.Once
	builtinFontInst *BitmapFont
)

// BuiltinFont 内置 5×7 像素字体（首次调用生成并缓存，可直接画 HUD / 调试信息）。
func BuiltinFont() *BitmapFont {
	builtinFontOnce.Do(func() {
		builtinFontInst = NewBitmapFont(builtinFontTexture(), 6, 8, 16, 32, 127)
		builtinFontInst.GlyphW, builtinFontInst.GlyphH = 5, 7
	})
	return builtinFontInst
}

// builtinFontTexture 把内置 5×7 点阵画成字形图集（白字 + 透明底）。
func builtinFontTexture() *Texture {
	const cellW, cellH = 6, 8
	const cols = 16
	const count = 95
	rows := (count + cols - 1) / cols
	t := NewTexture(cols*cellW, rows*cellH)
	white := RGB(255, 255, 255)
	for i := 0; i < count; i++ {
		glyph, ok := builtinGlyphRows[rune(32+i)]
		if !ok {
			// 小写统一用大写字形
			glyph, ok = builtinGlyphRows[upperRune(rune(32+i))]
		}
		if !ok {
			continue
		}
		cx := (i % cols) * cellW
		cy := (i / cols) * cellH
		for ry := 0; ry < 7; ry++ {
			bits := glyph[ry]
			for rx := 0; rx < 5; rx++ {
				if bits&(1<<(4-uint(rx))) != 0 {
					t.Set(cx+rx, cy+ry, white)
				}
			}
		}
	}
	return t
}
