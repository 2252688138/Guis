package ui

import (
	"strings"
	"testing"
)

// newTestTerm 建一个固定尺寸的终端（关掉 AutoSize，避免像素推导干扰）。
func newTestTerm(cols, rows, back int) *TermState {
	t := NewTermState("test-term")
	t.AutoSize = false
	t.ScrollMax = back
	t.applySize(cols, rows, false)
	return t
}

func TestTermBasicWrite(t *testing.T) {
	ts := newTestTerm(20, 4, 100)
	ts.Feed([]byte("hello\r\nworld"))
	lines := strings.Split(strings.TrimRight(ts.ScreenText(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("行数不足：%q", ts.ScreenText())
	}
	if lines[0] != "hello" {
		t.Errorf("第 0 行 = %q, 期望 hello", lines[0])
	}
	if lines[1] != "world" {
		t.Errorf("第 1 行 = %q, 期望 world", lines[1])
	}
	if col, row := ts.Cursor(); col != 5 || row != 1 {
		t.Errorf("光标 = (%d,%d), 期望 (5,1)", col, row)
	}
}

func TestTermAutoWrap(t *testing.T) {
	ts := newTestTerm(5, 3, 100)
	ts.Feed([]byte("abcdefg"))
	lines := strings.Split(strings.TrimRight(ts.ScreenText(), "\n"), "\n")
	if lines[0] != "abcde" {
		t.Errorf("第 0 行 = %q, 期望 abcde", lines[0])
	}
	if len(lines) < 2 || lines[1] != "fg" {
		t.Errorf("第 1 行 = %q, 期望 fg", strings.Join(lines, "|"))
	}
}

func TestTermCursorAndErase(t *testing.T) {
	ts := newTestTerm(10, 3, 100)
	ts.Feed([]byte("abcdef\x1b[2;3H"))
	if col, row := ts.Cursor(); col != 2 || row != 1 {
		t.Fatalf("CUP 后光标 = (%d,%d), 期望 (2,1)", col, row)
	}
	ts.Feed([]byte("XY\x1b[K"))
	lines := strings.Split(strings.TrimRight(ts.ScreenText(), "\n"), "\n")
	if lines[1] != "  XY" {
		t.Errorf("第 1 行 = %q, 期望 \"  XY\"", lines[1])
	}
	ts.Feed([]byte("Z"))
	lines = strings.Split(strings.TrimRight(ts.ScreenText(), "\n"), "\n")
	if lines[1] != "  XYZ" {
		t.Errorf("追加后第 1 行 = %q, 期望 \"  XYZ\"", lines[1])
	}
}

func TestTermSGRColors(t *testing.T) {
	ts := newTestTerm(20, 3, 100)
	ts.Feed([]byte("\x1b[31mA\x1b[38;2;1;2;3mB\x1b[0mC"))
	ts.mu.Lock()
	defer ts.mu.Unlock()
	a := ts.sc.cellAt(0, 0)
	b := ts.sc.cellAt(1, 0)
	c := ts.sc.cellAt(2, 0)
	if a.fg != termIndexed(1) {
		t.Errorf("A 前景 = %#x, 期望 256 色下标 1", a.fg)
	}
	if b.fg != termRGB(1, 2, 3) {
		t.Errorf("B 前景 = %#x, 期望真彩 #010203", b.fg)
	}
	if c.fg != termColorDefault {
		t.Errorf("C 前景应回到默认，实际 %#x", c.fg)
	}
}

func TestTermWideRunes(t *testing.T) {
	ts := newTestTerm(10, 3, 100)
	ts.Feed([]byte("中文abc"))
	lines := strings.Split(strings.TrimRight(ts.ScreenText(), "\n"), "\n")
	if lines[0] != "中文abc" {
		t.Errorf("第 0 行 = %q, 期望 中文abc", lines[0])
	}
	if col, _ := ts.Cursor(); col != 7 {
		t.Errorf("光标列 = %d, 期望 7（两个宽字符各占 2 格）", col)
	}
}

func TestTermSplitUTF8(t *testing.T) {
	ts := newTestTerm(10, 3, 100)
	for _, b := range []byte("汉字") {
		ts.Feed([]byte{b})
	}
	if got := strings.TrimRight(ts.ScreenText(), "\n "); got != "汉字" {
		t.Errorf("屏幕 = %q, 期望 汉字", got)
	}
}

func TestTermAltScreen(t *testing.T) {
	ts := newTestTerm(10, 3, 100)
	ts.Feed([]byte("main"))
	ts.Feed([]byte("\x1b[?1049h"))
	if got := strings.TrimRight(ts.ScreenText(), "\n "); got != "" {
		t.Errorf("备用屏应为空，实际 %q", got)
	}
	ts.Feed([]byte("alt"))
	ts.Feed([]byte("\x1b[?1049l"))
	if got := strings.TrimRight(ts.ScreenText(), "\n "); got != "main" {
		t.Errorf("回到主屏应为 main，实际 %q", got)
	}
}

func TestTermScrollback(t *testing.T) {
	ts := newTestTerm(10, 2, 50)
	for i := 0; i < 6; i++ {
		ts.Feed([]byte("line" + string(rune('0'+i)) + "\r\n"))
	}
	ts.mu.Lock()
	back := len(ts.sc.scrollBack)
	ts.mu.Unlock()
	if back < 4 {
		t.Errorf("回看行数 = %d, 期望 >= 4", back)
	}
	ts.ScrollLines(2)
	if off := ts.ScrollOffsetLines(); off != 2 {
		t.Errorf("回看偏移 = %d, 期望 2", off)
	}
	ts.ScrollBottom()
	if off := ts.ScrollOffsetLines(); off != 0 {
		t.Errorf("回到底部后偏移 = %d, 期望 0", off)
	}
}

func TestTermScrollbackTrim(t *testing.T) {
	ts := newTestTerm(10, 2, 3)
	for i := 0; i < 10; i++ {
		ts.Feed([]byte("x\r\n"))
	}
	ts.mu.Lock()
	back := len(ts.sc.scrollBack)
	ts.mu.Unlock()
	if back != 3 {
		t.Errorf("回看上限 3，实际 %d 行", back)
	}
}

func TestTermResizeKeepsContent(t *testing.T) {
	ts := newTestTerm(10, 3, 100)
	ts.Feed([]byte("hello\r\nworld"))
	ts.applySize(20, 5, false)
	lines := strings.Split(strings.TrimRight(ts.ScreenText(), "\n"), "\n")
	if lines[0] != "hello" || lines[1] != "world" {
		t.Errorf("resize 后内容丢失：%q", ts.ScreenText())
	}
	ts.Feed([]byte("\x1b[1;15HOK"))
	lines = strings.Split(strings.TrimRight(ts.ScreenText(), "\n"), "\n")
	if !strings.Contains(lines[0], "OK") {
		t.Errorf("放大后写入位置错误：%q", lines[0])
	}
}

func TestTermInsertDeleteChars(t *testing.T) {
	ts := newTestTerm(10, 2, 100)
	ts.Feed([]byte("abcdef\x1b[1;3H\x1b[2@"))
	if got := strings.TrimRight(ts.ScreenText(), "\n "); !strings.HasPrefix(got, "ab  cd") {
		t.Errorf("ICH 后 = %q, 期望 ab  cd…", got)
	}
	ts.Feed([]byte("\x1b[1;3H\x1b[2P"))
	if got := strings.TrimRight(ts.ScreenText(), "\n "); !strings.HasPrefix(got, "abcdef") {
		t.Errorf("DCH 后 = %q, 期望 abcdef", got)
	}
}

func TestTermKeyboardEncoding(t *testing.T) {
	ts := newTestTerm(20, 4, 100)
	var got []byte
	ts.OnData = func(b []byte) { got = append(got, b...) }

	cases := []struct {
		key  uint32
		want string
	}{
		{VK_RETURN, "\r"},
		{VK_BACK, "\x7f"},
		{VK_TAB, "\t"},
		{VK_ESCAPE, "\x1b"},
		{VK_UP, "\x1b[A"},
		{VK_DOWN, "\x1b[B"},
		{VK_RIGHT, "\x1b[C"},
		{VK_LEFT, "\x1b[D"},
		{0x24, "\x1b[H"},
		{0x23, "\x1b[F"},
		{VK_DELETE, "\x1b[3~"},
		{0x21, "\x1b[5~"},
		{0x22, "\x1b[6~"},
		{0x70, "\x1bOP"},
		{0x7B, "\x1b[24~"},
	}
	for _, c := range cases {
		got = got[:0]
		ts.HandleKey(c.key)
		if string(got) != c.want {
			t.Errorf("按键 %#x 编码 = %q, 期望 %q", c.key, string(got), c.want)
		}
	}
}

func TestTermKeyboardAppCursor(t *testing.T) {
	ts := newTestTerm(20, 4, 100)
	var got []byte
	ts.OnData = func(b []byte) { got = append(got, b...) }
	ts.Feed([]byte("\x1b[?1h"))
	ts.HandleKey(VK_UP)
	if string(got) != "\x1bOA" {
		t.Errorf("应用光标键模式 = %q, 期望 ESC O A", string(got))
	}
}

func TestTermSelectionText(t *testing.T) {
	ts := newTestTerm(20, 4, 100)
	ts.Feed([]byte("first line\r\nsecond line"))
	ts.mu.Lock()
	back := len(ts.sc.scrollBack)
	ts.sel = termSelection{active: true, ax: 0, ay: back, bx: 5, by: back + 1}
	ts.mu.Unlock()
	if got := ts.SelectionText(); got != "first line\nsecond" {
		t.Errorf("选区文本 = %q, 期望 first\\nsecond", got)
	}
}

func TestTermOSCTitle(t *testing.T) {
	ts := newTestTerm(20, 4, 100)
	var title string
	ts.OnTitle = func(s string) { title = s }
	ts.Feed([]byte("\x1b]0;my-title\x07"))
	if title != "my-title" {
		t.Errorf("标题 = %q, 期望 my-title", title)
	}
}

func TestTermDSRReply(t *testing.T) {
	ts := newTestTerm(20, 4, 100)
	var got []byte
	ts.OnData = func(b []byte) { got = append(got, b...) }
	ts.Feed([]byte("\x1b[3;5H\x1b[6n"))
	if string(got) != "\x1b[3;5R" {
		t.Errorf("DSR 应答 = %q, 期望 ESC [3;5R", string(got))
	}
}

func TestTermScrollRegion(t *testing.T) {
	ts := newTestTerm(10, 5, 100)
	ts.Feed([]byte("a\r\nb\r\nc\r\nd\r\ne"))
	ts.Feed([]byte("\x1b[2;4r\x1b[4;1H\n"))
	lines := strings.Split(strings.TrimRight(ts.ScreenText(), "\n"), "\n")
	if lines[0] != "a" {
		t.Errorf("滚动区外第 0 行 = %q, 期望 a", lines[0])
	}
	if lines[1] != "c" {
		t.Errorf("滚动后第 1 行 = %q, 期望 c", lines[1])
	}
}

func TestTermLineTextTrim(t *testing.T) {
	ts := newTestTerm(10, 2, 100)
	ts.Feed([]byte("abc   \r\n"))
	if got := ts.sc.lineText(0); got != "abc" {
		t.Errorf("行尾空白应去掉，实际 %q", got)
	}
}

// TestTermColorRender 颜色→漆色链路：16 色、256 色、真彩、粗体亮色、反显。
func TestTermColorRender(t *testing.T) {
	th := DarkTermTheme()

	ts := newTestTerm(20, 2, 10)
	ts.Feed([]byte("\x1b[31;42mX"))
	ts.mu.Lock()
	c := ts.sc.cellAt(0, 0)
	ts.mu.Unlock()
	if c.fg != termIndexed(1) || c.bg != termIndexed(2) {
		t.Fatalf("标准色解析错：fg=%#x bg=%#x", uint32(c.fg), uint32(c.bg))
	}
	if got, want := termFgPaint(c, &th), th.ANSI[1]; !samePaint(got, want) {
		t.Errorf("前景漆色 = %v, 期望 1 号红 %v", got, want)
	}
	if got := termPaintOf(c.bg, th.Bg, &th); !samePaint(got, th.ANSI[2]) {
		t.Errorf("背景漆色 = %v, 期望 2 号绿 %v", got, th.ANSI[2])
	}

	ts2 := newTestTerm(20, 2, 10)
	ts2.Feed([]byte("\x1b[38;5;196mX"))
	ts2.mu.Lock()
	c2 := ts2.sc.cellAt(0, 0)
	ts2.mu.Unlock()
	if got := termPaintOf(c2.fg, th.Fg, &th); got.R < 0.99 || got.G > 0.01 || got.B > 0.01 {
		t.Errorf("256 色 196 应为纯红，实际 %v", got)
	}
	if got := termPaintOf(termIndexed(232), th.Fg, &th); got.R > 0.05 {
		t.Errorf("灰阶 232 应接近黑，实际 %v", got)
	}

	ts3 := newTestTerm(20, 2, 10)
	ts3.Feed([]byte("\x1b[48;2;10;20;30mX"))
	ts3.mu.Lock()
	c3 := ts3.sc.cellAt(0, 0)
	ts3.mu.Unlock()
	if c3.bg != termRGB(10, 20, 30) {
		t.Errorf("真彩背景 = %#x，期望 RGB(10,20,30)", uint32(c3.bg))
	}

	bold := termCell{fg: termIndexed(1), attr: termAttrBold}
	if got, want := termFgPaint(bold, &th), th.ANSI[9]; !samePaint(got, want) {
		t.Errorf("粗体红应为亮红 %v，实际 %v", want, got)
	}
	rev := termCell{fg: termIndexed(1), bg: termIndexed(2), attr: termAttrReverse}
	if got, want := termFgPaint(rev, &th), th.ANSI[2]; !samePaint(got, want) {
		t.Errorf("反显前景应为背景色 %v，实际 %v", want, got)
	}
}

func TestTerminalNodeKind(t *testing.T) {
	ts := newTestTerm(20, 4, 100)
	n := Terminal(ts)
	if n == nil || n.Kind != KTerm {
		t.Fatalf("Terminal() 节点 Kind 错误：%+v", n)
	}
	// 键盘走 RegisterKeyHandler；节点上只挂 OnChar，两者同时挂会双发。
	if n.OnChar == nil || n.OnScroll == nil || n.OnDrag == nil || n.OnClickAt == nil {
		t.Errorf("终端节点缺少交互回调")
	}
	if n.OnKey != nil {
		t.Errorf("终端节点不应设 OnKey（按键统一走 RegisterKeyHandler，否则双发）")
	}
	if !n.Style.Focusable {
		t.Errorf("终端节点必须可聚焦，否则拿不到键盘事件")
	}
}

// TestTermPrintableKeyNotConsumed 可打印字符必须交给 OnChar，HandleKey 返回 false。
func TestTermPrintableKeyNotConsumed(t *testing.T) {
	ts := newTestTerm(20, 4, 100)
	var got []byte
	ts.OnData = func(b []byte) { got = append(got, b...) }
	if ts.HandleKey('A') {
		t.Errorf("HandleKey('A') 应返回 false（由 OnChar 发送）")
	}
	if len(got) != 0 {
		t.Errorf("HandleKey 不应发送可打印字符，实际发了 %q", string(got))
	}
	ts.HandleChar('A')
	if string(got) != "A" {
		t.Errorf("OnChar 应发送一次 A，实际 %q", string(got))
	}
}
