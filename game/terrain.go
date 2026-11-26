// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// 高度图地形：大型户外场景的地面。
//
//   - Heightmap：双线性高度查询 + 法线查询（角色贴地 / 抛物落点）
//   - Chunk：按块生成网格，块内按距离做 LOD（步长 1/2/4/8），
//     大世界只需保留相机附近的细节三角形
//   - Terrain：把块挂成 Scene 节点，Update 按相机位置切换 LOD

// Heightmap 高度图。
type Heightmap struct {
	// W, H 顶点列数 / 行数。
	W, H int
	// Heights 高度值（长度 W*H，行优先）。
	Heights []float32
	// Cell 网格间距（世界单位）。
	Cell float32
	// Origin 顶点 (0,0) 的世界坐标（X/Z 平面）。
	Origin Vec3
}

// NewHeightmap 创建全零高度图。
func NewHeightmap(w, h int, cell float32) *Heightmap {
	if w < 2 {
		w = 2
	}
	if h < 2 {
		h = 2
	}
	if cell <= 0 {
		cell = 1
	}
	return &Heightmap{
		W:       w,
		H:       h,
		Heights: make([]float32, w*h),
		Cell:    cell,
	}
}

// Set 设置某顶点高度（越界忽略）。
func (h *Heightmap) Set(x, y int, v float32) {
	if h == nil || x < 0 || y < 0 || x >= h.W || y >= h.H {
		return
	}
	h.Heights[y*h.W+x] = v
}

// At 读取某顶点高度。
func (h *Heightmap) At(x, y int) float32 {
	if h == nil {
		return 0
	}
	x = clampi(x, 0, h.W-1)
	y = clampi(y, 0, h.H-1)
	return h.Heights[y*h.W+x]
}

// Fill 用外部函数填充高度（可用于噪声地形）。
func (h *Heightmap) Fill(fn func(x, y int, wx, wz float32) float32) {
	if h == nil || fn == nil {
		return
	}
	for y := 0; y < h.H; y++ {
		for x := 0; x < h.W; x++ {
			wx := h.Origin.X + float32(x)*h.Cell
			wz := h.Origin.Z + float32(y)*h.Cell
			h.Heights[y*h.W+x] = fn(x, y, wx, wz)
		}
	}
}

// HeightAt 世界坐标 (wx, wz) 处的双线性插值高度。
func (h *Heightmap) HeightAt(wx, wz float32) float32 {
	if h == nil {
		return 0
	}
	fx := (wx - h.Origin.X) / h.Cell
	fz := (wz - h.Origin.Z) / h.Cell
	x0 := int(math.Floor(float64(fx)))
	z0 := int(math.Floor(float64(fz)))
	tx := fx - float32(x0)
	tz := fz - float32(z0)
	h00 := h.At(x0, z0)
	h10 := h.At(x0+1, z0)
	h01 := h.At(x0, z0+1)
	h11 := h.At(x0+1, z0+1)
	top := lerpf(h00, h10, tx)
	bot := lerpf(h01, h11, tx)
	return lerpf(top, bot, tz)
}

// NormalAt 世界坐标处的法线（中心差分）。
func (h *Heightmap) NormalAt(wx, wz float32) Vec3 {
	if h == nil {
		return V3(0, 1, 0)
	}
	d := h.Cell
	hl := h.HeightAt(wx-d, wz)
	hr := h.HeightAt(wx+d, wz)
	hd := h.HeightAt(wx, wz-d)
	hu := h.HeightAt(wx, wz+d)
	n := V3(hl-hr, 2*d, hd-hu)
	return n.Normalize()
}

// ChunkMesh 生成一块地形网格（step 为网格步长，1 = 全细节）。
//
// cx、cz 为块起点顶点索引，size 为块内顶点跨度（会被 step 调整）。
func (h *Heightmap) ChunkMesh(cx, cz, size, step int) *Mesh {
	if h == nil {
		return nil
	}
	if step < 1 {
		step = 1
	}
	// 顶点数（含右/下边界）
	nx := size/step + 1
	nz := size/step + 1
	if cx+nx*step > h.W-1 {
		nx = (h.W-1-cx)/step + 1
	}
	if cz+nz*step > h.H-1 {
		nz = (h.H-1-cz)/step + 1
	}
	if nx < 2 || nz < 2 {
		return nil
	}
	pos := make([]Vec3, 0, nx*nz)
	uv := make([]Vec2, 0, nx*nz)
	nrm := make([]Vec3, 0, nx*nz)
	for j := 0; j < nz; j++ {
		for i := 0; i < nx; i++ {
			gx := cx + i*step
			gz := cz + j*step
			wx := h.Origin.X + float32(gx)*h.Cell
			wz := h.Origin.Z + float32(gz)*h.Cell
			pos = append(pos, V3(wx, h.At(gx, gz), wz))
			uv = append(uv, Vec2{float32(gx) / float32(h.W-1), float32(gz) / float32(h.H-1)})
			nrm = append(nrm, h.NormalAt(wx, wz))
		}
	}
	idx := make([]uint32, 0, (nx-1)*(nz-1)*6)
	for j := 0; j < nz-1; j++ {
		for i := 0; i < nx-1; i++ {
			a := uint32(j*nx + i)
			b := a + 1
			c := uint32((j+1)*nx + i)
			d := c + 1
			// 逆时针（从上往下看）
			idx = append(idx, a, c, b, b, c, d)
		}
	}
	m := NewMesh(pos, idx)
	m.Normals = nrm
	m.UVs = uv
	m.RecomputeBounds()
	return m
}

// TerrainChunk 地形块。
type TerrainChunk struct {
	// X, Z 块起点顶点索引。
	X, Z int
	// Size 块顶点跨度。
	Size int
	// Nodes 各 LOD 级别的网格。
	Meshes []*Mesh
	// Node 场景节点。
	Node *Node
	// Box 世界包围盒（不含高度变化，用于剔除粗判）。
	Box AABB
	// LOD 当前级别。
	LOD int
}

// Terrain 地形（块 + LOD 管理）。
type Terrain struct {
	// HM 高度图。
	HM *Heightmap
	// ChunkSize 每块顶点跨度。
	ChunkSize int
	// LODSteps 各 LOD 级别的顶点步长（升序，默认 1/2/4/8）。
	LODSteps []int
	// LODDistances 各 LOD 的距离阈值（长度 = len(LODSteps)-1，默认 40/90/180）。
	LODDistances []float32
	// Material 地形材质。
	Material *Material
	// Root 挂载根节点。
	Root *Node
	// Chunks 全部块。
	Chunks []*TerrainChunk
	// Switches 累计 LOD 切换次数。
	Switches int
}

// NewTerrain 创建地形（自动生成块与 LOD 网格）。
func NewTerrain(hm *Heightmap, chunkSize int, mat *Material) *Terrain {
	if hm == nil {
		return nil
	}
	if chunkSize < 2 {
		chunkSize = 16
	}
	if mat == nil {
		mat = DefaultMaterial()
	}
	t := &Terrain{
		HM:           hm,
		ChunkSize:    chunkSize,
		LODSteps:     []int{1, 2, 4, 8},
		LODDistances: []float32{40, 90, 180},
		Material:     mat,
		Root:         &Node{Name: "terrain"},
	}
	span := chunkSize
	for cz := 0; cz < hm.H-1; cz += span {
		for cx := 0; cx < hm.W-1; cx += span {
			c := &TerrainChunk{X: cx, Z: cz, Size: span}
			for _, step := range t.LODSteps {
				if m := hm.ChunkMesh(cx, cz, span, step); m != nil {
					c.Meshes = append(c.Meshes, m)
				}
			}
			if len(c.Meshes) == 0 {
				continue
			}
			b := c.Meshes[0].Bounds
			c.Box = AABB{Min: b.Min, Max: b.Max}
			c.Node = NewMeshNode("chunk", c.Meshes[0], mat)
			c.Node.Pos = V3(0, 0, 0)
			t.Root.Add(c.Node)
			t.Chunks = append(t.Chunks, c)
		}
	}
	return t
}

// Update 按相机位置切换各块的 LOD，返回本帧切换次数。
func (t *Terrain) Update(camPos Vec3) int {
	if t == nil {
		return 0
	}
	n := 0
	for _, c := range t.Chunks {
		if c.Node == nil || len(c.Meshes) == 0 {
			continue
		}
		center := c.Box.Center()
		d := center.Sub(camPos).Len()
		lod := len(t.LODDistances)
		for i, th := range t.LODDistances {
			if d < th {
				lod = i
				break
			}
		}
		if lod >= len(c.Meshes) {
			lod = len(c.Meshes) - 1
		}
		if lod != c.LOD || c.Node.Mesh != c.Meshes[lod] {
			c.LOD = lod
			c.Node.Mesh = c.Meshes[lod]
			n++
		}
	}
	t.Switches += n
	return n
}

// HeightAt 地形世界坐标高度（转发高度图）。
func (t *Terrain) HeightAt(wx, wz float32) float32 {
	if t == nil || t.HM == nil {
		return 0
	}
	return t.HM.HeightAt(wx, wz)
}

// NormalAt 地形世界坐标法线。
func (t *Terrain) NormalAt(wx, wz float32) Vec3 {
	if t == nil || t.HM == nil {
		return V3(0, 1, 0)
	}
	return t.HM.NormalAt(wx, wz)
}

// Stats 地形统计（块数 / 各级 LOD 块数 / 三角形总数）。
func (t *Terrain) Stats() (chunks int, perLOD []int, tris int) {
	if t == nil {
		return 0, nil, 0
	}
	perLOD = make([]int, len(t.LODSteps))
	for _, c := range t.Chunks {
		if c.LOD >= 0 && c.LOD < len(perLOD) {
			perLOD[c.LOD]++
		}
		if c.LOD < len(c.Meshes) && c.Meshes[c.LOD] != nil {
			tris += len(c.Meshes[c.LOD].Indices) / 3
		}
	}
	return len(t.Chunks), perLOD, tris
}

// NoiseTerrain 简易值噪声地形（可复现，做原型够用）。
//
// seed 相同则结果相同；octaves 建议 3~5。
func NoiseTerrain(hm *Heightmap, seed uint32, scale, amplitude float32, octaves int) {
	if hm == nil {
		return
	}
	if octaves < 1 {
		octaves = 1
	}
	if scale <= 0 {
		scale = 1
	}
	hm.Fill(func(x, y int, wx, wz float32) float32 {
		var sum, amp, freq float32 = 0, 1, 1 / scale
		var norm float32 = 0
		for o := 0; o < octaves; o++ {
			sum += valueNoise2(wx*freq, wz*freq, seed+uint32(o)*7919) * amp
			norm += amp
			amp *= 0.5
			freq *= 2
		}
		if norm > 0 {
			sum /= norm
		}
		return sum * amplitude
	})
}

// valueNoise2 哈希值噪声（0..1）。
func valueNoise2(x, z float32, seed uint32) float32 {
	xi := int(math.Floor(float64(x)))
	zi := int(math.Floor(float64(z)))
	tx := x - float32(xi)
	tz := z - float32(zi)
	// 平滑插值
	tx = tx * tx * (3 - 2*tx)
	tz = tz * tz * (3 - 2*tz)
	v00 := hash2f(xi, zi, seed)
	v10 := hash2f(xi+1, zi, seed)
	v01 := hash2f(xi, zi+1, seed)
	v11 := hash2f(xi+1, zi+1, seed)
	top := lerpf(v00, v10, tx)
	bot := lerpf(v01, v11, tx)
	return lerpf(top, bot, tz)
}

// hash2f 2D 哈希到 0..1。
func hash2f(x, z int, seed uint32) float32 {
	h := uint32(x)*374761393 + uint32(z)*668265263 + seed*2246822519
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float32(h&0xffffff) / float32(0xffffff)
}
