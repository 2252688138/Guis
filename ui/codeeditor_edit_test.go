package ui

import "testing"

// TestCodeEditorAutoClosePair 左符号自动补右符号，右符号已存在时只跳过。
func TestCodeEditorAutoClosePair(t *testing.T) {
	e := NewCodeEditorState("t-ac", "x")
	e.SetCaret(CodePos{Line: 0, Col: 1}, false)
	e.HandleChar('(')
	if e.Lines[0] != "x()" {
		t.Fatalf("自动闭合失败：%q", e.Lines[0])
	}
	if e.Pos.Col != 2 {
		t.Fatalf("光标应停在括号中间，实得 col=%d", e.Pos.Col)
	}
	// 再输入 ')' 应只右移光标，不重复插入
	e.HandleChar(')')
	if e.Lines[0] != "x()" {
		t.Fatalf("右括号重复插入：%q", e.Lines[0])
	}
	if e.Pos.Col != 3 {
		t.Fatalf("光标应右移一格，实得 col=%d", e.Pos.Col)
	}
}

// TestCodeEditorAutoCloseOff 关闭 AutoPair 后行为与旧版一致（只插入左符号）。
func TestCodeEditorAutoCloseOff(t *testing.T) {
	e := NewCodeEditorState("t-ac2", "")
	e.AutoPair = false
	e.HandleChar('(')
	if e.Lines[0] != "(" {
		t.Fatalf("关闭后不应自动闭合：%q", e.Lines[0])
	}
}

// TestCodeEditorReadOnlyNoAutoClose 只读模式下编辑类新功能不生效。
func TestCodeEditorReadOnlyNoAutoClose(t *testing.T) {
	e := NewCodeEditorState("t-ro", "abc")
	e.ReadOnly = true
	e.HandleChar('(')
	if e.Lines[0] != "abc" {
		t.Fatalf("只读模式被写入：%q", e.Lines[0])
	}
}

// TestCodeEditorDeletePairBack 光标在空配对中间时一次退格删两侧。
func TestCodeEditorDeletePairBack(t *testing.T) {
	e := NewCodeEditorState("t-dp", "a()b")
	e.SetCaret(CodePos{Line: 0, Col: 2}, false)
	if !e.DeleteBack() {
		t.Fatal("退格应成功")
	}
	if e.Lines[0] != "ab" {
		t.Fatalf("空配对未被一次删除：%q", e.Lines[0])
	}
	// 非空配对只删一格
	e2 := NewCodeEditorState("t-dp2", "(abc)")
	e2.SetCaret(CodePos{Line: 0, Col: 4}, false)
	e2.DeleteBack()
	if e2.Lines[0] != "(ab)" {
		t.Fatalf("非空配对应只删一格：%q", e2.Lines[0])
	}
}

// TestCodeEditorSurroundSelection 有选区时输入左符号包裹选区，并可一次撤销。
func TestCodeEditorSurroundSelection(t *testing.T) {
	e := NewCodeEditorState("t-sr", "abc")
	e.SelectAll()
	e.HandleChar('(')
	if e.Lines[0] != "(abc)" {
		t.Fatalf("包裹失败：%q", e.Lines[0])
	}
	if got := e.SelectedText(); got != "abc" {
		t.Fatalf("包裹后应仍选中内部文本，实得 %q", got)
	}
	if !e.Undo() {
		t.Fatal("撤销应成功")
	}
	if e.Lines[0] != "abc" {
		t.Fatalf("撤销未还原：%q", e.Lines[0])
	}
}

// TestCodeEditorPasteIndent 多行粘贴跟随当前行缩进（空行不补空白）。
func TestCodeEditorPasteIndent(t *testing.T) {
	e := NewCodeEditorState("t-pi", "func f() {\n    \n}")
	e.SetCaret(CodePos{Line: 1, Col: 4}, false)
	got := e.pasteIndented("a\nb\n\nc")
	want := "a\n    b\n\n    c"
	if got != want {
		t.Fatalf("粘贴缩进不符\n got=%q\nwant=%q", got, want)
	}
	// 单行粘贴不动
	if s := e.pasteIndented("single"); s != "single" {
		t.Fatalf("单行粘贴不应加缩进：%q", s)
	}
}

// TestCodeEditorZoomFont 字号缩放受上下限约束，行高同步变化。
func TestCodeEditorZoomFont(t *testing.T) {
	e := NewCodeEditorState("t-zm", "line\nline2")
	oldSize, oldLineH := e.FontSize, e.LineH
	if !e.ZoomFont(1) {
		t.Fatal("放大应生效")
	}
	if e.FontSize <= oldSize {
		t.Fatalf("字号未放大：%v", e.FontSize)
	}
	if e.LineH <= oldLineH {
		t.Fatalf("行高未同步放大：%v", e.LineH)
	}
	for i := 0; i < 100; i++ {
		e.ZoomFont(-1)
	}
	if e.FontSize != e.MinFontSize {
		t.Fatalf("应夹在下限 %v，实得 %v", e.MinFontSize, e.FontSize)
	}
	for i := 0; i < 100; i++ {
		e.ZoomFont(1)
	}
	if e.FontSize != e.MaxFontSize {
		t.Fatalf("应夹在上限 %v，实得 %v", e.MaxFontSize, e.FontSize)
	}
}

// TestCodeEditorBracketPair 括号配对定位（含嵌套与找不到配对的情况）。
func TestCodeEditorBracketPair(t *testing.T) {
	e := NewCodeEditorState("t-bm", "f(a[0])")
	e.SetCaret(CodePos{Line: 0, Col: 1}, false) // 光标在 '(' 上
	a, b, ok := e.BracketPair()
	if !ok || a.Col != 1 || b.Col != 6 {
		t.Fatalf("括号配对错误：ok=%v a=%+v b=%+v", ok, a, b)
	}
	// 嵌套：光标在 '[' 上应配对到对应的 ']'
	e.SetCaret(CodePos{Line: 0, Col: 3}, false)
	a, b, ok = e.BracketPair()
	if !ok || a.Col != 3 || b.Col != 5 {
		t.Fatalf("嵌套配对错误：ok=%v a=%+v b=%+v", ok, a, b)
	}
	// 光标处无括号 → false
	e.SetCaret(CodePos{Line: 0, Col: 0}, false)
	if _, _, ok := e.BracketPair(); ok {
		t.Fatal("光标处无括号时不应返回配对")
	}
	// 关闭开关后不计算
	e.BracketMatchHL = false
	e.SetCaret(CodePos{Line: 0, Col: 1}, false)
	if _, _, ok := e.BracketPair(); ok {
		t.Fatal("关闭 BracketMatchHL 后不应返回配对")
	}
}

// TestCodeEditorBracketMatchCache 配对结果按 changeSeq / 光标缓存。
func TestCodeEditorBracketMatchCache(t *testing.T) {
	e := NewCodeEditorState("t-bmc", "(a)")
	e.SetCaret(CodePos{Line: 0, Col: 0}, false)
	_, b1, ok1 := e.matchPairCached()
	if !ok1 || b1.Col != 2 {
		t.Fatalf("首次配对错误：%+v", b1)
	}
	// 命中缓存
	_, b2, _ := e.matchPairCached()
	if b1 != b2 {
		t.Fatalf("缓存结果不一致：%+v vs %+v", b1, b2)
	}
	// 文本改动后重新计算
	e.InsertText("x")
	if _, _, ok := e.matchPairCached(); !ok {
		t.Fatal("文本改动后应重新计算配对")
	}
}
