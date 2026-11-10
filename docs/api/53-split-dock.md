<!-- group: 容器与浮层 -->
# 分栏与停靠布局 · Split / Dock / 更多展示组件

## 1. 可拖动分栏 Split

```go
ui.Split("main-split", leftPane(), rightPane(), ui.SplitOptions{
    Dir:      ui.Horizontal,   // Horizontal = 左右；Vertical = 上下
    Pos:      0.28,            // 初始比例 0..1（0 = 0.28）
    MinA:     180,             // 左侧最小尺寸
    MinB:     320,
    HandleW:  8,               // 分隔条厚度
    PrefsKey: "main-split",    // 非空则把比例写入偏好，重启恢复
    OnChange: func(pos float32) {},
})
ui.SplitPos("main-split")                       // 当前比例
ui.SetSplitPos("main-split", 0.18, "main-split") // 代码里「收起侧栏」
```

拖动手柄时会持续回调 `OnChange`，拖动结束自动持久化。

## 2. 停靠布局 Dock

```go
var dock = ui.NewDock("ide")   // DockState：跨帧 + 持久化

panels := map[string]ui.DockPanel{
    "files":  {Title: "资源管理器", Icon: "▤", Content: filesView},
    "out":    {Title: "输出", Icon: "≡", Badge: "3", Content: outView},
    "props":  {Title: "属性", Icon: "⚙", Content: propsView},
}

ui.DockLayout(dock, centerEditor(), panels)
```

`DockState`：

```go
type DockState struct {
    Key                        string
    Left, Right, Bottom        []string            // 各侧面板 ID 顺序
    LeftW, RightW, BottomH     float32             // 各侧尺寸
    Collapsed                  map[string]bool     // 折叠态
    Active                     map[string]string   // 各区域当前页签（left/right/bottom）
}
dock.TogglePanel("files")          // 面板显示/折叠（侧栏首个面板折叠时整侧收起）
dock.Activate("right", "props")    // 切换活动页签
dock.Save()                        // 手动持久化（拖动结束会自动存）
```

## 3. 轮播 Carousel

```go
ui.Carousel("hero", []*ui.Node{slide1, slide2, slide3}, ui.CarouselOptions{
    Height:   260,
    Autoplay: true,
    Interval: 4,     // 秒
    Loop:     true,
    Arrows:   true,
    Dots:     true,
})
```

## 4. 时间线 / 步骤 / 详情 / 指标

```go
ui.Timeline([]ui.TimelineItem{
    {Time: "09:12", Title: "创建订单", Sub: "系统自动", Color: ui.ColPrimary},
    {Time: "10:03", Title: "支付成功", Sub: "微信支付", Color: ui.ColSuccess,
     Body: ui.Text("金额 ¥128.00")},
})
ui.Steps([]ui.StepItem{{Title: "下单"}, {Title: "支付"}, {Title: "发货"}, {Title: "完成"}}, 2)
ui.Descriptions([]ui.KV{{K: "订单号", V: "20261104001"}, {K: "客户", V: "张三"}}, 2)
ui.Statistic("gmv", "成交额", 128640, ui.StatOptions{Prefix: "¥", Decimals: 0, Trend: 0.18, Animate: true})
ui.Segmented(keys, labels, active, pick)
```

## 5. 水印 / 省略文本 / 轮播外的小件

```go
ui.Watermark("内部资料", contentNode)          // 平铺低透明度文字，不拦截交互
ui.EllipsisText("e1", longText, 2, 320, ui.EllipsisOptions{
    Size: 13, Weight: 400, Color: ui.ColTextDim, LineH: 1.6, Expandable: true,
})
ui.Carousel("c", slides, opts)
```

## 6. 头像组 / 徽章 / 标签（补充）

```go
ui.AvatarGroup([]ui.AvatarItem{{Initial: "A", Color: "#6366f1"}, {Src: "b.png"}}, 4)
ui.Badge(12)
ui.Tag("已发布", ui.ColSuccess)
```

## 7. 组合建议：可折叠侧栏 + 主内容

```go
func view() *ui.Node {
    if collapsed {
        ui.SetSplitPos("main", 0, "main")
    }
    return ui.Row(
        ui.Column(
            ui.IconButton("≡", func() { collapsed = !collapsed }),
            navList(),
        ).St(ui.St().W(56).Hp(100).BgC(ui.ColBgSoft)),
        ui.Split("main", sidePanel(), mainContent(), ui.SplitOptions{
            Dir: ui.Horizontal, Pos: 0.24, MinA: 200, MinB: 480, PrefsKey: "main",
        }).St(ui.St().GrowTo(1)),
    ).St(ui.St().Hp(100))
}
```

## 8. 关键约束

* `Split` 的两侧都必须是**有确定尺寸**的节点；`MinA/MinB` 用来防止拖到 0。
* `Dock` 的面板 ID 必须与 `panels` map 的键一致，否则该位置渲染为空。
* 两者状态都可持久化（`PrefsKey` / `dock.Save()`），换机器后布局跟着偏好走。
