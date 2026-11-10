<!-- group: 事件与交互 -->
# 文本编辑与输入法 · TextSel / IME / 撤销

## 1. 输入控件已经处理好的事

`ui.Input` / `PasswordInput` / `Textarea` / `NumberInput` / `SearchInput` / `Composer` 内部都用
`ui.TextSel` 维护「文本 + 光标 + 选区锚点」，开箱支持：

* 点击定位光标、拖选、双击选中、Shift+方向键扩选
* Home / End / 左右 / Backspace / Delete
* Ctrl+A / C / X / V（走系统剪贴板）
* 输入法组合串（IME）：组合期间显示候选串与光标，回车 / 空格提交
* 垂直居中、光标闪烁（`AnimSlowOn` 低帧率，不吃 CPU）

## 2. TextSel —— 自绘输入框的编辑器状态机

```go
type TextSel struct {
    Runes  []rune
    Caret  int   // 光标（rune 下标）
    Anchor int   // 选区锚点；Anchor != Caret 即有选区
}

ts := ui.NewTextSel("hello", 5, 5)   // 文本, 光标, 锚点（越界自动夹取）
ts.Move(-1, false)                   // 左移一格并折叠选区
ts.Move(3, true)                     // Shift+右：扩选
ts.SetCaret(2, false)
ts.SelectAll()
ts.Insert("世界")                    // 有选区先删选区
ts.Backspace(); ts.DeleteForward()
ts.Copy(); ts.Cut(); ts.Paste(true)  // multiline=false 时把换行替换成空格
lo, hi, ok := ts.Selection()
sel := ts.Selected()
```

坐标 ↔ 光标：

```go
i := ts.IndexAtX(x, 13, 400)                       // 单行：横向偏移 → rune 下标
i  = ts.IndexAtPoint(x, y, 20, 13, 400)            // 多行：坐标 → 下标
```

选区渲染：

```go
segs := ts.SelLineSegments(start, end)   // 一行按选区切成若干片段
nodes := ui.TextSegNodes(ts, start, end, 13, 400, ui.ColText, true)  // 直接出节点
```

## 3. 剪贴板派发约定（Ctrl+A/C/X/V）

自绘文本控件需要登记一个动作处理器，`App` 会在分发快捷键**之前**先把 Ctrl 组合交给它：

```go
init() {
    ui.RegisterTextEdit("my-editor", func(a ui.TextEditAction) bool {
        switch a {
        case ui.TextEditCopy:    /* 复制选区 */ return true
        case ui.TextEditCut:     /* 剪切 */     return true
        case ui.TextEditPaste:   /* 粘贴 */     return true
        case ui.TextEditSelectAll: ts.SelectAll(); return true
        }
        return false
    })
}
ui.TextEditBound("my-editor")   // 是否已登记
ui.DispatchTextEdit(vk)         // 手动派发（一般不用）
```

动作枚举：`TextEditCopy / TextEditCut / TextEditPaste / TextEditSelectAll`。

## 4. 输入法（IME）

```go
ui.ComposingText()    // 当前窗口的组合串（未组合时为空）
ui.ComposingCaret()   // 组合串内的光标位置（rune 数）
```

绘制逻辑：组合串非空时，把候选串画在光标处（带下划线），提交后再插入 `TextSel`。
桌面端由 `SetImePosition` 把候选窗摆到光标附近，浏览器端由 DOM 事件驱动，移动端由软键盘接管。

## 5. 拖选文本（只读内容）

```go
ui.SelectionArea("article", ui.Markdown("md", src))   // 区域内可拖选
ui.SelectedText()      // 当前选中的文字
ui.HasSelection()
ui.ClearSelection()
```

## 6. 撤销 / 重做

```go
u := ui.NewUndo()                      // 默认上限 100 步
u.Do("加一行", func() { add() }, func() { remove() })   // 执行 + 记录
u.Push("改颜色", undoFn, redoFn)       // 只记录（业务已自行完成）
u.Undo(); u.Redo(); u.CanUndo(); u.CanRedo()
u.Label()                              // "加一行"（菜单显示「撤销：加一行」）
u.Clear(); u.History()

// 把多次修改合并成一步
u.BeginTx("批量调整")
u.Tx(undoFn, redoFn)
u.CommitTx()

ui.DefaultUndo()          // 已绑定 Ctrl+Z / Ctrl+Y 的全局栈
ui.UndoHistoryPanel(u)    // 现成的历史面板节点
```

`UndoStack` 非线程安全，只在 UI 线程操作。

## 7. 常用参数与坑

| 现象 | 原因 / 处理 |
| --- | --- |
| 输入框打字无反应 | 没 `FocusOn()`，或 `Key` 与其它控件重复 |
| 拖选拉不出来 | 自己写了 `OnPressEvt` 且每帧重建节点 → 用 `Key` + 框架去重（框架已内建） |
| Ctrl+V 输入了控制字符 | 没登记 `RegisterTextEdit`，字符被当成普通输入 |
| 输入法组合串错位 | 组合串要画在光标位置（用 `ComposingCaret` 定位） |
| 中文截断成半个字 | 光标 / 选区全部按 **rune** 计，不要用 byte 下标 |
