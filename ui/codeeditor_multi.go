// guis — 原生代码编辑器（九）：多光标
//
// 多光标不做「每个光标一条撤销栈」，而是把一次批量编辑合并成一条 delta：
// 撤销一次就回到批量编辑之前，与 VS Code 的直觉一致。
package ui

import "sort"

// codeCursor 一个额外光标（Pos 为插入点，Anchor 为选区锚点，均按文档位置）。
type codeCursor struct{ Pos, Anchor CodePos }

// codeEdit 一次文本替换（批量编辑用）。
type codeEdit struct {
	Start, End CodePos
	Text       string
}

// Carets 全部光标（主光标在前），按文档顺序升序返回。
func (e *CodeEditorState) Carets() [][2]CodePos {
	out := make([][2]CodePos, 0, len(e.cursors)+1)
	out = append(out, [2]CodePos{e.Pos, e.Anchor})
	for _, c := range e.cursors {
		out = append(out, [2]CodePos{c.Pos, c.Anchor})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i][0].Line != out[j][0].Line {
			return out[i][0].Line < out[j][0].Line
		}
		return out[i][0].Col < out[j][0].Col
	})
	return out
}

// CursorCount 光标数量（含主光标）。
func (e *CodeEditorState) CursorCount() int { return len(e.cursors) + 1 }

// HasMultiCursor 是否有额外光标。
func (e *CodeEditorState) HasMultiCursor() bool { return len(e.cursors) > 0 }

// ClearCursors 收起所有额外光标（Esc / 单击普通位置时调用）。
func (e *CodeEditorState) ClearCursors() {
	if len(e.cursors) > 0 {
		e.cursors = nil
		RequestRedraw()
	}
}

// AddCursor 在相邻行新增光标（Ctrl+Alt+↑ / ↓）。最多 16 个，保持不重复。
func (e *CodeEditorState) AddCursor(dLine int) bool {
	if dLine == 0 || len(e.cursors) >= 15 {
		return false
	}
	base := e.Pos
	if len(e.cursors) > 0 {
		// 以最后一个（最靠边）光标为基准继续扩展，按住可以一路选下去
		base = e.cursors[len(e.cursors)-1].Pos
	}
	target := base.Line + dLine
	if target < 0 || target >= len(e.Lines) {
		return false
	}
	p := e.clampPos(CodePos{Line: target, Col: base.Col})
	for _, c := range e.cursors {
		if c.Pos.Equal(p) {
			return false
		}
	}
	if p.Equal(e.Pos) {
		return false
	}
	e.cursors = append(e.cursors, codeCursor{Pos: p, Anchor: p})
	RequestRedraw()
	return true
}

// AddCursorAt Alt+点击：在指定位置添加光标。
func (e *CodeEditorState) AddCursorAt(p CodePos) {
	p = e.clampPos(p)
	for _, c := range e.cursors {
		if c.Pos.Equal(p) {
			return
		}
	}
	if p.Equal(e.Pos) {
		return
	}
	e.cursors = append(e.cursors, codeCursor{Pos: p, Anchor: p})
	RequestRedraw()
}

// editRanges 把「主光标 + 额外光标」统一成编辑区间（有选区时用选区）。
func (e *CodeEditorState) editRanges() []codeEdit {
	out := make([]codeEdit, 0, len(e.cursors)+1)
	add := func(pos, anchor CodePos) {
		sp, ep := pos, anchor
		if ep.Before(sp) {
			sp, ep = ep, sp
		}
		out = append(out, codeEdit{Start: sp, End: ep})
	}
	add(e.Pos, e.Anchor)
	for _, c := range e.cursors {
		add(c.Pos, c.Anchor)
	}
	return out
}

// multiEdit 批量套用编辑（一次撤销）。edits 的 Start 必须互不重叠。
func (e *CodeEditorState) multiEdit(edits []codeEdit) bool {
	if e.ReadOnly || len(edits) == 0 {
		return false
	}
	// 从后往前套用：前面的编辑不会让后面的坐标失效
	sort.SliceStable(edits, func(i, j int) bool { return edits[j].Start.Before(edits[i].Start) })
	startLine := edits[len(edits)-1].Start.Line
	if startLine < 0 {
		startLine = 0
	}
	if startLine > len(e.Lines) {
		startLine = len(e.Lines)
	}
	// 只记录受影响区间（不拷贝整篇文档：十万行下多光标退格不能跟着拷 3MB）
	maxEnd := startLine
	for _, ed := range edits {
		if ed.End.Line > maxEnd {
			maxEnd = ed.End.Line
		}
	}
	if maxEnd >= len(e.Lines) {
		maxEnd = len(e.Lines) - 1
	}
	before := append([]string(nil), e.Lines[startLine:maxEnd+1]...)
	posBefore, anchorBefore := e.Pos, e.Anchor

	e.quiet = true
	carets := make([]CodePos, 0, len(edits))
	endLine := startLine
	for _, ed := range edits {
		e.replaceRange(ed.Start, ed.End, ed.Text, false)
		if e.Pos.Line > endLine {
			endLine = e.Pos.Line
		}
		carets = append(carets, e.Pos)
	}
	e.quiet = false

	if endLine >= len(e.Lines) {
		endLine = len(e.Lines) - 1
	}
	if endLine < startLine {
		endLine = startLine
	}
	after := append([]string(nil), e.Lines[startLine:endLine+1]...)
	d := codeDelta{Start: startLine, Before: before, After: after,
		PosBefore: posBefore, AnchorBefore: anchorBefore}
	// 主光标取最上面那个编辑的落点，其余保留为额外光标
	e.Pos, e.Anchor = carets[len(carets)-1], carets[len(carets)-1]
	extra := make([]codeCursor, 0, len(carets)-1)
	for i := 0; i < len(carets)-1; i++ {
		extra = append(extra, codeCursor{Pos: carets[i], Anchor: carets[i]})
	}
	e.cursors = extra
	d.PosAfter, d.AnchorAfter = e.Pos, e.Anchor
	e.undo = append(e.undo, d)
	if len(e.undo) > codeUndoLimit {
		e.undo = e.undo[len(e.undo)-codeUndoLimit:]
	}
	e.redo = nil

	e.Dirty = true
	e.changeSeq++
	e.invalidate(startLine)
	e.compOpen = false
	e.hoverOpen = false
	e.ensureCaretVisible()
	if e.OnChange != nil {
		e.OnChange(e.Text())
	}
	e.lspSync()
	e.afterCaret()
	return true
}

// InsertAtCarets 在所有光标处插入文本（有选区先替换选区）。
func (e *CodeEditorState) InsertAtCarets(s string) bool {
	if e.ReadOnly || s == "" {
		return false
	}
	edits := e.editRanges()
	for i := range edits {
		edits[i].Text = s
	}
	return e.multiEdit(edits)
}

// deleteAtCarets 所有光标同时退格 / 删除。
func (e *CodeEditorState) deleteAtCarets(back bool) bool {
	if e.ReadOnly {
		return false
	}
	var edits []codeEdit
	for _, r := range e.editRanges() {
		if !r.Start.Equal(r.End) {
			edits = append(edits, codeEdit{Start: r.Start, End: r.End})
			continue
		}
		p := r.Start
		if back {
			if p.Col > 0 {
				edits = append(edits, codeEdit{Start: CodePos{Line: p.Line, Col: p.Col - 1}, End: p})
			} else if p.Line > 0 {
				edits = append(edits, codeEdit{
					Start: CodePos{Line: p.Line - 1, Col: runeLen(e.line(p.Line - 1))}, End: p})
			}
			continue
		}
		if p.Col < runeLen(e.line(p.Line)) {
			edits = append(edits, codeEdit{Start: p, End: CodePos{Line: p.Line, Col: p.Col + 1}})
		} else if p.Line < len(e.Lines)-1 {
			edits = append(edits, codeEdit{Start: p, End: CodePos{Line: p.Line + 1, Col: 0}})
		}
	}
	if len(edits) == 0 {
		return false
	}
	return e.multiEdit(edits)
}

// duplicateAtCarets 每个光标复制当前行（Ctrl+D 的多光标版本）。
func (e *CodeEditorState) duplicateAtCarets() bool {
	var edits []codeEdit
	for _, r := range e.editRanges() {
		line := r.Start.Line
		if line >= len(e.Lines) {
			continue
		}
		col := runeLen(e.line(line))
		edits = append(edits, codeEdit{
			Start: CodePos{Line: line, Col: col},
			End:   CodePos{Line: line, Col: col},
			Text:  "\n" + e.line(line),
		})
	}
	if len(edits) <= 1 {
		e.DuplicateLine()
		return true
	}
	return e.multiEdit(edits)
}

// SelectLine 选中整行（行号栏点击）。
func (e *CodeEditorState) SelectLine(line int) {
	if line < 0 || line >= len(e.Lines) {
		return
	}
	e.Anchor = CodePos{Line: line, Col: 0}
	e.Pos = CodePos{Line: line, Col: runeLen(e.line(line))}
	e.afterCaret()
}

// MoveCarets 上下移动所有光标（无选区时的 Alt+↑↓ 多光标版本）。
func (e *CodeEditorState) MoveCarets(dLine int) {
	if dLine == 0 {
		return
	}
	move := func(p CodePos) CodePos {
		line := codeClamp(p.Line+dLine, 0, len(e.Lines)-1)
		return e.clampPos(CodePos{Line: line, Col: p.Col})
	}
	e.Pos = move(e.Pos)
	e.Anchor = e.Pos
	for i := range e.cursors {
		p := move(e.cursors[i].Pos)
		e.cursors[i] = codeCursor{Pos: p, Anchor: p}
	}
	e.ensureCaretVisible()
	e.afterCaret()
}
