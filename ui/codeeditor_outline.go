package ui

import (
	"sort"
	"strings"

	"guis/internal/geom"
	"guis/internal/gpu"
	"guis/internal/text"
)

// 本文件：代码结构导航 —— 符号大纲面板（Ctrl+Shift+O）、面包屑、书签、折叠状态存取。

const (
	// codeOutlineW 大纲面板宽度。
	codeOutlineW = float32(360)
	// codeOutlineRowH 大纲行高。
	codeOutlineRowH = float32(22)
	// codeOutlineMaxRows 大纲最多显示行数（超出滚动略过）。
	codeOutlineMaxRows = 14
	// codeBreadcrumbH 面包屑条高度。
	codeBreadcrumbH = float32(20)
	// codeSymbolLimit 符号扫描上限。
	codeSymbolLimit = 2000
)

// CodeSymbol 大纲 / 面包屑里的一项符号。
type CodeSymbol struct {
	Name  string // 符号名
	Kind  string // func / method / type / class / var / const
	Line  int    // 0 起行号
	Depth int    // 缩进层级（面包屑父子关系用）
}

// ---- 符号扫描 ----

// ScanSymbols 扫描文档符号（同语言同内容复用缓存）。
func (e *CodeEditorState) ScanSymbols() []CodeSymbol {
	if e.symReady && e.symSeq == e.changeSeq && e.symLang == e.Language {
		return e.symbols
	}
	syms := e.computeSymbols()
	e.symbols, e.symSeq, e.symLang, e.symReady = syms, e.changeSeq, e.Language, true
	return syms
}

// computeSymbols 按语言规则提取符号（轻量字符串匹配，不依赖 LSP）。
func (e *CodeEditorState) computeSymbols() []CodeSymbol {
	lang := strings.ToLower(e.Language)
	var out []CodeSymbol
	tab := e.tabSize()
	if tab <= 0 {
		tab = 4
	}
	for i, raw := range e.Lines {
		trim := strings.TrimLeft(raw, " \t")
		if trim == "" || len(out) >= codeSymbolLimit {
			continue
		}
		name, kind := codeSymbolOf(lang, trim)
		if name == "" {
			continue
		}
		indent := 0
		for _, ch := range raw[:len(raw)-len(trim)] {
			if ch == '\t' {
				indent += tab
			} else {
				indent++
			}
		}
		out = append(out, CodeSymbol{Name: name, Kind: kind, Line: i, Depth: indent / tab})
	}
	return out
}

// codeSymbolOf 从一行（已去左侧空白）里识别符号名与类型。
func codeSymbolOf(lang, s string) (string, string) {
	switch {
	case strings.HasPrefix(s, "func "):
		body := s[5:]
		if strings.HasPrefix(body, "(") { // 方法：跳过接收者
			if k := strings.Index(body, ")"); k >= 0 {
				if n := codeLeadingIdent(body[k+1:]); n != "" {
					return n, "method"
				}
			}
			return "", ""
		}
		return codeLeadingIdent(body), "func"
	case strings.HasPrefix(s, "type "):
		return codeLeadingIdent(s[5:]), "type"
	case strings.HasPrefix(s, "def "):
		return codeLeadingIdent(s[4:]), "func"
	case strings.HasPrefix(s, "class "):
		return codeLeadingIdent(s[6:]), "class"
	case strings.HasPrefix(s, "function "):
		return codeLeadingIdent(s[9:]), "func"
	case strings.HasPrefix(s, "export "):
		return codeSymbolOf(lang, strings.TrimSpace(s[7:]))
	case strings.HasPrefix(s, "const "):
		return codeLeadingIdent(s[6:]), "const"
	case strings.HasPrefix(s, "var "):
		return codeLeadingIdent(s[4:]), "var"
	case strings.HasPrefix(s, "let ") && lang != "go":
		return codeLeadingIdent(s[4:]), "var"
	}
	return "", ""
}

// codeLeadingIdent 取开头的标识符（跳过空白与 * & 前缀）。
func codeLeadingIdent(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '*' || s[i] == '&') {
		i++
	}
	j := i
	for j < len(s) {
		c := s[j]
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			j++
			continue
		}
		break
	}
	if j == i {
		return ""
	}
	return s[i:j]
}

// ---- 大纲面板 ----

// ToggleOutline 展开 / 收起符号大纲（Ctrl+Shift+O）。
func (e *CodeEditorState) ToggleOutline() {
	e.outlineOpen = !e.outlineOpen
	if e.outlineOpen {
		e.findOpen = false
		e.menuOpen = false
		e.compOpen = false
		e.diagOpen = false
		e.outlineFilter = ""
		e.outlineSel = e.outlineIndexOfCaret()
	}
	RequestRedraw()
}

// OutlineOpen 大纲面板是否展开。
func (e *CodeEditorState) OutlineOpen() bool { return e.outlineOpen }

// CloseOutline 收起大纲面板。
func (e *CodeEditorState) CloseOutline() {
	e.outlineOpen = false
	e.outlineFilter = ""
	RequestRedraw()
}

// OutlineItems 过滤后的符号列表。
func (e *CodeEditorState) OutlineItems() []CodeSymbol {
	syms := e.ScanSymbols()
	if e.outlineFilter == "" {
		return syms
	}
	f := strings.ToLower(e.outlineFilter)
	out := make([]CodeSymbol, 0, len(syms))
	for _, s := range syms {
		if strings.Contains(strings.ToLower(s.Name), f) {
			out = append(out, s)
		}
	}
	return out
}

// outlineIndexOfCaret 光标所在（或之前最近的）符号序号。
func (e *CodeEditorState) outlineIndexOfCaret() int {
	items := e.OutlineItems()
	idx := 0
	for i, s := range items {
		if s.Line <= e.Pos.Line {
			idx = i
		}
	}
	return idx
}

// outlineMove 大纲选择上下移动。
func (e *CodeEditorState) outlineMove(delta int) {
	items := e.OutlineItems()
	if len(items) == 0 {
		e.outlineSel = 0
		return
	}
	idx := e.outlineSel + delta
	if idx < 0 {
		idx = 0
	}
	if idx > len(items)-1 {
		idx = len(items) - 1
	}
	e.outlineSel = idx
	RequestRedraw()
}

// AcceptOutline 跳到当前选中的符号。
func (e *CodeEditorState) AcceptOutline() bool {
	items := e.OutlineItems()
	if len(items) == 0 {
		return false
	}
	idx := e.outlineSel
	if idx < 0 {
		idx = 0
	}
	if idx > len(items)-1 {
		idx = len(items) - 1
	}
	s := items[idx]
	e.CloseOutline()
	e.SetCaret(CodePos{Line: s.Line, Col: 0}, false)
	e.ScrollToLine(s.Line)
	return true
}

// outlineRect 大纲面板相对编辑器的矩形（左上角，避开面包屑）。
func (e *CodeEditorState) outlineRect() geom.Rect {
	h := codeOutlineRowH*float32(minOf(len(e.OutlineItems()), codeOutlineMaxRows)+1) + codeOutlineRowH
	w := codeOutlineW
	if w > e.viewW-24 {
		w = e.viewW - 24
	}
	if w < 180 {
		w = 180
	}
	return geom.Rect{X: 16, Y: e.topPad() + 6, W: w, H: h}
}

// OutlinePanelClick 大纲面板点击；返回 true 表示已消费。
func (e *CodeEditorState) OutlinePanelClick(x, y float32) bool {
	if !e.outlineOpen {
		return false
	}
	r := e.outlineRect()
	if x < r.X || x > r.X+r.W || y < r.Y || y > r.Y+r.H {
		e.CloseOutline()
		return false
	}
	items := e.OutlineItems()
	row := int((y - r.Y - codeOutlineRowH) / codeOutlineRowH)
	if row >= 0 && row < len(items) {
		e.outlineSel = row
		e.AcceptOutline()
	}
	RequestRedraw()
	return true
}

// paintOutlinePanel 绘制符号大纲。
func (e *CodeEditorState) paintOutlinePanel(a *App, r geom.Rect, smallF *text.Font,
	bg, stroke, dim, accent gpu.Paint) {
	th := e.Theme
	pr := e.outlineRect()
	ox, oy := r.X+pr.X, r.Y+pr.Y
	a.r.Rect(gpu.Shape{Rect: geom.Rect{X: ox, Y: oy, W: pr.W, H: pr.H},
		Radius: [4]float32{8, 8, 8, 8}, Fill: bg, Stroke: 1, StrokeColor: stroke,
		ShadowBlur: 20, ShadowOffsetY: 8, ShadowColor: C("rgba(0,0,0,0.45)")})

	// 首行：标题 + 过滤词
	title := "符号"
	if e.outlineFilter != "" {
		title = "过滤：" + e.outlineFilter
	} else {
		title = "符号（↑↓ 选择，Enter 跳转，Esc 关闭）"
	}
	a.r.TextRun(gpu.TextRun{X: ox + 10, Y: oy + 5, Text: title, Font: smallF, Color: dim})
	a.r.Rect(gpu.Shape{Rect: geom.Rect{X: ox + 8, Y: oy + codeOutlineRowH - 2, W: pr.W - 16, H: 1}, Fill: stroke})

	items := e.OutlineItems()
	if len(items) == 0 {
		a.r.TextRun(gpu.TextRun{X: ox + 10, Y: oy + codeOutlineRowH + 5, Text: "没有符号", Font: smallF, Color: dim})
		return
	}
	// 保证选中项在可视范围内
	top := 0
	if e.outlineSel >= codeOutlineMaxRows {
		top = e.outlineSel - codeOutlineMaxRows + 1
	}
	shown := 0
	for i := top; i < len(items) && shown < codeOutlineMaxRows; i++ {
		s := items[i]
		ry := oy + codeOutlineRowH + float32(shown)*codeOutlineRowH
		if i == e.outlineSel {
			a.r.Rect(gpu.Shape{Rect: geom.Rect{X: ox + 6, Y: ry, W: pr.W - 12, H: codeOutlineRowH - 2},
				Radius: [4]float32{4, 4, 4, 4}, Fill: C("rgba(56,139,253,0.22)")})
		}
		label := s.Kind + "  " + s.Name
		col := th.Text
		switch s.Kind {
		case "func", "method":
			col = th.Function
		case "type", "class":
			col = th.Keyword
		}
		a.r.TextRun(gpu.TextRun{X: ox + 12 + float32(s.Depth)*8, Y: ry + 3, Text: label, Font: smallF, Color: col})
		num := codeItoa(s.Line + 1)
		a.r.TextRun(gpu.TextRun{X: ox + pr.W - 12 - smallF.Measure(num), Y: ry + 3, Text: num, Font: smallF, Color: dim})
		shown++
	}
}

// ---- 面包屑 ----

// breadcrumbH 面包屑占用高度（关闭或空文档时为 0）。
func (e *CodeEditorState) breadcrumbH() float32 {
	if !e.ShowBreadcrumb || len(e.Lines) == 0 {
		return 0
	}
	return codeBreadcrumbH
}

// topPad 文本区顶部留白（含面包屑）。
func (e *CodeEditorState) topPad() float32 { return codeTopPad + e.breadcrumbH() }

// BreadcrumbPath 光标处的作用域链（外层在前）。
func (e *CodeEditorState) BreadcrumbPath() []CodeSymbol {
	syms := e.ScanSymbols()
	last := -1
	for i, s := range syms {
		if s.Line <= e.Pos.Line {
			last = i
			continue
		}
		break
	}
	if last < 0 {
		return nil
	}
	out := []CodeSymbol{syms[last]}
	depth := syms[last].Depth
	for i := last - 1; i >= 0 && depth > 0; i-- {
		if syms[i].Depth < depth {
			out = append(out, syms[i])
			depth = syms[i].Depth
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// codeCrumbSpan 面包屑片段（绘制与命中共用同一份布局）。
type codeCrumbSpan struct {
	Rect  geom.Rect
	Line  int
	Label string
}

// breadcrumbSpans 面包屑片段的布局（相对编辑器节点）。
func (e *CodeEditorState) breadcrumbSpans(smallF *text.Font, width float32) []codeCrumbSpan {
	h := e.breadcrumbH()
	if h <= 0 {
		return nil
	}
	title := e.Title
	if title == "" {
		title = e.Path
	}
	if title == "" {
		title = "未命名"
	}
	path := append([]CodeSymbol{{Name: title, Kind: "file", Line: 0}}, e.BreadcrumbPath()...)
	var out []codeCrumbSpan
	x := float32(10)
	for i, s := range path {
		label := s.Name
		if i > 0 {
			label = s.Kind + " " + s.Name
		}
		w := smallF.Measure(label)
		if x+w > width-10 {
			label = "…"
			w = smallF.Measure(label)
		}
		out = append(out, codeCrumbSpan{Rect: geom.Rect{X: x, Y: 2, W: w, H: h - 6}, Line: s.Line, Label: label})
		x += w + 14 // 分隔符占位
		if x > width-10 {
			break
		}
	}
	return out
}

// BreadcrumbClick 面包屑点击：跳到对应片段所在行；返回 true 表示已消费。
func (e *CodeEditorState) BreadcrumbClick(x, y float32) bool {
	if e.breadcrumbH() <= 0 {
		return false
	}
	if y > e.breadcrumbH()+codeTopPad {
		return false
	}
	for _, sp := range e.bcSpans {
		if x >= sp.Rect.X-4 && x <= sp.Rect.X+sp.Rect.W+4 {
			e.SetCaret(CodePos{Line: sp.Line, Col: 0}, false)
			e.ScrollToLine(sp.Line)
			return true
		}
	}
	return false
}

// paintBreadcrumb 绘制顶部面包屑条。
func (e *CodeEditorState) paintBreadcrumb(a *App, r geom.Rect, smallF *text.Font, dim, accent gpu.Paint) {
	h := e.breadcrumbH()
	if h <= 0 {
		return
	}
	a.r.Rect(gpu.Shape{Rect: geom.Rect{X: r.X, Y: r.Y, W: r.W, H: h}, Fill: C("rgba(255,255,255,0.06)")})
	e.bcSpans = e.breadcrumbSpans(smallF, r.W)
	for i, sp := range e.bcSpans {
		col := dim
		if i == len(e.bcSpans)-1 {
			col = accent
		}
		a.r.TextRun(gpu.TextRun{X: r.X + sp.Rect.X, Y: r.Y + 3, Text: sp.Label, Font: smallF, Color: col})
		if i < len(e.bcSpans)-1 {
			// 分隔符›：放在两段之间，不参与点击命中
			a.r.TextRun(gpu.TextRun{X: r.X + sp.Rect.X + sp.Rect.W + 3, Y: r.Y + 3, Text: "›", Font: smallF, Color: dim})
		}
	}
}

// ---- 书签 ----

// ToggleBookmark 切换当前行书签（Ctrl+F2）。
func (e *CodeEditorState) ToggleBookmark() bool {
	if e.bookmarks == nil {
		e.bookmarks = map[int]bool{}
	}
	line := e.Pos.Line
	if e.bookmarks[line] {
		delete(e.bookmarks, line)
		e.status("已删除书签（第 " + codeItoa(line+1) + " 行）")
	} else {
		e.bookmarks[line] = true
		e.status("已添加书签（第 " + codeItoa(line+1) + " 行）")
	}
	RequestRedraw()
	return true
}

// BookmarkLines 全部书签行（升序）。
func (e *CodeEditorState) BookmarkLines() []int {
	out := make([]int, 0, len(e.bookmarks))
	for ln := range e.bookmarks {
		out = append(out, ln)
	}
	sort.Ints(out)
	return out
}

// HasBookmark 该行是否有书签（绘制用）。
func (e *CodeEditorState) HasBookmark(line int) bool { return e.bookmarks[line] }

// NextBookmark 跳到下一个 / 上一个书签（dir=1 下 / -1 上，循环）。
func (e *CodeEditorState) NextBookmark(dir int) bool {
	lines := e.BookmarkLines()
	if len(lines) == 0 {
		return false
	}
	target := -1
	if dir >= 0 {
		for _, ln := range lines {
			if ln > e.Pos.Line {
				target = ln
				break
			}
		}
		if target < 0 {
			target = lines[0]
		}
	} else {
		for i := len(lines) - 1; i >= 0; i-- {
			if lines[i] < e.Pos.Line {
				target = lines[i]
				break
			}
		}
		if target < 0 {
			target = lines[len(lines)-1]
		}
	}
	e.SetCaret(CodePos{Line: target, Col: 0}, false)
	e.ScrollToLine(target)
	return true
}

// ---- 折叠状态存取 ----

// FoldState 折叠块的起始行列表（升序），便于宿主把折叠状态存进会话。
func (e *CodeEditorState) FoldState() []int {
	out := make([]int, 0, len(e.folded))
	for ln := range e.folded {
		out = append(out, ln)
	}
	sort.Ints(out)
	return out
}

// SetFoldState 恢复折叠状态（非法行号静默忽略）。
func (e *CodeEditorState) SetFoldState(lines []int) {
	e.folded = map[int]bool{}
	for _, ln := range lines {
		if ln < 0 || ln >= len(e.Lines) {
			continue
		}
		if fo := e.foldAt(ln); fo == nil || fo.End <= fo.Start {
			continue
		}
		e.folded[ln] = true
	}
	e.foldsReady = false
	e.rebuildHidden()
	RequestRedraw()
}

// minOf 取较小值。
func minOf(a, b int) int {
	if a < b {
		return a
	}
	return b
}
