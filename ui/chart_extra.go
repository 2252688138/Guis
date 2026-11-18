// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"math"

	"guis/internal/geom"
	"guis/internal/gpu"
)

// 追加图表类型：水平条形 / 直方图 / 玫瑰 / 气泡。
//
// 与既有类型共用两套绘制通道：矩形的（水平条）走原生图元零纹理，
// 弧线与圆的（玫瑰、气泡）走 chart_draw 的矢量纹理。

// ---- 坐标系辅助（水平条形：值轴在 X、类目沿 Y）----

// mapX 数值 → 绘图区 x 坐标（值轴在 X 的图表用，纵轴范围沿用 yMin/yMax）。
func (lg *chartLayout) mapX(v float64) float32 {
	span := lg.yMax - lg.yMin
	if span <= 0 {
		return lg.plot.X
	}
	return lg.plot.X + float32((v-lg.yMin)/span)*lg.plot.W
}

// yAt 第 i 个类目在绘图区的纵坐标（组中心）。
func (lg *chartLayout) yAt(i, n int) float32 {
	if n <= 1 {
		return lg.plot.Y + lg.plot.H/2
	}
	return lg.plot.Y + lg.plot.H*(float32(i)+0.5)/float32(n)
}

// bandH 单个类目在绘图区占的高度。
func (lg *chartLayout) bandH(n int) float32 {
	if n <= 0 {
		return lg.plot.H
	}
	return lg.plot.H / float32(n)
}

// dataMin 数据最小值（全为非负时返回 0）。
func (c *Chart) dataMin() float64 {
	m := 0.0
	for _, s := range c.Series {
		for _, v := range s.Values {
			if v < m {
				m = v
			}
		}
	}
	return m
}

// ---- 水平条形图 ChartBarH ----

// barsHPlot 水平条形的绘图区：左侧留类目名列，底部留数值刻度。
func barsHPlot(a *App, c *Chart, inner geom.Rect) geom.Rect {
	catW := float32(0)
	if f := a.fonts.get(chartLabelSize, 500); f != nil {
		for i := 0; i < len(c.Labels) && i < 64; i++ {
			if w := f.Measure(c.Labels[i]); w > catW {
				catW = w
			}
		}
		catW += 10
	}
	bottomH := float32(0)
	if f := a.fonts.get(chartTickSize, 400); f != nil {
		bottomH = f.LineHeight + 8
	}
	return geom.Rect{
		X: inner.X + catW,
		Y: inner.Y,
		W: inner.W - catW,
		H: inner.H - bottomH,
	}
}

// barsHGeom 水平条形的分栏尺寸。
func barsHGeom(c *Chart, lg chartLayout) (band, groupH, barH, gap float32) {
	n := c.points()
	ns := len(c.Series)
	band = lg.bandH(n)
	groupH = band * (1 - chartBarGap)
	if groupH < 1 {
		groupH = 1
	}
	if c.Stacked || ns <= 1 {
		return band, groupH, groupH, 0
	}
	barH = groupH / float32(ns)
	gap = minf(barH*0.18, 3)
	if gap < 0 {
		gap = 0
	}
	return band, groupH, barH, gap
}

// barsHZeroX 水平条形的 0 值基准线。
func barsHZeroX(c *Chart, lg chartLayout) float32 {
	if c.Min > 0 {
		return lg.plot.X
	}
	return lg.mapX(math.Max(c.Min, 0))
}

// paintChartBarsH 水平条形图（支持分组 / 堆叠 / 负值）。
func (a *App) paintChartBarsH(c *Chart, lg chartLayout) {
	n := c.points()
	ns := len(c.Series)
	if n <= 0 || ns <= 0 || lg.plot.W <= 0 || lg.plot.H <= 0 {
		return
	}
	_, groupH, barH, gap := barsHGeom(c, lg)
	zeroX := barsHZeroX(c, lg)
	rad := [4]float32{}
	if r := minf(barH, 10) * 0.22; r > 1 {
		rad = [4]float32{r, r, r, r}
	}
	for i := 0; i < n; i++ {
		cy := lg.yAt(i, n)
		acc := 0.0
		for j := 0; j < ns; j++ {
			s := c.Series[j]
			if i >= len(s.Values) {
				continue
			}
			v := s.Values[i]
			p := c.colorOf(j)
			var x0, x1, y0, h float32
			if c.Stacked {
				x0, x1 = lg.mapX(acc), lg.mapX(acc+v)
				acc += v
				y0, h = cy-groupH/2, barH
			} else {
				x0, x1 = lg.mapX(v), zeroX
				y0, h = cy-groupH/2+float32(j)*barH+gap/2, barH-gap
			}
			if x1 < x0 {
				x0, x1 = x1, x0
			}
			w := x1 - x0
			if w < 1 {
				w = 1
			}
			to := gpu.Paint{R: p.R * 0.72, G: p.G * 0.72, B: p.B * 0.72, A: p.A}
			a.r.Rect(gpu.Shape{
				Rect:   geom.Rect{X: x0, Y: y0, W: w, H: maxf(h, 1)},
				Radius: rad,
				Grad:   gpu.Gradient{On: true, Angle: 0, From: p, To: to},
			})
		}
	}
}

// barHRect 水平条形（系列 j、类目 i）的矩形，供命中与高亮复用。
func barHRect(c *Chart, lg chartLayout, j, i int) geom.Rect {
	n := c.points()
	ns := len(c.Series)
	if n <= 0 || ns <= 0 || j < 0 || j >= ns || i < 0 || i >= n {
		return geom.Rect{}
	}
	_, groupH, barH, gap := barsHGeom(c, lg)
	cy := lg.yAt(i, n)
	acc, v := 0.0, 0.0
	if i < len(c.Series[j].Values) {
		v = c.Series[j].Values[i]
	}
	var x0, x1, y0, h float32
	if c.Stacked {
		for k := 0; k < j && k < ns; k++ {
			if i < len(c.Series[k].Values) {
				acc += c.Series[k].Values[i]
			}
		}
		x0, x1 = lg.mapX(acc), lg.mapX(acc+v)
		y0, h = cy-groupH/2, barH
	} else {
		x0, x1 = lg.mapX(v), barsHZeroX(c, lg)
		y0, h = cy-groupH/2+float32(j)*barH+gap/2, barH-gap
	}
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	w := x1 - x0
	if w < 1 {
		w = 1
	}
	return geom.Rect{X: x0, Y: y0, W: w, H: maxf(h, 1)}
}

// barsHHit 水平条形命中：逐条判断矩形包含关系（系列数有限，够快）。
func barsHHit(c *Chart, lg chartLayout, mx, my float32) (int, int, bool) {
	n := c.points()
	ns := len(c.Series)
	for i := 0; i < n; i++ {
		for j := 0; j < ns; j++ {
			if r := barHRect(c, lg, j, i); r.H > 0 && r.Contains(geom.Point{X: mx, Y: my}) {
				return j, i, true
			}
		}
	}
	return 0, 0, false
}

// paintBarsHText 水平条形的轴文字：底部数值刻度 + 左侧类目名。
func (a *App) paintBarsHText(c *Chart, lg chartLayout) {
	tick := a.fonts.get(chartTickSize, 400)
	label := a.fonts.get(chartLabelSize, 500)
	dim := C(ColTextMuted)
	if lg.plot.W <= 0 || lg.plot.H <= 0 {
		return
	}
	if c.ShowAxis && tick != nil {
		for _, v := range lg.yTicks {
			s := fmtNum(v)
			a.r.TextRun(gpu.TextRun{
				X:     lg.mapX(v) - tick.Measure(s)/2,
				Y:     lg.plot.Bottom() + 5,
				Text:  s,
				Font:  tick,
				Color: dim,
			})
		}
	}
	if label == nil {
		return
	}
	n := c.points()
	// 类目太多时按行高抽稀
	stride := 1
	for stride < n && lg.bandH(n)*float32(stride) < label.LineHeight+3 {
		stride++
	}
	for i := 0; i < n; i += stride {
		if i >= len(c.Labels) {
			break
		}
		s := c.Labels[i]
		w := label.Measure(s)
		x := lg.plot.X - w - 7
		if x < lg.r.X {
			x = lg.r.X
		}
		a.r.TextRun(gpu.TextRun{
			X:     x,
			Y:     lg.yAt(i, n) - label.LineHeight/2,
			Text:  s,
			Font:  label,
			Color: dim,
		})
	}
}

// ---- 直方图 ChartHistogram ----

// histogramData 把 Series.Values 当原始样本自动分桶：返回各系列计数与桶边界。
func (c *Chart) histogramData() ([][]float64, []float64) {
	bins := c.Bins
	if bins <= 0 {
		bins = 12
	}
	if bins > 200 {
		bins = 200
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range c.Series {
		for _, v := range s.Values {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if !(hi > lo) {
		base := 0.0
		if !math.IsInf(hi, 0) {
			base = hi
		}
		lo, hi = base-0.5, base+0.5
	}
	span := hi - lo
	counts := make([][]float64, len(c.Series))
	for j, s := range c.Series {
		row := make([]float64, bins)
		for _, v := range s.Values {
			i := int((v - lo) / span * float64(bins))
			if i < 0 {
				i = 0
			}
			if i >= bins {
				i = bins - 1
			}
			row[i]++
		}
		counts[j] = row
	}
	edges := make([]float64, bins+1)
	for i := 0; i <= bins; i++ {
		edges[i] = lo + span*float64(i)/float64(bins)
	}
	return counts, edges
}

// updateHistogram 每帧布局时刷新一次分桶结果，绘制 / 命中 / 提示直接复用。
func (c *Chart) updateHistogram() {
	if c.Type != ChartHistogram {
		return
	}
	c.histCounts, c.histEdges = c.histogramData()
}

// plotValues 第 j 条系列用于绘制的数值行（直方图是桶计数，其余是 Values）。
// 绘制 / 命中 / 提示一律走这里：直方图的 Series.Values 是原始样本，
// 直接取值会把每条样本当成一根柱（柱高超出值域几十倍）。
func (c *Chart) plotValues(j int) []float64 {
	if j < 0 || j >= len(c.Series) {
		return nil
	}
	if c.Type == ChartHistogram {
		if c.histCounts == nil {
			c.updateHistogram()
		}
		if j < len(c.histCounts) {
			return c.histCounts[j]
		}
		return nil
	}
	return c.Series[j].Values
}

// histLabel 直方图第 i 个桶的区间文案。
func (c *Chart) histLabel(i int) string {
	if i < 0 || i+1 >= len(c.histEdges) {
		return ""
	}
	return fmtNum(c.histEdges[i]) + "~" + fmtNum(c.histEdges[i+1])
}

// ---- 玫瑰图 ChartRose ----

// rose 玫瑰图（南丁格尔）：扇区角度均分，半径按该扇区的数值和。
func (d *chartDraw) rose(c *Chart) {
	n := len(c.Series)
	if n <= 0 {
		return
	}
	maxV := 0.0
	for _, s := range c.Series {
		sum := 0.0
		for _, v := range s.Values {
			if v > 0 {
				sum += v
			}
		}
		if sum > maxV {
			maxV = sum
		}
	}
	if c.Max > maxV {
		maxV = c.Max
	}
	if maxV <= 0 {
		return
	}
	cx, cy := d.lg.pieC.X*d.k, d.lg.pieC.Y*d.k
	r1 := d.lg.pieR * d.k
	r0 := r1 * 0.16
	gap := float32(0.02)
	step := float32(2*math.Pi) / float32(n)
	for j := range c.Series {
		sum := 0.0
		for _, v := range c.Series[j].Values {
			if v > 0 {
				sum += v
			}
		}
		if sum <= 0 {
			continue
		}
		r := r0 + float32(sum/maxV)*(r1-r0)
		ang := float32(-math.Pi/2) + float32(j)*step
		d.sector(cx, cy, r0, r, ang+gap/2, ang+step-gap/2, c.colorOf(j))
	}
}

// roseHit 玫瑰图命中：按角度定位扇区，半径不超过绘图半径。
func roseHit(c *Chart, lg chartLayout, mx, my float32) (int, int, bool) {
	n := len(c.Series)
	if n <= 0 {
		return 0, 0, false
	}
	dx := mx - lg.pieC.X
	dy := my - lg.pieC.Y
	if float32(math.Hypot(float64(dx), float64(dy))) > lg.pieR {
		return 0, 0, false
	}
	a := math.Atan2(float64(dy), float64(dx)) + math.Pi/2
	for a < 0 {
		a += 2 * math.Pi
	}
	i := int(a / (2 * math.Pi) * float64(n))
	if i >= n {
		i = 0
	}
	return i, 0, true
}

// ---- 气泡图 ChartBubble ----

// bubble 气泡图：散点 + 第三维（Series.Z）决定气泡半径。
func (d *chartDraw) bubble(c *Chart) {
	n := c.points()
	if n <= 0 {
		return
	}
	maxZ := 0.0
	for _, s := range c.Series {
		for _, z := range s.Z {
			if z > maxZ {
				maxZ = z
			}
		}
	}
	if maxZ <= 0 {
		maxZ = 1
	}
	rMax := minf(18, maxf(6, d.lg.plot.W/float32(n)*0.42))
	for j := range c.Series {
		s := c.Series[j]
		col := c.colorOf(j)
		fill := col
		fill.A *= 0.55
		for i, v := range s.Values {
			x := d.lg.xAt(i, n) - d.lg.plot.X
			if i < len(s.X) && d.lg.yMax > d.lg.yMin {
				x = float32((s.X[i] - d.lg.yMin) / (d.lg.yMax - d.lg.yMin) * float64(d.lg.plot.W))
			}
			y := d.lg.mapY(v) - d.lg.plot.Y
			r := float32(4)
			if i < len(s.Z) {
				r = 3 + rMax*float32(s.Z[i]/maxZ)
				if r < 1.5 {
					r = 1.5
				}
			}
			d.cv.disc(x*d.k, y*d.k, r*d.k, fill)
			d.cv.disc(x*d.k, y*d.k, maxf(1.2, r*d.k*0.42), col)
		}
	}
}

// bubbleZ 第 j 条系列第 i 个点的气泡半径权重（提示 / 命中用）。
func (c *Chart) bubbleZ(j, i int) float64 {
	if j < 0 || j >= len(c.Series) {
		return 0
	}
	s := c.Series[j]
	if i < 0 || i >= len(s.Z) {
		return 0
	}
	return s.Z[i]
}

// ---- 瀑布图 ChartWaterfall ----

// waterfallBase 瀑布图累计起点（Chart.Min，默认 0）。
func (c *Chart) waterfallBase() float64 { return c.Min }

// waterfallStep 第 i 个类目的起止累计值（多系列时按系列顺序各走一段）。
func (c *Chart) waterfallStep(j, i int) (from, to float64) {
	base := c.waterfallBase()
	acc := base
	for k := 0; k < i; k++ {
		if i < len(c.Labels) || k < len(c.Series[j].Values) {
			if k < len(c.Series[j].Values) {
				acc += c.Series[j].Values[k]
			}
		}
	}
	v := 0.0
	if i < len(c.Series[j].Values) {
		v = c.Series[j].Values[i]
	}
	return acc, acc + v
}

// paintChartWaterfall 瀑布图：每根柱从上一累计画到本步累计，涨跌两色。
func (a *App) paintChartWaterfall(c *Chart, lg chartLayout) {
	n := c.points()
	ns := len(c.Series)
	if n <= 0 || ns <= 0 || lg.plot.W <= 0 || lg.plot.H <= 0 {
		return
	}
	band := lg.bandW(n)
	groupW := band * (1 - chartBarGap)
	if groupW < 1 {
		groupW = 1
	}
	barW := groupW / float32(ns)
	gap := minf(barW*0.18, 3)
	rad := [4]float32{}
	if r := minf(barW, 12) * 0.2; r > 1 {
		rad = [4]float32{r, r, r, r}
	}
	for j := 0; j < ns; j++ {
		for i := 0; i < n; i++ {
			from, to := c.waterfallStep(j, i)
			y0, y1 := lg.mapY(from), lg.mapY(to)
			if y1 < y0 {
				y0, y1 = y1, y0
			}
			h := y1 - y0
			if h < 1 {
				h = 1
			}
			cx := lg.xAt(i, n)
			x := cx - groupW/2 + float32(j)*barW + gap/2
			p := c.candlePaint(Candle{Open: from, Close: to})
			to2 := gpu.Paint{R: p.R * 0.72, G: p.G * 0.72, B: p.B * 0.72, A: p.A}
			a.r.Rect(gpu.Shape{
				Rect:   geom.Rect{X: x, Y: y0, W: maxf(barW-gap, 1), H: h},
				Radius: rad,
				Grad:   gpu.Gradient{On: true, Angle: 90, From: p, To: to2},
			})
		}
	}
}

// waterfallHit 瀑布图命中：按类目列 + 系列分栏。
func waterfallHit(c *Chart, lg chartLayout, mx, my float32) (int, int, bool) {
	n := c.points()
	ns := len(c.Series)
	if n <= 0 || ns <= 0 || !lg.plot.Contains(geom.Point{X: mx, Y: my}) {
		return 0, 0, false
	}
	i := int((mx - lg.plot.X) / maxf(lg.bandW(n), 0.001))
	if i < 0 {
		i = 0
	}
	if i >= n {
		i = n - 1
	}
	band := lg.bandW(n)
	groupW := band * (1 - chartBarGap)
	j := int((mx - (lg.xAt(i, n) - groupW/2)) / maxf(groupW/float32(ns), 0.001))
	if j < 0 {
		j = 0
	}
	if j >= ns {
		j = ns - 1
	}
	return j, i, true
}

// ---- 箱线图 ChartBox ----

// boxStat 五数概括。
type boxStat struct {
	min, q1, med, q3, max float64
}

// quantile 线性插值分位数（sorted 必须是升序）。
func quantile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := p * float64(len(sorted)-1)
	i := int(math.Floor(pos))
	f := pos - float64(i)
	if i+1 < len(sorted) {
		return sorted[i]*(1-f) + sorted[i+1]*f
	}
	return sorted[i]
}

// boxStats 第 j 条系列的五数概括。
//
// Values 恰好 5 个时按 [min, Q1, 中位, Q3, max] 直接使用（调用方已算好统计量）；
// 否则把 Values 当原始样本自动计算。
func (c *Chart) boxStats(j int) (boxStat, bool) {
	if j < 0 || j >= len(c.Series) {
		return boxStat{}, false
	}
	s := c.Series[j]
	if len(s.Values) == 5 {
		return boxStat{min: s.Values[0], q1: s.Values[1], med: s.Values[2], q3: s.Values[3], max: s.Values[4]}, true
	}
	if len(s.Values) == 0 {
		return boxStat{}, false
	}
	sorted := append([]float64(nil), s.Values...)
	sortFloats(sorted)
	return boxStat{
		min: sorted[0],
		q1:  quantile(sorted, 0.25),
		med: quantile(sorted, 0.5),
		q3:  quantile(sorted, 0.75),
		max: sorted[len(sorted)-1],
	}, true
}

// sortFloats 升序（样本量小，插入排序即可，避免额外依赖）。
func sortFloats(v []float64) {
	for i := 1; i < len(v); i++ {
		x := v[i]
		k := i - 1
		for k >= 0 && v[k] > x {
			v[k+1] = v[k]
			k--
		}
		v[k+1] = x
	}
}

// paintChartBox 箱线图：须 + 箱体 + 中位线，每系列一个箱。
func (a *App) paintChartBox(c *Chart, lg chartLayout) {
	n := len(c.Series)
	if n <= 0 || lg.plot.W <= 0 || lg.plot.H <= 0 {
		return
	}
	band := lg.bandW(n)
	boxW := minf(band*0.52, 46)
	if boxW < 3 {
		boxW = 3
	}
	for j := 0; j < n; j++ {
		st, ok := c.boxStats(j)
		if !ok {
			continue
		}
		cx := lg.xAt(j, n)
		p := c.colorOf(j)
		fill := p
		fill.A *= 0.32
		yMax, yQ3 := lg.mapY(st.max), lg.mapY(st.q3)
		yMed, yQ1 := lg.mapY(st.med), lg.mapY(st.q1)
		yMin := lg.mapY(st.min)
		// 须
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx - 0.75, Y: yMax, W: 1.5, H: maxf(yMin-yMax, 1)}, Fill: p})
		// 须帽
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx - boxW*0.26, Y: yMax - 1, W: boxW * 0.52, H: 2}, Fill: p})
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx - boxW*0.26, Y: yMin - 1, W: boxW * 0.52, H: 2}, Fill: p})
		// 箱体
		a.r.Rect(gpu.Shape{
			Rect:   geom.Rect{X: cx - boxW/2, Y: yQ3, W: boxW, H: maxf(yQ1-yQ3, 2)},
			Radius: [4]float32{3, 3, 3, 3},
			Fill:   fill,
		})
		// 中位线
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx - boxW/2, Y: yMed - 1, W: boxW, H: 2}, Fill: p})
	}
}

// boxHit 箱线图命中：按 x 定位箱（系列下标即箱下标）。
func boxHit(c *Chart, lg chartLayout, mx, my float32) (int, int, bool) {
	n := len(c.Series)
	if n <= 0 || !lg.plot.Contains(geom.Point{X: mx, Y: my}) {
		return 0, 0, false
	}
	j := int((mx - lg.plot.X) / maxf(lg.bandW(n), 0.001))
	if j < 0 {
		j = 0
	}
	if j >= n {
		j = n - 1
	}
	if _, ok := c.boxStats(j); !ok {
		return 0, 0, false
	}
	return j, 0, true
}

// boxTip 箱线图提示文案。
func (c *Chart) boxTip(j int) string {
	st, ok := c.boxStats(j)
	if !ok {
		return ""
	}
	name := ""
	if j < len(c.Series) {
		name = c.Series[j].Name
	}
	if name == "" && j < len(c.Labels) {
		name = c.Labels[j]
	}
	return fmt.Sprintf("%s  min %s  Q1 %s  中位 %s  Q3 %s  max %s",
		name, fmtNum(st.min), fmtNum(st.q1), fmtNum(st.med), fmtNum(st.q3), fmtNum(st.max))
}

// waterfallTip 瀑布图提示文案（本步增减 + 累计）。
func (c *Chart) waterfallTip(j, i int) string {
	if j < 0 || j >= len(c.Series) {
		return ""
	}
	from, to := c.waterfallStep(j, i)
	head := ""
	if i >= 0 && i < len(c.Labels) {
		head = c.Labels[i] + " · "
	}
	name := c.Series[j].Name
	if name != "" {
		name += "  "
	}
	return fmt.Sprintf("%s%s%+.1f（累计 %s）", head, name, to-from, fmtNum(to))
}

// ---- K 线量能副图 ----

// paintChartVolume K 线量能副图：底部按成交量画柱（原生矩形，颜色跟涨跌）。
func (a *App) paintChartVolume(c *Chart, lg chartLayout) {
	vp := lg.volPlot
	if vp.H <= 2 || vp.W <= 2 {
		return
	}
	maxV := 0.0
	for _, s := range c.Series {
		for _, k := range s.Candles {
			if k.Volume > maxV {
				maxV = k.Volume
			}
		}
	}
	if maxV <= 0 {
		return
	}
	n := c.points()
	band := lg.bandW(n)
	bodyW := minf(band*0.68, 22)
	if bodyW < 1 {
		bodyW = 1
	}
	zero := gpu.Paint{R: 1, G: 1, B: 1, A: 0.12}
	a.r.Rect(gpu.Shape{Rect: geom.Rect{X: vp.X, Y: vp.Bottom() - 1, W: vp.W, H: 1}, Fill: zero})
	for i := 0; i < n; i++ {
		k, ok := c.candleOf(0, i)
		if !ok {
			continue
		}
		h := float32(k.Volume/maxV) * (vp.H - 2)
		if h < 1 {
			h = 1
		}
		p := c.candlePaint(k)
		p.A *= 0.75
		a.r.Rect(gpu.Shape{Rect: geom.Rect{
			X: lg.xAt(i, n) - bodyW/2,
			Y: vp.Bottom() - h,
			W: bodyW,
			H: h,
		}, Fill: p})
	}
	if f := a.fonts.get(chartTickSize, 400); f != nil {
		a.r.TextRun(gpu.TextRun{
			X:     vp.X + 2,
			Y:     vp.Y + 1,
			Text:  "量 " + fmtNum(maxV),
			Font:  f,
			Color: C(ColTextMuted),
		})
	}
}

// ---- 分时图 ChartTimeshare ----

// timeshare 分时图：价格线 + 基准线（Chart.Min，如昨收），价格线与基准之间填充。
func (d *chartDraw) timeshare(c *Chart) {
	n := c.points()
	if n <= 0 {
		return
	}
	pl := d.lg.plot
	base := c.Min
	baseY := float32(0)
	if base != 0 {
		baseY = (d.lg.mapY(base) - pl.Y) * d.k
		ghost := C(ColText)
		ghost.A = 0.18
		d.cv.line(fpt{0, baseY}, fpt{pl.W * d.k, baseY}, 1, ghost)
	} else {
		baseY = pl.H * d.k
	}
	for j := range c.Series {
		s := c.Series[j]
		if len(s.Values) == 0 {
			continue
		}
		col := C(candleUpColor)
		last := s.Values[len(s.Values)-1]
		if base != 0 && last < base {
			col = C(candleDownColor)
		}
		if s.Color != "" {
			col = C(s.Color)
		}
		seg := make([]fpt, 0, n)
		for i, v := range s.Values {
			if i >= n {
				break
			}
			seg = append(seg, d.pt(d.lg.xAt(i, n)-pl.X, d.lg.mapY(v)-pl.Y))
		}
		if len(seg) < 2 {
			continue
		}
		fill := col
		fill.A *= 0.18
		poly := make([]fpt, 0, len(seg)+2)
		poly = append(poly, seg...)
		poly = append(poly, fpt{seg[len(seg)-1].X, baseY}, fpt{seg[0].X, baseY})
		d.cv.fillPoly(poly, fill)
		d.cv.strokePoly(seg, d.lineWidth(), col, false)
	}
}

// ---- 热力图 ChartHeatmap ----

// heatGrid 热力图的行列数（Cols <= 0 时按方形推导）。
func (c *Chart) heatGrid() (cols, rows int) {
	if len(c.Series) == 0 {
		return 0, 0
	}
	L := len(c.Series[0].Values)
	if L == 0 {
		return 0, 0
	}
	cols = c.Cols
	if cols <= 0 {
		cols = int(math.Round(math.Sqrt(float64(L))))
		if cols < 1 {
			cols = 1
		}
	}
	rows = (L + cols - 1) / cols
	return cols, rows
}

// paintChartHeatmap 热力图：行列色块（原生矩形，透明度按数值线性映射）。
func (a *App) paintChartHeatmap(c *Chart, lg chartLayout) {
	cols, rows := c.heatGrid()
	vals := c.plotValues(0)
	if cols <= 0 || rows <= 0 || len(vals) == 0 || lg.plot.W <= 0 || lg.plot.H <= 0 {
		return
	}
	maxV := 0.0
	for _, v := range vals {
		if v > maxV {
			maxV = v
		}
	}
	if maxV <= 0 {
		maxV = 1
	}
	cw := lg.plot.W / float32(cols)
	ch := lg.plot.H / float32(rows)
	base := C(ColPrimary)
	for r := 0; r < rows; r++ {
		for col := 0; col < cols; col++ {
			i := r*cols + col
			if i >= len(vals) {
				break
			}
			p := base
			p.A = float32(0.06 + 0.9*(vals[i]/maxV))
			a.r.Rect(gpu.Shape{
				Rect: geom.Rect{
					X: lg.plot.X + float32(col)*cw + 1,
					Y: lg.plot.Y + float32(r)*ch + 1,
					W: maxf(cw-2, 1),
					H: maxf(ch-2, 1),
				},
				Radius: [4]float32{3, 3, 3, 3},
				Fill:   p,
			})
		}
	}
}

// heatHit 热力图命中：按行列定位格子。
func heatHit(c *Chart, lg chartLayout, mx, my float32) (int, int, bool) {
	cols, rows := c.heatGrid()
	if cols <= 0 || rows <= 0 || !lg.plot.Contains(geom.Point{X: mx, Y: my}) {
		return 0, 0, false
	}
	col := int((mx - lg.plot.X) / maxf(lg.plot.W/float32(cols), 0.001))
	r := int((my - lg.plot.Y) / maxf(lg.plot.H/float32(rows), 0.001))
	if col < 0 {
		col = 0
	}
	if col >= cols {
		col = cols - 1
	}
	if r < 0 {
		r = 0
	}
	if r >= rows {
		r = rows - 1
	}
	i := r*cols + col
	if i >= len(c.plotValues(0)) {
		return 0, 0, false
	}
	return 0, i, true
}

// ---- 漏斗图 ChartFunnel ----

// funnel 漏斗图：每系列一层，宽度按数值递减，中心对齐的梯形。
func (d *chartDraw) funnel(c *Chart) {
	n := len(c.Series)
	if n <= 0 {
		return
	}
	sums := make([]float64, n)
	maxV := 0.0
	for j, s := range c.Series {
		sum := 0.0
		for _, v := range s.Values {
			if v > 0 {
				sum += v
			}
		}
		sums[j] = sum
		if sum > maxV {
			maxV = sum
		}
	}
	if maxV <= 0 {
		return
	}
	pl := d.lg.plot
	h := pl.H / float32(n)
	mid := pl.W / 2
	for j := 0; j < n; j++ {
		wTop := pl.W * float32(sums[j]/maxV) * 0.92
		wBot := wTop * 0.82
		if j+1 < n {
			wBot = pl.W * float32(sums[j+1]/maxV) * 0.92
		}
		y0 := float32(j) * h
		y1 := y0 + maxf(h-2, 1)
		pts := []fpt{
			d.pt(mid-wTop/2, y0),
			d.pt(mid+wTop/2, y0),
			d.pt(mid+wBot/2, y1),
			d.pt(mid-wBot/2, y1),
		}
		d.cv.fillPoly(pts, c.colorOf(j))
	}
}

// funnelHit 漏斗图命中：按纵向均分层级。
func funnelHit(c *Chart, lg chartLayout, mx, my float32) (int, int, bool) {
	n := len(c.Series)
	if n <= 0 || !lg.plot.Contains(geom.Point{X: mx, Y: my}) {
		return 0, 0, false
	}
	j := int((my - lg.plot.Y) / maxf(lg.plot.H/float32(n), 0.001))
	if j < 0 {
		j = 0
	}
	if j >= n {
		j = n - 1
	}
	return j, 0, true
}

// ---- 仪表盘 ChartGauge ----

// gauge 仪表盘：270° 圆弧背景 + 按比例覆盖的值弧。
func (d *chartDraw) gauge(c *Chart) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		return
	}
	v := c.Series[0].Values[0]
	lo, hi := d.lg.yMin, d.lg.yMax
	if hi <= lo {
		hi = lo + 1
	}
	t := float32((v - lo) / (hi - lo))
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	cx, cy := d.lg.pieC.X*d.k, d.lg.pieC.Y*d.k
	r1 := d.lg.pieR * d.k
	r0 := r1 * 0.72
	start := float32(-math.Pi * 0.75)
	sweep := float32(math.Pi * 1.5)
	bg := C(ColText)
	bg.A = 0.12
	d.sector(cx, cy, r0, r1, start, start+sweep, bg)
	if t > 0.002 {
		d.sector(cx, cy, r0, r1, start, start+sweep*t, c.colorOf(0))
	}
}

// gaugeValue 仪表盘当前值。
func (c *Chart) gaugeValue() float64 {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		return 0
	}
	return c.Series[0].Values[0]
}

// paintGaugeText 仪表盘中心数值。
func (a *App) paintGaugeText(c *Chart, lg chartLayout) {
	f := a.fonts.get(22, 700)
	if f == nil {
		return
	}
	s := fmtNum(c.gaugeValue()) + c.Unit
	w := f.Measure(s)
	a.r.TextRun(gpu.TextRun{
		X:     lg.pieC.X - w/2,
		Y:     lg.pieC.Y - f.LineHeight/2,
		Text:  s,
		Font:  f,
		Color: C(ColText),
	})
}

// hideAxisTicks 这些类型不画直角坐标刻度（热力图 / 漏斗 / 仪表盘）。
func (c *Chart) hideAxisTicks() bool {
	switch c.Type {
	case ChartHeatmap, ChartFunnel, ChartGauge:
		return true
	}
	return false
}
