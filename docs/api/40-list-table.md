<!-- group: 数据展示 -->
# 列表 / 表格 / 树 · ListRows / Table / TableV / VList / FileTree

## 1. ListRows 列表行

```go
ui.ListRows([]ui.ListRowItem{
    {Key: "a", Title: "设计稿.fig", Sub: "12.4 MB · 今天", Tag: "已同步", TagColor: ui.ColSuccess,
     Glyph: "✓", Color: ui.ColPrimary},
    {Key: "b", Title: "需求.docx", Sub: "3.1 MB · 昨天", Checked: true},
}, ui.ListRowsOptions{
    Key:      "files",
    Selected: "a",
    Dividers: true,     // 行间分隔线
    CheckBox: true,     // 显示多选框
    Compact:  true,     // 紧凑行（36 高）
    OnPick:   func(it ui.ListRowItem, i int) { sel = it.Key },
    OnCheck:  func(it ui.ListRowItem, i int, on bool) { toggle(it.Key, on) },
})
```

## 2. 简单表格 Table

```go
ui.Table(headers, widths, rows, onRow)
// headers: []string
// widths:  []float32（0 = 弹性列）
// rows:    [][]*ui.Node（每行单元格节点）
// onRow:   整行点击回调（可为 nil）
```

斑马纹、表头、固定/弹性列宽由组件处理。

## 3. 虚拟表格 TableV（大数据量首选）

```go
ui.TableV(ui.TableVConfig{
    Key:      "orders",
    Headers:  []string{"订单号", "客户", "金额", "状态"},
    Widths:   []float32{140, 0, 110, 90},   // 0 = 弹性列
    RowH:     34,
    Height:   420,
    Count:    len(orders),
    Row: func(i int) []*ui.Node {
        o := orders[i]
        return []*ui.Node{ui.Text(o.ID), ui.Text(o.Customer), ui.Text(o.Amount), statusTag(o)}
    },
    Groups: []ui.TableGroup{{Label: "本月", Start: 0, Count: 12}},  // 可折叠分组
    Footer: []*ui.Node{ui.Text("合计"), nil, ui.Text(total), nil},
    Resizable: true,                     // 表头分隔条拖拽调列宽（持久化到偏好）
    SortCol: sortCol, SortAsc: sortAsc,
    OnSort:  func(col int) { sortCol, sortAsc = cycle(col) },
    OnRow:   func(i int) { open(i) },
})
ui.TableColWidth("orders", 2)             // 读取拖拽后的列宽
ui.SetTableColWidth("orders", 2, 160)
```

只渲染可见行，几万行也不卡。

## 4. 虚拟列表 VList

```go
ui.VList("logs", len(lines), 24, 360, func(i int) *ui.Node {   // 定行高
    return ui.Text(lines[i])
})

ui.VListOpt("logs2", len(lines), func(i int) *ui.Node { return row(i) },
    ui.VListOptions{RowHeight: 26, Height: 400, Overscan: 6, Gap: 2, Empty: ui.Empty("暂无", "")})

ui.VListVar("mix", heights, 420, func(i int) *ui.Node { return block(i) })  // 变行高

first, last := ui.VListVisible(len(lines), ui.ScrollOffset("logs"), 360, 24, 4)
```

## 5. 拖拽重排 ReorderList

```go
ui.ReorderList(ui.ReorderListOptions{
    Key: "queue", Count: len(items), RowH: 40, Height: 360,
    Row:    func(i int) *ui.Node { return row(items[i]) },
    OnMove: func(from, to int) { items = ui.MoveSlice(items, from, to) },
})
```

固定行高、整行拖动，插入位置高亮，松手回调 `OnMove`。

## 6. 文件树 FileTree

```go
nodes := []ui.TreeNode{
    {Key: "ui", Label: "ui", Kind: ui.TreeFolder, Children: []ui.TreeNode{
        {Key: "ui/app.go", Label: "app.go", Kind: ui.TreeFile, Tag: "改", TagColor: ui.ColWarn},
        {Key: "ui/layout.go", Label: "layout.go", Kind: ui.TreeFile},
    }},
    {Key: "README.md", Label: "README.md", Kind: ui.TreeLink},
}
ui.FileTree(nodes, ui.TreeOptions{
    Key:      "files",
    Expanded: []string{"ui"},      // 仅首次构建生效，之后由点击记忆
    Selected: "ui/app.go",
    Indent:   16,
    RowH:     30,
    OnPick:   func(n ui.TreeNode) { open(n.Key) },
    OnToggle: func(n ui.TreeNode, open bool) {},
})
ui.TreeSetAllOpen("files", nodes, true)   // 工具条：全部展开 / 收起
ui.TreeSetOpen("files", "ui", false)
ui.TreeSelected("files")
ui.TreeSetSelected("files", "README.md")
```

`Kind`：`TreeFolder / TreeFile / TreeLink`（决定图标）；`Tag` 是行右侧小标签（如「12」「已改」）。
折叠的子树**不构建节点**，所以上万节点的目录树也很轻。

## 7. 聊天列表

聊天行没有独立组件，用 `ListRows` + 气泡（`Stack` + 圆角 + 左右对齐）即可；
`gallery` 的 `case_chat.go` 是完整示例（头像、时间、左右气泡、输入条）。

## 8. 分页器

```go
ui.Pager(ui.PagerOptions{
    Key: "list", Page: page, Total: 9, Window: 2,   // 当前页左右各显示几页
    Locked: 6,                                       // 前 6 页之后上锁（锁标 + 不可点）
    OnPick: func(p int) { page = p },
})
```

注意：`Pager` 内部用 `Wrap` 排版并显式给了总宽；放进 `Row` 时不要再包一层会让它测量无限的容器。

## 9. 性能对照表

| 数据量 | 该用什么 |
| --- | --- |
| ≤ 200 行，行内容简单 | `Table` / `ListRows` |
| 上千行 | `TableV` / `VList` |
| 变行高长列表 | `VListVar` |
| 需要拖拽排序 | `ReorderList` |
| 目录树 | `FileTree`（折叠即不构建） |
| 卡片流 | `MediaCards`（`Wrap` 自动换行，见 41 章） |
