// guis — 伪终端组件（二）：组件状态 / 键盘编码 / 选区 / 滚动
//
// 用法（桌面端直接跑本地 shell）：
//
//	term := ui.NewTermState("build-shell")
//	term.FontSize = 13
//	node := ui.Terminal(term).St(ui.St().GrowTo(1))
//
//	// 接 PTY（见 term_pty_windows.go / term_pty_unix.go）
//	if err := ui.StartShell(term, ui.ShellOptions{}); err != nil { ... }
//
// 只要渲染（Web / 远程 / 日志流）时不启 PTY，自己往 term 里喂字节即可：
//
//	term.Feed([]byte("\x1b[32mok\x1b[0m\r\n"))
package ui

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"guis/internal/gpu"
)

// TermTheme 终端配色（16 色 ANSI + 界面色）。
type TermTheme struct {
	Bg        gpu.Paint // 背景
	Fg        gpu.Paint // 默认前景
	Cursor    gpu.Paint // 光标
	CursorTxt gpu.Paint // 反显光标下的字符色
	Selection gpu.Paint // 选中底色
	Scrollbar gpu.Paint // 回看滚动条
	ANSI      [16]gpu.Paint
}

// XTermThemeStyle xterm.js 主题风格。
type XTermThemeStyle int

const (
	// XTermDefault xterm.js 默认配色（黑底 + Tango 色板，浏览器里那块终端的样子）。
	XTermDefault XTermThemeStyle = iota
	// XTermVSCode VS Code 集成终端的配色（柔和一些，适合嵌在深色界面里）。
	XTermVSCode
)

// DarkTermTheme 深色终端配色（xterm.js 默认色板）。
func DarkTermTheme() TermTheme { return TermThemeOf(XTermDefault) }

// LightTermTheme 浅色终端配色。
func LightTermTheme() TermTheme {
	th := TermThemeOf(XTermDefault)
	th.Bg = C("#ffffff")
	th.Fg = C("#1f2328")
	th.Cursor = C("#1f2328")
	th.CursorTxt = C("#ffffff")
	th.Selection = C("rgba(60,120,220,0.25)")
	th.Scrollbar = C("rgba(0,0,0,0.22)")
	th.ANSI = [16]gpu.Paint{
		C("#000000"), C("#cd0000"), C("#00cd00"), C("#cdcd00"),
		C("#0000ee"), C("#cd00cd"), C("#00cdcd"), C("#e5e5e5"),
		C("#7f7f7f"), C("#ff0000"), C("#00ff00"), C("#ffff00"),
		C("#5c5cff"), C("#ff00ff"), C("#00ffff"), C("#000000"),
	}
	return th
}

// TermThemeOf 按风格返回配色。
func TermThemeOf(style XTermThemeStyle) TermTheme {
	if style == XTermVSCode {
		return TermTheme{
			Bg:        C("#0c0c0c"),
			Fg:        C("#cccccc"),
			Cursor:    C("#ffffff"),
			CursorTxt: C("#0c0c0c"),
			Selection: C("rgba(120,170,255,0.32)"),
			Scrollbar: C("rgba(255,255,255,0.20)"),
			ANSI: [16]gpu.Paint{
				C("#1e1e1e"), C("#cd3131"), C("#0dbc79"), C("#e5e510"),
				C("#2472c8"), C("#bc3fbc"), C("#11a8cd"), C("#e5e5e5"),
				C("#666666"), C("#f14c4c"), C("#23d18b"), C("#f5f543"),
				C("#3b8eea"), C("#d670d6"), C("#29b8db"), C("#ffffff"),
			},
		}
	}
	// xterm.js 默认（canvas 渲染器的默认 theme）
	return TermTheme{
		Bg:        C("#000000"),
		Fg:        C("#ffffff"),
		Cursor:    C("#ffffff"),
		CursorTxt: C("#000000"),
		Selection: C("rgba(255,255,255,0.30)"),
		Scrollbar: C("rgba(255,255,255,0.22)"),
		ANSI: [16]gpu.Paint{
			C("#000000"), C("#cd0000"), C("#00cd00"), C("#cdcd00"),
			C("#0000ee"), C("#cd00cd"), C("#00cdcd"), C("#e5e5e5"),
			C("#7f7f7f"), C("#ff0000"), C("#00ff00"), C("#ffff00"),
			C("#5c5cff"), C("#ff00ff"), C("#00ffff"), C("#ffffff"),
		},
	}
}

// DefaultTermTheme 按当前 UI 主题自动选择深/浅终端配色。
func DefaultTermTheme() TermTheme {
	if IsDark() {
		return DarkTermTheme()
	}
	return LightTermTheme()
}

// termPaintOf 把紧凑颜色解析成绘制色。
func termPaintOf(c termColor, def gpu.Paint, th *TermTheme) gpu.Paint {
	switch {
	case c == termColorDefault:
		return def
	case c&0xFF000000 == termColorRGBBase:
		return gpu.RGBA(uint8((c>>16)&0xff), uint8((c>>8)&0xff), uint8(c&0xff), 255)
	case c&0xFF000000 == termColorIndexedBase:
		i := int(c & 0xff)
		if i < 16 {
			return th.ANSI[i]
		}
		if i >= 232 { // 灰阶
			v := uint8(8 + (i-232)*10)
			return gpu.RGBA(v, v, v, 255)
		}
		// 6×6×6 立方
		i -= 16
		r := uint8((i / 36) % 6)
		g := uint8((i / 6) % 6)
		b := uint8(i % 6)
		scale := func(v uint8) uint8 {
			if v == 0 {
				return 0
			}
			return 55 + v*40
		}
		return gpu.RGBA(scale(r), scale(g), scale(b), 255)
	}
	return def
}

// termSelection 终端选区（全局行号：最旧的回看行为 0）。
type termSelection struct {
	active bool
	drag   bool
	ax, ay int
	bx, by int
}

// valid 选区是否有内容。
func (s *termSelection) valid() bool {
	return s.active && (s.ax != s.bx || s.ay != s.by)
}

// order 返回起点在前、终点在后的两个坐标。
func (s *termSelection) order() (x0, y0, x1, y1 int) {
	if s.ay > s.by || (s.ay == s.by && s.ax > s.bx) {
		return s.bx, s.by, s.ax, s.ay
	}
	return s.ax, s.ay, s.bx, s.by
}

// shift 回看缓冲丢弃最旧行时同步平移选区。
func (s *termSelection) shift(dy int) {
	if !s.active || dy == 0 {
		return
	}
	s.ay -= dy
	s.by -= dy
	if s.ay < 0 && s.by < 0 {
		s.active, s.drag = false, false
	}
}

// TermState 终端组件状态（每个终端一个实例，可在多处渲染同一实例）。
type TermState struct {
	// Key 节点 Key（焦点 / 键盘拦截器登记，必填且全局唯一）。
	Key string
	// FontSize 字号（默认 13）。
	FontSize float32
	// FontFamily 等宽字体家族（默认按平台：Consolas / Menlo / DejaVu Sans Mono）。
	FontFamily string
	// MinW / MinH 未显式指定宽高时的最小尺寸（默认 560×320）。
	MinW, MinH float32
	// Cols / Rows 终端尺寸：AutoSize 时由控件像素尺寸推导并写回这两个字段；
	// 显式指定（AutoSize=false）时固定不变。
	Cols, Rows int
	// AutoSize 跟随控件尺寸自动改变列数/行数（默认 true）。
	AutoSize bool
	// ScrollMax 回看缓冲行数（默认 2000，0 = 不保留）。
	ScrollMax int
	// Theme 配色（零值 = 跟随当前 UI 主题）。
	Theme TermTheme
	// CursorBlink 光标是否闪烁（默认 true；仅在获得焦点时闪）。
	CursorBlink bool
	// MouseReports 允许应用程序开启鼠标上报（默认 true）。
	MouseReports bool

	// OnData 终端要写给 PTY 的字节（键盘 / 粘贴 / 应答序列）。未设置时丢弃。
	OnData func([]byte)
	// OnResize 终端尺寸变化（业务据此调用 PTY Resize）。
	OnResize func(cols, rows int)
	// OnTitle 收到 OSC 0/2 标题。
	OnTitle func(title string)
	// OnBell 收到 BEL。
	OnBell func()
	// OnExit 从右键菜单选择「关闭」时触发（业务可清理 PTY）。
	OnContextMenu func(x, y float32)

	mu        sync.Mutex
	sc        *termScreen
	// outSeq 累计收到的输出字节数（业务据此判断 PTY 是否真的有输出）。
	outSeq    atomic.Uint64
	title     string
	scrollOff int
	sel       termSelection
	hoverX    float32
	hoverY    float32
	lastClick float32
	blinkAt   float32

	viewW, viewH float32
	cellW, cellH float32
}

// NewTermState 创建终端状态（默认 80×24，回看 2000 行）。
func NewTermState(key string) *TermState {
	t := &TermState{
		Key: key, FontSize: 13, AutoSize: true, ScrollMax: 2000,
		CursorBlink: true, MouseReports: true, Cols: 80, Rows: 24,
		MinW: 560, MinH: 320,
	}
	t.ensure()
	return t
}

// ensure 懒初始化（零值 TermState 也能用）。
func (t *TermState) ensure() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureLocked()
}

func (t *TermState) ensureLocked() {
	if t.Cols < 2 {
		t.Cols = 80
	}
	if t.Rows < 2 {
		t.Rows = 24
	}
	if t.ScrollMax == 0 {
		t.ScrollMax = 2000
	}
	if t.MinW <= 0 {
		t.MinW = 560
	}
	if t.MinH <= 0 {
		t.MinH = 320
	}
	if t.FontSize <= 0 {
		t.FontSize = 13
	}
	if t.FontFamily == "" {
		t.FontFamily = codeFontFamily()
	}
	if t.sc == nil {
		t.sc = newTermScreen(t.Cols, t.Rows, t.ScrollMax)
	} else if t.sc.scrollMax != t.ScrollMax {
		// 业务可能在构造后直接改 ScrollMax 字段，这里保持一致
		t.sc.scrollMax = t.ScrollMax
		if len(t.sc.scrollBack) > t.ScrollMax {
			t.sc.scrollBack = append(t.sc.scrollBack[:0], t.sc.scrollBack[len(t.sc.scrollBack)-t.ScrollMax:]...)
		}
	}
}

// Feed 解析 PTY 输出（任意 goroutine 可调用）。
func (t *TermState) Feed(p []byte) {
	if len(p) == 0 {
		return
	}
	t.outSeq.Add(uint64(len(p)))
	t.mu.Lock()
	t.ensureLocked()
	t.sc.feed(p)
	rep := t.sc.takeReply()
	title := t.sc.takeTitle()
	bell := t.sc.takeBell()
	if d := t.sc.dropped; d > 0 {
		t.sc.dropped = 0
		t.sel.shift(d)
	}
	if title != "" {
		t.title = title
	}
	// 回看到底时跟住新输出
	if t.scrollOff > 0 && t.scrollOff > len(t.sc.scrollBack) {
		t.scrollOff = len(t.sc.scrollBack)
	}
	t.mu.Unlock()

	if len(rep) > 0 && t.OnData != nil {
		t.OnData(rep)
	}
	if title != "" && t.OnTitle != nil {
		t.OnTitle(title)
	}
	if bell && t.OnBell != nil {
		t.OnBell()
	}
	RequestRedraw()
}

// OutputCount 累计收到的输出字节数（含本地回显写入；启动前后对比可判断 PTY 是否真有输出）。
func (t *TermState) OutputCount() uint64 {
	if t == nil {
		return 0
	}
	return t.outSeq.Load()
}

// Write 实现 io.Writer（io.Copy 直接把 PTY 输出灌进终端）。
func (t *TermState) Write(p []byte) (int, error) {
	t.Feed(p)
	return len(p), nil
}

// Size 当前列数 / 行数。
func (t *TermState) Size() (cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureLocked()
	return t.sc.cols, t.sc.rows
}

// Cursor 当前光标位置（0 基）。
func (t *TermState) Cursor() (col, row int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureLocked()
	return t.sc.curX, t.sc.curY
}

// Title 当前标题（OSC 0/2）。
func (t *TermState) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.title
}

// SetScrollback 调整回看行数上限。
func (t *TermState) SetScrollback(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureLocked()
	if n < 0 {
		n = 0
	}
	t.ScrollMax = n
	t.sc.scrollMax = n
	if len(t.sc.scrollBack) > n {
		t.sc.scrollBack = append(t.sc.scrollBack[:0], t.sc.scrollBack[len(t.sc.scrollBack)-n:]...)
	}
}

// Resize 设置终端尺寸（列 × 行），变化时触发 OnResize。
func (t *TermState) Resize(cols, rows int) {
	t.applySize(cols, rows, true)
}

// applySize 内部改尺寸；notify=true 时回调业务。
func (t *TermState) applySize(cols, rows int, notify bool) {
	if cols < 2 {
		cols = 2
	}
	if rows < 2 {
		rows = 2
	}
	t.mu.Lock()
	t.ensureLocked()
	oldC, oldR := t.sc.cols, t.sc.rows
	if oldC == cols && oldR == rows {
		t.mu.Unlock()
		return
	}
	t.sc.resize(cols, rows)
	t.Cols, t.Rows = cols, rows
	if t.scrollOff > len(t.sc.scrollBack) {
		t.scrollOff = len(t.sc.scrollBack)
	}
	t.mu.Unlock()
	if notify && t.OnResize != nil {
		t.OnResize(cols, rows)
	}
}

// Text 整个屏幕（含回看）转纯文本（导出 / 断言用）。
func (t *TermState) Text() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureLocked()
	var sb strings.Builder
	for _, ln := range t.sc.scrollBack {
		sb.WriteString(termLineText(ln, t.sc.cols))
		sb.WriteByte('\n')
	}
	for y := 0; y < t.sc.rows; y++ {
		sb.WriteString(t.sc.lineText(y))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// ScreenText 仅当前屏幕（不含回看）。
func (t *TermState) ScreenText() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureLocked()
	var sb strings.Builder
	for y := 0; y < t.sc.rows; y++ {
		sb.WriteString(t.sc.lineText(y))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// termLineText 把一行回看单元格转文本。
func termLineText(ln []termCell, cols int) string {
	var sb strings.Builder
	for x := 0; x < cols && x < len(ln); x++ {
		c := ln[x]
		if c.attr&termAttrWideTail != 0 && c.ch == 0 {
			continue
		}
		if c.ch == 0 {
			sb.WriteByte(' ')
			continue
		}
		sb.WriteRune(c.ch)
	}
	return strings.TrimRight(sb.String(), " ")
}

// Clear 清屏（保留回看）。
func (t *TermState) Clear() {
	t.mu.Lock()
	t.ensureLocked()
	t.sc.eraseDisplay(2)
	t.sc.curX, t.sc.curY = 0, 0
	t.mu.Unlock()
	RequestRedraw()
}

// ClearScrollback 清空回看缓冲。
func (t *TermState) ClearScrollback() {
	t.mu.Lock()
	t.ensureLocked()
	t.sc.scrollBack = nil
	t.scrollOff = 0
	t.sel = termSelection{}
	t.mu.Unlock()
	RequestRedraw()
}

// ScrollLines 回看滚动（正数向上看历史，负数向下）。
func (t *TermState) ScrollLines(n int) {
	t.mu.Lock()
	t.ensureLocked()
	t.scrollOff += n
	max := len(t.sc.scrollBack)
	if t.scrollOff > max {
		t.scrollOff = max
	}
	if t.scrollOff < 0 {
		t.scrollOff = 0
	}
	t.mu.Unlock()
	RequestRedraw()
}

// ScrollBottom 回到最新输出。
func (t *TermState) ScrollBottom() { t.ScrollLines(-1 << 20) }

// ScrollOffset 当前回看偏移（行，0 = 贴底）。
func (t *TermState) ScrollOffsetLines() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.scrollOff
}

// SelectAll 全选屏幕内容。
func (t *TermState) SelectAll() {
	t.mu.Lock()
	t.ensureLocked()
	total := len(t.sc.scrollBack) + t.sc.rows
	t.sel = termSelection{active: true, ax: 0, ay: 0, bx: t.sc.cols - 1, by: total - 1}
	t.mu.Unlock()
	RequestRedraw()
}

// ClearSelection 取消选区。
func (t *TermState) ClearSelection() {
	t.mu.Lock()
	t.sel = termSelection{}
	t.mu.Unlock()
	RequestRedraw()
}

// SelectionText 当前选中的文本。
func (t *TermState) SelectionText() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureLocked()
	if !t.sel.valid() {
		return ""
	}
	x0, y0, x1, y1 := t.sel.order()
	back := len(t.sc.scrollBack)
	lineOf := func(y int) []termCell {
		if y < back {
			return t.sc.scrollBack[y]
		}
		row := y - back
		if row < 0 || row >= t.sc.rows {
			return nil
		}
		return t.sc.row(row)
	}
	var sb strings.Builder
	for y := y0; y <= y1; y++ {
		ln := lineOf(y)
		if ln == nil {
			continue
		}
		start, end := 0, t.sc.cols
		if y == y0 {
			start = x0
		}
		if y == y1 {
			end = x1 + 1
		}
		if start < 0 {
			start = 0
		}
		if end > t.sc.cols {
			end = t.sc.cols
		}
		var line strings.Builder
		for x := start; x < end && x < len(ln); x++ {
			c := ln[x]
			if c.attr&termAttrWideTail != 0 && c.ch == 0 {
				continue
			}
			if c.ch == 0 {
				line.WriteByte(' ')
				continue
			}
			line.WriteRune(c.ch)
		}
		sb.WriteString(strings.TrimRight(line.String(), " "))
		if y < y1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// CopySelection 复制选区到剪贴板。
func (t *TermState) CopySelection() bool {
	s := t.SelectionText()
	if s == "" {
		return false
	}
	return ClipboardSetText(s)
}

// Paste 粘贴剪贴板内容到终端（支持括号粘贴模式）。
func (t *TermState) Paste() bool {
	s := ClipboardText()
	if s == "" {
		return false
	}
	// 统一换行为 CR（终端 Enter 语义）
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", "\r")
	t.mu.Lock()
	bracket := t.sc != nil && t.sc.brPaste
	t.mu.Unlock()
	if bracket {
		s = "\x1b[200~" + s + "\x1b[201~"
	}
	t.send([]byte(s))
	return true
}

// send 把字节交给 OnData（没有 PTY 时忽略）。
func (t *TermState) send(p []byte) {
	if len(p) == 0 || t.OnData == nil {
		return
	}
	buf := make([]byte, len(p))
	copy(buf, p)
	t.OnData(buf)
}

// ---- 键盘 ----

// HandleKey 处理按键（由节点 OnKey / RegisterKeyHandler 调用），返回是否已消费。
func (t *TermState) HandleKey(vk uint32) bool {
	ctrl, shift, alt := CtrlDown(), ShiftDown(), AltDown()

	// Ctrl+Shift+C / Ctrl+Shift+V：复制 / 粘贴（xterm 习惯，不打扰 Ctrl+C=SIGINT）
	if ctrl && shift {
		switch vk {
		case 'C', 'c':
			if t.CopySelection() {
				return true
			}
			return false
		case 'V', 'v':
			t.Paste()
			return true
		}
	}

	t.mu.Lock()
	ensureTermLocked(t)
	appCur := t.sc.appCur
	t.mu.Unlock()

	app := func(cursor string) string {
		if appCur {
			return "\x1bO" + cursor
		}
		return "\x1b[" + cursor
	}
	mod := 1
	if shift {
		mod += 1
	}
	if alt {
		mod += 2
	}
	if ctrl {
		mod += 4
	}
	csiMod := func(final byte) []byte {
		if mod <= 1 {
			return []byte{0x1b, '[', final}
		}
		return []byte("\x1b[1;" + strconv.Itoa(mod) + string(final))
	}

	// 历史回看快捷键：Shift+PageUp / Shift+PageDown / Ctrl+Shift+Home/End
	if shift && (vk == 0x21 /* VK_PRIOR */ || vk == 0x22 /* VK_NEXT */) {
		if vk == 0x21 {
			t.ScrollLines(maxInt(t.rowsOnScreen()/2, 1))
		} else {
			t.ScrollLines(-maxInt(t.rowsOnScreen()/2, 1))
		}
		return true
	}

	switch vk {
	case VK_RETURN:
		if alt {
			t.send([]byte("\x1b\r"))
		} else {
			t.send([]byte("\r"))
		}
		return true
	case VK_BACK:
		if alt {
			t.send([]byte("\x1b\x7f"))
		} else {
			t.send([]byte{0x7f})
		}
		return true
	case VK_TAB:
		if shift {
			t.send([]byte("\x1b[Z"))
		} else {
			t.send([]byte("\t"))
		}
		return true
	case VK_ESCAPE:
		t.send([]byte{0x1b})
		return true
	case VK_LEFT:
		if mod > 1 {
			t.send(csiMod('D'))
		} else {
			t.send([]byte(app("D")))
		}
		return true
	case VK_RIGHT:
		if mod > 1 {
			t.send(csiMod('C'))
		} else {
			t.send([]byte(app("C")))
		}
		return true
	case VK_UP:
		if mod > 1 {
			t.send(csiMod('A'))
		} else {
			t.send([]byte(app("A")))
		}
		return true
	case VK_DOWN:
		if mod > 1 {
			t.send(csiMod('B'))
		} else {
			t.send([]byte(app("B")))
		}
		return true
	case 0x24: // VK_HOME
		if mod > 1 {
			t.send(csiMod('H'))
		} else {
			t.send([]byte(app("H")))
		}
		return true
	case 0x23: // VK_END
		if mod > 1 {
			t.send(csiMod('F'))
		} else {
			t.send([]byte(app("F")))
		}
		return true
	case 0x2D: // VK_INSERT
		t.send(csiMod('~'))
		return true
	case VK_DELETE:
		if mod > 1 {
			t.send([]byte("\x1b[3;" + strconv.Itoa(mod) + "~"))
		} else {
			t.send([]byte("\x1b[3~"))
		}
		return true
	case 0x21: // VK_PRIOR
		t.send([]byte("\x1b[5~"))
		return true
	case 0x22: // VK_NEXT
		t.send([]byte("\x1b[6~"))
		return true
	}

	// 功能键 F1..F12
	if vk >= 0x70 && vk <= 0x7B {
		n := int(vk - 0x6F)
		seq := []string{
			"\x1bOP", "\x1bOQ", "\x1bOR", "\x1bOS",
			"\x1b[15~", "\x1b[17~", "\x1b[18~", "\x1b[19~",
			"\x1b[20~", "\x1b[21~", "\x1b[23~", "\x1b[24~",
		}
		t.send([]byte(seq[n-1]))
		return true
	}

	// Ctrl+字母 → 0x01..0x1A（在有焦点时优先于可打印字符）
	if ctrl && !alt {
		if vk >= 'A' && vk <= 'Z' {
			t.send([]byte{byte(vk - 'A' + 1)})
			return true
		}
		if vk >= 'a' && vk <= 'z' {
			t.send([]byte{byte(vk - 'a' + 1)})
			return true
		}
		switch vk {
		case 0x20: // Ctrl+Space → NUL
			t.send([]byte{0})
			return true
		case 0xBB, 0xBD, 0x6B: // Ctrl+= / Ctrl+- / Ctrl+Shift+=
			t.send([]byte{0x1f})
			return true
		}
	}
	// Alt+字母 → ESC 前缀
	if alt && !ctrl {
		if vk >= 'A' && vk <= 'Z' {
			t.send([]byte{0x1b, byte(vk)})
			return true
		}
		if vk >= 'a' && vk <= 'z' {
			t.send([]byte{0x1b, byte(vk)})
			return true
		}
	}
	// 注意：可打印字符一律不在 HandleKey 里发送 —— 框架会把同一按键再发一次 EvChar
	// （OnChar），两处都发就会变成「每敲一个字母终端收到两份」，回显重复、命令行错乱。
	// 这里只消费控制键，其余交给 HandleChar。
	return false
}

// HandleChar 处理字符输入（OnChar）。
func (t *TermState) HandleChar(r rune) {
	if r == 0 || r == '\r' || r == '\n' || r == '\t' {
		return // 由 HandleKey 负责，避免重复
	}
	t.send([]byte(string(r)))
}

// rowsOnScreen 当前可见行数。
func (t *TermState) rowsOnScreen() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sc == nil {
		return t.Rows
	}
	return t.sc.rows
}

// ---- 鼠标 ----

// clickAtPos 单击定位（节点内坐标）：选择 / 焦点 / 鼠标上报。
func (t *TermState) clickAtPos(x, y float32, press bool) {
	t.mu.Lock()
	ensureTermLocked(t)
	cw, ch := t.cellW, t.cellH
	if cw <= 0 || ch <= 0 {
		t.mu.Unlock()
		return
	}
	t.hoverX, t.hoverY = x, y
	col := int(x / cw)
	row := int(y / ch)
	if col < 0 {
		col = 0
	}
	if col >= t.sc.cols {
		col = t.sc.cols - 1
	}
	if row < 0 {
		row = 0
	}
	if row >= t.sc.rows {
		row = t.sc.rows - 1
	}
	gy := len(t.sc.scrollBack) - t.scrollOff + row
	mouseOn := t.sc.mouse != 0 && t.MouseReports

	if press {
		t.sel = termSelection{active: true, drag: true, ax: col, ay: gy, bx: col, by: gy}
	}
	t.mu.Unlock()

	if mouseOn {
		t.reportMouse(0, col, row, press, false)
	}
	if press {
		RequestRedraw()
	}
}

// dragTo 拖动选择（节点内坐标）。
func (t *TermState) dragTo(x, y float32) {
	t.mu.Lock()
	ensureTermLocked(t)
	cw, ch := t.cellW, t.cellH
	if cw <= 0 || ch <= 0 || !t.sel.drag {
		t.mu.Unlock()
		return
	}
	col := int(x / cw)
	row := int(y / ch)
	if col < 0 {
		col = 0
	}
	if col >= t.sc.cols {
		col = t.sc.cols - 1
	}
	if row < 0 {
		row = 0
	}
	if row >= t.sc.rows {
		row = t.sc.rows - 1
	}
	t.sel.bx = col
	t.sel.by = len(t.sc.scrollBack) - t.scrollOff + row
	t.mu.Unlock()
	RequestRedraw()
}

// endDrag 结束拖选。
func (t *TermState) endDrag() {
	t.mu.Lock()
	t.sel.drag = false
	t.mu.Unlock()
}

// wheel 滚轮回看（dy 为像素，正 = 看更早的内容）。
func (t *TermState) wheel(dy float32) {
	t.mu.Lock()
	ch := t.cellH
	if ch <= 0 {
		ch = 16
	}
	lines := int(dy / ch)
	if lines == 0 && dy != 0 {
		if dy > 0 {
			lines = 1
		} else {
			lines = -1
		}
	}
	// 应用开启鼠标模式时，滚轮交给应用（vim / less）
	mouseOn := t.sc.mouse != 0 && t.MouseReports
	rows := t.sc.rows
	cols := t.sc.cols
	t.mu.Unlock()

	if mouseOn {
		btn := 64
		if lines < 0 {
			btn = 65
		}
		n := lines
		if n < 0 {
			n = -n
		}
		for i := 0; i < n; i++ {
			t.reportMouse(btn, cols/2, rows/2, true, true)
		}
		return
	}
	t.ScrollLines(lines)
}

// reportMouse 发送 SGR 鼠标序列（1006）。
func (t *TermState) reportMouse(btn, col, row int, press bool, wheel bool) {
	t.mu.Lock()
	sgr := t.sc.mouseSGR
	if wheel {
		// 滚轮只发按下形式
		press = true
	}
	t.mu.Unlock()
	if sgr {
		final := "M"
		if !press {
			final = "m"
		}
		t.send([]byte("\x1b[<" + strconv.Itoa(btn) + ";" + strconv.Itoa(col+1) + ";" + strconv.Itoa(row+1) + final))
		return
	}
	if !press || wheel {
		return
	}
	// 兼容 X10：ESC [ M b x y
	b := byte(32 + btn)
	x := byte(32 + col + 1)
	y := byte(32 + row + 1)
	t.send([]byte{0x1b, '[', 'M', b, x, y})
}

// ensureTermLocked 零值保护（调用方已持锁）。
func ensureTermLocked(t *TermState) {
	if t.sc == nil {
		t.ensureLocked()
	}
}

// maxInt 两整数较大值。
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Terminal 把状态构建成节点（每帧调用；键盘拦截器在此重新登记）。
func Terminal(st *TermState) *Node {
	if st == nil {
		return nil
	}
	st.ensure()
	if st.Key == "" {
		st.Key = "terminal"
	}
	RegisterKeyHandler(st.Key, st.HandleKey)

	n := &Node{Kind: KTerm, Key: st.Key, Term: st}
	n.Style = Style{Opacity: 1, Focusable: true, Clip: true, NoRing: true}
	if IsFocused(st.Key) && st.CursorBlink {
		// 光标闪烁：低帧率周期动画，空闲时不会把整页拉回满帧。
		n.Style.Animated, n.Style.AnimSlow = true, true
	}
	// 提示：这里不设 OnKey —— 键盘统一走 RegisterKeyHandler，两者都设会让每个按键
	// 被处理两次（终端里表现为「敲一个字母出现两个」）。
	n.OnChar = st.HandleChar
	n.OnScroll = func(dy float32) { st.wheel(dy) }
	n.OnHoverAt = func(x, y float32) {
		st.mu.Lock()
		st.hoverX, st.hoverY = x, y
		st.mu.Unlock()
	}
	n.OnClickAt = func(x, y float32) {
		SetFocus(st.Key)
		st.clickAtPos(x, y, true)
		st.endDrag()
	}
	n.OnDrag = func(x, y float32) { st.dragTo(x, y) }
	n.OnDragEnd = func() { st.endDrag() }
	n.OnRightClickAt = func(x, y float32) {
		SetFocus(st.Key)
		if st.OnContextMenu != nil {
			st.OnContextMenu(x, y)
		}
	}
	return n
}

// TermCursorVisible 测试/绘制用：光标此刻是否显示（闪烁相位）。
func (t *TermState) cursorShown(now float32, focused bool) bool {
	t.mu.Lock()
	vis := t.sc != nil && t.sc.cursorVisible
	t.mu.Unlock()
	if !vis || !focused {
		return false
	}
	if !t.CursorBlink {
		return true
	}
	ph := now - t.blinkAt
	if ph < 0 {
		ph = 0
	}
	return int(ph*1.8)%2 == 0
}
