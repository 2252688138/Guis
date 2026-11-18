// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"testing"

	"guis/internal/geom"
)

// TestBarHChart 水平条形：值域含 0、负值向下扩展、命中按类目定位。
func TestBarHChart(t *testing.T) {
	labels := []string{"华北", "华东", "华南"}
	c := NewChart(ChartBarH, labels, Series{Name: "销量", Values: []float64{120, 80, 40}})
	if n := c.points(); n != 3 {
		t.Fatalf("点数应为 3，实际 %d", n)
	}
	lo, hi := c.axisRange()
	if lo > 0 || hi < 120 {
		t.Fatalf("值域应含 0~120：%v ~ %v", lo, hi)
	}

	neg := NewChart(ChartBarH, labels, Series{Values: []float64{50, -30, 10}})
	lo2, hi2 := neg.axisRange()
	if lo2 > -30 || hi2 < 50 {
		t.Fatalf("负值应向下扩展：%v ~ %v", lo2, hi2)
	}

	lg := testLayout()
	lg.yMin, lg.yMax = lo, hi
	lg.plot = geom.Rect{X: 60, Y: 10, W: 180, H: 90}
	// 值 60 落在第 2 个类目（80）的条内
	s, i, ok := barsHHit(c, lg, lg.mapX(60), lg.yAt(1, 3))
	if !ok || s != 0 || i != 1 {
		t.Fatalf("命中应为 (0,1)：ok=%v s=%d i=%d", ok, s, i)
	}
	if lg.mapX(lo) != lg.plot.X {
		t.Fatalf("mapX(lo) 应落在绘图区左边界：%v vs %v", lg.mapX(lo), lg.plot.X)
	}
}

// TestHistogramChart 直方图：自动分桶、计数总量守恒、上限按最大桶。
func TestHistogramChart(t *testing.T) {
	samples := []float64{1, 2, 2, 3, 3, 3, 4, 4, 5}
	c := NewChart(ChartHistogram, nil, Series{Name: "分布", Values: samples})
	if n := c.points(); n != 12 {
		t.Fatalf("默认应为 12 桶，实际 %d", n)
	}

	c.Bins = 4
	if n := c.points(); n != 4 {
		t.Fatalf("Bins=4 应返回 4，实际 %d", n)
	}
	c.updateHistogram()
	total := 0.0
	for _, v := range c.histCounts[0] {
		total += v
	}
	if int(total) != len(samples) {
		t.Fatalf("计数总量应等于样本数：%v vs %d", total, len(samples))
	}
	if len(c.histEdges) != 5 {
		t.Fatalf("桶边界应有 5 个，实际 %d", len(c.histEdges))
	}

	lo, hi := c.axisRange()
	if lo != 0 || hi < 3 {
		t.Fatalf("纵轴应从 0 起到最大桶计数：%v ~ %v", lo, hi)
	}
	if c.histLabel(0) == "" {
		t.Fatal("桶区间文案为空")
	}

	// 绘制取值走桶计数而不是原始样本
	if vals := c.plotValues(0); len(vals) != 4 {
		t.Fatalf("plotValues 应返回桶计数，实际长度 %d", len(vals))
	}
}

// TestRoseChartHit 玫瑰图：角度定位扇区，超出半径不命中。
func TestRoseChartHit(t *testing.T) {
	c := NewChart(ChartRose, []string{"A", "B", "C", "D"},
		PieSeries([]float64{10, 20, 30, 40}, nil)...)
	lg := testLayout()

	if s, _, ok := roseHit(c, lg, lg.pieC.X, lg.pieC.Y-10); !ok || s != 0 {
		t.Fatalf("正上方应为第 0 个扇区：ok=%v s=%d", ok, s)
	}
	if _, _, ok := roseHit(c, lg, lg.pieC.X+lg.pieR*2, lg.pieC.Y); ok {
		t.Fatal("超出半径不应命中")
	}
	if !c.legendVisible() || c.needsTexture() != true {
		t.Fatal("玫瑰图应显示图例并需要纹理")
	}
}

// TestBubbleChartRaster 气泡图：必须画出像素。
func TestBubbleChartRaster(t *testing.T) {
	c := NewChart(ChartBubble, nil, Series{
		Name:   "A",
		X:      []float64{10, 50, 90},
		Values: []float64{20, 60, 40},
		Z:      []float64{5, 9, 3},
	})
	if !c.needsTexture() {
		t.Fatal("气泡图需要纹理")
	}
	lg := testLayout()
	lg.yMin, lg.yMax = 0, 100
	px := c.rasterPlot(200, 110, lg.shifted(lg.plot.X, lg.plot.Y))
	if n := opaque(px); n < 100 {
		t.Fatalf("气泡纹理像素过少：%d", n)
	}
}

// TestHistogramGalleryGeom 复刻 gallery 的右偏样本，打印柱几何与值域，防止越界回归。
func TestHistogramGalleryGeom(t *testing.T) {
	samples := make([]float64, 140)
	for i := range samples {
		f := float64(i) / 140
		samples[i] = 620 * (0.25 + 2.6*f*f) * (1 + 0.18*math.Sin(f*9))
	}
	c := NewChart(ChartHistogram, nil, Series{Name: "耗时", Values: samples})
	c.Bins = 14

	lg := testLayout()
	lg.yMin, lg.yMax = c.axisRange()
	lg.plot = geom.Rect{X: 40, Y: 20, W: 400, H: 190}

	maxCount := 0.0
	for _, v := range c.plotValues(0) {
		if v > maxCount {
			maxCount = v
		}
	}
	t.Logf("值域 %v~%v 最大桶 %v 桶数 %d", lg.yMin, lg.yMax, maxCount, len(c.plotValues(0)))

	for i := 0; i < c.points(); i++ {
		r := barRect(c, lg, 0, i)
		if r.Y < lg.plot.Y-1 || r.Y+r.H > lg.plot.Y+lg.plot.H+1 {
			t.Fatalf("柱 %d 纵向越界：%+v（绘图区 %v~%v，柱顶 %v 高 %v）",
				i, r, lg.plot.Y, lg.plot.Y+lg.plot.H, r.Y, r.H)
		}
	}
}

// TestHistogramBarGeom 直方图柱几何：宽度合理且不越出绘图区。
func TestHistogramBarGeom(t *testing.T) {
	samples := make([]float64, 140)
	for i := range samples {
		samples[i] = float64(i)
	}
	c := NewChart(ChartHistogram, nil, Series{Name: "耗时", Values: samples})
	c.Bins = 14
	c.updateHistogram()

	lg := testLayout()
	lg.yMin, lg.yMax = c.axisRange()
	right := lg.plot.X + lg.plot.W
	bottom := lg.plot.Y + lg.plot.H
	for i := 0; i < c.points(); i++ {
		r := barRect(c, lg, 0, i)
		if r.W <= 1 || r.H <= 0 {
			t.Fatalf("柱 %d 尺寸异常：%+v", i, r)
		}
		if r.X < lg.plot.X-1 || r.X+r.W > right+1 {
			t.Fatalf("柱 %d 横向越界：%+v（绘图区 %v~%v）", i, r, lg.plot.X, right)
		}
		if r.Y < lg.plot.Y-1 || r.Y+r.H > bottom+1 {
			t.Fatalf("柱 %d 纵向越界：%+v（绘图区 %v~%v）", i, r, lg.plot.Y, bottom)
		}
	}
}

// TestWaterfallChart 瀑布图：累计路径、值域与命中。
func TestWaterfallChart(t *testing.T) {
	labels := []string{"期初", "收入", "成本", "税费"}
	c := NewChart(ChartWaterfall, labels, Series{Name: "利润", Values: []float64{100, 60, -40, -15}})
	if n := c.points(); n != 4 {
		t.Fatalf("点数应为 4，实际 %d", n)
	}
	if from, to := c.waterfallStep(0, 0); from != 0 || to != 100 {
		t.Fatalf("第 1 步应为 0→100，实际 %v→%v", from, to)
	}
	if from, to := c.waterfallStep(0, 3); from != 120 || to != 105 {
		t.Fatalf("第 4 步应为 120→105，实际 %v→%v", from, to)
	}

	lo, hi := c.axisRange()
	if lo > 0 || hi < 160 {
		t.Fatalf("值域应覆盖 0~160：%v ~ %v", lo, hi)
	}
	if c.waterfallTip(0, 2) == "" {
		t.Fatal("瀑布图提示为空")
	}

	lg := testLayout()
	lg.yMin, lg.yMax = lo, hi
	lg.plot = geom.Rect{X: 40, Y: 10, W: 200, H: 110}
	if _, i, ok := waterfallHit(c, lg, lg.xAt(2, 4), lg.plot.Y+lg.plot.H/2); !ok || i != 2 {
		t.Fatalf("应命中第 3 列：ok=%v i=%d", ok, i)
	}
}

// TestBoxChart 箱线图：五数概括（直给 5 个数 / 自动统计两种输入）。
func TestBoxChart(t *testing.T) {
	c := NewChart(ChartBox, []string{"A", "B"},
		Series{Name: "A", Values: []float64{10, 20, 30, 40, 50}},
		Series{Name: "B", Values: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
	)
	if n := c.points(); n != 2 {
		t.Fatalf("箱数应为 2，实际 %d", n)
	}

	st, ok := c.boxStats(0)
	if !ok || st.min != 10 || st.max != 50 || st.med != 30 || st.q1 != 20 || st.q3 != 40 {
		t.Fatalf("直给五数解析错误：%+v", st)
	}

	st2, ok := c.boxStats(1)
	if !ok || st2.min != 1 || st2.max != 10 {
		t.Fatalf("样本统计错误：%+v", st2)
	}
	if d := st2.med - 5.5; d > 1e-9 || d < -1e-9 {
		t.Fatalf("中位数应为 5.5，实际 %v", st2.med)
	}

	lo, hi := c.axisRange()
	if lo > 1 || hi < 50 {
		t.Fatalf("值域应覆盖 1~50：%v ~ %v", lo, hi)
	}
	if c.boxTip(0) == "" {
		t.Fatal("箱线图提示为空")
	}

	lg := testLayout()
	lg.yMin, lg.yMax = lo, hi
	if j, _, ok := boxHit(c, lg, lg.xAt(1, 2), lg.plot.Y+lg.plot.H/2); !ok || j != 1 {
		t.Fatalf("应命中第 2 个箱：ok=%v j=%d", ok, j)
	}
}

// TestTimeshareChart 分时图：以基准线对称、需要纹理、能画出像素。
func TestTimeshareChart(t *testing.T) {
	c := NewChart(ChartTimeshare, []string{"9:30", "10:30", "11:30"},
		Series{Name: "价格", Values: []float64{10.2, 10.6, 9.8}})
	c.Min = 10.0 // 基准线（昨收）

	lo, hi := c.axisRange()
	if !(lo < 10 && hi > 10) {
		t.Fatalf("值域应跨基准线：%v ~ %v", lo, hi)
	}
	if mid := (lo + hi) / 2; mid < 9.999 || mid > 10.001 {
		t.Fatalf("应以基准线为中心对称：%v ~ %v", lo, hi)
	}
	if !c.needsTexture() {
		t.Fatal("分时图需要纹理")
	}

	lg := testLayout()
	lg.yMin, lg.yMax = lo, hi
	px := c.rasterPlot(200, 110, lg.shifted(lg.plot.X, lg.plot.Y))
	if n := opaque(px); n < 100 {
		t.Fatalf("分时纹理像素过少：%d", n)
	}
}

// TestCandleVolumeFingerprint 量能开关 / 成交量变化都要让缓存失效。
func TestCandleVolumeFingerprint(t *testing.T) {
	_, cs := testCandles()
	c := NewChart(ChartCandle, nil, Series{Candles: cs})

	h1 := c.fingerprint(300, 200)
	c.ShowVolume = true
	if h2 := c.fingerprint(300, 200); h2 == h1 {
		t.Fatal("量能开关未影响指纹")
	}
	h3 := c.fingerprint(300, 200)
	next := append([]Candle(nil), cs...)
	next[0].Volume = 1234
	c.Series[0].Candles = next
	if h4 := c.fingerprint(300, 200); h4 == h3 {
		t.Fatal("成交量变化未影响指纹")
	}
}

// TestHeatmapChart 热力图：行列推导、原生图元、格子命中。
func TestHeatmapChart(t *testing.T) {
	vals := make([]float64, 9)
	for i := range vals {
		vals[i] = float64(i + 1)
	}
	c := NewChart(ChartHeatmap, nil, Series{Values: vals})

	cols, rows := c.heatGrid()
	if cols != 3 || rows != 3 {
		t.Fatalf("默认应为 3x3：%d x %d", cols, rows)
	}
	c.Cols = 2
	if cols, rows := c.heatGrid(); cols != 2 || rows != 5 {
		t.Fatalf("Cols=2 应为 2x5：%d x %d", cols, rows)
	}
	c.Cols = 0
	if c.needsTexture() {
		t.Fatal("热力图是原生图元，不需要纹理")
	}
	if !c.hideAxisTicks() {
		t.Fatal("热力图不应画坐标刻度")
	}

	lg := testLayout()
	if _, i, ok := heatHit(c, lg, lg.plot.X+lg.plot.W*0.2, lg.plot.Y+lg.plot.H*0.2); !ok || i != 0 {
		t.Fatalf("左上角应为第 0 格：ok=%v i=%d", ok, i)
	}
	if _, i, ok := heatHit(c, lg, lg.plot.X+lg.plot.W*0.8, lg.plot.Y+lg.plot.H*0.8); !ok || i != 8 {
		t.Fatalf("右下角应为第 8 格：ok=%v i=%d", ok, i)
	}
}

// TestFunnelChart 漏斗图：层级数、值域、命中与纹理。
func TestFunnelChart(t *testing.T) {
	c := NewChart(ChartFunnel, []string{"访问", "注册", "付费"},
		Series{Name: "访问", Values: []float64{1000}},
		Series{Name: "注册", Values: []float64{320}},
		Series{Name: "付费", Values: []float64{64}},
	)
	if n := c.points(); n != 3 {
		t.Fatalf("层数应为 3，实际 %d", n)
	}
	lo, hi := c.axisRange()
	if lo != 0 || hi < 1000 {
		t.Fatalf("值域应为 0~1000：%v ~ %v", lo, hi)
	}
	if !c.needsTexture() {
		t.Fatal("漏斗图需要纹理")
	}

	lg := testLayout()
	if j, _, ok := funnelHit(c, lg, lg.plot.X+lg.plot.W/2, lg.plot.Y+lg.plot.H*0.5); !ok || j != 1 {
		t.Fatalf("中段应为第 2 层：ok=%v j=%d", ok, j)
	}
	px := c.rasterPlot(200, 110, lg.shifted(lg.plot.X, lg.plot.Y))
	if n := opaque(px); n < 100 {
		t.Fatalf("漏斗纹理像素过少：%d", n)
	}
}

// TestGaugeChart 仪表盘：默认量程、单值、纹理。
func TestGaugeChart(t *testing.T) {
	c := NewChart(ChartGauge, nil, Series{Values: []float64{72}})
	lo, hi := c.axisRange()
	if lo != 0 || hi != 100 {
		t.Fatalf("默认量程应为 0~100：%v ~ %v", lo, hi)
	}
	if c.points() != 1 {
		t.Fatalf("仪表盘点数应为 1，实际 %d", c.points())
	}
	if v := c.gaugeValue(); v != 72 {
		t.Fatalf("当前值应为 72，实际 %v", v)
	}
	if !c.needsTexture() {
		t.Fatal("仪表盘需要纹理")
	}

	lg := testLayout()
	lg.yMin, lg.yMax = lo, hi
	px := c.rasterPlot(200, 200, lg.shifted(lg.plot.X, lg.plot.Y))
	if n := opaque(px); n < 100 {
		t.Fatalf("仪表盘纹理像素过少：%d", n)
	}
}
