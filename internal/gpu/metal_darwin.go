//go:build darwin && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"

	"guis/internal/objc"
)

// macOS Metal 后端（零 cgo，purego + Objective-C 运行时）。
//
// 设计：**不改渲染器**。internal/gpu 的 Renderer 只认识 glXxx 这组薄包装，
// 因此这里把同一组 glXxx 直接映射到 Metal：
//
//	GL 概念            → Metal
//	--------------------------------------------------------------
//	VBO/VAO/attrib     → MTLBuffer + MTLVertexDescriptor
//	program/uniform    → 预置 MSL 管线 + setVertexBytes/setFragmentBytes
//	texture (R8/RGBA)  → r8Unorm / rgba8Unorm MTLTexture（Shared）
//	FBO + color attach → 同一张 MTLTexture 作为 render pass 的 color attachment
//	glCopyTexSubImage2D→ 全屏四边形 + 拷贝管线渲染进目标纹理
//	glScissor / Blend  → encoder 的 scissorRect / 管线混合状态
//	Swap               → CAMetalLayer nextDrawable + presentDrawable
//
// 这样圆角/SDF、字形图集、图片、毛玻璃（Kawase 多级降采样）、组件栅格缓存
// 与整帧超采样（Resolve 降采样）全部原样复用，Metal 与 GL 输出一致。
//
// 坐标与纹理方向：渲染器沿用 GL 语义（屏幕 y 向下、离屏目标第 0 行 = 屏幕
// 底部）。Metal 的视口/纹理第 0 行都在上方，因此：
//   - 顶点变换保留 GLSL 的 y 取反，写入离屏目标时内存序即「上方在前」；
//   - 采样离屏目标的着色器统一做 v = 1 - v（见 metal_shaders_darwin.go）；
//   - 屏幕上屏时 Resolve/直绘都遵循同一约定，最终画面与 GL 完全一致。

// MTLPixelFormat
const (
	mtlFormatR8    = 10
	mtlFormatRGBA8 = 70
)

// MTLBlendFactor
const (
	mtlBlendZero                = 0
	mtlBlendOne                 = 1
	mtlBlendSourceAlpha         = 4
	mtlBlendOneMinusSourceAlpha = 5
)

// 管线种类（与 MSL 函数一一对应）
const (
	pipeShape = iota
	pipeText
	pipeImage
	pipeBlur
	pipeResolve
	pipeCopy
	pipeSolid
	pipeCount
)

// MTLViewport / MTLRegion / CGSize（按值传结构体）
type mtlViewport struct {
	X, Y, W, H float64
	Znear, Zfar float64
}

type mtlOrigin struct{ X, Y, Z uint64 }
type mtlSize3 struct{ W, H, D uint64 }
type mtlRegion struct {
	Origin mtlOrigin
	Size   mtlSize3
}
type cgSize struct{ W, H float64 }

// metalCtx 一次 Metal 渲染会话的全部原生对象与状态。
type metalCtx struct {
	dev   uintptr
	queue uintptr
	layer uintptr
	view  uintptr

	lib   uintptr
	pipes [pipeCount][2]uintptr
	smpl  uintptr

	// 每帧命令缓冲与当前 encoder
	cmd      uintptr
	enc      uintptr
	drawable uintptr
	drawTex  uintptr
	encoded  map[uint32]bool

	pw, ph int32
	scale  float64

	// GL 对象表
	bufs    map[uint32]*mtlBuf
	texs    map[uint32]*mtlTex
	vaos    map[uint32]*mtlVAO
	fbos    map[uint32]uint32
	shaders map[uint32]*mtlShader
	progs   map[uint32]*mtlProg
	nextID  uint32

	curBuf, curVAO, curTex, curProg uint32
	curFBO                          uint32 // 当前绑定的 FBO（0 = 默认帧缓冲）

	// GL 状态
	scissorOn               bool
	scX, scY, scW, scH      int32
	vpX, vpY, vpW, vpH      int32
	blendOn                 bool
	clearR, clearG, clearB, clearA float32

	uni metalUniforms

	// 清屏用的全屏四边形（NDC）
	quadBuf uintptr
}

// metalUniforms 与 MSL 的 Uniforms 完全同布局（float4 需 16 字节对齐）。
type metalUniforms struct {
	VP [2]float32
	_  [2]float32
	U  [4]float32
}

// mtlScissorRect MTLScissorRect（4 × NSUInteger）。
type mtlScissorRect struct{ X, Y, W, H uint64 }

type mtlBuf struct {
	buf  uintptr
	size int
}

type mtlTex struct {
	tex  uintptr
	w, h int
	r8   bool
	// rt 是否为离屏渲染目标（GL 语义：第 0 行在屏幕底部，采样需翻 v）。
	rt bool
}

type mtlAttr struct {
	loc, size, off int
}

type mtlVAO struct {
	attrs  []mtlAttr
	stride int
	buf    uint32
}

type mtlShader struct {
	typ  uint32
	src  string
	info string
}

type mtlProg struct {
	kind int
	loc  map[int32]string
	n    int32
}

var (
	metalOnce  sync.Once
	metalErr   error
	metalPtr   *metalCtx
	metalReady bool

	// 结构体参数的消息签名（purego 支持 darwin 上的按值结构体）
	sendSetViewport   func(uintptr, uintptr, mtlViewport) uintptr
	sendReplaceRegion func(uintptr, uintptr, mtlRegion, uintptr, uintptr, uintptr) uintptr
	sendSetDrawable   func(uintptr, uintptr, cgSize) uintptr
	sendSetScissor    func(uintptr, uintptr, mtlScissorRect) uintptr
)

// MetalActive 当前是否已启用 Metal 渲染。
func MetalActive() bool { return metalReady }

// MetalAvailable 当前机器是否支持 Metal（只探测设备，不建图层）。
func MetalAvailable() bool {
	initMetal()
	return metalErr == nil
}

// MetalInitError 初始化失败原因（无失败时返回 nil）。
func MetalInitError() error { return metalErr }

// initMetal 加载 Metal.framework 并创建默认设备（幂等）。
func initMetal() {
	metalOnce.Do(func() {
		if err := objc.Load(); err != nil {
			metalErr = err
			return
		}
		// QuartzCore 提供 CAMetalLayer
		_, _ = purego.Dlopen("/System/Library/Frameworks/QuartzCore.framework/QuartzCore", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		h, err := purego.Dlopen("/System/Library/Frameworks/Metal.framework/Metal", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			metalErr = fmt.Errorf("metal: 无法加载 Metal.framework: %w", err)
			return
		}
		var create func() uintptr
		if addr, e := purego.Dlsym(h, "MTLCreateSystemDefaultDevice"); e == nil && addr != 0 {
			purego.RegisterFunc(&create, addr)
		}
		if create == nil {
			metalErr = fmt.Errorf("metal: 缺少 MTLCreateSystemDefaultDevice")
			return
		}
		dev := create()
		if dev == 0 {
			metalErr = fmt.Errorf("metal: 该机器没有可用的 Metal 设备")
			return
		}
		// objc 侧结构体签名
		purego.RegisterFunc(&sendSetViewport, objc.MsgSend())
		purego.RegisterFunc(&sendReplaceRegion, objc.MsgSend())
		purego.RegisterFunc(&sendSetDrawable, objc.MsgSend())
		purego.RegisterFunc(&sendSetScissor, objc.MsgSend())

		m := &metalCtx{
			dev:     dev,
			encoded: map[uint32]bool{},
			bufs:    map[uint32]*mtlBuf{},
			texs:    map[uint32]*mtlTex{},
			vaos:    map[uint32]*mtlVAO{},
			fbos:    map[uint32]uint32{},
			shaders: map[uint32]*mtlShader{},
			progs:   map[uint32]*mtlProg{},
			scale:   1,
			blendOn: true,
		}
		m.queue = objc.Send(dev, "newCommandQueue")
		if m.queue == 0 {
			metalErr = fmt.Errorf("metal: 无法创建 MTLCommandQueue")
			return
		}
		metalPtr = m
		metalReady = true
		installMetalGL()
	})
}

// MetalAttach 在 NSView 上安装 CAMetalLayer 并设为当前 Metal 上下文。
func MetalAttach(nsview uintptr, scale float64) error {
	initMetal()
	if metalErr != nil {
		return metalErr
	}
	m := metalPtr
	if m == nil {
		return fmt.Errorf("metal: 上下文未初始化")
	}
	if scale <= 0 {
		scale = 1
	}
	m.view = nsview
	m.scale = scale

	cls := objc.Class("CAMetalLayer")
	if cls == 0 {
		return fmt.Errorf("metal: 找不到 CAMetalLayer（QuartzCore 未加载）")
	}
	layer := objc.Send(objc.Send(cls, "alloc"), "init")
	if layer == 0 {
		return fmt.Errorf("metal: CAMetalLayer 创建失败")
	}
	objc.Send1(layer, "setDevice:", m.dev)
	objc.Send1(layer, "setPixelFormat:", mtlFormatRGBA8)
	objc.SendB(layer, "setFramebufferOnly:", false)
	objc.SendB(layer, "setOpaque:", true)
	objc.SendF(layer, "setContentsScale:", scale)
	if nsview != 0 {
		objc.SendB(nsview, "setWantsLayer:", true)
		objc.Send1(nsview, "setLayer:", layer)
	}
	m.layer = layer
	if nsview != 0 {
		b := objc.RectOf(nsview, "bounds")
		m.SetDrawableSize(int32(b.Size.W*scale+0.5), int32(b.Size.H*scale+0.5))
	}
	return nil
}

// MetalDetach 卸载 Metal 上下文（窗口销毁时调用）。
func MetalDetach() {
	if metalPtr == nil {
		return
	}
	MetalPresent()
	metalPtr.layer = 0
	metalPtr.view = 0
}

// MetalSetScale 更新 Retina 比例。
func MetalSetScale(scale float64) {
	if metalPtr != nil && scale > 0 {
		metalPtr.scale = scale
		if metalPtr.layer != 0 {
			objc.SendF(metalPtr.layer, "setContentsScale:", scale)
		}
	}
}

// SetDrawableSize 设置 drawable 像素尺寸。
func (m *metalCtx) SetDrawableSize(pw, ph int32) {
	if pw < 1 {
		pw = 1
	}
	if ph < 1 {
		ph = 1
	}
	if m.pw == pw && m.ph == ph {
		return
	}
	m.pw, m.ph = pw, ph
	if m.layer != 0 && sendSetDrawable != nil {
		sendSetDrawable(m.layer, objc.Sel("setDrawableSize:"), cgSize{W: float64(pw), H: float64(ph)})
	}
}

// MetalSetDrawableSize 由窗口后端同步 drawable 像素尺寸（Retina / 缩放变化时调用）。
func MetalSetDrawableSize(pw, ph int32) {
	if metalPtr != nil {
		metalPtr.SetDrawableSize(pw, ph)
	}
}

// MetalPresent 结束本帧：presentDrawable + commit。
func MetalPresent() {
	m := metalPtr
	if m == nil {
		return
	}
	m.endEncoder()
	if m.cmd != 0 {
		if m.drawable != 0 {
			objc.Send1(m.cmd, "presentDrawable:", m.drawable)
		}
		objc.Send(m.cmd, "commit")
	}
	m.cmd, m.drawable, m.drawTex = 0, 0, 0
	m.encoded = map[uint32]bool{}
	m.curFBO = 0
}

// SetVSync 垂直同步开关（CAMetalLayer.displaySyncEnabled）。
func SetVSync(on bool) {
	if metalPtr != nil && metalPtr.layer != 0 {
		objc.SendB(metalPtr.layer, "setDisplaySyncEnabled:", on)
	}
}

// ---- 内部对象与管线 ----

func (m *metalCtx) endEncoder() {
	if m.enc != 0 {
		objc.Send(m.enc, "endEncoding")
		m.enc = 0
	}
}

// targetTexture 当前绘制目标纹理（FBO 0 → drawable）。
func (m *metalCtx) targetTexture() uintptr {
	if m.curFBO == 0 {
		if m.drawTex == 0 {
			if m.layer == 0 {
				return 0
			}
			m.drawable = objc.Send(m.layer, "nextDrawable")
			if m.drawable == 0 {
				return 0
			}
			m.drawTex = objc.Send(m.drawable, "texture")
		}
		return m.drawTex
	}
	tid := m.fbos[m.curFBO]
	if t, ok := m.texs[tid]; ok {
		return t.tex
	}
	return 0
}

// targetHeight 当前目标的像素高度（scissor / viewport 换算用）。
func (m *metalCtx) targetHeight() int32 {
	if m.curFBO == 0 {
		if m.drawTex != 0 {
			if h := objc.IntOfN(m.drawTex, "height"); h > 0 {
				return int32(h)
			}
		}
		return m.ph
	}
	if t, ok := m.texs[m.fbos[m.curFBO]]; ok {
		return int32(t.h)
	}
	return m.ph
}

// beginPass 为目标纹理开一个 render pass。
func (m *metalCtx) beginPass(tex uintptr, load bool) uintptr {
	if tex == 0 {
		return 0
	}
	desc := objc.Send(objc.Send(objc.Class("MTLRenderPassDescriptor"), "alloc"), "init")
	ca := objc.Send1(desc, "objectAtIndexedSubscript:", 0)
	objc.Send1(ca, "setTexture:", tex)
	if load {
		objc.SendI(ca, "setLoadAction:", 1) // MTLLoadActionLoad
	} else {
		objc.SendI(ca, "setLoadAction:", 2) // MTLLoadActionClear
	}
	objc.SendI(ca, "setStoreAction:", 1) // MTLStoreActionStore
	return objc.Send1(m.cmd, "renderCommandEncoderWithDescriptor:", desc)
}

// encoder 取当前 encoder（必要时按当前目标开启 pass）。
func (m *metalCtx) encoder() uintptr {
	if m.enc != 0 {
		return m.enc
	}
	if m.cmd == 0 {
		m.cmd = objc.Send(m.queue, "commandBuffer")
		if m.cmd == 0 {
			return 0
		}
	}
	tex := m.targetTexture()
	if tex == 0 {
		return 0
	}
	load := m.encoded[m.curFBO]
	m.enc = m.beginPass(tex, load)
	m.encoded[m.curFBO] = true
	return m.enc
}

// library 懒编译 MSL（只做一次）。
func (m *metalCtx) library() uintptr {
	if m.lib != 0 {
		return m.lib
	}
	src := objc.NSString(metalShaderSource)
	var errObj uintptr
	lib := objc.Send3(m.dev, "newLibraryWithSource:options:error:", src, 0, uintptr(unsafe.Pointer(&errObj)))
	if lib == 0 {
		msg := "unknown"
		if errObj != 0 {
			if d := objc.Send(errObj, "localizedDescription"); d != 0 {
				msg = objc.UTF8(d)
			}
		}
		fmt.Fprintf(os.Stderr, "[guis] metal: MSL 编译失败: %s\n", msg)
		return 0
	}
	m.lib = lib
	m.smpl = objc.Send(objc.Send(objc.Class("MTLSamplerDescriptor"), "alloc"), "init")
	if m.smpl != 0 {
		objc.SendI(m.smpl, "setMinFilter:", 1) // linear
		objc.SendI(m.smpl, "setMagFilter:", 1)
		objc.SendI(m.smpl, "setSAddressMode:", 0)
		objc.SendI(m.smpl, "setTAddressMode:", 0)
		m.smpl = objc.Send1(m.dev, "newSamplerStateWithDescriptor:", m.smpl)
	}
	return m.lib
}

// vfn/fn 名称表
func pipeNames(kind int) (string, string) {
	switch kind {
	case pipeShape:
		return "v_shape", "f_shape"
	case pipeText:
		return "v_text", "f_text"
	case pipeImage:
		return "v_image", "f_image"
	case pipeBlur:
		return "v_quad", "f_blur"
	case pipeResolve:
		return "v_quad", "f_resolve"
	case pipeCopy:
		return "v_copy", "f_copy"
	case pipeSolid:
		return "v_copy", "f_solid"
	}
	return "", ""
}

// pipeAttrs 各管线的顶点布局（nil = 走 setVertexBytes 的 copy/solid）。
func pipeAttrs(kind int) ([]mtlAttr, int) {
	switch kind {
	case pipeShape:
		return []mtlAttr{{0, 2, 0}, {1, 2, 8}, {2, 4, 16}, {3, 4, 32}, {4, 2, 48}, {5, 4, 56}, {6, 4, 72}}, 88
	case pipeText:
		return []mtlAttr{{0, 2, 0}, {1, 2, 8}, {2, 4, 16}, {3, 1, 32}}, 36
	case pipeImage:
		return []mtlAttr{{0, 2, 0}, {1, 2, 8}, {2, 4, 16}, {3, 2, 32}, {4, 2, 40}, {5, 4, 48}}, 64
	case pipeBlur, pipeResolve:
		return []mtlAttr{{0, 2, 0}, {1, 2, 8}}, 16
	}
	return nil, 0
}

// vertexDescriptor 构造 MTLVertexDescriptor。
func vertexDescriptor(attrs []mtlAttr, stride int) uintptr {
	if len(attrs) == 0 {
		return 0
	}
	vd := objc.Send(objc.Send(objc.Class("MTLVertexDescriptor"), "alloc"), "init")
	as := objc.Send(vd, "attributes")
	for _, a := range attrs {
		ad := objc.Send1(as, "objectAtIndexedSubscript:", uintptr(a.loc))
		format := 28 // float
		switch a.size {
		case 2:
			format = 29 // float2
		case 3:
			format = 30 // float3
		case 4:
			format = 31 // float4
		}
		objc.SendI(ad, "setFormat:", int64(format))
		objc.SendI(ad, "setOffset:", int64(a.off))
		objc.SendI(ad, "setBufferIndex:", 0)
	}
	ld := objc.Send1(objc.Send(vd, "layouts"), "objectAtIndexedSubscript:", 0)
	objc.SendI(ld, "setStride:", int64(stride))
	objc.SendI(ld, "setStepFunction:", 1) // per-vertex
	return vd
}

// boolIdx 布尔 → 0/1（管线缓存下标）。
func boolIdx(b bool) int {
	if b {
		return 1
	}
	return 0
}

// pipeline 取（必要时创建）管线状态；blend = 是否开启
// SRC_ALPHA/ONE_MINUS_SRC_ALPHA 混合（Resolve / 拷贝 / 清屏不混合）。
func (m *metalCtx) pipeline(kind int, blend bool) uintptr {
	if kind < 0 || kind >= pipeCount {
		return 0
	}
	if kind == pipeCopy || kind == pipeSolid {
		blend = false
	}
	bi := 0
	if blend {
		bi = 1
	}
	if p := m.pipes[kind][bi]; p != 0 {
		return p
	}
	lib := m.library()
	if lib == 0 {
		return 0
	}
	vname, fname := pipeNames(kind)
	vf := objc.Send1(lib, "newFunctionWithName:", objc.NSString(vname))
	ff := objc.Send1(lib, "newFunctionWithName:", objc.NSString(fname))
	if vf == 0 || ff == 0 {
		fmt.Fprintf(os.Stderr, "[guis] metal: 找不到着色器函数 %s/%s\n", vname, fname)
		return 0
	}
	pd := objc.Send(objc.Send(objc.Class("MTLRenderPipelineDescriptor"), "alloc"), "init")
	objc.Send1(pd, "setVertexFunction:", vf)
	objc.Send1(pd, "setFragmentFunction:", ff)
	ca := objc.Send1(objc.Send(pd, "colorAttachments"), "objectAtIndexedSubscript:", 0)
	objc.SendI(ca, "setPixelFormat:", mtlFormatRGBA8)
	// 与 GL 的 glBlendFunc(SRC_ALPHA, ONE_MINUS_SRC_ALPHA) 等价；拷贝/清屏不混合。
	if kind != pipeCopy && kind != pipeSolid && blend {
		objc.SendB(ca, "setBlendingEnabled:", true)
		objc.SendI(ca, "setSourceRGBBlendFactor:", mtlBlendSourceAlpha)
		objc.SendI(ca, "setDestinationRGBBlendFactor:", mtlBlendOneMinusSourceAlpha)
		objc.SendI(ca, "setSourceAlphaBlendFactor:", mtlBlendSourceAlpha)
		objc.SendI(ca, "setDestinationAlphaBlendFactor:", mtlBlendOneMinusSourceAlpha)
	}
	if attrs, stride := pipeAttrs(kind); len(attrs) > 0 {
		objc.Send1(pd, "setVertexDescriptor:", vertexDescriptor(attrs, stride))
	}
	var errObj uintptr
	pipe := objc.Send2(m.dev, "newRenderPipelineStateWithDescriptor:error:", pd, uintptr(unsafe.Pointer(&errObj)))
	if pipe == 0 {
		msg := "unknown"
		if errObj != 0 {
			if d := objc.Send(errObj, "localizedDescription"); d != 0 {
				msg = objc.UTF8(d)
			}
		}
		fmt.Fprintf(os.Stderr, "[guis] metal: 管线 %s 创建失败: %s\n", vname, msg)
		return 0
	}
	m.pipes[kind][bi] = pipe
	return pipe
}

// newTexture 创建一张 Metal 纹理（usage 含 shaderRead|renderTarget，Shared 存储）。
func (m *metalCtx) newTexture(w, h int, r8 bool, data []byte) uintptr {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	fmtv := uintptr(mtlFormatRGBA8)
	if r8 {
		fmtv = mtlFormatR8
	}
	desc := objc.Send4(objc.Class("MTLTextureDescriptor"), "texture2DDescriptorWithPixelFormat:width:height:mipmapped:",
		fmtv, uintptr(w), uintptr(h), 0)
	objc.SendI(desc, "setUsage:", 5) // shaderRead | renderTarget
	objc.SendI(desc, "setStorageMode:", 0)
	tex := objc.Send1(m.dev, "newTextureWithDescriptor:", desc)
	if tex == 0 {
		return 0
	}
	if len(data) > 0 {
		m.replace(tex, w, h, r8, 0, 0, w, h, data)
	}
	return tex
}

// replace 上传像素到纹理区域（GL 的 TexImage2D/TexSubImage2D）。
func (m *metalCtx) replace(tex uintptr, w, h int, r8 bool, x, y, rw, rh int, data []byte) {
	if tex == 0 || sendReplaceRegion == nil {
		return
	}
	bpp := 4
	if r8 {
		bpp = 1
	}
	need := rw * rh * bpp
	if len(data) < need {
		return
	}
	reg := mtlRegion{
		Origin: mtlOrigin{X: uint64(x), Y: uint64(y), Z: 0},
		Size:   mtlSize3{W: uint64(rw), H: uint64(rh), D: 1},
	}
	sendReplaceRegion(tex, objc.Sel("replaceRegion:mipmapLevel:withBytes:bytesPerRow:"),
		reg, 0, uintptr(unsafe.Pointer(&data[0])), uintptr(rw*bpp))
}

// drawQuad 用拷贝管线把源纹理的一段画进目标纹理（glCopyTexSubImage2D）。
func (m *metalCtx) drawCopy(src uintptr, srcW, srcH int, sx, sy, sw, sh int, dst uintptr, dstW, dstH int, dx, dy int) {
	if src == 0 || dst == 0 || sw <= 0 || sh <= 0 || m.cmd == 0 {
		return
	}
	if pipe := m.pipeline(pipeCopy, false); pipe == 0 {
		return
	}
	m.endEncoder()
	m.enc = m.beginPass(dst, false)
	if m.enc == 0 {
		return
	}
	objc.Send1(m.enc, "setRenderPipelineState:", m.pipes[pipeCopy][0])
	if m.smpl != 0 {
		objc.Send2(m.enc, "setFragmentSamplerState:atIndex:", m.smpl, 0)
	}
	objc.Send2(m.enc, "setFragmentTexture:atIndex:", src, 0)

	x0 := float32(dx) / float32(dstW) * 2
	y0 := float32(dy) / float32(dstH) * 2
	x1 := float32(dx+sw) / float32(dstW) * 2
	y1 := float32(dy+sh) / float32(dstH) * 2
	x0, y0, x1, y1 = x0-1, y0-1, x1-1, y1-1
	u0 := float32(sx) / float32(srcW)
	v0 := float32(sy) / float32(srcH)
	u1 := float32(sx+sw) / float32(srcW)
	v1 := float32(sy+sh) / float32(srcH)

	verts := [6][4]float32{
		{x0, y0, 0, 0}, {x1, y0, 1, 0}, {x1, y1, 1, 1},
		{x0, y0, 0, 0}, {x1, y1, 1, 1}, {x0, y1, 0, 1},
	}
	m.uni.U = [4]float32{u0, v0, u1 - u0, v1 - v0}
	objc.Send3(m.enc, "setVertexBytes:length:atIndex:", uintptr(unsafe.Pointer(&verts[0])), uintptr(len(verts)*16), 0)
	objc.Send3(m.enc, "setFragmentBytes:length:atIndex:", uintptr(unsafe.Pointer(&m.uni)), unsafe.Sizeof(m.uni), 1)
	objc.Send3(m.enc, "drawPrimitives:vertexStart:vertexCount:", 3, 0, 6)
	m.endEncoder()
}

// clearSolid 用纯色管线覆盖当前目标（GL 的 glClear）。
func (m *metalCtx) clearSolid() {
	enc := m.encoder()
	if enc == 0 {
		return
	}
	if m.pipeline(pipeSolid, false) == 0 {
		return
	}
	objc.Send1(enc, "setRenderPipelineState:", m.pipes[pipeSolid][0])
	h := m.targetHeight()
	if h < 1 {
		h = 1
	}
	w := m.targetW()
	if w < 1 {
		w = 1
	}
	m.setViewportInternal(0, 0, w, h)
	// 关闭 scissor，整屏覆盖
	setScissor(enc, 0, 0, w, h, h)
	quad := [6][4]float32{
		{-1, -1, 0, 0}, {1, -1, 0, 0}, {1, 1, 0, 0},
		{-1, -1, 0, 0}, {1, 1, 0, 0}, {-1, 1, 0, 0},
	}
	objc.Send3(enc, "setVertexBytes:length:atIndex:", uintptr(unsafe.Pointer(&quad[0])), uintptr(len(quad)*16), 0)
	m.uni.U = [4]float32{m.clearR, m.clearG, m.clearB, m.clearA}
	objc.Send3(enc, "setFragmentBytes:length:atIndex:", uintptr(unsafe.Pointer(&m.uni)), unsafe.Sizeof(m.uni), 1)
	objc.Send3(enc, "drawPrimitives:vertexStart:vertexCount:", 3, 0, 6)
}
