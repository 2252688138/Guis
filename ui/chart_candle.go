// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"math"
	"strconv"

	"guis/internal/geom"
	"guis/internal/gpu"
)

// K 线图（ChartCandle）。
//
// 蜡烛本体用原生矩形绘制（影线细条 + 实体矩形），与柱状图同一思路：
// 边缘永远锐利、零纹理内存；只有均线（斜线）需要光栅，走 chart_draw 的画布纹理，
// Chart.MA 为空时完全不建纹理。

// 默认涨跌配色：A 股习惯「红涨绿跌」，可用 Chart.CandleUp / CandleDown 覆盖。
const (
	candleUpColor   = "#ef4444"
	candleDownColor = "#22c55e"
)

// candleMAColors 均线配色：避开涨跌红绿，用蓝 / 紫 / 琥珀，条数再多也循环。
var candleMAColors = []string{"#60a5fa", "#c084fc", "#fcd34d", "#f472b6"}

// candleMAColor 第 k 条均线的画笔。
func candleMAColor(k int) gpu.Paint {
	if len(candleMAColors) == 0 {
		return C(ColPrimary)
	}
	return C(candleMAColors[k%len(candleMAColors)])
}

// candleOf 取第 j 条系列第 i 根蜡烛。
func (c *Chart) candleOf(j, i int) (Candle, bool) {
	if j < 0 || j >= len(c.Series) {
		return Candle{}, false
	}
	cs := c.Series[j].Candles
	if i < 0 || i >= len(cs) {
		return Candle{}, false
	}
	return cs[i], true
}

// candlePaint 一根蜡烛的画笔（收盘 ≥ 开盘为涨）。
func (c *Chart) candlePaint(k Candle) gpu.Paint {
	if k.Close >= k.Open {
		if c.CandleUp != "" {
			return C(c.CandleUp)
		}
		return C(candleUpColor)
	}
	if c.CandleDown != "" {
		return C(c.CandleDown)
	}
	return C(candleDownColor)
}

// candleExtent 所有蜡烛的最高价 / 最低价。
func (c *Chart) candleExtent() (float64, float64, bool) {
	lo, hi := 0.0, 0.0
	found := false
	for _, s := range c.Series {
		for _, k := range s.Candles {
			if !found {
				lo, hi, found = k.Low, k.High, true
				continue
			}
			lo = math.Min(lo, k.Low)
			hi = math.Max(hi, k.High)
		}
	}
	return lo, hi, found
}

// candleRange K 线纵轴范围：贴合最高/最低价并留 4% 余量
// （普通图表默认从 0 起，K 线从 0 起会把蜡烛压成一条线）。
//
// 显式 c.Min / c.Max 优先；ChartCandle 下 Min == 0 仍视为「自动」。
func (c *Chart) candleRange() (float64, float64) {
	lo, hi, ok := c.candleExtent()
	if !ok || hi <= lo {
		// 无数据 / 一字线：给一个能看见实体的窄区间
		base := hi
		if base == 0 {
			base = 1
		}
		lo, hi = base*0.95, base*1.05
	}
	pad := (hi - lo) * 0.04
	lo -= pad
	hi += pad
	// 自动模式下上下界对齐到「好看」的整步长：
	// 行情图刻度出现 97.2 / 113 这种零碎值会很穿。
	step := niceStep(hi-lo, c.YTicks)
	c.candleStep = step
	if c.Min != 0 {
		lo = c.Min
	} else {
		lo = math.Floor(lo/step) * step
	}
	if c.Max > 0 {
		hi = c.Max
	} else {
		hi = math.Ceil(hi/step) * step
	}
	if hi <= lo {
		hi = lo + step
	}
	return lo, hi
}

// niceStep 「好看」的刻度步长（1/2/2.5/5 × 10^n）。
func niceStep(span float64, ticks int) float64 {
	if span <= 0 {
		return 1
	}
	if ticks <= 0 {
		ticks = 4
	}
	raw := span / float64(ticks)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	norm := raw / mag
	switch {
	case norm <= 1:
		return mag
	case norm <= 2:
		return 2 * mag
	case norm <= 2.5:
		return 2.5 * mag
	case norm <= 5:
		return 5 * mag
	}
	return 10 * mag
}

// candleTicks K 线整步长刻度（每个值都是 step 的整数倍）。
func (c *Chart) candleTicks(lo, hi float64) []float64 {
	step := c.candleStep
	if step <= 0 {
		step = niceStep(hi-lo, c.YTicks)
	}
	if step <= 0 || hi <= lo {
		return []float64{lo, hi}
	}
	n := int(math.Round((hi - lo) / step))
	if n < 1 {
		n = 1
	}
	if n > 12 {
		n = 12
	}
	out := make([]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		out = append(out, lo+float64(i)*step)
	}
	return out
}

// candleGeom 一根蜡烛的两块原生矩形：影线（高→低）与实体（开↔收）。
// cx 为蜡烛中心 x；平盘（十字星）也保证实体至少 1.5px，否则细到看不见。
func candleGeom(lg chartLayout, k Candle, cx, bodyW, wickW float32) (geom.Rect, geom.Rect) {
	midX := float32(math.Round(float64(cx)))
	hiY, loY := lg.mapY(k.High), lg.mapY(k.Low)
	if loY < hiY {
		hiY, loY = loY, hiY
	}
	wick := geom.Rect{
		X: float32(math.Round(float64(midX - wickW/2))),
		Y: hiY,
		W: maxf(wickW, 1),
		H: maxf(loY-hiY, 1),
	}
	top, bot := lg.mapY(math.Max(k.Open, k.Close)), lg.mapY(math.Min(k.Open, k.Close))
	h := bot - top
	if h < 1.5 {
		top, h = top-0.75, 1.5
	}
	body := geom.Rect{X: midX - bodyW/2, Y: top, W: bodyW, H: h}
	return wick, body
}

// paintChartCandles 蜡烛图（影线 + 实体，全部原生圆角矩形）。
func (a *App) paintChartCandles(c *Chart, lg chartLayout) {
	n := c.points()
	ns := len(c.Series)
	if n <= 0 || ns <= 0 || lg.plot.W <= 0 || lg.plot.H <= 0 {
		return
	}
	band := lg.bandW(n)
	// 多条系列并排分栏（与柱状图一致），单系列占满整组宽度
	slot := band
	if ns > 1 {
		slot = band / float32(ns)
	}
	bodyW := minf(slot*0.68, 22)
	if bodyW < 1 {
		bodyW = 1
	}
	wickW := minf(maxf(bodyW*0.16, 1), 3)

	for i := 0; i < n; i++ {
		cx := lg.xAt(i, n)
		for j := 0; j < ns; j++ {
			k, ok := c.candleOf(j, i)
			if !ok || k.High < k.Low {
				continue
			}
			x := cx
			if ns > 1 {
				x = cx - band/2 + slot*(float32(j)+0.5)
			}
			// 影线 + 实体全部对齐到整像素，否则 1px 线会被抗锯齿糊成两半
			wick, body := candleGeom(lg, k, x, bodyW, wickW)
			p := c.candlePaint(k)
			a.r.Rect(gpu.Shape{Rect: wick, Fill: p})
			a.r.Rect(gpu.Shape{Rect: body, Fill: p})
		}
	}
}

// candleMAValues 第 j 条系列的 period 周期收盘均线（不足周期的点为 NaN）。
func (c *Chart) candleMAValues(j, period int) []float64 {
	n := c.points()
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if period <= 0 || j < 0 || j >= len(c.Series) {
		return out
	}
	cs := c.Series[j].Candles
	if len(cs) < period {
		return out
	}
	sum := 0.0
	for i := 0; i < len(cs) && i < n; i++ {
		sum += cs[i].Close
		if i >= period {
			sum -= cs[i-period].Close
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		}
	}
	return out
}

// candleMA 均线（光栅层，坐标相对绘图区）。
//
// 多系列 K 线只在第一条系列上叠均线：均线是「同一标的的趋势」，
// 画到并排的多只标的上没有意义。
func (d *chartDraw) candleMA(c *Chart) {
	n := c.points()
	pl := d.lg.plot
	if n <= 1 || len(c.MA) == 0 || pl.W <= 0 || pl.H <= 0 {
		return
	}
	for k, period := range c.MA {
		col := candleMAColor(k)
		vals := c.candleMAValues(0, period)
		seg := make([]fpt, 0, n)
		flush := func() {
			if len(seg) >= 2 {
				d.cv.strokePoly(seg, d.maWidth(), col, false)
			}
			seg = seg[:0]
		}
		for i, v := range vals {
			if i >= n || math.IsNaN(v) {
				flush()
				continue
			}
			seg = append(seg, d.pt(d.lg.xAt(i, n)-pl.X, d.lg.mapY(v)-pl.Y))
		}
		flush()
	}
}

// maWidth 均线线宽：比普通折线更细，均线只是辅助线。
func (d *chartDraw) maWidth() float32 { return maxf(1.4, 1.4*d.k) }

// paintCandleMALegend 均线图例（绘图区左上角：色条 + MA5 / MA10）。
//
// 均线画在纹理层、没有 Series 名，必须单独标注，
// 否则用户无法分辨哪条是哪个周期。
func (a *App) paintCandleMALegend(c *Chart, lg chartLayout) {
	if len(c.MA) == 0 || lg.plot.W <= 0 || lg.plot.H <= 0 {
		return
	}
	f := a.fonts.get(10.5, 500)
	if f == nil {
		return
	}
	x := lg.plot.X + 6
	y := lg.plot.Y + 2
	for k, period := range c.MA {
		if period <= 0 {
			continue
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{
			X: x, Y: y + f.LineHeight/2 - 1.5, W: 10, H: 3,
		}, Radius: [4]float32{1.5, 1.5, 1.5, 1.5}, Fill: candleMAColor(k)})
		x += 13
		s := "MA" + strconv.Itoa(period)
		a.r.TextRun(gpu.TextRun{X: x, Y: y, Text: s, Font: f, Color: C(ColTextMuted)})
		x += f.Measure(s) + 12
	}
}

// candleHit K 线命中：按 x 落在哪根蜡烛上，取该位置有数据的系列。
func candleHit(c *Chart, lg chartLayout, mx, my float32) (int, int, bool) {
	n := c.points()
	if n <= 0 || lg.plot.W <= 0 || lg.plot.H <= 0 {
		return 0, 0, false
	}
	if !lg.plot.Contains(geom.Point{X: mx, Y: my}) {
		return 0, 0, false
	}
	band := lg.bandW(n)
	if band <= 0 {
		return 0, 0, false
	}
	i := int((mx - lg.plot.X) / band)
	if i < 0 {
		i = 0
	}
	if i >= n {
		i = n - 1
	}
	for j := range c.Series {
		if _, ok := c.candleOf(j, i); ok {
			return j, i, true
		}
	}
	return 0, 0, false
}

// candleTip K 线悬浮文案：`日期 · 名称  开 … 高 … 低 … 收 … 涨跌幅`。
func (c *Chart) candleTip(s, i int) string {
	k, ok := c.candleOf(s, i)
	if !ok {
		return ""
	}
	head := ""
	if i >= 0 && i < len(c.Labels) && c.Labels[i] != "" {
		head = c.Labels[i] + " · "
	}
	name := ""
	if s >= 0 && s < len(c.Series) {
		name = c.Series[s].Name
	}
	pct := ""
	if k.Open > 0 {
		pct = fmt.Sprintf("  %+.2f%%", (k.Close-k.Open)/k.Open*100)
	}
	return fmt.Sprintf("%s%s  开 %.2f  高 %.2f  低 %.2f  收 %.2f%s",
		head, name, k.Open, k.High, k.Low, k.Close, pct)
}
