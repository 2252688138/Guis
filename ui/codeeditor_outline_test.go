package ui

import "testing"

// TestCodeEditorScanSymbols 符号扫描（Go 的函数 / 方法 / 类型）。
func TestCodeEditorScanSymbols(t *testing.T) {
	src := "package main\n\ntype Foo struct{}\n\nfunc Bar() {\n}\n\nfunc (f *Foo) Baz() {\n}\n"
	e := NewCodeEditorState("t-sym", src)
	e.SetLanguage("go")
	syms := e.ScanSymbols()
	if len(syms) != 3 {
		t.Fatalf("应识别 3 个符号，实得 %d：%+v", len(syms), syms)
	}
	if syms[0].Name != "Foo" || syms[0].Kind != "type" || syms[0].Line != 2 {
		t.Fatalf("第 1 个符号不符：%+v", syms[0])
	}
	if syms[1].Name != "Bar" || syms[1].Kind != "func" || syms[1].Line != 4 {
		t.Fatalf("第 2 个符号不符：%+v", syms[1])
	}
	if syms[2].Name != "Baz" || syms[2].Kind != "method" || syms[2].Line != 7 {
		t.Fatalf("方法符号不符：%+v", syms[2])
	}
}

// TestCodeEditorOutlinePanel 大纲过滤与跳转。
func TestCodeEditorOutlinePanel(t *testing.T) {
	e := NewCodeEditorState("t-ou", "func Alpha() {}\nfunc Beta() {}\n")
	e.SetLanguage("go")
	e.ToggleOutline()
	if !e.OutlineOpen() {
		t.Fatal("大纲应打开")
	}
	if items := e.OutlineItems(); len(items) != 2 {
		t.Fatalf("应有 2 项，实得 %d", len(items))
	}
	e.outlineFilter = "be"
	if items := e.OutlineItems(); len(items) != 1 || items[0].Name != "Beta" {
		t.Fatalf("过滤失败：%+v", items)
	}
	e.outlineFilter = ""
	e.outlineSel = 1
	if !e.AcceptOutline() {
		t.Fatal("跳转应成功")
	}
	if e.OutlineOpen() {
		t.Fatal("跳转后应收起大纲")
	}
	if e.Pos.Line != 1 {
		t.Fatalf("应跳到第 2 行，实得 %d", e.Pos.Line)
	}
}

// TestCodeEditorBreadcrumb 面包屑层级与高度。
func TestCodeEditorBreadcrumb(t *testing.T) {
	e := NewCodeEditorState("t-bc", "type Foo struct{}\nfunc (f *Foo) Baz() {\n}\n")
	e.SetLanguage("go")
	e.Title = "demo.go"
	e.Pos = CodePos{Line: 2, Col: 0}
	path := e.BreadcrumbPath()
	if len(path) != 1 || path[0].Name != "Baz" {
		t.Fatalf("面包屑路径不符：%+v", path)
	}
	if h := e.breadcrumbH(); h <= 0 {
		t.Fatalf("开启时面包屑应有高度，实得 %v", h)
	}
	if got, want := e.topPad(), codeTopPad+codeBreadcrumbH; got != want {
		t.Fatalf("topPad 不符：%v != %v", got, want)
	}
	e.ShowBreadcrumb = false
	if e.breadcrumbH() != 0 || e.topPad() != codeTopPad {
		t.Fatal("关闭面包屑后应回到默认留白")
	}
}

// TestCodeEditorBookmarks 书签切换与跳转。
func TestCodeEditorBookmarks(t *testing.T) {
	e := NewCodeEditorState("t-bm2", "a\nb\nc\nd")
	e.SetCaret(CodePos{Line: 1, Col: 0}, false)
	e.ToggleBookmark()
	e.SetCaret(CodePos{Line: 3, Col: 0}, false)
	e.ToggleBookmark()
	if lines := e.BookmarkLines(); len(lines) != 2 || lines[0] != 1 || lines[1] != 3 {
		t.Fatalf("书签记录不符：%v", lines)
	}
	e.SetCaret(CodePos{Line: 0, Col: 0}, false)
	if !e.NextBookmark(1) || e.Pos.Line != 1 {
		t.Fatalf("应跳到第 2 行，实得 %d", e.Pos.Line)
	}
	if !e.NextBookmark(1) || e.Pos.Line != 3 {
		t.Fatalf("应跳到第 4 行，实得 %d", e.Pos.Line)
	}
	// 循环回到第一个
	if !e.NextBookmark(1) || e.Pos.Line != 1 {
		t.Fatalf("应循环到第 2 行，实得 %d", e.Pos.Line)
	}
	// 取消书签
	e.ToggleBookmark()
	if lines := e.BookmarkLines(); len(lines) != 1 || lines[0] != 3 {
		t.Fatalf("取消书签失败：%v", lines)
	}
}

// TestCodeEditorFoldParens 圆括号 / 方括号块也能折叠（import ( ) / var ( ) / 多行字面量）。
func TestCodeEditorFoldParens(t *testing.T) {
	src := "import (\n\t\"fmt\"\n\t\"time\"\n)\n\nvar (\n\ta = 1\n)\n\nfunc f() {\n\tcall(1, 2)\n}\n"
	e := NewCodeEditorState("t-fp", src)
	e.SetLanguage("go")
	if fo := e.foldAt(0); fo == nil || fo.End != 3 {
		t.Fatalf("import ( 应可折叠到第 4 行，实得 %+v", fo)
	}
	if fo := e.foldAt(5); fo == nil || fo.End != 7 {
		t.Fatalf("var ( 应可折叠到第 8 行，实得 %+v", fo)
	}
	if fo := e.foldAt(9); fo == nil || fo.End != 11 {
		t.Fatalf("函数体应可折叠，实得 %+v", fo)
	}
	// 单行调用 / 单行括号不应产生可折叠区间
	if fo := e.foldAt(10); fo != nil {
		t.Fatalf("单行 call(1, 2) 不应可折叠，实得 %+v", fo)
	}
	if !e.ToggleFold(0) {
		t.Fatal("import 块应能折叠")
	}
	if !e.isHidden(1) || !e.isHidden(3) {
		t.Fatal("折叠后块内行（含 )）应隐藏")
	}
	if e.isHidden(4) {
		t.Fatal("块外行不应隐藏")
	}
}

// TestCodeEditorFoldState 折叠状态存取。
func TestCodeEditorFoldState(t *testing.T) {
	src := "func f() {\n\ta()\n\tb()\n}\n"
	e := NewCodeEditorState("t-fs", src)
	e.SetLanguage("go")
	e.SetFoldState([]int{0, 999}) // 999 是非法行号，应被忽略
	state := e.FoldState()
	if len(state) != 1 || state[0] != 0 {
		t.Fatalf("折叠状态不符：%v", state)
	}
	if !e.isHidden(2) {
		t.Fatal("第 3 行应被折叠隐藏")
	}
	e.SetFoldState(nil)
	if len(e.FoldState()) != 0 || e.isHidden(2) {
		t.Fatal("清空折叠状态后应全部展开")
	}
}
