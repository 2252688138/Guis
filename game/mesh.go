// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// AABB 轴对齐包围盒。
type AABB struct {
	Min, Max Vec3
}

// Center 中心点。
func (b AABB) Center() Vec3 { return b.Min.Add(b.Max).Scale(0.5) }

// Size 尺寸（各轴长度）。
func (b AABB) Size() Vec3 { return b.Max.Sub(b.Min) }

// Mesh 三角网格。Positions 必填，Indices 长度必须是 3 的倍数；
// Normals 缺失时可用 RecomputeNormals 补齐，UVs / Colors 可选。
type Mesh struct {
	Positions []Vec3
	Normals   []Vec3
	UVs       []Vec2
	Colors    []RGBA
	Indices   []uint32
	Bounds    AABB
	// bvh 惰性构建的三角形加速结构（射线拾取用，见 Mesh.BVH）。
	bvh *BVH
}

// NewMesh 创建只有顶点与索引的空网格。
func NewMesh(pos []Vec3, idx []uint32) *Mesh {
	m := &Mesh{Positions: pos, Indices: idx}
	m.RecomputeBounds()
	return m
}

// TriCount 三角形数量。
func (m *Mesh) TriCount() int {
	if m == nil {
		return 0
	}
	return len(m.Indices) / 3
}

// RecomputeBounds 重算包围盒。
func (m *Mesh) RecomputeBounds() {
	if m == nil || len(m.Positions) == 0 {
		if m != nil {
			m.Bounds = AABB{}
		}
		return
	}
	b := AABB{Min: m.Positions[0], Max: m.Positions[0]}
	for _, p := range m.Positions[1:] {
		b.Min = b.Min.Min(p)
		b.Max = b.Max.Max(p)
	}
	m.Bounds = b
}

// RecomputeNormals 按面积加权重算法线（平滑法线）。
func (m *Mesh) RecomputeNormals() {
	if m == nil || len(m.Positions) == 0 {
		return
	}
	acc := make([]Vec3, len(m.Positions))
	for i := 0; i+2 < len(m.Indices); i += 3 {
		a, b, c := m.Indices[i], m.Indices[i+1], m.Indices[i+2]
		if int(a) >= len(m.Positions) || int(b) >= len(m.Positions) || int(c) >= len(m.Positions) {
			continue
		}
		pa, pb, pc := m.Positions[a], m.Positions[b], m.Positions[c]
		n := pb.Sub(pa).Cross(pc.Sub(pa)) // 未归一化 = 2×面积，天然面积加权
		acc[a] = acc[a].Add(n)
		acc[b] = acc[b].Add(n)
		acc[c] = acc[c].Add(n)
	}
	m.Normals = make([]Vec3, len(m.Positions))
	for i := range acc {
		m.Normals[i] = acc[i].Normalize()
	}
}

// Transformed 返回应用矩阵后的新网格（法线用逆转置变换）。
func (m *Mesh) Transformed(mat Mat4) *Mesh {
	if m == nil {
		return nil
	}
	nm := mat.NormalMatrix()
	o := &Mesh{
		Positions: make([]Vec3, len(m.Positions)),
		Indices:   append([]uint32(nil), m.Indices...),
	}
	for i, p := range m.Positions {
		o.Positions[i] = mat.TransformPoint(p)
	}
	if len(m.Normals) == len(m.Positions) {
		o.Normals = make([]Vec3, len(m.Normals))
		for i, n := range m.Normals {
			o.Normals[i] = nm.TransformDir(n).Normalize()
		}
	}
	if len(m.UVs) == len(m.Positions) {
		o.UVs = append([]Vec2(nil), m.UVs...)
	}
	if len(m.Colors) == len(m.Positions) {
		o.Colors = append([]RGBA(nil), m.Colors...)
	}
	o.RecomputeBounds()
	return o
}

// NewBox 轴对齐立方体（24 顶点，每面独立法线，UV 按面平铺）。
func NewBox(w, h, d float32) *Mesh {
	hx, hy, hz := w/2, h/2, d/2
	m := &Mesh{}
	face := func(n Vec3, a, b, c, dd Vec3) {
		base := uint32(len(m.Positions))
		m.Positions = append(m.Positions, a, b, c, dd)
		for i := 0; i < 4; i++ {
			m.Normals = append(m.Normals, n)
		}
		m.UVs = append(m.UVs, V2(0, 1), V2(1, 1), V2(1, 0), V2(0, 0))
		m.Indices = append(m.Indices, base, base+1, base+2, base, base+2, base+3)
	}
	// +Z
	face(V3(0, 0, 1), V3(-hx, -hy, hz), V3(hx, -hy, hz), V3(hx, hy, hz), V3(-hx, hy, hz))
	// -Z
	face(V3(0, 0, -1), V3(hx, -hy, -hz), V3(-hx, -hy, -hz), V3(-hx, hy, -hz), V3(hx, hy, -hz))
	// +X
	face(V3(1, 0, 0), V3(hx, -hy, hz), V3(hx, -hy, -hz), V3(hx, hy, -hz), V3(hx, hy, hz))
	// -X
	face(V3(-1, 0, 0), V3(-hx, -hy, -hz), V3(-hx, -hy, hz), V3(-hx, hy, hz), V3(-hx, hy, -hz))
	// +Y
	face(V3(0, 1, 0), V3(-hx, hy, hz), V3(hx, hy, hz), V3(hx, hy, -hz), V3(-hx, hy, -hz))
	// -Y
	face(V3(0, -1, 0), V3(-hx, -hy, -hz), V3(hx, -hy, -hz), V3(hx, -hy, hz), V3(-hx, -hy, hz))
	m.RecomputeBounds()
	return m
}

// NewSphere 经纬度球（半径 radius，seg 经度段数，rings 纬度圈数）。
func NewSphere(radius float32, seg, rings int) *Mesh {
	if seg < 3 {
		seg = 3
	}
	if rings < 2 {
		rings = 2
	}
	m := &Mesh{}
	for y := 0; y <= rings; y++ {
		v := float32(y) / float32(rings)
		theta := v * float32(math.Pi)
		st, ct := float32(math.Sin(float64(theta))), float32(math.Cos(float64(theta)))
		for x := 0; x <= seg; x++ {
			u := float32(x) / float32(seg)
			phi := u * 2 * float32(math.Pi)
			sp, cp := float32(math.Sin(float64(phi))), float32(math.Cos(float64(phi)))
			n := V3(sp*st, ct, cp*st)
			m.Positions = append(m.Positions, n.Scale(radius))
			m.Normals = append(m.Normals, n)
			m.UVs = append(m.UVs, V2(u, v))
		}
	}
	row := seg + 1
	for y := 0; y < rings; y++ {
		for x := 0; x < seg; x++ {
			a := uint32(y*row + x)
			b := a + 1
			c := uint32((y+1)*row + x)
			d := c + 1
			m.Indices = append(m.Indices, a, c, b, b, c, d)
		}
	}
	m.RecomputeBounds()
	return m
}

// NewPlane 水平面（XZ 平面，法线 +Y，sub 为每边细分段数）。
func NewPlane(w, d float32, sub int) *Mesh {
	if sub < 1 {
		sub = 1
	}
	m := &Mesh{}
	hx, hz := w/2, d/2
	for z := 0; z <= sub; z++ {
		for x := 0; x <= sub; x++ {
			u := float32(x) / float32(sub)
			v := float32(z) / float32(sub)
			m.Positions = append(m.Positions, V3(-hx+u*w, 0, -hz+v*d))
			m.Normals = append(m.Normals, V3(0, 1, 0))
			m.UVs = append(m.UVs, V2(u, v))
		}
	}
	row := sub + 1
	for z := 0; z < sub; z++ {
		for x := 0; x < sub; x++ {
			a := uint32(z*row + x)
			b := a + 1
			c := uint32((z+1)*row + x)
			d := c + 1
			m.Indices = append(m.Indices, a, c, b, b, c, d)
		}
	}
	m.RecomputeBounds()
	return m
}

// NewCylinder 圆柱（半径 r、高 h，seg 段，含上下底面）。
func NewCylinder(r, h float32, seg int) *Mesh {
	if seg < 3 {
		seg = 3
	}
	m := &Mesh{}
	hy := h / 2
	for i := 0; i <= seg; i++ {
		u := float32(i) / float32(seg)
		a := u * 2 * float32(math.Pi)
		sa, ca := float32(math.Sin(float64(a))), float32(math.Cos(float64(a)))
		n := V3(sa, 0, ca)
		m.Positions = append(m.Positions, V3(n.X*r, -hy, n.Z*r), V3(n.X*r, hy, n.Z*r))
		m.Normals = append(m.Normals, n, n)
		m.UVs = append(m.UVs, V2(u, 0), V2(u, 1))
	}
	for i := 0; i < seg; i++ {
		a := uint32(i * 2)
		b := a + 1
		c := a + 2
		d := a + 3
		m.Indices = append(m.Indices, a, b, c, b, d, c)
	}
	// 顶面
	top := uint32(len(m.Positions))
	m.Positions = append(m.Positions, V3(0, hy, 0))
	m.Normals = append(m.Normals, V3(0, 1, 0))
	m.UVs = append(m.UVs, V2(0.5, 0.5))
	for i := 0; i <= seg; i++ {
		a := float32(i) / float32(seg) * 2 * float32(math.Pi)
		m.Positions = append(m.Positions, V3(float32(math.Sin(float64(a)))*r, hy, float32(math.Cos(float64(a)))*r))
		m.Normals = append(m.Normals, V3(0, 1, 0))
		m.UVs = append(m.UVs, V2(0.5+float32(math.Sin(float64(a)))*0.5, 0.5+float32(math.Cos(float64(a)))*0.5))
	}
	for i := 0; i < seg; i++ {
		m.Indices = append(m.Indices, top, top+1+uint32(i), top+2+uint32(i))
	}
	// 底面
	bot := uint32(len(m.Positions))
	m.Positions = append(m.Positions, V3(0, -hy, 0))
	m.Normals = append(m.Normals, V3(0, -1, 0))
	m.UVs = append(m.UVs, V2(0.5, 0.5))
	for i := 0; i <= seg; i++ {
		a := float32(i) / float32(seg) * 2 * float32(math.Pi)
		m.Positions = append(m.Positions, V3(float32(math.Sin(float64(a)))*r, -hy, float32(math.Cos(float64(a)))*r))
		m.Normals = append(m.Normals, V3(0, -1, 0))
		m.UVs = append(m.UVs, V2(0.5+float32(math.Sin(float64(a)))*0.5, 0.5-float32(math.Cos(float64(a)))*0.5))
	}
	for i := 0; i < seg; i++ {
		m.Indices = append(m.Indices, bot, bot+2+uint32(i), bot+1+uint32(i))
	}
	m.RecomputeBounds()
	return m
}

// NewTorus 圆环（主半径 R、管半径 r，seg 主环段数，side 管截面段数）。
func NewTorus(R, r float32, seg, side int) *Mesh {
	if seg < 3 {
		seg = 3
	}
	if side < 3 {
		side = 3
	}
	m := &Mesh{}
	for i := 0; i <= seg; i++ {
		u := float32(i) / float32(seg)
		au := u * 2 * float32(math.Pi)
		su, cu := float32(math.Sin(float64(au))), float32(math.Cos(float64(au)))
		for j := 0; j <= side; j++ {
			v := float32(j) / float32(side)
			av := v * 2 * float32(math.Pi)
			sv, cv := float32(math.Sin(float64(av))), float32(math.Cos(float64(av)))
			n := V3(cu*cv, sv, su*cv)
			m.Positions = append(m.Positions, V3((R+r*cv)*cu, r*sv, (R+r*cv)*su))
			m.Normals = append(m.Normals, n)
			m.UVs = append(m.UVs, V2(u, v))
		}
	}
	row := side + 1
	for i := 0; i < seg; i++ {
		for j := 0; j < side; j++ {
			a := uint32(i*row + j)
			b := a + 1
			c := uint32((i+1)*row + j)
			d := c + 1
			m.Indices = append(m.Indices, a, c, b, b, c, d)
		}
	}
	m.RecomputeBounds()
	return m
}
