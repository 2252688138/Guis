package ui

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// 本文件：查找增强 —— 正则查找 / 捕获组替换 / 全选匹配转多光标 /
// 选区内查找 / 搜索历史。纯文本查找仍走原有快速路径，正则模式在需要时才编译。

const (
	// codeFindMaxHits 单次收集的最大命中数（超大文档也不让查找拖死 UI）。
	codeFindMaxHits = 9999
	// codeFindMaxCursors 全选匹配时最多生成的光标数。
	codeFindMaxCursors = 500
	// codeFindHistoryLimit 搜索历史条数。
	codeFindHistoryLimit = 20
)

// codeFindHit 一处命中：区间 + 命中原文（正则捕获组展开要用原文）。
type codeFindHit struct {
	Range CodeRange
	Text  string
}

// FindRegexpCompile 按当前开关与大小写设置编译查找串；非法或未开正则返回 nil。
func (e *CodeEditorState) findRegexp() *regexp.Regexp {
	if !e.FindRegex || e.findText == "" {
		return nil
	}
	expr := e.findText
	if !e.findCase {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil
	}
	return re
}

// FindPatternError 查找串作为正则是否非法（面板提示用；空串 / 未开正则返回 ""）。
func (e *CodeEditorState) FindPatternError() string {
	if !e.FindRegex || e.findText == "" {
		return ""
	}
	expr := e.findText
	if !e.findCase {
		expr = "(?i)" + expr
	}
	if _, err := regexp.Compile(expr); err != nil {
		return "正则有误"
	}
	return ""
}

// findCacheKey 命中缓存的键：查询条件 + 选区内范围。
func (e *CodeEditorState) findCacheKey() string {
	var b strings.Builder
	b.WriteString(e.findText)
	if e.findCase {
		b.WriteString("|C")
	}
	if e.FindRegex {
		b.WriteString("|R")
	}
	if e.FindInSel {
		b.WriteString("|S")
		if r, ok := e.Selection(); ok {
			b.WriteString("|" + codeItoa(r.Start.Line) + ":" + codeItoa(r.Start.Col) +
				"-" + codeItoa(r.End.Line) + ":" + codeItoa(r.End.Col))
		}
	}
	return b.String()
}

// FindHits 当前查询的全部命中（带缓存；文档改动或查询条件变化才重算）。
func (e *CodeEditorState) FindHits() []codeFindHit {
	if e.findText == "" {
		return nil
	}
	if e.FindRegex && e.FindPatternError() != "" {
		return nil
	}
	key := e.findCacheKey()
	if e.fhReady && e.fhSeq == e.changeSeq && e.fhKey == key {
		return e.fhHits
	}
	hits := e.computeHits()
	e.fhSeq, e.fhKey, e.fhHits, e.fhReady = e.changeSeq, key, hits, true
	return hits
}

// computeHits 扫描全文（或选区范围）收集命中。
func (e *CodeEditorState) computeHits() []codeFindHit {
	if e.findText == "" {
		return nil
	}
	re := e.findRegexp()
	if e.FindRegex && re == nil {
		return nil // 正则非法：不匹配任何内容，也不报错
	}
	from, to := 0, len(e.Lines)-1
	var sel CodeRange
	hasSel := false
	if e.FindInSel {
		if r, ok := e.Selection(); ok {
			sel, hasSel = r, true
			from, to = r.Start.Line, r.End.Line
		}
	}
	if from < 0 {
		from = 0
	}
	if to > len(e.Lines)-1 {
		to = len(e.Lines) - 1
	}
	needle := e.findText
	lowerNeedle := needle
	if !e.findCase {
		lowerNeedle = strings.ToLower(needle)
	}
	var hits []codeFindHit
	for i := from; i <= to; i++ {
		line := e.line(i)
		c0, c1 := 0, utf8.RuneCountInString(line)
		if hasSel {
			if i == sel.Start.Line && sel.Start.Col > c0 {
				c0 = sel.Start.Col
			}
			if i == sel.End.Line && sel.End.Col < c1 {
				c1 = sel.End.Col
			}
			if c0 > c1 {
				c0 = c1
			}
		}
		b0, b1 := byteColOf(line, c0), byteColOf(line, c1)
		if b1 > len(line) {
			b1 = len(line)
		}
		if b0 > b1 {
			continue
		}
		seg := line[b0:b1]
		if re != nil {
			for _, m := range re.FindAllStringIndex(seg, -1) {
				if m[0] > m[1] || m[0] < 0 || m[1] > len(seg) {
					continue
				}
				s, en := b0+m[0], b0+m[1]
				if len(hits) > 0 && s == en && codeSamePos(hits[len(hits)-1].Range.End, CodePos{Line: i, Col: runeLen(line[:s])}) {
					continue // 跳过与上一处结尾重叠的空匹配
				}
				hits = append(hits, codeFindHit{
					Range: CodeRange{Start: CodePos{Line: i, Col: runeLen(line[:s])}, End: CodePos{Line: i, Col: runeLen(line[:en])}},
					Text:  line[s:en],
				})
				if len(hits) >= codeFindMaxHits {
					return hits
				}
			}
			continue
		}
		hay := seg
		if !e.findCase {
			hay = strings.ToLower(seg)
		}
		off := 0
		for {
			idx := strings.Index(hay[off:], lowerNeedle)
			if idx < 0 {
				break
			}
			s := b0 + off + idx
			en := s + len(needle)
			hits = append(hits, codeFindHit{
				Range: CodeRange{Start: CodePos{Line: i, Col: runeLen(line[:s])}, End: CodePos{Line: i, Col: runeLen(line[:en])}},
				Text:  line[s:en],
			})
			if len(hits) >= codeFindMaxHits {
				return hits
			}
			off += idx + len(needle)
			if off >= len(hay) {
				break
			}
		}
	}
	return hits
}

// codeSamePos 两点是否相同。
func codeSamePos(a, b CodePos) bool { return a.Line == b.Line && a.Col == b.Col }

// findRegexJump 正则 / 统一命中集下的「下一个 / 上一个」跳转。
func (e *CodeEditorState) findRegexJump(forward bool) bool {
	hits := e.FindHits()
	if len(hits) == 0 {
		return false
	}
	target := -1
	if forward {
		p := e.Pos
		if r, ok := e.Selection(); ok {
			p = r.End
		}
		for i := range hits {
			if !hits[i].Range.Start.Before(p) {
				target = i
				break
			}
		}
		if target < 0 {
			target = 0 // 回绕
		}
	} else {
		p := e.Pos
		if r, ok := e.Selection(); ok {
			p = r.Start
		}
		for i := len(hits) - 1; i >= 0; i-- {
			if hits[i].Range.Start.Before(p) {
				target = i
				break
			}
		}
		if target < 0 {
			target = len(hits) - 1 // 回绕到末尾
		}
	}
	e.selectHit(hits[target])
	return true
}

// selectHit 选中一处命中。
func (e *CodeEditorState) selectHit(h codeFindHit) {
	e.SetCaret(h.Range.Start, false)
	e.Anchor = h.Range.Start
	e.Pos = h.Range.End
	e.findAt = h.Range.Start.Line
	e.ensureCaretVisible()
	RequestRedraw()
}

// MatchLineRanges 某一行内的命中区间（runes 列），供绘制高亮用。
func (e *CodeEditorState) MatchLineRanges(line int) [][2]int {
	hits := e.FindHits()
	if len(hits) == 0 || line < 0 {
		return nil
	}
	var out [][2]int
	for _, h := range hits {
		if h.Range.Start.Line < line {
			continue
		}
		if h.Range.Start.Line > line {
			break
		}
		out = append(out, [2]int{h.Range.Start.Col, h.Range.End.Col})
	}
	return out
}

// SelectAllMatches 把所有命中的位置变成多光标（Ctrl+Shift+L 批量编辑）。
func (e *CodeEditorState) SelectAllMatches() int {
	hits := e.FindHits()
	if len(hits) == 0 || e.ReadOnly {
		return 0
	}
	n := len(hits)
	if n > codeFindMaxCursors {
		hits = hits[:codeFindMaxCursors]
		n = codeFindMaxCursors
	}
	// 主光标取最后一处，其余作为额外光标（均带选区，便于整段替换）
	last := hits[len(hits)-1]
	e.SetCaret(last.Range.Start, false)
	e.Anchor = last.Range.Start
	e.Pos = last.Range.End
	e.cursors = e.cursors[:0]
	for _, h := range hits[:len(hits)-1] {
		e.cursors = append(e.cursors, codeCursor{Pos: h.Range.End, Anchor: h.Range.Start})
	}
	e.ensureCaretVisible()
	RequestRedraw()
	return n
}

// ---- 正则替换 ----

// expandRepl 用当前正则把 replaceText 里的 $1/${name} 展开成实际文本。
func (e *CodeEditorState) expandRepl(re *regexp.Regexp, src string) string {
	if re == nil {
		return e.replaceText
	}
	idx := re.FindStringSubmatchIndex(src)
	if idx == nil {
		return e.replaceText
	}
	return string(re.ExpandString(nil, e.replaceText, src, idx))
}

// replaceRegexAll 正则全文替换，返回替换处数（一条撤销记录）。
func (e *CodeEditorState) replaceRegexAll() int {
	if e.ReadOnly || e.findText == "" {
		return 0
	}
	re := e.findRegexp()
	if re == nil {
		return 0
	}
	from, to := 0, len(e.Lines)-1
	var sel CodeRange
	hasSel := false
	if e.FindInSel {
		if r, ok := e.Selection(); ok {
			sel, hasSel = r, true
			from, to = r.Start.Line, r.End.Line
		}
	}
	if from < 0 {
		from = 0
	}
	if to > len(e.Lines)-1 {
		to = len(e.Lines) - 1
	}
	n := 0
	changed := map[int]string{}
	minLine, maxLine := -1, -1
	for i := from; i <= to; i++ {
		line := e.line(i)
		c0, c1 := 0, utf8.RuneCountInString(line)
		if hasSel {
			if i == sel.Start.Line && sel.Start.Col > c0 {
				c0 = sel.Start.Col
			}
			if i == sel.End.Line && sel.End.Col < c1 {
				c1 = sel.End.Col
			}
			if c0 > c1 {
				c0 = c1
			}
		}
		b0, b1 := byteColOf(line, c0), byteColOf(line, c1)
		if b0 > len(line) {
			b0 = len(line)
		}
		if b1 > len(line) {
			b1 = len(line)
		}
		if b0 > b1 {
			continue
		}
		seg := line[b0:b1]
		cnt := len(re.FindAllStringIndex(seg, -1))
		if cnt == 0 {
			continue
		}
		out := re.ReplaceAllString(seg, e.replaceText)
		if out == seg {
			continue
		}
		if minLine < 0 {
			minLine = i
		}
		maxLine = i
		changed[i] = line[:b0] + out + line[b1:]
		n += cnt
		if n >= 20000 {
			break
		}
	}
	if n == 0 || minLine < 0 {
		return 0
	}
	before := append([]string(nil), e.Lines[minLine:maxLine+1]...)
	for i, v := range changed {
		e.Lines[i] = v
	}
	after := append([]string(nil), e.Lines[minLine:maxLine+1]...)
	d := codeDelta{Start: minLine, Before: before, After: after,
		PosBefore: e.Pos, AnchorBefore: e.Anchor, PosAfter: e.Pos, AnchorAfter: e.Anchor}
	e.undo = append(e.undo, d)
	if len(e.undo) > codeUndoLimit {
		e.undo = e.undo[len(e.undo)-codeUndoLimit:]
	}
	e.redo = nil
	e.Dirty = true
	e.changeSeq++
	e.invalidate(minLine)
	e.compOpen = false
	if e.OnChange != nil {
		e.OnChange(e.Text())
	}
	e.lspSync()
	e.afterCaret()
	return n
}

// replaceRegexCurrent 替换当前（或下一处）命中的正则内容。
func (e *CodeEditorState) replaceRegexCurrent() bool {
	hits := e.FindHits()
	if len(hits) == 0 {
		return false
	}
	idx := -1
	if r, ok := e.Selection(); ok {
		for i, h := range hits {
			if h.Range.Start.Equal(r.Start) && h.Range.End.Equal(r.End) {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		if !e.findRegexJump(true) {
			return false
		}
		hits = e.FindHits()
		if r, ok := e.Selection(); ok {
			for i, h := range hits {
				if h.Range.Start.Equal(r.Start) && h.Range.End.Equal(r.End) {
					idx = i
					break
				}
			}
		}
		if idx < 0 {
			return false
		}
	}
	h := hits[idx]
	re := e.findRegexp()
	if re == nil {
		return false
	}
	e.replaceRange(h.Range.Start, h.Range.End, e.expandRepl(re, h.Text), true)
	e.findRegexJump(true)
	return true
}

// MatchIndex 当前选区对应的命中序号（1 起；不在命中上返回 0）。
func (e *CodeEditorState) MatchIndex() int {
	r, ok := e.Selection()
	if !ok {
		return 0
	}
	for i, h := range e.FindHits() {
		if h.Range.Start.Equal(r.Start) && h.Range.End.Equal(r.End) {
			return i + 1
		}
	}
	return 0
}

// statusRegexText 正则开关的提示文案。
func statusRegexText(on bool) string {
	if on {
		return "正则查找：开（替换支持 $1 捕获组）"
	}
	return "正则查找：关"
}

// statusInSelText 选区内查找开关的提示文案。
func statusInSelText(on bool) string {
	if on {
		return "只在选区内查找：开"
	}
	return "只在选区内查找：关"
}

// ---- 搜索历史 ----

// findHistoryPush 记录一次搜索（去重、限长）。
func (e *CodeEditorState) findHistoryPush(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	for i, v := range e.findHistory {
		if v == text {
			e.findHistory = append(e.findHistory[:i], e.findHistory[i+1:]...)
			break
		}
	}
	e.findHistory = append([]string{text}, e.findHistory...)
	if len(e.findHistory) > codeFindHistoryLimit {
		e.findHistory = e.findHistory[:codeFindHistoryLimit]
	}
	e.findHistIdx = -1
}

// FindHistoryMove 输入框内 ↑/↓ 翻历史（dir=-1 更早，+1 更近）。
func (e *CodeEditorState) FindHistoryMove(dir int) bool {
	if len(e.findHistory) == 0 {
		return false
	}
	idx := e.findHistIdx
	if dir < 0 {
		// 首次向上：取最近一条（索引 0），继续向上则更早
		if idx < 0 {
			idx = 0
		} else {
			idx++
		}
	} else {
		idx--
	}
	if idx >= len(e.findHistory) {
		idx = len(e.findHistory) - 1
	}
	if idx < -1 {
		idx = -1
	}
	e.findHistIdx = idx
	if idx < 0 {
		e.findText = ""
	} else {
		e.findText = e.findHistory[idx]
	}
	e.afterFindEdit()
	return true
}
