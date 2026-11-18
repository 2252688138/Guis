// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package pages

import (
	"math"
	"time"

	"guis/ui"
)

// 追加的一批图表：水平条形 / 直方图 / 玫瑰 / 气泡 / 瀑布 /
// 箱线 / 分时 / 热力 / 漏斗 / 仪表盘。

var (
	chartBarH   *ui.Chart
	chartHist   *ui.Chart
	chartRose   *ui.Chart
	chartBubble *ui.Chart
	chartWater  *ui.Chart
	chartBox    *ui.Chart
	chartShare  *ui.Chart
	chartHeat   *ui.Chart
	chartFunnel *ui.Chart
	chartGauge  *ui.Chart
)

// chartTimes 分时图的横轴时间标签。
func chartTimes(n int) []string {
	out := make([]string, n)
	start := time.Date(2024, 3, 1, 9, 30, 0, 0, time.Local)
	for i := range out {
		out[i] = start.Add(time.Duration(i) * 15 * time.Minute).Format("15:04")
	}
	return out
}

// ensureChartsExtra 初始化并刷新追加图表的示例数据。
func ensureChartsExtra() {
	if chartBarH == nil {
		chartBarH = ui.NewChart(ui.ChartBarH, []string{"东北", "西南", "华北", "华南", "华东", "西北", "华中"})
		chartBarH.Title = "区域销量排行"
		chartBarH.Unit = " 万"
		chartBarH.OnPick = func(s, i int) { pickChart("排行条", chartBarH, s, i) }

		chartHist = ui.NewChart(ui.ChartHistogram, nil)
		chartHist.Title = "响应耗时分布"
		chartHist.Bins = 14
		chartHist.Unit = " 次"

		chartRose = ui.NewChart(ui.ChartRose, []string{"搜索", "直推", "社交", "外链", "邮件"})
		chartRose.Title = "渠道构成"
		chartRose.ShowLegend = true
		chartRose.OnPick = func(s, i int) { pickChart("玫瑰图", chartRose, s, i) }

		chartBubble = ui.NewChart(ui.ChartBubble, nil)
		chartBubble.Title = "投入 / 产出 / 规模"
		chartBubble.Unit = " 万"

		chartWater = ui.NewChart(ui.ChartWaterfall, []string{"期初", "新签", "续费", "增购", "退款", "成本", "期末"})
		chartWater.Title = "收入构成（瀑布）"
		chartWater.Unit = " 万"
		chartWater.OnPick = func(s, i int) { pickChart("瀑布", chartWater, s, i) }

		chartBox = ui.NewChart(ui.ChartBox, []string{"A 组", "B 组", "C 组", "D 组"})
		chartBox.Title = "各组耗时分布"
		chartBox.Unit = " ms"
		chartBox.OnPick = func(s, i int) { pickChart("箱线", chartBox, s, i) }

		chartShare = ui.NewChart(ui.ChartTimeshare, chartTimes(24))
		chartShare.Title = "分时走势"
		chartShare.Min = 1000 // 基准线（昨收）
		chartShare.Unit = ""

		chartHeat = ui.NewChart(ui.ChartHeatmap, nil)
		chartHeat.Title = "活跃热力（12 列）"
		chartHeat.Cols = 12

		chartFunnel = ui.NewChart(ui.ChartFunnel, []string{"曝光", "点击", "加购", "下单", "支付"})
		chartFunnel.Title = "转化漏斗"
		chartFunnel.ShowLegend = true
		chartFunnel.OnPick = func(s, i int) { pickChart("漏斗", chartFunnel, s, i) }

		chartGauge = ui.NewChart(ui.ChartGauge, nil)
		chartGauge.Title = "健康度"
		chartGauge.Unit = "%"
		chartGauge.Max = 100
	}

	ph := chartPhase
	chartBarH.Series = []ui.Series{
		{Name: "本月", Color: "#6366f1", Values: []float64{320, 210, 480, 540, 610, 180, 240}},
		{Name: "上月", Color: "#22d3ee", Values: []float64{280, 190, 420, 505, 560, 150, 260}},
	}
	chartHist.Series = []ui.Series{
		{Name: "P50", Color: "#6366f1", Values: histSamples(140, 620, ph)},
	}
	chartRose.Series = pieSeriesVals([]float64{42, 26, 18, 10, 6}, chartRose.Labels)
	chartBubble.Series = []ui.Series{
		{Name: "区域 A", Color: "#34d399", X: wave(30, 12, 18, ph), Values: wave(60, 24, 18, ph+1.1), Z: wave(14, 9, 18, ph+2.3)},
		{Name: "区域 B", Color: "#a78bfa", X: wave(58, 16, 18, ph+0.7), Values: wave(78, 20, 18, ph+2.9), Z: wave(18, 11, 18, ph+0.4)},
	}
	chartWater.Series = []ui.Series{
		{Name: "收入", Values: []float64{1200, 380, 240, 90, -160, -430, 0}},
	}
	chartBox.Series = []ui.Series{
		{Name: "A 组", Values: []float64{120, 180, 240, 320, 420}},
		{Name: "B 组", Values: []float64{160, 220, 300, 380, 520}},
		{Name: "C 组", Values: boxSamples(90, 260, ph)},
		{Name: "D 组", Values: boxSamples(140, 300, ph+1.7)},
	}
	chartShare.Series = []ui.Series{
		{Name: "价格", Values: wave(1012, 9, 24, ph)},
	}
	chartHeat.Series = []ui.Series{
		{Name: "活跃度", Values: heatSamples(7 * 12)},
	}
	funnelVals := []float64{4200, 1860, 720, 268, 96}
	if len(chartFunnel.Series) == len(funnelVals) && len(chartFunnel.Labels) >= len(funnelVals) {
		for i := range chartFunnel.Series {
			chartFunnel.Series[i] = ui.Series{Name: chartFunnel.Labels[i], Values: []float64{funnelVals[i]}}
		}
	} else {
		chartFunnel.Series = []ui.Series{
			{Name: "曝光", Values: []float64{4200}},
			{Name: "点击", Values: []float64{1860}},
			{Name: "加购", Values: []float64{720}},
			{Name: "下单", Values: []float64{268}},
			{Name: "支付", Values: []float64{96}},
		}
	}
	chartGauge.Series = []ui.Series{
		{Name: "健康度", Values: []float64{72 + 12*math.Sin(ph*0.8)}},
	}
}

// pieSeriesVals 把数值列表转成扇区系列（名字取自 labels）。
func pieSeriesVals(vals []float64, labels []string) []ui.Series {
	out := make([]ui.Series, len(vals))
	for i, v := range vals {
		name := ""
		if i < len(labels) {
			name = labels[i]
		}
		out[i] = ui.Series{Name: name, Values: []float64{v}}
	}
	return out
}

// histSamples 造一批右侧长尾的耗时样本（直方图输入）。
func histSamples(n int, base float64, ph float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		f := float64(i) / float64(n)
		out[i] = base * (0.25 + 2.6*f*f) * (1 + 0.18*math.Sin(f*9+ph))
	}
	return out
}

// boxSamples 造一组带离散度差异的样本（箱线图自动统计用）。
func boxSamples(base, spread, ph float64) []float64 {
	out := make([]float64, 12)
	for i := range out {
		out[i] = base + spread*float64(i)/11 + spread*0.16*math.Sin(float64(i)*1.3+ph)
	}
	return out
}

// heatSamples 造 7×12 的活跃度矩阵。
func heatSamples(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		day := i / 12
		hour := i % 12
		v := 6 + 9*math.Sin(float64(hour)/12*math.Pi) + 3*math.Sin(float64(day)*1.1)
		if v < 0.5 {
			v = 0.5
		}
		out[i] = math.Round(v*10) / 10
	}
	return out
}

// chartsExtraRows 追加图表的展示行。
func chartsExtraRows() []*ui.Node {
	ensureChartsExtra()
	return []*ui.Node{
		ui.Row(
			chartCard("水平条形", "排行 / 分组", chartBarH, 220),
			chartCard("直方图", "样本自动分桶", chartHist, 220),
		).St(ui.St().GapOf(16)),
		ui.Row(
			chartCard("玫瑰图", "角度均分 / 半径编码", chartRose, 220),
			chartCard("气泡图", "第三维半径", chartBubble, 220),
		).St(ui.St().GapOf(16)),
		ui.Row(
			chartCard("瀑布图", "累计增减", chartWater, 220),
			chartCard("箱线图", "五数概括", chartBox, 220),
		).St(ui.St().GapOf(16)),
		ui.Row(
			chartCard("分时图", "基准线填充", chartShare, 220),
			chartCard("热力图", "12 列矩阵", chartHeat, 220),
		).St(ui.St().GapOf(16)),
		ui.Row(
			chartCard("漏斗图", "逐层转化", chartFunnel, 220),
			chartCard("仪表盘", "270° 弧", chartGauge, 220),
		).St(ui.St().GapOf(16)),
	}
}
