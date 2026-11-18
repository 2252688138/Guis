// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"sync/atomic"
	"time"

	"guis/internal/geom"
)

// 内置 DevTools（F12 打开）：节点树 / 帧耗时 / 重绘来源 / 缓存命中率。
//
// 设计要点：
//   - 面板就是一棵普通节点树，挂在窗口根 Stack 的最上层（与 Portal 全屏浮层
//     同一机制），所以 hover / 点击 / 滚轮 / 焦点全部复用既有逻辑，不需要
//     另开事件通道；
//   - 关闭时每帧只多一次 atomic.Load，采集与绘制全部跳过；
//   - 采集的是「上一帧」的数据：面板必须在 Layout 之前进树，因此展示数据滞后
//     一帧，视觉上无感，换来零侵入。
//
//	ui.SetDevTools(true)   // 代码里直接打开
//	F12                    // 运行时切换

const (
	devPanelW   = 380
	devSampleN  = 60
	devTreeRowH = 20
)

var (
	devOnFlag      atomic.Bool
	devTab         = "overview"
	devSelIdx      = -1
	devNodeList    []devNodeInfo
	devStatsCur    devStats
	devSamples     [devSampleN]devSample
	devSampleCount int
	devSampleNext  int
	devSelRect     geom.Rect
	devSelRectOK   bool
	devFrameNo     int
)

// DevToolsOn 内置调试面板是否打开。
func DevToolsOn() bool { return devOnFlag.Load() }

// SetDevTools 打开 / 关闭内置调试面板（改完立即重绘）。
func SetDevTools(on bool) {
	if devOnFlag.Swap(on) != on {
		if !on {
			devSelIdx, devSelRectOK = -1, false
			devSampleCount, devSampleNext = 0, 0
		}
		RequestRedraw()
	}
}

// ToggleDevTools 切换调试面板，返回切换后的状态。
func ToggleDevTools() bool {
	on := !devOnFlag.Load()
	SetDevTools(on)
	return on
}

// devSample 一帧的耗时采样（只保留最近 devSampleN 帧）。
type devSample struct {
	build float64 // view() + 布局
	paint float64
}

// devNodeInfo 节点树里的一行。
type devNodeInfo struct {
	n     *Node
	depth int
	abs   geom.Rect
	anim  bool
}

// devStats 面板展示的汇总指标（每帧刷新）。
type devStats struct {
	w, h        float32
	frameNo     int
	nodes       int
	interactive int
	animCount   int
	layerHit    int
	layerMiss   int
	wrapHit     uint64
	wrapMiss    uint64
	fonts       int
	atlasKB     int
	buildMs     float64
	paintMs     float64
	avgBuild    float64
	avgPaint    float64
	maxPaint    float64
	scrollAnim  bool
	pressed     bool
	clickPend   bool
}

// devtoolsWrap 把面板挂到窗口根 Stack 最上层（root 为 nil 时原样返回）。
func devtoolsWrap(root *Node, w, h float32) *Node {
	if root == nil {
		return root
	}
	panel := devtoolsPanel(w, h)
	if panel == nil {
		return root
	}
	return Stack(root, panel).St(St().Wp(100).Hp(100)).K("ui-devtools-root")
}

// devtoolsSample 采集本帧的节点树与指标（Layout 之后调用，Frame 已有效）。
func devtoolsSample(a *App, root *Node, w, h float32, buildMs float64) {
	devFrameNo++
	infos := make([]devNodeInfo, 0, 512)
	nodes, interactive, anim := 0, 0, 0
	var walk func(n *Node, depth int, ox, oy float32)
	walk = func(n *Node, depth int, ox, oy float32) {
		if n == nil || depth > 200 {
			return
		}
		nodes++
		abs := geom.Rect{
			X: n.Frame.X + ox,
			Y: n.Frame.Y + oy,
			W: n.Frame.W,
			H: n.Frame.H,
		}
		isAnim := n.Style.Animated
		if isAnim {
			anim++
		}
		if n.OnClick != nil || n.OnClickAt != nil || n.OnDrag != nil || n.OnActivate != nil {
			interactive++
		}
		infos = append(infos, devNodeInfo{n: n, depth: depth, abs: abs, anim: isAnim})
		for _, c := range n.Children {
			walk(c, depth+1, abs.X, abs.Y)
		}
	}
	walk(root, 0, 0, 0)
	devNodeList = infos

	st := devStats{
		w: w, h: h, frameNo: devFrameNo,
		nodes: nodes, interactive: interactive, animCount: anim,
		layerHit: a.layerHit, layerMiss: a.layerMiss,
		wrapHit: wrapCacheHits(), wrapMiss: wrapCacheMisses(),
		scrollAnim: a.isScrollAnimating(), pressed: a.pressed, clickPend: a.clickPending,
		buildMs: buildMs,
		// 采样环里的历史指标沿用上一帧算好的值
		avgBuild: devStatsCur.avgBuild, avgPaint: devStatsCur.avgPaint,
		maxPaint: devStatsCur.maxPaint,
	}
	if a.fonts != nil {
		st.fonts, st.atlasKB = len(a.fonts.fonts), a.fonts.atlasTotal()/1024
	}
	if devSelIdx >= len(devNodeList) {
		devSelIdx, devSelRectOK = -1, false
	}
	if devSelIdx >= 0 {
		devSelRect, devSelRectOK = devNodeList[devSelIdx].abs, true
	}
	devStatsCur = st
}

// devtoolsEnd 记录本帧耗时并刷新均值（paint 之后调用）。
func devtoolsEnd(paintMs float64) {
	devSamples[devSampleNext] = devSample{build: devStatsCur.buildMs, paint: paintMs}
	devSampleNext = (devSampleNext + 1) % devSampleN
	if devSampleCount < devSampleN {
		devSampleCount++
	}
	var sb, sp, maxP float64
	for i := 0; i < devSampleCount; i++ {
		s := devSamples[i]
		sb += s.build
		sp += s.paint
		if s.paint > maxP {
			maxP = s.paint
		}
	}
	if n := float64(devSampleCount); n > 0 {
		devStatsCur.avgBuild, devStatsCur.avgPaint = sb/n, sp/n
	}
	devStatsCur.maxPaint = maxP
	devStatsCur.paintMs = paintMs
}

// devSamplesInOrder 按时间先后返回采样（用于柱状图）。
func devSamplesInOrder() []devSample {
	out := make([]devSample, 0, devSampleCount)
	start := devSampleNext - devSampleCount
	if start < 0 {
		start += devSampleN
	}
	for i := 0; i < devSampleCount; i++ {
		out = append(out, devSamples[(start+i)%devSampleN])
	}
	return out
}

// devtoolsPanel 构建调试面板（含选中节点高亮框）。
func devtoolsPanel(w, h float32) *Node {
	st := devStatsCur
	head := Row(
		Text("DevTools").St(St().Text(13, 700, ColText)),
		Text(fmt.Sprintf("%d", st.frameNo)).Grow(1).
			St(St().Text(10.5, 400, ColTextDim)),
		Text("F12").St(St().Text(10, 500, ColTextMuted).PadXY(4, 0)),
		Text("×").Tap(func() { SetDevTools(false) }).
			St(St().Text(16, 500, ColTextMuted).PadXY(6, 0)),
	).St(St().GapOf(8)).AlignTo(AlignCenter)

	seg := Segmented(
		[]string{"overview", "tree", "repaint"},
		[]string{"概览", "节点树", "重绘"},
		devTab,
		func(k string) {
			devTab = k
			RequestRedraw()
		},
	)

	var body *Node
	switch devTab {
	case "tree":
		body = devTreePage(h)
	case "repaint":
		body = devRepaintPage(h)
	default:
		body = devOverviewPage()
	}

	panel := Box(
		Column(head, seg, body).St(St().GapOf(10).Padding(12).Hp(100)),
	).St(St().W(devPanelW).Hp(100).BgC(ColPanel).Stroke(1, ColStroke).
		Shadow(18, 4, 0, "rgba(0,0,0,0.45)").AbsTR(0, 0))

	kids := []*Node{panel}
	if devSelRectOK && devSelIdx >= 0 && devSelIdx < len(devNodeList) {
		r := devSelRect
		kids = append(kids, Box().St(St().AbsTL(r.X, r.Y).W(r.W).H(r.H).
			Stroke(1.5, ColDanger).R(3)))
	}
	return Stack(kids...).St(St().AbsFill())
}

// devRow 一行「标签 —— 值」。
func devRow(label, value string, color string) *Node {
	return Row(
		Text(label).St(St().Text(11, 400, ColTextMuted)).Grow(1),
		Text(value).St(St().Text(11.5, 600, color)),
	).St(St().GapOf(8)).AlignTo(AlignCenter)
}

// devSparkline 最近若干帧的耗时柱状图（底部对齐，绿/黄/红按预算分档）。
func devSparkline() *Node {
	ss := devSamplesInOrder()
	if len(ss) == 0 {
		return Text("还没有帧采样").St(St().Text(10.5, 400, ColTextDim))
	}
	const maxMs = 33.0 // 30fps 之外全部顶格
	bars := make([]*Node, 0, len(ss))
	for _, s := range ss {
		ms := s.build + s.paint
		ratio := ms / maxMs
		if ratio > 1 {
			ratio = 1
		}
		bh := float32(3 + ratio*33)
		col := ColSuccess
		if ms > 16.7 {
			col = ColWarn
		}
		if ms > 33 {
			col = ColDanger
		}
		bars = append(bars, Box().St(St().W(4).H(bh).BgC(col).R(1)))
	}
	return Row(bars...).St(St().GapOf(1).H(40)).AlignTo(AlignEnd)
}

// devOverviewPage 概览：帧耗时 + 规模 + 缓存命中率。
func devOverviewPage() *Node {
	st := devStatsCur
	layerTotal := st.layerHit + st.layerMiss
	layerPct := 0.0
	if layerTotal > 0 {
		layerPct = float64(st.layerHit) / float64(layerTotal) * 100
	}
	wrapTotal := st.wrapHit + st.wrapMiss
	wrapPct := 0.0
	if wrapTotal > 0 {
		wrapPct = float64(st.wrapHit) / float64(wrapTotal) * 100
	}
	frameMs := st.avgBuild + st.avgPaint
	fps := 0.0
	if frameMs > 0.01 {
		fps = 1000 / frameMs
	}
	col := func(v, warn, bad float64) string {
		switch {
		case v >= bad:
			return ColDanger
		case v >= warn:
			return ColWarn
		}
		return ColSuccess
	}
	_ = col
	return Scroll(
		Column(
			devSparkline(),
			devRow("窗口", fmt.Sprintf("%.0f × %.0f", st.w, st.h), ColText),
			devRow("帧号 / 理论 FPS", fmt.Sprintf("%d / %.0f", st.frameNo, fps), ColText),
			devRow("建树+布局", fmt.Sprintf("%.2f ms", st.buildMs), msColor(st.buildMs, 8)),
			devRow("绘制", fmt.Sprintf("%.2f ms", st.paintMs), msColor(st.paintMs, 8)),
			devRow("均值 建树/绘制", fmt.Sprintf("%.2f / %.2f ms", st.avgBuild, st.avgPaint), ColTextDim),
			devRow("最差绘制", fmt.Sprintf("%.2f ms", st.maxPaint), msColor(st.maxPaint, 16)),
			devRow("节点", fmt.Sprintf("%d（可交互 %d）", st.nodes, st.interactive), ColText),
			devRow("动画节点", fmt.Sprintf("%d", st.animCount), animColor(st.animCount)),
			devRow("图层缓存命中", fmt.Sprintf("%.0f%%（%d/%d）", layerPct, st.layerHit, layerTotal), hitColor(layerPct)),
			devRow("折行缓存命中", fmt.Sprintf("%.0f%%（%d/%d）", wrapPct, st.wrapHit, wrapTotal), hitColor(wrapPct)),
			devRow("字体 / 图集", fmt.Sprintf("%d / %d KB", st.fonts, st.atlasKB), ColTextDim),
			Text("F12 切换面板；节点树页签里点一行可高亮对应区域").
				St(St().Text(10.5, 400, ColTextDim).WrapOn()),
		).St(St().GapOf(6)),
	).St(St().H(bodyHeight(st.h)).ScrollbarOn())
}

// devTreePage 节点树：先序列出 Kind / Key / 绝对矩形，点击高亮。
func devTreePage(h float32) *Node {
	if len(devNodeList) == 0 {
		return EmptyState("", "暂无节点", "本帧还没有可展示的节点", "", nil)
	}
	listH := bodyHeight(h)
	return Column(
		VList("ui-devtools-tree", len(devNodeList), devTreeRowH, listH, func(i int) *Node {
			if i < 0 || i >= len(devNodeList) {
				return nil
			}
			info := devNodeList[i]
			n := info.n
			if n == nil {
				return nil
			}
			name := kindName(n.Kind)
			if n.Key != "" {
				name += " · " + n.Key
			}
			col := ColText
			if n.Key == "" {
				col = ColTextMuted
			}
			if info.anim {
				col = ColPrimary
			}
			if i == devSelIdx {
				col = ColDanger
			}
			row := Row(
				Box().St(St().W(float32(info.depth)*9)),
				Text(name).St(St().Text(10.5, 500, col).Clipped()).Grow(1),
				Text(fmt.Sprintf("%.0f,%.0f %.0fx%.0f", info.abs.X, info.abs.Y, info.abs.W, info.abs.H)).
					St(St().Text(9.5, 400, ColTextDim)),
			).St(St().GapOf(6)).AlignTo(AlignCenter).K(fmt.Sprintf("dn%d", i))
			sel := i
			row.Tap(func() {
				devSelIdx = sel
				if sel >= 0 && sel < len(devNodeList) {
					devSelRect, devSelRectOK = devNodeList[sel].abs, true
				}
				RequestRedraw()
			})
			return row
		}),
	).St(St().GapOf(6))
}

// devRepaintPage 重绘来源：动画节点、输入状态、缓存命中。
func devRepaintPage(h float32) *Node {
	st := devStatsCur
	anims := make([]*Node, 0, 24)
	for _, info := range devNodeList {
		if !info.anim || info.n == nil {
			continue
		}
		k := info.n.Key
		if k == "" {
			k = "(无 Key)"
		}
		anims = append(anims, Text(fmt.Sprintf("%s · %s", kindName(info.n.Kind), k)).
			St(St().Text(10.5, 500, ColPrimary)))
		if len(anims) >= 24 {
			break
		}
	}
	block := func(label string, v *Node) *Node {
		return Column(
			Text(label).St(St().Text(10.5, 600, ColTextMuted)),
			v,
		).St(St().GapOf(4))
	}
	flag := func(on bool, text string) *Node {
		c := ColTextDim
		if on {
			c = ColSuccess
		}
		return Row(Dot(c, 7), Text(text).St(St().Text(10.5, 500, ColText))).
			St(St().GapOf(6)).AlignTo(AlignCenter)
	}
	animList := Text("本帧没有动画节点").St(St().Text(10.5, 400, ColTextDim))
	if len(anims) > 0 {
		animList = Column(anims...).St(St().GapOf(3))
	}
	return Scroll(
		Column(
			flag(st.animCount > 0, fmt.Sprintf("动画节点 %d 个（持续重绘）", st.animCount)),
			flag(st.scrollAnim, "滚动惯性动画进行中"),
			flag(st.pressed, "鼠标按下中"),
			flag(st.clickPend, "有待处理的点击"),
			block("动画节点清单", animList),
			block("缓存", Column(
				devRow("图层", fmt.Sprintf("命中 %d / 未命中 %d", st.layerHit, st.layerMiss), ColText),
				devRow("折行", fmt.Sprintf("命中 %d / 未命中 %d", st.wrapHit, st.wrapMiss), ColText),
			).St(St().GapOf(4))),
			Text("提示：GUIS_FPS=1 会把同样的数据按秒打到 stdout").
				St(St().Text(10, 400, ColTextDim).WrapOn()),
		).St(St().GapOf(10)),
	).St(St().H(bodyHeight(h)).ScrollbarOn())
}

// bodyHeight 面板内容区高度（扣掉标题与页签）。
func bodyHeight(h float32) float32 {
	v := h - 150
	if v < 120 {
		v = 120
	}
	return v
}

// msColor 按耗时给颜色（warn 为黄色阈值，两倍为红色）。
func msColor(ms, warn float64) string {
	switch {
	case ms >= warn*2:
		return ColDanger
	case ms >= warn:
		return ColWarn
	}
	return ColSuccess
}

// hitColor 命中率着色（低于 50% 红，低于 80% 黄）。
func hitColor(pct float64) string {
	switch {
	case pct < 50:
		return ColDanger
	case pct < 80:
		return ColWarn
	}
	return ColSuccess
}

// animColor 动画节点数着色（数量多说明有常驻重绘源）。
func animColor(n int) string {
	switch {
	case n >= 8:
		return ColWarn
	case n > 0:
		return ColPrimary
	}
	return ColTextDim
}

// devNow 采样起点（仅供 frame 内部使用，便于阅读）。
func devNow() time.Time { return time.Now() }
