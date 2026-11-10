// guis — 原生代码编辑器（四）：右键菜单 / 补全 / 命令 / 快捷键说明
package ui

import (
	"strings"

	"guis/internal/geom"
)

// menuItems 右键菜单项（每帧重建；Run 为 nil 的项按禁用处理）。
func (e *CodeEditorState) menuItems() []CodeMenuItem {
	hasSel := false
	if _, ok := e.Selection(); ok {
		hasSel = true
	}
	clip := ClipboardText() != ""
	// 菜单固定在 20 行以内：语言、诊断条数、光标位置等纯展示信息全部走状态栏
	// （CodeEditorState.StatusLine），菜单只留可执行命令。
	items := []CodeMenuItem{
		{Label: "撤销", Hint: "Ctrl+Z", Run: func() { e.Undo() }},
		{Label: "重做", Hint: "Ctrl+Y", Run: func() { e.Redo() }},
		{Sep: true},
		{Label: "剪切", Hint: "Ctrl+X", Run: func() { e.Cut() }},
		{Label: "复制", Hint: "Ctrl+C", Run: func() { e.Copy() }},
		{Label: "粘贴", Hint: "Ctrl+V", Run: func() { e.Paste() }},
		{Sep: true},
		{Label: "查找", Hint: "Ctrl+F", Run: func() { e.OpenFind(0) }},
		{Label: "替换", Hint: "Ctrl+H", Run: func() { e.OpenReplace() }},
		{Label: "转到行…", Hint: "Ctrl+G", Run: func() { e.OpenFind(1) }},
		{Label: "转到定义", Hint: "F12", Run: func() { e.RequestDefinition() }},
		{Label: "悬停信息", Hint: "F1", Run: func() { e.RequestHover() }},
		{Label: "重命名符号…", Hint: "F2", Run: func() { e.RequestRename() }},
		{Sep: true},
		{Label: "行注释切换", Hint: "Ctrl+/", Run: func() { e.ToggleComment() }},
		{Label: "复制当前行", Hint: "Ctrl+D", Run: func() { e.DuplicateLine() }},
		{Label: "格式化文档", Hint: "Shift+Alt+F", Run: func() { e.FormatDocument() }},
		{Label: "软换行", Hint: "Alt+Z", Checked: e.Wrap, Run: func() { e.SetWrap(!e.Wrap) }},
		{Label: "折叠 / 展开块", Hint: "Ctrl+Shift+[", Run: func() { e.ToggleFoldAtCaret() }},
		{Label: "问题面板", Hint: "F8", Run: func() { e.ToggleDiagPanel() }},
	}
	if !hasSel {
		for i := range items {
			if items[i].Label == "剪切" {
				items[i].Run = nil // 剪切必须有选区；复制无选区时复制整行
			}
		}
	}
	if !clip {
		for i := range items {
			if items[i].Label == "粘贴" {
				items[i].Run = nil
			}
		}
	}
	return items
}

// openMenu 在节点内坐标 (x,y) 打开右键菜单。
func (e *CodeEditorState) openMenu(x, y float32) {
	e.menuOpen = true
	e.menuX, e.menuY = x, y
	e.menuSel = 0
	e.findOpen = false
	e.compOpen = false
	RequestRedraw()
}

// menuClick 命中右键菜单时执行；返回 true 表示事件已被菜单消费。
func (e *CodeEditorState) menuClick(x, y float32) bool {
	r, ok := LastFrameOf(e.Key)
	if !ok {
		e.menuOpen = false
		return false
	}
	places, box := e.menuLayout(geom.Rect{W: r.W, H: r.H})
	if x < box.X || x > box.X+box.W || y < box.Y || y > box.Y+box.H {
		e.menuOpen = false
		return false
	}
	items := e.menuItems()
	for _, p := range places {
		if p.Sep || p.Index >= len(items) {
			continue
		}
		if x >= p.X && x <= p.X+p.W && y >= p.Y && y <= p.Y+p.H {
			e.menuSel = p.Index
			e.menuOpen = false
			if items[p.Index].Run != nil {
				items[p.Index].Run()
			}
			RequestRedraw()
			return true
		}
	}
	return true
}

// ---- 补全 ----

// openBuiltinCompletion 内置补全：当前语言的关键字 / 类型 / 文档内已出现的标识符。
func (e *CodeEditorState) openBuiltinCompletion() {
	if e.ReadOnly {
		return
	}
	prefix := e.wordPrefix()
	kw := codeKeywords[e.Language]
	types := codeTypes[e.Language]
	seen := map[string]bool{}
	var items []CodeCompletion
	add := func(s, detail string) bool {
		if s == "" || seen[s] || len([]rune(s)) <= len([]rune(prefix)) && !strings.EqualFold(s, prefix) {
			return false
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix)) {
			return false
		}
		seen[s] = true
		items = append(items, CodeCompletion{Label: s, Detail: detail, Insert: s})
		return len(items) >= 200
	}
	for _, k := range kw {
		if add(k, "关键字") {
			break
		}
	}
	for _, t := range types {
		if add(t, "类型") {
			break
		}
	}
	// 文档内的标识符（只看光标附近 400 行，避免十万行全扫描）
	from := e.Pos.Line - 400
	if from < 0 {
		from = 0
	}
	to := e.Pos.Line + 400
	if to > len(e.Lines) {
		to = len(e.Lines)
	}
	for i := from; i < to && len(items) < 200; i++ {
		line := e.Lines[i]
		start := -1
		runes := []rune(line)
		for j := 0; j <= len(runes); j++ {
			var r rune
			if j < len(runes) {
				r = runes[j]
			}
			if j < len(runes) && isIdentRune(r) {
				if start < 0 {
					start = j
				}
				continue
			}
			if start >= 0 {
				word := string(runes[start:j])
				if isIdentStart([]rune(word)[0]) && add(word, "文档") {
					start = -1
					break
				}
				start = -1
			}
		}
	}
	if len(items) == 0 {
		e.compOpen = false
		return
	}
	sortCompletion(items)
	e.compItems = items
	e.compSel = 0
	e.compOpen = true
	rect, ok := LastFrameOf(e.Key)
	if ok {
		g := e.gutterTotal()
		x := rect.X + g - e.ScrollX + e.xOfCol(e.Pos.Line, e.Pos.Col)
		y := rect.Y + codeTopPad + float32(e.Pos.Line+1)*e.lineH() - e.ScrollY
		// 相对节点
		e.compX = x - rect.X
		e.compY = y - rect.Y
	}
	RequestRedraw()
}

// wordPrefix 光标前的单词前缀。
func (e *CodeEditorState) wordPrefix() string {
	runes := []rune(e.line(e.Pos.Line))
	col := codeClamp(e.Pos.Col, 0, len(runes))
	i := col
	for i > 0 && isIdentRune(runes[i-1]) {
		i--
	}
	return string(runes[i:col])
}

// filterCompletion 输入后按前缀过滤。
func (e *CodeEditorState) filterCompletion() {
	p := strings.ToLower(e.wordPrefix())
	if p == "" {
		return
	}
	out := e.compItems[:0]
	for _, it := range e.compItems {
		if strings.HasPrefix(strings.ToLower(it.Label), p) {
			out = append(out, it)
		}
	}
	e.compItems = out
	if len(e.compItems) == 0 {
		e.compOpen = false
		return
	}
	if e.compSel >= len(e.compItems) {
		e.compSel = len(e.compItems) - 1
	}
}

// compClick 补全列表点击；返回 true 表示已消费。
func (e *CodeEditorState) compClick(x, y float32) bool {
	if len(e.compItems) == 0 {
		return false
	}
	rowH := float32(22)
	w := float32(300)
	vis := len(e.compItems)
	if vis > 8 {
		vis = 8
	}
	mx, my := e.compX, e.compY
	if x < mx || x > mx+w || y < my || y > my+float32(vis)*rowH+8 {
		return false
	}
	idx := int((y - my - 4) / rowH)
	start := 0
	if e.compSel >= vis {
		start = e.compSel - vis + 1
	}
	idx += start
	if idx >= 0 && idx < len(e.compItems) {
		e.compSel = idx
		e.AcceptCompletion()
	}
	return true
}

// AcceptCompletion 接受当前补全项（替换光标前的单词前缀）。
func (e *CodeEditorState) AcceptCompletion() {
	if !e.compOpen || e.compSel < 0 || e.compSel >= len(e.compItems) {
		e.compOpen = false
		return
	}
	it := e.compItems[e.compSel]
	ins := it.Insert
	if ins == "" {
		ins = it.Label
	}
	prefix := e.wordPrefix()
	start := CodePos{Line: e.Pos.Line, Col: e.Pos.Col - len([]rune(prefix))}
	e.compOpen = false
	e.replaceRange(start, e.Pos, ins, true)
	// LSP 额外字段：文档 / 位置
	if it.DocPath != "" && e.hoverLines == nil {
		e.hoverLines = []string{"来源：" + it.DocPath}
	}
	RequestRedraw()
}

// ---- 命令（菜单 / 快捷键 / 页面按钮共用）----

// FormatDocument 格式化整个文档（有 LSP 用 LSP，否则做括号缩进整理）。
func (e *CodeEditorState) FormatDocument() {
	if e.srv != nil {
		if txt, ok := e.srv.Format(e.Path, e.Text(), e.TabSize); ok && txt != "" {
			e.applyLSPText(txt, "已通过 LSP 格式化")
			return
		}
	}
	e.autoIndentAll()
	e.status("已按内置规则整理缩进")
}

// autoIndentAll 内置格式化：按大括号层级重排行首缩进（保留空行与注释）。
func (e *CodeEditorState) autoIndentAll() {
	if e.ReadOnly {
		return
	}
	step := e.tabSize()
	depth := 0
	changed := false
	for i := 0; i < len(e.Lines); i++ {
		trim := strings.TrimSpace(e.Lines[i])
		if trim == "" {
			continue
		}
		// 行首是 } ) ] 时先减一层
		lineDepth := depth
		if strings.HasPrefix(trim, "}") || strings.HasPrefix(trim, ")") || strings.HasPrefix(trim, "]") {
			if lineDepth > 0 {
				lineDepth--
			}
		}
		// case / default 在 switch 内退回一层（Go）
		if strings.HasPrefix(trim, "case ") || strings.HasPrefix(trim, "default:") {
			if lineDepth > 0 {
				lineDepth--
			}
		}
		want := strings.Repeat(" ", lineDepth*step)
		if trim != e.Lines[i] && !strings.HasPrefix(e.Lines[i], want+strings.TrimLeft(trim, "")) {
			// 只在本行确实缩进不当的时候改
			if leadingSpaces(e.Lines[i]) != len(want) {
				e.Lines[i] = want + trim
				changed = true
			}
		}
		// 更新深度（按本行的括号净值，忽略字符串与注释里的括号）
		depth += codeBracketDelta(stripCodeStrings(trim))
		if depth < 0 {
			depth = 0
		}
	}
	if changed {
		e.changeSeq++
		e.cache.reset()
		e.Dirty = true
		if e.OnChange != nil {
			e.OnChange(e.Text())
		}
	}
	RequestRedraw()
}

// applyLSPText 用 LSP 返回的全文替换文档（保留光标行）。
func (e *CodeEditorState) applyLSPText(txt, msg string) {
	line := e.Pos.Line
	e.SetText(txt)
	if line < len(e.Lines) {
		e.SetCaret(CodePos{Line: line}, false)
	}
	e.status(msg)
	e.lspSync()
	RequestRedraw()
}

// NextDiagnostic 跳到下一条诊断（循环）。
func (e *CodeEditorState) NextDiagnostic() {
	ds := e.allDiagnostics()
	if len(ds) == 0 {
		e.status("没有诊断信息")
		return
	}
	best := -1
	for i, d := range ds {
		if d.Line > e.Pos.Line || (d.Line == e.Pos.Line && d.Col > e.Pos.Col) {
			if best < 0 || ds[best].Line > d.Line {
				best = i
			}
		}
	}
	if best < 0 {
		best = 0
	}
	d := ds[best]
	e.SetCaret(CodePos{Line: d.Line, Col: d.Col}, false)
	e.ensureCaretVisible()
	e.hoverLines = []string{d.Message}
	if d.Source != "" {
		e.hoverLines = append(e.hoverLines, "来源："+d.Source)
	}
	rect, ok := LastFrameOf(e.Key)
	if ok {
		e.hoverX = 24
		e.hoverY = rect.H - float32(len(e.hoverLines))*17 - 24
	}
	e.hoverOpen = true
	RequestRedraw()
}

// CodeEditorShortcuts 编辑器内建快捷键（帮助面板 / 文档用）。
func CodeEditorShortcuts() [][2]string {
	return [][2]string{
		{"Ctrl+F / Ctrl+H", "查找 / 替换（替换支持全部替换）"},
		{"Alt+Z", "软换行开关"},
		{"Ctrl+Shift+[ / ]", "折叠当前块 / 全部展开"},
		{"Ctrl+Alt+↑ / ↓", "添加光标（多光标编辑）"},
		{"Alt+点击", "在点击处添加光标"},
		{"Esc", "收起多光标 / 关闭面板"},
		{"Ctrl+Shift+M", "问题面板（可点击跳转）"},
		{"拖动滚动条 / 小地图", "连续滚动（按住拖动）"},
		{"Ctrl+S", "保存（触发 OnSave）"},
		{"Ctrl+Z / Ctrl+Y", "撤销 / 重做"},
		{"Ctrl+C / X / V", "复制 / 剪切 / 粘贴"},
		{"Ctrl+A", "全选"},
		{"Ctrl+D", "复制当前行"},
		{"Ctrl+/", "行注释切换"},
		{"Ctrl+F / Ctrl+G", "查找 / 转到行"},
		{"F3 / Shift+F3", "查找下一个 / 上一个"},
		{"Ctrl+Space", "触发补全"},
		{"F1", "悬停信息"},
		{"F2", "重命名符号"},
		{"F8", "跳到下一条诊断"},
		{"F12", "转到定义"},
		{"Alt+↑ / Alt+↓", "上移 / 下移当前行"},
		{"Shift+Alt+F", "格式化文档"},
		{"Tab / Shift+Tab", "缩进 / 反缩进（多行选区整体生效）"},
		{"Ctrl+← / Ctrl+→", "按词移动"},
		{"Home / End", "行首 / 行尾（Home 两段式）"},
	}
}

// codeMenuRowH / codeMenuW 右键菜单的单行高度与宽度。
const (
	codeMenuRowH = float32(26)
	codeMenuW    = float32(216)
)

// codeMenuPlace 菜单项在节点坐标系里的位置。
type codeMenuPlace struct {
	Index  int
	X, Y   float32
	W, H   float32
	Column int
	Sep    bool
}

// menuLayout 计算右键菜单布局（单列放不下时自动分两列），
// 返回每项位置与整体矩形（相对节点左上角）。
// 绘制与点击命中共用同一份布局，不会出现“看得见但点不中”。
func (e *CodeEditorState) menuLayout(r geom.Rect) ([]codeMenuPlace, geom.Rect) {
	items := e.menuItems()
	heightOf := func(from, to int) float32 {
		h := float32(10)
		for i := from; i < to; i++ {
			if items[i].Sep {
				h += 7
			} else {
				h += codeMenuRowH
			}
		}
		return h
	}
	n := len(items)
	per := n
	cols := 1
	if heightOf(0, n) > r.H-8 {
		cols = 2
		per = (n + 1) / 2
	}
	total := float32(0)
	var out []codeMenuPlace
	for c := 0; c < cols; c++ {
		from := c * per
		to := from + per
		if to > n {
			to = n
		}
		if from >= to {
			break
		}
		h := heightOf(from, to)
		if h > total {
			total = h
		}
		y := float32(5)
		for i := from; i < to; i++ {
			if items[i].Sep {
				out = append(out, codeMenuPlace{Index: i, X: float32(c) * codeMenuW, Y: y, W: codeMenuW, H: 7, Column: c, Sep: true})
				y += 7
				continue
			}
			out = append(out, codeMenuPlace{Index: i, X: float32(c) * codeMenuW, Y: y, W: codeMenuW, H: codeMenuRowH, Column: c})
			y += codeMenuRowH
		}
	}
	totalW := codeMenuW * float32(cols)
	// 整体贴边修正（右下溢出时向左上回退）
	x, y := e.menuX, e.menuY
	if x+totalW > r.W-4 {
		x = r.W - totalW - 4
	}
	if y+total > r.H-4 {
		y = r.H - total - 4
	}
	if x < 2 {
		x = 2
	}
	if y < 2 {
		y = 2
	}
	e.menuX, e.menuY = x, y
	for i := range out {
		out[i].X += x
		out[i].Y += y
	}
	return out, geom.Rect{X: x, Y: y, W: totalW, H: total}
}
