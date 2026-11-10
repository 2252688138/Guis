package ui

import "testing"

// TestCodeEditorTypeAtCaret 连续输入必须紧贴光标位置往后排，不能越插越跳。
//
// 回归：replaceRange 曾用「新行宽 - 行尾宽」当新光标列，行首有内容时会把光标推到
// 插入点右边，第二个字符就插到别处（"abc XdefY"），随后光标列越过行尾被 clamp，
// 左箭头 / Delete 也跟着失灵。
func TestCodeEditorTypeAtCaret(t *testing.T) {
	e := NewCodeEditorState("caret1", "abc def")
	e.SetCaret(CodePos{Line: 0, Col: 4}, false)
	for _, r := range "XY" {
		e.HandleChar(r)
	}
	if e.Lines[0] != "abc XYdef" {
		t.Fatalf("插入位置错乱：%q", e.Lines[0])
	}
	if e.Pos != (CodePos{Line: 0, Col: 6}) {
		t.Fatalf("光标应在插入内容末尾 Col=6，实际 %+v", e.Pos)
	}

	// 行首 Tab 缩进的行：从 Tab 后面开始输入也要连续
	e2 := NewCodeEditorState("caret2", "\t\"context\"\nnext")
	e2.SetCaret(CodePos{Line: 0, Col: 1}, false)
	for _, r := range "hello" {
		e2.HandleChar(r)
	}
	if e2.Lines[0] != "\thello\"context\"" {
		t.Fatalf("Tab 行插入位置错乱：%q", e2.Lines[0])
	}
	if e2.Pos != (CodePos{Line: 0, Col: 6}) {
		t.Fatalf("Tab 行光标错误：%+v", e2.Pos)
	}

	// 多行插入（粘贴）光标应落在最后一行插入内容之后
	e3 := NewCodeEditorState("caret3", "ab\ncd")
	e3.SetCaret(CodePos{Line: 0, Col: 1}, false)
	e3.InsertText("1\n2")
	if e3.Lines[0] != "a1" || e3.Lines[1] != "2b" || e3.Lines[2] != "cd" {
		t.Fatalf("多行插入错误：%q", e3.Lines)
	}
	if e3.Pos != (CodePos{Line: 1, Col: 1}) {
		t.Fatalf("多行插入光标错误：%+v", e3.Pos)
	}
}

// TestCodeEditorArrowCrossLine 左右方向键在行首 / 行尾要接到相邻行（否则看起来「没反应」）。
func TestCodeEditorArrowCrossLine(t *testing.T) {
	e := NewCodeEditorState("arrow1", "one\nthree")
	e.SetCaret(CodePos{Line: 1, Col: 0}, false)
	e.MoveCaret(0, -1, false)
	if e.Pos != (CodePos{Line: 0, Col: 3}) {
		t.Fatalf("行首左箭头应到上一行行尾，实际 %+v", e.Pos)
	}
	e.MoveCaret(0, 1, false)
	if e.Pos != (CodePos{Line: 1, Col: 0}) {
		t.Fatalf("行尾右箭头应到下一行行首，实际 %+v", e.Pos)
	}
	// 第一行行首再按左 / 最后一行行尾再按右：原地不动
	e.SetCaret(CodePos{Line: 0, Col: 0}, false)
	e.MoveCaret(0, -1, false)
	if e.Pos != (CodePos{}) {
		t.Fatalf("文档开头左箭头不应移动：%+v", e.Pos)
	}
	e.SetCaret(CodePos{Line: 1, Col: 5}, false)
	e.MoveCaret(0, 1, false)
	if e.Pos != (CodePos{Line: 1, Col: 5}) {
		t.Fatalf("文档末尾右箭头不应移动：%+v", e.Pos)
	}
	// 有选区时左右键折叠到选区端点
	e.Anchor, e.Pos = CodePos{Line: 0, Col: 1}, CodePos{Line: 1, Col: 2}
	e.MoveCaret(0, -1, false)
	if e.Pos != (CodePos{Line: 0, Col: 1}) || e.Anchor != e.Pos {
		t.Fatalf("左箭头应折叠到选区起点：pos=%+v anchor=%+v", e.Pos, e.Anchor)
	}
}

// TestCodeEditorHomeAndDeleteWithTab Tab 缩进行：Home 落在首个非空白 rune 上，
// 行尾 Delete 接行、Ctrl+/ 注释插在缩进之后。
func TestCodeEditorHomeAndDeleteWithTab(t *testing.T) {
	e := NewCodeEditorState("tab1", "\tfoo()")
	e.SetCaret(CodePos{Line: 0, Col: 6}, false)
	e.MoveLineStart(false)
	if e.Pos != (CodePos{Line: 0, Col: 1}) {
		t.Fatalf("Home 应到 Tab 之后的 Col=1，实际 %+v", e.Pos)
	}
	e.MoveLineStart(false)
	if e.Pos != (CodePos{Line: 0, Col: 0}) {
		t.Fatalf("再按 Home 应到行首，实际 %+v", e.Pos)
	}

	e2 := NewCodeEditorState("tab2", "a\nb")
	e2.SetCaret(CodePos{Line: 0, Col: 1}, false)
	if !e2.DeleteForward() {
		t.Fatal("行尾 Delete 应删除换行")
	}
	if len(e2.Lines) != 1 || e2.Lines[0] != "ab" {
		t.Fatalf("行尾 Delete 应合并两行：%q", e2.Lines)
	}

	e3 := NewCodeEditorState("tab3", "\tfoo\n\tbar")
	e3.SetLanguage("go")
	e3.Anchor, e3.Pos = CodePos{Line: 0, Col: 0}, CodePos{Line: 1, Col: 4}
	e3.ToggleComment()
	if e3.Lines[0] != "\t// foo" || e3.Lines[1] != "\t// bar" {
		t.Fatalf("Tab 缩进行的注释位置错误：%q", e3.Lines)
	}
	e3.ToggleComment()
	if e3.Lines[0] != "\tfoo" || e3.Lines[1] != "\tbar" {
		t.Fatalf("Tab 缩进行去注释错误：%q", e3.Lines)
	}
}

// TestCodeEditorBackspaceNoControlChar 退格只删字符，不往文档里塞 \b。
func TestCodeEditorBackspaceNoControlChar(t *testing.T) {
	e := NewCodeEditorState("bs1", "abc")
	e.SetCaret(CodePos{Line: 0, Col: 3}, false)
	e.HandleKey(VK_BACK)
	if e.Lines[0] != "ab" || e.Pos != (CodePos{Line: 0, Col: 2}) {
		t.Fatalf("退格结果错误：%q pos=%+v", e.Lines[0], e.Pos)
	}
	for _, r := range e.Lines[0] {
		if r < 0x20 {
			t.Fatalf("文档里混进了控制字符 %#x", r)
		}
	}
}

// TestIsTextRune 控制字符不属于可编辑文本（框架 EvChar 分发前用它过滤）。
func TestIsTextRune(t *testing.T) {
	for _, r := range []rune{'\b', '\r', '\n', '\t', 0x01, 0x1a, 0x7f} {
		if isTextRune(r) {
			t.Fatalf("%#x 不应被当作文本", r)
		}
	}
	for _, r := range []rune{' ', 'a', '中', '😀'} {
		if !isTextRune(r) {
			t.Fatalf("%q 应被当作文本", r)
		}
	}
}

// TestCodeEditorIgnoresControlChars Backspace / Enter / Tab 的 WM_CHAR 不能被当成文本插入。
//
// 回归：框架曾把 TranslateMessage 生成的 0x08（Backspace）也交给 OnChar，
// 退格既删了字又往文档里插了一个不可见字符（渲染成缺字形方块），
// 之后光标与插入位置全乱——表现为「左箭头 / Delete 失灵、输入乱跳」。
func TestCodeEditorIgnoresControlChars(t *testing.T) {
	e := NewCodeEditorState("ctrl1", "abc")
	e.SetCaret(CodePos{Line: 0, Col: 2}, false)
	for _, r := range []rune{'\b', 0x01, 0x0b, 0x7f} {
		e.HandleChar(r)
	}
	if e.Lines[0] != "abc" || e.Pos != (CodePos{Line: 0, Col: 2}) {
		t.Fatalf("控制字符不应进入文档：%q pos=%+v", e.Lines[0], e.Pos)
	}
	// 粘贴带控制字符的文本：控制字符被剔除，\n / \t 保留
	e.InsertText("x\x08y\tz\nw")
	if e.Lines[0] != "abxy\tz" || e.Lines[1] != "wc" {
		t.Fatalf("粘贴清洗错误：%q", e.Lines[:2])
	}
}

// TestCodeEditorTypingThenArrow Delete / 左箭头在连续输入之后仍然有效。
func TestCodeEditorTypingThenArrow(t *testing.T) {
	e := NewCodeEditorState("after1", "abc")
	e.SetCaret(CodePos{Line: 0, Col: 1}, false)
	for _, r := range "XY" {
		e.HandleChar(r)
	}
	e.HandleKey(VK_LEFT)
	if e.Pos != (CodePos{Line: 0, Col: 2}) {
		t.Fatalf("输入后左箭头失效：%+v", e.Pos)
	}
	e.HandleKey(VK_DELETE)
	if e.Lines[0] != "aXbc" {
		t.Fatalf("输入后 Delete 失效：%q", e.Lines[0])
	}
}
