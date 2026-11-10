// guis — 原生代码编辑器（三）：绘制（行号 / 高亮 / 选区 / 光标 / 小地图 / 滚动条）
//
// 绘制按「视觉行」推进（软换行一行可占多行、折叠行整段跳过），但代价仍然只与
// 视口有关：十万行文档每帧只画 30~60 行。行高亮来自行缓存，列宽走累计表，
// 小地图固定采样，因此行数增加不会改变每帧成本。
package ui

import (
	"strconv"
	"strings"

	"guis/internal/geom"
	"guis/internal/gpu"
	"guis/internal/text"
)

// codeMetrics 绘制期度量（每帧算一次）。
type codeMetrics struct {
	font      *text.Font
	lh        float32
	gutterW   float32
	minimapW  float32
	textX     float32
	textW     float32
	contentH  float32
	firstLine int
	lastLine  int
}

// codeMeasure 文本测量入口（单元测试可替换：无窗口时 TextWidthOf 恒为 0）。
var codeMeasure = func(s string, size float32, family string) float32 {
	return TextWidthOf(s, size, 400, family)
}

// measure 绘制 / 命中测试共用的行宽测量。
func (e *CodeEditorState) measure(s string) float32 {
	if s == "" {
		return 0
	}
	return codeMeasure(s, e.FontSize, e.FontFamily)
}

// gutterTotal 行号栏总宽（含左右内边距）。
func (e *CodeEditorState) gutterTotal() float32 {
	if !e.ShowGutter {
		return 0
	}
	if e.gutterW > 0 {
		return e.gutterW
	}
	digits := len(strconv.Itoa(len(e.Lines)))
	return e.measure(strings.Repeat("0", digits)) + codeGutterPad*2
}

// textAreaW 文本区宽度（扣除行号与小地图）。
func (e *CodeEditorState) textAreaW() float32 {
	if e.textW > 0 {
		return e.textW
	}
	return e.viewW - e.gutterTotal()
}

// caretXVisual 光标在文本坐标系里的 x（相对文本行首，未减 ScrollX）。
func (e *CodeEditorState) caretXVisual() float32 {
	if e.Wrap {
		seg := e.segOfCol(e.Pos.Line, e.Pos.Col)
		from, _ := e.segRange(e.Pos.Line, seg)
		return e.xOfCol(e.Pos.Line, e.Pos.Col) - e.xOfCol(e.Pos.Line, from)
	}
	return e.xOfCol(e.Pos.Line, e.Pos.Col)
}

// xOfCol 行内第 col 个 rune 的 x（累计列宽缓存，一行只算一次）。
func (e *CodeEditorState) xOfCol(line, col int) float32 {
	if line < 0 || line >= len(e.Lines) {
		return 0
	}
	colX := e.colXOf(line)
	if colX == nil {
		return 0
	}
	col = codeClamp(col, 0, len(colX)-1)
	return colX[col]
}

// buildColX 逐字累计宽度（Measure 按单字缓存，总体 O(n)）。
//
// Tab 统一按「TabSize 个空格」展开：绘制时也用同样宽度的空格代替 Tab，
// 这样列宽、光标、选区与缩进参考线才能对齐（否则 Tab 被字体当成窄字形，
// 竖线就会穿进代码正文里）。
func (e *CodeEditorState) buildColX(runes []rune) []float32 {
	out := make([]float32, len(runes)+1)
	tabW := float32(e.tabSize()) * e.measure(" ")
	x := float32(0)
	for i, r := range runes {
		if r == '\t' {
			x += tabW
		} else {
			x += e.measure(string(r))
		}
		out[i+1] = x
	}
	return out
}

// expandTabs 把 Tab 展开成空格（绘制与测量共用）。
func (e *CodeEditorState) expandTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	return strings.ReplaceAll(s, "\t", strings.Repeat(" ", e.tabSize()))
}

// viewState 可见文档行区间（兼容旧调用方：诊断扫描 / 横向滚动条）。
func (e *CodeEditorState) viewState(h float32) (first, last int) {
	first, last, _ = e.lineRange(h)
	return
}

// codeNow 编辑器动画时钟（测试可替换：无窗口时 AnimTime 恒为 0）。
var codeNow = AnimTime

// caretVisible 光标此刻是否应该画出来（标准闪烁：常亮 0.5s 后 530ms 亮 / 530ms 灭）。
func (e *CodeEditorState) caretVisible() bool {
	el := codeNow() - e.blinkAnchor
	if el < 0 {
		el = 0
	}
	if el < 0.5 {
		return true
	}
	ph := (el - 0.5) - float32(int((el-0.5)/1.06))*1.06
	return ph < 0.53
}

// indentWidthPx 行首空白占的像素宽度（Tab 按档位对齐，而不是当四个空格字宽），
// 缩进参考线据此定位，否则带 Tab 的文件里竖线会穿进代码正文。
func (e *CodeEditorState) indentWidthPx(line string, tabPx float32) float32 {
	if tabPx <= 0 {
		return 0
	}
	spaceW := tabPx / float32(e.tabSize())
	x := float32(0)
	for _, r := range line {
		switch r {
		case ' ':
			x += spaceW
		case '\t':
			x = (float32(int(x/tabPx)) + 1) * tabPx
		default:
			return x
		}
	}
	return x
}

// paintCodeEditor 绘制代码编辑器（KCodeEditor）。
func (a *App) paintCodeEditor(n *Node, r geom.Rect) {
	e := n.Code
	if e == nil || len(e.Lines) == 0 {
		return
	}
	th := &e.Theme
	lh := e.lineH()
	if e.FontSize <= 0 {
		e.FontSize = CodeDefaultFontSize
	}
	if e.FontFamily == "" {
		e.FontFamily = codeFontFamily()
	}
	f := a.fonts.getFamily(e.FontSize, 400, e.FontFamily)
	if f == nil {
		return
	}

	// ---- 度量 ----
	e.viewW, e.viewH = r.W, r.H
	digits := len(strconv.Itoa(len(e.Lines)))
	e.gutterW = 0
	if e.ShowGutter {
		e.gutterW = f.Measure(strings.Repeat("0", digits)) + codeGutterPad*2
	}
	minimapW := e.minimapW()
	const scrollW = float32(8)
	e.textW = r.W - e.gutterW - minimapW - scrollW
	if e.textW < 40 {
		e.textW = 40
	}
	textX := r.X + e.gutterW
	textY := r.Y + codeTopPad
	e.clampScroll()
	firstRow, lastRow := e.rowRange(r.H)
	total := e.sumRows()
	if lastRow >= total {
		lastRow = total - 1
	}
	focused := IsFocused(e.Key)
	sel, hasSel := e.Selection()

	// ---- 背景 ----
	a.r.Rect(gpu.Shape{Rect: r, Fill: th.Bg})
	if e.ShowGutter {
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X, Y: r.Y, W: e.gutterW, H: r.H}, Fill: th.GutterBg})
	}

	// ---- 当前行高亮（查找面板打开时不画：焦点在面板，编辑器不应看起来也在编辑）----
	if e.ShowCurrentLine && !e.findOpen {
		row := e.rowOfPos(e.Pos)
		y := textY + float32(row)*lh - e.ScrollY
		if y+lh > r.Y && y < r.Bottom() {
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: textX, Y: y, W: e.textW, H: lh}, Fill: th.CurrentLine})
		}
	}

	// ---- 文本区（裁剪，水平滚动时不会画到小地图上）----
	textRect := geom.Rect{X: textX, Y: r.Y, W: e.textW, H: r.H}
	a.r.PushClip(textRect)

	type caretDraw struct {
		x, y float32
		main bool
	}
	carets := make([]caretDraw, 0, len(e.cursors)+1)
	// 额外光标的选区也要画
	selRanges := make([]CodeRange, 0, len(e.cursors))
	for _, c := range e.cursors {
		sp, ep := c.Pos, c.Anchor
		if ep.Before(sp) {
			sp, ep = ep, sp
		}
		selRanges = append(selRanges, CodeRange{Start: sp, End: ep})
	}

	state := uint8(0)
	lastLine := -1
	var spans []codeSpan
	diags := e.allDiagnostics()
	showFind := e.findOpen && e.findText != ""
	needle := ""
	if showFind {
		needle = e.findText
		if !e.findCase {
			needle = strings.ToLower(needle)
		}
	}

	for row := firstRow; row <= lastRow; row++ {
		line, seg := e.lineOfRow(row)
		if line < 0 || line >= len(e.Lines) {
			continue
		}
		y := textY + float32(row)*lh - e.ScrollY
		from, to := e.segRange(line, seg)
		baseX := textX - e.ScrollX - e.xOfCol(line, from)

		if line != lastLine {
			if lastLine >= 0 && line != lastLine+1 {
				state = 0 // 折叠 / 跳行后跨行状态未知，按行首状态重算
			}
			info, next := e.cache.get(e, line, state)
			state = next
			spans = e.mergeSemanticSpans(line, info.spans)
			lastLine = line
		}

		// 选区底色（主选区 + 多光标选区）
		drawSel := func(sr CodeRange, paint gpu.Paint) {
			if line < sr.Start.Line || line > sr.End.Line {
				return
			}
			c0, c1 := 0, runeLen(e.line(line))
			if line == sr.Start.Line {
				c0 = sr.Start.Col
			}
			if line == sr.End.Line {
				c1 = sr.End.Col
			}
			c0 = codeClamp(c0, from, to)
			c1 = codeClamp(c1, from, to)
			if c1 <= c0 {
				return
			}
			x0 := baseX + e.xOfCol(line, c0)
			w := e.xOfCol(line, c1) - e.xOfCol(line, c0)
			if line < sr.End.Line && seg == len(e.segStarts(line))-1 {
				w += 6 // 选中换行时行尾延伸
			}
			if w > 0 {
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x0, Y: y, W: w, H: lh}, Fill: paint})
			}
		}
		for _, sr := range selRanges {
			drawSel(sr, C("rgba(88,166,255,0.18)"))
		}
		if hasSel {
			drawSel(sel, th.Selection)
		}

		// 缩进参考线：只画在缩进区域内（Tab 对齐到 tab 档位），不会穿进代码正文
		if e.ShowIndentGuide && seg == 0 {
			tabPx := float32(e.tabSize()) * e.measure(" ")
			if indentPx := e.indentWidthPx(e.line(line), tabPx); indentPx > tabPx {
				for gx := tabPx; gx <= indentPx-tabPx*0.5; gx += tabPx {
					x := textX - e.ScrollX + gx
					if x < textX || x > textX+e.textW {
						continue
					}
					a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x, Y: y + 2, W: 1, H: lh - 4}, Fill: th.IndentGuide})
				}
			}
		}

		// 语法 / 语义高亮文本（按段切片，gap 用默认文本色补齐）
		pen := from
		drawRuns := func(c0, c1 int, kind codeTokKind) {
			if c1 <= c0 {
				return
			}
			x := baseX + e.xOfCol(line, c0)
			runes := []rune(e.line(line))
			txt := e.expandTabs(string(runes[c0:c1]))
			a.r.TextRun(gpu.TextRun{X: x, Y: y, Text: txt, Font: f, Color: codePaintFor(*th, kind)})
		}
		for _, sp := range spans {
			s0 := codeClamp(sp.Pos, from, to)
			s1 := codeClamp(sp.End, from, to)
			if s1 <= s0 {
				continue
			}
			if s0 > pen {
				drawRuns(pen, s0, codePlain)
			}
			drawRuns(s0, s1, sp.Kind)
			pen = s1
		}
		if pen < to {
			drawRuns(pen, to, codePlain)
		}

		// 诊断下划线
		for _, d := range diags {
			if d.Line != line {
				continue
			}
			c0 := codeClamp(d.Col, from, to)
			c1 := to
			if d.EndLine == d.Line && d.EndCol > d.Col {
				c1 = codeClamp(d.EndCol, from, to)
			}
			w := e.xOfCol(line, c1) - e.xOfCol(line, c0)
			if w < 6 {
				w = 6
			}
			col := th.Invalid
			if d.Severity == 2 {
				col = th.Attribute
			}
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: baseX + e.xOfCol(line, c0), Y: y + lh - 3, W: w, H: 2}, Fill: col})
		}

		// 查找高亮（当前段的命中）
		if showFind {
			hay := e.line(line)
			low := hay
			if !e.findCase {
				low = strings.ToLower(hay)
			}
			for off := 0; off < len(low); {
				idx := strings.Index(low[off:], needle)
				if idx < 0 {
					break
				}
				b := off + idx
				col0 := runeLen(hay[:b])
				col1 := col0 + runeLen(e.findText)
				off = b + len(needle)
				if col1 <= from || col0 >= to {
					continue
				}
				c0, c1 := codeClamp(col0, from, to), codeClamp(col1, from, to)
				x := baseX + e.xOfCol(line, c0)
				w := e.xOfCol(line, c1) - e.xOfCol(line, c0)
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x, Y: y, W: w, H: lh}, Fill: C("rgba(255,196,0,0.22)")})
			}
		}

		// 收集本行可见的光标位置（同段才画）
		appendCaret := func(p CodePos, main bool) {
			if p.Line != line {
				return
			}
			segIdx := e.segOfCol(line, p.Col)
			if segIdx != seg {
				return
			}
			carets = append(carets, caretDraw{
				x:    baseX + e.xOfCol(line, p.Col),
				y:    y,
				main: main,
			})
		}
		if focused {
			appendCaret(e.Pos, true)
			for _, c := range e.cursors {
				appendCaret(c.Pos, false)
			}
		}
	}

	// ---- 光标（查找面板打开时焦点在面板，编辑器不画插入符）----
	if focused && !e.findOpen {
		on := e.caretVisible()
		for _, cd := range carets {
			if cd.main && !on {
				continue
			}
			h := lh - 2
			if !cd.main {
				h = lh - 2
			}
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cd.x, Y: cd.y + 1, W: 2, H: h},
				Fill: th.Caret})
			// 额外光标顶部加一个小标记，和主光标区分
			if !cd.main {
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cd.x - 1, Y: cd.y, W: 4, H: 3},
					Fill: C("rgba(255,196,0,0.75)")})
			}
		}
	}

	a.r.PopClip()

	// ---- 行号栏 + 折叠箭头 ----
	if e.ShowGutter {
		gx := r.X + e.gutterW
		for row := firstRow; row <= lastRow; row++ {
			line, seg := e.lineOfRow(row)
			if seg != 0 {
				continue
			}
			y := textY + float32(row)*lh - e.ScrollY
			num := strconv.Itoa(line + 1)
			w := f.Measure(num)
			col := th.GutterText
			if line == e.Pos.Line {
				col = th.GutterActive
			}
			a.r.TextRun(gpu.TextRun{X: gx - codeGutterPad - w, Y: y, Text: num, Font: f, Color: col})
			if fo := e.foldAt(line); fo != nil && fo.End > fo.Start {
				cx := gx - 9
				cy := y + lh/2
				if e.folded[line] {
					// ▶ 折叠状态
					a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx, Y: cy - 4, W: 7, H: 8}, Fill: th.GutterText})
				} else {
					// ▼ 展开状态
					a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx, Y: cy - 4, W: 7, H: 8}, Fill: th.GutterText})
					a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx + 1, Y: cy - 3, W: 5, H: 3}, Fill: th.GutterBg})
				}
			}
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: gx - 1, Y: r.Y, W: 1, H: r.H}, Fill: th.IndentGuide})
	}

	// ---- 折叠提示（折叠行末尾的 ┄ N 行）----
	if e.hasFolds() {
		for row := firstRow; row <= lastRow; row++ {
			line, seg := e.lineOfRow(row)
			if seg != 0 {
				continue
			}
			fo := e.foldAt(line)
			if fo == nil || !e.folded[line] {
				continue
			}
			y := textY + float32(row)*lh - e.ScrollY
			cw := e.xOfCol(line, runeLen(e.line(line)))
			x := textX - e.ScrollX + cw + 10
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x, Y: y + lh/2, W: 14, H: 1}, Fill: th.IndentGuide})
			a.r.TextRun(gpu.TextRun{X: x + 18, Y: y + 1, Text: "⋯ " + codeItoa(fo.End-fo.Start) + " 行",
				Font: f, Color: th.Comment})
		}
	}

	// ---- 滚动条（几何与命中同源）----
	g := e.scrollGeom()
	if g.thumbV.H > 0 {
		trackCol := gpu.Paint{A: 0.05}
		if e.dragKind == codeHitScrollV && e.dragActive {
			trackCol = gpu.Paint{A: 0.12}
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + g.trackV.X + 2, Y: r.Y + g.trackV.Y, W: 4, H: g.trackV.H},
			Radius: [4]float32{2, 2, 2, 2}, Fill: trackCol})
		thumb := th.Scrollbar
		if e.dragKind == codeHitScrollV && e.dragActive {
			thumb = C("rgba(120,170,255,0.55)")
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + g.thumbV.X, Y: r.Y + g.thumbV.Y, W: g.thumbV.W, H: g.thumbV.H},
			Radius: [4]float32{4, 4, 4, 4}, Fill: thumb})
		// 诊断标记（错误红点 / 警告黄点），与滚动条同高
		for _, d := range e.allDiagnostics() {
			if d.Severity > 2 {
				continue
			}
			frac := float32(0)
			if m := e.maxScrollY(); m > 0 {
				frac = e.yOfLine(d.Line) / (float32(e.sumRows()) * lh)
			}
			my := r.Y + g.trackV.Y + frac*g.trackV.H
			col := th.Invalid
			if d.Severity == 2 {
				col = th.Attribute
			}
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + g.trackV.X + 1, Y: my, W: 4, H: 3}, Fill: col})
		}
	}
	if g.thumbH.W > 0 {
		thumb := th.Scrollbar
		if e.dragKind == codeHitScrollH && e.dragActive {
			thumb = C("rgba(120,170,255,0.55)")
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + g.trackH.X, Y: r.Y + g.trackH.Y, W: g.trackH.W, H: 4},
			Radius: [4]float32{2, 2, 2, 2}, Fill: gpu.Paint{A: 0.06}})
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + g.thumbH.X, Y: r.Y + g.thumbH.Y, W: g.thumbH.W, H: 6},
			Radius: [4]float32{3, 3, 3, 3}, Fill: thumb})
	}

	// ---- 小地图 ----
	if minimapW > 0 {
		mr := geom.Rect{X: r.Right() - minimapW - scrollW, Y: r.Y, W: minimapW, H: r.H}
		a.r.Rect(gpu.Shape{Rect: mr, Fill: th.MinimapBg})
		e.paintMinimap(a, mr)
	}

	// ---- 浮层 ----
	e.paintOverlays(a, r, f)
}

// allDiagnostics 内置（括号 / 缩进提示）+ LSP 诊断合并。
func (e *CodeEditorState) allDiagnostics() []CodeDiagnostic {
	if len(e.builtinDiags) == 0 && len(e.lspDiags) == 0 {
		return nil
	}
	e.lspMu.Lock()
	lsp := e.lspDiags
	e.lspMu.Unlock()
	if len(lsp) == 0 {
		return e.builtinDiags
	}
	out := make([]CodeDiagnostic, 0, len(lsp)+len(e.builtinDiags))
	out = append(out, e.builtinDiags...)
	out = append(out, lsp...)
	return out
}

// maxLineWidth 最宽可见行的宽度（横向滚动条用；带缓存）。
func (e *CodeEditorState) maxLineWidth(f *text.Font) float32 {
	w := float32(0)
	first, last := e.viewState(e.viewH)
	for i := first; i <= last && i < len(e.Lines); i++ {
		info, ok := e.cache.lines[i]
		if ok && info.text == e.Lines[i] {
			if info.width < 0 {
				info.width = e.measure(e.Lines[i])
			}
			if info.width > w {
				w = info.width
			}
			continue
		}
		lw := e.measure(e.Lines[i])
		if info != nil {
			info.text, info.width = e.Lines[i], lw
		}
		if lw > w {
			w = lw
		}
	}
	return w
}

// paintMinimap 小地图：固定采样（与行数无关），画出当前视口框；按住可拖动。
func (e *CodeEditorState) paintMinimap(a *App, r geom.Rect) {
	rows := int((r.H - 8) / codeMinimapRowH)
	if rows < 4 {
		rows = 4
	}
	total := len(e.Lines)
	step := float32(total) / float32(rows)
	if step < 1 {
		step = 1
	}
	th := e.Theme
	inner := r.W - 8
	for k := 0; k < rows; k++ {
		idx := int(float32(k) * step)
		if idx >= total {
			break
		}
		line := e.Lines[idx]
		if e.isHidden(idx) {
			continue
		}
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		indent := float32(leadingSpaces(line)) * 0.35
		w := float32(runeLen(trim)) * 0.8
		if w > inner-2 {
			w = inner - 2
		}
		if w < 2 {
			w = 2
		}
		x := r.X + 4 + indent
		if x+w > r.Right()-4 {
			w = r.Right() - 4 - x
		}
		if w <= 0 {
			continue
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x, Y: r.Y + 4 + float32(k)*codeMinimapRowH, W: w, H: 2},
			Fill: th.MinimapLine})
	}
	// 视口框
	max := e.maxScrollY()
	if max > 0 {
		docH := float32(e.sumRows()) * e.lineH()
		winH := r.H * (e.viewH / docH)
		if winH < 24 {
			winH = 24
		}
		if winH > r.H {
			winH = r.H
		}
		y := r.Y + (r.H-winH)*(e.ScrollY/max)
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X, Y: y, W: r.W, H: winH}, Fill: th.MinimapView})
	}
}

// paintOverlays 绘制查找替换面板 / 诊断列表 / 补全列表 / 右键菜单 / 悬停提示。
