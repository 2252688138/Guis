// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// Overlaps 两个包围盒是否相交（边界相接算相交）。
func (b AABB) Overlaps(o AABB) bool {
	return b.Min.X <= o.Max.X && b.Max.X >= o.Min.X &&
		b.Min.Y <= o.Max.Y && b.Max.Y >= o.Min.Y &&
		b.Min.Z <= o.Max.Z && b.Max.Z >= o.Min.Z
}

// Union 合并两个包围盒。
func (b AABB) Union(o AABB) AABB {
	return AABB{Min: b.Min.Min(o.Min), Max: b.Max.Max(o.Max)}
}

// Contains 点是否在盒内。
func (b AABB) Contains(p Vec3) bool {
	return p.X >= b.Min.X && p.X <= b.Max.X &&
		p.Y >= b.Min.Y && p.Y <= b.Max.Y &&
		p.Z >= b.Min.Z && p.Z <= b.Max.Z
}

// Translated 平移包围盒。
func (b AABB) Translated(v Vec3) AABB {
	return AABB{Min: b.Min.Add(v), Max: b.Max.Add(v)}
}

// Expanded 各方向向外扩展 r。
func (b AABB) Expanded(r float32) AABB {
	d := V3(r, r, r)
	return AABB{Min: b.Min.Sub(d), Max: b.Max.Add(d)}
}

// AABBFromMesh 取网格包围盒（未计算时现场计算）。
func AABBFromMesh(m *Mesh) AABB {
	if m == nil {
		return AABB{}
	}
	if len(m.Positions) == 0 {
		return AABB{}
	}
	if m.Bounds.Min == m.Bounds.Max && m.Bounds.Min == (Vec3{}) && len(m.Positions) > 1 {
		m.RecomputeBounds()
	}
	return m.Bounds
}

// Sphere 球体。
type Sphere struct {
	C Vec3
	R float32
}

// RayAABB 射线与 AABB 相交（返回最近正向交点参数 t）。
func RayAABB(origin, dir Vec3, b AABB) (float32, bool) {
	tmin := float32(math.Inf(-1))
	tmax := float32(math.Inf(1))
	for axis := 0; axis < 3; axis++ {
		var o, d, lo, hi float32
		switch axis {
		case 0:
			o, d, lo, hi = origin.X, dir.X, b.Min.X, b.Max.X
		case 1:
			o, d, lo, hi = origin.Y, dir.Y, b.Min.Y, b.Max.Y
		default:
			o, d, lo, hi = origin.Z, dir.Z, b.Min.Z, b.Max.Z
		}
		if math.Abs(float64(d)) < 1e-8 {
			if o < lo || o > hi {
				return 0, false
			}
			continue
		}
		t1 := (lo - o) / d
		t2 := (hi - o) / d
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
		if tmin > tmax {
			return 0, false
		}
	}
	if tmax < 0 {
		return 0, false
	}
	if tmin < 0 {
		return tmax, true
	}
	return tmin, true
}

// RaySphere 射线与球相交。
func RaySphere(origin, dir Vec3, s Sphere) (float32, bool) {
	oc := origin.Sub(s.C)
	b := oc.Dot(dir)
	c := oc.Dot(oc) - s.R*s.R
	disc := b*b - c
	if disc < 0 {
		return 0, false
	}
	sq := float32(math.Sqrt(float64(disc)))
	t1 := -b - sq
	t2 := -b + sq
	if t1 > 1e-5 {
		return t1, true
	}
	if t2 > 1e-5 {
		return t2, true
	}
	return 0, false
}

// RayTriangle 射线与三角形相交（Möller–Trumbore，双面）。
func RayTriangle(origin, dir, a, b, c Vec3) (float32, bool) {
	e1 := b.Sub(a)
	e2 := c.Sub(a)
	p := dir.Cross(e2)
	det := e1.Dot(p)
	if math.Abs(float64(det)) < 1e-8 {
		return 0, false
	}
	inv := 1 / det
	tv := origin.Sub(a)
	u := tv.Dot(p) * inv
	if u < 0 || u > 1 {
		return 0, false
	}
	q := tv.Cross(e1)
	v := dir.Dot(q) * inv
	if v < 0 || u+v > 1 {
		return 0, false
	}
	t := e2.Dot(q) * inv
	if t <= 1e-5 {
		return 0, false
	}
	return t, true
}

// RayMesh 射线与网格相交（射线先变换到模型空间，返回世界空间距离 t）。
func RayMesh(origin, dir Vec3, m *Mesh, world Mat4) (float32, int, bool) {
	if m == nil || len(m.Indices) == 0 || len(m.Positions) == 0 {
		return 0, 0, false
	}
	inv, ok := world.Invert()
	if !ok {
		return 0, 0, false
	}
	o := inv.TransformPoint(origin)
	d := inv.TransformDir(dir)
	scale := d.Len()
	if scale == 0 {
		return 0, 0, false
	}
	d = d.Scale(1 / scale)

	// 先用本地包围盒粗筛
	if _, hit := RayAABB(o, d, AABBFromMesh(m)); !hit {
		return 0, 0, false
	}

	best := float32(math.Inf(1))
	bestTri := -1
	for i := 0; i+2 < len(m.Indices); i += 3 {
		ia, ib, ic := int(m.Indices[i]), int(m.Indices[i+1]), int(m.Indices[i+2])
		if ia >= len(m.Positions) || ib >= len(m.Positions) || ic >= len(m.Positions) {
			continue
		}
		if t, hit := RayTriangle(o, d, m.Positions[ia], m.Positions[ib], m.Positions[ic]); hit && t < best {
			best = t
			bestTri = i / 3
		}
	}
	if bestTri < 0 {
		return 0, 0, false
	}
	return best / scale, bestTri, true
}

// SphereAABB 球与包围盒相交时的最小推出向量（分离则 hit=false）。
func SphereAABB(s Sphere, b AABB) (Vec3, bool) {
	closest := V3(
		Clamp(s.C.X, b.Min.X, b.Max.X),
		Clamp(s.C.Y, b.Min.Y, b.Max.Y),
		Clamp(s.C.Z, b.Min.Z, b.Max.Z),
	)
	d := s.C.Sub(closest)
	d2 := d.Dot(d)
	if d2 > s.R*s.R {
		return Vec3{}, false
	}
	if d2 > 1e-8 {
		dl := float32(math.Sqrt(float64(d2)))
		return d.Scale((s.R - dl) / dl), true
	}
	// 球心在盒内：沿最浅轴推出
	cent := b.Center()
	off := s.C.Sub(cent)
	size := b.Size().Scale(0.5)
	px := size.X - absf(off.X)
	py := size.Y - absf(off.Y)
	pz := size.Z - absf(off.Z)
	switch {
	case px <= py && px <= pz:
		return V3(copysignf(size.X+off.X, off.X), 0, 0), true
	case py <= pz:
		return V3(0, copysignf(size.Y+off.Y, off.Y), 0), true
	default:
		return V3(0, 0, copysignf(size.Z+off.Z, off.Z)), true
	}
}

func copysignf(v, sign float32) float32 {
	return float32(math.Copysign(float64(v), float64(sign)))
}

// OBB 有向包围盒（中心 + 半边长 + 三根正交轴）。
type OBB struct {
	Center Vec3
	Half   Vec3
	Axes   [3]Vec3
}

// NewOBB 用中心、半边长与旋转矩阵（三列即轴向）构造 OBB。
func NewOBB(center, half Vec3, axes [3]Vec3) OBB {
	return OBB{Center: center, Half: half, Axes: axes}
}

// AABBOBB 把轴对齐盒当作单位轴向的 OBB。
func AABBOBB(b AABB) OBB {
	c := b.Center()
	h := b.Max.Sub(c)
	return OBB{Center: c, Half: h, Axes: [3]Vec3{{X: 1}, {Y: 1}, {Z: 1}}}
}

// toLocal 把世界坐标转到 OBB 局部空间（轴为基）。
func (o OBB) toLocal(p Vec3) Vec3 {
	d := p.Sub(o.Center)
	return Vec3{X: d.Dot(o.Axes[0]), Y: d.Dot(o.Axes[1]), Z: d.Dot(o.Axes[2])}
}

// Contains 点是否在盒内。
func (o OBB) Contains(p Vec3) bool {
	l := o.toLocal(p)
	return float32(math.Abs(float64(l.X))) <= o.Half.X &&
		float32(math.Abs(float64(l.Y))) <= o.Half.Y &&
		float32(math.Abs(float64(l.Z))) <= o.Half.Z
}

// ClosestPoint 盒外表面上离 p 最近的点（盒内返回 p）。
func (o OBB) ClosestPoint(p Vec3) Vec3 {
	l := o.toLocal(p)
	cl := Vec3{
		X: clampF(l.X, -o.Half.X, o.Half.X),
		Y: clampF(l.Y, -o.Half.Y, o.Half.Y),
		Z: clampF(l.Z, -o.Half.Z, o.Half.Z),
	}
	return o.Center.Add(o.Axes[0].Scale(cl.X)).Add(o.Axes[1].Scale(cl.Y)).Add(o.Axes[2].Scale(cl.Z))
}

// Ray 射线与 OBB 求交（局部空间 slab 法）。
func (o OBB) Ray(origin, dir Vec3) (float32, bool) {
	o0 := o.toLocal(origin)
	d := Vec3{X: dir.Dot(o.Axes[0]), Y: dir.Dot(o.Axes[1]), Z: dir.Dot(o.Axes[2])}
	tmin := float32(-math.MaxFloat32)
	tmax := float32(math.MaxFloat32)
	h := [3]float32{o.Half.X, o.Half.Y, o.Half.Z}
	p := [3]float32{o0.X, o0.Y, o0.Z}
	dd := [3]float32{d.X, d.Y, d.Z}
	for i := 0; i < 3; i++ {
		if float32(math.Abs(float64(dd[i]))) < 1e-8 {
			if float32(math.Abs(float64(p[i]))) > h[i] {
				return 0, false
			}
			continue
		}
		inv := 1 / dd[i]
		t1 := (-h[i] - p[i]) * inv
		t2 := (h[i] - p[i]) * inv
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
		if tmin > tmax {
			return 0, false
		}
	}
	if tmax < 0 {
		return 0, false
	}
	if tmin < 0 {
		return 0, true // 发射点在盒内
	}
	return tmin, true
}

// Capsule 胶囊体（线段 + 半径）。
type Capsule struct {
	A, B   Vec3
	Radius float32
}

// ClosestPoint 胶囊体上离 p 最近的点。
func (c Capsule) ClosestPoint(p Vec3) Vec3 {
	seg := c.B.Sub(c.A)
	l2 := seg.Dot(seg)
	if l2 <= 1e-9 {
		return c.A
	}
	t := clampF(p.Sub(c.A).Dot(seg)/l2, 0, 1)
	return c.A.Add(seg.Scale(t))
}

// Contains 点是否在胶囊体内。
func (c Capsule) Contains(p Vec3) bool {
	return c.ClosestPoint(p).Dist(p) <= c.Radius
}

// Ray 射线与胶囊体求交（圆柱无限解 + 端点球），返回最近正距离。
func (c Capsule) Ray(origin, dir Vec3) (float32, bool) {
	best := float32(-1)
	seg := c.B.Sub(c.A)
	l2 := seg.Dot(seg)
	if l2 > 1e-9 {
		// 解构建到「射线 vs 无限圆柱」，再校验轴向范围
		d := dir.Normalize()
		m := origin.Sub(c.A)
		dPerp := d.Sub(seg.Scale(d.Dot(seg) / l2))
		mPerp := m.Sub(seg.Scale(m.Dot(seg) / l2))
		a := dPerp.Dot(dPerp)
		if a > 1e-9 {
			b := 2 * mPerp.Dot(dPerp)
			cc := mPerp.Dot(mPerp) - c.Radius*c.Radius
			disc := b*b - 4*a*cc
			if disc >= 0 {
				sqrtD := float32(math.Sqrt(float64(disc)))
				for _, t := range [2]float32{(-b - sqrtD) / (2 * a), (-b + sqrtD) / (2 * a)} {
					if t < 0 {
						continue
					}
					hit := origin.Add(d.Scale(t))
					proj := hit.Sub(c.A).Dot(seg) / l2
					if proj >= 0 && proj <= 1 && (best < 0 || t < best) {
						best = t
					}
				}
			}
		}
	}
	// 两端球帽
	for _, ctr := range [2]Vec3{c.A, c.B} {
		if t, ok := raySphere(origin, dir, ctr, c.Radius); ok && (best < 0 || t < best) {
			best = t
		}
	}
	if best < 0 {
		return 0, false
	}
	return best, true
}

// raySphere 射线与球求交（返回最近正距离）。
func raySphere(origin, dir Vec3, c Vec3, r float32) (float32, bool) {
	d := dir.Normalize()
	m := origin.Sub(c)
	b := m.Dot(d)
	cc := m.Dot(m) - r*r
	disc := b*b - cc
	if disc < 0 {
		return 0, false
	}
	sqrtD := float32(math.Sqrt(float64(disc)))
	t1 := -b - sqrtD
	if t1 >= 0 {
		return t1, true
	}
	t2 := -b + sqrtD
	if t2 >= 0 {
		return t2, true
	}
	return 0, false
}

// StepMove 尝试把刚体水平移动 delta，自动翻越不高于 stepHeight 的台阶。
//
// ground 返回指定水平位置的支撑面高度；返回是否真正移动。
func (b *Body) StepMove(delta Vec3, ground func(x, z float32) float32, stepHeight float32) bool {
	if b == nil || ground == nil {
		return false
	}
	if stepHeight <= 0 {
		stepHeight = 0.35
	}
	from := ground(b.Pos.X, b.Pos.Z)
	to := ground(b.Pos.X+delta.X, b.Pos.Z+delta.Z)
	if to-from > stepHeight {
		return false // 台阶过高，挡住
	}
	b.Pos = b.Pos.Add(delta)
	if b.Pos.Y < to {
		b.Pos.Y = to
		if b.Vel.Y < 0 {
			b.Vel.Y = 0
		}
		b.Grounded = true
	}
	return true
}

// clampF 数值裁剪。
func clampF(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
