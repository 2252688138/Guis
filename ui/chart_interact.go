// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"sync"
)

// 图表交互状态：缩放窗口（span/start）+ 图例隐藏系列。
//
// 设计：**不改用户的 Chart**。交互状态按节点 Key 存在框架侧，绘制前用一个
// 浅拷贝 + 数据窗口切片得到「有效图表」（chartEffective），布局/绘制/缓存指纹
// 全都照原样工作 —— 缩放与隐藏系列因此对折线 / 面积 / 柱状 / 散点 / K 线 /
// 分时一致生效，且不需要在任何图型里插入分支。
//
//	ui.ChartZoom("sales", 0.35, 0.5)   // 只看后半段 35%
//	ui.ChartToggleSeries("sales", 1)   // 隐藏第 2 条系列
//	ui.ChartResetZoom("sales")
//
// 性能：只有在存在缩放/隐藏时才走拷贝（否则原对象直通，零开销）。

// chartInteract 单个图表的交互状态。
type chartInteract struct {
	span   float64 // 可视窗口占比（1 = 全量）
	start  float64 // 窗口起点（0..1）
	hidden map[int]bool
}

var (
	chartMu    sync.Mutex
	chartViews = map[string]*chartInteract{}
)

// chartStateOf 取（或创建）某 Key 的交互状态（调用方需持锁）。
func chartStateOfLocked(key string) *chartInteract {
	st := chartViews[key]
	if st == nil {
		st = &chartInteract{span: 1}
		chartViews[key] = st
	}
	return st
}

// ChartZoom 设置图表可视窗口（key 与图表节点 Key 一致）。
//
// span 为可视占比（0<span<=1，越小放大越多），start 为窗口起点（0..1）。
// 越界自动收敛；span>=0.999 等价于全量。图型不支持缩放（饼图 / 雷达等）时只
// 记录状态、绘制不受影响。
func ChartZoom(key string, span, start float64) {
	if key == "" {
		return
	}
	if span <= 0 || span > 1 {
		span = 1
	}
	if span >= 0.999 {
		span, start = 1, 0
	}
	if start < 0 {
		start = 0
	}
	if start+span > 1 {
		start = 1 - span
	}
	chartMu.Lock()
	st := chartStateOfLocked(key)
	st.span, st.start = span, start
	chartMu.Unlock()
	RequestRedraw()
}

// ChartZoomOf 读回可视窗口（未设置时返回 1, 0）。
func ChartZoomOf(key string) (span, start float64) {
	chartMu.Lock()
	defer chartMu.Unlock()
	if st := chartViews[key]; st != nil {
		return st.span, st.start
	}
	return 1, 0
}

// ChartResetZoom 恢复全量视图。
func ChartResetZoom(key string) { ChartZoom(key, 1, 0) }

// ChartSeriesHidden 某系列是否被图例隐藏。
func ChartSeriesHidden(key string, idx int) bool {
	chartMu.Lock()
	defer chartMu.Unlock()
	if st := chartViews[key]; st != nil {
		return st.hidden[idx]
	}
	return false
}

// ChartToggleSeries 切换某系列显隐，返回切换后的状态（true = 已隐藏）。
func ChartToggleSeries(key string, idx int) bool {
	if key == "" || idx < 0 {
		return false
	}
	chartMu.Lock()
	st := chartStateOfLocked(key)
	if st.hidden == nil {
		st.hidden = map[int]bool{}
	}
	st.hidden[idx] = !st.hidden[idx]
	hidden := st.hidden[idx]
	if !hidden {
		delete(st.hidden, idx)
	}
	chartMu.Unlock()
	RequestRedraw()
	return hidden
}

// ChartResetSeries 取消所有图例隐藏。
func ChartResetSeries(key string) {
	chartMu.Lock()
	if st := chartViews[key]; st != nil {
		st.hidden = nil
	}
	chartMu.Unlock()
	RequestRedraw()
}

// ChartClearInteract 丢弃某 Key 的全部交互状态（节点销毁时可调用）。
func ChartClearInteract(key string) {
	chartMu.Lock()
	delete(chartViews, key)
	chartMu.Unlock()
}

// chartZoomable 该图型是否支持「数据窗口缩放」（笛卡尔类：X 轴等距采样）。
func chartZoomable(t ChartType) bool {
	switch t {
	case ChartLine, ChartArea, ChartBar, ChartScatter, ChartCandle, ChartTimeshare, ChartBubble:
		return true
	}
	return false
}

// chartEffective 返回按交互状态处理后的图表；无状态时直接返回原对象。
func chartEffective(n *Node) *Chart {
	c := n.Chart
	if c == nil || n.Key == "" {
		return c
	}
	chartMu.Lock()
	st := chartViews[n.Key]
	span, start := 1.0, 0.0
	var hidden map[int]bool
	if st != nil {
		span, start = st.span, st.start
		if len(st.hidden) > 0 {
			hidden = make(map[int]bool, len(st.hidden))
			for k, v := range st.hidden {
				hidden[k] = v
			}
		}
	}
	chartMu.Unlock()
	zoomed := span > 0 && span < 0.999
	if !zoomed && len(hidden) == 0 {
		return c
	}
	return c.sliceView(start, span, hidden, zoomed)
}

// sliceView 浅拷贝图表并按数据窗口裁剪 + 移除隐藏系列。
//
// Series 是值类型，切片共享底层数组（只读，安全）；不修改原图表。
func (c *Chart) sliceView(start, span float64, hidden map[int]bool, zoomed bool) *Chart {
	cp := *c
	series := make([]Series, len(c.Series))
	copy(series, c.Series)
	cp.Series = series
	// 隐藏系列：保持下标不变（图例点击靠下标），把数据清空让绘制自然跳过
	for i := range cp.Series {
		if hidden[i] {
			cp.Series[i].Values = nil
			cp.Series[i].X = nil
			cp.Series[i].Z = nil
			cp.Series[i].Candles = nil
		}
	}

	if !zoomed || !chartZoomable(c.Type) {
		return &cp
	}
	pts := c.points()
	if pts <= 2 {
		return &cp
	}
	lo := int(start*float64(pts) + 0.5)
	hi := lo + int(span*float64(pts)+0.5)
	if hi-lo < 2 {
		hi = lo + 2
	}
	if lo < 0 {
		lo = 0
	}
	if hi > pts {
		hi = pts
	}
	if lo >= hi {
		return &cp
	}
	for i := range cp.Series {
		s := &cp.Series[i]
		if len(s.Values) == pts {
			s.Values = s.Values[lo:hi]
		} else if len(s.Values) >= hi {
			s.Values = s.Values[lo:hi]
		}
		if len(s.X) == pts {
			s.X = s.X[lo:hi]
		}
		if len(s.Z) == pts {
			s.Z = s.Z[lo:hi]
		}
		if len(s.Candles) == pts {
			s.Candles = s.Candles[lo:hi]
		}
	}
	if len(cp.Labels) == pts {
		cp.Labels = cp.Labels[lo:hi]
	}
	return &cp
}
