<!-- group: 布局与样式 -->
# 节点与布局 · Box / Row / Column / Stack / Grid / Wrap

## 1. 节点模型

```go
type Node struct {
    Kind     Kind      // KBox / KText / KButton / KScroll / KImage / KStack / KGrid / KWrap …
    Key      string    // 交互状态跨帧的标识（输入框、滚动区、开关都必须唯一）
    Style    Style
    Dir      Direction // Vertical / Horizontal
    Align    Align     // 主轴对齐
    Justify  Align     // 交叉轴对齐
    Text     string
    Children []*Node
    // 回调：OnClick / OnClickAt / OnHover / OnPress / OnDrag / OnDragEnd / OnScroll / OnChar / OnKey
    //       OnDrop / OnActivate / OnSpanClick
    // 运行时状态：Hover / Pressed / Focus / ScrollY / ContentH / Frame / Parent（由框架写）
}
```

`Node` 是**声明式**的：`view()` 每帧新建整棵树，框架负责测量 → 定位 → 绘制 → 命中。

## 2. 容器构造器（全部）

| 构造器 | 含义 |
| --- | --- |
| `Box(children...)` | 全能容器（默认纵向、交叉轴拉伸） |
| `Row(children...)` | 横向弹性容器 |
| `Column(children...)` | 纵向弹性容器 |
| `Center(child)` | 居中容器 |
| `Spacer()` | 可伸缩空白（吃掉剩余空间） |
| `Stack(children...)` | 叠层：子节点互相重叠，**后声明的画在上层**（浮层基石） |
| `Scroll(children...)` | 纵向滚动容器（`Key` 必须设置） |
| `Grid(children...)` | 等宽列网格（列数用 `ColsN` / `ColWidth`） |
| `Wrap(children...)` | 流式换行（`flex-wrap`） |
| `Masonry(children...)` | 瀑布流（每项落入当前最矮列） |
| `EqualRow(children...)` / `EqualCol(children...)` | 等宽横向 / 等高纵向分栏 |
| `Abs(child)` | 绝对定位容器（脱离 flex 流） |
| `Pick(xs, sm, md, lg)` | 按视口宽度断点选一个构造函数 |
| `If(cond, fn)` / `Each(items, fn)` | 条件构造 / 列表构造 |

`Each` 是泛型，任何切片都能直接铺：

```go
ui.Column(ui.Each(todos, func(t Todo, i int) *ui.Node {
    return ui.Text(t.Title).K(fmt.Sprintf("todo-%d", i))
})...)
```

## 3. 典型页面骨架

```go
func view() *ui.Node {
    return ui.Stack(
        ui.Column(
            header(),                                   // 固定高
            ui.Scroll(content()).K("scroll-main").St(ui.St().GrowTo(1)),
            footer(),
        ).St(ui.St().Hp(100).BgC(ui.ColBg)),
        ui.ToastLayer(),          // 浮层永远放最后
        ui.TooltipLayer(),
    ).St(ui.St().Hp(100))
}
```

## 4. 尺寸与弹性

```go
ui.Box().St(ui.St().W(180))              // 固定宽
ui.Box().St(ui.St().Wp(50))              // 占父容器 50% 宽
ui.Box().St(ui.St().Hp(100))             // 满高
ui.Box().St(ui.St().GrowTo(1))           // flex-grow: 1 吃掉剩余
ui.Box().St(ui.St().FlexBasis(120))      // flex-basis
ui.Box().St(ui.St().MinWidth(80).MaxWidth(320))
ui.Box().St(ui.St().ColsN(3))            // 网格 3 列
ui.Box().St(ui.St().ColWidth(220))       // 固定列宽（自动算列数）
```

## 5. 对齐

```go
row.St(ui.St()) // 主轴/交叉轴由 Style + 以下方法控制：
row.AlignTo(ui.AlignCenter)   // 主轴对齐
row.JustifyTo(ui.AlignCenter) // 交叉轴对齐
// 取值：AlignStart / AlignCenter / AlignEnd / AlignStretch
//       AlignSpaceBetween / AlignSpaceAround / AlignSpaceEvenly
ui.Box().St(ui.St().Self(ui.AlignStart))   // 单个子项覆盖交叉轴
```

`Row` 的交叉轴默认 **stretch**（与 CSS `align-items: stretch` 一致），
所以 `Row` 里的高卡片会把相邻元素拉高——需要时显式给 `Self(AlignCenter)`。

## 6. 响应式

```go
// ① 断点取常量
size := ui.BP(13, 14, 15, 16)          // <640 / <960 / <1280 / 其余

// ② 断点选节点
ui.Pick(
    func() *ui.Node { return mobileLayout() },   // <640
    func() *ui.Node { return tabletLayout() },   // <960
    func() *ui.Node { return desktopLayout() },  // <1280
    func() *ui.Node { return wideLayout() },     // >=1280
)

// ③ 直接读视口
if ui.ViewportW() < 900 { /* 窄屏精简 */ }
```

## 7. 绝对定位与叠层

```go
ui.Stack(
    heroImage(),
    ui.Box(title).St(ui.St().AbsTL(20, 20)),          // 左上
    ui.Box(badge).St(ui.St().AbsTR(16, 16)),          // 右上
    ui.Box(fab).St(ui.St().AbsBR(24, 24)),            // 右下
    ui.Box(footer).St(ui.St().AbsBottom(16)),         // 水平居中贴底
    ui.Box(center).St(ui.St().AbsCenter()),           // 居中
    ui.Box(scrim).St(ui.St().AbsFill()),              // 铺满（遮罩）
)
```

绝对定位相对**父容器内边距区**；容器需要 `Hp(100)` 之类的确定性尺寸，否则叠层没有可视高度。

## 8. 滚动

```go
ui.Scroll(items...).K("scroll-list").St(ui.St().GrowTo(1))
// 读写偏移
off := ui.ScrollOffset("scroll-list")
ui.ScrollTo("scroll-list", 0)
ui.ScrollBy("scroll-list", 240)
```

滚动条默认可见并会自动淡出；`ScrollbarOn()` 常显，`ui.St().ScrollY()` 仅启用滚动不改其它样式。
滚动容器必须有**稳定唯一的 Key**（状态按 Key 存）。

## 9. Constraints（Flutter 风格）

```go
type Constraints struct{ MinW, MaxW, MinH, MaxH float32 }
ui.Layout(root, w, h, nil)   // 框架内部调用；单测可用 ui.LayoutForTest
```

业务侧不需要手调 `Layout`，只有写布局单元测试（`tools/layouttest`）时才用：

```go
ui.LayoutForTest(view(), 1180, 760, "", 1)
ui.DebugDump(root)   // 打印整棵布局树
```

## 10. 常见坑

* **无子节点且无尺寸的 `Box` 高度为 0**，命中不到、拖不动——`W/H` 要给。
* `Wrap` 放在 `Row` 里测量时主轴无上限，必须显式 `W(...)`，否则会折成一列。
* `Stack` 里没有 `Hp(100)` 的容器不会有高度；遮罩用 `AbsFill()` 前先确认父容器有尺寸。
* 同一界面内 `Key` 必须唯一，否则滚动位置 / 输入光标 / 动画状态会互相串。
