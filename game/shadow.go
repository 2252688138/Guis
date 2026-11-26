// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// Shadow 方向光实时阴影配置（软件 shadow map，默认关闭）。
//
// 只有方向光（Scene.AddSun）会投射阴影，且整个场景共用一张阴影图；
// 打开后每个受光片元多一次（PCF 时多次）光空间深度采样，建议阴影图 ≤ 512。
//
//	sh := game.NewShadowMap()
//	sh.On = true
//	sh.Size = 512
//	renderer.Shadow = sh
//
// 注意：Renderer 每帧调用 ShadowMap.Build 重建阴影图，静态场景可把
// ShadowMap.Once 设为 true 只构建一次（相机移动不影响平行光阴影）。
type Shadow struct {
	// On 是否启用阴影。
	On bool
	// Size 阴影图分辨率（默认 256，建议 256~1024）。
	Size int
	// Bias 深度偏移，抑制自阴影（默认 0.0025，阴影发花时调大）。
	Bias float32
	// PCF 采样核（1 / 3 / 5，默认 3；越大越软且越慢）。
	PCF int
	// Strength 阴影强度（0~1，默认 0.65，越大越黑）。
	Strength float32
	// Radius 光空间正交投影半径（0 = 按场景包围球自动计算）。
	Radius float32
	// Near, Far 光空间深度范围（0 = 自动）。
	Near, Far float32
	// MaxDistance 超过该距离的片元不接收阴影（0 = 不限），用于远处阴影降级。
	MaxDistance float32
	// Once 只构建一次阴影图（静态场景优化）。
	Once bool
}

// DefaultShadow 默认阴影配置（未开启）。
func DefaultShadow() *Shadow {
	return &Shadow{Size: 256, Bias: 0.0025, PCF: 3, Strength: 0.65}
}

// ShadowMap 可复用的软件阴影图（挂在 Renderer.Shadow 上）。
type ShadowMap struct {
	Shadow

	size    int
	depth   []float32 // 光空间归一化深度（0=近 1=远），取最小值为遮蔽
	lightVP Mat4
	center  Vec3 // 光空间包围球中心（单图阴影的 MaxDistance 判远用）
	radius  float32
	valid   bool
	built   bool // Once 模式下已构建

	// 统计（调试用）
	tris int
}

// NewShadowMap 创建阴影图（默认关闭）。
func NewShadowMap() *ShadowMap {
	m := &ShadowMap{size: 256}
	m.Shadow = *DefaultShadow()
	return m
}

// Reset 丢弃已构建的阴影图（Once 模式下场景变化后调用）。
func (m *ShadowMap) Reset() {
	if m == nil {
		return
	}
	m.valid = false
	m.built = false
	m.tris = 0
}

// Valid 阴影图当前是否可用于采样。
func (m *ShadowMap) Valid() bool { return m != nil && m.valid }

// Triangles 本帧写入阴影图的三角形数（调试用）。
func (m *ShadowMap) Triangles() int {
	if m == nil {
		return 0
	}
	return m.tris
}

// Depth 返回阴影图尺寸与深度缓冲（调试 / 可视化用，行优先，0=近）。
func (m *ShadowMap) Depth() (size int, depth []float32) {
	if m == nil {
		return 0, nil
	}
	return m.size, m.depth
}

// Build 按场景方向光重建阴影深度图。
func (m *ShadowMap) Build(sc *Scene) {
	if m == nil || !m.On {
		return
	}
	if m.Once && m.built && m.valid {
		return
	}
	m.valid = false
	m.tris = 0
	if sc == nil || sc.Root == nil || sc.Camera == nil {
		return
	}
	var dir Vec3
	found := false
	for i := range sc.Lights {
		lt := &sc.Lights[i]
		if lt.Kind == LightDirectional && lt.Intensity > 0 && lt.Dir.Len() > 0 {
			dir = lt.Dir.Normalize()
			found = true
			break
		}
	}
	if !found {
		return
	}
	// 世界矩阵需在包围盒与光栅化前就锻（重复调用幂等）
	sc.BuildWorld()

	// 1) 场景包围盒（世界空间）
	minP := V3(0, 0, 0)
	maxP := V3(0, 0, 0)
	has := false
	sc.Root.Each(func(n *Node) {
		if n.Mesh == nil || len(n.Mesh.Positions) == 0 || !n.Visible {
			return
		}
		b := n.Mesh.Bounds
		for i := 0; i < 8; i++ {
			p := V3(b.Min.X, b.Min.Y, b.Min.Z)
			if i&1 != 0 {
				p.X = b.Max.X
			}
			if i&2 != 0 {
				p.Y = b.Max.Y
			}
			if i&4 != 0 {
				p.Z = b.Max.Z
			}
			w := n.WorldMatrix().TransformPoint(p)
			if !has {
				minP, maxP, has = w, w, true
				return
			}
			minP = V3(minf(minP.X, w.X), minf(minP.Y, w.Y), minf(minP.Z, w.Z))
			maxP = V3(maxf(maxP.X, w.X), maxf(maxP.Y, w.Y), maxf(maxP.Z, w.Z))
		}
	})
	if !has {
		return
	}
	center := minP.Add(maxP).Scale(0.5)
	radius := maxP.Sub(minP).Len() * 0.5
	if radius < 0.5 {
		radius = 0.5
	}
	if m.Radius > 0 {
		radius = m.Radius
	}

	m.buildFor(sc, center, radius, dir)
}

// buildFor 用指定的光空间包围球构建阴影图（单图阴影与级联阴影各级共用）。
//
// center / radius 是光空间正交投影的球心与半宽，dir 为光线照射方向。
// 调用方需保证场景世界矩阵已构建（sc.BuildWorld）。
func (m *ShadowMap) buildFor(sc *Scene, center Vec3, radius float32, dir Vec3) {
	if m == nil || sc == nil || sc.Root == nil {
		return
	}
	if radius < 0.5 {
		radius = 0.5
	}
	if dir.Len() <= 0 {
		return
	}
	dir = dir.Normalize()
	m.center = center
	m.radius = radius
	m.valid = false
	m.built = false
	m.tris = 0

	// 1) 光空间相机：沿光线方向反向后退到包围球外
	dist := radius*2 + 1
	eye := center.Sub(dir.Scale(dist))
	up := V3(0, 1, 0)
	if float32(math.Abs(float64(dir.Y))) > 0.95 {
		up = V3(1, 0, 0)
	}
	near, far := dist-radius*1.5, dist+radius*1.5
	if m.Near > 0 {
		near = m.Near
	}
	if m.Far > 0 {
		far = m.Far
	}
	if far <= near {
		far = near + radius*3 + 1
	}
	view := LookAt(eye, center, up)
	proj := Ortho(-radius, radius, -radius, radius, near, far)
	m.lightVP = proj.Mul(view)

	// 2) 分配阴影图
	size := m.Size
	if size <= 0 {
		size = 256
	}
	if size < 32 {
		size = 32
	}
	if size > 2048 {
		size = 2048
	}
	if size != m.size || len(m.depth) != size*size {
		m.size = size
		m.depth = make([]float32, size*size)
	}
	for i := range m.depth {
		m.depth[i] = 1
	}

	// 3) 光栅化所有三角形到深度图
	sc.Root.Each(func(n *Node) {
		if n.Mesh == nil || !n.Visible {
			return
		}
		m.rasterMesh(n.Mesh, n.WorldMatrix())
	})
	m.valid = true
	m.built = true
}

// rasterMesh 把一个网格写入阴影图。
func (m *ShadowMap) rasterMesh(mesh *Mesh, world Mat4) {
	idx := mesh.Indices
	pos := mesh.Positions
	size := m.size
	for i := 0; i+2 < len(idx); i += 3 {
		i0, i1, i2 := int(idx[i]), int(idx[i+1]), int(idx[i+2])
		if i0 >= len(pos) || i1 >= len(pos) || i2 >= len(pos) {
			continue
		}
		a := world.TransformPoint(pos[i0])
		b := world.TransformPoint(pos[i1])
		c := world.TransformPoint(pos[i2])
		p0 := m.lightVP.TransformVec4(V4(a.X, a.Y, a.Z, 1))
		p1 := m.lightVP.TransformVec4(V4(b.X, b.Y, b.Z, 1))
		p2 := m.lightVP.TransformVec4(V4(c.X, c.Y, c.Z, 1))
		if p0.W <= 0 || p1.W <= 0 || p2.W <= 0 {
			continue
		}
		x0, y0, z0 := ndcToMap(p0, size)
		x1, y1, z1 := ndcToMap(p1, size)
		x2, y2, z2 := ndcToMap(p2, size)
		area := (x1-x0)*(y2-y0) - (y1-y0)*(x2-x0)
		if area == 0 || math.IsNaN(float64(area)) {
			continue
		}
		inv := 1 / area
		minX := int(math.Floor(float64(minf(x0, minf(x1, x2)))))
		maxX := int(math.Ceil(float64(maxf(x0, maxf(x1, x2)))))
		minY := int(math.Floor(float64(minf(y0, minf(y1, y2)))))
		maxY := int(math.Ceil(float64(maxf(y0, maxf(y1, y2)))))
		if minX < 0 {
			minX = 0
		}
		if minY < 0 {
			minY = 0
		}
		if maxX > size-1 {
			maxX = size - 1
		}
		if maxY > size-1 {
			maxY = size - 1
		}
		if minX > maxX || minY > maxY {
			continue
		}
		m.tris++
		for y := minY; y <= maxY; y++ {
			py := float32(y) + 0.5
			row := y * size
			for x := minX; x <= maxX; x++ {
				px := float32(x) + 0.5
				w0 := ((x1-px)*(y2-py) - (y1-py)*(x2-px)) * inv
				w1 := ((x2-px)*(y0-py) - (y2-py)*(x0-px)) * inv
				w2 := 1 - w0 - w1
				if w0 < 0 || w1 < 0 || w2 < 0 {
					continue
				}
				d := w0*z0 + w1*z1 + w2*z2
				if d < m.depth[row+x] {
					m.depth[row+x] = d
				}
			}
		}
	}
}

// ndcToMap 把裁剪空间坐标转成阴影图坐标与归一化深度。
func ndcToMap(p Vec4, size int) (float32, float32, float32) {
	iw := 1 / p.W
	x := (p.X*iw*0.5 + 0.5) * float32(size)
	y := (p.Y*iw*0.5 + 0.5) * float32(size)
	z := p.Z*iw*0.5 + 0.5
	return x, y, z
}

// Factor 返回世界坐标处的受光系数：1 = 完全受光，1-Strength = 完全处于阴影。
func (m *ShadowMap) Factor(world Vec3) float32 {
	if m == nil || !m.valid || len(m.depth) == 0 {
		return 1
	}
	if m.MaxDistance > 0 && m.center.Dist(world) > m.MaxDistance {
		return 1
	}
	c := m.lightVP.TransformVec4(V4(world.X, world.Y, world.Z, 1))
	if c.W <= 0 {
		return 1
	}
	x, y, z := ndcToMap(c, m.size)
	if x < 0 || y < 0 || x >= float32(m.size) || y >= float32(m.size) {
		return 1
	}
	d := z
	bias := m.Bias
	if bias <= 0 {
		bias = 0.0025
	}
	strength := m.Strength
	if strength <= 0 {
		strength = 0.65
	}
	n := m.PCF
	if n < 1 {
		n = 1
	}
	if n > 5 {
		n = 5
	}
	if n == 1 {
		if m.occluded(x, y, d-bias) {
			return 1 - strength
		}
		return 1
	}
	lit := 0
	half := n / 2
	for oy := -half; oy <= half; oy++ {
		for ox := -half; ox <= half; ox++ {
			sx := int(x) + ox
			sy := int(y) + oy
			if sx < 0 {
				sx = 0
			}
			if sy < 0 {
				sy = 0
			}
			if sx >= m.size {
				sx = m.size - 1
			}
			if sy >= m.size {
				sy = m.size - 1
			}
			if m.depth[sy*m.size+sx] >= d-bias {
				lit++
			}
		}
	}
	total := (half*2 + 1) * (half*2 + 1)
	if lit >= total {
		return 1
	}
	inShadow := float32(total-lit) / float32(total)
	return 1 - strength*inShadow
}

// occluded 单点深度比较：阴影图里的遮挡深度是否比当前片元更近。
func (m *ShadowMap) occluded(x, y, d float32) bool {
	ix, iy := int(x), int(y)
	if ix < 0 || iy < 0 || ix >= m.size || iy >= m.size {
		return false
	}
	return m.depth[iy*m.size+ix] < d
}
