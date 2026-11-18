<!-- group: 容器与浮层 -->
# 轻提示 / 通知 / 动画

## 1. Toast 轻提示

```go
ui.ToastShow("已保存")            // 默认 ToastDefaultSeconds = 2 秒
ui.ToastShowFor("处理中…", 5)     // 指定秒数（<=0 用默认）
ui.ToastHide()
ui.ToastVisible()
ui.ToastLayer()                   // 根 Stack 末尾（自带淡入淡出，到点播完退场再消失）
```

也可以画一个「贴在容器底部」的静态 Toast 节点（不需要全局状态时）：

```go
ui.Stack(content, ui.Toast("已复制"))
```

## 2. 通知中心 Notify

```go
type Notice struct {
    ID       string
    Kind     string          // NoticeInfo / NoticeSuccess / NoticeWarn / NoticeError
    Title    string
    Body     string
    Icon     string
    Buttons  []NoticeButton  // {Label, Run}
    Progress float32         // 0..1 显示进度条
    Duration float32         // 秒；<=0 用 4 秒；<0 常驻
}

ui.Notify(ui.Notice{ID: "up", Kind: ui.NoticeInfo, Title: "下载中", Body: "更新包 42%", Progress: 0.42, Duration: -1})
ui.NotifyProgress("up", 0.86, "校验中…")     // 更新进度（不存在则创建）
ui.NotifyClose("up")
ui.NotifyClear()
ui.NotifyCount()
ui.NoticeMax(5)                             // 同时最多 5 条，超出淘汰最旧
ui.NotifyLayer()                            // 根 Stack 末尾；右上角堆叠，悬停暂停过期
ui.NoticeText(n)                            // 纯文本摘要（调试 / 朗读）
```

带按钮的通知：

```go
ui.Notify(ui.Notice{
    ID: "file", Kind: ui.NoticeSuccess, Title: "导出完成", Body: "报表.xlsx",
    Duration: 8,
    Buttons: []ui.NoticeButton{
        {Label: "打开", Run: func() { ui.ShellOpen(path) }},
        {Label: "定位", Run: func() { ui.RevealInExplorer(path) }},
    },
})
```

## 3. 动画原语

```go
v := ui.Tween("panel-w", 320, 0.24, ui.EaseOutCubic)          // 补间（按 key 记忆）
v  = ui.TweenFrom("enter", 0, 1, 0.3, ui.EaseOutBack)         // 首次以 initial 为起点（入场）
s  := ui.Spring("sheet", 1, 180, 22)                          // 弹簧（过冲回弹）
x  := ui.Keyframes("pulse", []ui.Key{                         // 关键帧
    {At: 0, Value: 1}, {At: 0.5, Value: 1.06, Ease: ui.EaseOutQuad}, {At: 1, Value: 1},
}, 1.2, true)
ui.ResetAnim("panel-w")
ui.AnimActive()
```

缓动函数：`EaseLinear / EaseInQuad / EaseOutQuad / EaseInOutQuad / EaseInCubic / EaseOutCubic /
EaseInOutCubic / EaseOutQuart / EaseOutBack / EaseOutElastic / EaseOutBounce`。

```go
// 把动画值用在样式上（绘制期位移，不参与布局）
ui.Box(card).St(ui.St().Translate(0, 12*(1-ui.Tween("enter", 1, 0.3, ui.EaseOutCubic))))
```

## 4. FLIP 布局动画（列表重排 / 增删）

```go
ui.FLIP("list-item-3", rowNode, 0.22)                              // 位置/尺寸变化自动过渡
ui.FLIPFrom("row", n, oldX, oldY, 0.18)                            // 用显式旧矩形驱动一次
ui.ResetFLIP("row")
ui.ListEnter("list", i, node)                                      // 前 n 项错峰淡入上移
ui.TabSlide("tab", dir)                                            // 页签切换的滑动位移
```

## 5. 编排与节奏

```go
p := ui.StaggerAt("cards", i, 0.04, 0.3)   // 第 i 项延迟 i*step 开始
p  = ui.SequenceAt("steps", i, 0.2)        // 串行：第 i 段
p  = ui.ParallelAt("all", 0.3)             // 并行
ui.Duration(0.3)                           // 按「减弱动态效果」自动归零
ui.ReducedMotion()                         // 系统是否要求减弱动效
ui.ShimmerPhase(1.6)                       // 骨架屏流光相位 0..1
```

## 6. 数值动画与格式化

```go
ui.AnimatedNumber("kpi", 1280, 0.6, 0)   // 缓动到目标值（KPI / 统计卡）
ui.FormatNumber(12345.678, 2)             // "12,345.68"
```

## 7. 低帧率动画（省 CPU 的关键）

| 动画类型 | 用哪个标记 | 重绘频率 |
| --- | --- | --- |
| 光标闪烁、骨架屏流光、呼吸灯 | `AnimSlowOn()` | ~30Hz |
| 加载环、拖拽跟手、过渡 | `AnimOn()` | 跟消息泵（~120Hz） |
| 一次性过渡 | `Tween/KeepAlive`（自然结束） | 结束后自动停 |

只有低帧率动画时，框架会把「整树重建 + 绘制」节流到 8~12Hz，消息泵仍跑 120Hz——
所以静态页面挂个骨架屏不会把 CPU 打满。`GUIS_FPS=1` 可以看到每帧哪些节点在续帧。

## 8. 无障碍

所有过渡时长都应经过 `ui.Duration(d)`：系统开启「减弱动态效果」时它返回 0，
动画直接到位，既符合规范也避免不适。

## 9. 示例

`cmd/gallery/pages/case_suite.go` 里集中演示了 Toast / Notify / 引导 / 骨架屏 / FLIP /
错峰入场（`--case basic` 或 `--case suite` 打开）。

## 10. 组件动画（绑定到节点）

`Node.Anim(...)` 给任意节点绑一组效果：绘制期压一层顶点变换，位移 / 缩放 / 旋转 /
透明度一起作用到文字、图标、图片与底板（见 `gpu.PushAnim`），布局完全不受影响。

```go
// 入场（一次性，节点首次出现时开始）
ui.Card(...).Anim(ui.SlideIn(ui.SideBottom, 24, 0.3), ui.FadeIn(0.3))
ui.Dialog(...).Anim(ui.PopIn(0.35))
ui.Row(...).Anim(ui.FlipInX(0.3))          // 横展开
ui.Row(...).Anim(ui.RotateIn(0.3))         // 从 -20° 转正
ui.Icon(...).Anim(ui.Spin(1.2))            // 循环旋转（一圈 1.2 秒）
ui.Bell(...).Anim(ui.Swing(0.6))           // 绕上边缘摆动
ui.Btn(...).Anim(ui.Jello(0.5))            // 果冻
ui.HeartBadge.Anim(ui.Heartbeat(0.8))      // 心跳
ui.KPI.Anim(ui.Tada(0.7))                  // 强调
ui.Led.Anim(ui.Flash(0.6))                 // 闪烁提示
ui.Panel.Anim(ui.DropIn(28, 0.45))         // 落下（自带回弹）
ui.Toolbar.Anim(ui.RubberBand(0.6))        // 橡皮筋
ui.Dot.Anim(ui.Pulse(1.4).Amount(0.08))    // 呼吸（循环）
ui.Icon.Anim(ui.FloatY(6, 2))              // 漂浮（循环）
if bad { ui.Field(...).Anim(ui.Shake(0.4)).Replay() }  // 抖一下（可重复触发）
```

出场：`*Out` 系列从终态退到 0，配合 `Present` 保留期使用——关闭后再留一会儿，
退场动画播完才真正消失（Dialog / Drawer / Modal / Toast / Tooltip 已内置这套做法）：

```go
if ui.Present("dlg", open, 0.2) {                       // 关闭后仍返回 true 0.2 秒
    ui.ModalPanel(o).Anim(ui.PopIn(0.2)).AnimIf(!open, ui.PopOut(0.2))
}
node.AnimIf(cond, ui.FadeOut(0.2), ui.SlideOut(ui.SideBottom, 16, 0.2))
ui.PresentLeaving("dlg")                                // 是否正处于退场期
```

其它接口：`AnimIf / NoAnim / Replay / HasAnim`、微调 `DelayBy / With / Amount /
From / Distance / At / Angle / Pivot`、全局清空 `ui.ResetWidgetAnims()`。
强调类效果（`Shake / Bounce / Swing / Jello / Heartbeat / Tada / Flash / RubberBand`）
默认只播一次、播完收幅停在终态；想让它一直动就加 `.LoopOn()`（`.Once()` 改回单次）：

```go
ui.Icon(...).Anim(ui.Swing(0.8).LoopOn())     // 一直摆
ui.Led(...).Anim(ui.Flash(1.2).LoopOn())      // 一直闪
```

系统开启「减弱动态效果」时所有效果直接落到终态。

