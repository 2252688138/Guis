// guis — 原生代码编辑器（十二）：浮层绘制（查找替换 / 问题列表 / 补全 / 右键菜单 / 悬停）
//
// 所有浮层坐标都相对编辑器节点左上角，绘制时叠加 r.X / r.Y；
// 几何与命中分区共用 codeeditor_find.go / codeeditor_menu.go 里的布局函数，
// 不会出现「看得见点不中」。
package ui

import (
	"guis/internal/geom"
	"guis/internal/gpu"
	"guis/internal/text"
)

// paintOverlays 绘制全部浮层。
func (e *CodeEditorState) paintOverlays(a *App, r geom.Rect, f *text.Font) {
	th := e.Theme
	smallF := a.fonts.getFamily(12, 400, e.FontFamily)
	monoF := a.fonts.getFamily(12, 400, e.FontFamily)
	panelBg := C("#161b22")
	panelStroke := C("#30363d")
	dim := C("#6e7681")
	accent := C("#388bfd")

	// ---- 查找 / 替换面板 ----
	if e.findOpen {
		e.paintFindPanel(a, r, smallF, monoF, panelBg, panelStroke, dim, accent)
	}

	// ---- 诊断列表面板 ----
	if e.diagOpen {
		e.paintDiagPanel(a, r, smallF, panelBg, panelStroke, dim)
	}

	// ---- 补全列表 ----
	if e.compOpen && len(e.compItems) > 0 {
		w := float32(300)
		if w > r.W-20 {
			w = r.W - 20
		}
		rowH := float32(22)
		vis := len(e.compItems)
		if vis > 8 {
			vis = 8
		}
		x := r.X + e.compX
		if x+w > r.Right()-8 {
			x = r.Right() - w - 8
		}
		if x < r.X+4 {
			x = r.X + 4
		}
		y := r.Y + e.compY
		if y+float32(vis)*rowH+8 > r.Bottom() {
			y = r.Bottom() - float32(vis)*rowH - 10
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x, Y: y, W: w, H: float32(vis)*rowH + 8},
			Radius: [4]float32{8, 8, 8, 8}, Fill: panelBg, Stroke: 1, StrokeColor: panelStroke})
		start := 0
		if e.compSel >= vis {
			start = e.compSel - vis + 1
		}
		for k := 0; k < vis; k++ {
			it := e.compItems[start+k]
			ry := y + 4 + float32(k)*rowH
			if start+k == e.compSel {
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x + 4, Y: ry, W: w - 8, H: rowH},
					Radius: [4]float32{5, 5, 5, 5}, Fill: C("rgba(88,166,255,0.20)")})
			}
			a.r.TextRun(gpu.TextRun{X: x + 12, Y: ry + 4, Text: it.Label, Font: smallF, Color: th.Text})
			if it.Detail != "" {
				dw := smallF.Measure(it.Label)
				a.r.TextRun(gpu.TextRun{X: x + 24 + dw, Y: ry + 4, Text: it.Detail, Font: smallF, Color: th.Comment})
			}
		}
	}

	// ---- 右键菜单 ----
	if e.menuOpen {
		items := e.menuItems()
		places, box := e.menuLayout(r)
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + box.X, Y: r.Y + box.Y, W: box.W, H: box.H},
			Radius: [4]float32{8, 8, 8, 8}, Fill: panelBg, Stroke: 1, StrokeColor: panelStroke,
			ShadowBlur: 24, ShadowOffsetY: 10, ShadowColor: C("rgba(0,0,0,0.55)")})
		for _, p := range places {
			px, py := r.X+p.X, r.Y+p.Y
			if p.Sep {
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: px + 8, Y: py + 3, W: p.W - 16, H: 1}, Fill: panelStroke})
				continue
			}
			if p.Index == e.menuSel {
				a.r.Rect(gpu.Shape{Rect: geom.Rect{X: px + 4, Y: py, W: p.W - 8, H: p.H},
					Radius: [4]float32{5, 5, 5, 5}, Fill: C("rgba(88,166,255,0.20)")})
			}
			if p.Index >= len(items) {
				continue
			}
			it := items[p.Index]
			col := th.Text
			if it.Run == nil {
				col = th.Comment
			}
			if it.Checked {
				a.r.TextRun(gpu.TextRun{X: px + 6, Y: py + 5, Text: "✓", Font: smallF, Color: th.Tag})
			}
			a.r.TextRun(gpu.TextRun{X: px + 18, Y: py + 5, Text: it.Label, Font: smallF, Color: col})
			if it.Hint != "" {
				hw := smallF.Measure(it.Hint)
				a.r.TextRun(gpu.TextRun{X: px + p.W - 12 - hw, Y: py + 5, Text: it.Hint, Font: smallF, Color: th.Comment})
			}
		}
	}

	// ---- 悬停信息（签名 / 标题 / 列表分色，段落间虚线隔开）----
	if e.hoverOpen && len(e.hoverLines) > 0 {
		w := float32(380)
		if w > r.W-20 {
			w = r.W - 20
		}
		h := e.hoverPanelHeight()
		x, y := r.X+e.hoverX, r.Y+e.hoverY
		if x+w > r.Right()-4 {
			x = r.Right() - w - 4
		}
		if y+h > r.Bottom()-4 {
			y = y - h - 14
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x, Y: y, W: w, H: h}, Radius: [4]float32{8, 8, 8, 8},
			Fill: panelBg, Stroke: 1, StrokeColor: panelStroke, ShadowBlur: 18, ShadowOffsetY: 8,
			ShadowColor: C("rgba(0,0,0,0.5)")})
		cy := y + codeHoverPadY
		for i, ln := range e.hoverLines {
			kind := e.hoverKindAt(i)
			if kind == hoverSepKind {
				// 虚线分隔（4px 实 / 3px 空；颜色比面板描边略亮，否则看不清）
				drawHoverDashes(a, x+codeHoverPadX, x+w-codeHoverPadX, cy+codeHoverSepH/2, hoverSepPaint())
				cy += codeHoverSepH
				continue
			}
			if ln != "" {
				if kind == hoverCodeKind {
					if spans := e.hoverCodeSpans(ln); len(spans) > 0 {
						px, prev := x+codeHoverPadX, 0
						for _, sp := range spans {
							if sp.Pos > prev {
								seg := hoverSub(ln, prev, sp.Pos)
								a.r.TextRun(gpu.TextRun{X: px, Y: cy, Text: seg, Font: smallF, Color: th.Text})
								px += smallF.Measure(seg)
							}
							seg := hoverSub(ln, sp.Pos, sp.End)
							if seg != "" {
								a.r.TextRun(gpu.TextRun{X: px, Y: cy, Text: seg, Font: smallF, Color: codePaintFor(th, sp.Kind)})
								px += smallF.Measure(seg)
							}
							prev = sp.End
						}
						if rest := hoverSub(ln, prev, runeLen(ln)); rest != "" {
							a.r.TextRun(gpu.TextRun{X: px, Y: cy, Text: rest, Font: smallF, Color: th.Text})
						}
						cy += codeHoverRowH
						continue
					}
				}
				a.r.TextRun(gpu.TextRun{X: x + codeHoverPadX, Y: cy, Text: ln, Font: smallF, Color: e.hoverLinePaint(kind)})
			}
			cy += codeHoverLineH(kind)
		}
	}

	// ---- 右下角状态摘要（软换行 / 多光标 / 折叠）----
	if e.HasMultiCursor() || e.Wrap || e.hasFolds() {
		txt := e.StatusLine()
		tw := smallF.Measure(txt)
		bx := r.Right() - tw - 24 - e.minimapW()
		if bx < r.X+8 {
			bx = r.X + 8
		}
		a.r.TextRun(gpu.TextRun{X: bx, Y: r.Bottom() - 18, Text: txt, Font: smallF, Color: th.Comment})
	}
}

// paintFindPanel 查找 / 替换面板。
func (e *CodeEditorState) paintFindPanel(a *App, r geom.Rect, smallF, monoF *text.Font,
	bg, stroke, dim, accent gpu.Paint) {
	th := e.Theme
	pr := e.findPanelRect()
	ox, oy := r.X+pr.X, r.Y+pr.Y
	a.r.Rect(gpu.Shape{Rect: geom.Rect{X: ox, Y: oy, W: pr.W, H: pr.H},
		Radius: [4]float32{8, 8, 8, 8}, Fill: bg, Stroke: 1, StrokeColor: stroke,
		ShadowBlur: 20, ShadowOffsetY: 8, ShadowColor: C("rgba(0,0,0,0.45)")})

	// 查找框
	fr := e.findFieldRect(pr, 0)
	active := e.findMode != 3 || e.replaceFocus == 0
	border := stroke
	if active {
		border = accent
	}
	a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + fr.X, Y: r.Y + fr.Y, W: fr.W, H: fr.H},
		Radius: [4]float32{5, 5, 5, 5}, Fill: C("#0d1117"), Stroke: 1, StrokeColor: border})
	shown := e.findText
	placeholder := "查找"
	if e.findMode == 3 {
		placeholder = "查找"
	}
	textCol := th.Text
	shownDraw := shown
	if shownDraw == "" {
		shownDraw, textCol = placeholder, dim
	}
	a.r.TextRun(gpu.TextRun{X: r.X + fr.X + 8, Y: r.Y + fr.Y + 5, Text: shownDraw, Font: monoF, Color: textCol})
	if active && e.findSelAll && shown != "" {
		tw := monoF.Measure(shown)
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + fr.X + 5, Y: r.Y + fr.Y + 4, W: tw + 6, H: fr.H - 6},
			Radius: [4]float32{3, 3, 3, 3}, Fill: C("rgba(56,139,253,0.28)")})
	}
	if active && e.caretVisible() {
		// 插入符按 caret 索引定位（不是永远画在行尾）
		caretX := r.X + fr.X + 8 + monoF.Measure(hoverSub(shown, 0, e.findCaret))
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: caretX, Y: r.Y + fr.Y + 4, W: 1.5, H: fr.H - 6}, Fill: th.Caret})
	}

	// 替换框 + 全部替换按钮
	if e.findMode == 3 {
		rr := e.findFieldRect(pr, 1)
		act := e.replaceFocus == 1
		rb := stroke
		if act {
			rb = accent
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + rr.X, Y: r.Y + rr.Y, W: rr.W, H: rr.H},
			Radius: [4]float32{5, 5, 5, 5}, Fill: C("#0d1117"), Stroke: 1, StrokeColor: rb})
		txt, col := e.replaceText, th.Text
		if txt == "" {
			txt, col = "替换为", dim
		}
		a.r.TextRun(gpu.TextRun{X: r.X + rr.X + 8, Y: r.Y + rr.Y + 5, Text: txt, Font: monoF, Color: col})
		if act && e.findSelAll && e.replaceText != "" {
			tw := monoF.Measure(e.replaceText)
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + rr.X + 5, Y: r.Y + rr.Y + 4, W: tw + 6, H: rr.H - 6},
				Radius: [4]float32{3, 3, 3, 3}, Fill: C("rgba(56,139,253,0.28)")})
		}
		if act && e.caretVisible() {
			caretX := r.X + rr.X + 8 + monoF.Measure(hoverSub(e.replaceText, 0, e.replaceCaret))
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: caretX, Y: r.Y + rr.Y + 4, W: 1.5, H: rr.H - 6}, Fill: th.Caret})
		}
		btn := e.findFieldRect(pr, 6)
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + btn.X, Y: r.Y + btn.Y, W: btn.W, H: btn.H},
			Radius: [4]float32{5, 5, 5, 5}, Fill: C("rgba(56,139,253,0.18)"), Stroke: 1, StrokeColor: C("#1f6feb")})
		a.r.TextRun(gpu.TextRun{X: r.X + btn.X + 12, Y: r.Y + btn.Y + 4, Text: "全部替换", Font: smallF, Color: th.Text})
	}

	// 右侧按钮：Aa（大小写）/ 上一个 / 下一个 / 关闭
	drawBtn := func(which int, label string, on bool) {
		br := e.findFieldRect(pr, which)
		col := th.Comment
		if on {
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X + br.X, Y: r.Y + br.Y, W: br.W, H: br.H},
				Radius: [4]float32{4, 4, 4, 4}, Fill: C("rgba(56,139,253,0.25)")})
			col = th.Text
		}
		w := smallF.Measure(label)
		a.r.TextRun(gpu.TextRun{X: r.X + br.X + (br.W-w)/2, Y: r.Y + br.Y + 3, Text: label, Font: smallF, Color: col})
	}
	drawBtn(2, "Aa", e.findCase)
	drawBtn(3, "↑", false)
	drawBtn(4, "↓", false)
	drawBtn(5, "×", false)

	// 命中数 / 模式提示
	if e.findMode != 1 && e.findText != "" {
		tip := codeItoa(e.MatchCount()) + " 处"
		if e.findMode == 3 {
			tip = "Tab 切换  Ctrl+Enter 全部"
		}
		a.r.TextRun(gpu.TextRun{X: r.X + pr.W - 12 - smallF.Measure(tip), Y: r.Y + pr.H - 13,
			Text: tip, Font: smallF, Color: dim})
	}
}

// paintDiagPanel 问题列表面板。
func (e *CodeEditorState) paintDiagPanel(a *App, r geom.Rect, smallF *text.Font,
	bg, stroke, dim gpu.Paint) {
	th := e.Theme
	ds := e.DiagnosticsList()
	dr := e.diagPanelRect()
	x, y := r.X+dr.X, r.Y+dr.Y
	a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x, Y: y, W: dr.W, H: dr.H},
		Radius: [4]float32{8, 8, 8, 8}, Fill: bg, Stroke: 1, StrokeColor: stroke,
		ShadowBlur: 22, ShadowOffsetY: 10, ShadowColor: C("rgba(0,0,0,0.5)")})
	a.r.TextRun(gpu.TextRun{X: x + 10, Y: y + 5, Text: "问题（" + codeItoa(len(ds)) + "）", Font: smallF, Color: th.Text})
	a.r.TextRun(gpu.TextRun{X: x + dr.W - 64, Y: y + 5, Text: "↑↓  Esc", Font: smallF, Color: dim})
	a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x + 6, Y: y + 22, W: dr.W - 12, H: 1}, Fill: stroke})
	if len(ds) == 0 {
		a.r.TextRun(gpu.TextRun{X: x + 10, Y: y + 30, Text: "没有诊断信息", Font: smallF, Color: dim})
		return
	}
	for i, d := range ds {
		if i >= codeDiagPanelMaxRows {
			break
		}
		ry := y + 24 + float32(i)*codeDiagPanelRowH
		if i == e.diagSel {
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x + 4, Y: ry, W: dr.W - 8, H: codeDiagPanelRowH},
				Radius: [4]float32{4, 4, 4, 4}, Fill: C("rgba(88,166,255,0.18)")})
		}
		col, mark := th.Invalid, "✕"
		if d.Severity == 2 {
			col, mark = th.Attribute, "!"
		} else if d.Severity > 2 {
			col, mark = th.Type, "i"
		}
		a.r.TextRun(gpu.TextRun{X: x + 10, Y: ry + 3, Text: mark, Font: smallF, Color: col})
		msg := d.Message
		if runeLen(msg) > 52 {
			msg = string([]rune(msg)[:52]) + "…"
		}
		a.r.TextRun(gpu.TextRun{X: x + 24, Y: ry + 3, Text: msg, Font: smallF, Color: th.Text})
		pos := codeItoa(d.Line+1) + ":" + codeItoa(d.Col+1)
		a.r.TextRun(gpu.TextRun{X: x + dr.W - 16 - smallF.Measure(pos), Y: ry + 3,
			Text: pos, Font: smallF, Color: dim})
	}
}
