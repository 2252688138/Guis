<!-- group: 基础组件 -->
# 基础组件 · 文本 / 按钮 / 标签 / 卡片 / 进度 / 开关

## 1. 文本与标题

```go
ui.Text("正文")                                     // 单一样式文本
ui.Text("自动折行").St(ui.St().WrapOn())            // 按测量宽度折行
ui.Rich("同 Text 的语义别名")
ui.Title("区块标题", "副标题说明")                  // 带副标题的区块头
ui.KeyValue("版本", "v1.2.3")                       // 左标题右值一行
ui.Link("查看文档", func() { ui.ShellOpen(url) })   // 文字链接
ui.Divider()                                        // 分隔线
```

## 2. 按钮

```go
ui.Btn("保存", ui.BtnPrimary, save)      // 主色
ui.Btn("取消", ui.BtnSecondary, cancel)
ui.Btn("更多", ui.BtnGhost, more)
ui.Btn("删除", ui.BtnDanger, del)
ui.Btn("完成", ui.BtnSuccess, done)
ui.Button("无变体按钮", fn)               // 最朴素的按钮
ui.IconButton("+", fn)                    // 方形图标按钮（建议用 + - × → ← ↑ ↓ 等有字形符号）
```

`ButtonKind`：`BtnPrimary / BtnSecondary / BtnGhost / BtnDanger / BtnSuccess`。
文字默认垂直居中（`CenterY`），禁用态用 `.DisabledOn(true)`。

## 3. 标签 / 徽章 / 头像 / 状态点

```go
ui.Tag("Beta", ui.ColWarn)        // 标签：文字 + 底色
ui.Badge(3)                       // 角标数字（父节点需为 Stack，附在右上角）
ui.Avatar("舒", ui.ColPrimary)    // 圆形头像（文字首字）
ui.Dot(ui.ColSuccess, 8)          // 状态圆点
ui.AvatarGroup([]ui.AvatarItem{   // 头像叠放组，超出 max 显示 +N
    {Initial: "A", Color: "#6366f1"},
    {Src: "a.png"},
}, 4)
```

## 4. 容器与卡片

```go
ui.Card(ui.Title("统计", ""), ui.Text("12,480")).
    St(ui.St().W(220).Padding(16).R(14).BgC(ui.ColBgSoft).Stroke(1, ui.ColStroke))
ui.Empty("暂无数据", "点右上角新建")     // 空状态占位
ui.Banner("warn", "磁盘空间不足", "只剩 120 MB", onClose)  // 信息条（左侧色条 + 关闭）
ui.Statistic("kpi", "今日活跃", 1280, ui.StatOptions{Suffix: " 人", Trend: 0.12, Animate: true})
```

## 5. 进度与加载

```go
ui.Progress(0.62, ui.ColPrimary)          // 进度条（宽度由父容器决定）
ui.Ring(0.4, 56, ui.ColPrimary)           // 环形进度（圆点分段）
ui.Spinner(18, ui.ColPrimary)             // 旋转加载指示器
ui.Skeleton(160, 12)                      // 骨架块（呼吸 + 流光）
ui.SkeletonText(3, 240)                   // 多行骨架
ui.SkeletonCard(220)                      // 卡片骨架
ui.LoadingOverlay(loading, "加载中…")      // 覆盖父容器的加载遮罩（拦截点击）
ui.EmptyState("▣", "没有结果", "换个关键词试试", "重置", reset)
ui.ResultPage("success", "提交成功", "我们会在 24 小时内审核", ui.Btn("返回", ui.BtnGhost, back))
ui.AsyncSection(loading, errMsg, isEmpty, retry, func() *ui.Node { return listView() })
```

`AsyncSection` 是最省事的三态容器：加载中 / 错误（带重试）/ 空 / 正常内容。

## 6. 开关 Switch（6 种预设 + 完全自定义）

```go
ui.Switch("on", on, func(v bool) { on = v })                   // 默认样式
ui.SwitchSized("sm", ui.SwitchSmall, on, nil)                  // 尺寸预设
ui.SwitchIcon("ico", "☀", on, nil)                             // 左侧图标 + 胶囊轨道
ui.SwitchSparkle("boost", on, nil)                             // ✦ 紫色发光胶囊
ui.SwitchLabeled("blur", "ON", "OFF", on, nil)                 // 轨道内文字
ui.SwitchGradient("g", "#22d3ee", "#818cf8", on, nil)          // 自定义渐变
ui.SwitchDisabled("lock", on)                                  // 禁用态
ui.SwitchDefault("d", on, nil)                                 // 默认样式的 SwitchEx 封装
```

自定义样式（构建器返回副本，可链式组合）：

```go
st := ui.SwitchStyleSparkle().
    WithIconNode(ui.Dot("#22d3ee", 10)).      // 任意节点当图标
    WithLabelStyle("#ffffff", "#c7d2fe", 11)  // 轨道文字色 + 字号
ui.SwitchEx("k1", on, st, func(v bool) { on = v })

st2 := ui.SwitchStyleLabeled("开", "关").WithLabelWidth(14)
ui.SwitchEx("k2", on, st2, nil)

st3 := ui.SwitchStyleDefault().WithLabelNodes(
    ui.Text("✓").St(ui.St().Text(12, 700, "#ffffff")),
    ui.Text("✕").St(ui.St().Text(12, 700, "#94a3b8")),
).WithLabelWidth(12)
ui.SwitchEx("k3", on, st3, nil)
```

可用预设：`SwitchStyleDefault()` / `SwitchStyleSparkle()` / `SwitchStyleLabeled(on, off)` /
`SwitchStyleGradient(from, to)`；定制方法：`WithIcon` / `WithIconNode` / `WithLabels` /
`WithLabelNodes` / `WithLabelStyle` / `WithLabelWidth`。
`SwitchStyle` 字段：`Size / Icon / IconC / OnLabel / OffLabel / OnFrom / OnTo / OffBg / KnobC /
Border / Glow / Boxed / Disabled / IconNode / OnNode / OffNode / LabelW / LabelSize / OnTextC / OffTextC`。
切换动画 160ms 缓动，由框架驱动。

## 7. 选择类小控件

```go
ui.Checkbox("agree", "我已阅读并同意", agree, func(v bool) { agree = v })
ui.Radio("g1", "选项 A", choice == "a", func() { choice = "a" })
ui.Slider("vol", volume, func(v float32) { volume = v })      // 0~1，点击定位 + 拖动跟手
ui.RangeSlider("price", lo, hi, func(l, h float32) { lo, hi = l, h })   // 双滑块 0~1
ui.Rate("stars", 3.5, 5, true, func(v float32) { stars = v })  // allowHalf 支持半星
ui.Chip("标签", active, func() { active = !active })            // 可选中胶囊
ui.ChipRemovable("已选", func() { remove() })                   // 可删除胶囊
```

## 8. 页签与折叠

```go
ui.Tabs([]string{"a", "b"}, []string{"概览", "详情"}, tab, func(k string) { tab = k })      // 分段
ui.TabsBar([]string{"a", "b"}, []string{"代码", "预览"}, tab, pick)                          // 下划线风格
ui.Segmented(keys, labels, active, pick)                                                    // 滑动指示条
ui.Breadcrumb([]string{"首页", "组件", "表格"}, func(i int) { goLevel(i) })
ui.Accordion("acc-1", "高级设置", "折叠内容", open, func() { open = !open })
ui.ListItem("row-1", "标题", "副标题", ui.ColPrimary, func() { open() })
```

## 9. 表格（简单版）

```go
ui.Table(
    []string{"名称", "大小", "修改时间"},
    []float32{0, 90, 140},        // 0 = 弹性列（均分剩余）
    [][]*ui.Node{
        {ui.Text("a.txt"), ui.Text("12 KB"), ui.Text("2026-11-04")},
    },
    func(row int) { selectRow(row) },
)
```

超大表 / 需要虚拟滚动、分组、合计、列宽拖拽时用 `ui.TableV`（见《40 列表 / 表格 / 树》）。

## 10. 轻提示与模态

```go
ui.ToastShow("已保存")                 // 全局轻提示（2 秒）
ui.ToastShowFor("处理中…", 5)
ui.ToastLayer()                        // 放根 Stack 末尾

ui.Modal(open, "标题", "正文", func() { open = false })
ui.ModalEx(open, "标题", "正文", "取消", "确定", onCancel, onOK, onClose, 420)
ui.ModalKey("del-1", open, "确认删除", "不可恢复", "取消", "删除", nil, doDelete, nil, 400)
```

更完整的弹窗（带图标语义 / 自定义正文 / 毛玻璃遮罩 / 抽屉）见《51 浮层 / 弹窗》。

## 11. 悬停提示

```go
ui.TooltipOf(ui.IconButton("?", nil), "查看帮助")
ui.TooltipLayer()          // 放根 Stack 最后一层
ui.SetTooltip("自定义提示", x, y)   // 手动（一般传 ui.MousePos()）
```
