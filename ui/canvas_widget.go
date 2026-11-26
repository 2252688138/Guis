// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// Canvas 自绘节点：用一组声明式形状（矩形 / 圆 / 线 / 多边形 / 圆弧…）在 CPU 上光栅成纹理，
// 再走与 KViewport3D / KChart 相同的纹理合成链路绘制。
//
//	ui.Canvas(ui.NewCanvasState("board"), []ui.CanvasShape{
//	    {Kind: "rect", X: 10, Y: 10, W: 80, H: 40, Fill: "#4c8dff"},
//	    {Kind: "line", X: 0, Y: 60, W: 200, H: 60, Stroke: "#ffffff88", StrokeW: 2},
//	}, ui.CanvasOptions{W: 220, H: 90})
//
// 只在形状指纹变化时重新光栅，静态画面每帧零 CPU 绘制。

import (
	"fmt"
	"hash/fnv"
	"math"

	"guis/internal/gpu"
)

// CanvasPoint 多边形 / 折线的顶点。
type CanvasPoint struct {
	X, Y float32
}

// CanvasShape 一条绘制指令。字段含义按 Kind 取用：
//
//	rect / roundrect   X,Y,W,H（roundrect 用 R 作圆角）
//	line / polyline    X,Y 起点 + W,H 终点（polyline 用 Points）
//	rect / polygon     Fill 填充，Stroke 描边，StrokeW 线宽
//	circle / ring      CX,CY,Radius
//	arc                CX,CY,Radius，A0/A1 为起止角度（度，0 = 正右，顺时针）
type CanvasShape struct {
	Kind    string
	X, Y    float32
	W, H    float32
	R       float32
	Points  []CanvasPoint
	CX, CY  float32
	Radius  float32
	A0, A1  float32
	Fill    string
	Stroke  string
	StrokeW float32
	Opacity float32
}

// CanvasOptions 画布参数（W/H 为逻辑像素）。
type CanvasOptions struct {
	W, H float32
	BG   string
}

// CanvasState 画布状态（由调用方长期持有，按 key 缓存）。
type CanvasState struct {
	key    string
	w, h   int
	shapes []CanvasShape
	bg     string

	tex  *gpu.Texture
	tw   int
	th   int
	pix  []byte
	fp   uint64
	hasFP bool
	rasters int // 光栅次数（测试 / 诊断用）
}

// NewCanvasState 创建画布状态。
func NewCanvasState(key string) *CanvasState {
	return &CanvasState{key: key, w: 320, h: 180}
}

// Key 画布 key。
func (s *CanvasState) Key() string { return s.key }

// Invalidate 强制下次绘制重新光栅。
func (s *CanvasState) Invalidate() { s.hasFP = false }

// Rasters 光栅次数（诊断 / 测试用）。
func (s *CanvasState) Rasters() int { return s.rasters }

// Pixels 最近一次光栅结果（直通道 RGBA，长度 w*h*4；未光栅时为 nil）。
func (s *CanvasState) Pixels() []byte { return s.pix }

// Size 画布逻辑尺寸。
func (s *CanvasState) Size() (w, h int) { return s.w, s.h }

// setSize 设置逻辑尺寸（变化时触发重光栅）。
func (s *CanvasState) setSize(w, h int) {
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	if s.w == w && s.h == h {
		return
	}
	s.w, s.h = w, h
	s.hasFP = false
}

// needRaster 形状 / 尺寸 / 底色是否变化（变化则返回 true 并记下新指纹）。
func (s *CanvasState) needRaster() bool {
	fp := s.fingerprint()
	if s.hasFP && fp == s.fp && s.pix != nil {
		return false
	}
	s.fp, s.hasFP = fp, true
	return true
}

// fingerprint 形状集合的指纹。
func (s *CanvasState) fingerprint() uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d|%d|%s|", s.w, s.h, s.bg)
	for _, sh := range s.shapes {
		fmt.Fprintf(h, "%s;%g,%g,%g,%g;%g;%g,%g,%g;%g,%g;%s;%s;%g;%g|",
			sh.Kind, sh.X, sh.Y, sh.W, sh.H, sh.R, sh.CX, sh.CY, sh.Radius,
			sh.A0, sh.A1, sh.Fill, sh.Stroke, sh.StrokeW, sh.Opacity)
		for _, p := range sh.Points {
			fmt.Fprintf(h, "%g,%g;", p.X, p.Y)
		}
	}
	return h.Sum64()
}

// Canvas 构造 Canvas 节点。
func Canvas(st *CanvasState, shapes []CanvasShape, opt CanvasOptions) *Node {
	if st == nil {
		st = NewCanvasState("")
	}
	if opt.W <= 0 {
		opt.W = 320
	}
	if opt.H <= 0 {
		opt.H = 180
	}
	st.setSize(int(opt.W), int(opt.H))
	st.shapes = append(st.shapes[:0], shapes...)
	st.bg = opt.BG
	n := &Node{Kind: KCanvas, Key: st.key, Canvas: st, Style: Style{Opacity: 1}}
	n.Style.Width = opt.W
	n.Style.Height = opt.H
	return n
}

// Raster 按需光栅并返回像素（形状未变时直接复用上次结果）。
func (s *CanvasState) Raster() []byte {
	if s.needRaster() {
		s.pix = s.rasterize()
	}
	return s.pix
}

// rasterize 把形状画到画布上（逻辑坐标，内部按 chartSS 超采样）。
func (s *CanvasState) rasterize() []byte {
	cv := newCanvas(s.w, s.h)
	if cv == nil {
		return nil
	}
	if bg := parseColor(s.bg); bg.A > 0 {
		cv.fillPoly([]fpt{
			{0, 0}, {float32(s.w), 0}, {float32(s.w), float32(s.h)}, {0, float32(s.h)},
		}, bg)
	}
	for _, sh := range s.shapes {
		drawCanvasShape(cv, sh)
	}
	s.rasters++
	return cv.result()
}

// paintOfShape 形状的填充 / 描边画刷（opacity 乘进 alpha）。
func paintOfShape(color string, opacity float32) gpu.Paint {
	p := parseColor(color)
	if p.A <= 0 {
		return p
	}
	if opacity > 0 && opacity < 1 {
		p.A *= opacity
	}
	return p
}

// drawCanvasShape 画一条形状（未知 Kind 直接忽略）。
func drawCanvasShape(cv *canvas, sh CanvasShape) {
	if cv == nil {
		return
	}
	fill := paintOfShape(sh.Fill, sh.Opacity)
	stroke := paintOfShape(sh.Stroke, sh.Opacity)
	sw := sh.StrokeW
	if sw <= 0 {
		sw = 1
	}
	switch sh.Kind {
	case "rect":
		pts := rectPoints(sh.X, sh.Y, sh.W, sh.H)
		if fill.A > 0 {
			cv.fillPoly(pts, fill)
		}
		if stroke.A > 0 {
			cv.strokePoly(pts, sw, stroke, true)
		}
	case "roundrect":
		pts := roundRectPoints(sh.X, sh.Y, sh.W, sh.H, sh.R)
		if fill.A > 0 {
			cv.fillPoly(pts, fill)
		}
		if stroke.A > 0 {
			cv.strokePoly(pts, sw, stroke, true)
		}
	case "line":
		if stroke.A > 0 {
			cv.line(fpt{sh.X, sh.Y}, fpt{sh.W, sh.H}, sw/2, stroke)
		}
	case "polyline":
		pts := toFpts(sh.Points)
		if len(pts) >= 2 && stroke.A > 0 {
			cv.strokePoly(pts, sw, stroke, false)
		}
	case "polygon":
		pts := toFpts(sh.Points)
		if len(pts) >= 3 {
			if fill.A > 0 {
				cv.fillPoly(pts, fill)
			}
			if stroke.A > 0 {
				cv.strokePoly(pts, sw, stroke, true)
			}
		}
	case "circle":
		if fill.A > 0 {
			cv.disc(sh.CX, sh.CY, sh.Radius, fill)
		}
		if stroke.A > 0 {
			cv.strokePoly(circlePoints(sh.CX, sh.CY, sh.Radius), sw, stroke, true)
		}
	case "ring":
		if stroke.A > 0 {
			cv.strokePoly(circlePoints(sh.CX, sh.CY, sh.Radius), sw, stroke, true)
		}
	case "arc":
		pts := arcPoints(sh.CX, sh.CY, sh.Radius, sh.A0, sh.A1)
		if len(pts) >= 2 && stroke.A > 0 {
			cv.strokePoly(pts, sw, stroke, false)
		}
	}
}

// rectPoints 矩形的四个角。
func rectPoints(x, y, w, h float32) []fpt {
	return []fpt{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
}

// roundRectPoints 圆角矩形的边界点（每角 6 段折线近似）。
func roundRectPoints(x, y, w, h, r float32) []fpt {
	if r <= 0 {
		return rectPoints(x, y, w, h)
	}
	if max := minf(w, h) / 2; r > max {
		r = max
	}
	const seg = 6
	pts := make([]fpt, 0, 4*seg)
	corners := []struct{ cx, cy, a0 float32 }{
		{x + w - r, y + r, -90},
		{x + w - r, y + h - r, 0},
		{x + r, y + h - r, 90},
		{x + r, y + r, 180},
	}
	for _, c := range corners {
		for i := 0; i <= seg; i++ {
			a := (c.a0 + 90*float32(i)/seg) * float32(math.Pi) / 180
			pts = append(pts, fpt{c.cx + r*float32(math.Cos(float64(a))), c.cy + r*float32(math.Sin(float64(a)))})
		}
	}
	return pts
}

// circlePoints 圆周折线（48 段）。
func circlePoints(cx, cy, r float32) []fpt {
	return arcPoints(cx, cy, r, 0, 360)
}

// arcPoints 圆弧折线（角度按度、顺时针，0 = 正右）。
func arcPoints(cx, cy, r, a0, a1 float32) []fpt {
	if r <= 0 {
		return nil
	}
	if a1 < a0 {
		a0, a1 = a1, a0
	}
	span := a1 - a0
	if span <= 0 {
		return nil
	}
	seg := int(span/7.5) + 2
	if seg > 180 {
		seg = 180
	}
	pts := make([]fpt, 0, seg)
	for i := 0; i <= seg; i++ {
		a := (a0 + span*float32(i)/float32(seg)) * float32(math.Pi) / 180
		pts = append(pts, fpt{cx + r*float32(math.Cos(float64(a))), cy + r*float32(math.Sin(float64(a)))})
	}
	return pts
}

// toFpts 顶点数组转换。
func toFpts(pts []CanvasPoint) []fpt {
	if len(pts) == 0 {
		return nil
	}
	out := make([]fpt, len(pts))
	for i, p := range pts {
		out[i] = fpt{p.X, p.Y}
	}
	return out
}
