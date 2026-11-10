<!-- group: 数据展示 -->
# 图表 · Chart / 8 种图形

## 1. 三条铁律

1. `*Chart` 由**业务持有**（放 `var` 或结构体里），不要每帧 `NewChart`；
2. 数据变了改 `Chart` 字段并 `ui.RequestRedraw()`；
3. 节点高度默认 280，可用 `St().H(...)` 覆盖。

```go
var sales = ui.NewChart(ui.ChartLine, []string{"1月", "2月", "3月"},
    ui.Series{Name: "线上", Color: "#6366f1", Values: []float64{120, 168, 210}},
    ui.Series{Name: "线下", Color: "#22d3ee", Values: []float64{80, 96, 120}},
)

ui.ChartView("sales", sales).St(ui.St().Wp(100).H(300))
```

## 2. 快捷构造（一行出图）

```go
ui.LineChart("c1", labels, series...)
ui.AreaChart("c2", labels, series...)
ui.BarChart("c3", labels, series...)
ui.PieChart("c4", values, labels)        // values 每项一个扇区
ui.DonutChart("c5", values, labels)      // 内径 58%
ui.RadarChart("c6", labels, series...)
ui.ScatterChart("c7", series...)         // 用 Series.X + Values 指定坐标
ui.ChartView("c8", custom)               // 自己配好的 Chart
```

## 3. Chart 字段全表

```go
type Chart struct {
    Type       ChartType         // ChartLine / ChartArea / ChartBar / ChartPie / ChartDonut / ChartRadar / ChartScatter
    Series     []Series
    Labels     []string          // X 轴标签 / 饼图扇区名 / 雷达轴名
    Title      string
    Unit       string            // 数值后缀（"元"、"%"）
    ShowLegend bool
    ShowGrid   bool
    ShowAxis   bool
    Stacked    bool              // 柱状 / 面积按系列堆叠
    Smooth     bool              // 折线平滑（Catmull-Rom）
    Donut      float32           // 环形内径比例 0~0.9（仅 ChartDonut）
    Min, Max   float64           // Max=0 自动；Min 可为负
    YTicks     int               // 纵轴刻度数（默认 4）
    Padding    float32           // 画布内边距（默认 14）
    OnPick     func(series, index int)   // 点击柱 / 点 / 扇区
}

type Series struct {
    Name   string
    Color  string
    Values []float64
    X      []float64     // 散点图横坐标
}
```

## 4. 组合示例

```go
c := ui.NewChart(ui.ChartBar, months, series...)
c.Stacked = true
c.ShowGrid = true
c.Unit = " 万元"
c.YTicks = 5
c.OnPick = func(s, i int) { ui.ToastShow(fmt.Sprintf("%s / %s: %.0f", series[s].Name, months[i], series[s].Values[i])) }
ui.ChartView("stacked", c).St(ui.St().H(320))
```

饼图数据助手：

```go
ui.PieChart("pie", []float64{38, 26, 20, 16}, []string{"直营", "代理", "线上", "其它"})
// 等价于 Chart{Type: ChartPie, Series: ui.PieSeries(values, labels)}
```

## 5. 绘制与性能

* 图表走**矢量层纹理**：每张图一张 RGBA 纹理，按 LRU 与总预算（`chartTexBudget`）淘汰；
  数据或尺寸不变时不重绘。
* 窗口缩放 / 主题切换 / 数据变更会触发重绘；只改 `Title` 之类也要 `RequestRedraw()`。
* 点击命中：柱、数据点、扇区都会命中（含雷达轴与堆叠柱的分段），回调给出系列下标与数据点下标。

## 6. 无图表时的替代

| 需求 | 轻量做法 |
| --- | --- |
| 迷你趋势线 | `ui.Spectrum` 风格的柱状自绘（`AnimOn`）或 `ui.Ring` |
| 单个比例 | `ui.Ring(v, size, color)` |
| KPI 数字 | `ui.Statistic(...)`（带滚动动效与趋势箭头） |
| 进度对照 | `ui.Progress(v, color)` |

## 7. 平台差异

图表是纯 CPU 光栅 + 一次贴图，桌面 / 浏览器 / 移动端表现一致；
超大图表（几千点）建议先降采样（按像素宽度取最大/最小值桶），否则每帧光栅会拖慢帧率。

## 8. 完整示例

`cmd/gallery/pages/case_charts.go` 里 7 种图形都有可运行代码，含 `OnPick`、堆叠、
平滑、环形、雷达与散点的真实数据。
