// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"strings"
	"testing"
)

// testCandles 五根演示蜡烛（价格区间 9.5 ~ 13.2）。
func testCandles() ([]string, []Candle) {
	labels := []string{"1/2", "1/3", "1/4", "1/5", "1/6"}
	candles := []Candle{
		{Open: 10, High: 12, Low: 9.5, Close: 11.5},
		{Open: 11.5, High: 11.8, Low: 10.2, Close: 10.6},
		{Open: 10.6, High: 13.2, Low: 10.4, Close: 12.8},
		{Open: 12.8, High: 13, Low: 12.1, Close: 12.3},
		{Open: 12.3, High: 12.9, Low: 11.8, Close: 12.6},
	}
	return labels, candles
}

// TestCandleAutoRange K 线纵轴必须贴合高低价，而不是像普通图表那样从 0 起。
func TestCandleAutoRange(t *testing.T) {
	labels, cs := testCandles()
	c := NewChart(ChartCandle, labels, Series{Candles: cs})

	lo, hi := c.axisRange()
	if lo >= 9.5 || hi <= 13.2 {
		t.Fatalf("纵轴没包住 9.5~13.2：%v ~ %v", lo, hi)
	}
	if lo <= 0 {
		t.Fatalf("K 线不应从 0 起：lo=%v", lo)
	}
	// 整步长对齐后上下界会略超出数据，但不应超过一个步长
	if lo < 9.5-1.01 || hi > 13.2+1.01 {
		t.Fatalf("留白过大（蜡烛会被压扁）：%v ~ %v", lo, hi)
	}

	// 显式 Min / Max 优先
	c.Min, c.Max = 5, 20
	lo, hi = c.axisRange()
	if lo != 5 || hi != 20 {
		t.Fatalf("显式范围未生效：%v ~ %v", lo, hi)
	}
}

// TestCandleNilData 空数据不能 panic，且给出可用区间。
func TestCandleNilData(t *testing.T) {
	c := NewChart(ChartCandle, nil)
	lo, hi := c.axisRange()
	if !(hi > lo) {
		t.Fatalf("空数据区间非法：%v ~ %v", lo, hi)
	}
	if n := c.points(); n != 0 {
		t.Fatalf("空数据点数应为 0，实际 %d", n)
	}
}

// TestCandlePointsAndHit 命中按 x 定位到具体蜡烛。
func TestCandlePointsAndHit(t *testing.T) {
	labels, cs := testCandles()
	c := NewChart(ChartCandle, labels, Series{Candles: cs})
	if n := c.points(); n != len(cs) {
		t.Fatalf("点数应为 %d，实际 %d", len(cs), n)
	}

	lg := testLayout()
	lg.yMin, lg.yMax = c.axisRange()

	for want := 0; want < len(cs); want++ {
		x := lg.xAt(want, len(cs))
		s, i, ok := candleHit(c, lg, x, lg.plot.Y+lg.plot.H/2)
		if !ok || s != 0 || i != want {
			t.Fatalf("第 %d 根命中失败：ok=%v s=%d i=%d", want, ok, s, i)
		}
	}
	if _, _, ok := candleHit(c, lg, lg.plot.X-30, lg.plot.Y+10); ok {
		t.Fatal("绘图区外不应命中")
	}
}

// TestCandleMA 均线：不足周期的点为 NaN，之后是收盘均值。
func TestCandleMA(t *testing.T) {
	_, cs := testCandles()
	c := NewChart(ChartCandle, nil, Series{Candles: cs})
	ma := c.candleMAValues(0, 3)
	if !math.IsNaN(ma[0]) || !math.IsNaN(ma[1]) {
		t.Fatalf("前 %d 个点应为 NaN：%v %v", 2, ma[0], ma[1])
	}
	want := (11.5 + 10.6 + 12.8) / 3
	if math.Abs(ma[2]-want) > 1e-9 {
		t.Fatalf("MA3[2] 应为 %v，实际 %v", want, ma[2])
	}
	ma5 := c.candleMAValues(0, 9)
	for i, v := range ma5 {
		if !math.IsNaN(v) {
			t.Fatalf("数据不足时第 %d 个点应为 NaN，实际 %v", i, v)
		}
	}
}

// TestCandleNeedsTexture 没有均线时不建纹理（蜡烛走原生图元）。
func TestCandleNeedsTexture(t *testing.T) {
	_, cs := testCandles()
	c := NewChart(ChartCandle, nil, Series{Candles: cs})
	if c.needsTexture() {
		t.Fatal("未设 MA 时不应需要纹理")
	}
	c.MA = []int{5}
	if !c.needsTexture() {
		t.Fatal("设了 MA 后应需要纹理")
	}
}

// TestCandleMARaster 均线纹理必须真的画出像素。
func TestCandleMARaster(t *testing.T) {
	_, cs := testCandles()
	c := NewChart(ChartCandle, nil, Series{Candles: cs})
	c.MA = []int{2}

	lg := testLayout()
	lg.yMin, lg.yMax = c.axisRange()
	px := c.rasterPlot(200, 110, lg.shifted(lg.plot.X, lg.plot.Y))
	if n := opaque(px); n == 0 {
		t.Fatal("均线纹理为空")
	}
}

// TestCandleTip 悬浮文案包含开高低收与涨跌幅。
func TestCandleTip(t *testing.T) {
	labels, cs := testCandles()
	c := NewChart(ChartCandle, labels, Series{Name: "样例", Candles: cs})
	tip := c.candleTip(0, 0)
	for _, want := range []string{"1/2", "样例", "开 10.00", "高 12.00", "低 9.50", "收 11.50", "+15.00%"} {
		if !strings.Contains(tip, want) {
			t.Fatalf("提示缺少 %q：%s", want, tip)
		}
	}
}

// TestCandleFingerprint 蜡烛数据 / 均线周期变化都要让缓存失效。
func TestCandleFingerprint(t *testing.T) {
	_, cs := testCandles()
	c := NewChart(ChartCandle, nil, Series{Candles: cs})

	h1 := c.fingerprint(300, 200)
	next := append([]Candle(nil), cs...)
	next[0].Close = 11.9
	c.Series[0].Candles = next
	if h2 := c.fingerprint(300, 200); h2 == h1 {
		t.Fatal("蜡烛数据变化后指纹未变")
	}

	h3 := c.fingerprint(300, 200)
	c.MA = []int{5}
	if h4 := c.fingerprint(300, 200); h4 == h3 {
		t.Fatal("MA 变化后指纹未变")
	}
}

// TestCandleGeom 影线覆盖高→低、实体覆盖开↔收，方向不颠倒（屏幕 y 向下）。
func TestCandleGeom(t *testing.T) {
	lg := testLayout()
	k := Candle{Open: 20, High: 90, Low: 10, Close: 60}
	wick, body := candleGeom(lg, k, 100, 8, 1)

	if d := wick.Y - lg.mapY(90); d > 0.01 || d < -0.01 {
		t.Fatalf("影线顶端应在最高价：%v vs %v", wick.Y, lg.mapY(90))
	}
	if d := wick.Bottom() - lg.mapY(10); d > 0.01 || d < -0.01 {
		t.Fatalf("影线底端应在最低价：%v vs %v", wick.Bottom(), lg.mapY(10))
	}
	if d := body.Y - lg.mapY(60); d > 0.01 || d < -0.01 {
		t.Fatalf("实体顶端应在收盘价（涨）：%v vs %v", body.Y, lg.mapY(60))
	}
	if d := body.Bottom() - lg.mapY(20); d > 0.01 || d < -0.01 {
		t.Fatalf("实体底端应在开盘价：%v vs %v", body.Bottom(), lg.mapY(20))
	}
	if body.W != 8 {
		t.Fatalf("实体宽度应为 8，实际 %v", body.W)
	}

	_, flat := candleGeom(lg, Candle{Open: 50, High: 50, Low: 50, Close: 50}, 100, 8, 1)
	if flat.H < 1.5 {
		t.Fatalf("十字星实体至少 1.5px，实际 %v", flat.H)
	}
}

// TestCandleTicksAligned K 线自动刻度必须落在整步长上（不出 97.2 这种值）。
func TestCandleTicksAligned(t *testing.T) {
	labels, cs := testCandles()
	c := NewChart(ChartCandle, labels, Series{Candles: cs})

	lo, hi := c.axisRange()
	ticks := c.candleTicks(lo, hi)
	if len(ticks) < 3 {
		t.Fatalf("刻度太少：%v", ticks)
	}
	step := ticks[1] - ticks[0]
	if step <= 0 {
		t.Fatalf("步长非法：%v", ticks)
	}
	for i, v := range ticks {
		if math.Abs(v-lo-float64(i)*step) > 1e-9 {
			t.Fatalf("第 %d 个刻度不等距：%v", i, ticks)
		}
		if math.Abs(v/step-math.Round(v/step)) > 1e-9 {
			t.Fatalf("刻度 %v 不是步长 %v 的整数倍", v, step)
		}
	}
	if ticks[0] > 9.5 || ticks[len(ticks)-1] < 13.2 {
		t.Fatalf("刻度未覆盖数据区间：%v", ticks)
	}
}

// TestCandleMABounds 均线只画在有效区间：不早于 period-1，不越过最后一根。
func TestCandleMABounds(t *testing.T) {
	_, cs := testCandles()
	c := NewChart(ChartCandle, nil, Series{Candles: cs})
	c.MA = []int{3}

	lg := testLayout()
	lg.yMin, lg.yMax = c.axisRange()
	sh := lg.shifted(lg.plot.X, lg.plot.Y)
	w := int(math.Round(float64(lg.plot.W * lg.scale)))
	h := int(math.Round(float64(lg.plot.H * lg.scale)))
	px := c.rasterPlot(w, h, sh)
	if px == nil {
		t.Fatal("光栅失败")
	}

	left, right := -1, -1
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			if px[(y*w+x)*4+3] > 8 {
				if left < 0 {
					left = x
				}
				right = x
				break
			}
		}
	}
	if left < 0 {
		t.Fatal("均线没有像素")
	}
	n := len(cs)
	wantL := sh.xAt(2, n) - sh.plot.X // 第 3 根是 MA3 的第一个有效点
	wantR := sh.xAt(n-1, n) - sh.plot.X
	if float32(left) < wantL-6 || float32(left) > wantL+6 {
		t.Fatalf("均线起点异常：left=%d want≈%.0f", left, wantL)
	}
	if float32(right) < wantR-6 || float32(right) > wantR+6 {
		t.Fatalf("均线终点异常：right=%d want≈%.0f", right, wantR)
	}
}
