// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// ShapeKind 刚体碰撞形状类型。
type ShapeKind uint8

const (
	// ShapeSphere 球体（半径 Radius）。
	ShapeSphere ShapeKind = iota
	// ShapeBox 盒子（三轴半边长 Half）。
	ShapeBox
	// ShapeCapsule 胶囊（半径 Radius + 圆柱段长度 Height，局部 +Y 为轴向）。
	ShapeCapsule
	// ShapePlane 无限平面（Half 存归一化法线，平面过刚体 Pos）。
	ShapePlane
)

// String 返回形状名称。
func (k ShapeKind) String() string {
	switch k {
	case ShapeSphere:
		return "sphere"
	case ShapeBox:
		return "box"
	case ShapeCapsule:
		return "capsule"
	case ShapePlane:
		return "plane"
	}
	return "unknown"
}

// Shape 简单凸体形状描述（刚体局部空间）。
type Shape struct {
	Kind   ShapeKind
	Radius float32
	Half   Vec3
	Height float32
}

// SphereShape 构造球体形状。
func SphereShape(r float32) Shape {
	if r < 0 {
		r = 0
	}
	return Shape{Kind: ShapeSphere, Radius: r}
}

// BoxShape 构造盒形状（hx/hy/hz 为三轴半边长）。
func BoxShape(hx, hy, hz float32) Shape {
	return Shape{Kind: ShapeBox, Half: V3(rigAbs(hx), rigAbs(hy), rigAbs(hz))}
}

// CapsuleShape 构造胶囊形状（r 半径，h 圆柱段长度，轴向为局部 +Y）。
func CapsuleShape(r, h float32) Shape {
	if r < 0 {
		r = 0
	}
	if h < 0 {
		h = 0
	}
	return Shape{Kind: ShapeCapsule, Radius: r, Height: h}
}

// PlaneShape 构造平面形状（n 为平面法线，构造时归一化）。
func PlaneShape(n Vec3) Shape {
	n = n.Normalize()
	if n.Len() == 0 {
		n = V3(0, 1, 0)
	}
	return Shape{Kind: ShapePlane, Half: n}
}

// LocalInertia 返回质量 mass 下体空间主轴惯性矩（平面返回零向量）。
func (s Shape) LocalInertia(mass float32) Vec3 {
	if mass <= 0 {
		return Vec3{}
	}
	switch s.Kind {
	case ShapeSphere:
		r := s.Radius
		i := 0.4 * mass * r * r
		return V3(i, i, i)
	case ShapeBox:
		hx, hy, hz := s.Half.X, s.Half.Y, s.Half.Z
		k := mass / 3
		return V3(k*(hy*hy+hz*hz), k*(hx*hx+hz*hz), k*(hx*hx+hy*hy))
	case ShapeCapsule:
		r, h := s.Radius, s.Height
		vc := math.Pi * r * r * h        // 圆柱段体积
		vs := 4.0 / 3.0 * math.Pi * r * r * r // 两端半球合成一个球
		tot := vc + vs
		if tot <= 0 {
			return Vec3{}
		}
		mc := mass * (vc / tot)
		ms := mass - mc
		iy := 0.5*mc*r*r + 0.4*ms*r*r
		ix := mc*(3*r*r+h*h)/12 + 0.4*ms*r*r + ms*(h*0.5)*(h*0.5)
		return V3(ix, iy, ix)
	}
	return Vec3{}
}

// boundingRadius 返回形状的包围球半径。
func (s Shape) boundingRadius() float32 {
	switch s.Kind {
	case ShapeSphere:
		return s.Radius
	case ShapeBox:
		return s.Half.Len()
	case ShapeCapsule:
		return s.Radius + s.Height*0.5
	}
	return 0
}

// RigidBody 刚体（位置 + 姿态 + 线速度 + 角速度 + 凸体形状）。
type RigidBody struct {
	Pos    Vec3
	Rot    Quat
	Vel    Vec3
	AngVel Vec3

	Mass, InvMass   float32
	InvInertiaLocal Vec3

	Shape Shape

	Restitution, Friction float32
	Static                bool

	LinearDamping, AngularDamping float32

	SleepTimer float32
	Awake      bool

	force   Vec3
	torque  Vec3
	idx     int
	world   *RigidWorld
	touching bool
}

// NewRigidBody 构造刚体（mass<=0 视为静态体）。
func NewRigidBody(pos Vec3, mass float32, sh Shape) *RigidBody {
	b := &RigidBody{
		Pos:            pos,
		Rot:            QuatIdentity(),
		Shape:          sh,
		Friction:       0.5,
		Restitution:    0,
		LinearDamping:  0.02,
		AngularDamping: 0.05,
		Awake:          true,
		idx:            -1,
	}
	b.SetMass(mass)
	return b
}

// SetMass 设置质量并同步惯量（m<=0 或平面形状 → 静态体）。
func (b *RigidBody) SetMass(m float32) {
	b.Mass = m
	if m <= 0 || b.Shape.Kind == ShapePlane {
		b.Mass = 0
		b.InvMass = 0
		b.InvInertiaLocal = Vec3{}
		b.Static = true
		return
	}
	b.InvMass = 1 / m
	b.Static = false
	ii := b.Shape.LocalInertia(m)
	b.InvInertiaLocal = V3(rigInv(ii.X), rigInv(ii.Y), rigInv(ii.Z))
}

// ApplyImpulse 在 worldPoint 处施加世界空间冲量。
func (b *RigidBody) ApplyImpulse(imp Vec3, worldPoint Vec3) {
	if b == nil || b.InvMass == 0 {
		return
	}
	b.Vel = b.Vel.Add(imp.Scale(b.InvMass))
	r := worldPoint.Sub(b.Pos)
	b.AngVel = b.AngVel.Add(rigInvInertiaMul(b, r.Cross(imp)))
	b.Wake()
}

// ApplyForce 累积一个世界空间力（Step 末尾清零，只在本子步生效）。
func (b *RigidBody) ApplyForce(f Vec3) {
	if b == nil || b.InvMass == 0 {
		return
	}
	b.force = b.force.Add(f)
	b.Wake()
}

// ApplyForceAt 在 worldPoint 处累积一个世界空间力（含力矩）。
func (b *RigidBody) ApplyForceAt(f Vec3, worldPoint Vec3) {
	if b == nil || b.InvMass == 0 {
		return
	}
	b.force = b.force.Add(f)
	r := worldPoint.Sub(b.Pos)
	b.torque = b.torque.Add(r.Cross(f))
	b.Wake()
}

// ApplyTorque 累积一个世界空间力矩，Step 末尾清零。
func (b *RigidBody) ApplyTorque(t Vec3) {
	if b == nil || b.InvMass == 0 {
		return
	}
	b.torque = b.torque.Add(t)
	b.Wake()
}

// ClearForces 清零累积的力与力矩。
func (b *RigidBody) ClearForces() {
	b.force = Vec3{}
	b.torque = Vec3{}
}

// WorldInertiaInv 返回世界空间惯性张量逆（R·I⁻¹·Rᵀ，3x3）。
func (b *RigidBody) WorldInertiaInv() [3][3]float32 {
	cols := [3]Vec3{
		b.Rot.Rotate(V3(1, 0, 0)),
		b.Rot.Rotate(V3(0, 1, 0)),
		b.Rot.Rotate(V3(0, 0, 1)),
	}
	inv := [3]float32{b.InvInertiaLocal.X, b.InvInertiaLocal.Y, b.InvInertiaLocal.Z}
	var m [3][3]float32
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			var s float32
			for k := 0; k < 3; k++ {
				s += rigComp(cols[k], i) * inv[k] * rigComp(cols[k], j)
			}
			m[i][j] = s
		}
	}
	return m
}

// AABB 返回刚体世界空间包围盒。
func (b *RigidBody) AABB() AABB {
	s := b.Shape
	switch s.Kind {
	case ShapeSphere:
		e := V3(s.Radius, s.Radius, s.Radius)
		return AABB{Min: b.Pos.Sub(e), Max: b.Pos.Add(e)}
	case ShapeBox:
		ax := b.Rot.Rotate(V3(1, 0, 0))
		ay := b.Rot.Rotate(V3(0, 1, 0))
		az := b.Rot.Rotate(V3(0, 0, 1))
		h := s.Half
		e := V3(
			rigAbs(ax.X)*h.X+rigAbs(ay.X)*h.Y+rigAbs(az.X)*h.Z,
			rigAbs(ax.Y)*h.X+rigAbs(ay.Y)*h.Y+rigAbs(az.Y)*h.Z,
			rigAbs(ax.Z)*h.X+rigAbs(ay.Z)*h.Y+rigAbs(az.Z)*h.Z)
		return AABB{Min: b.Pos.Sub(e), Max: b.Pos.Add(e)}
	case ShapeCapsule:
		ay := b.Rot.Rotate(V3(0, 1, 0))
		eh := s.Height * 0.5
		e := V3(
			s.Radius+rigAbs(ay.X)*eh,
			s.Radius+rigAbs(ay.Y)*eh,
			s.Radius+rigAbs(ay.Z)*eh)
		return AABB{Min: b.Pos.Sub(e), Max: b.Pos.Add(e)}
	case ShapePlane:
		big := float32(1e6)
		return AABB{Min: b.Pos.Sub(V3(big, big, big)), Max: b.Pos.Add(V3(big, big, big))}
	}
	return AABB{Min: b.Pos, Max: b.Pos}
}

// Wake 唤醒刚体。
func (b *RigidBody) Wake() {
	if b == nil {
		return
	}
	b.Awake = true
	b.SleepTimer = 0
}

// Sleep 让刚体休眠（速度清零）。
func (b *RigidBody) Sleep() {
	if b == nil || b.InvMass == 0 {
		return
	}
	b.Awake = false
	b.SleepTimer = 0
	b.Vel = Vec3{}
	b.AngVel = Vec3{}
}

// WorldPoint 把局部点变换到世界空间。
func (b *RigidBody) WorldPoint(local Vec3) Vec3 {
	return b.Pos.Add(b.Rot.Rotate(local))
}

// LocalPoint 把世界点变换到刚体局部空间。
func (b *RigidBody) LocalPoint(world Vec3) Vec3 {
	return rigQuatConj(b.Rot).Rotate(world.Sub(b.Pos))
}

// LocalDir 把世界方向旋转到刚体局部空间。
func (b *RigidBody) LocalDir(dir Vec3) Vec3 {
	return rigQuatConj(b.Rot).Rotate(dir)
}

// Contact 一个接触点（法线方向统一为「从 A 指向 B」）。
type Contact struct {
	A, B *RigidBody

	Point  Vec3
	Normal Vec3
	Depth  float32

	NormalImpulse  float32
	TangentImpulse float32

	ra, rb       Vec3
	t1, t2       Vec3
	mN, mT1, mT2 float32
	jT2          float32
	bias         float32
	feat         uint8
}

// ======================= 内部数学工具 =======================

// rigAbs 浮点绝对值。
func rigAbs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// rigInv 安全倒数（0 保持 0）。
func rigInv(v float32) float32 {
	if v == 0 {
		return 0
	}
	return 1 / v
}

// rigSign 符号（零返回 1）。
func rigSign(v float32) float32 {
	if v < 0 {
		return -1
	}
	return 1
}

// rigComp 取 Vec3 的第 i 个分量。
func rigComp(v Vec3, i int) float32 {
	switch i {
	case 0:
		return v.X
	case 1:
		return v.Y
	}
	return v.Z
}

// rigMin 浮点最小。
func rigMin(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// rigMax 浮点最大。
func rigMax(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// rigQuatConj 四元数共轭（逆旋转）。
func rigQuatConj(q Quat) Quat { return Quat{-q.X, -q.Y, -q.Z, q.W} }

// rigQuatIntegrate 用角速度 w 积分姿态并归一化。
func rigQuatIntegrate(q Quat, w Vec3, h float32) Quat {
	rw := -(w.X*q.X + w.Y*q.Y + w.Z*q.Z)
	rx := w.X*q.W + w.Y*q.Z - w.Z*q.Y
	ry := w.Y*q.W - w.X*q.Z + w.Z*q.X
	rz := w.Z*q.W + w.X*q.Y - w.Y*q.X
	nq := Quat{q.X + 0.5*h*rx, q.Y + 0.5*h*ry, q.Z + 0.5*h*rz, q.W + 0.5*h*rw}
	return nq.Normalize()
}

// rigInvInertiaMul 世界空间惯性张量逆乘向量（静态体返回零）。
func rigInvInertiaMul(b *RigidBody, v Vec3) Vec3 {
	if b == nil || b.InvMass == 0 {
		return Vec3{}
	}
	local := rigQuatConj(b.Rot).Rotate(v)
	local = local.Mul(b.InvInertiaLocal)
	return b.Rot.Rotate(local)
}

// rigPerp 求与 n 垂直的确定性单位向量。
func rigPerp(n Vec3) Vec3 {
	if rigAbs(n.X) >= 0.57735 {
		return V3(n.Y, -n.X, 0).Normalize()
	}
	return V3(0, n.Z, -n.Y).Normalize()
}

// rigMat3MulVec 3x3 矩阵乘向量。
func rigMat3MulVec(m [3][3]float32, v Vec3) Vec3 {
	return V3(
		m[0][0]*v.X+m[0][1]*v.Y+m[0][2]*v.Z,
		m[1][0]*v.X+m[1][1]*v.Y+m[1][2]*v.Z,
		m[2][0]*v.X+m[2][1]*v.Y+m[2][2]*v.Z)
}

// rigMat3Inv 3x3 矩阵求逆（奇异返回 false）。
func rigMat3Inv(m [3][3]float32) ([3][3]float32, bool) {
	a, b, c := m[0][0], m[0][1], m[0][2]
	d, e, f := m[1][0], m[1][1], m[1][2]
	g, h, i := m[2][0], m[2][1], m[2][2]
	c00 := e*i - f*h
	c01 := -(d*i - f*g)
	c02 := d*h - e*g
	det := a*c00 + b*c01 + c*c02
	if rigAbs(det) < 1e-12 {
		return [3][3]float32{}, false
	}
	s := 1 / det
	var r [3][3]float32
	r[0][0] = c00 * s
	r[0][1] = -(b*i - c*h) * s
	r[0][2] = (b*f - c*e) * s
	r[1][0] = c01 * s
	r[1][1] = (a*i - c*g) * s
	r[1][2] = -(a*f - c*d) * s
	r[2][0] = c02 * s
	r[2][1] = -(a*h - b*g) * s
	r[2][2] = (a*e - b*d) * s
	return r, true
}

// rigPointVel 刚体在世界点（相对质点 r）处的速度。
func rigPointVel(b *RigidBody, r Vec3) Vec3 {
	if b == nil || b.InvMass == 0 {
		return Vec3{}
	}
	return b.Vel.Add(b.AngVel.Cross(r))
}

// rigImpulseResponse 在 r 处施加冲量 p 引起的点速度变化。
func rigImpulseResponse(b *RigidBody, r Vec3, p Vec3) Vec3 {
	if b == nil || b.InvMass == 0 {
		return Vec3{}
	}
	lin := p.Scale(b.InvMass)
	ang := rigInvInertiaMul(b, r.Cross(p)).Cross(r)
	return lin.Add(ang)
}

// rigApplyImpulse 对 A 施加 -p、对 B 施加 +p（作用点 rA / rB）。
func rigApplyImpulse(a, b *RigidBody, ra, rb, p Vec3) {
	if a != nil && a.InvMass > 0 {
		a.Vel = a.Vel.Sub(p.Scale(a.InvMass))
		a.AngVel = a.AngVel.Sub(rigInvInertiaMul(a, ra.Cross(p)))
	}
	if b != nil && b.InvMass > 0 {
		b.Vel = b.Vel.Add(p.Scale(b.InvMass))
		b.AngVel = b.AngVel.Add(rigInvInertiaMul(b, rb.Cross(p)))
	}
}

// rigClosestOnSegment 线段 ab 上距离 p 最近的点。
func rigClosestOnSegment(a, b, p Vec3) Vec3 {
	ab := b.Sub(a)
	l2 := ab.Dot(ab)
	if l2 <= 1e-12 {
		return a
	}
	t := p.Sub(a).Dot(ab) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return a.Add(ab.Scale(t))
}

// rigSegSegClosest 两线段最近点对（Ericson）。
func rigSegSegClosest(p1, q1, p2, q2 Vec3) (Vec3, Vec3) {
	d1 := q1.Sub(p1)
	d2 := q2.Sub(p2)
	r := p1.Sub(p2)
	a := d1.Dot(d1)
	e := d2.Dot(d2)
	f := d2.Dot(r)
	const eps = 1e-9
	var s, t float32
	if a <= eps && e <= eps {
		return p1, p2
	}
	if a <= eps {
		s = 0
		t = clampF(f/e, 0, 1)
	} else {
		c := d1.Dot(r)
		if e <= eps {
			t = 0
			s = clampF(-c/a, 0, 1)
		} else {
			b := d1.Dot(d2)
			denom := a*e - b*b
			if denom != 0 {
				s = clampF((b*f-c*e)/denom, 0, 1)
			} else {
				s = 0
			}
			t = (b*s + f) / e
			if t < 0 {
				t = 0
				s = clampF(-c/a, 0, 1)
			} else if t > 1 {
				t = 1
				s = clampF((b-c)/a, 0, 1)
			}
		}
	}
	return p1.Add(d1.Scale(s)), p2.Add(d2.Scale(t))
}
