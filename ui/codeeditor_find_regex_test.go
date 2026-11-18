package ui

import "testing"

// TestCodeEditorFindRegex 正则查找与跳转（含非法正则不 panic）。
func TestCodeEditorFindRegex(t *testing.T) {
	e := NewCodeEditorState("t-rx", "foo1\nbar\nfoo22\n")
	e.FindRegex = true
	if !e.Find("foo\\d+", true) {
		t.Fatal("正则查找应有命中")
	}
	if got := e.SelectedText(); got != "foo1" {
		t.Fatalf("首个命中应为 foo1，实得 %q", got)
	}
	if !e.Find("foo\\d+", true) {
		t.Fatal("应能跳到下一处")
	}
	if got := e.SelectedText(); got != "foo22" {
		t.Fatalf("第二处命中应为 foo22，实得 %q", got)
	}
	// 再跳回绕到第一处
	e.Find("foo\\d+", true)
	if got := e.SelectedText(); got != "foo1" {
		t.Fatalf("应回绕到 foo1，实得 %q", got)
	}
	// 非法正则：不命中、不 panic
	e2 := NewCodeEditorState("t-rx2", "abc")
	e2.FindRegex = true
	if msg := e2.FindPatternError(); msg != "" {
		t.Fatalf("正常正则不应报错：%q", msg)
	}
	e2.findText = "("
	if e2.FindPatternError() == "" {
		t.Fatal("非法正则应给出提示")
	}
	if e2.Find("(", true) {
		t.Fatal("非法正则不应命中")
	}
}

// TestCodeEditorFindRegexReplaceAll 正则全部替换（含捕获组 $1）。
func TestCodeEditorFindRegexReplaceAll(t *testing.T) {
	e := NewCodeEditorState("t-rr", "x := f(1)\ny := f(2)\n")
	e.FindRegex = true
	e.findText = `f\((\d+)\)`
	e.replaceText = "g($1)"
	n := e.ReplaceAll()
	if n != 2 {
		t.Fatalf("应替换 2 处，实得 %d", n)
	}
	if e.Lines[0] != "x := g(1)" || e.Lines[1] != "y := g(2)" {
		t.Fatalf("替换结果不符：%q", e.Lines[:2])
	}
	// 一条撤销记录还原全部
	if !e.Undo() {
		t.Fatal("撤销应成功")
	}
	if e.Lines[0] != "x := f(1)" || e.Lines[1] != "y := f(2)" {
		t.Fatalf("撤销未还原：%q", e.Lines[:2])
	}
}

// TestCodeEditorFindInSel 只在选区内查找 / 替换。
func TestCodeEditorFindInSel(t *testing.T) {
	e := NewCodeEditorState("t-sel", "aa\naa\naa")
	e.FindInSel = true
	e.findText = "aa"
	e.Anchor = CodePos{Line: 1, Col: 0}
	e.Pos = CodePos{Line: 2, Col: 2} // 选区覆盖第 2、3 行
	if n := e.MatchCount(); n != 2 {
		t.Fatalf("选区内应命中 2 处，实得 %d", n)
	}
	if !e.Find("aa", true) {
		t.Fatal("选区内查找应命中")
	}
	if r, ok := e.Selection(); ok && r.Start.Line == 0 {
		t.Fatal("不应命中选区外（第 1 行）")
	}
}

// TestCodeEditorSelectAllMatches 全部匹配转多光标。
func TestCodeEditorSelectAllMatches(t *testing.T) {
	e := NewCodeEditorState("t-sam", "foo foo foo")
	e.findText = "foo"
	if n := e.SelectAllMatches(); n != 3 {
		t.Fatalf("应生成 3 个光标，实得 %d", n)
	}
	if e.CursorCount() != 3 {
		t.Fatalf("光标数不符：%d", e.CursorCount())
	}
	// 批量输入：三处一起被替换
	e.InsertAtCarets("bar")
	if e.Lines[0] != "bar bar bar" {
		t.Fatalf("多光标批量替换失败：%q", e.Lines[0])
	}
}

// TestCodeEditorFindHistory 搜索历史去重与翻页。
func TestCodeEditorFindHistory(t *testing.T) {
	e := NewCodeEditorState("t-his", "abc")
	e.findHistoryPush("foo")
	e.findHistoryPush("bar")
	e.findHistoryPush("foo") // 重复项上移
	if len(e.findHistory) != 2 || e.findHistory[0] != "foo" || e.findHistory[1] != "bar" {
		t.Fatalf("历史去重失败：%v", e.findHistory)
	}
	if !e.FindHistoryMove(-1) {
		t.Fatal("应能翻到上一条")
	}
	if e.findText != "foo" {
		t.Fatalf("历史取回不符：%q", e.findText)
	}
	e.FindHistoryMove(-1)
	if e.findText != "bar" {
		t.Fatalf("第二条历史不符：%q", e.findText)
	}
}

// TestCodeEditorMatchIndex 命中序号与总数。
func TestCodeEditorMatchIndex(t *testing.T) {
	e := NewCodeEditorState("t-mi", "aa bb aa")
	e.findText = "aa"
	if e.MatchCount() != 2 {
		t.Fatalf("命中数应为 2，实得 %d", e.MatchCount())
	}
	e.Find("aa", true)
	if idx := e.MatchIndex(); idx != 1 {
		t.Fatalf("应停在第 1 项，实得 %d", idx)
	}
	e.Find("aa", true)
	if idx := e.MatchIndex(); idx != 2 {
		t.Fatalf("应停在第 2 项，实得 %d", idx)
	}
}
