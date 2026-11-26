// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"errors"
	"image"
	"sort"
)

// GPUScene 把现有 Scene / Node / Material 接到 GPU 管线上。
//
// 与软件 Renderer 的区别：渲染由 GPU 后端（soft / gl）执行命令列表完成，
// 着色器可替换、支持 MRT/计算内核，便于做大型游戏的渲染管线（G-Buffer、
// 后处理链、实例化）而不是固定流程。
//
// 用法：
//
//	gs, _ := game.NewGPUScene(gpu, 1280, 720)
//	img, _ := gs.Render(scene, scene.Camera)  // 每帧调用
type GPUScene struct {
	gpu *GPU
	// W, H 离屏渲染尺寸。
	W, H int
	// ClearColor 清屏色（A=0 时用场景 Background）。
	ClearColor RGBA

	color *GPUTexture
	depth *GPUTexture
	fb    *GPUFramebuffer

	pipes  map[gpuPipeKey]*Pipeline
	meshes map[*Mesh]*gpuMeshRes
	texs   map[*Texture]*GPUTexture

	// Stats 最近一次渲染后的设备累计统计。
	Stats GPUStats
}

// gpuMeshRes 网格的 GPU 资源。
type gpuMeshRes struct {
	vb     *GPUBuffer
	ib     *GPUBuffer
	layout *VertexLayout
	count  int
}

// gpuPipeKey 管线缓存键。
type gpuPipeKey struct {
	shade      ShadeModel
	blend      BlendMode
	cull       CullMode
	depthWrite bool
	targets    int
}

// NewGPUScene 创建 GPU 场景渲染器（分配颜色与深度附件）。
func NewGPUScene(g *GPU, w, h int) (*GPUScene, error) {
	if g == nil {
		return nil, errors.New("game: GPU 设备为空")
	}
	if w <= 0 || h <= 0 {
		return nil, errors.New("game: 渲染尺寸非法")
	}
	s := &GPUScene{
		gpu:    g,
		W:      w,
		H:      h,
		pipes:  map[gpuPipeKey]*Pipeline{},
		meshes: map[*Mesh]*gpuMeshRes{},
		texs:   map[*Texture]*GPUTexture{},
	}
	if err := s.alloc(); err != nil {
		return nil, err
	}
	return s, nil
}

// alloc 分配/重建附件。
func (s *GPUScene) alloc() error {
	color, err := s.gpu.NewTexture(TexDesc{
		W: s.W, H: s.H, Format: PixRGBA8, Usage: TexSampled | TexRenderTarget,
	})
	if err != nil {
		return err
	}
	depth, err := s.gpu.NewTexture(TexDesc{
		W: s.W, H: s.H, Format: PixDepth32, Usage: TexDepthStencil,
	})
	if err != nil {
		return err
	}
	fb, err := s.gpu.NewFramebuffer([]*GPUTexture{color}, depth)
	if err != nil {
		return err
	}
	s.color, s.depth, s.fb = color, depth, fb
	return nil
}

// Resize 改变渲染尺寸（重建附件，已缓存资源保留）。
func (s *GPUScene) Resize(w, h int) error {
	if w <= 0 || h <= 0 {
		return errors.New("game: 渲染尺寸非法")
	}
	if w == s.W && h == s.H {
		return nil
	}
	s.W, s.H = w, h
	return s.alloc()
}

// Color 颜色附件（可作为后处理输入纹理）。
func (s *GPUScene) Color() *GPUTexture { return s.color }

// Depth 深度附件。
func (s *GPUScene) Depth() *GPUTexture { return s.depth }

// Framebuffer 帧缓冲。
func (s *GPUScene) Framebuffer() *GPUFramebuffer { return s.fb }

// Texture 把游戏纹理上传为 GPU 纹理（带缓存）。
func (s *GPUScene) Texture(t *Texture) (*GPUTexture, error) {
	if t == nil {
		return nil, nil
	}
	if g, ok := s.texs[t]; ok {
		return g, nil
	}
	g, err := s.gpu.NewTextureFromGame(t)
	if err != nil {
		return nil, err
	}
	s.texs[t] = g
	return g, nil
}

// Mesh 把网格上传为 GPU 缓冲（带缓存）。
func (s *GPUScene) Mesh(m *Mesh) (*gpuMeshRes, error) {
	if m == nil {
		return nil, errors.New("game: 空网格")
	}
	if r, ok := s.meshes[m]; ok {
		return r, nil
	}
	vb, ib, layout, err := s.gpu.NewMeshBuffers(m)
	if err != nil {
		return nil, err
	}
	count := len(m.Indices)
	if count == 0 {
		count = len(m.Positions)
	}
	r := &gpuMeshRes{vb: vb, ib: ib, layout: layout, count: count}
	s.meshes[m] = r
	return r, nil
}

// Pipeline 取（或创建）与材质匹配的管线。
func (s *GPUScene) Pipeline(mat *Material, blend BlendMode, cull CullMode, depthWrite bool) (*Pipeline, error) {
	if mat == nil {
		mat = DefaultMaterial()
	}
	key := gpuPipeKey{shade: mat.Shade, blend: blend, cull: cull, depthWrite: depthWrite, targets: 1}
	if p, ok := s.pipes[key]; ok {
		return p, nil
	}
	var prog *ShaderProgram
	switch mat.Shade {
	case ShadeUnlit:
		prog = ShaderUnlit()
	case ShadeToon:
		prog = ShaderToon(4)
	default:
		prog = ShaderPhong()
	}
	p, err := s.gpu.NewPipeline(PipelineDesc{
		Program:      prog,
		DepthTest:    true,
		DepthWrite:   depthWrite,
		DepthFunc:    DepthLess,
		Cull:         cull,
		Blend:        blend,
		ColorTargets: 1,
	})
	if err != nil {
		return nil, err
	}
	s.pipes[key] = p
	return p, nil
}

// gpuItem 单个待绘制项。
type gpuItem struct {
	node *Node
	mesh *Mesh
	res  *gpuMeshRes
	mat  *Material
	dist float32
}

// Render 渲染场景并回读为 image.RGBA（软件合成路径）。
func (s *GPUScene) Render(sc *Scene, cam *Camera) (*image.RGBA, error) {
	if err := s.renderPass(sc, cam); err != nil {
		return nil, err
	}
	return s.gpu.ReadPixels(s.fb, 0)
}

// renderPass 只执行绘制，结果留在离屏 FBO 上不回读（零拷贝呈现用）。
func (s *GPUScene) renderPass(sc *Scene, cam *Camera) error {
	if sc == nil {
		return errors.New("game: 场景为空")
	}
	if cam == nil {
		cam = sc.Camera
	}
	if cam == nil {
		return errors.New("game: 场景没有相机")
	}
	if sc.Root != nil {
		sc.Root.BuildWorld(Identity())
	}
	aspect := float32(s.W) / float32(s.H)
	mvp := cam.ViewProj(aspect)
	ld, lc, amb := gpuLightParams(sc)

	bg := s.ClearColor
	if sc.Background.A != 0 {
		bg = sc.Background
	}

	// 收集可见项，按不透明 / 透明分批
	var opaque, transparent []gpuItem
	camPos := cam.Pos
	collect := func(n *Node) {
		if n == nil || nMeshEmpty(n) {
			return
		}
		dist := n.WorldPos().Sub(camPos).Len()
		mesh := n.Mesh
		if len(n.LODs) > 0 {
			mesh = n.PickLOD(dist)
		}
		if mesh == nil {
			return
		}
		res, err := s.Mesh(mesh)
		if err != nil || res == nil {
			return
		}
		it := gpuItem{node: n, mesh: mesh, res: res, mat: n.Material, dist: dist}
		if it.mat == nil {
			it.mat = DefaultMaterial()
		}
		if it.mat.AlphaMode == AlphaBlend {
			transparent = append(transparent, it)
		} else {
			opaque = append(opaque, it)
		}
	}
	if sc.Root != nil {
		sc.Root.Each(collect)
	}
	sort.SliceStable(transparent, func(i, j int) bool { return transparent[i].dist > transparent[j].dist })

	cmd := s.gpu.Begin().SetFramebuffer(s.fb).Name("gpu-scene")
	cmd.ClearColor(0, bg)
	cmd.ClearDepth(1)

	drawItem := func(it gpuItem, blend BlendMode, depthWrite bool, cull CullMode) error {
		mat := it.mat
		p, err := s.Pipeline(mat, blend, cull, depthWrite)
		if err != nil {
			return err
		}
		cullMode := cull
		if mat.DoubleSided {
			cullMode = CullNone
		}
		if cullMode != cull {
			p, err = s.Pipeline(mat, blend, cullMode, depthWrite)
			if err != nil {
				return err
			}
		}
		tint := mat.BaseColor
		cmd.SetPipeline(p)
		cmd.SetUniformMat4("u_mvp", mvp.Mul(it.node.WorldMatrix()))
		cmd.SetUniformMat4("u_model", it.node.WorldMatrix())
		cmd.SetUniform("u_tint",
			float32(tint.R)/255, float32(tint.G)/255, float32(tint.B)/255, float32(tint.A)/255)
		cmd.SetUniform("u_lightDir", ld.X, ld.Y, ld.Z)
		cmd.SetUniform("u_lightColor",
			float32(lc.R)/255, float32(lc.G)/255, float32(lc.B)/255, float32(lc.A)/255)
		cmd.SetUniform("u_ambient", amb)
		cmd.SetUniform("u_specular", mat.Specular)
		cmd.SetUniform("u_shininess", mat.Shininess)
		if mat.Texture != nil {
			gt, err := s.Texture(mat.Texture)
			if err != nil {
				return err
			}
			cmd.SetTexture(0, gt, &TextureSampler{
				Tex:    gt,
				Linear: !mat.Nearest,
				Mip:    mat.Mipmap,
				Repeat: true,
			})
		} else {
			cmd.SetTexture(0, nil, nil)
		}
		cmd.SetVertexBuffer(it.res.vb, it.res.layout)
		if it.res.ib != nil {
			cmd.SetIndexBuffer(it.res.ib)
			cmd.Draw(it.res.count, 1)
		} else {
			cmd.DrawArrays(it.res.count, 1)
		}
		return nil
	}

	for _, it := range opaque {
		if err := drawItem(it, BlendNone, true, CullBack); err != nil {
			return err
		}
	}
	for _, it := range transparent {
		if err := drawItem(it, BlendAlpha, false, CullNone); err != nil {
			return err
		}
	}

	if err := s.gpu.Submit(cmd); err != nil {
		return err
	}
	s.Stats = s.gpu.Stats()
	return nil
}

// nMeshEmpty 节点是否没有可绘制网格。
func nMeshEmpty(n *Node) bool {
	if n.Mesh != nil {
		return len(n.Mesh.Positions) == 0
	}
	return len(n.LODs) == 0
}

// gpuLightParams 从场景提取方向光参数（取首个方向光 / 回退默认）。
func gpuLightParams(sc *Scene) (Vec3, RGBA, float32) {
	dir := normalizeV3(Vec3{-0.4, 1, 0.6})
	col := RGBA{255, 255, 255, 255}
	amb := float32(0.25)
	if sc != nil && sc.Ambient.A != 0 {
		amb = float32(sc.Ambient.R) / 255 * 0.6
	}
	if sc == nil {
		return dir, col, amb
	}
	for i := range sc.Lights {
		l := sc.Lights[i]
		if l.Kind == LightDirectional {
			if l.Dir != (Vec3{}) {
				dir = normalizeV3(l.Dir.Neg())
			}
			if l.Color.A != 0 {
				col = l.Color
			}
			if l.Intensity > 0 {
				amb = clampf(amb, 0.05, 0.95)
			}
			break
		}
	}
	return dir, col, amb
}

// Close 释放缓存（GPU 资源随设备关闭回收）。
func (s *GPUScene) Close() {
	if s == nil {
		return
	}
	s.pipes = map[gpuPipeKey]*Pipeline{}
	s.meshes = map[*Mesh]*gpuMeshRes{}
	s.texs = map[*Texture]*GPUTexture{}
	s.color, s.depth, s.fb = nil, nil, nil
}

// gpuPixel 读取 RGB 三元组的辅助（后处理测试用）。
func gpuPixel(img *image.RGBA, x, y int) RGBA {
	if img == nil {
		return RGBA{}
	}
	i := img.PixOffset(x, y)
	if i+3 >= len(img.Pix) {
		return RGBA{}
	}
	return RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]}
}
