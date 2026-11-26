// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build windows

package game

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

// OpenGL 硬件后端（Windows / WGL 离屏 FBO）。
//
// 设计取舍：
//   - 渲染全部发生在离线 FBO 上，结果经 glReadPixels 回读为 image.RGBA，
//     不改造 UI 合成路径，因此可随时与软件后端互换。
//   - 着色器需要 GLSL（内置程序都自带），没有 GLSL 的自定义程序会返回
//     ErrShaderNoGLSL。
//   - 计算内核没有 GLSL 时在 CPU 上降级执行（行为正确、不当成硬件加速）。

const (
	glDepthTest     = 0x0B71
	glCullFace      = 0x0B44
	glBlend         = 0x0BE2
	glBack          = 0x0405
	glFront         = 0x0404
	glLess          = 0x0201
	glLEqual        = 0x0203
	glGreater       = 0x0204
	glAlways        = 0x0207
	glSrcAlpha      = 0x0302
	glOneMinusSrcA  = 0x0303
	glOne           = 1
	glZero          = 0
	glDstColor      = 0x0306
	glTexture2D     = 0x0DE1
	glTexture0      = 0x84C0
	glRGBA          = 0x1908
	glRGBA8         = 0x8058
	glRGBA16F       = 0x881A
	glRGBA32F       = 0x8814
	glR8            = 0x8229
	glRed           = 0x1903
	glDepthComp32F  = 0x8CAC
	glDepth24Stenc  = 0x88F0
	glDepthAttach   = 0x8D00
	glColorAttach0  = 0x8CE0
	glFramebuffer   = 0x8D40
	glFBComplete    = 0x8CD5
	glColorBuffer   = 0x4000
	glDepthBuffer   = 0x0100
	glArrayBuffer   = 0x8892
	glElemenArray   = 0x8893
	glStaticDraw    = 0x88E4
	glFloat         = 0x1406
	glUnsignedByte  = 0x1401
	glUnsignedInt   = 0x1405
	glTriangles     = 0x0004
	glVertexShader  = 0x8B31
	glFragmentShdr  = 0x8B30
	glCompileStatus = 0x8B81
	glLinkStatus    = 0x8B82
	glInfoLogLen    = 0x8B84
	glTexMinFilter  = 0x2801
	glTexMagFilter  = 0x2800
	glTexWrapS      = 0x2802
	glTexWrapT      = 0x2803
	glLinear        = 0x2601
	glNearest       = 0x2600
	glLinearMipmap  = 0x2703
	glRepeat        = 0x2901
	glClampEdge     = 0x812F
	glFalse         = 0
	glTrue          = 1
	glVersion       = 0x1F02
	glRenderer      = 0x1F01
	glVendor        = 0x1F00
	glMaxTexSize    = 0x0D33
	glMaxColorAtt   = 0x8CDF
	glPackAlign     = 0x0D05
	glPolygonMode   = 0x0B40
	glFrontAndBack  = 0x0408
	glLine          = 0x1B01
	glFill          = 0x1B02
	glColorEnum     = 0x1800
	glDepthComponent = 0x1902
	glDepthStencil   = 0x84F9
	glUInt248        = 0x84FA
)

// glUnsignedInt248 GL_UNSIGNED_INT_24_8。
const glUnsignedInt248 = glUInt248

// glRequired GL 后端启动时必须解析到的函数。
var glRequired = []string{
	"glViewport", "glClearColor", "glClearDepth", "glClear", "glClearBufferfv",
	"glEnable", "glDisable", "glDepthFunc", "glDepthMask", "glCullFace",
	"glBlendFunc", "glPolygonMode",
	"glGenFramebuffers", "glBindFramebuffer", "glFramebufferTexture2D",
	"glDeleteFramebuffers", "glCheckFramebufferStatus",
	"glGenTextures", "glBindTexture", "glTexImage2D", "glTexParameteri",
	"glDeleteTextures", "glActiveTexture",
	"glGenBuffers", "glBindBuffer", "glBufferData", "glDeleteBuffers",
	"glGenVertexArrays", "glBindVertexArray",
	"glEnableVertexAttribArray", "glVertexAttribPointer", "glVertexAttribDivisor",
	"glCreateShader", "glShaderSource", "glCompileShader", "glGetShaderiv",
	"glGetShaderInfoLog", "glCreateProgram", "glAttachShader", "glLinkProgram",
	"glGetProgramiv", "glGetProgramInfoLog", "glUseProgram", "glDeleteShader",
	"glGetUniformLocation", "glUniform1i", "glUniform1f", "glUniform3f",
	"glUniform4f", "glUniform1fv", "glUniformMatrix4fv",
	"glDrawArrays", "glDrawElements", "glDrawArraysInstanced", "glDrawElementsInstanced",
	"glReadPixels", "glReadBuffer", "glPixelStorei", "glGetString", "glGetIntegerv", "glFinish",
	"glTexSubImage2D", "glDeleteVertexArrays",
}

// gpuGL OpenGL 后端。
type gpuGL struct {
	dev   *GPU
	ctx   *glContext
	vao   uint32
	fbIDs map[*GPUFramebuffer]uint32
	cap   GPUCaps
	stats GPUStats
	// curPipe 当前管线（uniform 位置来源）。
	curPipe *Pipeline
	// pinned 已把调用方 goroutine 钉在创建上下文的 OS 线程上（GL 上下文
	// 是线程局部绑定，goroutine 迁移会使 makeCurrent 失败）。
	pinned bool
}

func init() {
	RegisterGPUBackend("gl", newGPUGLBackend)
}

// newGPUGLBackend 创建 GL 后端；任何一步失败都返回错误（上层回退软件）。
func newGPUGLBackend(g *GPU) (gpuBackend, error) {
	glContextMu.Lock()
	defer glContextMu.Unlock()
	ctx, err := newGLContext()
	if err != nil {
		return nil, err
	}
	if err := ctx.load(glRequired); err != nil {
		ctx.destroy()
		return nil, err
	}
	verStr := ctx.str(glVersion)
	major := 0
	fmt.Sscanf(verStr, "%d", &major)
	if major < 3 {
		ctx.destroy()
		return nil, fmt.Errorf("game: OpenGL 版本过低（%s），需要 3.3+", verStr)
	}
	gl := &gpuGL{
		dev:   g,
		ctx:   ctx,
		fbIDs: map[*GPUFramebuffer]uint32{},
	}
	var maxTex, maxAtt int32
	ctx.call("glGetIntegerv", glMaxTexSize, uintptr(unsafe.Pointer(&maxTex)))
	ctx.call("glGetIntegerv", glMaxColorAtt, uintptr(unsafe.Pointer(&maxAtt)))
	if maxTex <= 0 {
		maxTex = 4096
	}
	if maxAtt <= 0 {
		maxAtt = 4
	}
	if maxAtt > 4 {
		maxAtt = 4
	}
	var vao uint32
	ctx.call("glGenVertexArrays", 1, uintptr(unsafe.Pointer(&vao)))
	if vao == 0 {
		ctx.destroy()
		return nil, errors.New("game: 创建 VAO 失败")
	}
	gl.vao = vao
	ctx.call("glBindVertexArray", uintptr(vao))
	ctx.call("glPixelStorei", glPackAlign, 1)
	gl.cap = GPUCaps{
		Backend:         "gl",
		Hardware:        true,
		MaxTextureSize:  int(maxTex),
		MaxColorTargets: int(maxAtt),
		MaxVertexAttrs:  16,
		Compute:         false,
		Instancing:      true,
		ShadowMap:       true,
		FloatTarget:     true,
		Threads:         0,
		Readback:        true,
	}
	// GL 上下文只对创建它的 OS 线程有效：把调用方 goroutine 钉在该线程上，
	// 否则 goroutine 迁移后 wglMakeCurrent 会因上下文已被旧线程占用而失败。
	runtime.LockOSThread()
	gl.pinned = true
	return gl, nil
}

func (g *gpuGL) name() string  { return "gl" }
func (g *gpuGL) caps() GPUCaps { return g.cap }

func (g *gpuGL) close() error {
	glContextMu.Lock()
	defer glContextMu.Unlock()
	if g.pinned {
		defer func() {
			g.pinned = false
			runtime.UnlockOSThread()
		}()
	}
	if g.ctx.makeCurrentNow() {
		for fb, id := range g.fbIDs {
			g.ctx.call("glDeleteFramebuffers", 1, uintptr(unsafe.Pointer(&id)))
			delete(g.fbIDs, fb)
		}
		if g.vao != 0 {
			g.ctx.call("glBindVertexArray", 0)
			g.ctx.call("glDeleteVertexArrays", 1, uintptr(unsafe.Pointer(&g.vao)))
			g.vao = 0
		}
	} else {
		// 上下文绑不回来（已被其它线程占用 / 已失效）：直接放弃 GL 侧清理，
		// 驱动会随进程或上下文销毁回收资源，但绝不能在无上下文时调 GL。
		g.fbIDs = map[*GPUFramebuffer]uint32{}
		g.vao = 0
	}
	g.ctx.destroy()
	return nil
}

// glInternalFormat 像素格式 → GL 内部格式 + 外部格式/类型。
func glInternalFormat(f PixFormat) (internal int, format int, typ int) {
	switch f {
	case PixRGBA16F:
		return glRGBA16F, glRGBA, glFloat
	case PixRGBA32F:
		return glRGBA32F, glRGBA, glFloat
	case PixR8:
		return glR8, glRed, glUnsignedByte
	case PixDepth32:
		// 深度纹理必须用 GL_DEPTH_COMPONENT 作为外部格式
		return glDepthComp32F, glDepthComponent, glFloat
	case PixDepth24Stencil8:
		return glDepth24Stenc, glDepthStencil, glUnsignedInt248
	}
	return glRGBA8, glRGBA, glUnsignedByte
}

func (g *gpuGL) newTexture(t *GPUTexture) error {
	glContextMu.Lock()
	defer glContextMu.Unlock()
	g.ctx.ensureCurrent()
	d := t.desc
	var id uint32
	g.ctx.call("glGenTextures", 1, uintptr(unsafe.Pointer(&id)))
	if id == 0 {
		return errors.New("game: glGenTextures 失败")
	}
	t.glID = id
	g.ctx.call("glActiveTexture", glTexture0)
	g.ctx.call("glBindTexture", glTexture2D, uintptr(id))
	internal, format, typ := glInternalFormat(d.Format)
	g.ctx.call("glTexImage2D", glTexture2D, 0, uintptr(internal),
		uintptr(d.W), uintptr(d.H), 0, uintptr(format), uintptr(typ), 0)
	// 过滤与环绕：RGBA8 采样纹理默认双线性 + 重复；渲染目标用最近邻避免越界采样
	minF, magF, wrap := glLinear, glLinear, glRepeat
	if !d.Usage.Has(TexSampled) {
		minF, magF, wrap = glNearest, glNearest, glClampEdge
	}
	g.ctx.call("glTexParameteri", glTexture2D, glTexMinFilter, uintptr(minF))
	g.ctx.call("glTexParameteri", glTexture2D, glTexMagFilter, uintptr(magF))
	g.ctx.call("glTexParameteri", glTexture2D, glTexWrapS, uintptr(wrap))
	g.ctx.call("glTexParameteri", glTexture2D, glTexWrapT, uintptr(wrap))
	// 上传已有 CPU 像素（软件后端创建后 SetPixels 写入的数据）
	if len(t.pix8) > 0 && len(t.pix8[0]) > 0 {
		g.uploadRGBA(t, t.pix8[0])
	}
	g.ctx.call("glBindTexture", glTexture2D, 0)
	return nil
}

// uploadRGBA 上传 RGBA8 像素。
func (g *gpuGL) uploadRGBA(t *GPUTexture, pix []RGBA) {
	if t == nil || t.glID == 0 || len(pix) == 0 {
		return
	}
	g.ctx.call("glActiveTexture", glTexture0)
	g.ctx.call("glBindTexture", glTexture2D, uintptr(t.glID))
	g.ctx.call("glTexSubImage2D", glTexture2D, 0, 0, 0,
		uintptr(t.desc.W), uintptr(t.desc.H), glRGBA, glUnsignedByte,
		uintptr(unsafe.Pointer(&pix[0])))
	g.ctx.call("glBindTexture", glTexture2D, 0)
}

func (g *gpuGL) newBuffer(b *GPUBuffer) error {
	glContextMu.Lock()
	defer glContextMu.Unlock()
	g.ctx.ensureCurrent()
	var id uint32
	g.ctx.call("glGenBuffers", 1, uintptr(unsafe.Pointer(&id)))
	if id == 0 {
		return errors.New("game: glGenBuffers 失败")
	}
	b.glID = id
	target := glArrayBuffer
	if b.desc.Usage.Has(BufIndex) {
		target = glElemenArray
	}
	g.ctx.call("glBindBuffer", uintptr(target), uintptr(id))
	var ptr uintptr
	if len(b.desc.Data) > 0 {
		buf := make([]byte, b.desc.Size)
		copy(buf, b.desc.Data)
		ptr = uintptr(unsafe.Pointer(&buf[0]))
	}
	g.ctx.call("glBufferData", uintptr(target), uintptr(b.desc.Size), ptr, glStaticDraw)
	g.ctx.call("glBindBuffer", uintptr(target), 0)
	b.data = make([]byte, b.desc.Size)
	return nil
}

func (g *gpuGL) compilePipeline(p *Pipeline) error {
	glContextMu.Lock()
	defer glContextMu.Unlock()
	g.ctx.ensureCurrent()
	prog := p.desc.Program
	if prog == nil {
		return errors.New("game: 管线缺少着色程序")
	}
	if prog.GLSL == nil {
		return ErrShaderNoGLSL
	}
	vs, err := g.compileShader(glVertexShader, prog.GLSL.Vertex)
	if err != nil {
		return fmt.Errorf("game: 顶点着色器编译失败: %w", err)
	}
	fs, err := g.compileShader(glFragmentShdr, prog.GLSL.Fragment)
	if err != nil {
		g.ctx.call("glDeleteShader", uintptr(vs))
		return fmt.Errorf("game: 片元着色器编译失败: %w", err)
	}
	pid := g.ctx.call("glCreateProgram")
	g.ctx.call("glAttachShader", pid, uintptr(vs))
	g.ctx.call("glAttachShader", pid, uintptr(fs))
	g.ctx.call("glLinkProgram", pid)
	var ok int32
	g.ctx.call("glGetProgramiv", pid, glLinkStatus, uintptr(unsafe.Pointer(&ok)))
	if ok == 0 {
		log := g.programLog(pid)
		g.ctx.call("glDeleteShader", uintptr(vs))
		g.ctx.call("glDeleteShader", uintptr(fs))
		return fmt.Errorf("game: 着色程序链接失败: %s", log)
	}
	g.ctx.call("glDeleteShader", uintptr(vs))
	g.ctx.call("glDeleteShader", uintptr(fs))
	p.glProgram = uint32(pid)
	// 解析常用 uniform 位置
	for _, n := range append([]string{
		"u_mvp", "u_model", "u_view", "u_proj", "u_tint", "u_hasTex",
		"u_lightDir", "u_lightColor", "u_ambient", "u_specular", "u_shininess", "u_tex",
	}, prog.GLSL.Uniforms...) {
		if _, ok := p.glAttrib[n]; ok {
			continue
		}
		if loc := g.uniformLoc(pid, n); loc >= 0 {
			p.glAttrib[n] = loc
		}
	}
	return nil
}

// compileShader 编译单个着色器。
func (g *gpuGL) compileShader(kind int, src string) (uint32, error) {
	if strings.TrimSpace(src) == "" {
		return 0, errors.New("源码为空")
	}
	sh := uint32(g.ctx.call("glCreateShader", uintptr(kind)))
	if sh == 0 {
		return 0, errors.New("glCreateShader 失败")
	}
	cs, err := bytePtr(src)
	if err != nil {
		return 0, err
	}
	g.ctx.call("glShaderSource", uintptr(sh), 1, uintptr(unsafe.Pointer(&cs)), 0)
	g.ctx.call("glCompileShader", uintptr(sh))
	var ok int32
	g.ctx.call("glGetShaderiv", uintptr(sh), glCompileStatus, uintptr(unsafe.Pointer(&ok)))
	if ok == 0 {
		var n int32
		g.ctx.call("glGetShaderiv", uintptr(sh), glInfoLogLen, uintptr(unsafe.Pointer(&n)))
		if n < 1 {
			n = 1
		}
		buf := make([]byte, n)
		g.ctx.call("glGetShaderInfoLog", uintptr(sh), uintptr(n), 0, uintptr(unsafe.Pointer(&buf[0])))
		g.ctx.call("glDeleteShader", uintptr(sh))
		return 0, errors.New(strings.TrimRight(string(buf), "\x00"))
	}
	return sh, nil
}

// programLog 取链接日志。
func (g *gpuGL) programLog(pid uintptr) string {
	var n int32
	g.ctx.call("glGetProgramiv", pid, glInfoLogLen, uintptr(unsafe.Pointer(&n)))
	if n < 1 {
		return ""
	}
	buf := make([]byte, n)
	g.ctx.call("glGetProgramInfoLog", pid, uintptr(n), 0, uintptr(unsafe.Pointer(&buf[0])))
	return strings.TrimRight(string(buf), "\x00")
}

// uniformLoc 取 uniform 位置（-1 表示不存在）。
func (g *gpuGL) uniformLoc(pid uintptr, name string) int32 {
	cs, err := bytePtr(name)
	if err != nil {
		return -1
	}
	return int32(g.ctx.call("glGetUniformLocation", pid, uintptr(unsafe.Pointer(cs))))
}

// bytePtr 字符串转 C 字符串指针。
func bytePtr(s string) (*byte, error) {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return &b[0], nil
}

// bindFramebuffer 绑定（惰性创建）FBO。
func (g *gpuGL) bindFramebuffer(fb *GPUFramebuffer) error {
	if fb == nil {
		return errors.New("game: 帧缓冲为空")
	}
	id, ok := g.fbIDs[fb]
	if !ok {
		var nid uint32
		g.ctx.call("glGenFramebuffers", 1, uintptr(unsafe.Pointer(&nid)))
		if nid == 0 {
			return errors.New("game: glGenFramebuffers 失败")
		}
		g.ctx.call("glBindFramebuffer", glFramebuffer, uintptr(nid))
		for i, c := range fb.Colors {
			g.ctx.call("glFramebufferTexture2D", glFramebuffer,
				uintptr(glColorAttach0+i), glTexture2D, uintptr(c.glID), 0)
		}
		if fb.Depth != nil {
			g.ctx.call("glFramebufferTexture2D", glFramebuffer, glDepthAttach,
				glTexture2D, uintptr(fb.Depth.glID), 0)
		}
		st := g.ctx.call("glCheckFramebufferStatus", glFramebuffer)
		if st != glFBComplete {
			g.ctx.call("glDeleteFramebuffers", 1, uintptr(unsafe.Pointer(&nid)))
			return fmt.Errorf("game: 帧缓冲不完整（0x%x）", st)
		}
		g.fbIDs[fb] = nid
		fb.glID = nid
		id = nid
	}
	g.ctx.call("glBindFramebuffer", glFramebuffer, uintptr(id))
	fb.glID = id
	return nil
}

// applyPipeline 应用固定渲染状态。
func (g *gpuGL) applyPipeline(p *Pipeline) {
	if p == nil || p.glProgram == 0 {
		return
	}
	g.curPipe = p
	d := p.desc
	g.ctx.call("glUseProgram", uintptr(p.glProgram))
	if d.DepthTest {
		g.ctx.call("glEnable", glDepthTest)
		fn := glLess
		switch d.DepthFunc {
		case DepthLEqual:
			fn = glLEqual
		case DepthGreater:
			fn = glGreater
		case DepthAlways:
			fn = glAlways
		}
		g.ctx.call("glDepthFunc", uintptr(fn))
	} else {
		g.ctx.call("glDisable", glDepthTest)
	}
	if d.DepthWrite {
		g.ctx.call("glDepthMask", glTrue)
	} else {
		g.ctx.call("glDepthMask", glFalse)
	}
	switch d.Cull {
	case CullNone:
		g.ctx.call("glDisable", glCullFace)
	case CullFront:
		g.ctx.call("glEnable", glCullFace)
		g.ctx.call("glCullFace", glFront)
	default:
		g.ctx.call("glEnable", glCullFace)
		g.ctx.call("glCullFace", glBack)
	}
	switch d.Blend {
	case BlendNone:
		g.ctx.call("glDisable", glBlend)
	case BlendAlpha:
		g.ctx.call("glEnable", glBlend)
		g.ctx.call("glBlendFunc", glSrcAlpha, glOneMinusSrcA)
	case BlendPremultiplied:
		g.ctx.call("glEnable", glBlend)
		g.ctx.call("glBlendFunc", glOne, glOneMinusSrcA)
	case BlendAdd:
		g.ctx.call("glEnable", glBlend)
		g.ctx.call("glBlendFunc", glOne, glOne)
	case BlendMultiply:
		g.ctx.call("glEnable", glBlend)
		g.ctx.call("glBlendFunc", glDstColor, glZero)
	}
	if d.Wireframe {
		g.ctx.call("glPolygonMode", glFrontAndBack, glLine)
	} else {
		g.ctx.call("glPolygonMode", glFrontAndBack, glFill)
	}
	// 纹理单元 0 绑定 u_tex
	if loc, ok := p.glAttrib["u_tex"]; ok {
		g.ctx.call("glUniform1i", uintptr(loc), 0)
	}
}

// setUniform 上传一个 uniform。
func (g *gpuGL) setUniform(p *Pipeline, name string, v []float32) {
	if p == nil || p.glProgram == 0 || len(v) == 0 {
		return
	}
	loc, ok := p.glAttrib[name]
	if !ok {
		if l := g.uniformLoc(uintptr(p.glProgram), name); l >= 0 {
			p.glAttrib[name] = l
			loc, ok = l, true
		}
	}
	if !ok || loc < 0 {
		return
	}
	switch len(v) {
	case 1:
		g.ctx.call("glUniform1f", uintptr(loc), uintptr(math.Float32bits(v[0])))
	case 3:
		g.ctx.call("glUniform3f", uintptr(loc),
			uintptr(math.Float32bits(v[0])), uintptr(math.Float32bits(v[1])), uintptr(math.Float32bits(v[2])))
	case 4:
		g.ctx.call("glUniform4f", uintptr(loc),
			uintptr(math.Float32bits(v[0])), uintptr(math.Float32bits(v[1])),
			uintptr(math.Float32bits(v[2])), uintptr(math.Float32bits(v[3])))
	case 16:
		// 项目 Mat4 为行主序（At(r,c) = m[r*4+c]），GL 需要列主序 → 上传转置
		g.ctx.call("glUniformMatrix4fv", uintptr(loc), 1, glTrue, uintptr(unsafe.Pointer(&v[0])))
	default:
		g.ctx.call("glUniform1fv", uintptr(loc), uintptr(len(v)), uintptr(unsafe.Pointer(&v[0])))
	}
	// u_hasTex 跟随槽 0 是否绑定
	if name == "u_tex" {
		if l, ok := p.glAttrib["u_hasTex"]; ok {
			g.ctx.call("glUniform1i", uintptr(l), 0)
		}
	}
}

// bindGLTexture 绑定纹理到槽位。
func (g *gpuGL) bindGLTexture(slot int, t *GPUTexture, p *Pipeline) {
	if slot < 0 || slot > 7 {
		return
	}
	g.ctx.call("glActiveTexture", uintptr(glTexture0+slot))
	if t == nil || t.glID == 0 {
		g.ctx.call("glBindTexture", glTexture2D, 0)
		if slot == 0 && p != nil {
			if l, ok := p.glAttrib["u_hasTex"]; ok {
				g.ctx.call("glUniform1i", uintptr(l), 0)
			}
		}
		return
	}
	g.ctx.call("glBindTexture", glTexture2D, uintptr(t.glID))
	if slot == 0 && p != nil {
		if l, ok := p.glAttrib["u_hasTex"]; ok {
			g.ctx.call("glUniform1i", uintptr(l), 1)
		}
	}
}

// clearColor 精确清空某个颜色附件。
func (g *gpuGL) clearColor(fb *GPUFramebuffer, target int, c RGBA) {
	if fb == nil || target < 0 || target >= len(fb.Colors) {
		return
	}
	col := [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
	g.ctx.call("glClearBufferfv", glColorEnum, uintptr(target), uintptr(unsafe.Pointer(&col[0])))
}

// clearDepth 清空深度附件。
func (g *gpuGL) clearDepth(v float32) {
	g.ctx.call("glDepthMask", glTrue)
	g.ctx.call("glClearDepth", uintptr(math.Float64bits(float64(v))))
	g.ctx.call("glClear", glDepthBuffer)
}

// bindVertexStream 配置一个顶点/实例流。
func (g *gpuGL) bindVertexStream(b *GPUBuffer, layout *VertexLayout, instanced bool) {
	if b == nil || layout == nil || b.glID == 0 {
		return
	}
	g.ctx.call("glBindBuffer", glArrayBuffer, uintptr(b.glID))
	for _, a := range layout.Attrs {
		if a.Location < 0 || a.Location > 15 {
			continue
		}
		typ, norm := glFloat, glFalse
		if a.Format == VU8x4N {
			typ, norm = glUnsignedByte, glTrue
		}
		g.ctx.call("glEnableVertexAttribArray", uintptr(a.Location))
		g.ctx.call("glVertexAttribPointer", uintptr(a.Location), uintptr(a.Format.Count()),
			uintptr(typ), uintptr(norm), uintptr(layout.Stride), uintptr(a.Offset))
		if instanced {
			g.ctx.call("glVertexAttribDivisor", uintptr(a.Location), 1)
		} else {
			g.ctx.call("glVertexAttribDivisor", uintptr(a.Location), 0)
		}
	}
}

// submit 执行命令列表。
func (g *gpuGL) submit(c *GPUCommands) error {
	if c.fb == nil {
		return errors.New("game: 提交前需要先 SetFramebuffer")
	}
	glContextMu.Lock()
	defer glContextMu.Unlock()
	g.ctx.ensureCurrent()
	start := time.Now()
	if err := g.bindFramebuffer(c.fb); err != nil {
		return err
	}
	g.ctx.call("glViewport", 0, 0, uintptr(c.fb.W), uintptr(c.fb.H))

	var (
		pipe     *Pipeline
		vb, ib   *GPUBuffer
		inst     *GPUBuffer
		vlayout  *VertexLayout
		instLay  *VertexLayout
		st       GPUStats
		texBound [8]bool
	)
	_ = texBound
	for i := range c.ops {
		op := &c.ops[i]
		switch op.kind {
		case gpuCmdSetPipeline:
			pipe = op.pipeline
			g.applyPipeline(pipe)
		case gpuCmdSetVertex:
			vb, vlayout = op.buffer, op.layout
		case gpuCmdSetInstance:
			inst, instLay = op.buffer, op.layout
		case gpuCmdSetIndex:
			ib = op.buffer
		case gpuCmdSetUniform:
			g.setUniform(pipe, op.name, op.floats)
		case gpuCmdSetTexture:
			g.bindGLTexture(op.slot, op.tex, pipe)
		case gpuCmdSetViewport:
			g.ctx.call("glViewport", uintptr(op.viewport[0]), uintptr(op.viewport[1]),
				uintptr(op.viewport[2]), uintptr(op.viewport[3]))
		case gpuCmdClearColor:
			g.clearColor(c.fb, op.target, op.color)
			st.Clears++
		case gpuCmdClearDepth:
			g.clearDepth(op.depth)
			st.Clears++
		case gpuCmdCopyTexture:
			// CPU 侧拷贝（软硬前端语义一致，避免 PBO 复杂度）
			if op.dst != nil && op.src != nil && len(op.dst.pix8) > 0 && len(op.src.pix8) > 0 {
				copy(op.dst.pix8[0], op.src.pix8[0])
				g.uploadRGBA(op.dst, op.dst.pix8[0])
			}
		case gpuCmdDraw:
			if pipe == nil || vb == nil || vlayout == nil {
				continue
			}
			g.ctx.call("glBindVertexArray", uintptr(g.vao))
			g.bindVertexStream(vb, vlayout, false)
			if inst != nil && instLay != nil {
				g.bindVertexStream(inst, instLay, true)
			}
			if op.indexed && ib != nil && ib.glID != 0 {
				g.ctx.call("glBindBuffer", glElemenArray, uintptr(ib.glID))
			}
			nInst := op.instances
			if nInst <= 0 {
				nInst = 1
			}
			if op.indexed {
				if nInst > 1 {
					g.ctx.call("glDrawElementsInstanced", glTriangles, uintptr(op.count),
						glUnsignedInt, 0, uintptr(nInst))
				} else {
					g.ctx.call("glDrawElements", glTriangles, uintptr(op.count), glUnsignedInt, 0)
				}
			} else {
				if nInst > 1 {
					g.ctx.call("glDrawArraysInstanced", glTriangles, 0, uintptr(op.count), uintptr(nInst))
				} else {
					g.ctx.call("glDrawArrays", glTriangles, 0, uintptr(op.count))
				}
			}
			st.DrawCalls++
			st.Instances += nInst
			st.Vertices += op.count * nInst
			if op.indexed {
				st.Triangles += op.count / 3 * nInst
			} else {
				st.Triangles += op.count / 3 * nInst
			}
		case gpuCmdDispatch:
			// GL 后端不跑 compute（内核是 Go 闭包）：在 CPU 上降级执行
			if op.kernel != nil && op.kernel.Run != nil {
				for z := 0; z < op.groups[2]; z++ {
					for y := 0; y < op.groups[1]; y++ {
						for x := 0; x < op.groups[0]; x++ {
							op.kernel.Run(x, y, z)
						}
					}
				}
				st.Dispatches++
			}
		}
	}
	g.ctx.call("glFinish")
	st.SubmitMs = float64(time.Since(start).Microseconds()) / 1000
	g.dev.mu.Lock()
	agg := g.dev.stats
	agg.DrawCalls += st.DrawCalls
	agg.Instances += st.Instances
	agg.Vertices += st.Vertices
	agg.Triangles += st.Triangles
	agg.Clears += st.Clears
	agg.Dispatches += st.Dispatches
	agg.SubmitMs = st.SubmitMs
	agg.TotalMs += st.SubmitMs
	g.dev.stats = agg
	g.dev.mu.Unlock()
	return nil
}

// readPixels 从绑定的 FBO 回读颜色。
func (g *gpuGL) readPixels(fb *GPUFramebuffer, target int, dst []RGBA) error {
	glContextMu.Lock()
	defer glContextMu.Unlock()
	g.ctx.ensureCurrent()
	if fb == nil || target < 0 || target >= len(fb.Colors) {
		return errors.New("game: 帧缓冲或颜色目标序号非法")
	}
	if err := g.bindFramebuffer(fb); err != nil {
		return err
	}
	g.ctx.call("glPixelStorei", glPackAlign, 1)
	g.ctx.call("glReadBuffer", uintptr(glColorAttach0+target))
	buf := make([]byte, fb.W*fb.H*4)
	g.ctx.call("glReadPixels", 0, 0, uintptr(fb.W), uintptr(fb.H),
		glRGBA, glUnsignedByte, uintptr(unsafe.Pointer(&buf[0])))
	// GL 原点在左下，图片原点在左上 → 垂直翻转
	for y := 0; y < fb.H; y++ {
		src := (fb.H - 1 - y) * fb.W * 4
		for x := 0; x < fb.W; x++ {
			i := src + x*4
			dst[y*fb.W+x] = RGBA{buf[i], buf[i+1], buf[i+2], buf[i+3]}
		}
	}
	return nil
}
