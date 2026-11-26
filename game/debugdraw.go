// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"image"
	"image/color"
	"math"
)

// dbgSeg 一条待投影的世界空间线段。
type dbgSeg struct {
	a, b Vec3
	c    RGBA
}

// DebugDraw 调试线框叠加层（实现 Sprite2D，在 3D 之后画 AABB/线/射线/球/坐标轴）。
//
// 用法：每帧 Clear() → 加入图元 → 作为 Renderer.SetSprite2D 的当前叠加层（或与游戏 HUD 组合）。
type DebugDraw struct {
	// Cam 投影相机（必填）。
	Cam *Camera
	// Color 默认颜色（图元未指定时使用）。
	Color RGBA

	// Lines 已提交的线段数（统计用）。
	Lines int
	// Pixels 上帧实际写入的像素数。
	Pixels int

	segs []dbgSeg
}

// NewDebugDraw 创建调试线框层。
func NewDebugDraw(cam *Camera) *DebugDraw {
	return &DebugDraw{Cam: cam, Color: RGB(0, 255, 0)}
}

// Clear 清空本帧图元（应在每帧开头调用）。
func (d *DebugDraw) Clear() {
	if d == nil {
		return
	}
	d.segs = d.segs[:0]
	d.Lines = 0
}

func (d *DebugDraw) color(c RGBA) RGBA {
	if c.A == 0 {
		return d.Color
	}
	return c
}

// Line 提交一条世界空间线段（DrawSprites 时统一投影）。
func (d *DebugDraw) Line(a, b Vec3, c RGBA) {
	if d == nil {
		return
	}
	d.segs = append(d.segs, dbgSeg{a: a, b: b, c: d.color(c)})
	d.Lines++
}

// Box 提交 AABB 的 12 条边。
func (d *DebugDraw) Box(min, max Vec3, c RGBA) {
	if d == nil {
		return
	}
	corners := [8]Vec3{
		{X: min.X, Y: min.Y, Z: min.Z}, {X: max.X, Y: min.Y, Z: min.Z},
		{X: max.X, Y: min.Y, Z: max.Z}, {X: min.X, Y: min.Y, Z: max.Z},
		{X: min.X, Y: max.Y, Z: min.Z}, {X: max.X, Y: max.Y, Z: min.Z},
		{X: max.X, Y: max.Y, Z: max.Z}, {X: min.X, Y: max.Y, Z: max.Z},
	}
	edges := [12][2]int{
		{0, 1}, {1, 2}, {2, 3}, {3, 0},
		{4, 5}, {5, 6}, {6, 7}, {7, 4},
		{0, 4}, {1, 5}, {2, 6}, {3, 7},
	}
	for _, e := range edges {
		d.Line(corners[e[0]], corners[e[1]], c)
	}
}

// Axes 提交坐标轴（X 红 / Y 绿 / Z 蓝）。
func (d *DebugDraw) Axes(pos Vec3, size float32) {
	d.Line(pos, pos.Add(V3(size, 0, 0)), RGB(255, 60, 60))
	d.Line(pos, pos.Add(V3(0, size, 0)), RGB(60, 255, 60))
	d.Line(pos, pos.Add(V3(0, 0, size)), RGB(60, 60, 255))
}

// Ray 提交一条射线（origin 起、dir 方向、长度 length）。
func (d *DebugDraw) Ray(origin, dir Vec3, length float32, c RGBA) {
	n := dir.Normalize()
	d.Line(origin, origin.Add(n.Scale(length)), c)
}

// Sphere 提交三个正交大圆近似球体。
func (d *DebugDraw) Sphere(center Vec3, r float32, c RGBA) {
	const segs = 16
	for i := 0; i < segs; i++ {
		a0 := float32(i) / segs * 2 * math.Pi
		a1 := float32(i+1) / segs * 2 * math.Pi
		// XY 平面
		d.Line(
			center.Add(V3(cos32(a0)*r, sin32(a0)*r, 0)),
			center.Add(V3(cos32(a1)*r, sin32(a1)*r, 0)), c)
		// XZ 平面
		d.Line(
			center.Add(V3(cos32(a0)*r, 0, sin32(a0)*r)),
			center.Add(V3(cos32(a1)*r, 0, sin32(a1)*r)), c)
		// YZ 平面
		d.Line(
			center.Add(V3(0, cos32(a0)*r, sin32(a0)*r)),
			center.Add(V3(0, cos32(a1)*r, sin32(a1)*r)), c)
	}
}

// DrawSprites 实现 Sprite2D：投影所有线段并直接写入 dst。
func (d *DebugDraw) DrawSprites(dst *image.RGBA, w, h int) {
	if d == nil || dst == nil || d.Cam == nil || w <= 0 || h <= 0 {
		return
	}
	if dst.Rect.Dx() != w || dst.Rect.Dy() != h {
		// 尺寸不一致时按实际目标尺寸绘制（越界像素会被 setPx 丢弃）
		w, h = dst.Rect.Dx(), dst.Rect.Dy()
		if w <= 0 || h <= 0 {
			return
		}
	}
	aspect := float32(w) / float32(h)
	vp := d.Cam.ViewProj(aspect)
	d.Pixels = 0
	for i := range d.segs {
		s := &d.segs[i]
		ax, ay, ok0 := projectPoint(vp, w, h, s.a)
		bx, by, ok1 := projectPoint(vp, w, h, s.b)
		if !ok0 || !ok1 {
			continue // 端点在相机后方：跳过（避免画到屏幕外）
		}
		d.Pixels += drawSeg(dst, ax, ay, bx, by, s.c)
	}
}

// projectPoint 世界坐标 → 屏幕像素；w<=0 表示在相机后方。
func projectPoint(vp Mat4, w, h int, p Vec3) (float32, float32, bool) {
	v := vp.TransformVec4(Vec4{X: p.X, Y: p.Y, Z: p.Z, W: 1})
	if v.W <= 0.0001 {
		return 0, 0, false
	}
	ndcX := v.X / v.W
	ndcY := v.Y / v.W
	if math.IsNaN(float64(ndcX)) || math.IsNaN(float64(ndcY)) {
		return 0, 0, false
	}
	x := (ndcX*0.5 + 0.5) * float32(w)
	// NDC 的 Y 向上，屏幕 Y 向下
	y := (0.5 - ndcY*0.5) * float32(h)
	return x, y, true
}

// drawSeg 栅格化一条 1px 线段，返回写入像素数。
func drawSeg(dst *image.RGBA, x0, y0, x1, y1 float32, c RGBA) int {
	dx := x1 - x0
	dy := y1 - y0
	steps := int(math.Abs(float64(dx)))
	if n := int(math.Abs(float64(dy))); n > steps {
		steps = n
	}
	if steps <= 0 {
		if setPx(dst, int(x0), int(y0), c) {
			return 1
		}
		return 0
	}
	n := 0
	for i := 0; i <= steps; i++ {
		t := float32(i) / float32(steps)
		if setPx(dst, int(x0+dx*t), int(y0+dy*t), c) {
			n++
		}
	}
	return n
}

// setPx 写一个像素（带 alpha 混合），越界返回 false。
func setPx(dst *image.RGBA, x, y int, c RGBA) bool {
	if dst == nil || x < 0 || y < 0 || x >= dst.Rect.Dx() || y >= dst.Rect.Dy() {
		return false
	}
	if c.A == 0 || c.A == 255 {
		dst.SetRGBA(x, y, color.RGBA{R: c.R, G: c.G, B: c.B, A: 255})
		return true
	}
	dst.SetRGBA(x, y, color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A})
	return true
}

func cos32(v float32) float32 { return float32(math.Cos(float64(v))) }

func sin32(v float32) float32 { return float32(math.Sin(float64(v))) }
