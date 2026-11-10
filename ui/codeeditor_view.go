// guis — 原生代码编辑器（八）：视觉行映射（软换行 / 代码折叠）与命中分区
//
// 编辑器的所有纵向几何都收敛到「视觉行（visual row）」这一层：
//   - 不换行且没有折叠时，视觉行 == 文档行，映射是 O(1) 的快路径（十万行）；
//   - 开启软换行后，一行被切成若干段，每段占一个视觉行；
//   - 折叠的行不产生视觉行，整段跳过。
//
// 前缀缓存 rowBefore / rowCount 是增量构建的：编辑第 L 行只让 L 之后失效，
// 需要滚动到更远处时按 512 行一块补算；未走到的行按 1 行估算总高度，
// 所以改一个字不会重排十万行，滚动条也不会跳。
package ui

import (
	"sort"
	"strings"
	"unicode/utf8"

	"guis/internal/geom"
)

// codeFold 一个可折叠区间（Start 为起始行，End 为匹配到的结束行，含）。
type codeFold struct{ Start, End int }

// wrapPreciseRuneLimit 超过这个长度的行不做软换行测量（按不换行处理）。
const wrapPreciseRuneLimit = 4096

// ---- 软换行分段 ----

// colXOf 行内累计列宽（rune 边界，带缓存：一行只量一次）。
func (e *CodeEditorState) colXOf(line int) []float32 {
	if line < 0 || line >= len(e.Lines) {
		return nil
	}
	info, ok := e.cache.lines[line]
	if !ok || info.text != e.Lines[line] {
		info = &codeLineInfo{text: e.Lines[line], width: -1, wrapW: -1}
		e.cache.lines[line] = info
	}
	if info.colX == nil {
		info.colX = e.buildColX([]rune(e.Lines[line]))
	}
	return info.colX
}

// wrapSegs 返回一行按当前文本宽度换行后的段起始列（首段 0 不记录）。
// nil 表示该行不需要换行（未开启软换行 / 行足够短）。
func (e *CodeEditorState) wrapSegs(line int) []int32 {
	if !e.Wrap || e.textW <= 0 || line < 0 || line >= len(e.Lines) {
		return nil
	}
	info, ok := e.cache.lines[line]
	if !ok || info.text != e.Lines[line] {
		info = &codeLineInfo{text: e.Lines[line], width: -1, wrapW: -1}
		e.cache.lines[line] = info
	}
	if info.wrapW == e.textW {
		return info.wrap
	}
	runes := []rune(e.Lines[line])
	if len(runes) > wrapPreciseRuneLimit {
		// 超长行（压缩后的 JS / 一行几万字）不做软换行测量：
		// 十万行文档里逐字量这种行会让 Alt+Z 卡住几秒。
		info.wrapW, info.wrap = e.textW, nil
		return nil
	}
	colX := e.colXOf(line)
	limit := e.textW - 2
	if limit < 24 {
		limit = 24
	}
	var segs []int32
	if len(colX) > 0 && colX[len(colX)-1] > limit {
		start := 0
		for start < len(runes) {
			// 二分：当前段能放下的最大列
			lo, hi := start+1, len(runes)
			end := start + 1
			for lo <= hi {
				mid := (lo + hi) / 2
				if colX[mid]-colX[start] <= limit {
					end, lo = mid, mid+1
				} else {
					hi = mid - 1
				}
			}
			if end >= len(runes) {
				break
			}
			if bp := wrapBreakPoint(runes, start, end); bp > start {
				end = bp
			}
			segs = append(segs, int32(end))
			start = end
		}
	}
	info.wrapW, info.wrap = e.textW, segs
	return segs
}

// wrapBreakPoint 在一段的 [start, end) 里找最后一个适合断行的位置（空格 / 标点之后），
// 找不到时返回 0（调用方按硬断行处理）。
func wrapBreakPoint(runes []rune, start, end int) int {
	for i := end; i > start+2; i-- {
		switch runes[i-1] {
		case ' ', '\t', ',', ';', ')', ']', '}', '>', '+', '-', '*', '/', '=':
			return i
		}
	}
	return 0
}

// segStarts 一行内所有段起点（含 0）；未换行时返回 nil。
func (e *CodeEditorState) segStarts(line int) []int32 {
	segs := e.wrapSegs(line)
	if len(segs) == 0 {
		return nil
	}
	out := make([]int32, 0, len(segs)+1)
	out = append(out, 0)
	return append(out, segs...)
}

// segOfCol 列所在段序号（未换行时为 0）。
func (e *CodeEditorState) segOfCol(line, col int) int {
	segs := e.wrapSegs(line)
	if len(segs) == 0 {
		return 0
	}
	idx := 0
	for idx < len(segs) && col >= int(segs[idx]) {
		idx++
	}
	return idx
}

// segRange 该段覆盖的列区间 [from, to)。
func (e *CodeEditorState) segRange(line, seg int) (int, int) {
	starts := e.segStarts(line)
	n := runeLen(e.line(line))
	if len(starts) == 0 {
		return 0, n
	}
	if seg < 0 {
		seg = 0
	}
	if seg >= len(starts) {
		return n, n
	}
	from := int(starts[seg])
	to := n
	if seg+1 < len(starts) {
		to = int(starts[seg+1])
	}
	return from, to
}

// visualRows 一行的视觉行数（被折叠隐藏时返回 0）。
func (e *CodeEditorState) visualRows(line int) int {
	if e.isHidden(line) {
		return 0
	}
	return len(e.wrapSegs(line)) + 1
}

// SetWrap 开 / 关软换行（关闭时清掉换行缓存并恢复水平滚动）。
func (e *CodeEditorState) SetWrap(on bool) {
	if e.Wrap == on {
		return
	}
	e.Wrap = on
	for _, info := range e.cache.lines {
		info.wrapW = -1
		info.wrap = nil
	}
	if on {
		e.ScrollX = 0
	}
	e.invalidateRows(0)
	e.ensureCaretVisible()
	RequestRedraw()
}

// ---- 折叠 ----

// hasFolds 是否处于折叠状态（决定能否走 O(1) 快路径）。
func (e *CodeEditorState) hasFolds() bool { return len(e.folded) > 0 }

// simpleRows 行号与视觉行一一对应（不换行 + 无折叠）。
func (e *CodeEditorState) simpleRows() bool { return !e.Wrap && !e.hasFolds() }

// isHidden 该行是否被某个折叠区间藏起来（折叠起始行本身仍然可见）。
func (e *CodeEditorState) isHidden(line int) bool {
	return e.hidden != nil && line >= 0 && line < len(e.hidden) && e.hidden[line] > 0
}

// foldAt 以 line 为起始行的折叠区间（需要先 computeFolds）。
func (e *CodeEditorState) foldAt(line int) *codeFold {
	folds := e.computeFolds()
	i := sort.Search(len(folds), func(i int) bool { return folds[i].Start >= line })
	if i < len(folds) && folds[i].Start == line {
		return &folds[i]
	}
	return nil
}

// foldsIn 与 [from,to] 相交的折叠区间（绘制折叠图标用）。
func (e *CodeEditorState) foldsIn(from, to int) []codeFold {
	if from > to {
		return nil
	}
	folds := e.computeFolds()
	i := sort.Search(len(folds), func(i int) bool { return folds[i].End >= from })
	var out []codeFold
	for ; i < len(folds) && folds[i].Start <= to; i++ {
		out = append(out, folds[i])
	}
	return out
}

// computeFolds 扫描文档生成可折叠区间（按 { } 配对，支持嵌套）。
// 结果按起始行升序；折叠状态变化后同步重建 hidden 前缀表。
func (e *CodeEditorState) computeFolds() []codeFold {
	if e.foldsReady {
		return e.folds
	}
	e.foldsReady = true
	folds := make([]codeFold, 0, 64)
	// 单趟扫描：跳过字符串 / 行注释，不分配临时字符串（十万行也不能每次输入都卡）。
	var stack []int
	for i := 0; i < len(e.Lines); i++ {
		line := e.Lines[i]
		var quote rune
		esc := false
		for j := 0; j < len(line); {
			r, size := utf8.DecodeRuneInString(line[j:])
			j += size
			if quote != 0 {
				if esc {
					esc = false
					continue
				}
				if r == '\\' {
					esc = true
					continue
				}
				if r == quote {
					quote = 0
				}
				continue
			}
			switch r {
			case '"', '\'', '`':
				quote = r
			case '/':
				if j < len(line) && line[j] == '/' {
					j = len(line)
				}
			case '{':
				stack = append(stack, i)
			case '}':
				if len(stack) == 0 {
					continue
				}
				start := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if i > start {
					folds = append(folds, codeFold{Start: start, End: i})
				}
			}
		}
	}
	last := len(e.Lines) - 1
	for _, start := range stack {
		if last > start {
			folds = append(folds, codeFold{Start: start, End: last})
		}
	}
	sort.Slice(folds, func(i, j int) bool { return folds[i].Start < folds[j].Start })
	e.folds = folds
	// 清理已失效的折叠状态
	for start := range e.folded {
		idx := sort.Search(len(folds), func(i int) bool { return folds[i].Start >= start })
		if idx >= len(folds) || folds[idx].Start != start {
			delete(e.folded, start)
		}
	}
	e.rebuildHidden()
	return folds
}

// rebuildHidden 由折叠状态生成「行 → 是否隐藏」前缀表（diff 数组一次前缀和）。
func (e *CodeEditorState) rebuildHidden() {
	n := len(e.Lines)
	if len(e.folded) == 0 {
		e.hidden = nil
		return
	}
	diff := make([]int32, n+2)
	any := false
	for start, on := range e.folded {
		if !on {
			continue
		}
		idx := sort.Search(len(e.folds), func(i int) bool { return e.folds[i].Start >= start })
		if idx >= len(e.folds) || e.folds[idx].Start != start {
			continue
		}
		f := e.folds[idx]
		if f.End <= f.Start {
			continue
		}
		diff[f.Start+1]++
		diff[f.End+1]--
		any = true
	}
	if !any {
		e.hidden = nil
		return
	}
	mark := make([]int32, n+1)
	cur := int32(0)
	for i := 0; i <= n; i++ {
		if i < len(diff) {
			cur += diff[i]
		}
		mark[i] = cur
	}
	e.hidden = mark
}

// ToggleFold 折叠 / 展开以 line 起始的块。
func (e *CodeEditorState) ToggleFold(line int) bool {
	fo := e.foldAt(line)
	if fo == nil || fo.End <= fo.Start {
		return false
	}
	if e.folded == nil {
		e.folded = map[int]bool{}
	}
	if e.folded[line] {
		delete(e.folded, line)
	} else {
		e.folded[line] = true
		if e.Pos.Line > fo.Start && e.Pos.Line <= fo.End {
			e.SetCaret(CodePos{Line: fo.Start, Col: runeLen(e.line(fo.Start))}, false)
		}
	}
	e.rebuildHidden()
	e.invalidateRows(fo.Start)
	e.ensureCaretVisible()
	RequestRedraw()
	return true
}

// FoldAll 全部折叠 / 展开。
func (e *CodeEditorState) FoldAll(collapse bool) {
	folds := e.computeFolds()
	if e.folded == nil {
		e.folded = map[int]bool{}
	}
	for _, f := range folds {
		if collapse {
			e.folded[f.Start] = true
		} else {
			delete(e.folded, f.Start)
		}
	}
	e.rebuildHidden()
	e.invalidateRows(0)
	if collapse && e.isHidden(e.Pos.Line) {
		e.SetCaret(CodePos{Line: 0}, false)
	}
	e.ensureCaretVisible()
	RequestRedraw()
}

// ToggleFoldAtCaret 折叠光标所在的块（块内时折叠该块，块首时折叠自身）。
func (e *CodeEditorState) ToggleFoldAtCaret() bool {
	folds := e.computeFolds()
	if e.folded[e.Pos.Line] {
		return e.ToggleFold(e.Pos.Line)
	}
	for i := len(folds) - 1; i >= 0; i-- {
		if folds[i].Start <= e.Pos.Line && e.Pos.Line <= folds[i].End {
			return e.ToggleFold(folds[i].Start)
		}
	}
	e.status("当前行没有可折叠的块")
	return false
}

// FoldedCount 已折叠块数量。
func (e *CodeEditorState) FoldedCount() int { return len(e.folded) }

// ---- 前缀缓存（视觉行 ↔ 文档行）----

// invalidateRows 让 from 之后的视觉行前缀失效（含折叠重算）。
func (e *CodeEditorState) invalidateRows(from int) {
	if from < 0 {
		from = 0
	}
	if from < e.rowsValid {
		e.rowsValid = from
	}
	if len(e.folds) > 0 || e.foldsReady {
		e.foldsReady = false
		if e.hasFolds() {
			e.computeFolds()
		}
	}
}

// ensureRows 保证 [0, upTo] 的视觉行信息可用（按需增量补算）。
func (e *CodeEditorState) ensureRows(upTo int) {
	n := len(e.Lines)
	if n == 0 {
		return
	}
	if upTo >= n {
		upTo = n - 1
	}
	if upTo < 0 {
		upTo = 0
	}
	if len(e.rowCount) != n {
		e.rowCount = make([]int32, n)
		e.rowBefore = make([]int32, n+1)
		e.rowsValid = 0
	}
	if e.rowsValid > upTo {
		return
	}
	for i := e.rowsValid; i <= upTo && i < n; i++ {
		e.rowCount[i] = int32(e.visualRows(i))
		e.rowBefore[i+1] = e.rowBefore[i] + e.rowCount[i]
	}
	e.rowsValid = upTo + 1
}

// sumRows 视觉行总数；未走到的行按 1 行估算（避免每次输入都全量重排）。
func (e *CodeEditorState) sumRows() int {
	n := len(e.Lines)
	if n == 0 {
		return 0
	}
	if e.simpleRows() {
		return n
	}
	if e.rowsValid >= n {
		return int(e.rowBefore[n])
	}
	if len(e.rowBefore) != n+1 {
		return n
	}
	return int(e.rowBefore[e.rowsValid]) + (n - e.rowsValid)
}

// totalRows 视觉行总数（精确值；需要完整前缀时调用）。
func (e *CodeEditorState) totalRows() int {
	n := len(e.Lines)
	if n == 0 {
		return 0
	}
	if e.simpleRows() {
		return n
	}
	e.ensureRows(n - 1)
	return int(e.rowBefore[n])
}

// rowOfLine line 行首的视觉行号。
func (e *CodeEditorState) rowOfLine(line int) int {
	n := len(e.Lines)
	if n == 0 {
		return 0
	}
	if line < 0 {
		line = 0
	}
	if line >= n {
		line = n - 1
	}
	if e.simpleRows() {
		return line
	}
	e.ensureRows(line)
	return int(e.rowBefore[line])
}

// yOfLine 行首在文档坐标系里的 y（未减 ScrollY）。
func (e *CodeEditorState) yOfLine(line int) float32 {
	return float32(e.rowOfLine(line)) * e.lineH()
}

// yOfPos 位置在文档坐标系里的 y（软换行时按段计算）。
func (e *CodeEditorState) yOfPos(p CodePos) float32 {
	return e.yOfLine(p.Line) + float32(e.segOfCol(p.Line, p.Col))*e.lineH()
}

// lineOfRow 视觉行 → 文档行 + 段序号（越界夹到末行末段）。
func (e *CodeEditorState) lineOfRow(row int) (int, int) {
	n := len(e.Lines)
	if n == 0 {
		return 0, 0
	}
	if row < 0 {
		row = 0
	}
	if e.simpleRows() {
		if row >= n {
			return n - 1, 0
		}
		return row, 0
	}
	if len(e.rowBefore) != n+1 {
		e.ensureRows(0)
	}
	for e.rowsValid < n && int(e.rowBefore[e.rowsValid]) <= row {
		next := e.rowsValid + 512
		if next > n-1 {
			next = n - 1
		}
		e.ensureRows(next)
		if e.rowsValid >= n {
			break
		}
	}
	if e.rowsValid == 0 {
		e.ensureRows(0)
	}
	hi := e.rowsValid
	if hi > n {
		hi = n
	}
	lo, best := 0, 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if mid <= n && int(e.rowBefore[mid]) <= row {
			best, lo = mid, mid+1
		} else {
			hi = mid - 1
		}
	}
	if best > n-1 {
		best = n - 1
	}
	seg := row - int(e.rowBefore[best])
	rows := e.visualRows(best)
	if rows <= 0 {
		rows = 1
	}
	if seg >= rows {
		seg = rows - 1
	}
	if seg < 0 {
		seg = 0
	}
	return best, seg
}

// visibleNext 下一可见行（被折叠的行整段跳过）。
func (e *CodeEditorState) visibleNext(line int) int {
	if !e.hasFolds() {
		return line + 1
	}
	line++
	for line < len(e.Lines) && e.isHidden(line) {
		line++
	}
	return line
}

// ---- 视口 ----

// rowRange 视口覆盖的视觉行区间 [first, last]。
func (e *CodeEditorState) rowRange(h float32) (int, int) {
	lh := e.lineH()
	first := int(e.ScrollY / lh)
	if first < 0 {
		first = 0
	}
	rows := int((h-codeTopPad*2)/lh) + 2
	return first, first + rows
}

// lineRange 视口覆盖的文档行区间（首行、末行、首视觉行）。
func (e *CodeEditorState) lineRange(h float32) (int, int, int) {
	firstRow, lastRow := e.rowRange(h)
	total := e.sumRows()
	if lastRow >= total {
		lastRow = total - 1
	}
	if firstRow > lastRow {
		return 0, -1, firstRow
	}
	first, _ := e.lineOfRow(firstRow)
	last, _ := e.lineOfRow(lastRow)
	return first, last, firstRow
}

// ---- 坐标换算 ----

// posAtXY 节点内坐标 → 文档位置（软换行按段换算列）。
func (e *CodeEditorState) posAtXY(x, y float32) CodePos {
	lh := e.lineH()
	row := int((y - codeTopPad + e.ScrollY) / lh)
	if row < 0 {
		row = 0
	}
	line, seg := e.lineOfRow(row)
	tx := x - e.gutterTotal() + e.ScrollX
	if seg > 0 {
		starts := e.segStarts(line)
		if seg < len(starts) {
			tx = x - e.gutterTotal() + e.xOfCol(line, int(starts[seg]))
		} else {
			tx = -1
		}
	}
	col := e.colAtX(e.line(line), tx)
	return CodePos{Line: line, Col: col}
}

// rowOfPos 位置所在视觉行。
func (e *CodeEditorState) rowOfPos(p CodePos) int {
	return e.rowOfLine(p.Line) + e.segOfCol(p.Line, p.Col)
}

// ---- 命中分区 ----

// codeHitKind 鼠标落点分区。
type codeHitKind int

const (
	codeHitText codeHitKind = iota
	codeHitScrollV
	codeHitScrollH
	codeHitMinimap
	codeHitGutter
	codeHitFoldMark
)

// codeScrollGeom 滚动条 / 小地图 / 行号栏几何（绘制与命中同源）。
type codeScrollGeom struct {
	trackV, thumbV geom.Rect
	trackH, thumbH geom.Rect
	minimap        geom.Rect
	gutter         geom.Rect
	maxY, maxX     float32
}

// scrollGeom 计算本帧滚动条几何（节点内坐标）。
func (e *CodeEditorState) scrollGeom() codeScrollGeom {
	var g codeScrollGeom
	w, h := e.viewW, e.viewH
	if w <= 0 || h <= 0 {
		return g
	}
	const scrollW = 8
	g.maxY = e.maxScrollY()
	g.gutter = geom.Rect{X: 0, Y: 0, W: e.gutterTotal(), H: h}
	trackH := h - 4
	g.trackV = geom.Rect{X: w - scrollW - 1, Y: 2, W: scrollW, H: trackH}
	total := float32(e.sumRows()) * e.lineH()
	if total > h && g.maxY > 0 {
		thumbH := trackH * (h / total)
		if thumbH < 28 {
			thumbH = 28
		}
		if thumbH > trackH {
			thumbH = trackH
		}
		y := g.trackV.Y + (e.ScrollY/g.maxY)*(trackH-thumbH)
		g.thumbV = geom.Rect{X: g.trackV.X, Y: y, W: scrollW, H: thumbH}
	}
	if !e.Wrap {
		maxW := e.maxLineWidth(nil)
		if maxW > e.textW {
			g.maxX = maxW - e.textW + 40
			trackW := e.textW - 8
			thumbW := trackW * (e.textW / maxW)
			if thumbW < 28 {
				thumbW = 28
			}
			if thumbW > trackW {
				thumbW = trackW
			}
			g.trackH = geom.Rect{X: e.gutterTotal(), Y: h - 4, W: trackW, H: 6}
			x := g.trackH.X
			if g.maxX > 0 {
				x += (e.ScrollX / g.maxX) * (trackW - thumbW)
			}
			g.thumbH = geom.Rect{X: x, Y: h - 4, W: thumbW, H: 6}
		}
	}
	if mw := e.minimapW(); mw > 0 {
		g.minimap = geom.Rect{X: w - mw - scrollW, Y: 0, W: mw, H: h}
	}
	return g
}

// minimapW 小地图实际宽度（0 表示不显示）。
func (e *CodeEditorState) minimapW() float32 {
	if !e.ShowMinimap || e.viewW <= 380 {
		return 0
	}
	if e.MinimapW > 0 {
		return e.MinimapW
	}
	return codeMinimapDefaultW
}

// foldMarkRect 该行的折叠箭头区域（行号栏右侧 14px）；无折叠块返回 false。
func (e *CodeEditorState) foldMarkRect(line int, y float32) (geom.Rect, bool) {
	fo := e.foldAt(line)
	if fo == nil || fo.End <= fo.Start {
		return geom.Rect{}, false
	}
	return geom.Rect{X: e.gutterTotal() - 14, Y: y, W: 12, H: e.lineH()}, true
}

// hitZone 落点分区（x,y 为节点内坐标）。
func (e *CodeEditorState) hitZone(x, y float32) codeHitKind {
	g := e.scrollGeom()
	// 顺序很重要：竖向滚动条在小地图右侧，先判滚动条再判小地图
	if x >= g.trackV.X-2 {
		return codeHitScrollV
	}
	if g.minimap.W > 0 && x >= g.minimap.X {
		return codeHitMinimap
	}
	if g.trackH.W > 0 && y >= e.viewH-7 {
		return codeHitScrollH
	}
	if e.ShowGutter && x < g.gutter.W {
		row := int((y - codeTopPad + e.ScrollY) / e.lineH())
		line, _ := e.lineOfRow(row)
		if _, ok := e.foldMarkRect(line, y); ok && x >= g.gutter.W-16 {
			return codeHitFoldMark
		}
		return codeHitGutter
	}
	return codeHitText
}

// ---- 滚动（拖动 / 小地图共用）----

// scrollToFracY 按 0..1 比例定位纵向滚动。
func (e *CodeEditorState) scrollToFracY(frac float32) {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	e.ScrollY = frac * e.maxScrollY()
	e.clampScroll()
	RequestRedraw()
}

// scrollToFracX 按 0..1 比例定位横向滚动。
func (e *CodeEditorState) scrollToFracX(frac float32) {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	e.ScrollX = frac * e.maxScrollX()
	e.clampScroll()
	RequestRedraw()
}

// maxScrollX 横向最大滚动量（软换行时为 0）。
func (e *CodeEditorState) maxScrollX() float32 {
	if e.Wrap {
		return 0
	}
	m := e.maxLineWidth(nil) - e.textW + 40
	if m < 0 {
		return 0
	}
	return m
}

// dragScroll 拖动滚动条 / 小地图（grab 为按下时在滑块内的抓取偏移）。
func (e *CodeEditorState) dragScroll(kind codeHitKind, x, y float32) {
	g := e.scrollGeom()
	switch kind {
	case codeHitScrollV:
		if g.trackV.H <= 0 {
			return
		}
		thumbH := g.thumbV.H
		if thumbH <= 0 {
			thumbH = 28
		}
		span := g.trackV.H - thumbH
		if span <= 0 {
			return
		}
		e.scrollToFracY((y - g.trackV.Y - e.dragGrab) / span)
	case codeHitScrollH:
		if g.trackH.W <= 0 {
			return
		}
		thumbW := g.thumbH.W
		if thumbW <= 0 {
			thumbW = 28
		}
		span := g.trackH.W - thumbW
		if span <= 0 {
			return
		}
		e.scrollToFracX((x - g.trackH.X - e.dragGrab) / span)
	case codeHitMinimap:
		if g.minimap.H <= 0 {
			return
		}
		total := float32(e.sumRows()) * e.lineH()
		frac := (y - g.minimap.Y) / g.minimap.H
		e.ScrollY = frac*total - e.viewH/2
		e.clampScroll()
		RequestRedraw()
	}
}

// startDrag 记录抓取偏移；在轨道（非滑块）上按下时直接跳到落点。
func (e *CodeEditorState) startDrag(kind codeHitKind, x, y float32) {
	e.dragKind = kind
	e.dragGrab = 0
	g := e.scrollGeom()
	switch kind {
	case codeHitScrollV:
		if g.thumbV.H > 0 && y >= g.thumbV.Y && y <= g.thumbV.Y+g.thumbV.H {
			e.dragGrab = y - g.thumbV.Y
			return
		}
		if g.maxY > 0 {
			e.dragGrab = g.thumbV.H / 2
			e.dragScroll(kind, x, y)
		}
	case codeHitScrollH:
		if g.thumbH.W > 0 && x >= g.thumbH.X && x <= g.thumbH.X+g.thumbH.W {
			e.dragGrab = x - g.thumbH.X
			return
		}
		if g.maxX > 0 {
			e.dragGrab = g.thumbH.W / 2
			e.dragScroll(kind, x, y)
		}
	case codeHitMinimap:
		e.dragScroll(kind, x, y)
	}
}

// ---- 状态栏文本（语言 / 诊断等信息已从右键菜单移到状态栏）----

// StatusLine 一行状态摘要：语言 · 软换行 · 折叠 · 多光标 · 诊断数。
func (e *CodeEditorState) StatusLine() string { return e.statusLine() }

func (e *CodeEditorState) statusLine() string {
	s := e.Language
	if e.Wrap {
		s += " · 软换行"
	}
	if n := len(e.folded); n > 0 {
		s += " · 折叠 " + codeItoa(n)
	}
	if n := len(e.cursors); n > 0 {
		s += " · 多光标 " + codeItoa(n+1)
	}
	if len(e.findText) > 0 {
		s += " · 查找 " + e.findText
	}
	s += " · 诊断 " + codeItoa(len(e.allDiagnostics()))
	return s
}

// trimRightRunes 行尾空白前的列号（软换行选区延伸到行尾用）。
func trimRightRunes(s string) int { return runeLen(strings.TrimRight(s, " \t")) }

// codeItoa 小整数转字符串（不依赖 strconv，且与平台专用文件里的 itoa 区分）。
func codeItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
