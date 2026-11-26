// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// Camera 相机（默认看向 -Z 的右手系）。
type Camera struct {
	Pos, Target, Up Vec3
	FOVY            float32
	Near, Far       float32
	Ortho           bool
	OrthoSize       float32
}

// NewCamera 构造默认相机（位于 (0,0,5)、看向原点）。
func NewCamera() *Camera {
	return &Camera{
		Pos: V3(0, 0, 5), Target: V3(0, 0, 0), Up: V3(0, 1, 0),
		FOVY: 60, Near: 0.1, Far: 1000, OrthoSize: 5,
	}
}

// View 视图矩阵。
func (c *Camera) View() Mat4 { return LookAt(c.Pos, c.Target, c.Up) }

// Proj 投影矩阵（aspect = 宽/高）。
func (c *Camera) Proj(aspect float32) Mat4 {
	if c.Ortho {
		sz := c.OrthoSize
		if sz <= 0 {
			sz = 5
		}
		if aspect <= 0 {
			aspect = 1
		}
		return Ortho(-sz*aspect, sz*aspect, -sz, sz, c.Near, c.Far)
	}
	return Perspective(Radians(c.FOVY), aspect, c.Near, c.Far)
}

// ViewProj 组合矩阵。
func (c *Camera) ViewProj(aspect float32) Mat4 { return c.Proj(aspect).Mul(c.View()) }

// Forward 相机前方向（单位向量）。
func (c *Camera) Forward() Vec3 { return c.Target.Sub(c.Pos).Normalize() }

// Ray 从屏幕像素发出一根射线（用于射线拾取），返回起点与单位方向。
func (c *Camera) Ray(px, py float32, w, h int) (Vec3, Vec3) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	ndcX := px/float32(w)*2 - 1
	ndcY := 1 - py/float32(h)*2
	if c.Ortho {
		sz := c.OrthoSize
		if sz <= 0 {
			sz = 5
		}
		aspect := float32(w) / float32(h)
		// 正交：起点在近平面上，方向即视线方向
		right := c.Forward().Cross(c.Up).Normalize()
		up := right.Cross(c.Forward()).Normalize()
		origin := c.Pos.Add(right.Scale(ndcX * sz * aspect)).Add(up.Scale(ndcY * sz))
		return origin, c.Forward()
	}
	tanHalf := float32(math.Tan(float64(Radians(c.FOVY)) / 2))
	aspect := float32(w) / float32(h)
	fwd := c.Forward()
	right := fwd.Cross(c.Up).Normalize()
	up := right.Cross(fwd).Normalize()
	dir := fwd.Add(right.Scale(ndcX * tanHalf * aspect)).Add(up.Scale(ndcY * tanHalf)).Normalize()
	return c.Pos, dir
}

// Plane 平面（N·x + D = 0，内侧为正）。
type Plane struct {
	N Vec3
	D float32
}

// Frustum 视锥（6 个平面，内侧为正）。
type Frustum [6]Plane

// Frustum 从相机提取视锥体（世界空间，内侧为正）。
func (c *Camera) Frustum(aspect float32) Frustum {
	m := c.ViewProj(aspect)
	// 平面 = k0*row0 + k1*row1 + k2*row2 + k3*row3，行主序取列分量。
	mk := func(k0, k1, k2, k3 float32) Plane {
		p := Plane{
			N: V3(
				m[0]*k0+m[4]*k1+m[8]*k2+m[12]*k3,
				m[1]*k0+m[5]*k1+m[9]*k2+m[13]*k3,
				m[2]*k0+m[6]*k1+m[10]*k2+m[14]*k3,
			),
			D: m[3]*k0 + m[7]*k1 + m[11]*k2 + m[15]*k3,
		}
		l := p.N.Len()
		if l != 0 {
			p.N = p.N.Scale(1 / l)
			p.D /= l
		}
		return p
	}
	return Frustum{
		mk(1, 0, 0, 1),  // left:   x + w >= 0
		mk(-1, 0, 0, 1), // right:  w - x >= 0
		mk(0, 1, 0, 1),  // bottom: y + w >= 0
		mk(0, -1, 0, 1), // top:    w - y >= 0
		mk(0, 0, 1, 1),  // near:   z + w >= 0
		mk(0, 0, -1, 1), // far:    w - z >= 0
	}
}

// ContainsAABB 判断包围盒是否与视锥相交（保守测试，可接受假阴性为零）。
func (f Frustum) ContainsAABB(b AABB) bool {
	for i := range f {
		p := f[i]
		// 取沿法线最远的顶点
		v := Vec3{
			X: pick(b.Min.X, b.Max.X, p.N.X),
			Y: pick(b.Min.Y, b.Max.Y, p.N.Y),
			Z: pick(b.Min.Z, b.Max.Z, p.N.Z),
		}
		if p.N.Dot(v)+p.D < 0 {
			return false
		}
	}
	return true
}

func pick(lo, hi, n float32) float32 {
	if n >= 0 {
		return hi
	}
	return lo
}

// pickFlag 取角点分量：flag 为真取 hi，否则取 lo（用于包围盒 8 角遍历）。
func pickFlag(lo, hi float32, flag bool) float32 {
	if flag {
		return hi
	}
	return lo
}

// Orbit 轨道相机控制器（鼠标拖拽旋转 + 滚轮缩放）。
type Orbit struct {
	Camera   *Camera
	Target   Vec3
	Distance float32
	Yaw      float32
	Pitch    float32
	MinDist  float32
	MaxDist  float32
}

// NewOrbit 构造围绕 target 的轨道相机。
func NewOrbit(c *Camera, target Vec3, distance float32) *Orbit {
	o := &Orbit{Camera: c, Target: target, Distance: distance, MinDist: 0.5, MaxDist: 200}
	o.Apply()
	return o
}

// HandleDrag 处理鼠标拖拽（dx/dy 为像素位移）与滚轮。
func (o *Orbit) HandleDrag(dx, dy, wheel float32) {
	o.Yaw += dx * 0.01
	o.Pitch = Clamp(o.Pitch+dy*0.01, -1.5, 1.5)
	if wheel != 0 {
		lo, hi := o.MinDist, o.MaxDist
		if lo <= 0 {
			lo = 0.5
		}
		if hi <= 0 {
			hi = 200
		}
		o.Distance = Clamp(o.Distance*(1-wheel*0.1), lo, hi)
	}
	o.Apply()
}

// Apply 按当前角度把相机位置写回 Camera。
func (o *Orbit) Apply() {
	cp, sp := float32(math.Cos(float64(o.Pitch))), float32(math.Sin(float64(o.Pitch)))
	cy, sy := float32(math.Cos(float64(o.Yaw))), float32(math.Sin(float64(o.Yaw)))
	d := o.Distance
	o.Camera.Pos = o.Target.Add(V3(d*cp*sy, d*sp, d*cp*cy))
	o.Camera.Target = o.Target
	if o.Camera.Up.Len() == 0 {
		o.Camera.Up = V3(0, 1, 0)
	}
}

// Fly 第一人称飞行控制器。
type Fly struct {
	Camera      *Camera
	Speed       float32 // 单位/秒
	Sensitivity float32 // 弧度/像素
	Yaw, Pitch  float32
}

// NewFly 构造飞行控制器（由相机当前朝向初始化角度）。
func NewFly(c *Camera) *Fly {
	f := &Fly{Camera: c, Speed: 6, Sensitivity: 0.003}
	d := c.Forward()
	f.Yaw = float32(math.Atan2(float64(d.X), float64(-d.Z)))
	f.Pitch = float32(math.Asin(float64(Clamp(d.Y, -1, 1))))
	return f
}

// Look 处理鼠标位移（改变朝向）。
func (f *Fly) Look(dx, dy float32) {
	f.Yaw -= dx * f.Sensitivity
	f.Pitch = Clamp(f.Pitch-dy*f.Sensitivity, -1.5, 1.5)
	cp := float32(math.Cos(float64(f.Pitch)))
	f.Camera.Target = f.Camera.Pos.Add(V3(
		cp*float32(math.Sin(float64(f.Yaw))),
		float32(math.Sin(float64(f.Pitch))),
		-cp*float32(math.Cos(float64(f.Yaw))),
	))
}

// Move 按前后/左右/上下轴移动相机（保持朝向不变）。
func (f *Fly) Move(forward, right, up, dt float32) {
	dir := f.Camera.Forward()
	r := dir.Cross(f.Camera.Up).Normalize()
	delta := dir.Scale(forward).Add(r.Scale(right)).Add(V3(0, up, 0)).Scale(f.Speed * dt)
	f.Camera.Pos = f.Camera.Pos.Add(delta)
	f.Camera.Target = f.Camera.Target.Add(delta)
}
