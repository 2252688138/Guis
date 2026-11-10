package ui

import (
	"fmt"
	"strings"
	"testing"
)

// TestCodeEditorInsertAndUndo 插入 / 换行 / 撤销 / 重做。
func TestCodeEditorInsertAndUndo(t *testing.T) {
	e := NewCodeEditorState("t1", "package main\n\nfunc main() {\n}\n")
	e.SetCaret(CodePos{Line: 0, Col: 0}, false)
	e.InsertText("// header\n")
	if e.Lines[0] != "// header" || e.Lines[1] != "package main" {
		t.Fatalf("插入行失败：%q", e.Lines[:2])
	}
	if !e.Dirty {
		t.Fatal("编辑后应标记 Dirty")
	}
	if !e.Undo() {
		t.Fatal("撤销应成功")
	}
	if e.Lines[0] != "package main" {
		t.Fatalf("撤销未恢复：%q", e.Lines[0])
	}
	if !e.Redo() {
		t.Fatal("重做应成功")
	}
	if e.Lines[0] != "// header" {
		t.Fatalf("重做未应用：%q", e.Lines[0])
	}
}

// TestCodeEditorAutoIndentAndNewline 回车自动缩进 / 花括号成对。
func TestCodeEditorAutoIndentNewline(t *testing.T) {
	e := NewCodeEditorState("t2", "func f() {\n}\n")
	// 光标放在第一行末尾，回车应插入一层缩进，并带出右括号行
	e.SetCaret(CodePos{Line: 0, Col: len([]rune("func f() {"))}, false)
	e.InsertNewline()
	if len(e.Lines) != 4 {
		t.Fatalf("期望 4 行（末尾空行），实际 %d：%q", len(e.Lines), e.Lines)
	}
	if e.Lines[1] != "    " {
		t.Fatalf("缩进行错误：%q", e.Lines[1])
	}
	if strings.TrimSpace(e.Lines[2]) != "}" {
		t.Fatalf("右括号行错误：%q", e.Lines[2])
	}
	if e.Pos.Line != 1 {
		t.Fatalf("光标应在中间行，实际 %d", e.Pos.Line)
	}
}

// TestCodeEditorSelection 选区 / 复制文本 / 删除选区 / 全选。
func TestCodeEditorSelection(t *testing.T) {
	e := NewCodeEditorState("t3", "alpha\nbeta\ngamma")
	e.Anchor = CodePos{Line: 0, Col: 2}
	e.Pos = CodePos{Line: 1, Col: 2}
	if got := e.SelectedText(); got != "pha\nbe" {
		t.Fatalf("跨行选区错误：%q", got)
	}
	e.DeleteForward() // 有选区时删除整个选区
	if e.Lines[0] != "alta" || e.Lines[1] != "gamma" {
		t.Fatalf("删除选区后：%q", e.Lines[:2])
	}
	e.SelectAll()
	lo, _ := e.Selection()
	if !lo.Start.Equal(CodePos{}) {
		t.Fatalf("全选起点错误：%+v", lo.Start)
	}
}

// TestCodeEditorIndentSelection 多行缩进 / 反缩进。
func TestCodeEditorIndentSelection(t *testing.T) {
	e := NewCodeEditorState("t4", "a\nb\nc")
	e.Anchor = CodePos{Line: 0, Col: 0}
	e.Pos = CodePos{Line: 1, Col: 1}
	e.IndentSelection(false)
	if e.Lines[0] != "    a" || e.Lines[1] != "    b" {
		t.Fatalf("缩进失败：%q", e.Lines)
	}
	e.IndentSelection(true)
	if e.Lines[0] != "a" || e.Lines[1] != "b" {
		t.Fatalf("反缩进失败：%q", e.Lines)
	}
}

// TestCodeEditorToggleComment 行注释切换（Go）。
func TestCodeEditorToggleComment(t *testing.T) {
	e := NewCodeEditorState("t5", "x := 1\ny := 2")
	e.SetLanguage("go")
	e.ToggleComment()
	if e.Lines[0] != "// x := 1" {
		t.Fatalf("加注释失败：%q", e.Lines[0])
	}
	e.Anchor, e.Pos = CodePos{Line: 0, Col: 0}, CodePos{Line: 0, Col: 3}
	e.ToggleComment()
	if e.Lines[0] != "x := 1" {
		t.Fatalf("去注释失败：%q", e.Lines[0])
	}
}

// TestCodeEditorMoveLines 上下移动行。
func TestCodeEditorMoveLines(t *testing.T) {
	e := NewCodeEditorState("t6", "one\ntwo\nthree")
	e.SetCaret(CodePos{Line: 1, Col: 0}, false)
	e.MoveLines(-1)
	if e.Lines[0] != "two" || e.Lines[1] != "one" {
		t.Fatalf("上移失败：%q", e.Lines)
	}
	e.MoveLines(1)
	if e.Lines[0] != "one" || e.Lines[1] != "two" {
		t.Fatalf("下移失败：%q", e.Lines)
	}
}

// TestCodeEditorFind 查找前进 / 回绕。
func TestCodeEditorFind(t *testing.T) {
	e := NewCodeEditorState("t7", "aaa\nbbb\naaa")
	if !e.Find("bbb", true) {
		t.Fatal("应找到 bbb")
	}
	if e.Pos.Line != 1 {
		t.Fatalf("定位行错误：%d", e.Pos.Line)
	}
	if !e.Find("aaa", true) {
		t.Fatal("应回绕找到 aaa")
	}
	if e.Pos.Line != 2 {
		t.Fatalf("回绕定位行错误：%d", e.Pos.Line)
	}
}

// TestCodeEditorHugeDocument 十万行：行数 / 可见区间 / 光标定位不随行数线性放大。
func TestCodeEditorHugeDocument(t *testing.T) {
	e := NewCodeEditorState("t8", "")
	e.Lines = make([]string, 100000)
	for i := range e.Lines {
		e.Lines[i] = fmt.Sprintf("\tvalue%06d := compute(%d)", i, i)
	}
	e.cache.reset()
	if got := e.LineCount(); got != 100000 {
		t.Fatalf("行数错误：%d", got)
	}
	e.viewH = 600
	e.LineH = 20
	e.ScrollY = 500000
	first, last := e.viewState(e.viewH)
	if first <= 0 || last-first > 40 {
		t.Fatalf("可见区间应只有几十行，实际 %d..%d", first, last)
	}
	e.ScrollToLine(99999)
	if e.ScrollY <= 0 {
		t.Fatal("滚动到末尾失败")
	}
	e.clampScroll()
	if e.ScrollY > e.maxScrollY()+1 {
		t.Fatal("滚动偏移未限位")
	}
}

// TestTokenizeCLike Go 关键字 / 字符串 / 注释 / 跨行块注释。
func TestTokenizeCLike(t *testing.T) {
	spans, st := tokenizeCodeLine("go", `func main() { s := "hi" } // tail`, codeStateNormal)
	if st != codeStateNormal {
		t.Fatalf("行尾状态错误：%d", st)
	}
	has := func(k codeTokKind) bool {
		for _, s := range spans {
			if s.Kind == k {
				return true
			}
		}
		return false
	}
	if !has(codeKeyword) || !has(codeString) || !has(codeComment) || !has(codeFunction) {
		t.Fatalf("着色片段缺失：%+v", spans)
	}
	// 跨行块注释
	if _, st := tokenizeCodeLine("go", "/* start", codeStateNormal); st != codeStateBlockComment {
		t.Fatal("块注释起始状态错误")
	}
	if _, st := tokenizeCodeLine("go", "still comment */ x := 1", codeStateBlockComment); st != codeStateNormal {
		t.Fatal("块注释结束状态错误")
	}
	// 中文注释不越界（历史 bug：字节偏移当 rune 下标）
	if _, st := tokenizeCodeLine("go", "/* 中文注释 */", codeStateNormal); st != codeStateNormal {
		t.Fatal("中文块注释解析失败")
	}
}

// TestTokenizeJSONAndHTML JSON 键名与 HTML 标签。
func TestTokenizeJSONAndHTML(t *testing.T) {
	spans, _ := tokenizeCodeLine("json", `"name": "guis",`, codeStateNormal)
	hasAttr := false
	for _, s := range spans {
		if s.Kind == codeAttribute {
			hasAttr = true
		}
	}
	if !hasAttr {
		t.Fatalf("JSON 键名未高亮：%+v", spans)
	}
	spans, _ = tokenizeCodeLine("html", `<div class="x">hi</div>`, codeStateNormal)
	tag, attr := false, false
	for _, s := range spans {
		if s.Kind == codeTag {
			tag = true
		}
		if s.Kind == codeAttribute {
			attr = true
		}
	}
	if !tag || !attr {
		t.Fatalf("HTML 标签/属性未高亮：%+v", spans)
	}
}

// TestCodeBracketsDiagnostics 括号诊断（多余 / 未闭合）。
func TestCodeBracketsDiagnostics(t *testing.T) {
	ds := checkBrackets([]string{"func f() {", "}"}, 0, 2)
	if len(ds) != 0 {
		t.Fatalf("正常代码不应有诊断：%+v", ds)
	}
	ds = checkBrackets([]string{"func f() {"}, 0, 1)
	if len(ds) == 0 {
		t.Fatal("未闭合括号应有诊断")
	}
	ds = checkBrackets([]string{"}"}, 0, 1)
	if len(ds) == 0 {
		t.Fatal("多余右括号应有诊断")
	}
	// 字符串里的括号不算
	ds = checkBrackets([]string{`s := "{(}"`}, 0, 1)
	if len(ds) != 0 {
		t.Fatalf("字符串内括号不应计数：%+v", ds)
	}
}

// TestLSPPositionUTF16 LSP 位置换算（emoji 占 2 个 UTF-16 单元）。
func TestLSPPositionUTF16(t *testing.T) {
	p := lspPosition("a😀b", 0, 1)
	if p["character"].(int) != 1 {
		t.Fatalf("第一个字符后应为 1，实际 %v", p["character"])
	}
	p = lspPosition("a😀b", 0, 2)
	if p["character"].(int) != 3 {
		t.Fatalf("emoji 后应占 2 单元，实际 %v", p["character"])
	}
}

// TestLSPApplyTextEdits 文本编辑倒序套用（多编辑不互相破坏偏移）。
func TestLSPApplyTextEdits(t *testing.T) {
	text := "one two three"
	edits := []any{
		map[string]any{"newText": "TWO", "range": map[string]any{
			"start": map[string]any{"line": float64(0), "character": float64(4)},
			"end":   map[string]any{"line": float64(0), "character": float64(7)},
		}},
		map[string]any{"newText": "ONE", "range": map[string]any{
			"start": map[string]any{"line": float64(0), "character": float64(0)},
			"end":   map[string]any{"line": float64(0), "character": float64(3)},
		}},
	}
	out := applyTextEdits(text, edits)
	if out != "ONE TWO three" {
		t.Fatalf("文本编辑套用错误：%q", out)
	}
}

// TestLSPStripSnippet 去掉 snippet 占位符。
func TestLSPStripSnippet(t *testing.T) {
	if got := stripSnippet("fmt.Println(${1:args})"); got != "fmt.Println(args)" {
		t.Fatalf("snippet 处理错误：%q", got)
	}
	if got := stripSnippet("hello$0"); got != "hello" {
		t.Fatalf("snippet 序号处理错误：%q", got)
	}
}

// TestFileURI Windows 路径 → URI → 路径 往返。
func TestFileURIRoundTrip(t *testing.T) {
	for _, p := range []string{`C:\proj\main.go`, `H:\JIMaX\guis\ui\codeeditor.go`} {
		u := FileURI(p)
		if !strings.HasPrefix(u, "file:///") {
			t.Fatalf("URI 格式错误：%q", u)
		}
		if back := URIPath(u); strings.ReplaceAll(back, "\\", "/") != strings.ReplaceAll(p, "\\", "/") {
			t.Fatalf("往返失败：%q → %q → %q", p, u, back)
		}
	}
}

// TestCodeLanguageByPath 扩展名推断。
func TestCodeLanguageByPath(t *testing.T) {
	cases := map[string]string{
		"a.go": "go", "b.ts": "typescript", "c.json": "json",
		"d.py": "python", "e.md": "markdown", "f.unknown": "plain",
	}
	for path, want := range cases {
		if got := codeLanguageByPath(path); got != want {
			t.Fatalf("%s 推断为 %q，期望 %q", path, got, want)
		}
	}
}
