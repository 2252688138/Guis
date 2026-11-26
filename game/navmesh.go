// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"math"
	"math/rand"
)

// 多边形导航网格（NavMesh）。
//
// nav.go 里的 NavGrid 是格子 A*：实现简单，但路径只能沿格子走、拐角生硬，
// 对地形起伏与大面积可行走区的表达力有限。NavMesh 用「凸多边形面积」表达可
// 行走区域：多边形内部任意两点之间直线可达，于是寻路变成多边形级 A* +
// 漏斗算法（simple stupid funnel）求真实拐点，路径短、转弯少，也不受网格
// 分辨率限制。
//
//	mesh := game.NewNavMesh()
//	mesh.BuildFromMesh(levelMesh, game.Identity(), game.NavBuildOpts{MaxSlope: 45})
//	path, ok := mesh.FindPath(from, to)
//
// 约定：多边形顶点在 XZ 平面上按逆时针（俯视）排列，法线朝上；所有几何判定
// 都投影到 XZ 平面进行（导航网格本就用于可行走面，竖直面不参与）。

// navEps 顶点/共线判定容差。
const navEps = 1e-3

// NavPoly 一个凸多边形导航面。
type NavPoly struct {
	// Verts 顶点（XZ 平面逆时针，可带高度）。
	Verts []Vec3
	// Center 多边形中心（顶点平均）。
	Center Vec3
	// Normal 单位法线（朝上）。
	Normal Vec3
	// Neighbors 相邻多边形下标。
	Neighbors []int
	// Cost 进入该多边形的额外代价（默认 1，可用来表达泥地/沼泽）。
	Cost float32
	// Flags 自定义标记位（供上层区分区域类型）。
	Flags uint16
}

// navPortal 两个多边形之间的门户（共享边）。
type navPortal struct {
	a, b Vec3
	mid  Vec3
}

// navPair 无向邻接对。
type navPair struct{ a, b int }

// NavMesh 多边形导航网格。
type NavMesh struct {
	polys   []NavPoly
	portals map[navPair]navPortal
	bounds  AABB
	hasBnd  bool
}

// NewNavMesh 创建空导航网格。
func NewNavMesh() *NavMesh {
	return &NavMesh{portals: map[navPair]navPortal{}}
}

// AddPolygon 添加一个凸多边形，返回其下标（顶点不足 3 个返回 -1）。
func (m *NavMesh) AddPolygon(verts []Vec3) int {
	if m == nil || len(verts) < 3 {
		return -1
	}
	vs := make([]Vec3, len(verts))
	copy(vs, verts)
	n := navNewellNormal(vs)
	if n.Y < 0 {
		// 统一成「俯视逆时针」
		for i, j := 0, len(vs)-1; i < j; i, j = i+1, j-1 {
			vs[i], vs[j] = vs[j], vs[i]
		}
		n = n.Neg()
	}
	if n.Len() < 1e-6 {
		return -1
	}
	p := NavPoly{
		Verts:  vs,
		Center: navCentroid(vs),
		Normal: n.Normalize(),
		Cost:   1,
	}
	m.polys = append(m.polys, p)
	for _, v := range vs {
		if !m.hasBnd {
			m.bounds = AABB{Min: v, Max: v}
			m.hasBnd = true
			continue
		}
		m.bounds.Min = m.bounds.Min.Min(v)
		m.bounds.Max = m.bounds.Max.Max(v)
	}
	return len(m.polys) - 1
}

// PolygonCount 多边形数量。
func (m *NavMesh) PolygonCount() int {
	if m == nil {
		return 0
	}
	return len(m.polys)
}

// Poly 按下标取多边形（越界返回 nil）。
func (m *NavMesh) Poly(i int) *NavPoly {
	if m == nil || i < 0 || i >= len(m.polys) {
		return nil
	}
	return &m.polys[i]
}

// Bounds 整个导航网格的包围盒。
func (m *NavMesh) Bounds() AABB {
	if m == nil || !m.hasBnd {
		return AABB{}
	}
	return m.bounds
}

// LinkNeighbors 按共享边自动建立邻接关系（可重复调用，先清空旧邻接）。
func (m *NavMesh) LinkNeighbors() {
	if m == nil {
		return
	}
	for i := range m.polys {
		m.polys[i].Neighbors = m.polys[i].Neighbors[:0]
	}
	m.portals = make(map[navPair]navPortal, len(m.polys))
	for i := 0; i < len(m.polys); i++ {
		bi := navPolyBounds(&m.polys[i])
		for j := i + 1; j < len(m.polys); j++ {
			bj := navPolyBounds(&m.polys[j])
			// 快速排除：包围盒不相接就不可能共享边
			if !navBoxNear(bi, bj, 0.05) {
				continue
			}
			a, b, ok := navSharedEdge(&m.polys[i], &m.polys[j])
			if !ok {
				continue
			}
			m.polys[i].Neighbors = append(m.polys[i].Neighbors, j)
			m.polys[j].Neighbors = append(m.polys[j].Neighbors, i)
			p := navPortal{a: a, b: b, mid: a.Add(b).Scale(0.5)}
			m.portals[navPair{i, j}] = p
			m.portals[navPair{j, i}] = p
		}
	}
}

// Portal 返回两个相邻多边形之间的门户端点。
func (m *NavMesh) Portal(from, to int) (Vec3, Vec3, bool) {
	if m == nil || m.portals == nil {
		return Vec3{}, Vec3{}, false
	}
	p, ok := m.portals[navPair{from, to}]
	return p.a, p.b, ok
}

// ClosestPoint 返回网格上离 p 最近的点及其所在多边形。
func (m *NavMesh) ClosestPoint(p Vec3) (Vec3, int, bool) {
	if m == nil || len(m.polys) == 0 {
		return Vec3{}, -1, false
	}
	best := Vec3{}
	bestIdx := -1
	bestD := float32(math.MaxFloat32)
	for i := range m.polys {
		q, in := navClosestInPoly(&m.polys[i], p)
		if in {
			return q, i, true
		}
		if d := q.Dist(p); d < bestD {
			best, bestIdx, bestD = q, i, d
		}
	}
	return best, bestIdx, bestIdx >= 0
}

// Contains 点是否落在网格内（XZ 平面判定，容差 1e-3）。
func (m *NavMesh) Contains(p Vec3) bool {
	if m == nil {
		return false
	}
	for i := range m.polys {
		if navPointInPoly(&m.polys[i], p) {
			return true
		}
	}
	return false
}

// RandomPoint 按面积加权随机取一点。
func (m *NavMesh) RandomPoint(r *rand.Rand) (Vec3, bool) {
	if m == nil || len(m.polys) == 0 {
		return Vec3{}, false
	}
	if r == nil {
		r = rand.New(rand.NewSource(1))
	}
	total := float32(0)
	for i := range m.polys {
		total += navPolyArea(&m.polys[i])
	}
	if total <= 0 {
		return m.polys[0].Center, true
	}
	t := r.Float32() * total
	for i := range m.polys {
		a := navPolyArea(&m.polys[i])
		if t <= a || i == len(m.polys)-1 {
			return navRandomInPoly(&m.polys[i], r), true
		}
		t -= a
	}
	return m.polys[0].Center, true
}

// PathLength 路径总长。
func (m *NavMesh) PathLength(path []Vec3) float32 {
	total := float32(0)
	for i := 1; i < len(path); i++ {
		total += path[i].Dist(path[i-1])
	}
	return total
}

// ---------------------------------------------------------------- 几何辅助

// navNewellNormal Newell 法求多边形法线（未归一化）。
func navNewellNormal(vs []Vec3) Vec3 {
	var n Vec3
	for i := range vs {
		a := vs[i]
		b := vs[(i+1)%len(vs)]
		n.X += (a.Y - b.Y) * (a.Z + b.Z)
		n.Y += (a.Z - b.Z) * (a.X + b.X)
		n.Z += (a.X - b.X) * (a.Y + b.Y)
	}
	return n
}

// navCentroid 顶点平均中心。
func navCentroid(vs []Vec3) Vec3 {
	var c Vec3
	for _, v := range vs {
		c = c.Add(v)
	}
	return c.Scale(1 / float32(len(vs)))
}

// navPolyArea 多边形在 XZ 平面的面积。
func navPolyArea(p *NavPoly) float32 {
	if p == nil || len(p.Verts) < 3 {
		return 0
	}
	a := float32(0)
	for i := 1; i+1 < len(p.Verts); i++ {
		a += navTri2Area(p.Verts[0], p.Verts[i], p.Verts[i+1])
	}
	if a < 0 {
		a = -a
	}
	return a
}

// navTri2Area XZ 平面三角形有符号面积。
func navTri2Area(a, b, c Vec3) float32 {
	return (b.X-a.X)*(c.Z-a.Z) - (c.X-a.X)*(b.Z-a.Z)
}

// navPolyBounds 多边形包围盒。
func navPolyBounds(p *NavPoly) AABB {
	b := AABB{Min: p.Verts[0], Max: p.Verts[0]}
	for _, v := range p.Verts[1:] {
		b.Min = b.Min.Min(v)
		b.Max = b.Max.Max(v)
	}
	return b
}

// navBoxNear 两个包围盒在 XZ 平面是否接近（容差 tol）。
func navBoxNear(a, b AABB, tol float32) bool {
	if a.Max.X+tol < b.Min.X || b.Max.X+tol < a.Min.X {
		return false
	}
	if a.Max.Z+tol < b.Min.Z || b.Max.Z+tol < a.Min.Z {
		return false
	}
	return true
}

// navSharedEdge 找两个多边形的共享边，返回其两个端点。
func navSharedEdge(a, b *NavPoly) (Vec3, Vec3, bool) {
	for i := range a.Verts {
		a0 := a.Verts[i]
		a1 := a.Verts[(i+1)%len(a.Verts)]
		for j := range b.Verts {
			b0 := b.Verts[j]
			b1 := b.Verts[(j+1)%len(b.Verts)]
			if navSameXZ(a0, b1) && navSameXZ(a1, b0) {
				return a0, a1, true
			}
			if navSameXZ(a0, b0) && navSameXZ(a1, b1) {
				return a0, a1, true
			}
		}
	}
	return Vec3{}, Vec3{}, false
}

// navSameXZ 两个点在 XZ 平面是否重合（高度容差放宽到 0.5，允许台阶）。
func navSameXZ(a, b Vec3) bool {
	if math.Abs(float64(a.X-b.X)) > navEps || math.Abs(float64(a.Z-b.Z)) > navEps {
		return false
	}
	return math.Abs(float64(a.Y-b.Y)) <= 0.5
}

// navPointInPoly 点是否在凸多边形内（XZ 投影，重心定侧，稳健于绕向）。
func navPointInPoly(p *NavPoly, pt Vec3) bool {
	if p == nil || len(p.Verts) < 3 {
		return false
	}
	c := p.Center
	for i := range p.Verts {
		a := p.Verts[i]
		b := p.Verts[(i+1)%len(p.Verts)]
		sc := navTri2Area(a, b, c)
		sp := navTri2Area(a, b, pt)
		if sc*sp < 0 && math.Abs(float64(sp)) > navEps {
			return false
		}
	}
	return true
}

// navClosestInPoly 返回多边形内/边上离 pt 最近的点，in 表示是否已在多边形内。
func navClosestInPoly(p *NavPoly, pt Vec3) (Vec3, bool) {
	if navPointInPoly(p, pt) {
		return navProjectToPlane(p, pt), true
	}
	best := p.Verts[0]
	bestD := best.Dist(pt)
	for i := range p.Verts {
		a := p.Verts[i]
		b := p.Verts[(i+1)%len(p.Verts)]
		q := navClosestOnSeg(a, b, pt)
		if d := q.Dist(pt); d < bestD {
			best, bestD = q, d
		}
	}
	return best, false
}

// navProjectToPlane 把点投影到多边形所在平面。
func navProjectToPlane(p *NavPoly, pt Vec3) Vec3 {
	n := p.Normal
	d := pt.Sub(p.Center).Dot(n)
	return pt.Sub(n.Scale(d))
}

// navClosestOnSeg 线段上离 pt 最近的点。
func navClosestOnSeg(a, b, pt Vec3) Vec3 {
	ab := b.Sub(a)
	l2 := ab.Dot(ab)
	if l2 < 1e-9 {
		return a
	}
	t := pt.Sub(a).Dot(ab) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return a.Add(ab.Scale(t))
}

// navRandomInPoly 在多边形内均匀取点（扇形三角加权）。
func navRandomInPoly(p *NavPoly, r *rand.Rand) Vec3 {
	if len(p.Verts) < 3 {
		return p.Center
	}
	total := float32(0)
	areas := make([]float32, 0, len(p.Verts)-2)
	for i := 1; i+1 < len(p.Verts); i++ {
		a := navTri2Area(p.Verts[0], p.Verts[i], p.Verts[i+1])
		if a < 0 {
			a = -a
		}
		areas = append(areas, a)
		total += a
	}
	if total <= 0 {
		return p.Center
	}
	t := r.Float32() * total
	idx := 0
	for i, a := range areas {
		idx = i
		if t <= a {
			break
		}
		t -= a
	}
	v0 := p.Verts[0]
	v1 := p.Verts[idx+1]
	v2 := p.Verts[idx+2]
	u := r.Float32()
	v := r.Float32()
	if u+v > 1 {
		u, v = 1-u, 1-v
	}
	return v0.Add(v1.Sub(v0).Scale(u)).Add(v2.Sub(v0).Scale(v))
}
