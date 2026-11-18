package ui

import (
	"math"
	"strings"
	"unicode/utf8"
)

// 本文件：编辑体验增强（自动闭合 / 选区包裹 / 空配对退格 / 括号匹配高亮 /
// 粘贴自动缩进 / 字号缩放）。所有能力都受开关控制，关闭后行为与旧版一致。

// ---- 符号工具 ----

// codeCloserOf 左符号对应的右符号（0 表示不是可配对符号）。
func codeCloserOf(r rune) rune {
	switch r {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	case '"':
		return '"'
	case '\'':
		return '\''
	case '`':
		return '`'
	}
	return 0
}

// codeIsPair 判断 (l, r) 是否构成空配对（含同字符引号）。
func codeIsPair(l, r rune) bool {
	c := codeCloserOf(l)
	return c != 0 && c == r
}

// codeIsOpenBracket 是否为需要向后找配对的左括号（不含引号）。
func codeIsOpenBracket(r rune) bool {
	return r == '(' || r == '[' || r == '{'
}

// codeIsCloseBracket 是否为需要向前找配对的右括号（不含引号）。
func codeIsCloseBracket(r rune) bool {
	return r == ')' || r == ']' || r == '}'
}

// codeCounterpart 括号的反向符号（扫描配对时用）。
func codeCounterpart(r rune) rune {
	switch r {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	case ')':
		return '('
	case ']':
		return '['
	case '}':
		return '{'
	}
	return 0
}

// codePosAdvance 从 p 起走完 s 之后的位置（按换行推进行列）。
func codePosAdvance(p CodePos, s string) CodePos {
	for _, ch := range s {
		if ch == '\n' {
			p.Line++
			p.Col = 0
		} else {
			p.Col++
		}
	}
	return p
}

// ---- 自动闭合 ----

// AutoClosePair 输入左符号时自动补右符号并让光标停在中间；返回 true 表示已消费该字符。
// 有选区时转为「包裹选区」（见 Surround）。
func (e *CodeEditorState) AutoClosePair(r rune) bool {
	cl := codeCloserOf(r)
	if cl == 0 {
		return false
	}
	if sel, ok := e.Selection(); ok {
		return e.Surround(sel, r, cl)
	}
	if len(e.cursors) > 0 {
		return false // 多光标不做自动配对：避免额外光标与主光标位置错位
	}
	e.InsertText(string(r) + string(cl))
	if p := e.Pos; p.Col > 0 {
		p.Col--
		e.SetCaret(p, false)
	}
	return true
}

// Surround 用 (open, close) 包裹区间，包裹后仍选中内部文本，便于继续输入替换。
func (e *CodeEditorState) Surround(sel CodeRange, open, close rune) bool {
	if e.ReadOnly {
		return false
	}
	sel.Start, sel.End = e.clampPos(sel.Start), e.clampPos(sel.End)
	if sel.End.Before(sel.Start) {
		sel.Start, sel.End = sel.End, sel.Start
	}
	inner := e.textRange(sel)
	text := string(open) + inner + string(close)
	e.replaceRange(sel.Start, sel.End, text, true)
	innerStart := CodePos{Line: sel.Start.Line, Col: sel.Start.Col + 1}
	innerEnd := codePosAdvance(innerStart, inner)
	e.Anchor = innerStart
	e.Pos = innerEnd
	e.afterCaret()
	return true
}

// SurroundSelection 用 (open, close) 包裹当前选区；无选区返回 false。
func (e *CodeEditorState) SurroundSelection(open, close rune) bool {
	sel, ok := e.Selection()
	if !ok {
		return false
	}
	return e.Surround(sel, open, close)
}


// textRange 取任意区间文本（不受当前选区限制）。
func (e *CodeEditorState) textRange(r CodeRange) string {
	r.Start, r.End = e.clampPos(r.Start), e.clampPos(r.End)
	if r.End.Before(r.Start) {
		r.Start, r.End = r.End, r.Start
	}
	if r.Start.Line == r.End.Line {
		runes := []rune(e.line(r.Start.Line))
		if r.Start.Col < 0 {
			r.Start.Col = 0
		}
		if r.End.Col > len(runes) {
			r.End.Col = len(runes)
		}
		return string(runes[r.Start.Col:r.End.Col])
	}
	var b strings.Builder
	first := []rune(e.line(r.Start.Line))
	if r.Start.Col < len(first) {
		b.WriteString(string(first[r.Start.Col:]))
	}
	b.WriteByte('\n')
	for l := r.Start.Line + 1; l < r.End.Line && l < len(e.Lines); l++ {
		b.WriteString(e.line(l))
		b.WriteByte('\n')
	}
	last := []rune(e.line(r.End.Line))
	end := r.End.Col
	if end > len(last) {
		end = len(last)
	}
	b.WriteString(string(last[:end]))
	return b.String()
}

// deletePairBack 光标位于空配对中间时一次删掉两侧；返回 true 表示已处理。
func (e *CodeEditorState) deletePairBack() bool {
	p := e.Pos
	if p.Col <= 0 {
		return false
	}
	runes := []rune(e.line(p.Line))
	if p.Col >= len(runes) {
		return false
	}
	if !codeIsPair(runes[p.Col-1], runes[p.Col]) {
		return false
	}
	e.replaceRange(CodePos{Line: p.Line, Col: p.Col - 1}, CodePos{Line: p.Line, Col: p.Col + 1}, "", true)
	return true
}

// ---- 超长行保护 ----

// codeLongLineLimit 默认的着色行数上限。
const codeLongLineLimit = 4096

// isLongLine 该行是否超过着色上限（超长行只做纯文本绘制）。
// 单行几万字符时着色要逐字符扫描并产出上万个 span，每帧都会把绘制拖死。
func (e *CodeEditorState) isLongLine(line int) bool {
	limit := e.LongLineLimit
	if limit < 0 {
		return false
	}
	if limit == 0 {
		limit = codeLongLineLimit
	}
	if line < 0 || line >= len(e.Lines) {
		return false
	}
	s := e.Lines[line]
	if len(s) <= limit { // 字节数已小于 rune 上限：必然不是超长行
		return false
	}
	return utf8.RuneCountInString(s) > limit
}

// ---- 粘贴自动缩进 ----

// pasteIndented 让多行粘贴文本跟随当前行缩进（首行贴原位，其余行补前缀）。
func (e *CodeEditorState) pasteIndented(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if !strings.Contains(s, "\n") {
		return s
	}
	// 当前行的缩进（到光标位置为止的空白）；光标停在非空白处时取行首缩进
	line := []rune(e.line(e.Pos.Line))
	upto := e.Pos.Col
	if upto > len(line) {
		upto = len(line)
	}
	prefix := ""
	if head := string(line[:upto]); strings.TrimSpace(head) == "" {
		prefix = head
	} else {
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				prefix += string(ch)
				continue
			}
			break
		}
	}
	if prefix == "" {
		return s
	}
	parts := strings.Split(s, "\n")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue // 空行保持空行，避免行尾多出空白
		}
		parts[i] = prefix + parts[i]
	}
	return strings.Join(parts, "\n")
}

// ---- 字号缩放 ----

// ZoomFont 调整字号（delta 为步进，正数放大）。范围夹在 [MinFontSize, MaxFontSize]，
// 缩放后同步行高、清行缓存并让光标行保持在可视区。
func (e *CodeEditorState) ZoomFont(delta int) bool {
	if delta == 0 {
		return false
	}
	minSize, maxSize := e.MinFontSize, e.MaxFontSize
	if minSize <= 0 {
		minSize = 8
	}
	if maxSize <= 0 {
		maxSize = 40
	}
	if maxSize < minSize {
		minSize, maxSize = maxSize, minSize
	}
	old := e.FontSize
	if old <= 0 {
		old = CodeDefaultFontSize
	}
	nw := old + float32(delta)
	if nw < minSize {
		nw = minSize
	}
	if nw > maxSize {
		nw = maxSize
	}
	if nw == old {
		return false
	}
	lh := e.LineH
	if lh <= 0 {
		lh = CodeDefaultLineHeight
	}
	// 行高按字号比例缩放，保持原始行距观感
	e.FontSize = nw
	e.LineH = float32(math.Round(float64(lh * nw / old)))
	if e.LineH < nw {
		e.LineH = nw + 2
	}
	// 度量缓存整体失效
	if e.cache != nil {
		e.cache.reset()
	}
	e.resetViewState()
	e.ensureCaretVisible()
	e.status("字号：" + codeItoa(int(math.Round(float64(nw)))))
	RequestRedraw()
	return true
}

// matchPairCached 带缓存的括号配对查询（文本 / 光标 / 开关不变时直接复用结果）。
func (e *CodeEditorState) matchPairCached() (CodePos, CodePos, bool) {
	if !e.BracketMatchHL {
		return CodePos{}, CodePos{}, false
	}
	if e.bmReady && e.bmSeq == e.changeSeq && e.bmPos.Equal(e.Pos) && e.bmAnchor.Equal(e.Anchor) {
		return e.bmA, e.bmB, e.bmOK
	}
	a, b, ok := e.BracketPair()
	e.bmSeq, e.bmPos, e.bmAnchor = e.changeSeq, e.Pos, e.Anchor
	e.bmA, e.bmB, e.bmOK, e.bmReady = a, b, ok, true
	return a, b, ok
}

// ---- 括号匹配 ----

const (
	// codeBracketScanLines 括号配对最多向下 / 向上扫描的行数。
	codeBracketScanLines = 2000
	// codeBracketScanBytes 括号配对最多扫描的字节数。
	codeBracketScanBytes = 100 * 1024
)

// BracketPair 返回光标处（优先左侧）括号与它配对括号的位置，用于高亮。
// 找不到配对（或超出扫描上限）时返回 false，调用方静默跳过。
func (e *CodeEditorState) BracketPair() (CodePos, CodePos, bool) {
	if !e.BracketMatchHL || len(e.Lines) == 0 {
		return CodePos{}, CodePos{}, false
	}
	p := e.clampPos(e.Pos)
	runes := []rune(e.line(p.Line))
	var (
		at  CodePos
		ch  rune
		ok  bool
	)
	if p.Col < len(runes) && (codeIsOpenBracket(runes[p.Col]) || codeIsCloseBracket(runes[p.Col])) {
		at, ch, ok = p, runes[p.Col], true
	} else if p.Col > 0 && p.Col-1 < len(runes) &&
		(codeIsOpenBracket(runes[p.Col-1]) || codeIsCloseBracket(runes[p.Col-1])) {
		at, ch, ok = CodePos{Line: p.Line, Col: p.Col - 1}, runes[p.Col-1], true
	}
	if !ok {
		return CodePos{}, CodePos{}, false
	}
	match, found := e.scanBracket(at, ch)
	if !found {
		return CodePos{}, CodePos{}, false
	}
	return at, match, true
}

// scanBracket 从 at 处的括号 ch 出发找配对位置。
func (e *CodeEditorState) scanBracket(at CodePos, ch rune) (CodePos, bool) {
	other := codeCounterpart(ch)
	if other == 0 {
		return CodePos{}, false
	}
	forward := codeIsOpenBracket(ch)
	depth := 0
	budget := codeBracketScanBytes
	line, col := at.Line, at.Col
	if forward {
		col++
	} else {
		col--
	}
	for steps := 0; steps <= codeBracketScanLines; steps++ {
		if line < 0 || line >= len(e.Lines) {
			return CodePos{}, false
		}
		runes := []rune(e.line(line))
		for {
			if forward {
				if col >= len(runes) {
					break
				}
			} else if col < 0 {
				break
			}
			c := runes[col]
			budget -= utf8.RuneLen(c)
			if budget <= 0 {
				return CodePos{}, false
			}
			if c == ch {
				depth++
			} else if c == other {
				if depth == 0 {
					return CodePos{Line: line, Col: col}, true
				}
				depth--
			}
			if forward {
				col++
			} else {
				col--
			}
		}
		if forward {
			line++
			col = 0
		} else {
			line--
			if line >= 0 && line < len(e.Lines) {
				col = len([]rune(e.line(line))) - 1
			}
		}
	}
	return CodePos{}, false
}
