// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// 软件 GPU 后端：tile-based 多线程光栅化。
//
// 管线阶段与真实 GPU 对齐：顶点着色（并行）→ 近平面裁剪 → 透视除法 →
// 背面剔除 → tile 分箱 → 片元着色（按 tile 并行，深度/颜色按 tile 独占，
// 无需加锁）→ 混合写色（MRT）。
//
// 特性：实例化、多渲染目标（最多 4）、深度测试、自定义 Go 着色器、
// 计算内核、纹理最近邻/双线性采样、统计计数。

// softTile 分箱 tile 边长（像素）。
const softTile = 32

// softMaxTargets 软件后端支持的颜色目标上限。
const softMaxTargets = 4

func init() {
	RegisterGPUBackend("soft", newGPUSoftBackend)
}

// gpuSoft 软件后端。
type gpuSoft struct {
	dev     *GPU
	workers int
}

func newGPUSoftBackend(g *GPU) (gpuBackend, error) {
	w := g.workers
	if w <= 0 {
		w = defaultGPUWorkers()
	}
	return &gpuSoft{dev: g, workers: w}, nil
}

func (s *gpuSoft) name() string { return "soft" }

func (s *gpuSoft) caps() GPUCaps {
	return GPUCaps{
		Backend:         "soft",
		Hardware:        false,
		MaxTextureSize:  4096,
		MaxColorTargets: softMaxTargets,
		MaxVertexAttrs:  12,
		Compute:         true,
		Instancing:      true,
		ShadowMap:       true,
		FloatTarget:     true,
		Threads:         s.workers,
		Readback:        true,
	}
}

func (s *gpuSoft) newTexture(t *GPUTexture) error {
	d := t.desc
	if d.Depth > 1 {
		return fmt.Errorf("game: soft 后端暂不支持 %d 层纹理", d.Depth)
	}
	levels := d.LevelCount()
	if d.Format.IsDepth() {
		t.depth = make([]float32, d.W*d.H)
		for i := range t.depth {
			t.depth[i] = 1
		}
		return nil
	}
	switch {
	case d.Format.IsFloat():
		t.pixF = make([][]float32, levels)
		for i := 0; i < levels; i++ {
			w, h := d.MipSize(i)
			t.pixF[i] = make([]float32, w*h*4)
			for j := 3; j < len(t.pixF[i]); j += 4 {
				t.pixF[i][j] = 1
			}
		}
	case d.Format == PixR8:
		t.pixR = make([][]uint8, levels)
		for i := 0; i < levels; i++ {
			w, h := d.MipSize(i)
			t.pixR[i] = make([]uint8, w*h)
		}
	default:
		t.pix8 = make([][]RGBA, levels)
		for i := 0; i < levels; i++ {
			w, h := d.MipSize(i)
			t.pix8[i] = make([]RGBA, w*h)
		}
	}
	return nil
}

func (s *gpuSoft) newBuffer(b *GPUBuffer) error {
	b.data = make([]byte, b.desc.Size)
	if len(b.desc.Data) > 0 {
		copy(b.data, b.desc.Data)
	}
	return nil
}

func (s *gpuSoft) compilePipeline(p *Pipeline) error {
	if p.desc.Program == nil || p.desc.Program.Vertex == nil {
		return errors.New("game: soft 后端需要 Go 顶点着色回调（Program.Vertex）")
	}
	return nil
}

func (s *gpuSoft) close() error { return nil }

func (s *gpuSoft) readPixels(fb *GPUFramebuffer, target int, dst []RGBA) error {
	if fb == nil || target < 0 || target >= len(fb.Colors) {
		return errors.New("game: 帧缓冲或颜色目标序号非法")
	}
	t := fb.Colors[target]
	n := t.desc.W * t.desc.H
	if len(dst) < n {
		return errors.New("game: 回读目标容量不足")
	}
	switch {
	case len(t.pix8) > 0 && len(t.pix8[0]) >= n:
		copy(dst, t.pix8[0][:n])
	case len(t.pixF) > 0 && len(t.pixF[0]) >= n*4:
		p := t.pixF[0]
		for i := 0; i < n; i++ {
			dst[i] = RGBA{
				floatToU8(p[i*4]),
				floatToU8(p[i*4+1]),
				floatToU8(p[i*4+2]),
				floatToU8(p[i*4+3]),
			}
		}
	case len(t.pixR) > 0 && len(t.pixR[0]) >= n:
		for i := 0; i < n; i++ {
			v := t.pixR[0][i]
			dst[i] = RGBA{v, v, v, 255}
		}
	case len(t.depth) >= n:
		for i := 0; i < n; i++ {
			v := floatToU8(t.depth[i])
			dst[i] = RGBA{v, v, v, 255}
		}
	default:
		return errors.New("game: 该纹理无可回读数据")
	}
	return nil
}

// softState 一次 Submit 内的渲染状态。
type softState struct {
	fb       *GPUFramebuffer
	pipe     *Pipeline
	vb       *GPUBuffer
	vlayout  *VertexLayout
	ib       *GPUBuffer
	inst     *GPUBuffer
	instLay  *VertexLayout
	samplers [8]*TextureSampler
	uniforms map[string][]float32
	mvp      Mat4
	model    Mat4
	viewport [4]int
	// 内置着色器常用的 uniform 解析结果（每个 draw 从 uniform 表刷新一次）。
	tint      RGBA
	lightDir  Vec3
	lightCol  RGBA
	ambient   float32
	specular  float32
	shininess float32
	// 每目标是否 RGBA8 快速路径
	fast [softMaxTargets]bool
	col8 [softMaxTargets][]RGBA
	colF [softMaxTargets][]float32
	depth []float32
	stats GPUStats
}

func (s *gpuSoft) submit(c *GPUCommands) error {
	if c.fb == nil {
		return errors.New("game: 提交前需要先 SetFramebuffer")
	}
	if c.fb.Depth == nil {
		return errors.New("game: 提交前需要深度附件（以便深度测试；纯 2D 可给 1x1 深度纹理）")
	}
	start := time.Now()
	st := &softState{
		fb:       c.fb,
		uniforms: map[string][]float32{},
		mvp:      Identity(),
		model:    Identity(),
		viewport: [4]int{0, 0, c.fb.W, c.fb.H},
		depth:    c.fb.Depth.depth,
	}
	for i := 0; i < len(c.fb.Colors); i++ {
		t := c.fb.Colors[i]
		switch {
		case len(t.pix8) > 0:
			st.fast[i] = true
			st.col8[i] = t.pix8[0]
		case len(t.pixF) > 0:
			st.colF[i] = t.pixF[0]
		}
	}
	for i := range c.ops {
		op := &c.ops[i]
		switch op.kind {
		case gpuCmdSetPipeline:
			st.pipe = op.pipeline
			if op.pipeline != nil {
				for k, v := range op.pipeline.uniforms {
					st.uniforms[k] = v
				}
				s.pullMatrices(st, op.pipeline.uniforms)
			}
		case gpuCmdSetVertex:
			st.vb, st.vlayout = op.buffer, op.layout
		case gpuCmdSetInstance:
			st.inst, st.instLay = op.buffer, op.layout
		case gpuCmdSetIndex:
			st.ib = op.buffer
		case gpuCmdSetUniform:
			st.uniforms[op.name] = op.floats
			if op.name == "u_mvp" || op.name == "u_model" {
				s.pullMatrices(st, map[string][]float32{op.name: op.floats})
			}
		case gpuCmdSetTexture:
			st.samplers[op.slot] = op.sampler
		case gpuCmdSetViewport:
			st.viewport = op.viewport
		case gpuCmdClearColor:
			s.clearColor(st, op.target, op.color)
			st.stats.Clears++
		case gpuCmdClearDepth:
			s.clearDepth(st, op.depth)
			st.stats.Clears++
		case gpuCmdCopyTexture:
			s.copyTexture(op.dst, op.src)
		case gpuCmdDraw:
			inst := op.instances
			if inst <= 0 {
				inst = 1
			}
			s.draw(st, op.indexed, op.count, inst)
		case gpuCmdDispatch:
			s.dispatch(st, op.kernel, op.groups)
		}
	}
	st.stats.SubmitMs = float64(time.Since(start).Microseconds()) / 1000
	s.dev.mu.Lock()
	g := s.dev.stats
	g.DrawCalls += st.stats.DrawCalls
	g.Instances += st.stats.Instances
	g.Vertices += st.stats.Vertices
	g.Triangles += st.stats.Triangles
	g.Culled += st.stats.Culled
	g.Fragments += st.stats.Fragments
	g.Shaded += st.stats.Shaded
	g.Clears += st.stats.Clears
	g.Dispatches += st.stats.Dispatches
	g.SubmitMs = st.stats.SubmitMs
	g.TotalMs += st.stats.SubmitMs
	s.dev.stats = g
	s.dev.mu.Unlock()
	return nil
}

// pullMatrices 从 uniform 表提取 u_mvp / u_model。
func (s *gpuSoft) pullMatrices(st *softState, m map[string][]float32) {
	if v, ok := m["u_mvp"]; ok && len(v) >= 16 {
		var mm Mat4
		copy(mm[:], v[:16])
		st.mvp = mm
	}
	if v, ok := m["u_model"]; ok && len(v) >= 16 {
		var mm Mat4
		copy(mm[:], v[:16])
		st.model = mm
	}
}

func (s *gpuSoft) clearColor(st *softState, target int, c RGBA) {
	if target < 0 || target >= softMaxTargets {
		return
	}
	if st.fast[target] && st.col8[target] != nil {
		for i := range st.col8[target] {
			st.col8[target][i] = c
		}
		return
	}
	if f := st.colF[target]; f != nil {
		r, g, b, a := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255
		for i := 0; i+3 < len(f); i += 4 {
			f[i], f[i+1], f[i+2], f[i+3] = r, g, b, a
		}
	}
}

func (s *gpuSoft) clearDepth(st *softState, v float32) {
	for i := range st.depth {
		st.depth[i] = v
	}
}

func (s *gpuSoft) copyTexture(dst, src *GPUTexture) {
	if dst == nil || src == nil {
		return
	}
	if len(dst.pix8) > 0 && len(src.pix8) > 0 {
		copy(dst.pix8[0], src.pix8[0])
		return
	}
	if len(dst.pixF) > 0 && len(src.pixF) > 0 {
		copy(dst.pixF[0], src.pixF[0])
		return
	}
	if len(dst.depth) > 0 && len(src.depth) > 0 {
		copy(dst.depth, src.depth)
	}
}

// dispatch 并行执行计算内核。
func (s *gpuSoft) dispatch(st *softState, k *ComputeKernel, groups [3]int) {
	if k == nil || k.Run == nil {
		return
	}
	st.stats.Dispatches++
	total := groups[0] * groups[1] * groups[2]
	var next int64
	var wg sync.WaitGroup
	nw := s.workers
	if nw > total {
		nw = total
	}
	for i := 0; i < nw; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				n := int(atomic.AddInt64(&next, 1)) - 1
				if n >= total {
					return
				}
				x := n % groups[0]
				y := (n / groups[0]) % groups[1]
				z := n / (groups[0] * groups[1])
				k.Run(x, y, z)
			}
		}()
	}
	wg.Wait()
}

// pullShading 从 uniform 表刷新内置着色器参数（每个 draw 一次）。
func (s *gpuSoft) pullShading(st *softState) {
	st.tint = RGBA{255, 255, 255, 255}
	st.lightDir = Vec3{-0.4, 1, 0.6}
	st.lightDir = normalizeV3(st.lightDir)
	st.lightCol = RGBA{255, 255, 255, 255}
	st.ambient = 0.25
	st.specular = 0
	st.shininess = 32
	if v := st.uniforms["u_tint"]; len(v) >= 4 {
		st.tint = RGBA{floatToU8(v[0]), floatToU8(v[1]), floatToU8(v[2]), floatToU8(v[3])}
	}
	if v := st.uniforms["u_lightDir"]; len(v) >= 3 {
		st.lightDir = normalizeV3(Vec3{v[0], v[1], v[2]})
	}
	if v := st.uniforms["u_lightColor"]; len(v) >= 4 {
		st.lightCol = RGBA{floatToU8(v[0]), floatToU8(v[1]), floatToU8(v[2]), floatToU8(v[3])}
	}
	if v := st.uniforms["u_ambient"]; len(v) >= 1 {
		st.ambient = v[0]
	}
	if v := st.uniforms["u_specular"]; len(v) >= 1 {
		st.specular = v[0]
	}
	if v := st.uniforms["u_shininess"]; len(v) >= 1 {
		st.shininess = v[0]
	}
}

// softVertex 顶点着色输出（裁剪空间 + 待插值属性）。
type softVertex struct {
	clip Vec4
	uv   Vec2
	nrm  Vec3
	col  [4]float32
	v    [8]Vec4
}

// softTri 已完成投影的三角形（屏幕空间）。
type softTri struct {
	vi [3]int32
	x  [3]float32
	y  [3]float32
	z  [3]float32
	iw [3]float32
	// 屏幕包围盒
	minX, minY, maxX, maxY float32
}

// draw 执行一次绘制调用。
func (s *gpuSoft) draw(st *softState, indexed bool, count, instances int) {
	if st.pipe == nil || st.vb == nil || st.vlayout == nil || count <= 0 {
		return
	}
	prog := st.pipe.desc.Program
	if prog == nil || prog.Vertex == nil {
		return
	}
	if indexed && st.ib == nil {
		return
	}
	st.stats.DrawCalls++
	st.stats.Instances += instances
	s.pullShading(st)

	total := count * instances
	st.stats.Vertices += total
	verts := make([]softVertex, total)

	// ---- 顶点着色（并行） ----
	nw := s.workers
	if nw > total {
		nw = total
	}
	if nw < 1 {
		nw = 1
	}
	per := (total + nw - 1) / nw
	var wg sync.WaitGroup
	for k := 0; k < nw; k++ {
		lo := k * per
		hi := lo + per
		if hi > total {
			hi = total
		}
		if lo >= hi {
			break
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				vi := i
				inst := 0
				if instances > 1 {
					vi = i % count
					inst = i / count
				}
				src := vi
				if indexed {
					src = int(u32At(st.ib.data, vi*4))
				}
				pos, nrm, uv, col, attr := readAttrs(st.vb.data, st.vlayout, st.vlayout.Stride, src)
				in := VertexIn{
					Position:   pos,
					Normal:     nrm,
					UV:         uv,
					Color:      col,
					Attr:       attr,
					VertexID:   vi,
					InstanceID: inst,
					MVP:        st.mvp,
					Model:      st.model,
				}
				if st.inst != nil && st.instLay != nil {
					_, _, _, _, ia := readAttrs(st.inst.data, st.instLay, st.instLay.Stride, inst)
					in.Inst = ia
				}
				var out Varyings
				prog.Vertex(&in, &out)
				verts[i] = softVertex{
					clip: out.Pos,
					uv:   out.UV,
					nrm:  out.Normal,
					col:  [4]float32{float32(out.Color.R) / 255, float32(out.Color.G) / 255, float32(out.Color.B) / 255, float32(out.Color.A) / 255},
					v:    out.Var,
				}
			}
		}(lo, hi)
	}
	wg.Wait()

	// ---- 装配 / 裁剪 / 投影 / tile 分箱 ----
	vpX, vpY, vpW, vpH := st.viewport[0], st.viewport[1], st.viewport[2], st.viewport[3]
	if vpW <= 0 || vpH <= 0 {
		return
	}
	tilesX := (vpW + softTile - 1) / softTile
	tilesY := (vpH + softTile - 1) / softTile
	bins := make([][]int32, tilesX*tilesY)
	tris := make([]softTri, 0, total/3+1)

	cull := st.pipe.desc.Cull

	addTri := func(a, b, c int32) {
		va, vb, vc := &verts[a], &verts[b], &verts[c]
		// 透视除法
		if va.clip.W <= 1e-6 || vb.clip.W <= 1e-6 || vc.clip.W <= 1e-6 {
			st.stats.Culled++
			return
		}
		t := softTri{vi: [3]int32{a, b, c}}
		cl := [3]*softVertex{va, vb, vc}
		for i := 0; i < 3; i++ {
			v := cl[i]
			inv := 1 / v.clip.W
			ndcX := v.clip.X * inv
			ndcY := v.clip.Y * inv
			ndcZ := v.clip.Z * inv
			t.x[i] = float32(vpX) + (ndcX*0.5+0.5)*float32(vpW)
			t.y[i] = float32(vpY) + (0.5-ndcY*0.5)*float32(vpH)
			t.z[i] = clampf(ndcZ*0.5+0.5, 0, 1)
			t.iw[i] = inv
		}
		area := (t.x[1]-t.x[0])*(t.y[2]-t.y[0]) - (t.y[1]-t.y[0])*(t.x[2]-t.x[0])
		if area == 0 {
			st.stats.Culled++
			return
		}
		// 屏幕 y 向下：area < 0 为正面。
		front := area < 0
		if cull == CullBack && !front {
			st.stats.Culled++
			return
		}
		if cull == CullFront && front {
			st.stats.Culled++
			return
		}
		t.minX = minf3(t.x[0], t.x[1], t.x[2])
		t.maxX = maxf3(t.x[0], t.x[1], t.x[2])
		t.minY = minf3(t.y[0], t.y[1], t.y[2])
		t.maxY = maxf3(t.y[0], t.y[1], t.y[2])
		if !(t.maxX > t.x[0]-1e6) { // 保守判断：包围盒与视口完全分离则丢弃
			st.stats.Culled++
			return
		}
		if t.maxX < float32(vpX) || t.minX > float32(vpX+vpW) ||
			t.maxY < float32(vpY) || t.minY > float32(vpY+vpH) {
			st.stats.Culled++
			return
		}
		idx := int32(len(tris))
		tris = append(tris, t)
		tx0 := clampi(int((t.minX-float32(vpX))/softTile), 0, tilesX-1)
		tx1 := clampi(int((t.maxX-float32(vpX))/softTile), 0, tilesX-1)
		ty0 := clampi(int((t.minY-float32(vpY))/softTile), 0, tilesY-1)
		ty1 := clampi(int((t.maxY-float32(vpY))/softTile), 0, tilesY-1)
		for ty := ty0; ty <= ty1; ty++ {
			row := ty * tilesX
			for tx := tx0; tx <= tx1; tx++ {
				bins[row+tx] = append(bins[row+tx], idx)
			}
		}
	}

	tmp := make([]int32, 0, 8)
	for inst := 0; inst < instances; inst++ {
		base := inst * count
		for i := 0; i+2 < count; i += 3 {
			ia, ib, ic := int32(base+i), int32(base+i+1), int32(base+i+2)
			// 近平面裁剪
			poly := clipNearSoft(&verts, [3]int32{ia, ib, ic}, &tmp)
			if len(poly) < 3 {
				st.stats.Culled++
				continue
			}
			for k := 1; k+1 < len(poly); k++ {
				addTri(poly[0], poly[k], poly[k+1])
			}
		}
	}
	st.stats.Triangles += len(tris)

	// ---- 片元着色（按 tile 并行） ----
	shade := &softShade{
		st:       st,
		prog:     prog,
		verts:    verts,
		tris:     tris,
		vpX:      vpX,
		vpY:      vpY,
		vpW:      vpW,
		vpH:      vpH,
		tilesX:  tilesX,
	}
	var next int64
	var wg2 sync.WaitGroup
	nw2 := s.workers
	if nw2 > len(bins) {
		nw2 = len(bins)
	}
	if nw2 < 1 {
		nw2 = 1
	}
	var frags, shaded int64
	for k := 0; k < nw2; k++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			var lf, ls int64
			for {
				n := int(atomic.AddInt64(&next, 1)) - 1
				if n >= len(bins) {
					break
				}
				if len(bins[n]) == 0 {
					continue
				}
				shade.shadeTile(n, bins[n], &lf, &ls)
			}
			atomic.AddInt64(&frags, lf)
			atomic.AddInt64(&shaded, ls)
		}()
	}
	wg2.Wait()
	st.stats.Fragments += int(frags)
	st.stats.Shaded += int(shaded)
}

// softShade 片元阶段上下文。
type softShade struct {
	st       *softState
	prog     *ShaderProgram
	verts    []softVertex
	tris     []softTri
	vpX      int
	vpY      int
	vpW      int
	vpH      int
	tilesX int
}

// shadeTile 处理一个 tile 的全部三角形。
func (sh *softShade) shadeTile(tile int, list []int32, frags, shaded *int64) {
	st := sh.st
	tx := tile % sh.tilesX
	ty := tile / sh.tilesX
	x0 := sh.vpX + tx*softTile
	y0 := sh.vpY + ty*softTile
	x1 := x0 + softTile
	y1 := y0 + softTile
	if x1 > sh.vpX+sh.vpW {
		x1 = sh.vpX + sh.vpW
	}
	if y1 > sh.vpY+sh.vpH {
		y1 = sh.vpY + sh.vpH
	}
	desc := st.pipe.desc
	for _, ti := range list {
		t := &sh.tris[ti]
		// 三角与 tile 求交
		ix0 := int(math.Floor(float64(t.minX)))
		iy0 := int(math.Floor(float64(t.minY)))
		ix1 := int(math.Ceil(float64(t.maxX))) + 1
		iy1 := int(math.Ceil(float64(t.maxY))) + 1
		if ix0 < x0 {
			ix0 = x0
		}
		if iy0 < y0 {
			iy0 = y0
		}
		if ix1 > x1 {
			ix1 = x1
		}
		if iy1 > y1 {
			iy1 = y1
		}
		if ix0 >= ix1 || iy0 >= iy1 {
			continue
		}
		x0f, y0f := t.x[0], t.y[0]
		x1f, y1f := t.x[1], t.y[1]
		x2f, y2f := t.x[2], t.y[2]
		va, vb, vc := &sh.verts[t.vi[0]], &sh.verts[t.vi[1]], &sh.verts[t.vi[2]]
		for py := iy0; py < iy1; py++ {
			cy := float32(py) + 0.5
			for px := ix0; px < ix1; px++ {
				cx := float32(px) + 0.5
				e0 := (x2f-x1f)*(cy-y1f) - (y2f-y1f)*(cx-x1f)
				e1 := (x0f-x2f)*(cy-y2f) - (y0f-y2f)*(cx-x2f)
				e2 := (x1f-x0f)*(cy-y0f) - (y1f-y0f)*(cx-x0f)
				sum := e0 + e1 + e2
				if sum == 0 {
					continue
				}
				if sum > 0 {
					if e0 < 0 || e1 < 0 || e2 < 0 {
						continue
					}
				} else if e0 > 0 || e1 > 0 || e2 > 0 {
					continue
				}
				b0, b1, b2 := e0/sum, e1/sum, e2/sum
				// 透视校正
				iw := b0*t.iw[0] + b1*t.iw[1] + b2*t.iw[2]
				if iw <= 0 {
					continue
				}
				inv := 1 / iw
				depth := (b0*t.z[0]*t.iw[0] + b1*t.z[1]*t.iw[1] + b2*t.z[2]*t.iw[2]) * inv
				*frags = *frags + 1
				if desc.DepthTest {
					d := st.depth[py*st.fb.W+px]
					if !depthPass(desc.DepthFunc, depth, d) {
						continue
					}
				}
				uvx := (b0*va.uv.X*t.iw[0] + b1*vb.uv.X*t.iw[1] + b2*vc.uv.X*t.iw[2]) * inv
				uvy := (b0*va.uv.Y*t.iw[0] + b1*vb.uv.Y*t.iw[1] + b2*vc.uv.Y*t.iw[2]) * inv
				nrx := (b0*va.nrm.X*t.iw[0] + b1*vb.nrm.X*t.iw[1] + b2*vc.nrm.X*t.iw[2]) * inv
				nry := (b0*va.nrm.Y*t.iw[0] + b1*vb.nrm.Y*t.iw[1] + b2*vc.nrm.Y*t.iw[2]) * inv
				nrz := (b0*va.nrm.Z*t.iw[0] + b1*vb.nrm.Z*t.iw[1] + b2*vc.nrm.Z*t.iw[2]) * inv
				fout := FragmentOut{Depth: -1}
				fin := FragmentIn{
					Coord:  Vec2{float32(px) + 0.5, float32(py) + 0.5},
					UV:     Vec2{uvx, uvy},
					Normal: Vec3{nrx, nry, nrz},
					Tint:      st.tint,
					LightDir:  st.lightDir,
					LightColor: st.lightCol,
					Ambient:   st.ambient,
					Specular:  st.specular,
					Shininess: st.shininess,
					Color: RGBA{
						floatToU8((b0*va.col[0]*t.iw[0] + b1*vb.col[0]*t.iw[1] + b2*vc.col[0]*t.iw[2]) * inv),
						floatToU8((b0*va.col[1]*t.iw[0] + b1*vb.col[1]*t.iw[1] + b2*vc.col[1]*t.iw[2]) * inv),
						floatToU8((b0*va.col[2]*t.iw[0] + b1*vb.col[2]*t.iw[1] + b2*vc.col[2]*t.iw[2]) * inv),
						floatToU8((b0*va.col[3]*t.iw[0] + b1*vb.col[3]*t.iw[1] + b2*vc.col[3]*t.iw[2]) * inv),
					},
					Depth: depth,
					Tex:   st.samplers,
				}
				for k := 0; k < 8; k++ {
					fin.Var[k] = varLerp(&va.v[k], &vb.v[k], &vc.v[k], t, b0, b1, b2, inv)
				}
				if sh.prog.Fragment != nil {
					sh.prog.Fragment(&fin, &fout)
				} else {
					fout.Color[0] = fin.Color
				}
				if fout.Discard {
					continue
				}
				if desc.DepthWrite && desc.DepthTest {
					d := depth
					if fout.Depth >= 0 {
						d = fout.Depth
					}
					st.depth[py*st.fb.W+px] = d
				}
				*shaded = *shaded + 1
				// 写色（MRT）
				nt := desc.ColorTargets
				if nt > softMaxTargets {
					nt = softMaxTargets
				}
				for ti2 := 0; ti2 < nt; ti2++ {
					mode := desc.Blend
					if ti2 < len(desc.BlendTargets) {
						mode = desc.BlendTargets[ti2]
					}
					src := fout.Color[ti2]
					if st.fast[ti2] && st.col8[ti2] != nil {
						idx := py*st.fb.W + px
						if idx < 0 || idx >= len(st.col8[ti2]) {
							continue
						}
						if mode == BlendNone {
							st.col8[ti2][idx] = src
						} else {
							st.col8[ti2][idx] = blendRGBA(st.col8[ti2][idx], src, mode)
						}
					} else if f := st.colF[ti2]; f != nil {
						i4 := (py*st.fb.W + px) * 4
						if i4+3 >= len(f) {
							continue
						}
						r, g, b, a := float32(src.R)/255, float32(src.G)/255, float32(src.B)/255, float32(src.A)/255
						if mode == BlendNone {
							f[i4], f[i4+1], f[i4+2], f[i4+3] = r, g, b, a
						} else {
							switch mode {
							case BlendAlpha:
								f[i4] = r*a + f[i4]*(1-a)
								f[i4+1] = g*a + f[i4+1]*(1-a)
								f[i4+2] = b*a + f[i4+2]*(1-a)
								f[i4+3] = a + f[i4+3]*(1-a)
							case BlendAdd:
								f[i4] += r
								f[i4+1] += g
								f[i4+2] += b
								f[i4+3] += a
							case BlendMultiply:
								f[i4] *= r
								f[i4+1] *= g
								f[i4+2] *= b
								f[i4+3] *= a
							default:
								f[i4], f[i4+1], f[i4+2], f[i4+3] = r, g, b, a
							}
						}
					}
				}
			}
		}
	}
}

// varLerp 单个自定义插值量的透视校正插值。
func varLerp(a, b, c *Vec4, t *softTri, b0, b1, b2, inv float32) Vec4 {
	return Vec4{
		(b0*a.X*t.iw[0] + b1*b.X*t.iw[1] + b2*c.X*t.iw[2]) * inv,
		(b0*a.Y*t.iw[0] + b1*b.Y*t.iw[1] + b2*c.Y*t.iw[2]) * inv,
		(b0*a.Z*t.iw[0] + b1*b.Z*t.iw[1] + b2*c.Z*t.iw[2]) * inv,
		(b0*a.W*t.iw[0] + b1*b.W*t.iw[1] + b2*c.W*t.iw[2]) * inv,
	}
}

// depthPass 深度比较。
func depthPass(fn DepthFunc, a, b float32) bool {
	switch fn {
	case DepthLess:
		return a < b
	case DepthLEqual:
		return a <= b
	case DepthGreater:
		return a > b
	case DepthAlways:
		return true
	}
	return a < b
}

// blendRGBA 8 位混合。
func blendRGBA(dst, src RGBA, mode BlendMode) RGBA {
	switch mode {
	case BlendAlpha:
		a := float32(src.A) / 255
		return RGBA{
			uint8(float32(src.R)*a + float32(dst.R)*(1-a)),
			uint8(float32(src.G)*a + float32(dst.G)*(1-a)),
			uint8(float32(src.B)*a + float32(dst.B)*(1-a)),
			uint8(clampf(a+float32(dst.A)/255*(1-a), 0, 1)*255 + 0.5),
		}
	case BlendPremultiplied:
		a := float32(src.A) / 255
		return RGBA{
			minU8(255, uint8(float32(src.R)+float32(dst.R)*(1-a))),
			minU8(255, uint8(float32(src.G)+float32(dst.G)*(1-a))),
			minU8(255, uint8(float32(src.B)+float32(dst.B)*(1-a))),
			src.A,
		}
	case BlendAdd:
		return RGBA{
			minU8(255, src.R+dst.R),
			minU8(255, src.G+dst.G),
			minU8(255, src.B+dst.B),
			minU8(255, src.A+dst.A),
		}
	case BlendMultiply:
		return RGBA{
			uint8(uint16(src.R) * uint16(dst.R) / 255),
			uint8(uint16(src.G) * uint16(dst.G) / 255),
			uint8(uint16(src.B) * uint16(dst.B) / 255),
			uint8(uint16(src.A) * uint16(dst.A) / 255),
		}
	}
	return src
}

func minU8(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}

func minf3(a, b, c float32) float32 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func maxf3(a, b, c float32) float32 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

// clipNearSoft 对三角形做近平面（z + w >= 0）裁剪，返回多边形顶点索引。
// 新顶点追加到 verts，因此以指针方式传入（调用方需在装配结束后重新取切片）。
func clipNearSoft(verts *[]softVertex, tri [3]int32, scratch *[]int32) []int32 {
	const eps = 1e-6
	vs := *verts
	d := [3]float32{}
	allIn := true
	allOut := true
	for i := 0; i < 3; i++ {
		c := vs[tri[i]].clip
		d[i] = c.Z + c.W
		if d[i] < eps {
			allIn = false
		} else {
			allOut = false
		}
	}
	if allOut {
		return nil
	}
	if allIn {
		return tri[:]
	}
	// Sutherland-Hodgman（单平面）
	outIdx := (*scratch)[:0]
	for i := 0; i < 3; i++ {
		j := (i + 1) % 3
		di, dj := d[i], d[j]
		if di >= eps {
			outIdx = append(outIdx, tri[i])
		}
		if (di >= eps) != (dj >= eps) {
			t := di / (di - dj)
			a, b := vs[tri[i]], vs[tri[j]]
			nv := softVertex{
				clip: lerpVec4(a.clip, b.clip, t),
				uv:   Vec2{lerpf(a.uv.X, b.uv.X, t), lerpf(a.uv.Y, b.uv.Y, t)},
				nrm:  Vec3{lerpf(a.nrm.X, b.nrm.X, t), lerpf(a.nrm.Y, b.nrm.Y, t), lerpf(a.nrm.Z, b.nrm.Z, t)},
			}
			for k := 0; k < 4; k++ {
				nv.col[k] = lerpf(a.col[k], b.col[k], t)
			}
			for k := 0; k < 8; k++ {
				nv.v[k] = lerpVec4(a.v[k], b.v[k], t)
			}
			vs = append(vs, nv)
			*verts = vs
			outIdx = append(outIdx, int32(len(vs)-1))
		}
	}
	*scratch = outIdx
	if len(outIdx) < 3 {
		return nil
	}
	return outIdx
}

func lerpVec4(a, b Vec4, t float32) Vec4 {
	return Vec4{lerpf(a.X, b.X, t), lerpf(a.Y, b.Y, t), lerpf(a.Z, b.Z, t), lerpf(a.W, b.W, t)}
}
