// guis — 图表交互状态与 SVG 导出回归

package ui

import (
	"strings"
	"testing"
)

// devChartNode 造一个图表节点（交互状态按 Key 存放）。
func devChartNode(key string, c *Chart) *Node {
	return &Node{Kind: KChart, Key: key, Chart: c}
}

// newLineChart 3 系列 × n 点的折线图。
func newLineChart(n int) *Chart {
	c := &Chart{Type: ChartLine, ShowLegend: true}
	for s := 0; s < 3; s++ {
		vals := make([]float64, n)
		for i := range vals {
			vals[i] = float64(s*10 + i)
		}
		c.Series = append(c.Series, Series{Name: string(rune('A' + s)), Values: vals})
	}
	labels := make([]string, n)
	for i := range labels {
		labels[i] = "L" + string(rune('0'+i%10))
	}
	c.Labels = labels
	return c
}

// TestChartViewSlice：缩放只影响「有效图表」，原图表数据不变。
func TestChartViewSlice(t *testing.T) {
	const key = "test-zoom"
	t.Cleanup(func() { ChartClearInteract(key) })
	c := newLineChart(10)
	n := devChartNode(key, c)

	if got := chartEffective(n); got != c {
		t.Fatal("无交互状态时应原对象直通")
	}

	ChartZoom(key, 0.5, 0.5)
	if span, start := ChartZoomOf(key); span != 0.5 || start != 0.5 {
		t.Fatalf("缩放状态错误：span=%.2f start=%.2f", span, start)
	}
	got := chartEffective(n)
	if got == c {
		t.Fatal("有缩放状态时应返回副本")
	}
	for i, s := range got.Series {
		if len(s.Values) != 5 {
			t.Fatalf("第 %d 条系列应裁到 5 个点，实际 %d", i, len(s.Values))
		}
		if s.Values[0] != c.Series[i].Values[5] {
			t.Fatalf("第 %d 条系列裁剪起点错误：%.1f != %.1f", i, s.Values[0], c.Series[i].Values[5])
		}
	}
	if len(got.Labels) != 5 {
		t.Fatalf("标签应同步裁剪到 5 个，实际 %d", len(got.Labels))
	}
	if len(c.Series[0].Values) != 10 || len(c.Labels) != 10 {
		t.Fatal("原图表数据不能被修改")
	}
}

// TestChartZoomClamp：越界参数收敛到合法窗口。
func TestChartZoomClamp(t *testing.T) {
	const key = "test-clamp"
	t.Cleanup(func() { ChartClearInteract(key) })
	ChartZoom(key, 0.05, 0.99)
	span, start := ChartZoomOf(key)
	if span != 0.05 || start < 0.94 || start > 0.9501 {
		t.Fatalf("应收敛为 span=0.05 start=0.95，实际 span=%.2f start=%.3f", span, start)
	}
	ChartZoom(key, 1.5, -3) // 非法 span：回到全量
	if span, start = ChartZoomOf(key); span != 1 || start != 0 {
		t.Fatalf("非法 span 应收敛为全量，实际 span=%.2f start=%.2f", span, start)
	}
}

// TestChartSeriesHide：隐藏系列只影响绘制，原数据完好。
func TestChartSeriesHide(t *testing.T) {
	const key = "test-hide"
	t.Cleanup(func() { ChartClearInteract(key) })
	c := newLineChart(4)
	n := devChartNode(key, c)

	if hidden := ChartToggleSeries(key, 1); !hidden {
		t.Fatal("首次切换应变为隐藏")
	}
	if !ChartSeriesHidden(key, 1) {
		t.Fatal("隐藏状态没记住")
	}
	got := chartEffective(n)
	if got == c {
		t.Fatal("有隐藏系列时应返回副本")
	}
	if len(got.Series) != 3 {
		t.Fatalf("系列数量不能变（图例下标要稳定），实际 %d", len(got.Series))
	}
	if got.Series[1].Values != nil || got.Series[1].Candles != nil {
		t.Fatal("被隐藏的系列数据应清空（绘制跳过）")
	}
	if !strings.Contains(got.Series[1].Name, "B") {
		t.Fatalf("隐藏的是第 2 条（B），实际 %q", got.Series[1].Name)
	}
	if len(c.Series[1].Values) != 4 {
		t.Fatal("原图表数据不能被改")
	}

	if hidden := ChartToggleSeries(key, 1); hidden {
		t.Fatal("再次切换应恢复显示")
	}
	if got := chartEffective(n); len(got.Series) != 3 || got != c {
		t.Fatal("取消隐藏后应回到原对象直通")
	}
}

// TestChartSVG 折线图导出：结构完整、标题转义、点数正确。
func TestChartSVG(t *testing.T) {
	c := newLineChart(6)
	c.Title = `销量 <top> & "增长"`
	svg := c.SVG(400, 240)
	if !strings.HasPrefix(svg, "<svg") {
		t.Fatal("导出内容应以 <svg 开头")
	}
	if !strings.Contains(svg, "</svg>") {
		t.Fatal("导出内容缺少闭合标签")
	}
	if strings.Contains(svg, "<top>") {
		t.Fatal("标题中的尖括号必须转义")
	}
	if !strings.Contains(svg, "&lt;top&gt;") {
		t.Fatal("标题转义结果不对")
	}
	if n := strings.Count(svg, "<polyline"); n != 3 {
		t.Fatalf("3 条系列应有 3 条 polyline，实际 %d", n)
	}
	// 每条折线应包含 6 个坐标点（x,y 各一逗号）
	line := svg[strings.Index(svg, "<polyline"):]
	line = line[:strings.Index(line, "/>")]
	if got := strings.Count(line, ","); got != 6 {
		t.Fatalf("折线应有 6 个坐标点，实际 %d", got)
	}
	if !strings.Contains(svg, "L0") && !strings.Contains(svg, "L5") {
		t.Fatal("X 轴标签应出现在导出内容里")
	}
}

// TestChartSVGCandles：K 线导出包含影线与实体。
func TestChartSVGCandles(t *testing.T) {
	c := &Chart{Type: ChartCandle}
	c.Series = []Series{{Name: "K", Candles: []Candle{
		{Open: 10, High: 12, Low: 9, Close: 11},
		{Open: 11, High: 13, Low: 10, Close: 10.5},
	}}}
	svg := c.SVG(320, 200)
	if got := strings.Count(svg, "<rect"); got < 2 {
		t.Fatalf("每根蜡烛应有实体矩形，实际 rect 数 %d", got)
	}
	if !strings.Contains(svg, "<line") {
		t.Fatal("缺少影线")
	}
}

// TestChartSVGUnsupported：未支持的图型给出明确占位，不 panic。
func TestChartSVGUnsupported(t *testing.T) {
	c := &Chart{Type: ChartPie}
	c.Series = []Series{{Name: "A", Values: []float64{1}}, {Name: "B", Values: []float64{2}}}
	svg := c.SVG(200, 160)
	if !strings.Contains(svg, "暂不支持") {
		t.Fatal("饼图应给出降级提示")
	}
	if strings.Count(svg, "<svg") != 1 {
		t.Fatal("仍应是合法 SVG 文档")
	}
}

// TestChartSVGEmpty：空数据不 panic，且没有 polyline。
func TestChartSVGEmpty(t *testing.T) {
	c := &Chart{Type: ChartLine}
	svg := c.SVG(0, 0)
	if !strings.Contains(svg, "<svg") {
		t.Fatal("空数据也应产出 SVG 骨架")
	}
	if strings.Contains(svg, "<polyline") {
		t.Fatal("空数据不应画折线")
	}
}
