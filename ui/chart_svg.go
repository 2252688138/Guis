// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// SVG 导出：把图表数据渲染成矢量图（不依赖 GL / 显卡 / 截屏）。
//
//	svg := c.SVG(640, 320)          // 拿到文本，可嵌网页 / 存文档
//	_ = c.SaveSVG("chart.svg", 640, 320)
//
// 覆盖常用笛卡尔图型（折线 / 面积 / 柱状 / 散点 / K 线 / 分时）；饼图、雷达等
// 极坐标图型超出 v1 范围，会导出一张带说明的占位图（明确提示，不静默出错）。
// 与屏幕渲染保持一致的地方：标题、单位、网格、Y 轴刻度、X 轴标签、图例配色。

const (
	svgPadL = 46
	svgPadR = 16
	svgPadT = 26
	svgPadB = 34
	// svgMaxPts 折线最多采样点数：再多就按等距抽稀（SVG 文件不该无限膨胀）。
	svgMaxPts = 800
)

// SVGOk 该图型是否支持矢量导出。
func (c *Chart) SVGOk() bool {
	switch c.Type {
	case ChartLine, ChartArea, ChartBar, ChartScatter, ChartBubble, ChartCandle, ChartTimeshare:
		return true
	}
	return false
}

// SVG 生成 SVG 文本（w/h 为画布尺寸，<=0 时用 640×320）。
func (c *Chart) SVG(w, h int) string {
	if c == nil {
		return ""
	}
	c.applyDefaults()
	if w <= 0 {
		w = 640
	}
	if h <= 0 {
		h = 320
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="Segoe UI, Microsoft YaHei UI, sans-serif">`+"\n",
		w, h, w, h)
	fmt.Fprintf(&sb, `<title>%s</title>`+"\n", xmlEscape(c.Title))
	fmt.Fprintf(&sb, `<rect width="%d" height="%d" fill="%s"/>`+"\n", w, h, hexOf(C(ColBg)))

	if !c.SVGOk() {
		fmt.Fprintf(&sb, `<text x="%d" y="%d" fill="%s" font-size="13" text-anchor="middle">该图型暂不支持矢量导出</text>`+"\n",
			w/2, h/2, hexOf(C(ColTextMuted)))
		sb.WriteString("</svg>\n")
		return sb.String()
	}

	// 绘图区
	padB := svgPadB
	if c.ShowLegend {
		padB += 18
	}
	px, py := float64(svgPadL), float64(svgPadT)
	pw, ph := float64(w-svgPadL-svgPadR), float64(h-svgPadT-padB)
	if pw <= 1 || ph <= 1 {
		sb.WriteString("</svg>\n")
		return sb.String()
	}

	if c.Title != "" {
		fmt.Fprintf(&sb, `<text x="%d" y="%d" fill="%s" font-size="13" font-weight="600">%s</text>`+"\n",
			svgPadL, 17, hexOf(C(ColText)), xmlEscape(c.Title))
	}

	lo, hi := c.svgRange()
	yAt := func(v float64) float64 { return py + ph*(1-(v-lo)/(hi-lo)) }
	n := c.points()

	// 网格 + Y 轴刻度
	ticks := c.YTicks
	if ticks <= 0 {
		ticks = 4
	}
	if c.ShowGrid || c.ShowAxis {
		for i := 0; i <= ticks; i++ {
			v := lo + (hi-lo)*float64(i)/float64(ticks)
			y := yAt(v)
			if c.ShowGrid {
				fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`+"\n",
					px, y, px+pw, y, hexOf(C(ColStroke)))
			}
			fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" font-size="10" text-anchor="end">%s</text>`+"\n",
				px-8, y+3.5, hexOf(C(ColTextMuted)), xmlEscape(formatTick(v)))
		}
	}

	// X 轴标签（最多 12 个，等距抽）
	if len(c.Labels) > 0 && n > 0 {
		maxLabels := 12
		step := 1
		if n > maxLabels {
			step = (n + maxLabels - 1) / maxLabels
		}
		for i := 0; i < n && i < len(c.Labels); i += step {
			x := svgX(i, n, c.Type, px, pw)
			fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" font-size="10" text-anchor="middle">%s</text>`+"\n",
				x, py+ph+16, hexOf(C(ColTextMuted)), xmlEscape(c.Labels[i]))
		}
	}

	// 系列
	for si, s := range c.Series {
		col := hexOf(c.colorOf(si))
		switch c.Type {
		case ChartCandle:
			c.svgCandles(&sb, s, yAt, n, px, pw)
		case ChartBar:
			step := pw / float64(max1(n))
			bw := step * 0.6
			for i, v := range s.Values {
				x := px + (float64(i)+0.5)*step
				y0, y1 := yAt(0), yAt(v)
				if y1 > y0 {
					y0, y1 = y1, y0
				}
				fmt.Fprintf(&sb, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" rx="2"/>`+"\n",
					x-bw/2, y0, bw, math.Max(1, y1-y0), col)
			}
		case ChartScatter, ChartBubble:
			for i, v := range s.Values {
				x := svgX(i, n, c.Type, px, pw)
				r := 3.0
				if c.Type == ChartBubble && i < len(s.Z) && len(s.Z) > 0 {
					r = 3 + math.Mod(math.Abs(s.Z[i]), 10)
				}
				fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s" fill-opacity="0.8"/>`+"\n",
					x, yAt(v), r, col)
			}
		default: // 折线 / 面积 / 分时
			pts := svgPolyline(s.Values, yAt, n, c.Type, px, pw)
			if pts == "" {
				continue
			}
			if c.Type == ChartArea || c.Type == ChartTimeshare {
				fmt.Fprintf(&sb, `<path d="M%.1f %.1f L%s L%.1f %.1f Z" fill="%s" fill-opacity="0.16"/>`+"\n",
					svgX(0, n, c.Type, px, pw), yAt(0), pts, svgX(n-1, n, c.Type, px, pw), yAt(0), col)
			}
			fmt.Fprintf(&sb, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>`+"\n",
				pts, col)
		}
	}

	// 图例
	if c.ShowLegend && len(c.Series) > 0 {
		x := px
		y := py + ph + 26
		for i, s := range c.Series {
			name := s.Name
			if name == "" {
				name = fmt.Sprintf("系列 %d", i+1)
			}
			wbox := 10 + float64(len([]rune(name)))*12 + 14
			if x+wbox > px+pw {
				break
			}
			fmt.Fprintf(&sb, `<rect x="%.1f" y="%.1f" width="8" height="8" rx="2" fill="%s"/>`+"\n",
				x, y-7, hexOf(c.colorOf(i)))
			fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" font-size="10.5">%s</text>`+"\n",
				x+12, y, hexOf(C(ColTextDim)), xmlEscape(name))
			x += wbox
		}
	}

	sb.WriteString("</svg>\n")
	return sb.String()
}

// SaveSVG 导出 SVG 文件。
func (c *Chart) SaveSVG(path string, w, h int) error {
	return os.WriteFile(path, []byte(c.SVG(w, h)), 0o644)
}

// svgX 第 i 个采样点的 X 坐标（柱状取柱心，其余按端点均分）。
func svgX(i, n int, t ChartType, px, pw float64) float64 {
	if n <= 1 {
		return px + pw/2
	}
	if t == ChartBar {
		return px + (float64(i)+0.5)*pw/float64(n)
	}
	return px + float64(i)*pw/float64(n-1)
}

// svgPolyline 生成属性点串（超长自动等距抽稀）。
func svgPolyline(vals []float64, yAt func(float64) float64, n int, t ChartType, px, pw float64) string {
	if len(vals) == 0 {
		return ""
	}
	cnt := len(vals)
	step := 1
	if cnt > svgMaxPts {
		step = (cnt + svgMaxPts - 1) / svgMaxPts
	}
	var sb strings.Builder
	first := true
	for i := 0; i < cnt; i += step {
		if !first {
			sb.WriteByte(' ')
		}
		first = false
		fmt.Fprintf(&sb, "%.1f,%.1f", svgX(i, n, t, px, pw), yAt(vals[i]))
	}
	// 保证最后一个点不被抽掉（收尾形状不塌）
	if last := cnt - 1; (last)%step != 0 {
		fmt.Fprintf(&sb, " %.1f,%.1f", svgX(last, n, t, px, pw), yAt(vals[last]))
	}
	return sb.String()
}

// svgCandles 画 K 线（影线 + 实体）。
func (c *Chart) svgCandles(sb *strings.Builder, s Series, yAt func(float64) float64, n int, px, pw float64) {
	step := pw / float64(max1(n))
	bw := math.Max(1.5, step*0.6)
	up := c.CandleUp
	if up == "" {
		up = "#ef5350"
	}
	down := c.CandleDown
	if down == "" {
		down = "#26a69a"
	}
	for i, k := range s.Candles {
		x := px + (float64(i)+0.5)*step
		col := up
		top, bottom := k.Close, k.Open
		if k.Close < k.Open {
			col = down
			top, bottom = k.Open, k.Close
		}
		fmt.Fprintf(sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`+"\n",
			x, yAt(k.High), x, yAt(k.Low), col)
		fmt.Fprintf(sb, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`+"\n",
			x-bw/2, yAt(top), bw, math.Max(1, yAt(bottom)-yAt(top)), col)
	}
}

// svgRange Y 轴范围（Min/Max 显式指定优先，否则按数据自动并含 0）。
func (c *Chart) svgRange() (lo, hi float64) {
	if c.Max > c.Min {
		return c.Min, c.Max
	}
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, s := range c.Series {
		if len(s.Candles) > 0 {
			for _, k := range s.Candles {
				lo, hi = math.Min(lo, k.Low), math.Max(hi, k.High)
			}
			continue
		}
		for _, v := range s.Values {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	if math.IsInf(lo, 1) || math.IsInf(hi, -1) {
		return 0, 1
	}
	if lo > 0 {
		lo = 0
	}
	if hi < 0 {
		hi = 0
	}
	if hi-lo < 1e-9 {
		hi = lo + 1
	}
	return lo, hi
}

// formatTick 刻度文案（整数不带小数，大数带 k）。
func formatTick(v float64) string {
	av := math.Abs(v)
	switch {
	case av >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case av >= 1e4:
		return fmt.Sprintf("%.1fk", v/1e3)
	case av >= 10:
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.1f", v)
}


// xmlEscape 转义 XML 特殊字符。
func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// max1 至少为 1（避免除零）。
func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
