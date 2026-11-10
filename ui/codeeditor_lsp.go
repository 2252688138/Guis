// guis — 原生代码编辑器（六）：LSP 接入 + 内置诊断
//
// 线程模型：LSP 的读取协程只把结果压进 pending 队列，真正的状态修改发生在
// 每帧构建 CodeEditor() 时的 drainLSP（UI 线程），因此编辑器状态始终单线程访问，
// 不需要为每个字段加锁，也不会出现「画画到一半被后台协程改数组」的崩溃。
package ui

import (
	"fmt"
	"strings"
	"sync"
)

// lspKind 后台结果类型。
type lspKind uint8

const (
	lspCompletion lspKind = iota + 1
	lspHover
	lspDiagnostics
	lspFormatted
	lspRename
	lspStatus
	lspSemantic
	// lspLogLine 服务端日志（只进日志缓冲，不进状态栏）。
	lspLogLine
	// lspError 连接层错误（进程退出 / 读取失败），进日志并提示状态栏。
	lspError
	// lspAttach 绑定 / 解绑语言服务器（保证在 UI 线程执行）。
	lspAttach
	// lspSemanticOn 开关语义高亮（保证在 UI 线程执行）。
	lspSemanticOn
	// lspHoverDue 鼠标悬停去抖到期（seq 为本次悬停请求序号）。
	lspHoverDue
)

// codeLSPLogMax 日志缓冲保留条数。
const codeLSPLogMax = 60

// codeSemanticRetries 语义 token 空结果的退避重试次数：
// 服务端刚启动时 workspace 还没 type-check 完，semanticTokens/full 会返回空数组。
const codeSemanticRetries = 8

// lspPending 一条待应用的后台结果。
type lspPending struct {
	kind    lspKind
	items   []CodeCompletion
	lines   []string
	diags   []CodeDiagnostic
	text    string
	msg     string
	// md 悬停服务端返回的 Markdown（lspHover）。
	md      string
	rename  string
	semToks []codeSemToken
	// seq 语义 token 的请求序号（丢弃过期结果）。
	seq uint64
	// srv 绑定 / 解绑目标（lspAttach）。
	srv *LSPClient
	// on 语义高亮开关（lspSemanticOn）。
	on bool
}

var (
	codePendingMu sync.Mutex
	codePending   []lspPending
)

func pushLSP(p lspPending) {
	codePendingMu.Lock()
	codePending = append(codePending, p)
	if len(codePending) > 32 {
		codePending = codePending[len(codePending)-32:]
	}
	codePendingMu.Unlock()
	RequestRedraw()
}

// drainLSP 在 UI 线程应用后台结果（CodeEditor 每帧调用）。
func (e *CodeEditorState) drainLSP() {
	codePendingMu.Lock()
	list := codePending
	codePending = nil
	codePendingMu.Unlock()
	for _, p := range list {
		switch p.kind {
		case lspCompletion:
			if len(p.items) > 0 {
				e.compItems = p.items
				e.compSel = 0
				e.compOpen = true
				e.positionCompletion()
			}
		case lspHover:
			// 鼠标已经移开 / 提示已被收起的过期结果直接丢弃（否则会在鼠标不在时弹出面板）
			if !e.hoverPinned {
				continue
			}
			if strings.TrimSpace(p.md) == "" {
				continue // 服务端没给内容：保留本地占位
			}
			e.setHoverMarkdown(p.md)
		case lspDiagnostics:
			e.lspDiags = p.diags
		case lspFormatted:
			if p.text != "" {
				e.applyLSPText(p.text, p.msg)
			}
		case lspRename:
			if p.text != "" {
				e.applyLSPText(p.text, p.msg)
			}
		case lspStatus:
			e.status(p.msg)
		case lspSemantic:
			e.semanticBusy = false
			if p.seq != 0 && p.seq != e.semanticSeq {
				continue
			}
			// 空结果不覆盖已有着色（只在拿到 token 时替换，避免闪烁）
			if !e.semanticOn {
				continue
			}
			if len(p.semToks) == 0 {
				e.semanticEmpty = len(e.semantic) == 0
				RequestRedraw()
				continue
			}
			e.semanticEmpty = false
			e.applySemantic(p.semToks)
		case lspLogLine:
			e.appendLSPLog(p.msg)
		case lspError:
			e.appendLSPLog("[错误] " + p.msg)
			e.status(p.msg)
		case lspAttach:
			e.attachLSPNow(p.srv)
		case lspSemanticOn:
			e.setSemanticNow(p.on)
		case lspHoverDue:
			e.hoverDue(p.seq)
		}
	}
}

// positionCompletion 把补全面板定位到光标下方（相对节点）。
func (e *CodeEditorState) positionCompletion() {
	if _, ok := LastFrameOf(e.Key); !ok {
		return
	}
	x := e.gutterTotal() - e.ScrollX + e.xOfCol(e.Pos.Line, e.Pos.Col)
	y := codeTopPad + float32(e.Pos.Line+1)*e.lineH() - e.ScrollY
	if x < 0 {
		x = 0
	}
	e.compX = x
	e.compY = y
}

// AttachLSP 绑定语言服务器（传 nil 解绑并回退内置解析器）。
//
// 可在任意协程调用：真正的状态修改会投递到 UI 线程（drainLSP 里执行），
// 避免后台协程（例如启动服务后回调）直接改编辑器字段造成竞态。
func (e *CodeEditorState) AttachLSP(c *LSPClient) {
	pushLSP(lspPending{kind: lspAttach, srv: c})
}

// attachLSPNow 真正的绑定（只在 UI 线程调用）。
func (e *CodeEditorState) attachLSPNow(c *LSPClient) {
	if e.srv == c {
		return
	}
	e.srv = c
	e.lspDiags = nil
	if c == nil {
		e.semantic = nil
		e.semanticStamp++
		e.refreshBuiltinDiags()
		e.status("已断开 LSP，使用内置解析器")
		RequestRedraw()
		return
	}
	c.OnDiagnostics = func(uri string, ds []CodeDiagnostic) {
		if e.Path != "" && FileURI(e.Path) != uri {
			return
		}
		pushLSP(lspPending{kind: lspDiagnostics, diags: ds})
	}
	c.OnLog = func(s string) { pushLSP(lspPending{kind: lspLogLine, msg: s}) }
	c.OnError = func(s string) { pushLSP(lspPending{kind: lspError, msg: s}) }
	if e.Path != "" {
		c.DidOpen(FileURI(e.Path), codeLSPLanguageID(e.Language), e.Text())
	}
	if e.semanticOn {
		e.RequestSemantic()
	}
	e.status("已连接 " + c.Name)
	RequestRedraw()
}

// SetStatus 设置状态栏文案（可在后台协程调用：投递到 UI 线程，下一帧生效）。
func (e *CodeEditorState) SetStatus(msg string) {
	pushLSP(lspPending{kind: lspStatus, msg: msg})
}

// LSP 当前绑定的服务（未接为 nil）。
func (e *CodeEditorState) LSP() *LSPClient { return e.srv }

// LSPStatus 一行连接状态（页面顶部展示）。
func (e *CodeEditorState) LSPStatus() string {
	if e.srv == nil {
		n := len(e.builtinDiags)
		return fmt.Sprintf("内置解析器 · %s · %d 条诊断", e.Language, n)
	}
	if !e.srv.Started() {
		return e.srv.Name + " · 未连接"
	}
	return fmt.Sprintf("%s · 已连接 · %d 条诊断", e.srv.Name, len(e.lspDiags))
}

// appendLSPLog 追加一条服务端日志（多行压成一行，超出上限丢掉最旧的）。
//
// 语言服务器（尤其 gopls）的日志是多行、且一行几百字符，直接写状态栏会把
// 底栏撑成一大块文本；这里只进缓冲，需要展示时用 LSPLog()。
func (e *CodeEditorState) appendLSPLog(msg string) {
	line := strings.Join(strings.Fields(msg), " ")
	if line == "" {
		return
	}
	if rs := []rune(line); len(rs) > 300 {
		line = string(rs[:300]) + "…"
	}
	e.lspLog = append(e.lspLog, line)
	if n := len(e.lspLog); n > codeLSPLogMax {
		e.lspLog = append([]string(nil), e.lspLog[n-codeLSPLogMax:]...)
	}
	RequestRedraw()
}

// LSPLog 最近的服务端日志（旧 → 新；页面可做成日志面板）。
func (e *CodeEditorState) LSPLog() []string {
	return append([]string(nil), e.lspLog...)
}

// codeLSPLanguageID 编辑器语言 → LSP languageId。
func codeLSPLanguageID(lang string) string {
	switch lang {
	case "javascript":
		return "javascript"
	case "typescript":
		return "typescript"
	case "c":
		return "c"
	case "csharp":
		return "csharp"
	}
	return lang
}

// lspSync 编辑后同步（全文变更；未接 LSP 时刷新内置诊断）。
func (e *CodeEditorState) lspSync() {
	if e.srv == nil || e.Path == "" || !e.srv.Started() {
		e.refreshBuiltinDiags()
		return
	}
	e.srv.DidChange(FileURI(e.Path), e.Text())
	// 语义高亮的节流放在 RequestSemantic 里（同一时间只允一个在飞请求 + 退避重试），
	// 这里每次编辑通知一次，服务端重算完会回带新 token。
	if e.semanticOn {
		e.RequestSemantic()
	}
}

// SaveDocument 保存（通知 LSP + 触发 OnSave 回调）。
func (e *CodeEditorState) SaveDocument() {
	text := e.Text()
	if e.srv != nil && e.Path != "" && e.srv.Started() {
		e.srv.DidSave(FileURI(e.Path), text)
	}
	e.Dirty = false
	if e.OnSave != nil {
		e.OnSave(text)
	}
	e.status("已保存")
}

// RequestCompletion 触发补全（LSP 异步 / 内置同步）。
func (e *CodeEditorState) RequestCompletion() {
	if e.ReadOnly {
		return
	}
	if e.srv == nil || e.Path == "" || !e.srv.Started() {
		e.openBuiltinCompletion()
		return
	}
	e.openBuiltinCompletion() // 先给本地结果，后台再覆盖（响应快）
	srv := e.srv
	uri := FileURI(e.Path)
	lang := codeLSPLanguageID(e.Language)
	line, col := e.Pos.Line, e.Pos.Col
	text := e.Text()
	go func() {
		items := srv.Completion(uri, lang, line, col, text)
		if len(items) > 0 {
			pushLSP(lspPending{kind: lspCompletion, items: items})
		}
	}()
}

// RequestHover 悬停信息（F1 / 右键菜单）：作用在光标所在符号上。
func (e *CodeEditorState) RequestHover() {
	e.hoverPinned = true
	e.requestHoverAt(e.Pos.Line, e.Pos.Col, -1, -1)
}

// requestHoverAt 在文档位置 line/col 取词并弹出提示。
//
// mx/my 为节点内鼠标坐标：>=0 时面板贴在鼠标右下方（鼠标悬停），
// 否则贴在光标右下方（F1 / 右键菜单）。
func (e *CodeEditorState) requestHoverAt(line, col int, mx, my float32) {
	e.hoverPinned = true // 标记提示由本次请求弹出（异步结果回来时据此判断还要不要显示）
	// 本地先给出「当前词 + 本行诊断」
	word := e.wordAt(line, col)
	lines := []string{}
	if word != "" {
		lines = append(lines, word)
	}
	for _, d := range e.allDiagnostics() {
		if d.Line == line {
			lines = append(lines, "["+diagSeverityName(d.Severity)+"] "+d.Message)
		}
	}
	if len(lines) > 0 {
		e.setHoverPlainLines(lines)
	}
	if mx >= 0 {
		e.positionHoverAt(mx, my)
	} else {
		e.positionHover()
	}
	if e.srv == nil || e.Path == "" || !e.srv.Started() {
		if len(lines) == 0 {
			e.status("光标处没有可显示的信息")
		}
		return
	}
	srv := e.srv
	uri := FileURI(e.Path)
	lang := codeLSPLanguageID(e.Language)
	text := e.Text()
	go func() {
		md := srv.HoverMarkdown(uri, lang, line, col, text)
		if strings.TrimSpace(md) == "" {
			return
		}
		pushLSP(lspPending{kind: lspHover, md: md})
	}()
}

// positionHover 悬停面板定位到光标右下方（相对节点）。
func (e *CodeEditorState) positionHover() {
	x := e.gutterTotal() - e.ScrollX + e.xOfCol(e.Pos.Line, e.Pos.Col) + 16
	y := codeTopPad + float32(e.Pos.Line+1)*e.lineH() - e.ScrollY + 6
	e.placeHover(x, y)
}

// positionHoverAt 悬停面板定位到鼠标位置右下方（相对节点）。
func (e *CodeEditorState) positionHoverAt(mx, my float32) {
	e.placeHover(mx+14, my+18)
}

// placeHover 把面板框在可视区域内并置为打开。
func (e *CodeEditorState) placeHover(x, y float32) {
	if x < 0 {
		x = 0
	}
	rect, ok := LastFrameOf(e.Key)
	if ok {
		if x > rect.W-380 {
			x = rect.W - 400
		}
		if x < 8 {
			x = 8
		}
		if y > rect.H-120 {
			y = rect.H - 160
		}
	}
	e.hoverX, e.hoverY = x, y
	e.hoverOpen = true
	RequestRedraw()
}

// RequestDefinition 转到定义（F12）。
func (e *CodeEditorState) RequestDefinition() {
	if e.srv == nil || e.Path == "" || !e.srv.Started() {
		e.status("转到定义需要语言服务器（gopls / tsserver 等）")
		return
	}
	srv := e.srv
	uri := FileURI(e.Path)
	lang := codeLSPLanguageID(e.Language)
	line, col := e.Pos.Line, e.Pos.Col
	text := e.Text()
	e.status("正在查找定义…")
	go func() {
		locs := srv.Definition(uri, lang, line, col, text)
		if len(locs) == 0 {
			pushLSP(lspPending{kind: lspStatus, msg: "未找到定义"})
			return
		}
		// 多文件跳转：当前文件内直接定位；跳转行由 refreshJump 在 UI 线程应用
		loc := locs[0]
		if loc.Path != "" && FileURI(loc.Path) != uri {
			pushLSP(lspPending{kind: lspStatus,
				msg: fmt.Sprintf("定义在 %s:%d:%d", loc.Path, loc.Line+1, loc.Col+1)})
			return
		}
		codeJumpLine = loc.Line
		codeJumpCol = loc.Col
		pushLSP(lspPending{kind: lspStatus, msg: fmt.Sprintf("已定位到第 %d 行", loc.Line+1)})
		RequestRedraw()
	}()
}

// codeJumpLine / codeJumpCol 待跳转位置（drainLSP 之后由 refreshJump 应用）。
var (
	codeJumpLine = -1
	codeJumpCol  = -1
)

// refreshJump 应用后台请求的跳转（CodeEditor 每帧调用）。
func (e *CodeEditorState) refreshJump() {
	if codeJumpLine < 0 {
		return
	}
	e.SetCaret(CodePos{Line: codeJumpLine, Col: codeJumpCol}, false)
	e.ensureCaretVisible()
	codeJumpLine, codeJumpCol = -1, -1
}

// RequestRename 打开重命名输入条（F2）。
func (e *CodeEditorState) RequestRename() {
	word := e.wordAt(e.Pos.Line, e.Pos.Col)
	if word == "" {
		e.status("光标不在符号上")
		return
	}
	e.renameWord = word
	e.findOpen = true
	e.findMode = 2
	e.findText = word
	e.replaceFocus = 0
	// 插入点放到末尾并全选：直接输入即替换整个符号（回车不改则保持原名）
	e.findCaret = runeLen(word)
	e.findSelAll = true
	RequestRedraw()
}

// applyRename 提交重命名：有 LSP 走 workspace/rename，否则只替换当前行同词。
func (e *CodeEditorState) applyRename(oldWord, newName string) {
	if oldWord == "" || newName == "" || oldWord == newName {
		return
	}
	if e.srv != nil && e.Path != "" && e.srv.Started() {
		srv := e.srv
		uri := FileURI(e.Path)
		lang := codeLSPLanguageID(e.Language)
		line, col := e.Pos.Line, e.Pos.Col
		text := e.Text()
		e.status("正在重命名…")
		go func() {
			changes, ok := srv.Rename(uri, lang, line, col, text, newName)
			if !ok {
				pushLSP(lspPending{kind: lspStatus, msg: "重命名失败（服务端无改动）"})
				return
			}
			if out, ok := changes[uri]; ok && out != "" {
				pushLSP(lspPending{kind: lspRename, text: out, msg: "已重命名为 " + newName})
			}
		}()
		return
	}
	// 内置：只替换光标所在行的整词（范围可控）
	i := e.Pos.Line
	if i >= len(e.Lines) {
		return
	}
	line := e.Lines[i]
	var sb strings.Builder
	off := 0
	count := 0
	for off < len(line) {
		idx := strings.Index(line[off:], oldWord)
		if idx < 0 {
			break
		}
		b := off + idx
		okLeft := b == 0 || !isIdentRune(rune(line[b-1]))
		okRight := b+len(oldWord) >= len(line) || !isIdentRune(rune(line[b+len(oldWord)]))
		if okLeft && okRight {
			sb.WriteString(line[off:b])
			sb.WriteString(newName)
			count++
			off = b + len(oldWord)
			continue
		}
		sb.WriteString(line[off : b+len(oldWord)])
		off = b + len(oldWord)
	}
	sb.WriteString(line[off:])
	if count == 0 {
		return
	}
	newLine := sb.String()
	e.replaceRange(CodePos{Line: i, Col: 0}, CodePos{Line: i, Col: len([]rune(line))}, newLine, true)
	e.status(fmt.Sprintf("本行替换 %d 处", count))
}

// wordAt 取指定位置的完整单词（不在单词上返回空串）。
func (e *CodeEditorState) wordAt(line, col int) string {
	runes := []rune(e.line(line))
	if len(runes) == 0 {
		return ""
	}
	c := codeClamp(col, 0, len(runes)-1)
	if !codeWordRune(runes[c]) {
		return ""
	}
	lo, hi := c, c+1
	for lo > 0 && codeWordRune(runes[lo-1]) {
		lo--
	}
	for hi < len(runes) && codeWordRune(runes[hi]) {
		hi++
	}
	return string(runes[lo:hi])
}

// SetDiagnostics 外部注入诊断（业务侧把编译错误塞进编辑器）。
func (e *CodeEditorState) SetDiagnostics(ds []CodeDiagnostic) {
	e.lspDiags = ds
	RequestRedraw()
}

// Diagnostics 当前全部诊断（内置 + LSP）。
func (e *CodeEditorState) Diagnostics() []CodeDiagnostic { return e.allDiagnostics() }

// GoToLine 跳到指定行（1 基，滚动到视口中间）。
func (e *CodeEditorState) GoToLine(line int) {
	e.SetCaretLine(line)
	e.ScrollY = float32(line-1)*e.lineH() - e.viewH/2 + e.lineH()
	e.clampScroll()
	RequestRedraw()
}

// ---- 内置诊断（括号配对，不依赖语言服务器）----

// refreshBuiltinDiags 扫描光标附近 ±120 行做括号配对检查，
// 十万行文档也不会因为「检查全文」而卡住输入。
func (e *CodeEditorState) refreshBuiltinDiags() {
	if len(e.Lines) > 5000 {
		// 超大文档只检查可见范围，避免每次输入都全量扫描
		first, last := e.viewState(e.viewH)
		e.builtinDiags = checkBrackets(e.Lines, first-40, last+40)
		return
	}
	e.builtinDiags = checkBrackets(e.Lines, 0, len(e.Lines))
}

// checkBrackets 括号配对诊断（剥掉字符串与注释后判断）。
func checkBrackets(lines []string, from, to int) []CodeDiagnostic {
	if from < 0 {
		from = 0
	}
	if to > len(lines) {
		to = len(lines)
	}
	var ds []CodeDiagnostic
	stack := make([]rune, 0, 32)
	stackLine := make([]int, 0, 32)
	stackCol := make([]int, 0, 32)
	for i := from; i < to; i++ {
		code := stripCodeStrings(lines[i])
		col := 0
		for _, r := range code {
			switch r {
			case '{', '(', '[':
				stack = append(stack, r)
				stackLine = append(stackLine, i)
				stackCol = append(stackCol, col)
			case '}', ')', ']':
				if len(stack) == 0 {
					ds = append(ds, CodeDiagnostic{Line: i, Col: col, EndLine: i, EndCol: col + 1,
						Severity: 1, Message: "多余的右括号", Source: "内置解析器"})
					break
				}
				open := stack[len(stack)-1]
				if !codeBracketsMatch(open, r) {
					ds = append(ds, CodeDiagnostic{Line: i, Col: col, EndLine: i, EndCol: col + 1,
						Severity: 1, Message: fmt.Sprintf("%c 与 %c 不匹配", open, r), Source: "内置解析器"})
					stack = stack[:len(stack)-1]
					stackLine = stackLine[:len(stackLine)-1]
					stackCol = stackCol[:len(stackCol)-1]
					break
				}
				stack = stack[:len(stack)-1]
				stackLine = stackLine[:len(stackLine)-1]
				stackCol = stackCol[:len(stackCol)-1]
			}
			col++
		}
	}
	for i := range stack {
		ds = append(ds, CodeDiagnostic{Line: stackLine[i], Col: stackCol[i],
			EndLine: stackLine[i], EndCol: stackCol[i] + 1,
			Severity: 2, Message: fmt.Sprintf("未闭合的 %c", stack[i]), Source: "内置解析器"})
	}
	if len(ds) > 50 {
		ds = ds[:50]
	}
	return ds
}

func codeBracketsMatch(open, close rune) bool {
	return (open == '{' && close == '}') || (open == '(' && close == ')') || (open == '[' && close == ']')
}

// stripCodeStrings 去掉字符串 / 行注释，供括号检查用。
func stripCodeStrings(line string) string {
	var sb strings.Builder
	var quote rune
	esc := false
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
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
			continue
		case '/':
			if i+1 < len(rs) && rs[i+1] == '/' {
				return sb.String()
			}
		case '#':
			if i == 0 || strings.TrimSpace(string(rs[:i])) == "" {
				return sb.String()
			}
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
