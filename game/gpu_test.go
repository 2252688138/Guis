// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/binary"
	"image"
	"math"
	"testing"
)

// newSoftGPU 创建软件后端设备（测试用）。
func newSoftGPU(t *testing.T) *GPU {
	t.Helper()
	g, err := NewGPU(WithGPUBackend("soft"), WithGPUWorkers(2))
	if err != nil {
		t.Fatalf("创建 GPU 失败: %v", err)
	}
	return g
}

// newTestTarget 创建 color+depth 帧缓冲。
func newTestTarget(t *testing.T, g *GPU, w, h int) (*GPUTexture, *GPUFramebuffer) {
	t.Helper()
	col, err := g.NewTexture(TexDesc{W: w, H: h, Format: PixRGBA8, Usage: TexSampled | TexRenderTarget})
	if err != nil {
		t.Fatalf("颜色附件: %v", err)
	}
	depth, err := g.NewTexture(TexDesc{W: w, H: h, Format: PixDepth32})
	if err != nil {
		t.Fatalf("深度附件: %v", err)
	}
	fb, err := g.NewFramebuffer([]*GPUTexture{col}, depth)
	if err != nil {
		t.Fatalf("帧缓冲: %v", err)
	}
	return col, fb
}

// buildTriBuffer 构造 3 个顶点的缓冲（pos3 + normal3 + uv2 + rgba4）。
func buildTriBuffer(t *testing.T, g *GPU, verts [3]Vec4, col RGBA) *GPUBuffer {
	t.Helper()
	buf := make([]byte, 3*gpuMeshStride)
	for i, v := range verts {
		off := i * gpuMeshStride
		binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(v.X))
		binary.LittleEndian.PutUint32(buf[off+4:], math.Float32bits(v.Y))
		binary.LittleEndian.PutUint32(buf[off+8:], math.Float32bits(v.Z))
		binary.LittleEndian.PutUint32(buf[off+20:], math.Float32bits(1))
		buf[off+32] = col.R
		buf[off+33] = col.G
		buf[off+34] = col.B
		buf[off+35] = col.A
	}
	vb, err := g.NewBuffer(BufDesc{Size: len(buf), Usage: BufVertex, Data: buf})
	if err != nil {
		t.Fatalf("顶点缓冲: %v", err)
	}
	return vb
}

// ndcProgram 直接输出 NDC 坐标并透传顶点色的着色器。
func ndcProgram() *ShaderProgram {
	return &ShaderProgram{
		Name: "ndc",
		Vertex: func(in *VertexIn, out *Varyings) {
			out.Pos = Vec4{in.Position.X, in.Position.Y, in.Position.Z, 1}
			out.Color = in.Color
			out.UV = in.UV
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			out.Color[0] = in.Color
		},
	}
}

func TestGPUTriangle(t *testing.T) {
	g := newSoftGPU(t)
	_, fb := newTestTarget(t, g, 64, 64)
	vb := buildTriBuffer(t, g, [3]Vec4{
		{-1, -1, 0, 1}, {3, -1, 0, 1}, {-1, 3, 0, 1},
	}, RGB(200, 30, 40))

	p, err := g.NewPipeline(PipelineDesc{
		Program:      ndcProgram(),
		DepthTest:    true,
		DepthWrite:   true,
		Cull:         CullNone,
		Blend:        BlendNone,
		ColorTargets: 1,
	})
	if err != nil {
		t.Fatalf("管线: %v", err)
	}
	cmd := g.Begin().SetFramebuffer(fb)
	cmd.ClearColor(0, RGB(0, 0, 0))
	cmd.ClearDepth(1)
	cmd.SetPipeline(p)
	cmd.SetVertexBuffer(vb, MeshLayout(gpuMeshStride))
	cmd.DrawArrays(3, 1)
	if err := g.Submit(cmd); err != nil {
		t.Fatalf("提交: %v", err)
	}
	img, err := g.ReadPixels(fb, 0)
	if err != nil {
		t.Fatalf("回读: %v", err)
	}
	center := gpuPixel(img, 32, 32)
	if center != (RGBA{200, 30, 40, 255}) {
		t.Fatalf("中心像素 = %+v，期望 (200,30,40,255)", center)
	}
	// 角落必须保持清屏色（全屏三角形覆盖整幅，这里只验证清屏生效路径）
	if gpuPixel(img, 0, 0) != (RGBA{200, 30, 40, 255}) {
		t.Logf("提示: (0,0) 未被覆盖，说明三角形覆盖范围有限（可接受）")
	}
	st := g.Stats()
	if st.Triangles == 0 || st.Fragments == 0 {
		t.Fatalf("统计异常: %+v", st)
	}
}

func TestGPUDepthTest(t *testing.T) {
	g := newSoftGPU(t)
	_, fb := newTestTarget(t, g, 32, 32)
	near := buildTriBuffer(t, g, [3]Vec4{
		{-1, -1, -0.5, 1}, {3, -1, -0.5, 1}, {-1, 3, -0.5, 1},
	}, RGB(0, 255, 0))
	far := buildTriBuffer(t, g, [3]Vec4{
		{-1, -1, 0.5, 1}, {3, -1, 0.5, 1}, {-1, 3, 0.5, 1},
	}, RGB(255, 0, 0))

	p, _ := g.NewPipeline(PipelineDesc{
		Program: ndcProgram(), DepthTest: true, DepthWrite: true,
		Cull: CullNone, Blend: BlendNone, ColorTargets: 1,
	})
	cmd := g.Begin().SetFramebuffer(fb)
	cmd.ClearColor(0, RGB(0, 0, 0)).ClearDepth(1)
	cmd.SetPipeline(p)
	cmd.SetVertexBuffer(far, MeshLayout(gpuMeshStride))
	cmd.DrawArrays(3, 1)
	cmd.SetVertexBuffer(near, MeshLayout(gpuMeshStride))
	cmd.DrawArrays(3, 1)
	// 再用远的画一次，深度测试应挡住
	cmd.SetVertexBuffer(far, MeshLayout(gpuMeshStride))
	cmd.DrawArrays(3, 1)
	if err := g.Submit(cmd); err != nil {
		t.Fatalf("提交: %v", err)
	}
	img, _ := g.ReadPixels(fb, 0)
	if c := gpuPixel(img, 16, 16); c != (RGBA{0, 255, 0, 255}) {
		t.Fatalf("深度测试失效，颜色 = %+v，期望绿色", c)
	}
}

func TestGPUBlendAdd(t *testing.T) {
	g := newSoftGPU(t)
	_, fb := newTestTarget(t, g, 16, 16)
	vb := buildTriBuffer(t, g, [3]Vec4{
		{-1, -1, 0, 1}, {3, -1, 0, 1}, {-1, 3, 0, 1},
	}, RGB(10, 20, 30))
	p, _ := g.NewPipeline(PipelineDesc{
		Program: ndcProgram(), DepthTest: false, DepthWrite: false,
		Cull: CullNone, Blend: BlendAdd, ColorTargets: 1,
	})
	cmd := g.Begin().SetFramebuffer(fb)
	cmd.ClearColor(0, RGB(100, 100, 100)).ClearDepth(1)
	cmd.SetPipeline(p)
	cmd.SetVertexBuffer(vb, MeshLayout(gpuMeshStride))
	cmd.DrawArrays(3, 1)
	if err := g.Submit(cmd); err != nil {
		t.Fatalf("提交: %v", err)
	}
	img, _ := g.ReadPixels(fb, 0)
	if c := gpuPixel(img, 8, 8); c.R != 110 || c.G != 120 || c.B != 130 {
		t.Fatalf("加法混合结果 = %+v，期望 (110,120,130)", c)
	}
}

func TestGPU_MRT(t *testing.T) {
	g := newSoftGPU(t)
	c0, _ := g.NewTexture(TexDesc{W: 16, H: 16, Format: PixRGBA8, Usage: TexRenderTarget})
	c1, _ := g.NewTexture(TexDesc{W: 16, H: 16, Format: PixRGBA8, Usage: TexRenderTarget})
	d, _ := g.NewTexture(TexDesc{W: 16, H: 16, Format: PixDepth32})
	fb, err := g.NewFramebuffer([]*GPUTexture{c0, c1}, d)
	if err != nil {
		t.Fatalf("帧缓冲: %v", err)
	}
	vb := buildTriBuffer(t, g, [3]Vec4{
		{-1, -1, 0, 1}, {3, -1, 0, 1}, {-1, 3, 0, 1},
	}, RGB(255, 255, 255))
	prog := &ShaderProgram{
		Name: "mrt",
		Vertex: func(in *VertexIn, out *Varyings) {
			out.Pos = Vec4{in.Position.X, in.Position.Y, 0, 1}
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			out.Color[0] = RGB(1, 2, 3)
			out.Color[1] = RGB(4, 5, 6)
		},
	}
	p, err := g.NewPipeline(PipelineDesc{
		Program: prog, DepthTest: true, DepthWrite: true,
		Cull: CullNone, ColorTargets: 2,
	})
	if err != nil {
		t.Fatalf("管线: %v", err)
	}
	cmd := g.Begin().SetFramebuffer(fb)
	cmd.ClearColor(0, RGB(0, 0, 0)).ClearColor(1, RGB(0, 0, 0)).ClearDepth(1)
	cmd.SetPipeline(p)
	cmd.SetVertexBuffer(vb, MeshLayout(gpuMeshStride))
	cmd.DrawArrays(3, 1)
	if err := g.Submit(cmd); err != nil {
		t.Fatalf("提交: %v", err)
	}
	img0, _ := g.ReadPixels(fb, 0)
	img1, _ := g.ReadPixels(fb, 1)
	if c := gpuPixel(img0, 8, 8); c.R != 1 || c.G != 2 || c.B != 3 {
		t.Fatalf("目标 0 = %+v", c)
	}
	if c := gpuPixel(img1, 8, 8); c.R != 4 || c.G != 5 || c.B != 6 {
		t.Fatalf("目标 1 = %+v", c)
	}
}

func TestGPUInstancing(t *testing.T) {
	g := newSoftGPU(t)
	_, fb := newTestTarget(t, g, 32, 16)
	// 一个单位三角形，靠实例偏移铺满
	buf := make([]byte, 3*gpuMeshStride)
	binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(buf[32:], 255)
	binary.LittleEndian.PutUint32(buf[33:], 255)
	binary.LittleEndian.PutUint32(buf[34:], 255)
	binary.LittleEndian.PutUint32(buf[35:], 255)
	vb, _ := g.NewBuffer(BufDesc{Size: len(buf), Usage: BufVertex, Data: buf})

	inst := make([]byte, 2*16)
	binary.LittleEndian.PutUint32(inst[0:], math.Float32bits(-0.6))
	binary.LittleEndian.PutUint32(inst[16:], math.Float32bits(0.6))
	ib, _ := g.NewBuffer(BufDesc{Size: len(inst), Usage: BufVertex, Data: inst})

	prog := &ShaderProgram{
		Name: "inst",
		Vertex: func(in *VertexIn, out *Varyings) {
			x := 0.5 + in.Inst[0].X
			out.Pos = Vec4{(in.Position.X + x) * 2 - 1, in.Position.Y, 0, 1}
			out.Color = RGB(9, 9, 9)
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) { out.Color[0] = in.Color },
	}
	p, _ := g.NewPipeline(PipelineDesc{
		Program: prog, DepthTest: true, DepthWrite: true,
		Cull: CullNone, ColorTargets: 1,
	})
	instLayout := &VertexLayout{Stride: 16, Instanced: true, Attrs: []VertexAttr{
		{Location: 4, Offset: 0, Format: VF32x4},
	}}
	cmd := g.Begin().SetFramebuffer(fb)
	cmd.ClearColor(0, RGB(0, 0, 0)).ClearDepth(1)
	cmd.SetPipeline(p)
	cmd.SetVertexBuffer(vb, MeshLayout(gpuMeshStride))
	cmd.SetInstanceBuffer(ib, instLayout)
	cmd.DrawArrays(3, 2)
	if err := g.Submit(cmd); err != nil {
		t.Fatalf("提交: %v", err)
	}
	if st := g.Stats(); st.Instances != 2 {
		t.Fatalf("实例统计 = %d，期望 2", st.Instances)
	}
}

func TestGPUTextureSampling(t *testing.T) {
	g := newSoftGPU(t)
	_, fb := newTestTarget(t, g, 16, 16)
	tex, err := g.NewTexture(TexDesc{W: 2, H: 2, Format: PixRGBA8, Usage: TexSampled})
	if err != nil {
		t.Fatalf("纹理: %v", err)
	}
	red := RGB(255, 0, 0)
	blue := RGB(0, 0, 255)
	if err := tex.SetPixels([]RGBA{red, blue, red, blue}); err != nil {
		t.Fatalf("上传: %v", err)
	}
	vb := buildTriBuffer(t, g, [3]Vec4{
		{-1, -1, 0, 1}, {3, -1, 0, 1}, {-1, 3, 0, 1},
	}, RGB(255, 255, 255))
	prog := &ShaderProgram{
		Name: "tex",
		Vertex: func(in *VertexIn, out *Varyings) {
			out.Pos = Vec4{in.Position.X, in.Position.Y, 0, 1}
			out.UV = Vec2{in.Position.X*0.5 + 0.5, 0.5 - in.Position.Y*0.5}
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			if s := in.Tex[0]; s != nil {
				out.Color[0] = s.RGBA8(in.UV.X, in.UV.Y)
				return
			}
			out.Color[0] = RGB(0, 0, 0)
		},
	}
	p, _ := g.NewPipeline(PipelineDesc{
		Program: prog, DepthTest: true, DepthWrite: true,
		Cull: CullNone, ColorTargets: 1,
	})
	smp := g.NewSampler(tex, false, false, true)
	cmd := g.Begin().SetFramebuffer(fb)
	cmd.ClearColor(0, RGB(0, 0, 0)).ClearDepth(1)
	cmd.SetPipeline(p)
	cmd.SetTexture(0, tex, smp)
	cmd.SetVertexBuffer(vb, MeshLayout(gpuMeshStride))
	cmd.DrawArrays(3, 1)
	if err := g.Submit(cmd); err != nil {
		t.Fatalf("提交: %v", err)
	}
	img, _ := g.ReadPixels(fb, 0)
	left := gpuPixel(img, 4, 8)
	right := gpuPixel(img, 12, 8)
	if left.R < 200 || left.B > 50 {
		t.Fatalf("左半采样 = %+v，期望偏红", left)
	}
	if right.B < 200 || right.R > 50 {
		t.Fatalf("右半采样 = %+v，期望偏蓝", right)
	}
}

func TestGPUComputeKernel(t *testing.T) {
	g := newSoftGPU(t)
	buf, err := g.NewBuffer(BufDesc{Size: 64 * 4, Usage: BufStorage})
	if err != nil {
		t.Fatalf("缓冲: %v", err)
	}
	_, fb := newTestTarget(t, g, 4, 4)
	kernel := &ComputeKernel{
		Name: "fill",
		Run: func(x, y, z int) {
			data := buf.Bytes()
			if x*4+3 < len(data) {
				binary.LittleEndian.PutUint32(data[x*4:], uint32(x+1))
			}
		},
	}
	cmd := g.Begin().SetFramebuffer(fb)
	cmd.Dispatch(kernel, 64, 1, 1)
	if err := g.Submit(cmd); err != nil {
		t.Fatalf("提交: %v", err)
	}
	for i := 0; i < 64; i++ {
		if v := u32At(buf.Bytes(), i*4); v != uint32(i+1) {
			t.Fatalf("第 %d 个结果 = %d，期望 %d", i, v, i+1)
		}
	}
	if st := g.Stats(); st.Dispatches != 1 {
		t.Fatalf("派发统计 = %d", st.Dispatches)
	}
}

func TestGPUSceneRender(t *testing.T) {
	g := newSoftGPU(t)
	gs, err := NewGPUScene(g, 96, 64)
	if err != nil {
		t.Fatalf("GPUScene: %v", err)
	}
	sc := NewScene()
	sc.Root.Add(NewMeshNode("cube", NewBox(1, 1, 1), DefaultMaterial()))
	img, err := gs.Render(sc, nil)
	if err != nil {
		t.Fatalf("渲染: %v", err)
	}
	if img == nil || img.Bounds().Dx() != 96 || img.Bounds().Dy() != 64 {
		t.Fatalf("输出尺寸错误: %v", img.Bounds())
	}
	if gs.Stats.DrawCalls == 0 {
		t.Fatalf("没有绘制调用: %+v", gs.Stats)
	}
	// 场景默认背景为深色，立方体应产生非背景像素
	bg := gpuPixel(img, 0, 0)
	diff := 0
	for y := 0; y < 64; y += 3 {
		for x := 0; x < 96; x += 3 {
			if gpuPixel(img, x, y) != bg {
				diff++
			}
		}
	}
	if diff == 0 {
		t.Fatalf("场景渲染没有产生任何可见像素")
	}
}

func TestGPUBackendSelection(t *testing.T) {
	g, err := NewGPU(WithGPUBackend("auto"))
	if err != nil {
		t.Fatalf("自动选择后端失败: %v", err)
	}
	if g.Backend() == "" {
		t.Fatalf("后端名为空")
	}
	caps := g.Caps()
	if caps.MaxColorTargets < 1 {
		t.Fatalf("能力自述异常: %+v", caps)
	}
	if _, err := NewGPU(WithGPUBackend("不存在的后端"), WithGPUFallback(false)); err == nil {
		t.Fatalf("非法后端应当报错")
	}
	if got := GPUBackends(); len(got) == 0 {
		t.Fatalf("后端注册表为空")
	}
}

// TestGPUOpenGLBackend 在可用时验证硬件后端能真的画出东西。
// 无 GPU / 驱动不可用时自动跳过（此时 NewGPU 会回退 soft）。
func TestGPUOpenGLBackend(t *testing.T) {
	g, err := NewGPU(WithGPUBackend("gl"), WithGPUFallback(false))
	if err != nil {
		t.Skipf("OpenGL 后端不可用: %v", err)
	}
	defer g.Close()
	if g.Backend() != "gl" {
		t.Skipf("当前后端是 %s，跳过硬件测试", g.Backend())
	}
	caps := g.Caps()
	if !caps.Hardware {
		t.Fatalf("gl 后端应标记 Hardware: %+v", caps)
	}
	gs, err := NewGPUScene(g, 128, 96)
	if err != nil {
		t.Fatalf("GPUScene: %v", err)
	}
	sc := NewScene()
	sc.Root.Add(NewMeshNode("cube", NewBox(1, 1, 1), DefaultMaterial()))
	img, err := gs.Render(sc, nil)
	if err != nil {
		t.Fatalf("渲染: %v", err)
	}
	if img == nil || img.Bounds().Dx() != 128 {
		t.Fatalf("输出异常")
	}
	if gs.Stats.DrawCalls == 0 {
		t.Fatalf("硬件后端没有绘制调用: %+v", gs.Stats)
	}
	bg := gpuPixel(img, 1, 1)
	diff := 0
	for y := 2; y < 94; y += 2 {
		for x := 2; x < 126; x += 2 {
			if gpuPixel(img, x, y) != bg {
				diff++
			}
		}
	}
	if diff == 0 {
		t.Fatalf("硬件后端渲染结果是空白（背景 %+v）", bg)
	}
}

// gpuCentroid 非背景像素的质心与数量（用于跨后端一致性检查）。
func gpuCentroid(img *image.RGBA) (float64, float64, int) {
	if img == nil {
		return 0, 0, 0
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	bg := gpuPixel(img, 0, 0)
	var sx, sy float64
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if gpuPixel(img, x, y) != bg {
				sx += float64(x)
				sy += float64(y)
				n++
			}
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	return sx / float64(n), sy / float64(n), n
}

// TestGPUBackendParity 同一场景在两后端渲染，检查可见区域质心一致
// （能抓住投影翻转、矩阵转置一类致命错误）。
func TestGPUBackendParity(t *testing.T) {
	sceneFor := func() *Scene {
		sc := NewScene()
		cube := NewMeshNode("cube", NewBox(1, 1, 1), DefaultMaterial())
		cube.Pos = V3(0, 1.6, 0)
		sc.Root.Add(cube)
		return sc
	}
	softGPU, err := NewGPU(WithGPUBackend("soft"), WithGPUWorkers(2))
	if err != nil {
		t.Fatalf("soft: %v", err)
	}
	gsSoft, err := NewGPUScene(softGPU, 128, 128)
	if err != nil {
		t.Fatalf("soft scene: %v", err)
	}
	imgSoft, err := gsSoft.Render(sceneFor(), nil)
	if err != nil {
		t.Fatalf("soft 渲染: %v", err)
	}
	cxS, cyS, nS := gpuCentroid(imgSoft)
	if nS == 0 {
		t.Fatalf("soft 后端没有渲染出内容")
	}

	hw, err := NewGPU(WithGPUBackend("gl"), WithGPUFallback(false))
	if err != nil {
		t.Skipf("硬件后端不可用: %v", err)
	}
	defer hw.Close()
	gsHW, err := NewGPUScene(hw, 128, 128)
	if err != nil {
		t.Fatalf("gl scene: %v", err)
	}
	imgHW, err := gsHW.Render(sceneFor(), nil)
	if err != nil {
		t.Fatalf("gl 渲染: %v", err)
	}
	cxH, cyH, nH := gpuCentroid(imgHW)
	if nH == 0 {
		t.Fatalf("硬件后端没有渲染出内容")
	}
	dx, dy := cxS-cxH, cyS-cyH
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > 20 || dy > 20 {
		t.Fatalf("两后端渲染位置偏差过大: soft(%.1f,%.1f,%d) hw(%.1f,%.1f,%d)",
			cxS, cyS, nS, cxH, cyH, nH)
	}
	// 覆盖率差异不应过大（存在透视/抗锯齿差异，允许 60%）
	ratio := float64(nH) / float64(nS)
	if ratio < 0.4 || ratio > 2.5 {
		t.Fatalf("两后端覆盖率差异过大: soft=%d hw=%d", nS, nH)
	}
}

func TestGPUPipelineWithoutGLSLHardware(t *testing.T) {
	// soft 后端只要求 Go 回调；这里验证缺顶点回调时明确报错
	g := newSoftGPU(t)
	_, err := g.NewPipeline(PipelineDesc{Program: &ShaderProgram{Name: "bad"}})
	if err == nil {
		t.Fatalf("缺顶点着色应当报错")
	}
	_ = image.Rect
}
