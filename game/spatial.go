// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// 空间分区与可见性剔除：
//   - Octree：静态/半静态场景的八叉树分区（范围查询、视锥查询）
//   - OcclusionCuller：软件遮挡剔除（低分辨率深度缓冲 + 屏幕矩形保守判断）
//   - CollectVisible：视锥剔除，收集可见节点
//
// 大场景（几千个对象）里，先做空间查询与视锥/遮挡剔除，再提交绘制，
// 比逐个对象走完整管线快一个数量级。

// aabbIntersects 两个 AABB 是否相交。
func aabbIntersects(a, b AABB) bool {
	return a.Min.X <= b.Max.X && a.Max.X >= b.Min.X &&
		a.Min.Y <= b.Max.Y && a.Max.Y >= b.Min.Y &&
		a.Min.Z <= b.Max.Z && a.Max.Z >= b.Min.Z
}

// aabbContains 完全包含。
func aabbContains(outer, inner AABB) bool {
	return outer.Min.X <= inner.Min.X && outer.Max.X >= inner.Max.X &&
		outer.Min.Y <= inner.Min.Y && outer.Max.Y >= inner.Max.Y &&
		outer.Min.Z <= inner.Min.Z && outer.Max.Z >= inner.Max.Z
}

// aabbUnion 合并。
func aabbUnion(a, b AABB) AABB {
	return AABB{
		Min: V3(minf3(a.Min.X, b.Min.X, b.Min.X), minf3(a.Min.Y, b.Min.Y, b.Min.Y), minf3(a.Min.Z, b.Min.Z, b.Min.Z)),
		Max: V3(maxf3(a.Max.X, b.Max.X, b.Max.X), maxf3(a.Max.Y, b.Max.Y, b.Max.Y), maxf3(a.Max.Z, b.Max.Z, b.Max.Z)),
	}
}

// OctItem 八叉树条目。
type OctItem struct {
	// ID 外部对象标识（通常为 Scene 节点索引或 ECS 实体索引）。
	ID int
	// Box 世界空间包围盒。
	Box AABB
}

// OctNode 八叉树节点。
type OctNode struct {
	Box      AABB
	Items    []OctItem
	Children []*OctNode
}

// Octree 八叉树。
type Octree struct {
	// Root 根节点。
	Root *OctNode
	// MaxDepth 最大深度（默认 6）。
	MaxDepth int
	// Capacity 单节点条目上限（超过则分裂，默认 8）。
	Capacity int
	// Count 条目总数。
	Count int
}

// NewOctree 创建八叉树。
func NewOctree(bounds AABB, maxDepth, capacity int) *Octree {
	if maxDepth <= 0 {
		maxDepth = 6
	}
	if capacity <= 0 {
		capacity = 8
	}
	return &Octree{
		Root:     &OctNode{Box: bounds},
		MaxDepth: maxDepth,
		Capacity: capacity,
	}
}

// Clear 清空。
func (o *Octree) Clear() {
	if o == nil {
		return
	}
	o.Root.Children = nil
	o.Root.Items = nil
	o.Count = 0
}

// Insert 插入条目（超出根包围盒时自动扩展根）。
func (o *Octree) Insert(box AABB, id int) {
	if o == nil {
		return
	}
	if !aabbContains(o.Root.Box, box) {
		for !aabbContains(o.Root.Box, box) {
			o.grow(box)
		}
	}
	o.insert(o.Root, OctItem{ID: id, Box: box}, 0)
	o.Count++
}

// grow 扩大根节点一倍并下推旧子节点。
func (o *Octree) grow(box AABB) {
	old := o.Root
	size := old.Box.Size()
	b := old.Box
	maxSize := size
	for i := 0; i < 3; i++ {
		v := [3]float32{box.Size().X, box.Size().Y, box.Size().Z}[i]
		cur := [3]float32{maxSize.X, maxSize.Y, maxSize.Z}[i]
		if v > cur {
			cur = v * 2
		}
		switch i {
		case 0:
			maxSize.X = cur
		case 1:
			maxSize.Y = cur
		case 2:
			maxSize.Z = cur
		}
	}
	center := b.Center()
	newBox := AABB{
		Min: V3(center.X-maxSize.X, center.Y-maxSize.Y, center.Z-maxSize.Z),
		Max: V3(center.X+maxSize.X, center.Y+maxSize.Y, center.Z+maxSize.Z),
	}
	o.Root = &OctNode{Box: newBox, Children: []*OctNode{old}}
}

func (o *Octree) insert(n *OctNode, it OctItem, depth int) {
	if len(n.Children) == 0 {
		n.Items = append(n.Items, it)
		if len(n.Items) > o.Capacity && depth < o.MaxDepth {
			o.split(n, depth)
		}
		return
	}
	for _, c := range n.Children {
		if aabbIntersects(c.Box, it.Box) {
			o.insert(c, it, depth+1)
			return
		}
	}
	n.Items = append(n.Items, it)
}

func (o *Octree) split(n *OctNode, depth int) {
	c := n.Box.Center()
	min, max := n.Box.Min, n.Box.Max
	boxes := [8]AABB{}
	idx := 0
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			for k := 0; k < 2; k++ {
				b := AABB{Min: min, Max: max}
				if i == 1 {
					b.Min.X, b.Max.X = c.X, max.X
				} else {
					b.Min.X, b.Max.X = min.X, c.X
				}
				if j == 1 {
					b.Min.Y, b.Max.Y = c.Y, max.Y
				} else {
					b.Min.Y, b.Max.Y = min.Y, c.Y
				}
				if k == 1 {
					b.Min.Z, b.Max.Z = c.Z, max.Z
				} else {
					b.Min.Z, b.Max.Z = min.Z, c.Z
				}
				boxes[idx] = b
				idx++
			}
		}
	}
	n.Children = make([]*OctNode, 0, 8)
	for _, b := range boxes {
		n.Children = append(n.Children, &OctNode{Box: b})
	}
	items := n.Items
	n.Items = nil
	for _, it := range items {
		placed := false
		for _, ch := range n.Children {
			if aabbContains(ch.Box, it.Box) {
				o.insert(ch, it, depth+1)
				placed = true
				break
			}
		}
		if !placed {
			n.Items = append(n.Items, it)
		}
	}
}

// QueryFrustum 收集与视锥相交的条目 ID。
func (o *Octree) QueryFrustum(f Frustum, out []int) []int {
	if o == nil {
		return out
	}
	var walk func(n *OctNode)
	walk = func(n *OctNode) {
		if !f.ContainsAABB(n.Box) {
			return
		}
		for i := range n.Items {
			if f.ContainsAABB(n.Items[i].Box) {
				out = append(out, n.Items[i].ID)
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(o.Root)
	return out
}

// QueryAABB 收集与给定盒相交的条目 ID。
func (o *Octree) QueryAABB(q AABB, out []int) []int {
	if o == nil {
		return out
	}
	var walk func(n *OctNode)
	walk = func(n *OctNode) {
		if !aabbIntersects(n.Box, q) {
			return
		}
		for i := range n.Items {
			if aabbIntersects(n.Items[i].Box, q) {
				out = append(out, n.Items[i].ID)
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(o.Root)
	return out
}

// QuerySphere 收集与球体相交的条目 ID。
func (o *Octree) QuerySphere(center Vec3, radius float32, out []int) []int {
	q := AABB{
		Min: V3(center.X-radius, center.Y-radius, center.Z-radius),
		Max: V3(center.X+radius, center.Y+radius, center.Z+radius),
	}
	ids := o.QueryAABB(q, out)
	// 精确球测试
	r2 := radius * radius
	kept := ids[:0]
	var walk func(n *OctNode)
	walk = func(n *OctNode) {
		for i := range n.Items {
			it := &n.Items[i]
			c := it.Box.Center()
			d := c.Sub(center)
			if d.Dot(d) <= r2 {
				kept = append(kept, it.ID)
			}
		}
		for _, ch := range n.Children {
			walk(ch)
		}
	}
	if len(ids) > 0 {
		kept = kept[:0]
		walk(o.Root)
	}
	return kept
}

// projectBoxCorners 把 AABB 8 个角投影到屏幕，返回矩形与深度范围。
// ok=false 表示有角落在相机后方或 w 非正，无法安全剔除。
func projectBoxCorners(mvp Mat4, b AABB, vpW, vpH int) (minX, minY, maxX, maxY, zmin, zmax float32, ok bool) {
	minX, minY = 1e30, 1e30
	maxX, maxY = -1e30, -1e30
	zmin, zmax = 1, 0
	for i := 0; i < 8; i++ {
		p := Vec3{
			X: b.Min.X + (b.Max.X-b.Min.X)*float32(i&1),
			Y: b.Min.Y + (b.Max.Y-b.Min.Y)*float32((i>>1)&1),
			Z: b.Min.Z + (b.Max.Z-b.Min.Z)*float32((i>>2)&1),
		}
		c := mvp.TransformVec4(Vec4{p.X, p.Y, p.Z, 1})
		if c.W <= 1e-6 {
			return 0, 0, 0, 0, 0, 0, false
		}
		inv := 1 / c.W
		sx := (c.X*inv*0.5 + 0.5) * float32(vpW)
		sy := (0.5 - c.Y*inv*0.5) * float32(vpH)
		z := clampf(c.Z*inv*0.5+0.5, 0, 1)
		minX = minf3(minX, sx, sx)
		maxX = maxf3(maxX, sx, sx)
		minY = minf3(minY, sy, sy)
		maxY = maxf3(maxY, sy, sy)
		if z < zmin {
			zmin = z
		}
		if z > zmax {
			zmax = z
		}
	}
	return minX, minY, maxX, maxY, zmin, zmax, true
}

// OcclusionCuller 软件遮挡剔除器。
//
// 用法：每帧 Begin → 对大型遮挡物（建筑 / 地形块）调用 AddOccluder →
// 其余对象用 Visible 判断是否被完全遮挡。
type OcclusionCuller struct {
	// W, H 深度缓冲分辨率（建议屏幕 / 4）。
	W, H int
	depth []float32
	// Tested / Culled 本帧统计。
	Tested, Culled int
	occluders      int
}

// NewOcclusionCuller 创建遮挡剔除器（downsample 建议 4）。
func NewOcclusionCuller(w, h, downsample int) *OcclusionCuller {
	if downsample <= 0 {
		downsample = 4
	}
	o := &OcclusionCuller{W: w / downsample, H: h / downsample}
	if o.W < 1 {
		o.W = 1
	}
	if o.H < 1 {
		o.H = 1
	}
	o.depth = make([]float32, o.W*o.H)
	for i := range o.depth {
		o.depth[i] = 1
	}
	return o
}

// Resize 改变屏幕尺寸（内部分辨率随之重建）。
func (o *OcclusionCuller) Resize(w, h, downsample int) {
	if downsample <= 0 {
		downsample = 4
	}
	nw, nh := w/downsample, h/downsample
	if nw == o.W && nh == o.H {
		return
	}
	o.W, o.H = maxi(nw, 1), maxi(nh, 1)
	o.depth = make([]float32, o.W*o.H)
	for i := range o.depth {
		o.depth[i] = 1
	}
}

// Begin 开始新一帧（清空遮挡深度）。
func (o *OcclusionCuller) Begin() {
	if o == nil {
		return
	}
	for i := range o.depth {
		o.depth[i] = 1
	}
	o.Tested, o.Culled, o.occluders = 0, 0, 0
}

// AddOccluder 记录遮挡物：把它的屏幕矩形按「最远深度」写入深度缓冲
// （保守策略，避免错误剔除）。
func (o *OcclusionCuller) AddOccluder(mvp Mat4, b AABB, vpW, vpH int) {
	if o == nil {
		return
	}
	minX, minY, maxX, maxY, _, zmax, ok := projectBoxCorners(mvp, b, vpW, vpH)
	if !ok {
		return
	}
	// 屏幕 → 内部缓冲坐标
	sx := float32(o.W) / float32(vpW)
	sy := float32(o.H) / float32(vpH)
	x0 := clampi(int(minX*sx), 0, o.W-1)
	x1 := clampi(int(maxX*sx), 0, o.W-1)
	y0 := clampi(int(minY*sy), 0, o.H-1)
	y1 := clampi(int(maxY*sy), 0, o.H-1)
	for y := y0; y <= y1; y++ {
		row := y * o.W
		for x := x0; x <= x1; x++ {
			if zmax < o.depth[row+x] {
				o.depth[row+x] = zmax
			}
		}
	}
	o.occluders++
}

// Visible 判断对象是否可能可见（返回 false 表示可安全剔除）。
func (o *OcclusionCuller) Visible(mvp Mat4, b AABB, vpW, vpH int) bool {
	if o == nil {
		return true
	}
	minX, minY, maxX, maxY, zmin, _, ok := projectBoxCorners(mvp, b, vpW, vpH)
	if !ok {
		return true
	}
	o.Tested++
	if maxX < 0 || maxY < 0 || minX > float32(vpW) || minY > float32(vpH) {
		o.Culled++
		return false
	}
	sx := float32(o.W) / float32(vpW)
	sy := float32(o.H) / float32(vpH)
	x0 := clampi(int(minX*sx), 0, o.W-1)
	x1 := clampi(int(maxX*sx), 0, o.W-1)
	y0 := clampi(int(minY*sy), 0, o.H-1)
	y1 := clampi(int(maxY*sy), 0, o.H-1)
	for y := y0; y <= y1; y++ {
		row := y * o.W
		for x := x0; x <= x1; x++ {
			// 有任何一个采样点比对象最近深度更远 → 可能可见
			if o.depth[row+x] >= zmin {
				return true
			}
		}
	}
	o.Culled++
	return false
}

// Occluders 本帧记录的遮挡物数量。
func (o *OcclusionCuller) Occluders() int {
	if o == nil {
		return 0
	}
	return o.occluders
}

// CollectVisible 用视锥剔除收集可见节点（跳过无网格节点）。
//
// 返回的节点已按到相机距离从近到远排序，便于配合遮挡剔除。
func CollectVisible(sc *Scene, cam *Camera, aspect float32) []*Node {
	if sc == nil || cam == nil || sc.Root == nil {
		return nil
	}
	sc.Root.BuildWorld(Identity())
	f := cam.Frustum(aspect)
	out := make([]*Node, 0, 128)
	sc.Root.Each(func(n *Node) {
		if n.Mesh == nil && len(n.LODs) == 0 {
			return
		}
		box := n.WorldBounds()
		if !f.ContainsAABB(box) {
			return
		}
		out = append(out, n)
	})
	return out
}

// WorldBounds 节点的世界空间包围盒（网格局部包围盒经世界矩阵变换后的 AABB）。
// 要求已经调用过 BuildWorld（或由 CollectVisible 内部完成）。
func (n *Node) WorldBounds() AABB {
	if n == nil {
		return AABB{}
	}
	m := n.world
	if n.Mesh == nil || len(n.Mesh.Positions) == 0 {
		p := m.TransformPoint(V3(0, 0, 0))
		return AABB{Min: p, Max: p}
	}
	b := n.Mesh.Bounds
	minX, minY, minZ := float32(1e30), float32(1e30), float32(1e30)
	maxX, maxY, maxZ := float32(-1e30), float32(-1e30), float32(-1e30)
	for i := 0; i < 8; i++ {
		p := Vec3{
			X: b.Min.X + (b.Max.X-b.Min.X)*float32(i&1),
			Y: b.Min.Y + (b.Max.Y-b.Min.Y)*float32((i>>1)&1),
			Z: b.Min.Z + (b.Max.Z-b.Min.Z)*float32((i>>2)&1),
		}
		w := m.TransformPoint(p)
		minX = minf3(minX, w.X, w.X)
		maxX = maxf3(maxX, w.X, w.X)
		minY = minf3(minY, w.Y, w.Y)
		maxY = maxf3(maxY, w.Y, w.Y)
		minZ = minf3(minZ, w.Z, w.Z)
		maxZ = maxf3(maxZ, w.Z, w.Z)
	}
	return AABB{Min: V3(minX, minY, minZ), Max: V3(maxX, maxY, maxZ)}
}

// maxi 整数最大值。
func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}
