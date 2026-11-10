<!-- group: 数据展示 -->
# 富文本与编辑器 · RichText / RichEditor / Undo

## 1. 只读富文本 RichText

`TextSpan` 是一段连续同一样式的文字：

```go
type TextSpan struct {
    Text      string
    Color     string    // 空 = 继承
    BgColor   string    // 片段底色（高亮）
    Size      float32   // 0 = 继承
    Weight    int32     // 0 = 继承
    Family    string    // 空 = 继承
    Link      string    // 非空可点击（悬停带下划线）
    Underline bool
    Strike    bool
    Code      bool      // 等宽字体 + 底色
}
```

```go
ui.RichText([]ui.TextSpan{
    {Text: "状态："},
    {Text: "已通过", Color: ui.ColSuccess, Weight: 700},
    {Text: "（"},
    {Text: "查看详情", Link: "detail:42", Color: ui.ColPrimary},
    {Text: "）"},
    {Text: " v1.2.3", Code: true},
}, ui.RichTextOptions{
    Size:     13,
    Color:    ui.ColText,
    MaxLines: 2,          // >0 时超出截断并加省略号
    Align:    ui.AlignStart,
    LinkColor: ui.ColPrimary,
    CodeBg:    ui.ColHover,
}).OnSpanClickEvt(func(s ui.TextSpan) {
    if strings.HasPrefix(s.Link, "detail:") { openDetail(strings.TrimPrefix(s.Link, "detail:")) }
})
```

`RichTextOptions`：`Size / Weight / Color / Family / LineHeight / MaxLines / Align / LinkColor /
CodeBg / NoWrap`。默认自动折行；`NoWrap` 需配合 `St().W(...)` 裁剪。

`ui.Rich(s)` 是 `ui.Text(s)` 的语义别名（单一样式）。

## 2. 富文本编辑器 RichEditor

编辑器状态由业务持有：

```go
var ed = ui.NewRichEditor("mail-body")

ui.Column(
    ui.RichToolbar(ed, func() { ui.RequestRedraw() }),          // 粗/斜/下划线/删除线/代码/颜色/字号/撤销重做
    ui.RichEditor(ed, 220, func() { ui.RequestRedraw() }),      // 编辑区
).St(ui.St().GapOf(8))
// 等价于：
ui.RichEditorEx(ed, 220, func() { ui.RequestRedraw() })
```

`RichEditorState`：

```go
ed.Text()                 // 全文
ed.Selection()            // (lo, hi) rune 区间
ed.Insert("文字")         // 在光标处插入（无选区）
ed.Toggle(func(s *ui.RichStyle) { s.Bold = true })   // 有选区应用到选区，否则作用于后续输入
ed.Bold(); ed.Italic(); ed.Underline(); ed.Strike(); ed.Code()
ed.SetColor("#f87171"); ed.SetSize(15)
ed.Undo        // *ui.UndoStack（Ctrl+Z / Ctrl+Y 已接）
```

`RichRun` = `{Text, Style}`，`RichStyle` = `{Bold, Italic, Underline, Strike, Code, Color, Size, Link}`。

调试 / 导出：

```go
ui.RichEditorText(ed)    // 带标记的纯文本："**粗** _斜_"
ui.RichEditorDebug(ed)   // 结构快照
```

## 3. 编辑器已内建的行为

* 点击定位光标、拖选、双击选词、Shift+方向键扩选
* 输入法组合串（与 `TextSel` 同一套 IME 处理）
* Home / End / 左右 / Backspace / Delete
* 有选区时输入会替换选区
* 样式切换：有选区改选区，无选区改「后续输入」

## 4. 撤销栈

```go
type UndoEntry struct{ Label string; Undo, Redo func() }

u := ui.NewUndo()                 // 默认 100 步上限
u.Do("加粗", redoFn, undoFn)       // 执行并记录
u.Push("配色", undoFn, redoFn)     // 只记录
u.BeginTx("批量替换"); u.Tx(undo, redo); u.CommitTx()
u.Undo(); u.Redo(); u.Label(); u.History(); u.Clear()
```

## 5. 用法对照

| 场景 | 选择 |
| --- | --- |
| 一段带颜色的说明文字 / 状态行 | `RichText` + `TextSpan` |
| 需要链接可点击 | `RichText` + `OnSpanClickEvt` |
| 简单多行纯文本 | `ui.Textarea` |
| 邮件正文 / 备注（带粗斜体） | `RichEditorEx` |
| 大段文档（标题、列表、代码块） | `ui.Markdown`（见 42 章） |
| 只读长文可拖选复制 | `ui.SelectionArea` + `ui.RichText` |

## 6. 示例

`cmd/gallery/pages/case_settings.go` / `case_markdown.go` 里有 `RichText` 与 `RichEditor`
的可运行组合；`ui/richtext_editor.go` 是完整实现（约 700 行，可直接读）。
