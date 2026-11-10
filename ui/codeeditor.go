// guis — 原生代码编辑器（一）：状态机 / 编辑操作 / 键盘 / 鼠标 / 右键菜单
//
// 目标：十万行不卡顿。做法与 Monaco 一致：
//   - 文档按行存（[]string），不做整篇字符串扫描；
//   - 只对「可见行」做高亮与排版，滚动只改 ScrollY，不重建文档结构；
//   - 高亮结果按行缓存（行内容 hash + 跨行状态），编辑只失效受影响的行；
//   - 小地图按行采样成固定数量的矩形，与文档长度无关。
//
// 与框架的对接：
//
//	KCodeEditor 节点由本包自绘（paintCodeEditor），节点不再产生子节点，
//	所以十万行也只有 40~60 行的绘制量；键盘走 RegisterKeyHandler（优先于
//	Tab 焦点切换），Ctrl+A/C/X/V 走 RegisterTextEdit，右键走 OnRightClickAt。
package ui

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"guis/internal/gpu"
)

// 编辑器默认度量（逻辑像素）。
const (
	// CodeDefaultLineHeight 默认行高（单倍行距下 Consolas 13px 的视觉行高）。
	CodeDefaultLineHeight float32 = 20
	// CodeDefaultFontSize 默认字号。
	CodeDefaultFontSize float32 = 13
	// codeGutterPad 行号栏左右内边距。
	codeGutterPad float32 = 12
	// codeTopPad 文本区顶部留白。
	codeTopPad float32 = 6
	// codeMinimapDefaultW 小地图默认宽度。
	codeMinimapDefaultW float32 = 84
	// codeMinimapRowH 小地图每个采样行的高度。
	codeMinimapRowH float32 = 3
	// codeUndoLimit 撤销栈上限。
	codeUndoLimit = 400
)

// CodePos 文档位置：行号 + 行内列号（均从 0 开始，列按 rune 计）。
type CodePos struct{ Line, Col int }

// Before 位置是否严格在前。
func (p CodePos) Before(q CodePos) bool {
	if p.Line != q.Line {
		return p.Line < q.Line
	}
	return p.Col < q.Col
}

// After 位置是否严格在后。
func (p CodePos) After(q CodePos) bool { return q.Before(p) }

// Equal 位置是否相同。
func (p CodePos) Equal(q CodePos) bool { return p.Line == q.Line && p.Col == q.Col }

// CodeRange 文档区间（[Start, End)）。
type CodeRange struct{ Start, End CodePos }

// CodeDiagnostic 一条诊断（LSP publishDiagnostics 与内置解析器共用）。
type CodeDiagnostic struct {
	Line, Col       int // 起点（0 基）
	EndLine, EndCol int // 终点
	Severity        int // 1=error 2=warning 3=info 4=hint
	Message         string
	Source          string
}

// CodeTheme 编辑器配色（独立于 UI 主题，切主题时由页面自行换一套）。
type CodeTheme struct {
	Bg, GutterBg, GutterText, GutterActive gpu.Paint
	CurrentLine, Selection, Caret          gpu.Paint
	IndentGuide, Scrollbar, MinimapBg      gpu.Paint
	MinimapView, MinimapLine               gpu.Paint
	Text                                   gpu.Paint

	Keyword, Type, String, Comment gpu.Paint
	Number, Function, Operator     gpu.Paint
	Punct, Constant, Attribute     gpu.Paint
	Tag, Invalid                   gpu.Paint
}

// DarkCodeTheme 深色配色（默认）。
func DarkCodeTheme() CodeTheme {
	return CodeTheme{
		Bg:          C("#0d1117"),
		GutterBg:    C("#0b0f16"),
		GutterText:  C("#4b5563"),
		GutterActive: C("#c9d1d9"),
		CurrentLine: C("rgba(110,130,180,0.10)"),
		Selection:   C("rgba(58,110,196,0.42)"),
		Caret:        C("#e6edf3"),
		Text:         C("#c9d1d9"),
		IndentGuide: C("rgba(255,255,255,0.07)"),
		Scrollbar:   C("rgba(255,255,255,0.18)"),
		MinimapBg:   C("#0a0e14"),
		MinimapView: C("rgba(255,255,255,0.10)"),
		MinimapLine: C("rgba(160,180,210,0.55)"),

		Keyword:   C("#ff7b72"),
		Type:      C("#79c0ff"),
		String:    C("#a5d6ff"),
		Comment:   C("#8b949e"),
		Number:    C("#79c0ff"),
		Function:  C("#d2a8ff"),
		Operator:  C("#ff7b72"),
		Punct:     C("#c9d1d9"),
		Constant:  C("#79c0ff"),
		Attribute: C("#ffa657"),
		Tag:       C("#7ee787"),
		Invalid:   C("#f85149"),
	}
}

// LightCodeTheme 浅色配色。
func LightCodeTheme() CodeTheme {
	return CodeTheme{
		Bg:           C("#ffffff"),
		GutterBg:     C("#f6f8fa"),
		GutterText:   C("#8c959f"),
		GutterActive: C("#1f2328"),
		CurrentLine:  C("rgba(60,110,200,0.07)"),
		Selection:    C("rgba(84,174,255,0.30)"),
		Caret:        C("#1f2328"),
		Text:         C("#1f2328"),
		IndentGuide:  C("rgba(0,0,0,0.08)"),
		Scrollbar:    C("rgba(0,0,0,0.16)"),
		MinimapBg:    C("#fbfcfd"),
		MinimapView:  C("rgba(0,0,0,0.08)"),
		MinimapLine:  C("rgba(80,100,130,0.45)"),

		Keyword:   C("#cf222e"),
		Type:      C("#0550ae"),
		String:    C("#0a3069"),
		Comment:   C("#6e7781"),
		Number:    C("#0550ae"),
		Function:  C("#8250df"),
		Operator:  C("#cf222e"),
		Punct:     C("#1f2328"),
		Constant:  C("#0550ae"),
		Attribute: C("#953800"),
		Tag:       C("#116329"),
		Invalid:   C("#cf222e"),
	}
}

// CodeMenuItem 内建右键菜单项（空格分隔的 Hint 显示在右侧）。
type CodeMenuItem struct {
	Label   string
	Hint    string
	Sep     bool
	Checked bool
	Run     func()
}

// CodeCompletion 一条补全项（内置关键字 / LSP CompletionItem 统一结构）。
type CodeCompletion struct {
	Label   string
	Detail  string
	Insert  string
	Kind    int
	DocPath string
	Line    int
	Col     int
}

// codeDelta 一次原子编辑（撤销 / 重做都按它反向或正向套用）。
type codeDelta struct {
	Start       int // 受影响的首行
	Before      []string
	After       []string
	PosBefore   CodePos
	AnchorBefore CodePos
	PosAfter    CodePos
	AnchorAfter CodePos
}

// CodeEditorState 代码编辑器状态（跨帧保持；由 CodeEditor 构建成节点）。
type CodeEditorState struct {
	// ---- 文档（UI 线程独占）----
	Lines []string
	Pos   CodePos
	// Anchor 选区锚点：与 Pos 相同表示无选区。
	Anchor CodePos

	// ---- 视图 ----
	ScrollY, ScrollX float32
	LineH            float32
	FontSize         float32
	FontFamily       string
	TabSize          int

	Theme                    CodeTheme
	MinW, MinH               float32
	ShowGutter               bool
	ShowMinimap              bool
	ShowCurrentLine          bool
	ShowIndentGuide          bool
	MinimapW                 float32
	ReadOnly                 bool
	Dirty                    bool

	// ---- 身份 / 回调 ----
	Key      string
	Path     string // 文件路径（LSP 文档 URI 用）
	Language string // go / javascript / typescript / json / python / …
	Title    string

	OnChange func(text string)
	OnCaret  func(line, col int)
	OnSave   func(text string)
	OnStatus func(msg string)

	// ---- 内部 ----
	undo, redo []codeDelta
	cache      *codeCache
	viewW      float32
	viewH      float32
	gutterW    float32

	menuOpen     bool
	menuX, menuY float32
	menuSel      int
	findOpen     bool
	findMode     int // 0 = 查找，1 = 转到行，2 = 重命名
	findText     string
	findCase     bool
	findAt       int
	lastSync     time.Time

	compOpen   bool
	compItems  []CodeCompletion
	compSel    int
	compX, compY float32
	// renameWord 重命名输入条打开时的原符号名
	renameWord string
	// textW 本帧文本区宽度（由绘制写入，命中测试 / 补全面板复用）
	textW float32

	// ---- 视图扩展（软换行 / 折叠 / 视觉行前缀缓存，见 codeeditor_view.go）----
	Wrap       bool
	folds      []codeFold
	folded     map[int]bool
	hidden     []int32
	foldsReady bool
	rowCount   []int32
	rowBefore  []int32
	rowsValid  int

	// ---- 交互扩展 ----
	// dragKind 本次按下的落点分区（文本 / 滚动条 / 小地图），OnDrag 期间不变。
	dragKind codeHitKind
	// dragGrab 按下时鼠标在滑块内的抓取偏移，保证拖动不跳。
	dragGrab float32
	// dragActive 本次拖动是否真的在「非文本区」（滚动条 / 小地图）。
	dragActive bool
	// dragMoved 本次按下发生过拖动（松手时不再当成单击）。
	dragMoved bool
	// dragStartX/Y 按下帧的位置（判定是否真的拖动过）。
	dragStartX, dragStartY float32
	// dragPress 按下帧落点对应的文档位置（真正拖动时选区从这里开始）。
	dragPress CodePos
	// dragSelecting 文本区是否已经越过阈值、开始按拖动扩展选区。
	dragSelecting bool
	// skipClick 拖动结束后的第一次松手点击忽略（否则会折叠掉刚拖出的选区）。
	skipClick bool
	// blinkAnchor 光标闪烁的相位起点（AnimTime 秒）；输入后短暂常亮。
	blinkAnchor float32
	// cursors 额外光标（主光标仍是 Pos / Anchor）；Ctrl+Alt+↑↓ / Alt+点击添加。
	cursors []codeCursor
	// diagOpen 诊断列表面板是否展开；diagSel 选中项。
	diagOpen bool
	diagSel  int
	// replaceText 替换输入框内容；replaceFocus 0=查找框 1=替换框。
	replaceText  string
	replaceFocus int
	// findCaret / replaceCaret 两个输入框的插入位置（按 rune 计）；findSelAll Ctrl+A 全选。
	findCaret    int
	replaceCaret int
	findSelAll   bool
	// semantic LSP 语义高亮片段（行 → 片段）；semanticOn 是否启用。
	semantic      map[int][]codeSpan
	semanticOn    bool
	semanticStamp uint64
	// semanticSeq 语义 token 请求序号：每次新请求 +1，后台结果回来时序号对不上就丢弃。
	semanticSeq uint64
	// semanticBusy 已有在飞的语义 token 请求（含退避重试），避免每次按键都开一串协程。
	semanticBusy bool
	// semanticEmpty 轮询重试都拿不到 token（服务端没返回，页面不要一直说“等待服务端”）。
	semanticEmpty bool
	// quiet 批量编辑期间跳过逐次 OnChange / LSP 同步（多光标只通知一次）。
	quiet bool

	hoverText  string
	hoverX, hoverY float32
	hoverOpen  bool
	// ---- 鼠标悬停语法提示 ----
	// hoverKeyNow 鼠标当前所在符号（行:列:词），用于去重与判断是否仍停在同一个词上。
	hoverKeyNow string
	// hoverSeq 悬停去抖序号（计时回调投递时比对，过期丢弃）。
	hoverSeq int
	// hoverPinned 提示由鼠标弹出：移到别的词 / 离开编辑器 / 输入 / 点击时收起。
	hoverPinned bool
	// hoverMouseX/Y 鼠标在节点内的坐标（提示面板定位用）。
	hoverMouseX, hoverMouseY float32

	lspMu        sync.Mutex
	lspDiags     []CodeDiagnostic
	builtinDiags []CodeDiagnostic
	lspStatus    string
	// lspLog 服务端日志（单行化，最多保留 codeLSPLogMax 条；不写状态栏，状态栏只放一行摘要）。
	lspLog []string
	srv       *LSPClient
	hoverLines []string
	// hoverKinds 与 hoverLines 等长：每行的排版类型（代码 / 标题 / 列表 / 虚线）。
	hoverKinds []codeHoverKind

	lastClick     time.Time
	lastClickLine int
	autoPairs     bool
	changeSeq     uint64
	searchSeq     int
}

// NewCodeEditorState 新建编辑器状态（key 必须全局唯一：同一时间只有一个编辑器持有焦点）。
func NewCodeEditorState(key, text string) *CodeEditorState {
	e := &CodeEditorState{
		Key:             key,
		LineH:           CodeDefaultLineHeight,
		FontSize:        CodeDefaultFontSize,
		TabSize:         4,
		Theme:           DarkCodeTheme(),
		MinW:            560,
		MinH:            320,
		ShowGutter:      true,
		ShowMinimap:     true,
		ShowCurrentLine: true,
		ShowIndentGuide: true,
		MinimapW:        codeMinimapDefaultW,
		autoPairs:       true,
		Language:        "plain",
		cache:           newCodeCache(),
	}
	e.SetText(text)
	return e
}

// ---- 文档基础操作 ----

// SetText 整体替换文档（重置撤销栈、光标与高亮缓存）。
func (e *CodeEditorState) SetText(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	e.Lines = strings.Split(s, "\n")
	if len(e.Lines) == 0 {
		e.Lines = []string{""}
	}
	e.Pos, e.Anchor = CodePos{}, CodePos{}
	e.undo, e.redo = nil, nil
	e.Dirty = false
	e.changeSeq++
	e.invalidate(0)
	e.resetViewState()
}

// Text 文档全文。
func (e *CodeEditorState) Text() string { return strings.Join(e.Lines, "\n") }

// LineCount 行数。
func (e *CodeEditorState) LineCount() int { return len(e.Lines) }

// SetLanguage 指定语法着色语言名（go / json / …；未知名称按 plain 处理）。
func (e *CodeEditorState) SetLanguage(name string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = "plain"
	}
	if e.Language != name {
		e.Language = name
		e.cache.reset()
		e.invalidateRows(0)
		e.semantic = nil
		RequestRedraw()
	}
}

// SetLanguageByPath 按文件名推断语言（.go / .json / .ts …）。
func (e *CodeEditorState) SetLanguageByPath(path string) {
	e.Language = codeLanguageByPath(path)
	e.cache.reset()
}

// SetTheme 换配色。
func (e *CodeEditorState) SetTheme(t CodeTheme) {
	e.Theme = t
	RequestRedraw()
}

// MarkSaved 标记已保存（清脏标记）。
func (e *CodeEditorState) MarkSaved() {
	e.Dirty = false
}

// ---- 位置 / 选区工具 ----

// Selection 返回规范化后的选区（无选区时 ok=false）。
func (e *CodeEditorState) Selection() (CodeRange, bool) {
	if e.Anchor.Equal(e.Pos) {
		return CodeRange{}, false
	}
	if e.Anchor.Before(e.Pos) {
		return CodeRange{Start: e.Anchor, End: e.Pos}, true
	}
	return CodeRange{Start: e.Pos, End: e.Anchor}, true
}

// SelectedText 选中的文本（无选区返回空串）。
func (e *CodeEditorState) SelectedText() string {
	r, ok := e.Selection()
	if !ok {
		return ""
	}
	if r.Start.Line == r.End.Line {
		ln := []rune(e.line(r.Start.Line))
		return string(ln[codeClamp(r.Start.Col, 0, len(ln)):codeClamp(r.End.Col, 0, len(ln))])
	}
	var sb strings.Builder
	first := []rune(e.line(r.Start.Line))
	sb.WriteString(string(first[codeClamp(r.Start.Col, 0, len(first)):]))
	sb.WriteByte('\n')
	for i := r.Start.Line + 1; i < r.End.Line && i < len(e.Lines); i++ {
		sb.WriteString(e.Lines[i])
		sb.WriteByte('\n')
	}
	last := []rune(e.line(r.End.Line))
	sb.WriteString(string(last[:codeClamp(r.End.Col, 0, len(last))]))
	return sb.String()
}

func (e *CodeEditorState) line(i int) string {
	if i < 0 || i >= len(e.Lines) {
		return ""
	}
	return e.Lines[i]
}

func (e *CodeEditorState) clampPos(p CodePos) CodePos {
	if p.Line < 0 {
		p.Line = 0
	}
	if p.Line >= len(e.Lines) {
		p.Line = len(e.Lines) - 1
	}
	n := utf8.RuneCountInString(e.Lines[p.Line])
	if p.Col < 0 {
		p.Col = 0
	}
	if p.Col > n {
		p.Col = n
	}
	return p
}

// SetCaret 定位光标（keep=false 折叠选区）。
func (e *CodeEditorState) SetCaret(p CodePos, keep bool) {
	e.Pos = e.clampPos(p)
	if !keep {
		e.Anchor = e.Pos
	}
	e.afterCaret()
}

// SetCaretLine 跳到指定行首（1 基，供「转到行」使用）。
func (e *CodeEditorState) SetCaretLine(line int) {
	if line < 1 {
		line = 1
	}
	if line > len(e.Lines) {
		line = len(e.Lines)
	}
	e.SetCaret(CodePos{Line: line - 1}, false)
	e.ensureCaretVisible()
}

// SelectAll 全选。
func (e *CodeEditorState) SelectAll() {
	e.Anchor = CodePos{Line: 0, Col: 0}
	last := len(e.Lines) - 1
	e.Pos = CodePos{Line: last, Col: utf8.RuneCountInString(e.Lines[last])}
	e.afterCaret()
}

func (e *CodeEditorState) afterCaret() {
	// 光标闪烁相位从最后一次操作重新计时：输入时保持常亮，停下后才开始闪。
	e.blinkAnchor = codeNow()
	if e.OnCaret != nil {
		e.OnCaret(e.Pos.Line+1, e.Pos.Col+1)
	}
	RequestRedraw()
}

// ---- 编辑（所有修改都经 replaceRange，统一记录撤销）----

// InsertText 在光标处插入文本（有选区先替换选区）。
func (e *CodeEditorState) InsertText(s string) {
	if e.ReadOnly || s == "" {
		return
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = codeStripControl(s)
	sp, ep := e.Pos, e.Pos
	if r, ok := e.Selection(); ok {
		sp, ep = r.Start, r.End
	}
	// 输入右括号 / 引号时跳过已存在的配对字符（编辑器习惯）
	e.replaceRange(sp, ep, s, true)
}

// replaceRange 用 text 替换 [sp, ep)（text 可含换行）。
// pushUndo=false 用于撤销 / 重做自身套用时不写栈。
func (e *CodeEditorState) replaceRange(sp, ep CodePos, text string, pushUndo bool) {
	sp, ep = e.clampPos(sp), e.clampPos(ep)
	if ep.Before(sp) {
		sp, ep = ep, sp
	}
	start := sp.Line
	end := ep.Line + 1
	if end > len(e.Lines) {
		end = len(e.Lines)
	}
	before := append([]string(nil), e.Lines[start:end]...)

	pre := []rune(e.line(sp.Line))
	if sp.Col > len(pre) {
		sp.Col = len(pre)
	}
	post := []rune(e.line(ep.Line))
	if ep.Col > len(post) {
		ep.Col = len(post)
	}
	head := string(pre[:sp.Col])
	tail := string(post[ep.Col:])

	var repl []string
	// caretCol 插入后光标在本行的列（rune）：插入内容末尾，而不是「新行宽 - 行尾宽」。
	// 旧实现按后者计算，行首有 head 时会把光标推到插入点右边，于是连续输入每敲一个
	// 字都往后多偏一截（表现为「输入内容乱跳位置插入」），光标列还会越过行尾被
	// clampPos 悄悄拉回，左箭头 / Delete 看起来就像失灵。
	caretCol := sp.Col
	if text == "" {
		repl = []string{head + tail}
	} else {
		ins := strings.Split(text, "\n")
		repl = make([]string, len(ins))
		for i, s := range ins {
			repl[i] = s
		}
		repl[0] = head + repl[0]
		repl[len(repl)-1] = repl[len(repl)-1] + tail
		caretCol = utf8.RuneCountInString(ins[0])
		if len(ins) == 1 {
			caretCol += sp.Col
		}
	}

	out := make([]string, 0, len(e.Lines)-len(before)+len(repl))
	out = append(out, e.Lines[:start]...)
	out = append(out, repl...)
	out = append(out, e.Lines[end:]...)
	if len(out) == 0 {
		out = []string{""}
	}

	posBefore, anchorBefore := e.Pos, e.Anchor
	e.Lines = out

	// 新光标：插入内容末尾（多行插入落在最后一行插入内容之后；纯删除落在删除起点）
	if text == "" {
		e.Pos = CodePos{Line: start, Col: sp.Col}
	} else {
		e.Pos = CodePos{Line: start + len(repl) - 1, Col: caretCol}
	}
	e.Anchor = e.Pos
	e.Pos = e.clampPos(e.Pos)

	if pushUndo {
		d := codeDelta{
			Start: start, Before: before, After: append([]string(nil), repl...),
			PosBefore: posBefore, AnchorBefore: anchorBefore,
			PosAfter: e.Pos, AnchorAfter: e.Anchor,
		}
		// 单字符连续输入合并为一条撤销（不合并会在长文档里堆出成千上万条）
		if n := len(e.undo); n > 0 && e.canCoalesce(e.undo[n-1], d) {
			e.undo[n-1].After = d.After
			e.undo[n-1].PosAfter = d.PosAfter
			e.undo[n-1].AnchorAfter = d.AnchorAfter
		} else {
			e.undo = append(e.undo, d)
			if len(e.undo) > codeUndoLimit {
				e.undo = e.undo[len(e.undo)-codeUndoLimit:]
			}
		}
		e.redo = nil
	}
	e.afterEdit(start, len(repl))
}

// canCoalesce 连续的单行插入（无换行、位置相接）合并成一次撤销。
func (e *CodeEditorState) canCoalesce(prev, cur codeDelta) bool {
	if len(prev.After) != 1 || len(cur.After) != 1 || len(prev.Before) != 1 || len(cur.Before) != 1 {
		return false
	}
	if strings.Contains(cur.After[0], "\n") {
		return false
	}
	return prev.Start == cur.Start && prev.PosAfter.Equal(cur.PosBefore) &&
		utf8.RuneCountInString(cur.After[0]) > utf8.RuneCountInString(prev.After[0])
}

// afterEdit 编辑后统一收尾：失效缓存、通知业务、LSP 增量同步。
func (e *CodeEditorState) afterEdit(startLine, nLines int) {
	e.Dirty = true
	e.changeSeq++
	e.invalidate(startLine)
	e.compOpen = false
	e.hoverOpen = false
	e.ensureCaretVisible()
	if !e.quiet {
		if e.OnChange != nil {
			e.OnChange(e.Text())
		}
		e.lspSync()
		e.afterCaret()
	}
}

// invalidate 让 startLine 之后的高亮缓存失效（跨行状态可能改变）。
func (e *CodeEditorState) invalidate(startLine int) {
	if e.cache != nil {
		e.cache.invalidateFrom(startLine)
	}
	e.invalidateRows(startLine)
	if len(e.semantic) > 0 {
		for k := range e.semantic {
			if k >= startLine {
				delete(e.semantic, k)
			}
		}
	}
	e.builtinDiags = nil
}

// Undo 撤销。
func (e *CodeEditorState) Undo() bool {
	n := len(e.undo)
	if n == 0 {
		return false
	}
	d := e.undo[n-1]
	e.undo = e.undo[:n-1]
	e.applyDelta(d.Start, d.After, d.Before)
	e.Pos, e.Anchor = d.PosBefore, d.AnchorBefore
	e.redo = append(e.redo, d)
	e.afterEdit(d.Start, len(d.Before))
	return true
}

// Redo 重做。
func (e *CodeEditorState) Redo() bool {
	n := len(e.redo)
	if n == 0 {
		return false
	}
	d := e.redo[n-1]
	e.redo = e.redo[:n-1]
	e.applyDelta(d.Start, d.Before, d.After)
	e.Pos, e.Anchor = d.PosAfter, d.AnchorAfter
	e.undo = append(e.undo, d)
	e.afterEdit(d.Start, len(d.After))
	return true
}

func (e *CodeEditorState) applyDelta(start int, from, to []string) {
	if start < 0 {
		start = 0
	}
	if start > len(e.Lines) {
		start = len(e.Lines)
	}
	end := start + len(from)
	if end > len(e.Lines) {
		end = len(e.Lines)
	}
	out := make([]string, 0, len(e.Lines)-len(from)+len(to))
	out = append(out, e.Lines[:start]...)
	out = append(out, to...)
	out = append(out, e.Lines[end:]...)
	if len(out) == 0 {
		out = []string{""}
	}
	e.Lines = out
	e.Pos, e.Anchor = e.clampPos(e.Pos), e.clampPos(e.Anchor)
}

// DeleteBack 退格（有选区先删选区；多光标时所有光标同时删）。
func (e *CodeEditorState) DeleteBack() bool {
	if e.ReadOnly {
		return false
	}
	if len(e.cursors) > 0 {
		return e.deleteAtCarets(true)
	}
	if r, ok := e.Selection(); ok {
		e.replaceRange(r.Start, r.End, "", true)
		return true
	}
	p := e.Pos
	if p.Col == 0 {
		if p.Line == 0 {
			return false
		}
		prevLen := utf8.RuneCountInString(e.line(p.Line - 1))
		e.replaceRange(CodePos{Line: p.Line - 1, Col: prevLen}, p, "", true)
		return true
	}
	// 删除整个缩进（与主流编辑器一致：行首空格按 TabSize 一档退）
	cur := []rune(e.line(p.Line))
	startCol := p.Col - 1
	if strings.TrimSpace(string(cur[:p.Col])) == "" && p.Col%e.tabSize() == 0 && p.Col > 0 {
		startCol = p.Col - e.tabSize()
		if startCol < 0 {
			startCol = 0
		}
	}
	e.replaceRange(CodePos{Line: p.Line, Col: startCol}, p, "", true)
	return true
}

// DeleteForward 删除光标后一个字符（多光标时所有光标同时删）。
func (e *CodeEditorState) DeleteForward() bool {
	if e.ReadOnly {
		return false
	}
	if len(e.cursors) > 0 {
		return e.deleteAtCarets(false)
	}
	if r, ok := e.Selection(); ok {
		e.replaceRange(r.Start, r.End, "", true)
		return true
	}
	p := e.Pos
	n := utf8.RuneCountInString(e.line(p.Line))
	if p.Col >= n {
		if p.Line >= len(e.Lines)-1 {
			return false
		}
		e.replaceRange(p, CodePos{Line: p.Line + 1, Col: 0}, "", true)
		return true
	}
	e.replaceRange(p, CodePos{Line: p.Line, Col: p.Col + 1}, "", true)
	return true
}

// InsertNewline 回车：自动保持当前缩进（花括号后再多缩进一档）。
func (e *CodeEditorState) InsertNewline() {
	if e.ReadOnly {
		return
	}
	cur := []rune(e.line(e.Pos.Line))
	col := codeClamp(e.Pos.Col, 0, len(cur))
	indent := e.visualIndent(cur, len(cur))
	if strings.TrimSpace(string(cur[:col])) == "" {
		// 光标还在缩进区里：按光标前的视觉缩进宽度回车（Tab 按 TabSize 格算）
		indent = e.visualIndent(cur, col)
	}
	next := ""
	suffix := strings.TrimSpace(string(cur[col:]))
	prev := strings.TrimSpace(string(cur[:col]))
	open := strings.HasSuffix(prev, "{") || strings.HasSuffix(prev, "(") || strings.HasSuffix(prev, "[")
	closeCh := ""
	if open {
		next = strings.Repeat(" ", indent+e.tabSize())
	} else {
		next = strings.Repeat(" ", indent)
	}
	if open && (strings.HasPrefix(suffix, "}") || strings.HasPrefix(suffix, ")") || strings.HasPrefix(suffix, "]")) {
		closeCh = strings.Repeat(" ", indent) + string(suffix[0])
	}
	if closeCh != "" {
		e.replaceRange(e.Pos, e.Pos, "\n"+next+"\n"+closeCh, true)
		// 光标留在中间那行
		e.Pos = CodePos{Line: e.Pos.Line - 1, Col: len(next)}
		e.Anchor = e.Pos
		e.afterCaret()
		return
	}
	e.replaceRange(e.Pos, e.Pos, "\n"+next, true)
}

// IndentSelection 选中多行时整体增减缩进；无选区时插入 / 删除一档空格。
func (e *CodeEditorState) IndentSelection(outdent bool) {
	if e.ReadOnly {
		return
	}
	r, ok := e.Selection()
	step := e.tabSize()
	if !ok {
		// 无选区：Tab 插入一档空格，Shift+Tab 去掉当前行的缩进
		if outdent {
			e.outdentLine(e.Pos.Line)
			return
		}
		e.replaceRange(e.Pos, e.Pos, strings.Repeat(" ", step), true)
		return
	}
	from, to := r.Start.Line, r.End.Line
	if !ok {
		from, to = e.Pos.Line, e.Pos.Line
	}
	if outdent {
		for i := from; i <= to && i < len(e.Lines); i++ {
			e.outdentLine(i)
		}
	} else {
		for i := from; i <= to && i < len(e.Lines); i++ {
			e.replaceRange(CodePos{Line: i, Col: 0}, CodePos{Line: i, Col: 0}, strings.Repeat(" ", step), true)
		}
	}
	// 选区扩展到整行，避免缩进后选区错位
	e.Anchor = CodePos{Line: from, Col: 0}
	e.Pos = CodePos{Line: to, Col: utf8.RuneCountInString(e.line(to))}
	e.afterCaret()
}

func (e *CodeEditorState) outdentLine(i int) {
	cur := e.line(i)
	step := e.tabSize()
	n := 0
	for n < len(cur) && n < step && cur[n] == ' ' {
		n++
	}
	if n == 0 {
		return
	}
	e.replaceRange(CodePos{Line: i, Col: 0}, CodePos{Line: i, Col: n}, "", true)
}

func (e *CodeEditorState) tabSize() int {
	if e.TabSize <= 0 {
		return 4
	}
	return e.TabSize
}

// ---- 光标移动 ----

// MoveCaret 移动光标（extend=true 时保持锚点即扩展选区）。
//
// 上下移动保持「视觉列」而不是字符列：缩进不同的行之间跳转不会左右漂移，
// 软换行开启时还按视觉行跨段移动。
func (e *CodeEditorState) MoveCaret(dLine, dCol int, extend bool) {
	p := e.Pos
	if dLine != 0 {
		x := e.caretXVisual()
		if e.Wrap {
			row := e.rowOfPos(p) + dLine
			if row < 0 {
				row = 0
			}
			if total := e.sumRows(); row >= total {
				row = total - 1
			}
			line, seg := e.lineOfRow(row)
			// 软换行：x 是「段内偏移」，要加上目标段起点才是文档列
			from, _ := e.segRange(line, seg)
			segText := string([]rune(e.line(line))[from:])
			p = CodePos{Line: line, Col: from + e.colAtX(segText, x)}
		} else {
			target := p.Line + dLine
			if target < 0 {
				target = 0
			}
			if target >= len(e.Lines) {
				target = len(e.Lines) - 1
			}
			p = CodePos{Line: target, Col: e.colAtX(e.line(target), x)}
		}
	}
	if dCol != 0 {
		if sel, ok := e.Selection(); ok && !extend {
			// 有选区时方向键先折叠到选区一端（与主流编辑器一致），
			// 否则会从「活动端」再挪一格，光标像是乱跳。
			if dCol < 0 {
				p = sel.Start
			} else {
				p = sel.End
			}
		} else {
			dir, n := 1, dCol
			if n < 0 {
				dir, n = -1, -n
			}
			for i := 0; i < n; i++ {
				p = e.stepCaret(p, dir)
			}
		}
	}
	p = e.clampPos(p)
	e.SetCaret(p, extend)
	e.ensureCaretVisible()
}

// stepCaret 光标左 / 右移动一个字符：行首再往左接到上一行行尾，行尾再往右接到
// 下一行行首（折叠掉的行整段跳过）。
//
// 旧实现只在行内加减列号，行首按左箭头（或行尾按右箭头）会原地不动，
// 看起来就是「左箭头键无效」。
func (e *CodeEditorState) stepCaret(p CodePos, dir int) CodePos {
	if dir < 0 {
		if p.Col > 0 {
			p.Col--
			return p
		}
		line := p.Line - 1
		for line >= 0 && e.isHidden(line) {
			line--
		}
		if line < 0 {
			return p
		}
		return CodePos{Line: line, Col: utf8.RuneCountInString(e.line(line))}
	}
	if p.Col < utf8.RuneCountInString(e.line(p.Line)) {
		p.Col++
		return p
	}
	next := e.visibleNext(p.Line)
	if next >= len(e.Lines) {
		return p
	}
	return CodePos{Line: next, Col: 0}
}

// MoveLineStart 行首 / 首个非空白（Home 键两段式）。
func (e *CodeEditorState) MoveLineStart(extend bool) {
	// 行首空白按 rune 数算（Tab 只是 1 个 rune）：leadingSpaces 把 Tab 展开成
	// 4 格视觉宽度，拿它当列号会让 Home 落到缩进后面的正文中间，
	// 之后输入就从那里插进去（看起来像「内容乱跳到别处」）。
	cur := []rune(e.line(e.Pos.Line))
	first := leadingBlankRunes(cur)
	col := 0
	if e.Pos.Col != first {
		col = first
	}
	e.SetCaret(CodePos{Line: e.Pos.Line, Col: col}, extend)
	e.ensureCaretVisible()
}

// visualIndent 前 n 个 rune 的缩进视觉宽度（Tab 按 TabSize 展开；遇到非空白即停）。
func (e *CodeEditorState) visualIndent(runes []rune, n int) int {
	if n > len(runes) {
		n = len(runes)
	}
	w := 0
	for i := 0; i < n; i++ {
		switch runes[i] {
		case ' ':
			w++
		case '\t':
			w += e.tabSize()
		default:
			return w
		}
	}
	return w
}

// MoveLineEnd 行尾。
func (e *CodeEditorState) MoveLineEnd(extend bool) {
	e.SetCaret(CodePos{Line: e.Pos.Line, Col: utf8.RuneCountInString(e.line(e.Pos.Line))}, extend)
	e.ensureCaretVisible()
}

// MoveWord 按词移动（Ctrl+←/→）。
func (e *CodeEditorState) MoveWord(dir int, extend bool) {
	p := e.Pos
	cur := []rune(e.line(p.Line))
	if dir < 0 {
		if p.Col == 0 {
			if p.Line > 0 {
				e.SetCaret(CodePos{Line: p.Line - 1, Col: utf8.RuneCountInString(e.line(p.Line - 1))}, extend)
			}
			return
		}
		i := p.Col
		for i > 0 && !codeWordRune(cur[i-1]) {
			i--
		}
		for i > 0 && codeWordRune(cur[i-1]) {
			i--
		}
		e.SetCaret(CodePos{Line: p.Line, Col: i}, extend)
		return
	}
	if p.Col >= len(cur) {
		if p.Line < len(e.Lines)-1 {
			e.SetCaret(CodePos{Line: p.Line + 1, Col: 0}, extend)
		}
		return
	}
	i := p.Col
	for i < len(cur) && !codeWordRune(cur[i]) {
		i++
	}
	for i < len(cur) && codeWordRune(cur[i]) {
		i++
	}
	e.SetCaret(CodePos{Line: p.Line, Col: i}, extend)
}

// PageMove 翻页（PageUp / PageDown）。
func (e *CodeEditorState) PageMove(dir int, extend bool) {
	rows := int(e.viewH / e.lineH()) - 2
	if rows < 3 {
		rows = 10
	}
	e.MoveCaret(dir*rows, 0, extend)
}

func (e *CodeEditorState) lineH() float32 {
	if e.LineH <= 0 {
		return CodeDefaultLineHeight
	}
	return e.LineH
}

// ---- 剪贴板 ----

// Copy 复制选区到系统剪贴板。
func (e *CodeEditorState) Copy() bool {
	s := e.SelectedText()
	if s == "" {
		// 无选区时复制当前整行（VS Code 行为）
		if e.Pos.Line < len(e.Lines) {
			s = e.Lines[e.Pos.Line]
		}
	}
	if s == "" {
		return false
	}
	return ClipboardSetText(s)
}

// Cut 剪切选区。
func (e *CodeEditorState) Cut() bool {
	if e.ReadOnly {
		return false
	}
	s := e.SelectedText()
	if s == "" {
		return false
	}
	ok := ClipboardSetText(s)
	if r, has := e.Selection(); has {
		e.replaceRange(r.Start, r.End, "", true)
	}
	return ok
}

// Paste 粘贴剪贴板文本。
func (e *CodeEditorState) Paste() bool {
	if e.ReadOnly {
		return false
	}
	s := ClipboardText()
	if s == "" {
		return false
	}
	e.InsertText(s)
	return true
}

// ---- 行操作 ----

// MoveLines 上下移动当前行 / 选中行。
func (e *CodeEditorState) MoveLines(dir int) {
	if e.ReadOnly {
		return
	}
	r, ok := e.Selection()
	from, to := e.Pos.Line, e.Pos.Line
	if ok {
		from, to = r.Start.Line, r.End.Line
	}
	if dir < 0 {
		if from == 0 {
			return
		}
		// 与上一行交换
		block := append([]string(nil), e.Lines[from:to+1]...)
		prev := e.Lines[from-1]
		e.replaceRange(CodePos{Line: from - 1, Col: 0}, CodePos{Line: to, Col: utf8.RuneCountInString(e.Lines[to])},
			strings.Join(append(block, prev), "\n"), true)
		e.Anchor = CodePos{Line: from - 1, Col: e.Anchor.Col}
		e.Pos = CodePos{Line: to - 1, Col: e.Pos.Col}
		e.afterCaret()
		return
	}
	if to >= len(e.Lines)-1 {
		return
	}
	block := append([]string(nil), e.Lines[from:to+1]...)
	nxt := e.Lines[to+1]
	e.replaceRange(CodePos{Line: from, Col: 0}, CodePos{Line: to + 1, Col: utf8.RuneCountInString(e.Lines[to+1])},
		strings.Join(append([]string{nxt}, block...), "\n"), true)
	e.Anchor = CodePos{Line: from + 1, Col: e.Anchor.Col}
	e.Pos = CodePos{Line: to + 1, Col: e.Pos.Col}
	e.afterCaret()
}

// DuplicateLine 复制当前行（Ctrl+D）。
func (e *CodeEditorState) DuplicateLine() {
	if e.ReadOnly {
		return
	}
	i := e.Pos.Line
	if i >= len(e.Lines) {
		return
	}
	cur := e.Lines[i]
	e.replaceRange(CodePos{Line: i, Col: utf8.RuneCountInString(cur)}, CodePos{Line: i, Col: utf8.RuneCountInString(cur)},
		"\n"+cur, true)
}

// ToggleComment 行注释切换（// 、# 、-- 按语言）。
func (e *CodeEditorState) ToggleComment() {
	if e.ReadOnly {
		return
	}
	tok := codeLineCommentToken(e.Language)
	if tok == "" {
		return
	}
	r, ok := e.Selection()
	from, to := e.Pos.Line, e.Pos.Line
	if ok {
		from, to = r.Start.Line, r.End.Line
	}
	allCommented := true
	for i := from; i <= to && i < len(e.Lines); i++ {
		if !strings.HasPrefix(strings.TrimSpace(e.Lines[i]), tok) {
			allCommented = false
			break
		}
	}
	for i := to; i >= from && i >= 0; i-- {
		rs := []rune(e.line(i))
		// 行首缩进按 rune 数算（Tab 只占 1 个 rune）：把视觉宽度当列号 / 字节下标，
		// // 会插进正文中间，行首带 Tab 时还会切碎字符。
		lead := leadingBlankRunes(rs)
		if allCommented {
			rest := string(rs[lead:])
			if strings.HasPrefix(rest, tok) {
				rest = strings.TrimPrefix(rest[len(tok):], " ")
				e.replaceRange(CodePos{Line: i, Col: 0}, CodePos{Line: i, Col: len(rs)},
					string(rs[:lead])+rest, true)
			}
			continue
		}
		e.replaceRange(CodePos{Line: i, Col: lead}, CodePos{Line: i, Col: lead}, tok+" ", true)
	}
	e.Anchor = CodePos{Line: from, Col: 0}
	e.Pos = CodePos{Line: to, Col: utf8.RuneCountInString(e.line(to))}
	e.afterCaret()
}

// ---- 查找 ----

// Find 查找 next / prev（区分大小写受 findCase 控制），返回是否命中。
func (e *CodeEditorState) Find(text string, forward bool) bool {
	if text == "" {
		return false
	}
	cmp := func(s string) string {
		if e.findCase {
			return s
		}
		return strings.ToLower(s)
	}
	needle := cmp(text)
	start := e.Pos
	if r, ok := e.Selection(); ok && forward {
		start = r.End
	}
	if forward {
		for i := start.Line; i < len(e.Lines); i++ {
			hay := cmp(e.Lines[i])
			off := 0
			if i == start.Line {
				off = byteColOf(hay, start.Col)
			}
			if idx := strings.Index(hay[off:], needle); idx >= 0 {
				col := utf8.RuneCountInString(hay[:off+idx])
				e.SetCaret(CodePos{Line: i, Col: col}, false)
				e.Pos.Col = col + utf8.RuneCountInString(text)
				e.Anchor = CodePos{Line: i, Col: col}
				e.findAt = i
				e.ensureCaretVisible()
				RequestRedraw()
				return true
			}
		}
		// 回绕
		for i := 0; i <= start.Line && i < len(e.Lines); i++ {
			hay := cmp(e.Lines[i])
			if idx := strings.Index(hay, needle); idx >= 0 {
				col := utf8.RuneCountInString(hay[:idx])
				e.SetCaret(CodePos{Line: i, Col: col}, false)
				e.Pos.Col = col + utf8.RuneCountInString(text)
				e.Anchor = CodePos{Line: i, Col: col}
				e.findAt = i
				e.ensureCaretVisible()
				RequestRedraw()
				return true
			}
		}
		return false
	}
	for i := start.Line; i >= 0; i-- {
		hay := cmp(e.Lines[i])
		limit := len(hay)
		if i == start.Line {
			limit = byteColOf(hay, start.Col)
		}
		if idx := strings.LastIndex(hay[:limit], needle); idx >= 0 {
			col := utf8.RuneCountInString(hay[:idx])
			e.SetCaret(CodePos{Line: i, Col: col}, false)
			e.Pos.Col = col + utf8.RuneCountInString(text)
			e.Anchor = CodePos{Line: i, Col: col}
			e.findAt = i
			e.ensureCaretVisible()
			RequestRedraw()
			return true
		}
	}
	for i := len(e.Lines) - 1; i > start.Line; i-- {
		hay := cmp(e.Lines[i])
		if idx := strings.LastIndex(hay, needle); idx >= 0 {
			col := utf8.RuneCountInString(hay[:idx])
			e.SetCaret(CodePos{Line: i, Col: col}, false)
			e.Pos.Col = col + utf8.RuneCountInString(text)
			e.Anchor = CodePos{Line: i, Col: col}
			e.findAt = i
			e.ensureCaretVisible()
			RequestRedraw()
			return true
		}
	}
	return false
}

// byteColOf rune 列 → 字节偏移（Find 在字节串里搜索，中文 / emoji 不能直接把列号当偏移）。
func byteColOf(s string, col int) int {
	if col <= 0 {
		return 0
	}
	rs := []rune(s)
	if col >= len(rs) {
		return len(s)
	}
	return len(string(rs[:col]))
}

// codeStripControl 去掉粘贴文本里的控制字符（保留 \n 与 \t）。
//
// 从终端 / PDF / Word 复制出来的文本常带 \x00-\x1f，插进文档会变成不可见的
// 「缺字形方块」，而且列宽 / 光标位置也会跟着错位。
func codeStripControl(s string) string {
	clean := true
	for _, r := range s {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// FindText 当前查找串。
func (e *CodeEditorState) FindText() string { return e.findText }

// FindOpen 查找栏是否展开。
func (e *CodeEditorState) FindOpen() bool { return e.findOpen }

// ---- 键盘 ----

// HandleKey 处理按键（框架 RegisterKeyHandler 调用；返回 true 表示已消费）。
func (e *CodeEditorState) HandleKey(vk uint32) bool {
	ctrl, shift, alt := CtrlDown(), ShiftDown(), AltDown()

	// 右键菜单打开时：上下 + 回车 + Esc
	if e.menuOpen {
		switch vk {
		case VK_ESCAPE:
			e.menuOpen = false
			return true
		case 0x26: // ↑
			if e.menuSel > 0 {
				e.menuSel--
			}
			return true
		case 0x28: // ↓
			if e.menuSel < len(e.menuItems())-1 {
				e.menuSel++
			}
			return true
		case VK_RETURN:
			items := e.menuItems()
			if e.menuSel >= 0 && e.menuSel < len(items) && items[e.menuSel].Run != nil {
				it := items[e.menuSel]
				e.menuOpen = false
				it.Run()
			}
			return true
		}
	}

	// 补全列表打开
	if e.compOpen {
		switch vk {
		case VK_ESCAPE:
			e.compOpen = false
			return true
		case 0x26:
			if e.compSel > 0 {
				e.compSel--
				RequestRedraw()
			}
			return true
		case 0x28:
			if e.compSel < len(e.compItems)-1 {
				e.compSel++
				RequestRedraw()
			}
			return true
		case VK_RETURN, VK_TAB:
			e.AcceptCompletion()
			return true
		}
	}

	// 问题面板打开
	if e.diagOpen {
		switch vk {
		case VK_ESCAPE:
			e.diagOpen = false
			return true
		case 0x26: // ↑
			e.DiagStep(-1)
			return true
		case 0x28: // ↓
			e.DiagStep(1)
			return true
		case VK_RETURN:
			e.PickDiagnostic(e.diagSel)
			return true
		}
	}

	// 查找 / 替换 / 转到行 / 重命名输入框打开
	if e.findOpen {
		switch vk {
		case VK_ESCAPE:
			e.CloseFind()
			return true
		case VK_TAB:
			if e.findMode == 3 {
				if e.replaceFocus == 0 {
					e.replaceFocus = 1
					e.replaceCaret = runeLen(e.replaceText)
				} else {
					e.replaceFocus = 0
					e.findCaret = runeLen(e.findText)
				}
				e.findSelAll = false
				RequestRedraw()
			}
			return true
		case 0x25: // ← 面板内移动插入点（不能落到编辑器里）
			e.FindCaretMove(-1)
			return true
		case 0x27: // →
			e.FindCaretMove(1)
			return true
		case 0x24: // Home
			e.FindCaretHome()
			return true
		case 0x23: // End
			e.FindCaretEnd()
			return true
		case 0x2E: // Delete
			e.FindDelete()
			return true
		case 0x26: // ↑ 上一个命中
			if e.findMode == 0 || e.findMode == 3 {
				e.Find(e.findText, false)
			}
			return true
		case 0x28: // ↓ 下一个命中
			if e.findMode == 0 || e.findMode == 3 {
				e.Find(e.findText, true)
			}
			return true
		case VK_RETURN:
			switch e.findMode {
			case 1:
				if n, err := parseIntSafe(e.findText); err == nil {
					e.GoToLine(n)
				}
			case 2:
				name := e.findText
				e.findOpen = false
				if e.renameWord != "" {
					e.applyRename(e.renameWord, name)
				}
			case 3:
				if ctrl {
					if n := e.ReplaceAll(); n > 0 {
						e.status("已全部替换 " + codeItoa(n) + " 处")
					} else {
						e.status("没有匹配项")
					}
				} else if e.ReplaceCurrent() {
					e.status("已替换 1 处（Ctrl+Enter 全部替换）")
				} else {
					e.status("没有匹配项")
				}
			case 0:
				e.Find(e.findText, !shift)
			}
			return true
		case VK_BACK:
			e.FindBackspace()
			return true
		}
		if ctrl && !alt {
			switch vk {
			case 'A', 'a':
				e.FindSelectAll()
				return true
			case 'V', 'v':
				e.FindPaste()
				return true
			case 'C', 'c', 'X', 'x':
				// 面板是单行输入：复制/剪切不去动编辑器选区
				return true
			}
		}
		if ctrl || alt {
			// 其它带修饰键的组合在面板打开时一律不落到编辑器
			// （否则 Ctrl+Z 会撤回代码、Ctrl+S 会存盘，看起来像“两边都聚焦”）
			return true
		}
		if vk == 0x72 { // F3 查找下一个 / 上一个
			e.Find(e.findText, !shift)
			return true
		}
		return false
	}

	// 多光标：Ctrl+Alt+↑ / ↓ 在当前光标上下各加一个光标
	if ctrl && alt {
		switch vk {
		case 0x26: // ↑
			if e.AddCursor(-1) {
				e.status("多光标：" + codeItoa(e.CursorCount()) + " 个")
			}
			return true
		case 0x28: // ↓
			if e.AddCursor(1) {
				e.status("多光标：" + codeItoa(e.CursorCount()) + " 个")
			}
			return true
		}
	}

	// 带修饰键的编辑器命令（在全局快捷键之前拦截）
	if ctrl && !alt {
		switch vk {
		case 'S', 's':
			if e.OnSave != nil {
				e.OnSave(e.Text())
			}
			e.Dirty = false
			e.status("已保存")
			return true
		case 'Z', 'z':
			if shift {
				e.Redo()
			} else {
				e.Undo()
			}
			return true
		case 'Y', 'y':
			e.Redo()
			return true
		case 'D', 'd':
			e.DuplicateLine()
			return true
		case VK_SPACE:
			e.RequestCompletion()
			return true
		case '/', 0xBF, 0xE2: // Ctrl+/（不同键盘布局的斜杠键码）
			e.ToggleComment()
			return true
		case 'F', 'f':
			// 走 OpenFind：统一初始化插入点 / 全选状态，否则会沿用上次的位置
			e.OpenFind(0)
			return true
		case 'G', 'g':
			e.OpenFind(1)
			return true
		case 'H', 'h', VK_BACK:
			// Windows 下 Ctrl+H 发的是 VK_BACK（0x08，等同 Ctrl+Backspace），
			// 两个键码都要接，否则按 Ctrl+H 会变成删一个字符。
			e.OpenReplace()
			return true
		case 'M', 'm':
			if shift {
				e.ToggleDiagPanel()
				return true
			}
			return false
		case 0xDB: // Ctrl+Shift+[ 折叠当前块
			if shift {
				e.ToggleFoldAtCaret()
				return true
			}
			return false
		case 0xDD: // Ctrl+Shift+] 全部展开
			if shift {
				e.FoldAll(false)
				return true
			}
			return false
		case 'K', 'k':
			return false // 交给命令面板
		}
	}

	if alt && !ctrl {
		switch vk {
		case 0x26: // Alt+↑ 上移行
			if len(e.cursors) > 0 {
				e.MoveCarets(-1)
			} else {
				e.MoveLines(-1)
			}
			return true
		case 0x28: // Alt+↓ 下移行
			if len(e.cursors) > 0 {
				e.MoveCarets(1)
			} else {
				e.MoveLines(1)
			}
			return true
		case 'Z', 'z': // Alt+Z 软换行
			e.SetWrap(!e.Wrap)
			e.status(map[bool]string{true: "软换行：开", false: "软换行：关"}[e.Wrap])
			return true
		}
	}

	// 无修饰键的编辑键
	switch vk {
	case VK_TAB:
		e.IndentSelection(shift)
		return true
	case VK_RETURN:
		e.InsertNewline()
		return true
	case VK_BACK:
		e.DeleteBack()
		return true
	case VK_DELETE:
		e.DeleteForward()
		return true
	case 0x25: // ←
		if ctrl {
			e.MoveWord(-1, shift)
		} else {
			e.MoveCaret(0, -1, shift)
		}
		return true
	case 0x27: // →
		if ctrl {
			e.MoveWord(1, shift)
		} else {
			e.MoveCaret(0, 1, shift)
		}
		return true
	case 0x26: // ↑
		e.MoveCaret(-1, 0, shift)
		return true
	case 0x28: // ↓
		e.MoveCaret(1, 0, shift)
		return true
	case 0x24: // Home
		e.MoveLineStart(shift)
		return true
	case 0x23: // End
		e.MoveLineEnd(shift)
		return true
	case 0x21: // PageUp
		e.PageMove(-1, shift)
		return true
	case 0x22: // PageDown
		e.PageMove(1, shift)
		return true
	case VK_ESCAPE:
		if len(e.cursors) > 0 {
			e.ClearCursors()
			return true
		}
		return false
	case 0x72: // F3
		e.Find(e.findText, !shift)
		return true
	case 0x77: // F8 问题面板 / Shift+F8 下一条
		if shift {
			e.NextDiagnostic()
		} else {
			e.ToggleDiagPanel()
		}
		return true
	case 0x7B: // F12 转到定义
		e.RequestDefinition()
		return true
	case 0x71: // F2 重命名
		e.RequestRename()
		return true
	case 0x70: // F1（帮助）/ 悬停信息
		e.RequestHover()
		return true
	}
	return false
}

// HandleChar 处理字符输入（OnChar）。
func (e *CodeEditorState) HandleChar(r rune) {
	// 控制字符不是文本：Backspace / Enter / Tab 的键位由 HandleKey 处理，
	// TranslateMessage 额外送来的 0x08 / 0x0D / 0x09 若当成普通字符插入，
	// 文档里会多出不可见字符（渲染成缺字形方块），光标也跟着错位。
	if r < 0x20 || r == 0x7f {
		if r != '\t' && r != '\r' && r != '\n' {
			return
		}
	}
	if e.menuOpen {
		return
	}
	if e.findOpen {
		if r == '\n' || r == '\r' {
			return
		}
		e.appendFindRune(r)
		return
	}
	// 悬停信息 / 问题面板在任何输入下自动关闭
	e.hoverOpen = false
	e.hoverPinned = false
	e.hoverKeyNow = ""
	if e.diagOpen {
		e.diagOpen = false
	}
	if r == '\t' || r == '\r' || r == '\n' {
		return
	}
	if e.ReadOnly {
		return
	}
	// 自动配对：输入 ) ] } " ' 时若右边正是同一字符则只移动光标
	if e.autoPairs && (r == ')' || r == ']' || r == '}' || r == '"' || r == '\'') {
		cur := []rune(e.line(e.Pos.Line))
		if e.Pos.Col < len(cur) && cur[e.Pos.Col] == r {
			if _, has := e.Selection(); !has {
				e.MoveCaret(0, 1, false)
				e.lspTrigger(')', e.Pos)
				return
			}
		}
	}
	if len(e.cursors) > 0 {
		e.InsertAtCarets(string(r))
		return
	}
	e.InsertText(string(r))
	if e.compOpen {
		e.filterCompletion()
	}
	e.lspTrigger(r, e.Pos)
}

// lspTrigger 按输入字符触发补全（. 与字母触发，其它只过滤已开的列表）。
func (e *CodeEditorState) lspTrigger(r rune, _ CodePos) {
	if e.srv == nil {
		// 未接 LSP 时用内置关键字补全
		if r == '.' || unicode.IsLetter(r) || r == '_' {
			e.openBuiltinCompletion()
		}
		return
	}
	if r == '.' || unicode.IsLetter(r) || r == '_' {
		e.RequestCompletion()
	}
}

// ---- 视图滚动 ----

// scrollBy 滚轮（dy>0 向下）。
func (e *CodeEditorState) scrollBy(dy float32) {
	e.ScrollY += dy * 3 * e.lineH()
	e.clampScroll()
	RequestRedraw()
}

// resetViewState 文档整体替换后清空视觉行 / 折叠缓存。
func (e *CodeEditorState) resetViewState() {
	e.folds = nil
	e.folded = nil
	e.hidden = nil
	e.foldsReady = false
	e.rowCount = nil
	e.rowBefore = nil
	e.rowsValid = 0
	e.semantic = nil
	e.cursors = nil
	e.dragKind = codeHitText
}

// ScrollByLines 键盘 / API 滚动。
func (e *CodeEditorState) ScrollByLines(n float32) {
	e.ScrollY += n * e.lineH()
	e.clampScroll()
	RequestRedraw()
}

// ScrollToLine 把指定行滚动到视口顶部（1 基）。
func (e *CodeEditorState) ScrollToLine(line int) {
	if line < 1 {
		line = 1
	}
	if line > len(e.Lines) {
		line = len(e.Lines)
	}
	e.ScrollY = e.yOfLine(line - 1)
	e.clampScroll()
	RequestRedraw()
}

func (e *CodeEditorState) maxScrollY() float32 {
	content := float32(e.sumRows()) * e.lineH()
	m := content - e.viewH + e.lineH()/2
	if m < 0 {
		return 0
	}
	return m
}

func (e *CodeEditorState) clampScroll() {
	max := e.maxScrollY()
	if e.ScrollY < 0 {
		e.ScrollY = 0
	}
	if e.ScrollY > max {
		e.ScrollY = max
	}
	maxX := e.maxScrollX()
	if e.ScrollX < 0 {
		e.ScrollX = 0
	}
	if e.ScrollX > maxX {
		e.ScrollX = maxX
	}
}

// ensureCaretVisible 保证光标在视口内（滚动最小距离；软换行 / 折叠按视觉行计算）。
func (e *CodeEditorState) ensureCaretVisible() {
	if e.viewH <= 0 {
		return
	}
	lh := e.lineH()
	y := e.yOfPos(e.Pos)
	if y < e.ScrollY {
		e.ScrollY = y
	} else if y+lh > e.ScrollY+e.viewH {
		e.ScrollY = y + lh - e.viewH + lh*0.2
	}
	x := e.caretXVisual()
	w := e.textAreaW()
	if e.Wrap {
		e.ScrollX = 0
	} else {
		if x < e.ScrollX {
			e.ScrollX = x - 40
		} else if x > e.ScrollX+w-24 {
			e.ScrollX = x - w + 80
		}
	}
	e.clampScroll()
}

// ---- 鼠标 ----

// PosAt 把节点内坐标换算成文档位置（不修改状态；软换行 / 折叠已经纳入）。
func (e *CodeEditorState) PosAt(x, y float32) CodePos { return e.posAtXY(x, y) }

// colAtX 用二分查找定位列（避免逐字符测量）。
func (e *CodeEditorState) colAtX(line string, x float32) int {
	if x <= 0 {
		return 0
	}
	runes := []rune(line)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi) / 2
		if e.measure(e.expandTabs(string(runes[:mid+1]))) <= x {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// ClickAt 左键点击：定位光标 / 双击选词。
func (e *CodeEditorState) ClickAt(x, y float32) {
	p := e.PosAt(x, y)
	now := time.Now()
	double := now.Sub(e.lastClick) < 420*time.Millisecond && p.Line == e.lastClickLine
	e.lastClick, e.lastClickLine = now, p.Line
	if double {
		e.selectWordAt(p)
		e.afterCaret()
		return
	}
	e.SetCaret(p, ShiftDown())
}

// DragTo 拖动选择（文本区）。
func (e *CodeEditorState) DragTo(x, y float32) {
	p := e.PosAt(x, y)
	if e.Anchor.Equal(e.Pos) {
		e.Anchor = e.Pos
	}
	e.Pos = p
	e.ensureCaretVisible()
	e.afterCaret()
}

// TextClick 文本区左键单击：Alt+点击新增光标，普通点击收起多光标后定位。
//
// 普通点击必须收起多光标：否则额外光标会一直留着（界面上永远是「双光标」），
// 之后的输入 / 退格又对所有光标批量生效，看起来像「点了别处却选中多行」。
func (e *CodeEditorState) TextClick(x, y float32, alt bool) {
	if alt {
		e.AddCursorAt(e.PosAt(x, y))
		e.status("已添加光标（" + codeItoa(e.CursorCount()) + " 个）")
		return
	}
	e.ClearCursors()
	e.ClickAt(x, y)
}

// MouseDrag 鼠标拖动（含按下帧）：按下帧只记录落点，越过阈值后才开始选。
//
// 框架的 OnDrag 在按下帧就会回调，旧实现直接 DragTo：一次带轻微抖动的普通点击
// 都会被当成拖动，从上一个光标位置拉出一条跨行选区（「点一下选中多行」）。
func (e *CodeEditorState) MouseDrag(x, y float32) {
	if e.menuOpen {
		return
	}
	if !e.dragActive {
		// 第一帧：把落点定成起始点（框架的 OnDrag 在按下帧就会回调，
		// 所以这里拿到的就是按下位置，足够判定拖动阈值）
		e.dragActive = true
		e.dragStartX, e.dragStartY = x, y
		e.dragPress = e.PosAt(x, y)
		if z := e.hitZone(x, y); z != codeHitText {
			e.dragKind = z
			e.startDrag(z, x, y)
		}
		// 文本区按下帧不改选区：只是点一下的时候不能先拖出一条选区
		return
	}
	if dist := absF(x-e.dragStartX) + absF(y-e.dragStartY); dist > 3 {
		e.dragMoved = true
	}
	if e.dragKind != codeHitText {
		e.dragScroll(e.dragKind, x, y)
		return
	}
	if !e.dragMoved {
		return
	}
	if !e.dragSelecting {
		// 真拖动：选区从按下的位置开始，而不是从上一个光标位置开始；
		// 同时收起多光标（新建选区时旧光标没有意义）
		e.dragSelecting = true
		e.ClearCursors()
		e.Anchor = e.clampPos(e.dragPress)
		e.Pos = e.Anchor
	}
	e.DragTo(x, y)
}

// MouseDragEnd 结束拖动（拖动后的第一次松手点击不再折叠刚选出来的选区）。
func (e *CodeEditorState) MouseDragEnd() {
	if e.dragMoved {
		e.skipClick = true
	}
	e.dragKind = codeHitText
	e.dragGrab = 0
	e.dragActive = false
	e.dragMoved = false
	e.dragSelecting = false
	e.dragPress = CodePos{}
}

// selectWordAt 选中光标所在单词。
func (e *CodeEditorState) selectWordAt(p CodePos) {
	runes := []rune(e.line(p.Line))
	if len(runes) == 0 {
		return
	}
	col := codeClamp(p.Col, 0, len(runes))
	if col == len(runes) && col > 0 {
		col--
	}
	if !codeWordRune(runes[col]) {
		e.Anchor, e.Pos = p, p
		return
	}
	lo, hi := col, col+1
	for lo > 0 && codeWordRune(runes[lo-1]) {
		lo--
	}
	for hi < len(runes) && codeWordRune(runes[hi]) {
		hi++
	}
	e.Anchor = CodePos{Line: p.Line, Col: lo}
	e.Pos = CodePos{Line: p.Line, Col: hi}
}

// ---- 小工具 ----

func codeClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
			continue
		}
		if r == '\t' {
			n += 4
			continue
		}
		break
	}
	return n
}

// leadingBlankRunes 行首空白占的 rune 数（Tab 只算 1 个 rune）。
//
// 光标列 / 切片下标必须用它：leadingSpaces 给的是「视觉宽度」（Tab 展开成 4 格），
// 在 Tab 缩进的行上两者差好几列，混用会把光标定位到缩进后面的正文中间。
func leadingBlankRunes(runes []rune) int {
	n := 0
	for _, r := range runes {
		if r == ' ' || r == '\t' {
			n++
			continue
		}
		break
	}
	return n
}

func codeWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$' || r > 0x2E7F
}

func parseIntSafe(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("空")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("非法数字")
		}
		n = n*10 + int(r-'0')
	}
	if n <= 0 {
		return 0, fmt.Errorf("非法行号")
	}
	return n, nil
}

func (e *CodeEditorState) status(msg string) {
	if e.OnStatus != nil {
		e.OnStatus(msg)
	}
}

// sortCompletion 按标签稳定排序（补全列表按键名分组展示）。
func sortCompletion(items []CodeCompletion) {
	sort.SliceStable(items, func(i, j int) bool {
		return strings.ToLower(items[i].Label) < strings.ToLower(items[j].Label)
	})
}

// CodeEditor 把状态构建成节点（每帧调用；回调与快捷键在此重新登记）。
//
// 用法：
//
//	st := ui.NewCodeEditorState("code-demo", text)
//	st.SetLanguage("go")
//	node := ui.CodeEditor(st).St(ui.St().GrowTo(1))
func CodeEditor(st *CodeEditorState) *Node {
	if st == nil {
		return nil
	}
	if st.MinW <= 0 {
		st.MinW = 560
	}
	if st.MinH <= 0 {
		st.MinH = 320
	}
	if st.Key == "" {
		st.Key = "code-editor"
	}
	if st.cache == nil {
		st.cache = newCodeCache()
	}
	if st.FontFamily == "" {
		st.FontFamily = codeFontFamily()
	}
	e := st
	RegisterKeyHandler(e.Key, e.HandleKey)
	RegisterTextEdit(e.Key, func(a TextEditAction) bool {
		switch a {
		case TextEditCopy:
			return e.Copy()
		case TextEditCut:
			return e.Cut()
		case TextEditPaste:
			return e.Paste()
		case TextEditSelectAll:
			e.SelectAll()
			return true
		}
		return false
	})

	// 应用后台 LSP 结果 / 待跳转位置（UI 线程内，避免竞态）
	e.drainLSP()
	e.refreshJump()

	n := &Node{Kind: KCodeEditor, Key: e.Key, Code: e}
	n.Style = Style{Opacity: 1, Focusable: true, Clip: true, NoRing: true}
	if IsFocused(e.Key) && !st.findOpen {
		// 光标闪烁是周期型装饰动画：用 AnimSlow 让 runLoop 以 8~12Hz 刷新，
		// 空闲时不会因为一个闪烁光标把整页拉回满帧重建。
		// 查找面板打开时编辑器不画光标，也不需要为闪烁重绘。
		n.Style.Animated, n.Style.AnimSlow = true, true
	}
	n.OnChar = func(r rune) { e.HandleChar(r) }
	n.OnKey = func(vk uint32) { e.HandleKey(vk) }
	// 鼠标悬停：停在标识符上弹语法提示（坐标相对节点）。
	n.OnHoverAt = func(x, y float32) { e.mouseMoveAt(x, y) }
	n.OnHover = func(h bool) {
		if !h {
			e.hoverLeave()
		}
	}
	n.OnClickAt = func(x, y float32) {
		SetFocus(e.Key)
		if e.skipClick {
			e.skipClick = false
			return
		}
		if e.diagOpen && e.DiagPanelClick(x, y) {
			RequestRedraw()
			return
		}
		if e.findOpen && e.FindPanelClick(x, y) {
			return
		}
		// 点面板外（正文 / 行号 / 小地图）：面板收起，键盘重新交给编辑器
		e.dismissFind()
		if e.menuOpen {
			if e.menuClick(x, y) {
				RequestRedraw()
				return
			}
			e.menuOpen = false
		}
		if e.compOpen {
			if e.compClick(x, y) {
				return
			}
			e.compOpen = false
		}
		e.hoverLeave()
		e.hoverOpen = false
		switch zone := e.hitZone(x, y); zone {
		case codeHitFoldMark:
			row := int((y - codeTopPad + e.ScrollY) / e.lineH())
			line, _ := e.lineOfRow(row)
			e.ToggleFold(line)
		case codeHitScrollV, codeHitScrollH, codeHitMinimap:
			// 单击轨道 / 小地图直接跳转（按住拖动在 OnDrag 里连续生效）
			e.startDrag(zone, x, y)
		case codeHitGutter:
			row := int((y - codeTopPad + e.ScrollY) / e.lineH())
			line, _ := e.lineOfRow(row)
			e.ClearCursors()
			e.SelectLine(line)
		default:
			e.TextClick(x, y, AltDown())
		}
	}
	n.OnDrag = func(x, y float32) { e.MouseDrag(x, y) }
	n.OnDragEnd = func() { e.MouseDragEnd() }
	n.OnRightClickAt = func(x, y float32) {
		SetFocus(e.Key)
		if !e.menuOpen {
			p := e.PosAt(x, y)
			if r, ok := e.Selection(); !ok || p.Before(r.Start) || p.After(r.End) {
				e.SetCaret(p, false)
			}
		}
		e.openMenu(x, y)
	}
	n.OnScroll = func(dy float32) {
		if e.menuOpen {
			e.menuOpen = false
		}
		e.scrollBy(dy)
	}
	return n
}

// codeFontFamily 编辑器等宽字体（按平台挑一个系统自带字体）。
func codeFontFamily() string {
	switch runtime.GOOS {
	case "windows":
		return "Consolas"
	case "darwin":
		return "Menlo"
	}
	return "DejaVu Sans Mono"
}
