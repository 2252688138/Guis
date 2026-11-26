// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"errors"
	"math"
)

// NavMesh 的构建入口：把一个三角网格 / 高度图 / 一组盒体转成凸多边形导航网格。
//
// 构建流程统一为三步：① 逐三角形做坡度与面积过滤；② 可选地把共面且共边的
// 三角形贪心合并成更大的凸多边形（减少寻路节点数与拐点）；③ LinkNeighbors
// 建立邻接关系。

// NavBuildOpts 导航网格构建参数（零值即默认：坡度 45°、不合并）。
type NavBuildOpts struct {
	// MaxSlope 可行走最大坡度（度，默认 45）。
	MaxSlope float32
	// MinPolyArea 小于该面积（XZ 投影）的面被丢弃（默认 0 = 不过滤）。
	MinPolyArea float32
	// MergeCoplanar 是否把共面共边的三角形合并成更大的凸多边形。
	MergeCoplanar bool
	// MergeMaxVerts 合并后多边形顶点上限（默认 8）。
	MergeMaxVerts int
	// CellSize 高度图逐格采样时的格边长（0 = 用高度图自身 Cell）。
	CellSize float32
}

// norm 补齐默认值。
func (o NavBuildOpts) norm() NavBuildOpts {
	if o.MaxSlope <= 0 {
		o.MaxSlope = 45
	}
	if o.MaxSlope > 89 {
		o.MaxSlope = 89
	}
	if o.MergeMaxVerts < 3 {
		o.MergeMaxVerts = 8
	}
	if o.MergeMaxVerts > 24 {
		o.MergeMaxVerts = 24
	}
	return o
}

// BuildFromMesh 从三角网格构建导航网格（world 把顶点变换到世界空间）。
// 会把已有内容清空重建。
func (m *NavMesh) BuildFromMesh(mesh *Mesh, world Mat4, opts NavBuildOpts) error {
	if m == nil {
		return errors.New("game: 导航网格为空")
	}
	if mesh == nil || len(mesh.Positions) == 0 {
		return errors.New("game: 网格没有顶点")
	}
	opts = opts.norm()
	cosLimit := float32(math.Cos(float64(opts.MaxSlope) * math.Pi / 180))

	m.reset()
	pos := make([]Vec3, len(mesh.Positions))
	for i, p := range mesh.Positions {
		pos[i] = world.TransformPoint(p)
	}
	idx := mesh.Indices
	tri := func(a, b, c uint32) {
		v0, v1, v2 := pos[a], pos[b], pos[c]
		n := v1.Sub(v0).Cross(v2.Sub(v0))
		if n.Len() < 1e-9 {
			return
		}
		n = n.Normalize()
		// 网格绕向可能是 CW 也可能是 CCW：只看法线竖直分量绝对值，
		// 朝下的面由 AddPolygon 规范成朝上，只有真正过陡的才丢弃。
		if float32(math.Abs(float64(n.Y))) < cosLimit {
			return
		}
		if opts.MinPolyArea > 0 {
			a2 := navTri2Area(v0, v1, v2)
			if a2 < 0 {
				a2 = -a2
			}
			if a2*0.5 < opts.MinPolyArea {
				return
			}
		}
		m.AddPolygon([]Vec3{v0, v1, v2})
	}
	if len(idx) >= 3 {
		for i := 0; i+2 < len(idx); i += 3 {
			tri(idx[i], idx[i+1], idx[i+2])
		}
	} else {
		for i := 0; i+2 < len(pos); i += 3 {
			tri(uint32(i), uint32(i+1), uint32(i+2))
		}
	}
	if opts.MergeCoplanar {
		m.mergeCoplanar(opts.MergeMaxVerts)
	}
	m.LinkNeighbors()
	return nil
}

// BuildNavMeshFromHeightmap 用高度图铺满可行走区域（每格一个凸四边形）。
// walkableSlope 是允许的最大坡度（度）；传 0 用默认 45°。
func BuildNavMeshFromHeightmap(hm *Heightmap, world Mat4, walkableSlope float32) (*NavMesh, error) {
	if hm == nil || hm.W < 2 || hm.H < 2 || len(hm.Heights) < hm.W*hm.H {
		return nil, errors.New("game: 高度图尺寸非法")
	}
	if walkableSlope <= 0 {
		walkableSlope = 45
	}
	cosLimit := float32(math.Cos(float64(walkableSlope) * math.Pi / 180))
	m := NewNavMesh()
	cell := hm.Cell
	if cell <= 0 {
		cell = 1
	}
	height := func(x, y int) float32 {
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if x >= hm.W {
			x = hm.W - 1
		}
		if y >= hm.H {
			y = hm.H - 1
		}
		return hm.Heights[y*hm.W+x]
	}
	wp := func(x, y int) Vec3 {
		p := V3(hm.Origin.X+float32(x)*cell, height(x, y), hm.Origin.Z+float32(y)*cell)
		return world.TransformPoint(p)
	}
	for y := 0; y+1 < hm.H; y++ {
		for x := 0; x+1 < hm.W; x++ {
			// 以格子中心的两条对角差分估坡度
			dhx := (height(x+1, y) + height(x+1, y+1)) - (height(x, y) + height(x, y+1))
			dhz := (height(x, y+1) + height(x+1, y+1)) - (height(x, y) + height(x+1, y))
			span := 2 * cell
			n := V3(-dhx, span, -dhz).Normalize()
			if n.Y < cosLimit {
				continue
			}
			a := wp(x, y)
			b := wp(x+1, y)
			c := wp(x+1, y+1)
			d := wp(x, y+1)
			m.AddPolygon([]Vec3{a, b, c, d})
		}
	}
	if m.PolygonCount() == 0 {
		return nil, errors.New("game: 高度图里没有可行走面")
	}
	m.LinkNeighbors()
	return m, nil
}

// BuildNavMeshFromBoxes 把每个盒体的顶面当作一块可行走平台。
// 顶面高度差 ≤ 0.5、XZ 相接的平台会自动连通（LinkNeighbors 的高度容差）。
func BuildNavMeshFromBoxes(boxes []AABB, world Mat4) *NavMesh {
	m := NewNavMesh()
	for _, b := range boxes {
		if b.Max.X <= b.Min.X || b.Max.Z <= b.Min.Z {
			continue
		}
		y := b.Max.Y
		p0 := world.TransformPoint(V3(b.Min.X, y, b.Min.Z))
		p1 := world.TransformPoint(V3(b.Max.X, y, b.Min.Z))
		p2 := world.TransformPoint(V3(b.Max.X, y, b.Max.Z))
		p3 := world.TransformPoint(V3(b.Min.X, y, b.Max.Z))
		m.AddPolygon([]Vec3{p0, p1, p2, p3})
	}
	m.LinkNeighbors()
	return m
}

// ---------------------------------------------------------------- 共面合并

// reset 清空内容。
func (m *NavMesh) reset() {
	m.polys = m.polys[:0]
	m.portals = map[navPair]navPortal{}
	m.bounds, m.hasBnd = AABB{}, false
}

// mergeCoplanar 反复扫描，把共面、共边、合并后仍凸的多边形合并成更大的面。
func (m *NavMesh) mergeCoplanar(maxVerts int) {
	if m == nil || len(m.polys) < 2 {
		return
	}
	dead := make([]bool, len(m.polys))
	for round := 0; round < 8; round++ {
		merged := false
		for i := 0; i < len(m.polys); i++ {
			if dead[i] {
				continue
			}
			for j := i + 1; j < len(m.polys); j++ {
				if dead[j] {
					continue
				}
				if len(m.polys[i].Verts)+len(m.polys[j].Verts)-2 > maxVerts {
					continue
				}
				if !navCoplanar(&m.polys[i], &m.polys[j]) {
					continue
				}
				out, ok := navMergePolyVerts(m.polys[i].Verts, m.polys[j].Verts)
				if !ok || !navIsConvex(out) {
					continue
				}
				n := navNewellNormal(out)
				if n.Y < 0 {
					for a, b := 0, len(out)-1; a < b; a, b = a+1, b-1 {
						out[a], out[b] = out[b], out[a]
					}
					n = n.Neg()
				}
				m.polys[i].Verts = out
				m.polys[i].Center = navCentroid(out)
				m.polys[i].Normal = n.Normalize()
				dead[j] = true
				merged = true
			}
		}
		if !merged {
			break
		}
		// 压缩掉被吞并的多边形
		keep := make([]NavPoly, 0, len(m.polys))
		for i := range m.polys {
			if !dead[i] {
				keep = append(keep, m.polys[i])
			}
		}
		m.polys = keep
		dead = make([]bool, len(m.polys))
	}
}

// navCoplanar 两个多边形是否近似共面（法线同向且顶点贴合对方平面）。
func navCoplanar(a, b *NavPoly) bool {
	if a.Normal.Dot(b.Normal) < 0.9995 {
		return false
	}
	for _, v := range b.Verts {
		if math.Abs(float64(v.Sub(a.Center).Dot(a.Normal))) > 1e-2 {
			return false
		}
	}
	return true
}

// navMergePolyVerts 沿共享边把 b 拼到 a 上，返回合并后的顶点环。
func navMergePolyVerts(a, b []Vec3) ([]Vec3, bool) {
	for ii := range a {
		a0 := a[ii]
		a1 := a[(ii+1)%len(a)]
		for jj := range b {
			b0 := b[jj]
			b1 := b[(jj+1)%len(b)]
			// 反向共享边：a 的 a0→a1 与 b 的 b0(b1→b0 方向) 对应
			if !navSameXZ(a0, b1) || !navSameXZ(a1, b0) {
				continue
			}
			out := make([]Vec3, 0, len(a)+len(b)-2)
			// 先走 a：从 a1 开始绕一圈回到 a0
			for k := 0; k < len(a); k++ {
				out = append(out, a[(ii+1+k)%len(a)])
			}
			// 再沿 b 的反向补齐（跳过两个共享端点）
			for k := 1; k <= len(b)-2; k++ {
				out = append(out, b[(jj-k+len(b))%len(b)])
			}
			return out, true
		}
	}
	return nil, false
}

// navIsConvex 顶点环在 XZ 投影上是否凸（所有转角同号）。
func navIsConvex(vs []Vec3) bool {
	if len(vs) < 3 {
		return false
	}
	sign := 0
	for i := range vs {
		a := vs[i]
		b := vs[(i+1)%len(vs)]
		c := vs[(i+2)%len(vs)]
		cr := navTri2Area(a, b, c)
		if math.Abs(float64(cr)) < 1e-6 {
			continue // 共线点，忽略
		}
		s := 1
		if cr < 0 {
			s = -1
		}
		if sign == 0 {
			sign = s
		} else if sign != s {
			return false
		}
	}
	return sign != 0
}
