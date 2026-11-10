package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 真机 LSP 回归：启动本机 gopls，跑通补全 / 悬停 / 定义 / 重命名 / 格式化 / 诊断 / 语义高亮。
//
// 没装 gopls 时自动跳过（不阻塞 CI）；指定路径：
//
//	GUIS_GOPLS=D:\go\bin\gopls.exe go test ./ui -run TestLSPLive -v
//
// go test -short 也会跳过（gopls 冷启动 + 首次构建需要几秒）。
func goplsPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("GUIS_GOPLS"); p != "" {
		return p
	}
	p, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("未找到 gopls，跳过真机 LSP 回归")
	}
	return p
}

// lspWrite 在临时工作区写文件。
func lspWrite(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("写文件失败：%v", err)
	}
	return p
}

// lspRead 读取文件内容。
func lspRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读文件失败：%v", err)
	}
	return string(b)
}

// lspFindPos 找到文本里 marker 之后 offset 个字符的相对位置（行、rune 列）。
func lspFindPos(t *testing.T, text, marker string, offset int) (int, int) {
	t.Helper()
	idx := strings.Index(text, marker)
	if idx < 0 {
		t.Fatalf("文本里找不到 %q", marker)
	}
	line := strings.Count(text[:idx], "\n")
	lineStart := strings.LastIndex(text[:idx], "\n") + 1
	col := len([]rune(text[lineStart:idx])) + offset
	return line, col
}

func TestLSPLiveGopls(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过真机 LSP 回归")
	}
	gopls := goplsPath(t)
	dir := t.TempDir()
	lspWrite(t, dir, "go.mod", "module livesample\n\ngo 1.21\n")

	const mainSrc = `package main

import "fmt"

type Greeter struct{ Name string }

func (g *Greeter) Hello() string {
	return "hello " + g.Name
}

func main() {
	g := &Greeter{Name: "guis"}
	fmt.Println(g.Hello())
}
`
	const otherSrc = `package main

// Helper 被重命名的目标。
func Helper() string { return "h" }
`
	mainPath := lspWrite(t, dir, "main.go", mainSrc)
	otherPath := lspWrite(t, dir, "other.go", otherSrc)

	srv := NewLSPClient("gopls", gopls)
	srv.Root = dir
	srv.Timeout = 60 * time.Second
	if err := srv.Start(); err != nil {
		t.Fatalf("gopls 启动失败：%v", err)
	}
	defer srv.Stop()
	if !srv.Started() {
		t.Fatal("gopls 未完成 initialize 握手")
	}

	mainURI := FileURI(mainPath)
	otherURI := FileURI(otherPath)
	srv.DidOpen(mainURI, "go", mainSrc)
	srv.DidOpen(otherURI, "go", otherSrc)

	// ---- 1. 补全：fmt. 之后应出现 Println ----
	line, col := lspFindPos(t, mainSrc, "fmt.", len("fmt."))
	var items []CodeCompletion
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		items = srv.Completion(mainURI, "go", line, col, mainSrc)
		if len(items) > 0 {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if len(items) == 0 {
		t.Fatal("补全无结果（gopls 未就绪或 completion 解析失败）")
	}
	hasPrintln := false
	for _, it := range items {
		if strings.Contains(it.Label, "Println") {
			hasPrintln = true
			if it.Insert == "" {
				t.Fatalf("补全项 %q 缺少插入文本", it.Label)
			}
			break
		}
	}
	if !hasPrintln {
		names := make([]string, 0, 8)
		for i, it := range items {
			if i >= 8 {
				break
			}
			names = append(names, it.Label)
		}
		t.Fatalf("补全结果中没有 Println（前几项：%v）", names)
	}

	// ---- 2. 定义：g.Hello() 的 Hello → Greeter.Hello ----
	dLine, dCol := lspFindPos(t, mainSrc, "g := &Greeter", 0)
	_ = dLine
	_ = dCol
	hLine, hCol := lspFindPos(t, mainSrc, "g.Hello()", len("g.He"))
	locs := srv.Definition(mainURI, "go", hLine, hCol, mainSrc)
	if len(locs) == 0 {
		t.Fatal("Definition 无结果")
	}
	if filepath.Base(locs[0].Path) != "main.go" {
		t.Fatalf("定义位置文件错误：%q", locs[0].Path)
	}
	if locs[0].Line < 5 || locs[0].Line > 7 {
		t.Fatalf("定义行号不符（期望 Greeter.Hello 附近），实际第 %d 行", locs[0].Line+1)
	}

	// ---- 3. 悬停 ----
	hover := srv.Hover(mainURI, "go", hLine, hCol, mainSrc)
	if strings.TrimSpace(hover) == "" {
		t.Fatal("Hover 无内容")
	}
	if !strings.Contains(hover, "Hello") {
		t.Fatalf("Hover 内容未提及 Hello：%q", hover)
	}

	// ---- 4. 重命名：Helper → GreetHelper ----
	oLine, oCol := lspFindPos(t, otherSrc, "func Helper", len("func "))
	changes, ok := srv.Rename(otherURI, "go", oLine, oCol, otherSrc, "GreetHelper")
	if !ok || len(changes) == 0 {
		t.Fatal("Rename 无改动")
	}
	renamed := ""
	for path, txt := range changes {
		t.Logf("rename 改动：%q → %d 字节", path, len(txt))
		if filepath.Base(path) == "other.go" {
			renamed = txt
		}
	}
	if !strings.Contains(renamed, "func GreetHelper") {
		t.Fatalf("重命名结果错误：%q", renamed)
	}

	// ---- 5. 格式化：故意不排版的文件应被改写 ----
	const uglySrc = "package main\n\nfunc Ugly() {\n  x := 1\n  _ = x\n}\n"
	uglyPath := lspWrite(t, dir, "ugly.go", uglySrc)
	uglyURI := FileURI(uglyPath)
	srv.DidOpen(uglyURI, "go", uglySrc)
	formatted, fok := srv.Format(uglyURI, uglySrc, 4)
	if !fok || formatted == "" {
		t.Fatal("Format 无结果")
	}
	if formatted == uglySrc {
		t.Fatalf("Format 未改写未排版文本：%q", formatted)
	}
	if strings.Contains(formatted, "  x := 1") {
		t.Fatalf("Format 未把两空格改为制表符：%q", formatted)
	}

	// ---- 6. 诊断：把变量改成未定义名，应收到 error 级诊断 ----
	bad := strings.Replace(mainSrc, `g := &Greeter{Name: "guis"}`, "g := undefinedVar", 1)
	srv.DidChange(mainURI, bad)
	var diags []CodeDiagnostic
	deadline = time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		diags = srv.Diagnostics(mainURI)
		for _, d := range diags {
			if d.Severity == 1 && strings.Contains(d.Message, "undefined") {
				goto gotDiag
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("未收到 undefined 诊断（收到 %d 条）", len(diags))

gotDiag:
	// ---- 7. 语义高亮：先把文档恢复成能正常解析的版本 ----
	srv.DidChange(mainURI, mainSrc)
	if !srv.SupportsSemantic() {
		t.Fatal("gopls 未声明 semanticTokensProvider")
	}
	var toks []codeSemToken
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if ts, ok := srv.SemanticTokens(mainURI); ok && len(ts) > 0 {
			toks = ts
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if len(toks) == 0 {
		t.Fatal("语义 token 为空：确认 LSPClient 把 semanticTokens 配置发给了 gopls")
	}
	semEd := NewCodeEditorState("live", mainSrc)
	semEd.SetLanguage("go")
	semEd.SetSemantic(true)
	semEd.applySemantic(toks)
	if semEd.semanticLineCount() == 0 {
		t.Fatal("语义 token 未映射到任何行")
	}
	if semEd.SemanticSummary() == "" {
		t.Fatal("语义摘要为空")
	}
	t.Logf("语义高亮：%d 个 token / %d 行", len(toks), semEd.semanticLineCount())

	// ---- 8. 编辑器整体接入：AttachLSP 后补全走 LSP 路径（异步） ----
	ed := NewCodeEditorState("live-attach", mainSrc)
	ed.SetLanguage("go")
	ed.Path = mainPath
	ed.AttachLSP(srv) // 下一帧（drainLSP）才真正绑定
	ed.SetSemantic(true)
	ed.SetCaret(CodePos{Line: line, Col: col}, false)
	ed.RequestCompletion()
	deadline = time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		ed.drainLSP()
		if ed.LSP() == srv && len(ed.semantic) > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if ed.LSP() != srv {
		t.Fatal("AttachLSP 未在 drainLSP 里生效")
	}
	if len(ed.semantic) == 0 {
		t.Fatal("编辑器没有拿到语义高亮（语义 token 请求链路断了）")
	}
	ed.RequestCompletion()
	deadline = time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		ed.drainLSP()
		if ed.compOpen && len(ed.compItems) > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ed.compOpen {
		t.Fatal("AttachLSP 后补全面板未打开")
	}

	// ---- 9. 鼠标悬停提示：先用本地词占位，再被 gopls hover 结果覆盖 ----
	hvLine, hvCol := lspFindPos(t, mainSrc, "g.Hello()", len("g.H"))
	ed.requestHoverAt(hvLine, hvCol, 40, 40)
	if !ed.hoverOpen || len(ed.hoverLines) == 0 {
		t.Fatal("悬停提示未弹出")
	}
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		ed.drainLSP()
		// 本地占位只有词本身（"Hello"），LSP 结果是完整签名
		if len(ed.hoverLines) > 0 && ed.hoverLines[0] != "Hello" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if len(ed.hoverLines) == 0 || ed.hoverLines[0] == "Hello" {
		t.Fatalf("悬停内容未被 LSP 结果替换（仍是本地占位）：%v", ed.hoverLines)
	}
	if !strings.Contains(strings.Join(ed.hoverLines, "\n"), "Hello() string") {
		t.Fatalf("悬停内容不是 Hello 的签名：%v", ed.hoverLines)
	}
	t.Logf("悬停：%v", ed.hoverLines)

	ed.ClearCursors()
	srv.Stop()
}
