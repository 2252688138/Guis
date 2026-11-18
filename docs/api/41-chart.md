<!-- group: 数据展示 -->
# 图表 · Chart / 18 种图形

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
ui.CandleChart("c8", dates, candles...)  // K 线：OHLC 蜡烛
ui.ChartView("c9", custom)               // 自己配好的 Chart
```

## 3. Chart 字段全表

```go
type Chart struct {
    Type       ChartType         // ChartLine / ChartArea / ChartBar / ChartPie / ChartDonut / ChartRadar / ChartScatter / ChartCandle
    // 扩展：ChartBarH / ChartHistogram / ChartRose / ChartBubble /
    //       ChartWaterfall / ChartBox / ChartTimeshare / ChartHeatmap / ChartFunnel / ChartGauge
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
    CandleUp, CandleDown string  // K 线涨 / 跌配色（默认红涨绿跌）
    MA         []int             // 均线周期，如 {5, 10, 20}（仅 ChartCandle）
    ShowVolume bool              // K 线下方叠量能副图（仅 ChartCandle）
    VolumeRatio float32          // 量能副图高度占比（默认 0.24）
    Bins       int               // 直方图分桶数（默认 12）
    Cols       int               // 热力图列数（默认按样本数开平方）
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
    Candles []Candle     // K 线：开高低收（Values / X 忽略）
}

type Candle struct {
    Open, High, Low, Close float64
    Volume                 float64   // 可选，框架不绘制
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

## 4.1 K 线图（ChartCandle）

```go
dates := []string{"03-01", "03-04", "03-05", "03-06", "03-07"}
candles := []ui.Candle{
    {Open: 100, High: 106, Low: 98, Close: 104},
    {Open: 104, High: 105, Low: 99, Close: 100},
}

k := ui.NewChart(ui.ChartCandle, dates, ui.Series{Name: "演示指数", Candles: candles})
k.MA = []int{5, 10}       // 收盘均线，留空不画
k.CandleUp = "#ef4444"    // 涨（默认红涨绿跌，A 股习惯）
k.CandleDown = "#22c55e"  // 跌
k.OnPick = func(s, i int) { /* 第 i 根蜡烛 */ }
ui.ChartView("kline", k).St(ui.St().Wp(100).H(320))

// 一行出图
ui.CandleChart("kline2", dates, candles...)
```

* 纵轴自动贴合最高 / 最低价并留 4% 余量（**不**从 0 起），显式 `Min` / `Max` 仍优先；
* 蜡烛（影线 + 实体）用原生矩形绘制，零纹理；只有 `MA` 非空时才多生成一张均线纹理；
* 悬浮 / 点击提示给出 `开 / 高 / 低 / 收 + 涨跌幅`；
* 多系列时按 `Series.Candles` 并排分组（均线只叠在第一条系列上）。

## 4.2 扩展类型（10 种）

| 类型 | 构造 | 数据来源 | 绘制 |
| --- | --- | --- | --- |
| `ChartBarH` 水平条形 | `ui.BarHChart(key, labels, series...)` | `Values`；`Labels` 为类目名 | 原生 |
| `ChartHistogram` 直方图 | `ui.HistogramChart(key, series...)` | `Values` 当原始样本，`Bins` 分桶 | 原生 |
| `ChartRose` 瑰图 | `ui.RoseChart(key, labels, series...)` | 每 Series 一个扇区，半径按数值 | 纹理 |
| `ChartBubble` 气泡图 | `ui.BubbleChart(key, series...)` | `X` 横坐标 + `Values` 纵坐标 + `Z` 半径 | 纹理 |
| `ChartWaterfall` 瀑希图 | `ui.WaterfallChart(key, labels, series...)` | `Values` 为增减量，`Min` 是累计起点 | 原生 |
| `ChartBox` 箱线图 | `ui.BoxChart(key, labels, series...)` | 每 Series 一个箱，`Values` 五个数或原始样本 | 原生 |
| `ChartTimeshare` 分时图 | `ui.TimeshareChart(key, labels, series...)` | `Values` 价格，`Min` 为基准线 | 纹理 |
| `ChartHeatmap` 热力图 | `ui.HeatmapChart(key, series...)` | `Values` 行优先矩阵，`Cols` 列数 | 原生 |
| `ChartFunnel` 漏斗图 | `ui.FunnelChart(key, labels, series...)` | 每 Series 一层，宽度按数值 | 纹理 |
| `ChartGauge` 仪表盘 | `ui.GaugeChart(key, series...)` | `Series[0].Values[0]`，`Min`/`Max` 量程 | 纹理 |

```go
// 排行榜（水平条形）+ 样本分布（直方图）
ui.BarHChart("rank", []string{"华东", "华南", "华北"},
    ui.Series{Name: "本月", Values: []float64{610, 540, 480}},
)
ui.HistogramChart("hist", ui.Series{Name: "耗时", Values: samples})

// 转化漏斗（每层一个 Series）
ui.FunnelChart("funnel", []string{"曝光", "点击", "支付"},
    ui.Series{Name: "曝光", Values: []float64{4200}},
    ui.Series{Name: "点击", Values: []float64{1860}},
    ui.Series{Name: "支付", Values: []float64{96}},
)

// K 线 + 量能副图
k.ShowVolume, k.VolumeRatio = true, 0.24
```

* 箱线图的 `Values` 恰好 5 个时按 `[min, Q1, 中位, Q3, max]` 解释，超过 5 个则当样本自动统计；
* 水平条形 / 瀑希 / 箱线 / 热力图走原生图元（零纹理），其余四种走矢量纹理并参与 LRU 缓存；
* 直方图与热力图的 `axisRange` 只用于内部归一化，不画数值轴刻度。

## 5. 绘制与性能

* 图表走**矢量层纹理**：每张图一张 RGBA 纹理，按 LRU 与总预算（`chartTexBudget`）淘汰；
  数据或尺寸不变时不重绘。
* 窗口缩放 / 主题切换 / 数据变更会触发重绘；只改 `Title` 之类也要 `RequestRedraw()`。
* 点击命中：柱、数据点、扇区、K 线蜡烛都会命中（含雷达轴与堆叠柱的分段），回调给出系列下标与数据点下标。
* K 线只有 `MA` 非空时才占用一张纹理，纯蜡烛图完全走原生图元（零纹理内存）。

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

`cmd/gallery/pages/case_charts.go`（前 8 种）与 `case_charts_extra.go`（后 10 种）里 18 种图形都有可运行代码，含 `OnPick`、堆叠、
平滑、环形、雷达与散点的真实数据。
