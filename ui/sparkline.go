// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 迷你趋势图：KPI 卡片里的小折柱。用一组细矩形拼成，不依赖图表管线与纹理缓存，
// 也不需要 Key/状态（纯展示，随父节点每帧重建即可）。

// SparkOptions 迷你趋势图选项。
type SparkOptions struct {
	Width         float32 // 总宽，默认 96
	Height        float32 // 总高，默认 28
	Color         string  // 默认 ColPrimary
	Bars          int     // 柱数，默认按宽度 / 6 估算（3~60）
	HighlightLast bool    // 末柱高亮（强调当前值）
	Baseline      bool    // 底部基线
}

// Sparkline 迷你趋势图。values 为空时返回占位基线。
func Sparkline(values []float64, opts SparkOptions) *Node {
	if opts.Width <= 0 {
		opts.Width = 96
	}
	if opts.Height <= 0 {
		opts.Height = 28
	}
	if opts.Color == "" {
		opts.Color = ColPrimary
	}
	bars := opts.Bars
	if bars <= 0 {
		bars = int(opts.Width / 6)
	}
	if bars < 3 {
		bars = 3
	}
	if bars > 60 {
		bars = 60
	}

	series := sparkResample(values, bars)
	lo, hi := sparkRange(series)
	span := hi - lo
	if span <= 0 {
		span = 1
	}

	gap := float32(2)
	barW := (opts.Width - gap*float32(len(series)-1)) / float32(len(series))
	if barW < 1 {
		gap, barW = 1, 1
	}

	cells := make([]*Node, 0, len(series))
	for i, v := range series {
		ratio := float32(0)
		if len(series) > 0 {
			ratio = float32((v - lo) / span)
		}
		h := 2 + ratio*(opts.Height-2)
		c := opts.Color
		if opts.HighlightLast && i == len(series)-1 {
			c = ColPrimary2
		}
		cells = append(cells, Box().St(St().W(barW).H(h).R(1).BgC(c)))
		if i < len(series)-1 {
			cells = append(cells, Box().St(St().W(gap)))
		}
	}

	n := Row(cells...).St(St().W(opts.Width).H(opts.Height)).AlignTo(AlignEnd)
	if !opts.Baseline {
		return n
	}
	return Stack(
		Box().St(St().AbsFill().AbsBottom(0).H(1).BgC(ColStroke)),
		n,
	).St(St().W(opts.Width).H(opts.Height))
}

// sparkResample 把序列重采样到 n 个点（多则分桶平均，少则原样保留）。
func sparkResample(values []float64, n int) []float64 {
	if len(values) == 0 {
		return make([]float64, n)
	}
	if len(values) <= n {
		return values
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		start := i * len(values) / n
		end := (i + 1) * len(values) / n
		if end <= start {
			end = start + 1
		}
		sum := 0.0
		for _, v := range values[start:end] {
			sum += v
		}
		out[i] = sum / float64(end-start)
	}
	return out
}

// sparkRange 返回序列最小/最大值。
func sparkRange(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	lo, hi := values[0], values[0]
	for _, v := range values[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return lo, hi
}
