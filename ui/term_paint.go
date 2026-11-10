// guis — 伪终端组件（三）：绘制
//
// 按「单元格网格」自绘：背景按同色连续段合并成一个矩形，文字按同色同属性
// 连续段合并成一个 TextRun，因此一屏 24 行 × 80 列只有几十条绘制指令，
// 不会为每个字符单独提交顶点。
package ui

import (
	"guis/internal/geom"
	"guis/internal/gpu"
	"guis/internal/text"
)

// termRenderWin 解析阶段算出的可见窗口。
type termRenderWin struct {
	f     *text.Font
	cellW float32
	cellH float32
	cols  int
	rows  int
	top   int // 第一行全局行号（回看缓冲内偏移）
}

// termMetrics 依据控件矩形推导单元格尺寸与列数/行数。
func termMetrics(a *App, st *TermState, r geom.Rect) termRenderWin {
	size := st.FontSize
	if size <= 0 {
		size = 13
	}
	family := st.FontFamily
	if family == "" {
		family = codeFontFamily()
	}
	f := a.fonts.getFamily(size, 400, family)
	if f == nil {
		return termRenderWin{}
	}
	cellW := f.Measure("M")
	if cellW <= 0 {
		cellW = size * 0.6
	}
	cellH := f.LineHeight
	if cellH <= 0 {
		cellH = size * 1.35
	}
	cols := st.Cols
	rows := st.Rows
	if st.AutoSize && r.W > 0 && r.H > 0 {
		cols = int((r.W - termPadX*2) / cellW)
		rows = int((r.H - termPadY*2) / cellH)
		if cols < 2 {
			cols = 2
		}
		if rows < 2 {
			rows = 2
		}
	}
	return termRenderWin{f: f, cellW: cellW, cellH: cellH, cols: cols, rows: rows}
}

// termFgPaint 单元格前景色（xterm 风格：粗体→亮色、暗淡降低 alpha、反显取背景色）。
func termFgPaint(c termCell, th *TermTheme) gpu.Paint {
	fg := c.fg
	if c.attr&termAttrBold != 0 && fg >= termColorIndexedBase && fg < termColorIndexedBase+8 {
		// xterm.js 默认 drawBoldTextInBrightColors：0-7 号色加粗时用 8-15 号亮色
		fg = termIndexed(int(fg-termColorIndexedBase) + 8)
	}
	if c.attr&termAttrReverse != 0 {
		p := termPaintOf(c.bg, th.Bg, th)
		if c.attr&termAttrBold != 0 && c.bg != termColorDefault {
			p = termPaintOf(c.bg, th.Bg, th)
		}
		return p
	}
	p := termPaintOf(fg, th.Fg, th)
	if c.attr&termAttrFaint != 0 {
		p.A *= 0.55
	}
	return p
}

// 终端内边距（逻辑像素）。
const (
	termPadX float32 = 8
	termPadY float32 = 6
	// termScrollbarW 回看滚动条宽度。
	termScrollbarW float32 = 6
)

// paintTerm 绘制终端（app.paint 的 KTerm 分支）。
func (a *App) paintTerm(n *Node, r geom.Rect) {
	st := n.Term
	if st == nil {
		return
	}
	st.ensure()
	theme := st.Theme
	if theme.Bg.A == 0 && theme.Fg.A == 0 {
		theme = DefaultTermTheme()
	}
	m := termMetrics(a, st, r)
	if m.f == nil {
		return
	}
	if st.AutoSize {
		// 尺寸变化在绘制前同步给 PTY（业务多用于改窗口大小）
		st.applySize(m.cols, m.rows, true)
	}

	// 背景
	a.r.Rect(gpu.Shape{Rect: r, Fill: theme.Bg})

	st.mu.Lock()
	sc := st.sc
	if sc == nil {
		st.mu.Unlock()
		return
	}
	back := len(sc.scrollBack)
	scroll := st.scrollOff
	if scroll > back {
		scroll = back
		st.scrollOff = scroll
	}
	rows := sc.rows
	cols := sc.cols
	top := back - scroll
	selActive := st.sel.valid()
	sx0, sy0, sx1, sy1 := st.sel.order()
	curX, curY := sc.curX, sc.curY
	curVisible := sc.cursorVisible && scroll == 0
	curStyle := sc.cursorStyle
	st.mu.Unlock()
	focused := IsFocused(st.Key)

	originX := r.X + termPadX
	originY := r.Y + termPadY
	cellW, cellH := m.cellW, m.cellH
	textRect := geom.Rect{X: r.X + 2, Y: r.Y, W: r.W - 4 - termScrollbarW, H: r.H}
	a.r.PushClip(textRect)
	defer a.r.PopClip()

	runes := make([]rune, 0, cols)
	flushText := func(x float32, y float32, f *text.Font, color gpu.Paint) {
		if len(runes) == 0 {
			return
		}
		a.r.TextRun(gpu.TextRun{X: x, Y: y, Text: string(runes), Font: f, Color: color})
		runes = runes[:0]
	}

	for vy := 0; vy < rows; vy++ {
		gy := top + vy
		var line []termCell
		if gy < back {
			line = sc.scrollBack[gy]
		} else {
			row := gy - back
			if row < 0 || row >= rows {
				continue
			}
			line = sc.row(row)
		}
		if line == nil {
			continue
		}
		y := originY + float32(vy)*cellH

		// ---- 背景（同色合并）----
		bgStart := 0
		var bgCur gpu.Paint
		bgOn := false
		for x := 0; x < cols && x < len(line); x++ {
			c := line[x]
			bg := theme.Bg
			if c.attr&termAttrReverse != 0 {
				bg = termPaintOf(c.fg, theme.Fg, &theme)
			} else if c.bg != termColorDefault {
				bg = termPaintOf(c.bg, theme.Bg, &theme)
			}
			same := bgOn && samePaint(bg, bgCur)
			if !same {
				if bgOn && bgCur.A > 0 && bgCur != theme.Bg {
					a.r.Rect(gpu.Shape{Rect: geom.Rect{X: originX + float32(bgStart)*cellW, Y: y, W: float32(x-bgStart) * cellW, H: cellH}, Fill: bgCur})
				}
				bgStart, bgCur, bgOn = x, bg, true
			}
		}
		if bgOn && bgCur.A > 0 && bgCur != theme.Bg {
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: originX + float32(bgStart)*cellW, Y: y, W: float32(cols-bgStart) * cellW, H: cellH}, Fill: bgCur})
		}

		// ---- 选区 ----
		if selActive && gy >= sy0 && gy <= sy1 {
			from, to := 0, cols
			if gy == sy0 {
				from = sx0
			}
			if gy == sy1 {
				to = sx1 + 1
			}
			if from < 0 {
				from = 0
			}
			if to > cols {
				to = cols
			}
			if to > from {
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: originX + float32(from)*cellW, Y: y, W: float32(to-from) * cellW, H: cellH}, Fill: theme.Selection})
			}
		}

		// ---- 光标（xterm 风格：聚焦实心、失焦空心框）----
		isCurRow := scroll == 0 && vy == curY
		if curVisible && isCurRow {
			cx := originX + float32(curX)*cellW
			if !focused {
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx, Y: y, W: cellW, H: cellH}, Stroke: 1, StrokeColor: theme.Cursor})
			} else {
				switch curStyle {
				case 1: // 竖线
					a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx, Y: y, W: 2, H: cellH}, Fill: theme.Cursor})
				case 2: // 下划线
					a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx, Y: y + cellH - 2, W: cellW, H: 2}, Fill: theme.Cursor})
				default: // 方块
					a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx, Y: y, W: cellW, H: cellH}, Fill: theme.Cursor})
				}
			}
		}

		// ---- 文字（同色合并，xterm 风格：粗体走亮色而非字重）----
		runStart := -1
		var runColor gpu.Paint
		for x := 0; x < cols && x < len(line); x++ {
			c := line[x]
			if c.attr&termAttrWideTail != 0 {
				continue
			}
			ch := c.ch
			if ch == 0 {
				ch = ' '
			}
			fg := termFgPaint(c, &theme)
			if c.attr&termAttrConceal != 0 {
				ch = ' '
			}
			// 光标覆盖的字符用反显色
			if curVisible && isCurRow && x == curX && curStyle == 0 {
				fg = theme.CursorTxt
			}
			if runStart < 0 {
				runStart, runColor = x, fg
				runes = append(runes, ch)
				continue
			}
			if !samePaint(fg, runColor) {
				flushText(originX+float32(runStart)*cellW, y, m.f, runColor)
				runStart, runColor = x, fg
			}
			runes = append(runes, ch)
		}
		if runStart >= 0 {
			flushText(originX+float32(runStart)*cellW, y, m.f, runColor)
		}

		// ---- 下划线 / 删除线 ----
		for x := 0; x < cols && x < len(line); x++ {
			c := line[x]
			if c.attr&termAttrUnderline == 0 && c.attr&termAttrStrike == 0 {
				continue
			}
			if c.attr&termAttrWideTail != 0 {
				continue
			}
			col := termPaintOf(c.fg, theme.Fg, &theme)
			if c.attr&termAttrReverse != 0 {
				col = termPaintOf(c.bg, theme.Bg, &theme)
			}
			px := originX + float32(x)*cellW
			if c.attr&termAttrUnderline != 0 {
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: px, Y: y + cellH - 2, W: cellW, H: 1}, Fill: col})
			}
			if c.attr&termAttrStrike != 0 {
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: px, Y: y + cellH*0.55, W: cellW, H: 1}, Fill: col})
			}
		}
	}

	// ---- 回看滚动条 ----
	total := back + rows
	if total > rows && rows > 0 {
		track := geom.Rect{X: r.Right() - termScrollbarW - 2, Y: r.Y + 2, W: termScrollbarW, H: r.H - 4}
		thumbH := track.H * float32(rows) / float32(total+rows)
		if thumbH < 24 {
			thumbH = 24
		}
		// scroll=0 → 滑块贴底；scroll=back → 贴顶
		frac := float32(0)
		if back > 0 {
			frac = float32(scroll) / float32(back)
		}
		y := track.Y + (track.H-thumbH)*(1-frac)
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: track.X, Y: y, W: track.W, H: thumbH}, Radius: [4]float32{track.W / 2, track.W / 2, track.W / 2, track.W / 2}, Fill: theme.Scrollbar})
	}
}

// samePaint 两个绘制色是否相同。
func samePaint(a, b gpu.Paint) bool {
	const eps = 0.004
	d := func(x, y float32) bool {
		if x > y {
			return x-y < eps
		}
		return y-x < eps
	}
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && d(a.A, b.A)
}
