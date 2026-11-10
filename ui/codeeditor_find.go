// guis — 原生代码编辑器（十）：查找替换面板 + 诊断列表面板
//
// 两个面板都是「编辑器内部浮层」：几何、命中、绘制三者共用同一份布局函数，
// 不会出现「看得见点不中」；坐标一律相对节点左上角，绘制时再叠加 r.X / r.Y。
package ui

import (
	"sort"
	"strings"

	"guis/internal/geom"
)

// 面板度量。
const (
	codeFindPanelW       = float32(348)
	codeFindRowH         = float32(30)
	codeFindPad          = float32(8)
	codeFindBtnW         = float32(24)
	codeDiagPanelRowH    = float32(22)
	codeDiagPanelMaxRows = 9
)

// OpenFind 打开查找栏（mode：0 查找 / 1 转到行 / 2 重命名 / 3 查找替换）。
func (e *CodeEditorState) OpenFind(mode int) {
	e.menuOpen = false
	e.compOpen = false
	e.diagOpen = false
	e.findOpen = true
	e.findMode = mode
	e.replaceFocus = 0
	if mode == 1 {
		e.findText = codeItoa(e.Pos.Line + 1)
	} else if mode == 2 {
		if w := e.wordAt(e.Pos.Line, e.Pos.Col); w != "" {
			e.renameWord, e.findText = w, w
		} else {
			e.status("光标不在符号上")
			e.findOpen = false
			return
		}
	} else if r, ok := e.Selection(); ok && r.Start.Line == r.End.Line {
		if s := e.SelectedText(); s != "" && runeLen(s) <= 120 {
			e.findText = s
		}
	}
	e.findSelAll = false
	e.findCaret = runeLen(e.findText)
	e.replaceCaret = runeLen(e.replaceText)
	RequestRedraw()
}

// dismissFind 点击编辑区域时收起面板（保留已输入的查找/替换内容，下次 Ctrl+F 还在）。
func (e *CodeEditorState) dismissFind() {
	if !e.findOpen {
		return
	}
	e.findOpen = false
	e.findSelAll = false
	e.replaceFocus = 0
	RequestRedraw()
}

// OpenReplace 打开查找替换面板（Ctrl+H）。
func (e *CodeEditorState) OpenReplace() { e.OpenFind(3) }

// CloseFind 关闭查找栏。
func (e *CodeEditorState) CloseFind() {
	e.findOpen = false
	e.replaceFocus = 0
	e.replaceText = ""
	e.findSelAll = false
	e.findCaret, e.replaceCaret = 0, 0
	RequestRedraw()
}

// findInput 当前聚焦的输入缓冲（查找 / 替换）。
func (e *CodeEditorState) findInput() *string {
	if e.findMode == 3 && e.replaceFocus == 1 {
		return &e.replaceText
	}
	return &e.findText
}

// findInputCaret 当前缓冲的插入位置（按 rune 计，0..len）。
func (e *CodeEditorState) findInputCaret() *int {
	if e.findMode == 3 && e.replaceFocus == 1 {
		return &e.replaceCaret
	}
	return &e.findCaret
}

// clampFindCaret 把插入位置夹到 [0, 长度]。
func (e *CodeEditorState) clampFindCaret() {
	cur := e.findInputCaret()
	n := runeLen(*e.findInput())
	if *cur > n {
		*cur = n
	}
	if *cur < 0 {
		*cur = 0
	}
}

// findLiveSearch 输入查找词后是否要实时定位（替换框输入不跳转）。
func (e *CodeEditorState) findLiveSearch() bool {
	return e.findMode == 0 || (e.findMode == 3 && e.replaceFocus == 0)
}

// afterFindEdit 面板输入变更后的统一收尾：实时定位 + 重绘。
func (e *CodeEditorState) afterFindEdit() {
	if e.findLiveSearch() && e.findText != "" {
		e.Find(e.findText, true)
	}
	RequestRedraw()
}

// appendFindRune 面板输入一个字符（插在插入点处）。
func (e *CodeEditorState) appendFindRune(r rune) { e.findInsert(string(r)) }

// findInsert 在插入点插入文本；Ctrl+A 全选状态下先清空。
func (e *CodeEditorState) findInsert(s string) {
	buf, cur := e.findInput(), e.findInputCaret()
	rs := []rune(*buf)
	if e.findSelAll {
		rs, *cur, e.findSelAll = nil, 0, false
	}
	e.clampFindCaret()
	at := *cur
	out := make([]rune, 0, len(rs)+runeLen(s))
	out = append(out, rs[:at]...)
	out = append(out, []rune(s)...)
	out = append(out, rs[at:]...)
	*buf = string(out)
	*cur = at + runeLen(s)
	e.afterFindEdit()
}

// FindBackspace 面板退格（删插入点前一个字符；全选时清空）。
func (e *CodeEditorState) FindBackspace() {
	buf, cur := e.findInput(), e.findInputCaret()
	if e.findSelAll {
		*buf, *cur, e.findSelAll = "", 0, false
		e.afterFindEdit()
		return
	}
	e.clampFindCaret()
	if *cur <= 0 {
		return
	}
	rs := []rune(*buf)
	at := *cur
	*buf = string(append(rs[:at-1], rs[at:]...))
	*cur = at - 1
	e.afterFindEdit()
}

// FindDelete 面板 Delete（删插入点处字符）。
func (e *CodeEditorState) FindDelete() {
	buf, cur := e.findInput(), e.findInputCaret()
	if e.findSelAll {
		*buf, *cur, e.findSelAll = "", 0, false
		e.afterFindEdit()
		return
	}
	e.clampFindCaret()
	rs := []rune(*buf)
	at := *cur
	if at >= len(rs) {
		return
	}
	*buf = string(append(rs[:at], rs[at+1:]...))
	e.afterFindEdit()
}

// FindCaretMove 左右移动插入点。
func (e *CodeEditorState) FindCaretMove(delta int) {
	e.findSelAll = false
	cur := e.findInputCaret()
	*cur += delta
	e.clampFindCaret()
	RequestRedraw()
}

// FindCaretHome 插入点移到行首。
func (e *CodeEditorState) FindCaretHome() {
	e.findSelAll = false
	*e.findInputCaret() = 0
	RequestRedraw()
}

// FindCaretEnd 插入点移到行尾。
func (e *CodeEditorState) FindCaretEnd() {
	e.findSelAll = false
	*e.findInputCaret() = runeLen(*e.findInput())
	RequestRedraw()
}

// FindSelectAll Ctrl+A：全选（下一次输入 / 退格直接替换全部）。
func (e *CodeEditorState) FindSelectAll() {
	if *e.findInput() == "" {
		return
	}
	e.findSelAll = true
	RequestRedraw()
}

// FindPaste 粘贴剪贴板（换行压成空格，面板是单行输入框）。
func (e *CodeEditorState) FindPaste() {
	s := ClipboardText()
	if s == "" {
		return
	}
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", " ")
	e.findInsert(s)
}

// findPanelRect 查找 / 替换面板整体矩形（节点内坐标）。
func (e *CodeEditorState) findPanelRect() geom.Rect {
	h := codeFindRowH
	if e.findMode == 3 {
		h = codeFindRowH * 2
	}
	w := codeFindPanelW
	if w > e.viewW-24 {
		w = e.viewW - 24
	}
	if w < 160 {
		w = 160
	}
	x := e.viewW - w - 16
	if e.minimapW() > 0 {
		x -= e.minimapW()
	}
	if x < 4 {
		x = 4
	}
	return geom.Rect{X: x, Y: 8, W: w, H: h}
}

// findFieldRect 面板内某个输入框 / 按钮的相对矩形（i：0 查找框 1 替换框 2.. 按钮）。
func (e *CodeEditorState) findFieldRect(r geom.Rect, which int) geom.Rect {
	rowW := r.W - codeFindPad*2
	switch which {
	case 0: // 查找输入框
		return geom.Rect{X: r.X + codeFindPad, Y: r.Y + 5, W: rowW - codeFindBtnW*4 - 6, H: codeFindRowH - 8}
	case 1: // 替换输入框
		return geom.Rect{X: r.X + codeFindPad, Y: r.Y + codeFindRowH + 5,
			W: r.W - codeFindPad*2 - 74, H: codeFindRowH - 8}
	case 2: // Aa 区分大小写
		return geom.Rect{X: r.X + r.W - codeFindPad - codeFindBtnW*4 - 6, Y: r.Y + 4, W: codeFindBtnW, H: codeFindRowH - 6}
	case 3: // 上一个
		return geom.Rect{X: r.X + r.W - codeFindPad - codeFindBtnW*3 - 4, Y: r.Y + 4, W: codeFindBtnW, H: codeFindRowH - 6}
	case 4: // 下一个
		return geom.Rect{X: r.X + r.W - codeFindPad - codeFindBtnW*2 - 2, Y: r.Y + 4, W: codeFindBtnW, H: codeFindRowH - 6}
	case 5: // 关闭
		return geom.Rect{X: r.X + r.W - codeFindPad - codeFindBtnW, Y: r.Y + 4, W: codeFindBtnW, H: codeFindRowH - 6}
	case 6: // 全部替换
		return geom.Rect{X: r.X + r.W - codeFindPad - 70, Y: r.Y + codeFindRowH + 5, W: 70, H: codeFindRowH - 8}
	}
	return geom.Rect{}
}

// FindPanelClick 面板点击；返回 true 表示事件已被面板消费。
func (e *CodeEditorState) FindPanelClick(x, y float32) bool {
	if !e.findOpen {
		return false
	}
	r := e.findPanelRect()
	if x < r.X || x > r.X+r.W || y < r.Y || y > r.Y+r.H {
		return false
	}
	in := func(b geom.Rect) bool { return x >= b.X && x <= b.X+b.W && y >= b.Y && y <= b.Y+b.H }
	switch {
	case in(e.findFieldRect(r, 0)):
		e.replaceFocus = 0
		e.findSelAll = false
		e.findCaret = runeLen(e.findText)
	case e.findMode == 3 && in(e.findFieldRect(r, 1)):
		e.replaceFocus = 1
		e.findSelAll = false
		e.replaceCaret = runeLen(e.replaceText)
	case in(e.findFieldRect(r, 2)):
		e.findCase = !e.findCase
		e.status(statusCaseText(e.findCase))
	case in(e.findFieldRect(r, 3)):
		e.Find(e.findText, false)
	case in(e.findFieldRect(r, 4)):
		e.Find(e.findText, true)
	case in(e.findFieldRect(r, 5)):
		e.CloseFind()
	case e.findMode == 3 && in(e.findFieldRect(r, 6)):
		if n := e.ReplaceAll(); n > 0 {
			e.status("已全部替换 " + codeItoa(n) + " 处")
		} else {
			e.status("没有匹配项")
		}
	}
	RequestRedraw()
	return true
}

// statusCaseText 大小写开关的提示文案。
func statusCaseText(on bool) string {
	if on {
		return "区分大小写：开"
	}
	return "区分大小写：关"
}

// MatchCount 查找串的命中总数（最多统计 9999 处，十万行也不卡输入）。
func (e *CodeEditorState) MatchCount() int {
	if e.findText == "" {
		return 0
	}
	needle := e.findText
	hay := needle
	if !e.findCase {
		needle, hay = strings.ToLower(needle), ""
		_ = hay
	}
	n := 0
	for _, line := range e.Lines {
		s := line
		nd := e.findText
		if !e.findCase {
			s = strings.ToLower(s)
			nd = strings.ToLower(nd)
		}
		off := 0
		for {
			idx := strings.Index(s[off:], nd)
			if idx < 0 {
				break
			}
			n++
			off += idx + len(nd)
			if n >= 9999 {
				return n
			}
			if off >= len(s) {
				break
			}
		}
	}
	return n
}

// ReplaceCurrent 替换当前命中（无命中先定位下一个）。
func (e *CodeEditorState) ReplaceCurrent() bool {
	if e.ReadOnly || e.findText == "" {
		return false
	}
	r, ok := e.Selection()
	same := false
	if ok && r.Start.Line == r.End.Line {
		if e.findCase {
			same = e.SelectedText() == e.findText
		} else {
			same = strings.EqualFold(e.SelectedText(), e.findText)
		}
	}
	if !same {
		if !e.Find(e.findText, true) {
			return false
		}
		r, ok = e.Selection()
		if !ok {
			return false
		}
	}
	e.replaceRange(r.Start, r.End, e.replaceText, true)
	e.Find(e.findText, true)
	return true
}

// ReplaceAll 全文替换，返回替换处数（一条撤销记录）。
func (e *CodeEditorState) ReplaceAll() int {
	if e.ReadOnly || e.findText == "" {
		return 0
	}
	type change struct {
		line     int
		old, new string
		count    int
	}
	var chs []change
	n := 0
	for i, line := range e.Lines {
		out, c := replaceInLine(line, e.findText, e.replaceText, e.findCase)
		if c > 0 {
			chs = append(chs, change{line: i, old: line, new: out, count: c})
			n += c
		}
		if n >= 20000 {
			break
		}
	}
	if n == 0 {
		return 0
	}
	start, end := chs[0].line, chs[len(chs)-1].line
	before := append([]string(nil), e.Lines[start:end+1]...)
	for _, c := range chs {
		e.Lines[c.line] = c.new
	}
	after := append([]string(nil), e.Lines[start:end+1]...)
	d := codeDelta{Start: start, Before: before, After: after,
		PosBefore: e.Pos, AnchorBefore: e.Anchor, PosAfter: e.Pos, AnchorAfter: e.Anchor}
	e.undo = append(e.undo, d)
	if len(e.undo) > codeUndoLimit {
		e.undo = e.undo[len(e.undo)-codeUndoLimit:]
	}
	e.redo = nil
	e.Dirty = true
	e.changeSeq++
	e.invalidate(start)
	e.compOpen = false
	if e.OnChange != nil {
		e.OnChange(e.Text())
	}
	e.lspSync()
	e.afterCaret()
	return n
}

// replaceInLine 单行替换，返回新行与替换次数（大小写不敏感时保留原字符大小写之外的内容）。
func replaceInLine(line, needle, repl string, caseSensitive bool) (string, int) {
	if needle == "" {
		return line, 0
	}
	s, nd := line, needle
	if !caseSensitive {
		s, nd = strings.ToLower(line), strings.ToLower(needle)
	}
	var sb strings.Builder
	off, n := 0, 0
	for {
		idx := strings.Index(s[off:], nd)
		if idx < 0 {
			break
		}
		b := off + idx
		sb.WriteString(line[off:b])
		sb.WriteString(repl)
		off = b + len(needle)
		n++
		if off >= len(s) {
			break
		}
	}
	if n == 0 {
		return line, 0
	}
	sb.WriteString(line[off:])
	return sb.String(), n
}

// ---- 诊断列表面板 ----

// ToggleDiagPanel 展开 / 收起诊断列表（F8 / Ctrl+Shift+M）。
func (e *CodeEditorState) ToggleDiagPanel() {
	e.diagOpen = !e.diagOpen
	if e.diagOpen {
		e.findOpen = false
		e.menuOpen = false
		e.diagSel = 0
		e.FocusDiagCaret()
	}
	RequestRedraw()
}

// DiagPanelOpen 问题面板是否展开。
func (e *CodeEditorState) DiagPanelOpen() bool { return e.diagOpen }

// DiagnosticsList 诊断列表（按行、列排序）。
func (e *CodeEditorState) DiagnosticsList() []CodeDiagnostic {
	ds := e.allDiagnostics()
	if len(ds) == 0 {
		return nil
	}
	out := append([]CodeDiagnostic(nil), ds...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Col < out[j].Col
	})
	return out
}

// FocusDiagCaret 选中光标处 / 之后的诊断。
func (e *CodeEditorState) FocusDiagCaret() {
	ds := e.DiagnosticsList()
	if len(ds) == 0 {
		return
	}
	for i, d := range ds {
		if d.Line > e.Pos.Line || (d.Line == e.Pos.Line && d.Col >= e.Pos.Col) {
			e.diagSel = i
			return
		}
	}
	e.diagSel = len(ds) - 1
}

// DiagPanelRect 诊断面板矩形（右下角，最多 9 行）。
func (e *CodeEditorState) diagPanelRect() geom.Rect {
	n := len(e.DiagnosticsList())
	if n > codeDiagPanelMaxRows {
		n = codeDiagPanelMaxRows
	}
	if n == 0 {
		n = 1
	}
	h := float32(n)*codeDiagPanelRowH + 30
	w := float32(420)
	if w > e.viewW-24 {
		w = e.viewW - 24
	}
	if w < 200 {
		w = 200
	}
	return geom.Rect{X: e.viewW - w - 52, Y: e.viewH - h - 14, W: w, H: h}
}

// DiagPanelClick 诊断面板点击；返回 true 表示已消费。
func (e *CodeEditorState) DiagPanelClick(x, y float32) bool {
	if !e.diagOpen {
		return false
	}
	r := e.diagPanelRect()
	if x < r.X || x > r.X+r.W || y < r.Y || y > r.Y+r.H {
		// 点在面板外：收起面板并把事件还给文本区
		e.diagOpen = false
		return false
	}
	if y < r.Y+24 {
		return true // 标题栏
	}
	idx := int((y - r.Y - 24) / codeDiagPanelRowH)
	ds := e.DiagnosticsList()
	if idx >= 0 && idx < len(ds) && idx < codeDiagPanelMaxRows {
		e.diagSel = idx
		e.PickDiagnostic(idx)
	}
	return true
}

// PickDiagnostic 跳到第 idx 条诊断。
func (e *CodeEditorState) PickDiagnostic(idx int) {
	ds := e.DiagnosticsList()
	if idx < 0 || idx >= len(ds) {
		return
	}
	d := ds[idx]
	e.diagSel = idx
	e.SetCaret(CodePos{Line: d.Line, Col: d.Col}, false)
	e.ensureCaretVisible()
	e.status("[" + diagSeverityName(d.Severity) + "] " + d.Message)
}

// DiagStep 在诊断列表里上下移动（面板打开时 ↑↓ / F8 Shift+F8）。
func (e *CodeEditorState) DiagStep(dir int) {
	ds := e.DiagnosticsList()
	if len(ds) == 0 {
		e.status("没有诊断信息")
		return
	}
	e.diagSel += dir
	if e.diagSel < 0 {
		e.diagSel = len(ds) - 1
	}
	if e.diagSel >= len(ds) {
		e.diagSel = 0
	}
	e.PickDiagnostic(e.diagSel)
	RequestRedraw()
}
