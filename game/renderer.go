// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"image"
	"math"
	"runtime"
	"sort"
	"sync"
	"time"
	"unsafe"
)

// FrameStats 单帧统计（性能定位与测试用）。
//
// 注意：13 个位域数据只在 Frame 返回后有效，下一帧开始时重置。
type FrameStats struct {
	// Triangles 提交到光栅化的三角形数。
	Triangles int
	// Culled 被视锥/背面剔除的三角形数。
	Culled int
	// Pixels 通过深度测试并写色的像素数。
	Pixels int
	// Objects 参与渲染的网格节点数。
	Objects int
	// Sprites 2D 叠加层绘制数。
	Sprites int
	// Billboards 实际绘制的世界空间广告牌数（被遮挡 / 相机背后的不计）。
	Billboards int
	// LODSwitches 本帧使用非默认 LOD 层的次数。
	LODSwitches int
	// DrawCalls 本帧提交次数（每个网格节点 / 每个实例批次算 1 次）。
	DrawCalls int
	// Batches 实例批次数量。
	Batches int
	// Instances 实例总数。
	Instances int
	// TrianglesSaved 合批相比逐实例提交省下的三角形提交量（顶点重建量）。
	TrianglesSaved int
	// ShadowTris 本帧阴影图光栅化的三角形数（单图或 CSM 各级之和）。
	ShadowTris int
	// Ms 本帧总耗时（毫秒）。
	Ms float64
}

// Sprite2D 2D 叠加层（HUD / 精灵 / 瓦片地图），在 3D 之后绘制到同一帧。
type Sprite2D interface {
	DrawSprites(dst *image.RGBA, w, h int)
}

// Renderer 软件光栅化 3D 渲染器（非并发安全，应在 UI 线程使用）。
type Renderer struct {
	// W, H 内部渲染分辨率（与控件尺寸无关；建议 ≤ 960×540）。
	W, H int
	// Threads 光栅化并行度，0 = GOMAXPROCS。
	Threads int
	// Supersample 超采样倍数（1=关闭，2=2×2，4=4×4；>1 时先高分辨率光栅化再盒式降采样）。
	// 开销约按 ss² 增长，建议只在 UI 层需要抗锯齿时开到 2。
	Supersample int
	// BgColor 背景色（场景 Background 的 A=0 时用它）。
	BgColor RGBA
	// Shadow 方向光实时阴影（nil = 关闭）。构建开销与场景三角形数相关，
	// 静态场景把 ShadowMap.Once 设为 true 可只构建一次。
	Shadow *ShadowMap
	// Cascades 级联阴影（nil = 关闭）。非空且 On 时优先于 Shadow：
	// 按相机视锥分级，近处阴影更精细，适合大场景。见 csm.go。
	Cascades *CSMShadow
	// Post 屏幕空间后处理链（nil = 关闭），作用于 3D 画面（精灵之前）。
	Post *PostFX

	fb     *frameBuf
	img    *image.RGBA
	stats  FrameStats
	sprite Sprite2D
	tris   []tri
	transp []tri
	bands  [][]tri
}

// NewRenderer 创建渲染器（默认背景深灰）。
func NewRenderer(w, h int) *Renderer {
	r := &Renderer{W: w, H: h, BgColor: RGB(18, 20, 26)}
	r.ensure()
	return r
}

// Resize 调整内部渲染分辨率（尺寸不变时无开销）。
func (r *Renderer) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if r.W == w && r.H == h {
		return
	}
	r.W, r.H = w, h
	r.ensure()
}

// Stats 上一帧统计。
func (r *Renderer) Stats() FrameStats { return r.stats }

// SetSprite2D 设置 2D 叠加层（传 nil 关闭）。
func (r *Renderer) SetSprite2D(s Sprite2D) { r.sprite = s }

// Image 返回内部帧缓冲（尺寸 W×H，RGBA 直通 alpha）。
func (r *Renderer) Image() *image.RGBA {
	r.ensure()
	return r.img
}

func (r *Renderer) ensure() {
	if r.W < 1 {
		r.W = 1
	}
	if r.H < 1 {
		r.H = 1
	}
	if r.fb == nil {
		r.fb = newFrameBuf(r.W*r.ss(), r.H*r.ss())
	} else if r.fb.w != r.W*r.ss() || r.fb.h != r.H*r.ss() {
		r.fb.resize(r.W*r.ss(), r.H*r.ss())
	}
	if r.img == nil || r.img.Rect.Dx() != r.W || r.img.Rect.Dy() != r.H {
		r.img = image.NewRGBA(image.Rect(0, 0, r.W, r.H))
	}
}

// ss 返回生效的超采样倍数（限 1~4，非 2/4 的值退化为 1）。
func (r *Renderer) ss() int {
	switch r.Supersample {
	case 2:
		return 2
	case 3:
		return 3
	case 4:
		return 4
	}
	return 1
}

// Frame 渲染一帧并返回 RGBA 图像（缓冲区复用，下次调用会被覆写）。
func (r *Renderer) Frame(sc *Scene) *image.RGBA {
	t0 := time.Now()
	r.ensure()
	st := FrameStats{}
	r.stats = st

	bg := r.BgColor
	if sc != nil && sc.Background.A != 0 {
		bg = sc.Background
	}
	r.clearFrame(bg)

	aspect := float32(r.W) / float32(r.H)
	if sc != nil && sc.Camera != nil && sc.Sky.Enabled() {
		renderSky(r.fb, sc.Camera, aspect, sc.Sky)
	}

	r.tris = r.tris[:0]
	r.transp = r.transp[:0]

	if sc != nil && sc.Camera != nil && sc.Root != nil {
		sc.BuildWorld()
		vp := sc.Camera.ViewProj(aspect)
		fr := sc.Camera.Frustum(aspect)
		r.collect(sc.Root, vp, fr, sc.Camera.Pos, &st)

		ctx := shadeCtx{
			lights:  sc.Lights,
			ambient: sc.Ambient,
			fog:     sc.Fog,
			camPos:  sc.Camera.Pos,
		}
		// Sky 在树内就填，供 PBR 环境光（IBL 近似）采样
	if sc.Sky.Enabled() {
		ctx.sky = &sc.Sky
	}
	if r.Cascades != nil && r.Cascades.On {
			r.Cascades.Aspect = aspect
			r.Cascades.Build(sc)
			if r.Cascades.Valid() {
				ctx.cascades = r.Cascades
				st.ShadowTris, _ = r.Cascades.Stats()
			}
		}
		if ctx.cascades == nil && r.Shadow != nil && r.Shadow.On {
			r.Shadow.Build(sc)
			if r.Shadow.Valid() {
				ctx.shadow = r.Shadow
				st.ShadowTris = r.Shadow.Triangles()
			}
		}

		r.rasterPass(&ctx, &st)

		if len(r.transp) > 0 {
			// 透明三角形按平均 1/w 升序（远→近）排序，保证混合结果稳定。
			sort.SliceStable(r.transp, func(i, j int) bool {
				return r.transp[i].depthKey < r.transp[j].depthKey
			})
			for i := range r.transp {
				t := &r.transp[i]
				if t.mat.Wireframe {
					rasterWireframe(r.fb, t)
					continue
				}
				rasterTriangle(r.fb, t, &ctx, 0, r.fb.h, &st)
			}
		}
	}

	r.blit(&st)

	if r.Post != nil && r.Post.On {
		r.Post.Apply(r.img)
	}

	if sc != nil && sc.Billboards.Len() > 0 {
		st.Billboards = r.drawBillboards(sc)
	}

	if r.sprite != nil {
		r.sprite.DrawSprites(r.img, r.W, r.H)
		if c, ok := r.sprite.(interface{ SpriteCount() int }); ok {
			st.Sprites = c.SpriteCount()
		}
	}

	st.Ms = float64(time.Since(t0).Microseconds()) / 1000
	r.stats = st
	return r.img
}

// rasterPass 并行光栅化不透明三角形（按屏幕行分带，无锁竞争）。
func (r *Renderer) rasterPass(ctx *shadeCtx, st *FrameStats) {
	if len(r.tris) == 0 {
		return
	}
	threads := r.Threads
	if threads <= 0 {
		threads = runtime.GOMAXPROCS(0)
	}
	if threads > 16 {
		threads = 16
	}
	if threads > r.fb.h {
		threads = r.fb.h
	}
	if threads <= 1 {
		for i := range r.tris {
			t := &r.tris[i]
			if t.mat.Wireframe {
				rasterWireframe(r.fb, t)
				continue
			}
			rasterTriangle(r.fb, t, ctx, 0, r.fb.h, st)
		}
		return
	}

	parts := make([]FrameStats, threads)
	var wg sync.WaitGroup
	band := (r.fb.h + threads - 1) / threads

	// 先按屏幕 Y 范围把三角形分到各带，避免每个带都重扫全部三角形（顶点设置开销）。
	if cap(r.bands) < threads {
		r.bands = make([][]tri, threads)
	}
	r.bands = r.bands[:threads]
	for i := range r.bands {
		r.bands[i] = r.bands[i][:0]
	}
	for i := range r.tris {
		t := &r.tris[i]
		minYf := minf(t.a.y, minf(t.b.y, t.c.y))
		maxYf := maxf(t.a.y, maxf(t.b.y, t.c.y))
		b0 := int(math.Floor(float64(minYf))) / band
		b1 := int(math.Floor(float64(maxYf))) / band
		if b0 < 0 {
			b0 = 0
		}
		if b1 >= threads {
			b1 = threads - 1
		}
		for b := b0; b <= b1; b++ {
			r.bands[b] = append(r.bands[b], *t)
		}
	}

	for k := 0; k < threads; k++ {
		y0 := k * band
		y1 := y0 + band
		if y1 > r.fb.h {
			y1 = r.fb.h
		}
		if y0 >= y1 {
			continue
		}
		wg.Add(1)
		go func(k, y0, y1 int) {
			defer wg.Done()
			ps := &parts[k]
			bucket := r.bands[k]
			for i := range bucket {
				t := &bucket[i]
				if t.mat.Wireframe {
					// 线框按带段近似：只在前半带才画，避免重复
					if k == 0 {
						rasterWireframe(r.fb, t)
					}
					continue
				}
				rasterTriangle(r.fb, t, ctx, y0, y1, ps)
			}
		}(k, y0, y1)
	}
	wg.Wait()
	for i := range parts {
		st.Pixels += parts[i].Pixels
		st.Triangles += parts[i].Triangles
		st.Culled += parts[i].Culled
	}
}

// collect 遍历场景树，把三角形变换/裁剪/投影后放入待光栅化列表。
func (r *Renderer) collect(n *Node, vp Mat4, fr Frustum, camPos Vec3, st *FrameStats) {
	if n == nil || !n.Visible {
		return
	}
	for _, c := range n.Children {
		r.collect(c, vp, fr, camPos, st)
	}
	if n.Instances != nil && n.Instances.Mesh != nil {
		r.collectInstances(n, vp, fr, st)
		return
	}
	if n.Mesh == nil || len(n.Mesh.Positions) == 0 {
		return
	}
	m := n.Mesh
	w := n.world
	if n.Skinned != nil && n.Skinned.Mesh != nil {
		var pose []Mat4
		if n.SkinAnim != nil {
			pose = n.SkinAnim.Pose()
		} else {
			pose = n.Skinned.BindPose()
		}
		if rm := n.Skinned.RenderMesh(pose); rm != nil && len(rm.Positions) > 0 {
			m = rm
		}
	}
	if len(n.LODs) > 0 {
		center := w.TransformPoint(m.Bounds.Center())
		if lod := n.PickLOD(camPos.Dist(center)); lod != nil && len(lod.Positions) > 0 {
			if lod != m {
				st.LODSwitches++
			}
			m = lod
		}
	}

	// 世界包围盒 + 视锥剔除（8 个角点：位 0/1/2 分别选 min/max）
	wb := AABB{}
	first := true
	for i := 0; i < 8; i++ {
		corner := Vec3{
			X: pickFlag(m.Bounds.Min.X, m.Bounds.Max.X, i&1 == 1),
			Y: pickFlag(m.Bounds.Min.Y, m.Bounds.Max.Y, (i>>1)&1 == 1),
			Z: pickFlag(m.Bounds.Min.Z, m.Bounds.Max.Z, (i>>2)&1 == 1),
		}
		p := w.TransformPoint(corner)
		if first {
			wb = AABB{Min: p, Max: p}
			first = false
			continue
		}
		wb.Min = wb.Min.Min(p)
		wb.Max = wb.Max.Max(p)
	}
	if !first && !fr.ContainsAABB(wb) {
		st.Culled += m.TriCount()
		return
	}

	mat := n.Material
	if mat == nil {
		mat = DefaultMaterial()
	}
	st.Objects++
	st.DrawCalls++
	r.emitMesh(w, m, mat, vp, st, RGB(255, 255, 255))
}

// emitMesh 把网格按世界矩阵 w 提交为待光栅化三角形（tint 会乘到顶点色上）。
//
// 实例批次里每个实例走一次；普通节点整帧走一次。
func (r *Renderer) emitMesh(w Mat4, m *Mesh, mat *Material, vp Mat4, st *FrameStats, tint RGBA) {
	nm := w.NormalMatrix()
	white := RGB(255, 255, 255)
	if tint.A == 0 {
		tint = white
	}
	idx := m.Indices
	for i := 0; i+2 < len(idx); i += 3 {
		ia, ib, ic := int(idx[i]), int(idx[i+1]), int(idx[i+2])
		if ia >= len(m.Positions) || ib >= len(m.Positions) || ic >= len(m.Positions) {
			continue
		}
		v := [3]vtx{}
		ids := [3]int{ia, ib, ic}
		for k, vi := range ids {
			p := m.Positions[vi]
			var nr Vec3
			if vi < len(m.Normals) {
				nr = nm.TransformDir(m.Normals[vi])
			}
			var uv Vec2
			if vi < len(m.UVs) {
				uv = m.UVs[vi]
			}
			vc := white
			if vi < len(m.Colors) {
				vc = m.Colors[vi]
			}
			if tint != white {
				vc = vc.Mul(tint)
			}
			wp := w.TransformPoint(p)
			v[k] = vtx{clip: vp.TransformVec4(V4(wp.X, wp.Y, wp.Z, 1)), world: wp, normal: nr, uv: uv, color: vc}
		}

		// 世界空间面法线（顶点法线缺失时兜底）
		fn := v[1].world.Sub(v[0].world).Cross(v[2].world.Sub(v[0].world)).Normalize()

		poly := clipNear(v[:])
		if len(poly) < 3 {
			st.Culled++
			continue
		}
		for k := 1; k+1 < len(poly); k++ {
			t := tri{
				a:     project(poly[0], r.fb.w, r.fb.h),
				b:     project(poly[k], r.fb.w, r.fb.h),
				c:     project(poly[k+1], r.fb.w, r.fb.h),
				mat:   mat,
				faceN: fn,
			}
			if !orientTriangle(&t) {
				st.Culled++
				continue
			}
			st.Triangles++
			// AlphaOpaque / AlphaCutout 走不透明通道（写深度），
			// Cutout 的逐像素裁剪在 rasterTriangle 里做，避免逐三角形建两次排序项。
			if mat.AlphaMode == AlphaOpaque || mat.AlphaMode == AlphaCutout {
				r.tris = append(r.tris, t)
				continue
			}
			if mat.Alpha < 1 || !mat.BaseColor.Opaque() {
				t.transparent = true
				// 排序 key 取最远顶点（最小 invW）：painter 算法从远到近画才不出错
				minInvW := t.a.invW
				if t.b.invW < minInvW {
					minInvW = t.b.invW
				}
				if t.c.invW < minInvW {
					minInvW = t.c.invW
				}
				t.depthKey = minInvW
				r.transp = append(r.transp, t)
				continue
			}
			r.tris = append(r.tris, t)
		}
	}
}

// collectInstances 提交实例批次的全部实例（合批：整批只算一次提交）。
func (r *Renderer) collectInstances(n *Node, vp Mat4, fr Frustum, st *FrameStats) {
	b := n.Instances
	if b == nil || b.Mesh == nil || len(b.Data) == 0 {
		return
	}
	mat := b.Material
	if mat == nil {
		mat = n.Material
	}
	if mat == nil {
		mat = DefaultMaterial()
	}
	st.Batches++
	st.Instances += len(b.Data)
	st.Objects++
	st.DrawCalls++
	tris := b.Mesh.TriCount()
	base := n.world
	for i := range b.Data {
		w := base.Mul(b.MatrixOf(i))
		if !fr.ContainsAABB(worldBounds(b.Mesh, w)) {
			st.Culled += tris
			continue
		}
		r.emitMesh(w, b.Mesh, mat, vp, st, b.Data[i].Tint)
	}
	if len(b.Data) > 1 {
		// 相比「每个实例建一个节点逐个提交」，省下 (n-1) 次顶点重建与提交
		st.TrianglesSaved += (len(b.Data) - 1) * tris
	}
}

// worldBounds 网格在世界矩阵下的世界包围盒。
func worldBounds(m *Mesh, w Mat4) AABB {
	b := m.Bounds
	out := AABB{}
	first := true
	for i := 0; i < 8; i++ {
		p := V3(
			pickFlag(b.Min.X, b.Max.X, i&1 == 1),
			pickFlag(b.Min.Y, b.Max.Y, (i>>1)&1 == 1),
			pickFlag(b.Min.Z, b.Max.Z, (i>>2)&1 == 1),
		)
		wp := w.TransformPoint(p)
		if first {
			out = AABB{Min: wp, Max: wp}
			first = false
			continue
		}
		out.Min = out.Min.Min(wp)
		out.Max = out.Max.Max(wp)
	}
	return out
}

// threadCount 实际使用的并行了数（0 = GOMAXPROCS，上限 16）。
func (r *Renderer) threadCount() int {
	t := r.Threads
	if t <= 0 {
		t = runtime.GOMAXPROCS(0)
	}
	if t > 16 {
		t = 16
	}
	if t < 1 {
		t = 1
	}
	return t
}

// clearFrame 清屏（按行分带并行）。
func (r *Renderer) clearFrame(c RGBA) {
	threads := r.threadCount()
	h := r.fb.h
	if threads <= 1 || h < threads*4 {
		r.fb.clear(c)
		return
	}
	band := (h + threads - 1) / threads
	var wg sync.WaitGroup
	for k := 0; k < threads; k++ {
		y0 := k * band
		y1 := y0 + band
		if y1 > h {
			y1 = h
		}
		if y0 >= y1 {
			continue
		}
		wg.Add(1)
		go func(y0, y1 int) {
			defer wg.Done()
			r.fb.clearRange(c, y0, y1)
		}(y0, y1)
	}
	wg.Wait()
}

// blit 把颜色缓冲拷到 image.RGBA。
// fb.color 已按 R,G,B,A 字节序预打包，小端机上一个 uint32 就是四个像素字节，直接 memmove 即可。
func (r *Renderer) blit(st *FrameStats) {
	w, h := r.W, r.H
	if r.img == nil || r.img.Rect.Dx() != w || r.img.Rect.Dy() != h {
		r.img = image.NewRGBA(image.Rect(0, 0, w, h))
	}
	n := w * h
	if n <= 0 || len(r.fb.color) < n {
		return
	}
	if r.ss() > 1 {
		r.downsample(r.ss())
		return
	}
	if littleEndian {
		dst := unsafe.Slice((*uint32)(unsafe.Pointer(&r.img.Pix[0])), n)
		copy(dst, r.fb.color[:n])
		return
	}
	// 大端机（极罕见）回退到逐字节转换
	pix := r.img.Pix
	for i := 0; i < n; i++ {
		c := unpackColor(r.fb.color[i])
		j := i * 4
		pix[j], pix[j+1], pix[j+2], pix[j+3] = c.R, c.G, c.B, c.A
	}
}

// downsample 把 ss×ss 超采样缓冲盒式滤波降采样到输出分辨率（按目标行并行）。
func (r *Renderer) downsample(ss int) {
	dst := r.img.Pix
	w, h := r.W, r.H
	fw := r.fb.w
	samples := int32(ss * ss)
	threads := r.threadCount()
	if threads > h {
		threads = h
	}
	if threads < 1 {
		threads = 1
	}
	band := (h + threads - 1) / threads
	var wg sync.WaitGroup
	for k := 0; k < threads; k++ {
		y0 := k * band
		y1 := y0 + band
		if y1 > h {
			y1 = h
		}
		if y0 >= y1 {
			continue
		}
		wg.Add(1)
		go func(y0, y1 int) {
			defer wg.Done()
			for y := y0; y < y1; y++ {
				for x := 0; x < w; x++ {
					var sr, sg, sb, sa int32
					for dy := 0; dy < ss; dy++ {
						row := (y*ss+dy)*fw + x*ss
						if row+ss > len(r.fb.color) {
							break
						}
						for dx := 0; dx < ss; dx++ {
							c := unpackColor(r.fb.color[row+dx])
							sr += int32(c.R)
							sg += int32(c.G)
							sb += int32(c.B)
							sa += int32(c.A)
						}
					}
					j := (y*w + x) * 4
					dst[j] = uint8((sr + samples/2) / samples)
					dst[j+1] = uint8((sg + samples/2) / samples)
					dst[j+2] = uint8((sb + samples/2) / samples)
					dst[j+3] = uint8((sa + samples/2) / samples)
				}
			}
		}(y0, y1)
	}
	wg.Wait()
}

// littleEndian 是否小端（决定 blit 能否直接用 memmove）。
var littleEndian = func() bool {
	v := uint16(1)
	return *(*byte)(unsafe.Pointer(&v)) == 1
}()
