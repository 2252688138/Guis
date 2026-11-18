// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"guis/internal/geom"
	"guis/internal/gpu"
)

// 图表交互的事件侧：滚轮缩放、拖拽平移、图例点击隐藏、十字光标。
//
// 全部只在 Chart.Interactive = true 时生效（默认关闭，页面滚动与拖拽不受影响）。
// 状态本身存在 chart_interact.go 的按 Key 表里，这里只负责把鼠标事件翻译成状态变更。

// chartLegendHit 图例项的命中区（绘制时记录，点击时查询）。
type chartLegendHit struct {
	r     geom.Rect
	index int
}

// chartInteractiveNode 从命中节点向上找「开启了交互的图表节点」。
func chartInteractiveNode(a *App, x, y float32) *Node {
	if a == nil || a.root == nil {
		return nil
	}
	for c := hitTest(a.root, x, y, 0, a); c != nil; c = c.Parent {
		if c.Kind == KChart && c.Chart != nil && c.Chart.Interactive && c.Key != "" {
			return c
		}
	}
	return nil
}

// chartZoomStep 以可视窗口中心为锚点缩放（dy>0 放大，dy<0 缩小，steps 为档数）。
func chartZoomStep(n *Node, dy, steps float32) {
	if n == nil || n.Chart == nil {
		return
	}
	if dy == 0 {
		return
	}
	span, start := ChartZoomOf(n.Key)
	center := start + span/2
	next := span
	const minSpan = 0.02
	for i := float32(0); i < steps; i++ {
		if dy > 0 {
			next /= 1.25
		} else {
			next *= 1.25
		}
	}
	if next > 1 {
		next = 1
	}
	if next < minSpan {
		next = minSpan
	}
	ChartZoom(n.Key, next, center-next/2)
}

// chartPanBy 拖拽平移：dx 为逻辑像素位移（向右拖 = 看更早的数据）。
func chartPanBy(n *Node, dx float32) {
	if n == nil || n.Frame.W <= 0 || dx == 0 {
		return
	}
	span, start := ChartZoomOf(n.Key)
	if span >= 0.999 {
		return // 全量视图没有可平移空间
	}
	d := float64(dx) / float64(n.Frame.W) * span
	ChartZoom(n.Key, span, start-d)
}

// chartLegendClick 命中图例时切换系列显隐，返回是否命中。
func (a *App) chartLegendClick(n *Node, x, y float32) bool {
	if a == nil || n == nil || n.Chart == nil || n.Key == "" {
		return false
	}
	for _, h := range a.chartLegendHits[n.Key] {
		if x >= h.r.X && x <= h.r.Right() && y >= h.r.Y && y <= h.r.Bottom() {
			if h.index >= 0 && h.index < len(n.Chart.Series) {
				ChartToggleSeries(n.Key, h.index)
			}
			return true
		}
	}
	return false
}

// chartCrosshair 鼠标在绘图区内时画十字光标 + 最近采样点高亮。
func (a *App) chartCrosshair(c *Chart, lg chartLayout) {
	if c == nil || !a.mouseIn || lg.plot.W <= 0 || lg.plot.H <= 0 {
		return
	}
	mx, my := a.mouseX, a.mouseY
	if mx < lg.plot.X || mx > lg.plot.Right() || my < lg.plot.Y || my > lg.plot.Bottom() {
		return
	}
	line := gpu.Paint{R: 1, G: 1, B: 1, A: 0.16}
	const dash, gap = 4.0, 4.0
	// 垂直虚线
	for y := lg.plot.Y; y < lg.plot.Bottom(); y += dash + gap {
		y2 := y + float32(dash)
		if y2 > lg.plot.Bottom() {
			y2 = lg.plot.Bottom()
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: mx - 0.5, Y: y, W: 1, H: y2 - y}, Fill: line})
	}
	// 水平虚线
	for x := lg.plot.X; x < lg.plot.Right(); x += dash + gap {
		x2 := x + float32(dash)
		if x2 > lg.plot.Right() {
			x2 = lg.plot.Right()
		}
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: x, Y: my - 0.5, W: x2 - x, H: 1}, Fill: line})
	}
	// 最近采样点
	hs, hi, ok := chartHit(c, lg, mx, my)
	if !ok {
		return
	}
	v := c.seriesBase(hs, hi)
	if v != v { // NaN
		return
	}
	cx, cy := lg.xAt(hi, c.points()), lg.mapY(v)
	p := c.colorOf(hs)
	a.r.Rect(gpu.Shape{Rect: geom.Rect{X: cx - 4, Y: cy - 4, W: 8, H: 8}, Radius: [4]float32{4, 4, 4, 4},
		Fill: p, Stroke: 1, StrokeColor: gpu.Paint{R: 1, G: 1, B: 1, A: 0.9}})
}
