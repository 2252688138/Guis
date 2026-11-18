<!-- group: 容器与浮层 -->
# 气泡 / 右键菜单 / 日期区间

## 1. Popover 锚点气泡

结构是「整窗遮罩 + 绝对定位面板」：点遮罩关闭，面板自动夹取窗口边界，
下方空间不够时翻到锚点上方。

```go
var pop = ui.NewPopover("filter")
pop.W, pop.H = 240, 180 // W 面板宽；H 用于判断下方空间够不够

// 点击按钮时展开（x, y 是锚点，相对窗口左上角）
btn.OnClick = func() { pop.Toggle(btn.Frame.X, btn.Frame.Y+btn.Frame.H) }

// 页面根 Stack 末尾：
ui.Stack(page, ui.PopoverLayer(pop, filterPanel()))
```

| API | 说明 |
| --- | --- |
| `ui.NewPopover(key)` | 创建状态（默认 220×200） |
| `ShowAt(x, y)` / `Toggle(x, y)` / `Close()` | 展开 / 切换 / 收起 |
| `IsOpen()` | 是否展开 |
| `ui.PopoverLayer(st, content)` | 渲染浮层，未展开返回 nil |

## 2. ContextMenu 右键菜单

基于 `MenuItem`（与 `MenuBar` 同一套：子菜单、分隔线、勾选、禁用），
点击任意菜单项时**先关菜单再执行回调**，不需要自己处理。

```go
var menu = ui.NewContextMenu("row-menu")
row.WithContextMenu(menu, []ui.MenuItem{
    {Label: "打开", OnPick: open},
    {Separator: true},
    {Label: "删除", OnPick: del},
})

// 页面根 Stack 末尾：
ui.Stack(page, ui.ContextMenuLayer(menu))
```

需要自定义弹出位置（例如表格空白处）时，直接调 `menu.At(x, y, items)`。

## 3. DateRange 日期区间

两个 `DateBox` 并排 + 区间合法性提示；日历浮层仍由 `DateLayers()` 统一渲染。

```go
var rng = ui.NewDateRange("order-range") // 内部两个 DateState：-from / -to

ui.DateRangeBox(rng, from, to, func(f, t string) {
    from, to = f, t
})
ui.DateRangeValid("2026-04-01", "2026-05-31") // true
```

开始日期晚于结束日期时只标红提示、**不自动交换**（避免改掉用户正在输入的值）。
