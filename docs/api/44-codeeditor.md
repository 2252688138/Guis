<!-- group: 数据展示 -->
# 代码编辑器 · CodeEditor（十万行 / 语法高亮 / LSP）

`ui.CodeEditor` 是**原生自绘**的代码编辑器：不做整篇字符串扫描，只绘制可见行，
十万行文档的绘制量仍然是 40~60 行；带行号、小地图、缩进参考线、当前行高亮、
内建语法解析、查找替换、折叠、多光标、右键菜单，并可直接接任意 LSP 语言服务器。

## 1. 最小用法

```go
var ed = ui.NewCodeEditorState("main-editor", "package main\n\nfunc main() {\n}\n")
ed.Language = "go"                 // 或 ed.SetLanguageByPath("main.go")
ed.OnChange = func(text string) { /* 内容变了 */ }

func view() *ui.Node {
    return ui.CodeEditor(ed).St(ui.St().Grow(1))
}
```

* `NewCodeEditorState(key, text)` 里的 `key` **必须全局唯一**：同一时间只有一个编辑器持有键盘焦点；
* 节点由框架自绘（`Kind == KCodeEditor`），不产生子节点，所以不会被文档长度拖慢；
* 状态跨帧保持（文档、光标、滚动、折叠、补全、面板都在 `CodeEditorState` 里）。

## 2. 为什么十万行不卡

与 Monaco 同思路：

* 文档按行存（`Lines []string`），编辑只动受影响的行；
* 只对**可见行**做词法高亮与排版，滚动只改 `ScrollY`；
* 高亮结果按行缓存（行内容 hash + 跨行词法状态），编辑只失效受影响的行；
* 列宽按「累计列宽」缓存，Tab 按 `TabSize` 个空格测量（否则缩进参考线会落进正文）；
* 小地图按行采样成固定数量的矩形，与文档长度无关；
* 键盘走 `RegisterKeyHandler`（优先于 Tab 焦点切换），右键走 `OnRightClickAt`。

## 3. 配置字段

```go
ed.Lines                 // []string，文档（按行）
ed.Pos / ed.Anchor       // 光标 / 选区锚点（CodePos{Line, Col}，0 基，列按 rune 计）
ed.ScrollY, ed.ScrollX   // 滚动位置
ed.LineH                 // 行高，默认 ui.CodeDefaultLineHeight(20)
ed.FontSize              // 字号，默认 ui.CodeDefaultFontSize(13)
ed.FontFamily            // 字体，默认等宽字体
ed.TabSize               // Tab 宽度（空格数），默认 4
ed.Theme                 // CodeTheme 配色
ed.MinW, ed.MinH         // 最小尺寸，默认 560×320
ed.ShowGutter            // 行号栏
ed.ShowMinimap           // 小地图
ed.ShowCurrentLine       // 当前行高亮
ed.ShowIndentGuide       // 缩进参考线
ed.MinimapW              // 小地图宽度，默认 84
ed.Wrap                  // 软换行（等价的 ed.SetWrap(on) 会重算缓存）
ed.ReadOnly              // 只读
ed.Dirty                 // 有未保存修改（编辑自动置位，MarkSaved 清除）
ed.Key                   // 节点 Key，务必唯一
ed.Path                  // 文件路径（LSP 文档 URI 用）
ed.Language              // go / javascript / typescript / json / python / markdown / plain …
ed.Title                 // 标题（状态栏 / 页签用）
ed.OnChange func(text string)
ed.OnCaret  func(line, col int)   // 0 基
ed.OnSave   func(text string)     // Ctrl+S
ed.OnStatus func(msg string)      // 状态栏消息
```

## 4. 文档、光标与选区

```go
ed.SetText("package main\n")   // 整体替换（重置撤销栈 / 光标 / 高亮缓存 / Dirty）
ed.Text()                      // 全文
ed.LineCount()                 // 行数
ed.SetLanguage("json")         // 换语言（重置词法与语义高亮缓存）
ed.SetLanguageByPath("a.ts")   // 按扩展名推断
ed.SetTheme(ui.LightCodeTheme())
ed.MarkSaved()                 // 清除 Dirty

ed.Selection()                 // (CodeRange, bool) 是否存在选区
ed.SelectedText()              // 选中文本
ed.SetCaret(ui.CodePos{Line: 3, Col: 2}, false)  // keep=true 保留选区锚点（等于扩展选区）
ed.SetCaretLine(10)            // 跳到某行行首（并滚动到可见）
ed.SelectAll()
ed.InsertText("内容")          // 在光标处插入（有选区时替换选区）
```

`CodePos` 提供 `Before / After / Equal`；`CodeRange` = `{Start, End}`（左闭右开）。

## 5. 编辑操作

| 分组 | 方法 |
| --- | --- |
| 撤销 | `Undo()` / `Redo()`（往返 400 步） |
| 删除 | `DeleteBack()` / `DeleteForward()` |
| 换行 | `InsertNewline()`（自动缩进 + 大括号内自动多缩进一级） |
| 缩进 | `IndentSelection(outdent bool)`（多行整体生效） |
| 移动光标 | `MoveCaret(dLine, dCol, extend)` / `MoveLineStart(extend)` / `MoveLineEnd(extend)` / `MoveWord(dir, extend)` / `PageMove(dir, extend)` |
| 剪贴板 | `Copy()` / `Cut()` / `Paste()` |
| 行操作 | `MoveLines(dir)` / `DuplicateLine()` / `ToggleComment()` |
| 跳转 | `GoToLine(line)` / `ScrollToLine(line)` / `ScrollByLines(n)` |

带返回值的 `bool` 表示这次操作是否真的改了内容（用于决定要不要 `RequestRedraw`）。

## 6. 查找 / 替换 / 问题面板

```go
ed.OpenFind(0)        // 0=查找 1=转到行 2=重命名符号 3=查找+替换
ed.OpenReplace()      // = OpenFind(3)
ed.CloseFind()
ed.Find("foo", true)  // 从光标向后找
ed.FindText()         // 当前查找词
ed.FindOpen()         // 面板是否打开
ed.MatchCount()       // 命中数（状态栏「3/17」）
ed.ReplaceCurrent()   // 替换当前命中
ed.ReplaceAll()       // 全部替换，返回替换次数
```

查找面板自己维护插入点与焦点，支持 `← →` / `Home` / `End` / `Delete` / `Backspace` /
`Ctrl+A` / `Ctrl+V`，所有 `Ctrl`/`Alt` 组合都被面板吞掉（不会误触发编辑器快捷键）；
面板打开时编辑器不画光标与当前行高亮，点正文自动收起。

问题（诊断）面板：

```go
ed.SetDiagnostics(ds)      // 手动设置诊断（LSP 与内置括号检查会自动合并）
ed.Diagnostics()           // 全部诊断（按位置排序）
ed.ToggleDiagPanel()       // Ctrl+Shift+M
ed.DiagPanelOpen()
ed.PickDiagnostic(i)       // 跳到第 i 条
ed.NextDiagnostic()        // F8
ed.DiagStep(dir)
```

## 7. 视图：软换行、折叠、滚动条、小地图

```go
ed.SetWrap(true)            // Alt+Z
ed.ToggleFold(line)         // 折叠/展开该行起始的块
ed.ToggleFoldAtCaret()      // Ctrl+Shift+[
ed.FoldAll(true)            // 全部折叠 / 展开
ed.FoldedCount()
ed.StatusLine()             // 状态栏文本：语言 · 行:列 · 软换行 · 折叠 · 多光标 · 诊断数
```

滚动条与小地图都支持**按住拖动连续滚动**（不是点一下跳一段）；
拖动文本先过阈值才从按下点开始选，普通单击会清掉多光标（不会出现「双光标 / 点一下选中多行」）。

## 8. 多光标

```go
ed.AddCursor(+1)            // Ctrl+Alt+↓ 向下加一个光标（-1 向上）
ed.AddCursorAt(pos)         // Alt+点击
ed.Carets()                 // [][2]CodePos：主光标 + 附加光标
ed.CursorCount()
ed.HasMultiCursor()
ed.ClearCursors()           // Esc
ed.InsertAtCarets("x")      // 在全部光标处插入（批量编辑只触发一次 OnChange）
ed.MoveCarets(+1)
ed.SelectLine(line)         // 行号点击：选中整行
```

## 9. 语法高亮

```go
ed.SetLanguage("go")                       // 内置词法分析，未知名称按 plain
ed.SetTheme(ui.DarkCodeTheme())            // 默认
ed.SetTheme(ui.LightCodeTheme())
```

`CodeTheme` 的字段：`Bg / GutterBg / GutterText / GutterActive / CurrentLine / Selection / Caret /
IndentGuide / Scrollbar / MinimapBg / MinimapView / MinimapLine / Text / Keyword / Type / String /
Comment / Number / Function / Operator / Punct / Constant / Attribute / Tag / Invalid`，
全部是 `gpu.Paint`（用 `ui.C(...)` 构造）。配色独立于 UI 主题，切主题时由页面自己换一套。

内置语言覆盖 `go / javascript / typescript / json / python / markdown / plain`
（`codeLanguageByPath` 还能按扩展名推断更多别名），未知语言按 `plain` 处理。

## 10. 接入 LSP

```go
srv := ui.NewLSPClient("gopls", "gopls", "-mode=stdio")
srv.Root = "H:/proj"                  // 工作区根（go.mod / package.json 所在目录）
srv.OnLog = func(s string) { log.Print(s) }
if err := srv.Start(); err == nil {
    ed.Path = "H:/proj/main.go"
    ed.AttachLSP(srv)                 // 内部投递到 UI 线程，可在后台协程调用
}
```

| 能力 | 方法 / 快捷键 |
| --- | --- |
| 补全 | `Ctrl+Space` → `RequestCompletion()`，`AcceptCompletion()` 接受 |
| 悬停 | `F1` → `RequestHover()`，鼠标停留 450ms 自动触发 |
| 转到定义 | `F12` → `RequestDefinition()`（返回 `[]CodeLocation`） |
| 格式化 | `Shift+Alt+F` → `FormatDocument()` |
| 重命名 | `F2` → `RequestRename()`（输入条，回车提交） |
| 诊断 | 服务端 `publishDiagnostics` 自动合并，`F8` 跳下一条 |
| 语义高亮 | `ed.SetSemantic(true)` / `SetSemanticAsync(true)` / `RequestSemantic()` |
| 日志 | `ed.LSPLog()` 取服务端日志（最多保留若干行），`ed.LSPStatus()` 取一行状态摘要 |

要点：

* **只有 LSP 报错才写状态栏**，多行服务端日志进 `LSPLog()` 缓冲，不会把底栏撑成大块文本；
* `AttachLSP` / `SetSemantic` 都是**投递**执行（`attachLSPNow` / `setSemanticNow` 在 `drainLSP` 里跑），
  后台协程不会直接改编辑器状态；
* `InitOptions` / `Settings` 默认已打开 gopls 的 `semanticTokens`（否则语义高亮会一直「等待服务端」）；
* `ui.FileURI(path)` / `ui.URIPath(uri)` 处理盘符、空格、中文与 `%3A`；
* Web / 移动端没有子进程，`Start()` 返回可读错误，界面应隐藏 LSP 入口。

## 11. 鼠标行为

| 操作 | 行为 |
| --- | --- |
| 单击 | 定位光标（并清空多光标） |
| 拖动 | 过了 3px 阈值才从**按下点**开始选（抖动不会误选） |
| `Alt` + 单击 | 在点击处添加光标 |
| 行号点击 | 选中整行 |
| 悬停 450ms | 在该符号上方弹提示：先显示词/诊断占位，gopls hover 结果回来再覆盖；换词 / 离开 / 输入 / 点击即收起 |
| 右键 | 打开右键菜单（不进左键拖选状态） |

悬停提示按 Markdown 轻量排版：代码块按编辑器语言着色、`#` 标题用 Keyword 色、列表加 `·` 前缀，
段落之间有虚线分隔；面板位置会避开光标与节点边界。

## 12. 快捷键

`ui.CodeEditorShortcuts()` 返回内建快捷键表（帮助面板直接用它渲染）：

| 快捷键 | 功能 |
| --- | --- |
| `Ctrl+S` | 保存（触发 `OnSave`） |
| `Ctrl+Z` / `Ctrl+Y` | 撤销 / 重做 |
| `Ctrl+C` / `X` / `V` / `A` | 复制 / 剪切 / 粘贴 / 全选 |
| `Ctrl+D` | 复制当前行 |
| `Ctrl+/` | 行注释切换 |
| `Ctrl+F` / `Ctrl+H` | 查找 / 替换 |
| `Ctrl+G` | 转到行 |
| `F3` / `Shift+F3` | 查找下一个 / 上一个 |
| `Alt+↑` / `Alt+↓` | 上移 / 下移当前行 |
| `Tab` / `Shift+Tab` | 缩进 / 反缩进 |
| `Home` / `End` | 行首（两段式：先到首个非空字符）/ 行尾 |
| `Ctrl+←` / `Ctrl+→` | 按词移动 |
| `Alt+Z` | 软换行 |
| `Ctrl+Shift+[` / `]` | 折叠当前块 / 全部展开 |
| `Ctrl+Alt+↑` / `↓` | 添加光标 |
| `Esc` | 收起多光标 / 关闭面板 |
| `Ctrl+Shift+M` | 问题面板 |
| `F8` | 下一条诊断 |
| `F12` / `F2` / `F1` | 定义 / 重命名 / 悬停 |
| `Shift+Alt+F` | 格式化文档 |

## 13. 示例

```bash
go run ./cmd/gallery --case code
```

示例页演示十万行文档、语言/配色切换、行号与小地图开关、右键菜单、快捷键帮助面板，
以及一键连接本机 `gopls`（`GUIS_LSP_CMD` 可指定其它语言服务器）。
