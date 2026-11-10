// guis — gallery 页面：代码编辑器（十万行 / 高亮 / 行号 / 小地图 / LSP）
//
// 演示 ui.CodeEditor 组件的全部能力：
//   - 十万行文档虚拟渲染（只绘制可见行，滚动不卡）；
//   - 内置语法解析器（Go / JS / TS / JSON / Python / Markdown 等着色）；
//   - 左侧行号、右侧小地图、当前行高亮、缩进参考线；
//   - 右键菜单与完整快捷键（Ctrl+S/Z/Y/D/F/G、Tab 缩进、Alt+↑↓ 移动行…）；
//   - LSP 接入：一键连接本机 gopls（可用 GUIS_LSP_CMD 指定其它服务），
//     提供补全 / 悬停 / 定义跳转 / 格式化 / 重命名 / 实时诊断。
package pages

import (
	"fmt"
	"os"
	"strings"
	"time"

	"guis/ui"
)

// ---- 页面状态 ----

var (
	// codeEd 编辑器实例（懒创建，跨帧保持）。
	codeEd *ui.CodeEditorState
	// codeLang 当前示例语言。
	codeLang = "go"
	// codeDark 编辑器配色（深色 / 浅色）。
	codeDark    = true
	codeStatus  = "点击编辑器获得焦点，右键可打开菜单"
	codeCaret   = "行 1, 列 1"
	codeShowMap = true
	codeShowNum = true
	codeHelp    = false
	// codeLSPStarting 正在启动的语言服务器（防重复点击；为空表示没在启动）
	codeLSPStarting *ui.LSPClient
)

// codeLanguages 示例页可切换的语言（语法解析器覆盖范围）。
var codeLanguages = []string{"go", "javascript", "typescript", "json", "python", "markdown", "plain"}

// ---- 示例文本 ----

const codeSampleGo = `// guis — 代码编辑器示例（内置解析器着色）
package main

import (
	"context"
	"fmt"
	"time"
)

// Server 一个可取消的后台服务。
type Server struct {
	Name    string
	Timeout time.Duration
	handles map[string]func(context.Context) error
}

// NewServer 构造服务并注册默认处理器。
func NewServer(name string) *Server {
	return &Server{
		Name:    name,
		Timeout: 3 * time.Second,
		handles: map[string]func(context.Context) error{
			"health": func(ctx context.Context) error {
				fmt.Println("ok")
				return nil
			},
		},
	}
}

// Run 在超时时间内逐个执行处理器；任一失败即返回。
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	for name, fn := range s.handles {
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务 %s 被取消：%w", name, ctx.Err())
		default:
			if err := fn(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	srv := NewServer("demo")
	if err := srv.Run(context.Background()); err != nil {
		panic(err)
	}
}
`

const codeSampleJS = `// 虚拟列表 + 增量渲染（JavaScript 示例）
const LINE_HEIGHT = 20;

export class VirtualList {
  constructor({ total, onRange }) {
    this.total = total;
    this.onRange = onRange;
    this.scrollTop = 0;
    this.height = 600;
  }

  get range() {
    const first = Math.floor(this.scrollTop / LINE_HEIGHT);
    const count = Math.ceil(this.height / LINE_HEIGHT) + 2;
    return [Math.max(0, first), Math.min(this.total, first + count)];
  }

  scrollTo(top) {
    this.scrollTop = Math.max(0, Math.min(top, this.total * LINE_HEIGHT - this.height));
    const [a, b] = this.range;
    this.onRange?.(a, b);
  }
}

export default new VirtualList({ total: 1e5, onRange: (a, b) => console.log(a, b) });
`

const codeSampleJSON = `{
  "name": "guis-code-editor",
  "version": "1.0.0",
  "features": ["virtual-render", "syntax-highlight", "minimap", "lsp"],
  "limits": { "maxLines": 100000, "lineHeight": 20, "tabSize": 4 },
  "lsp": {
    "command": "gopls",
    "languageId": "go",
    "capabilities": { "completion": true, "hover": true, "rename": true }
  },
  "enabled": true,
  "weight": 0.85
}
`

const codeSamplePython = `"""代码编辑器示例（Python）"""

from dataclasses import dataclass, field
from typing import Callable


@dataclass
class Token:
    kind: str
    text: str
    start: int
    end: int = field(default=0)


def tokenize(line: str) -> list[Token]:
    out: list[Token] = []
    i = 0
    while i < len(line):
        ch = line[i]
        if ch.isspace():
            i += 1
            continue
        if ch == '#':
            out.append(Token("comment", line[i:], i))
            break
        if ch in ('"', "'"):
            j = i + 1
            while j < len(line) and line[j] != ch:
                j += 1
            out.append(Token("string", line[i:j + 1], i))
            i = j + 1
            continue
        i += 1
    return out
`

const codeSampleMarkdown = `# 代码编辑器 · 使用说明

## 快捷键

| 按键 | 功能 |
| --- | --- |
| Ctrl+S | 保存 |
| Ctrl+Z / Ctrl+Y | 撤销 / 重做 |
| Ctrl+F / Ctrl+G | 查找 / 转到行 |
| Ctrl+D | 复制当前行 |
| Alt+↑ / Alt+↓ | 移动当前行 |
| F12 | 转到定义（需 LSP） |

## 十万行

> 只绘制可见行：行数从 100 增加到 100000，每帧成本几乎不变。

- [x] 虚拟渲染
- [x] 小地图
- [ ] 多光标
`

// codeSampleText 按语言返回示例文本。
func codeSampleText(lang string) string {
	switch lang {
	case "javascript":
		return codeSampleJS
	case "typescript":
		return codeSampleJS
	case "json":
		return codeSampleJSON
	case "python":
		return codeSamplePython
	case "markdown":
		return codeSampleMarkdown
	}
	return codeSampleGo
}

// ---- 编辑器创建 ----

// ensureCodeEditor 懒创建编辑器并按当前主题同步配色。
func ensureCodeEditor() *ui.CodeEditorState {
	if codeEd == nil {
		codeEd = ui.NewCodeEditorState("gallery-code", codeSampleText(codeLang))
		codeEd.SetLanguage(codeLang)
		codeEd.Path = codeSamplePath(codeLang)
		codeEd.Title = "main"
		codeEd.TabSize = 4
		codeEd.OnCaret = func(line, col int) {
			codeCaret = fmt.Sprintf("行 %d, 列 %d", line, col)
			ui.RequestRedraw()
		}
		codeEd.OnSave = func(text string) {
			codeStatus = fmt.Sprintf("已保存（%d 行 / %.1f KB）", codeEd.LineCount(), float64(len(text))/1024)
		}
		codeEd.OnStatus = func(msg string) {
			codeStatus = msg
			ui.RequestRedraw()
		}
		codeEd.OnChange = func(text string) {
			ui.RequestRedraw()
		}
	}
	if codeDark {
		codeEd.SetTheme(ui.DarkCodeTheme())
	} else {
		codeEd.SetTheme(ui.LightCodeTheme())
	}
	codeEd.ShowMinimap = codeShowMap
	codeEd.ShowGutter = codeShowNum
	return codeEd
}

// codeSamplePath 示例文件路径（LSP 需要真实可解析的路径；项目根下的虚拟文件名）。
func codeSamplePath(lang string) string {
	dir := codeProjectRoot()
	name := "main.go"
	switch lang {
	case "javascript":
		name = "virtual-list.js"
	case "typescript":
		name = "virtual-list.ts"
	case "json":
		name = "config.json"
	case "python":
		name = "tokenizer.py"
	case "markdown":
		name = "README.md"
	}
	return dir + string(os.PathSeparator) + name
}

// codeProjectRoot 当前工作目录（gallery 启动目录即项目根）。
func codeProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ---- 动作 ----

// SwitchCodeLanguage 切换示例语言（换文本 + 换解析器）。
func SwitchCodeLanguage(lang string) {
	e := ensureCodeEditor()
	codeLang = lang
	e.SetLanguage(lang)
	e.Path = codeSamplePath(lang)
	e.SetText(codeSampleText(lang))
	codeStatus = "已切换到 " + lang + " 示例"
	ui.RequestRedraw()
}

// LoadCodeStress 载入十万行压力测试文本。
func LoadCodeStress() {
	e := ensureCodeEditor()
	e.SetLanguage("go")
	e.Path = ""
	e.SetText(CodeStressText(100000))
	e.ScrollToLine(1)
	codeStatus = fmt.Sprintf("已载入 %d 行压力测试文本：滚动试试（只绘制可见行）", e.LineCount())
	SwitchCase("code")
	ui.RequestRedraw()
}

// CodeStressText 生成 n 行 Go 代码（约 34B/行，十万行 ≈ 3.4MB）。
// 用显式行计数逐行写入，保证生成行数与宣称一致（曾因估算偏差只生成 6 万行）。
func CodeStressText(n int) string {
	if n < 16 {
		n = 16
	}
	var sb strings.Builder
	sb.Grow(n * 34)
	lines := 0
	emit := func(s string) {
		sb.WriteString(s)
		sb.WriteByte('\n')
		lines++
	}
	emit("// 十万行压力测试：以下内容全部可编辑、可高亮、可滚动")
	emit("package main")
	emit("")
	for i := 0; lines < n-8; i++ {
		emit(fmt.Sprintf("// block %06d：虚拟滚动只渲染视口内的行", i))
		emit(fmt.Sprintf("func handler%06d(ctx context.Context, id int) error {", i))
		emit(fmt.Sprintf("\tvalue := compute(%d, \"item-%06d\")", i, i))
		emit("\treturn nil")
		emit("}")
		emit("")
	}
	for lines < n {
		emit("// padding：把行数补满")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// ToggleCodeLSP 连接 / 断开本机语言服务器（默认 gopls）。
func ToggleCodeLSP() {
	e := ensureCodeEditor()
	if srv := e.LSP(); srv != nil {
		srv.Stop()
		e.AttachLSP(nil)
		e.SetSemantic(false)
		Toast("已断开 LSP，回退内置解析器")
		ui.RequestRedraw()
		return
	}
	if codeLSPStarting != nil {
		Toast("正在连接 LSP，请稍候…")
		return
	}
	cmd := os.Getenv("GUIS_LSP_CMD")
	if cmd == "" {
		cmd = "gopls"
	}
	srv := ui.NewLSPClient("gopls", cmd)
	srv.Root = codeProjectRoot()
	srv.Timeout = 8 * time.Second
	codeLSPStarting = srv
	codeStatus = "正在启动 " + cmd + " …"
	go func() {
		if err := srv.Start(); err != nil {
			codeLSPStarting = nil
			// 后台协程不能直接改页面状态：状态 / 开关都走编辑器的投递接口，下一帧生效
			e.SetStatus("LSP 启动失败：" + err.Error() + "（可设置 GUIS_LSP_CMD 指定其它服务）")
			ui.RequestRedraw()
			return
		}
		codeLSPStarting = nil
		e.AttachLSP(srv)
		// 语义高亮：服务端声明 semanticTokensProvider 时自动开启
		e.SetSemanticAsync(true)
	}()
}

// FormatCode 格式化（LSP 优先，内置缩进整理兜底）。
func FormatCode() {
	ensureCodeEditor().FormatDocument()
	ui.RequestRedraw()
}

// ---- 页面构建 ----

// codeCase 代码编辑器示例页。
func codeCase() *ui.Node {
	e := ensureCodeEditor()

	head := pageHead("代码编辑器",
		"十万行虚拟渲染 · 语法高亮 · 左侧行号 / 右侧小地图 · 右键菜单 + 快捷键 · LSP 语法解析器")

	// 第一行：语言切换
	langRow := ui.Row()
	langRow.St(ui.St().GapOf(6)).AlignTo(ui.AlignCenter)
	langRow.Add(ui.Text("语言").St(ui.St().Text(11, 500, ui.ColTextMuted).PadOnly(0, 6, 0, 0)))
	for _, lang := range codeLanguages {
		l := lang
		langRow.Add(codeChip(l, codeLang == l, func() { SwitchCodeLanguage(l) }))
	}

	// 第二行：能力开关 / 动作
	actRow := ui.Row(
		codeChip("十万行压力测试", false, LoadCodeStress),
		codeChip("载入示例", false, func() { SwitchCodeLanguage(codeLang) }),
		codeChip("格式化", false, FormatCode),
		codeChip("小地图", codeShowMap, func() {
			codeShowMap = !codeShowMap
			codeStatus = fmt.Sprintf("小地图：%v", codeShowMap)
		}),
		codeChip("行号", codeShowNum, func() {
			codeShowNum = !codeShowNum
			codeStatus = fmt.Sprintf("行号：%v", codeShowNum)
		}),
		codeChip("浅色", !codeDark, func() {
			codeDark = !codeDark
			codeStatus = "编辑器配色：" + map[bool]string{true: "深色", false: "浅色"}[codeDark]
		}),
		codeChip("软换行", e.Wrap, func() {
			e.SetWrap(!e.Wrap)
			codeStatus = fmt.Sprintf("软换行：%v", e.Wrap)
			ui.RequestRedraw()
		}),
		codeChip("折叠", e.FoldedCount() > 0, func() {
			if !e.ToggleFoldAtCaret() {
				codeStatus = "当前行没有可折叠的块（{ } 配对范围）"
			}
			ui.RequestRedraw()
		}),
		codeChip("问题", e.DiagPanelOpen(), func() {
			e.ToggleDiagPanel()
			ui.RequestRedraw()
		}),
		codeChip("连接 gopls", e.LSP() != nil, ToggleCodeLSP),
		codeChip("快捷键", codeHelp, func() { codeHelp = !codeHelp }),
		ui.Spacer(),
		ui.Text(codeLSPStatusText(e)).St(ui.St().Text(11, 500, ui.ColTextMuted)),
	).St(ui.St().GapOf(6)).AlignTo(ui.AlignCenter)

	// 编辑区（自绘节点：不加外层 Scroll，滚动由组件自己管理）
	editor := ui.CodeEditor(e).St(ui.St().Wp(100).GrowTo(1).MinHeight(240))
	editorBox := ui.Box(editor).St(ui.St().Wp(100).GrowTo(1).R(10).Stroke(1, ui.ColStroke).Clipped())

	// 状态栏：语言 / 诊断条数 / 软换行 / 折叠 / 多光标都在这里，右键菜单保持精简
	status := ui.Row(
		ui.Text(codeCaret).St(ui.St().Text(11, 500, ui.ColTextMuted)),
		ui.Text("·").St(ui.St().Text(11, 400, ui.ColTextMuted)),
		ui.Text(fmt.Sprintf("%d 行", e.LineCount())).St(ui.St().Text(11, 500, ui.ColTextMuted)),
		ui.Text("·").St(ui.St().Text(11, 400, ui.ColTextMuted)),
		ui.Text(e.StatusLine()).St(ui.St().Text(11, 500, ui.ColPrimary)),
		ui.Text("·").St(ui.St().Text(11, 400, ui.ColTextMuted)),
		ui.Text(codeStatusText()).St(ui.St().Text(11, 400, ui.ColTextDim)).Grow(1),
		ui.Text("右键菜单 / Ctrl+F / Ctrl+H / F8 / F12").St(ui.St().Text(10.5, 400, ui.ColTextMuted)),
	).St(ui.St().GapOf(8)).AlignTo(ui.AlignCenter)

	rows := []*ui.Node{head, langRow, actRow}
	if codeHelp {
		rows = append(rows, codeShortcutPanel())
	}
	rows = append(rows, editorBox, status)
	return ui.Column(rows...).St(ui.St().GapOf(10).Padding(18).Hp(100))
}

// codeLSPStatusText LSP 状态文案。
func codeLSPStatusText(e *ui.CodeEditorState) string {
	if srv := e.LSP(); srv != nil {
		if srv.Started() {
			return "LSP：" + srv.Name + " 已连接"
		}
		return "LSP：" + srv.Name + " 未就绪"
	}
	return "LSP：未连接（内置解析器）"
}

// codeStatusText 状态栏文案（诊断条数已并入 StatusLine，这里只补语义高亮状态）。
func codeStatusText() string {
	e := ensureCodeEditor()
	msg := codeStatus
	if s := e.SemanticSummary(); s != "" {
		msg = fmt.Sprintf("%s · %s", msg, s)
	}
	return msg
}

// codeChip 一个可点的小胶囊（页签 / 开关）。
func codeChip(label string, active bool, onTap func()) *ui.Node {
	n := ui.Text(label).St(ui.St().Text(11.5, 500, ui.ColTextDim).PadXY(9, 5).R(7).
		BgC(ui.ColBgSoft).Stroke(1, ui.ColStroke))
	if active {
		n.St(ui.St().Text(11.5, 600, ui.ColText).BgC("rgba(99,102,241,0.22)").Stroke(1, ui.ColPrimary))
	}
	return n.Tap(onTap)
}

// codeShortcutPanel 快捷键一览（帮助面板）。
func codeShortcutPanel() *ui.Node {
	list := ui.CodeEditorShortcuts()
	var cols []*ui.Node
	for i := 0; i < len(list); i += 2 {
		row := ui.Row()
		row.St(ui.St().GapOf(10))
		for j := i; j < i+2 && j < len(list); j++ {
			pair := list[j]
			row.Add(ui.Row(
				ui.Text(pair[0]).St(ui.St().Text(10.5, 600, ui.ColText).W(140).
					PadXY(6, 3).R(5).BgC(ui.ColBgSoft).Stroke(1, ui.ColStroke)),
				ui.Text(pair[1]).St(ui.St().Text(11, 400, ui.ColTextMuted)),
			).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter).Grow(1))
		}
		cols = append(cols, row)
	}
	return ui.Column(cols...).St(ui.St().GapOf(6).Padding(12).R(10).
		BgC(ui.ColPanel).Stroke(1, ui.ColStroke))
}
