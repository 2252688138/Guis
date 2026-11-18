//go:build darwin && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

import (
	"unsafe"

	"guis/internal/objc"
)

// Metal 的 GL 调用表：把 glFns 的 51 个入口逐一映射到 Metal 对象与 encoder 操作。
// 渲染器（renderer*.go）完全不用改，圆角/文字/图片/毛玻璃/超采样全部复用。

var metalFns *glFns

// metalActive 供 glFnsOverride 判断是否已切到 Metal。
func metalActive() bool { return metalFns != nil }

// cString 读取 NUL 结尾的 C 串（GL 名字参数）。
func cString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
		if n > 1<<16 {
			break
		}
	}
	return string(unsafe.Slice(p, n))
}

// 静态渲染器名（glGetString(GL_RENDERER)）。
var metalNameC = []byte("Apple Metal\x00")

// installMetalGL 把 Metal 调用表挂到 GL 包装层（见 glport.go 的 glFnsOverride）。
func installMetalGL() {
	m := metalPtr
	if m == nil {
		return
	}
	metalFns = &glFns{
		GenBuffers: func(n int32, ids *uint32) {
			for i := int32(0); i < n; i++ {
				m.nextID++
				(unsafe.Slice(ids, int(n)))[i] = m.nextID
			}
		},
		BindBuffer: func(target, id uint32) { m.curBuf = id },
		BufferData: func(target uint32, size int, data unsafe.Pointer, usage uint32) {
			b := m.bufs[m.curBuf]
			if b == nil {
				return
			}
			if id := objc.ObjOf(b.buf, "contents"); data != nil && b.size >= size {
				copy(unsafe.Slice((*byte)(unsafe.Pointer(id)), size), unsafe.Slice((*byte)(data), size))
				return
			}
			// 容量不足（或孤儿化分配）：重建缓冲
			if b.buf != 0 {
				objc.Send(b.buf, "release")
			}
			nb := objc.Send2(m.dev, "newBufferWithLength:options:", uintptr(size), 0)
			b.buf, b.size = nb, size
			if data != nil && nb != 0 {
				dst := unsafe.Slice((*byte)(unsafe.Pointer(objc.ObjOf(nb, "contents"))), size)
				copy(dst, unsafe.Slice((*byte)(data), size))
			}
		},
		BufferSubData: func(target uint32, offset, size int, data unsafe.Pointer) {
			b := m.bufs[m.curBuf]
			if b == nil || b.buf == 0 || data == nil || offset+size > b.size {
				return
			}
			dst := unsafe.Slice((*byte)(unsafe.Pointer(objc.ObjOf(b.buf, "contents"))), b.size)
			copy(dst[offset:offset+size], unsafe.Slice((*byte)(data), size))
		},
		DeleteBuffers: func(n int32, ids *uint32) {
			for i := int32(0); i < n; i++ {
				if b := m.bufs[(unsafe.Slice(ids, int(n)))[i]]; b != nil {
					if b.buf != 0 {
						objc.Send(b.buf, "release")
					}
					delete(m.bufs, (unsafe.Slice(ids, int(n)))[i])
				}
			}
		},
		GenVertexArrays: func(n int32, ids *uint32) {
			for i := int32(0); i < n; i++ {
				m.nextID++
				(unsafe.Slice(ids, int(n)))[i] = m.nextID
				m.vaos[(unsafe.Slice(ids, int(n)))[i]] = &mtlVAO{}
			}
		},
		BindVertexArray: func(id uint32) { m.curVAO = id },
		DeleteVertexArrays: func(n int32, ids *uint32) {
			for i := int32(0); i < n; i++ {
				delete(m.vaos, (unsafe.Slice(ids, int(n)))[i])
			}
		},
		EnableVertexAttribArray: func(index uint32) {
			v := m.vaos[m.curVAO]
			if v == nil {
				return
			}
			for len(v.attrs) <= int(index) {
				v.attrs = append(v.attrs, mtlAttr{loc: len(v.attrs), size: 2})
			}
			if int(index) >= 0 {
				// 标记启用（size 由 VertexAttribPointer 填）
				if v.attrs[index].size == 2 {
					v.attrs[index] = mtlAttr{loc: int(index), size: 2}
				}
			}
		},
		VertexAttribPointer: func(index uint32, size int32, typ uint32, normalized uint8, stride int32, offset unsafe.Pointer) {
			v := m.vaos[m.curVAO]
			if v == nil {
				return
			}
			for len(v.attrs) <= int(index) {
				v.attrs = append(v.attrs, mtlAttr{loc: len(v.attrs)})
			}
			v.attrs[index] = mtlAttr{loc: int(index), size: int(size), off: int(uintptr(offset))}
			v.stride = int(stride)
			v.buf = m.curBuf
		},
		CreateShader: func(typ uint32) uint32 {
			m.nextID++
			m.shaders[m.nextID] = &mtlShader{typ: typ}
			return m.nextID
		},
		ShaderSource: func(shader uint32, count int32, src **byte, length *int32) {
			s := m.shaders[shader]
			if s == nil || src == nil {
				return
			}
			s.src = cString(*src)
		},
		CompileShader: func(shader uint32) {},
		GetShaderiv: func(shader uint32, pname uint32, params *int32) {
			if params != nil {
				*params = 1 // 编译状态恒为成功（MSL 由管线创建时统一校验）
			}
		},
		GetShaderInfoLog: func(shader uint32, maxLen int32, length *int32, info *byte) {
			if length != nil {
				*length = 0
			}
		},
		DeleteShader:  func(shader uint32) { delete(m.shaders, shader) },
		CreateProgram: func() uint32 {
			m.nextID++
			m.progs[m.nextID] = &mtlProg{loc: map[int32]string{}}
			return m.nextID
		},
		AttachShader: func(program, shader uint32) {
			p := m.progs[program]
			s := m.shaders[shader]
			if p == nil || s == nil {
				return
			}
			if kind, ok := metalKindFromSource(s.src); ok {
				p.kind = kind
			}
		},
		LinkProgram: func(program uint32) {},
		GetProgramiv: func(program uint32, pname uint32, params *int32) {
			if params != nil {
				*params = 1
			}
		},
		GetProgramInfoLog: func(program uint32, maxLen int32, length *int32, info *byte) {
			if length != nil {
				*length = 0
			}
		},
		UseProgram:   func(program uint32) { m.curProg = program },
		DeleteProgram: func(program uint32) { delete(m.progs, program) },
		GetUniformLocation: func(program uint32, name *byte) int32 {
			p := m.progs[program]
			if p == nil {
				return -1
			}
			// 统一的 uniform 槽位（与 MSL 的字段对应），避免受 names 顺序影响。
			switch cString(name) {
			case "uViewport":
				return 0
			case "uTextGamma":
				return 1
			case "uTex":
				return 2
			case "uTexel":
				return 3
			case "uRadius", "uSpan":
				return 4
			case "uSharpen":
				return 5
			}
			return -1
		},
		Uniform1i: func(loc, v int32) {},
		Uniform1f: func(loc int32, v float32) {
			switch loc {
			case 1:
				m.uni.U[0] = v // uTextGamma
			case 4:
				m.uni.U[2] = v // uRadius / uSpan
			case 5:
				m.uni.U[3] = v // uSharpen
			}
		},
		Uniform2f: func(loc int32, x, y float32) {
			switch loc {
			case 0:
				m.uni.VP = [2]float32{x, y} // uViewport
			case 3:
				m.uni.U[0], m.uni.U[1] = x, y // uTexel
			}
		},
		GenTextures: func(n int32, ids *uint32) {
			for i := int32(0); i < n; i++ {
				m.nextID++
				(unsafe.Slice(ids, int(n)))[i] = m.nextID
			}
		},
		BindTexture: func(target, id uint32) { m.curTex = id },
		DeleteTextures: func(n int32, ids *uint32) {
			for i := int32(0); i < n; i++ {
				if t := m.texs[(unsafe.Slice(ids, int(n)))[i]]; t != nil {
					if t.tex != 0 {
						objc.Send(t.tex, "release")
					}
					delete(m.texs, (unsafe.Slice(ids, int(n)))[i])
				}
			}
		},
		TexImage2D: func(target uint32, level, internalFormat int32, w, h int32, border int32, format, typ uint32, pixels unsafe.Pointer) {
			id := m.curTex
			if id == 0 {
				return
			}
			if old := m.texs[id]; old != nil && old.tex != 0 {
				objc.Send(old.tex, "release")
			}
			r8 := internalFormat == int32(GL_R8)
			n := int(w) * int(h)
			if !r8 {
				n *= 4
			}
			var data []byte
			if pixels != nil && n > 0 {
				data = unsafe.Slice((*byte)(pixels), n)
			}
			t := &mtlTex{w: int(w), h: int(h), r8: r8, rt: pixels == nil}
			t.tex = m.newTexture(int(w), int(h), r8, data)
			if t.tex == 0 {
				return
			}
			m.texs[id] = t
		},
		TexSubImage2D: func(target uint32, level, x, y, w, h int32, format, typ uint32, pixels unsafe.Pointer) {
			t := m.texs[m.curTex]
			if t == nil || pixels == nil {
				return
			}
			n := int(w) * int(h)
			if !t.r8 {
				n *= 4
			}
			if n <= 0 {
				return
			}
			m.replace(t.tex, t.w, t.h, t.r8, int(x), int(y), int(w), int(h), unsafe.Slice((*byte)(pixels), n))
		},
		TexParameteri: func(target, pname uint32, param int32) {},
		ActiveTexture: func(tex uint32) {},
		PixelStorei:   func(pname uint32, param int32) {},
		GenFramebuffers: func(n int32, ids *uint32) {
			for i := int32(0); i < n; i++ {
				m.nextID++
				(unsafe.Slice(ids, int(n)))[i] = m.nextID
				m.fbos[(unsafe.Slice(ids, int(n)))[i]] = 0
			}
		},
		BindFramebuffer: func(target, fb uint32) {
			if m.curFBO == fb {
				return
			}
			// 切换目标必须先结束当前 pass：Metal 的 render pass 绑定单个目标。
			m.endEncoder()
			m.curFBO = fb
		},
		FramebufferTexture2D: func(target, attachment, textarget, texture uint32, level int32) {
			m.fbos[m.curFBO] = texture
			if t := m.texs[texture]; t != nil {
				t.rt = true
			}
		},
		CheckFramebufferStatus: func(target uint32) uint32 { return uint32(GL_FRAMEBUFFER_COMPLETE) },
		DeleteFramebuffers: func(n int32, ids *uint32) {
			for i := int32(0); i < n; i++ {
				delete(m.fbos, (unsafe.Slice(ids, int(n)))[i])
				if m.curFBO == (unsafe.Slice(ids, int(n)))[i] {
					m.curFBO = 0
				}
			}
		},
		CopyTexSubImage2D: func(target uint32, level, xoff, yoff, x, y, w, h int32) {
			dst := m.texs[m.curTex]
			if dst == nil || w <= 0 || h <= 0 {
				return
			}
			src := m.targetTexture()
			if src == 0 || m.cmd == 0 {
				return
			}
			srcW, srcH := int(m.pw), int(m.ph)
			if m.curFBO != 0 {
				if t, ok := m.texs[m.fbos[m.curFBO]]; ok {
					srcW, srcH = t.w, t.h
				}
			} else if srcH0 := objc.IntOfN(src, "height"); srcH0 > 0 {
				srcW, srcH = int(objc.IntOfN(src, "width")), int(srcH0)
			}
			m.drawCopy(src, srcW, srcH, int(x), int(y), int(w), int(h), dst.tex, dst.w, dst.h, int(xoff), int(yoff))
		},
		GetString: func(name uint32) *byte { return &metalNameC[0] },
		Viewport: func(x, y, w, h int32) {
			m.vpX, m.vpY, m.vpW, m.vpH = x, y, w, h
		},
		Scissor: func(x, y, w, h int32) {
			m.scX, m.scY, m.scW, m.scH = x, y, w, h
		},
		ClearColor: func(r, g, b, a float32) {
			m.clearR, m.clearG, m.clearB, m.clearA = r, g, b, a
		},
		Clear: func(mask uint32) { m.clearSolid() },
		Enable: func(cap uint32) {
			switch cap {
			case GL_SCISSOR_TEST:
				m.scissorOn = true
			case GL_BLEND:
				m.blendOn = true
			}
		},
		Disable: func(cap uint32) {
			switch cap {
			case GL_SCISSOR_TEST:
				m.scissorOn = false
			case GL_BLEND:
				m.blendOn = false
			}
		},
		BlendFunc:  func(sf, df uint32) { m.blendOn = true },
		DrawArrays: func(mode uint32, first, count int32) { m.draw(count) },
	}
	clear(metalNameC)
	copy(metalNameC, []byte("Apple Metal\x00"))
	glFnsOverride = func() *glFns {
		if metalActive() {
			return metalFns
		}
		return nil
	}
}

// setScissor 应用 MTLScissorRect（th = 目标像素高度，y 从顶部量）。
func setScissor(enc uintptr, x, y, w, h, th int32) {
	if enc == 0 || sendSetScissor == nil {
		return
	}
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	sy := th - y - h
	if sy < 0 {
		sy = 0
	}
	sendSetScissor(enc, objc.Sel("setScissorRect:"), mtlScissorRect{
		X: uint64(x), Y: uint64(sy), W: uint64(w), H: uint64(h),
	})
}

// setViewport 应用 GL 视口（逻辑像素）到当前 encoder。
func (m *metalCtx) setViewportInternal(x, y, w, h int32) {
	enc := m.enc
	if enc == 0 {
		return
	}
	th := m.targetHeight()
	if th < 1 {
		th = 1
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	// GL 视口 y 从底部量；Metal 从顶部量。
	my := th - y - h
	if my < 0 {
		my = 0
	}
	v := mtlViewport{X: float64(x), Y: float64(my), W: float64(w), H: float64(h), Znear: 0, Zfar: 1}
	if sendSetViewport != nil {
		sendSetViewport(enc, objc.Sel("setViewport:"), v)
	}
}

// setScissorInternal 应用裁剪矩形（GL 语义）。
func (m *metalCtx) setScissorInternal() {
	enc := m.enc
	if enc == 0 {
		return
	}
	th := m.targetHeight()
	if th < 1 {
		th = 1
	}
	x, y, w, h := m.scX, m.scY, m.scW, m.scH
	if !m.scissorOn {
		x, y, w, h = 0, 0, m.targetW(), th
	}
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	setScissor(enc, x, y, w, h, th)
}

// targetW 当前目标像素宽度。
func (m *metalCtx) targetW() int32 {
	if m.curFBO == 0 {
		if m.drawTex != 0 {
			if w := objc.IntOfN(m.drawTex, "width"); w > 0 {
				return int32(w)
			}
		}
		return m.pw
	}
	if t, ok := m.texs[m.fbos[m.curFBO]]; ok {
		return int32(t.w)
	}
	return m.pw
}

// draw 执行一次 glDrawArrays。
func (m *metalCtx) draw(count int32) {
	p := m.progs[m.curProg]
	v := m.vaos[m.curVAO]
	if p == nil || v == nil || count <= 0 || len(v.attrs) == 0 {
		return
	}
	b := m.bufs[v.buf]
	if b == nil || b.buf == 0 {
		return
	}
	if m.pipeline(p.kind, m.blendOn) == 0 {
		return
	}
	enc := m.encoder()
	if enc == 0 {
		return
	}
	objc.Send1(enc, "setRenderPipelineState:", m.pipes[p.kind][boolIdx(m.blendOn)])
	m.setViewportInternal(m.vpX, m.vpY, m.vpW, m.vpH)
	m.setScissorInternal()
	objc.Send3(enc, "setVertexBuffer:offset:atIndex:", b.buf, 0, 0)

	// 图片批次：判断纹理是否为离屏目标（第 0 行在屏幕顶部），需要翻转 v。
	if p.kind == pipeImage {
		m.uni.U[0] = 0
		if t := m.texs[m.curTex]; t != nil && t.rt {
			m.uni.U[0] = 1
		}
	}
	if m.smpl != 0 {
		objc.Send2(enc, "setFragmentSamplerState:atIndex:", m.smpl, 0)
	}
	if t := m.texs[m.curTex]; t != nil && t.tex != 0 {
		objc.Send2(enc, "setFragmentTexture:atIndex:", t.tex, 0)
	}
	objc.Send3(enc, "setVertexBytes:length:atIndex:", uintptr(unsafe.Pointer(&m.uni)), unsafe.Sizeof(m.uni), 1)
	objc.Send3(enc, "setFragmentBytes:length:atIndex:", uintptr(unsafe.Pointer(&m.uni)), unsafe.Sizeof(m.uni), 1)
	objc.Send3(enc, "drawPrimitives:vertexStart:vertexCount:", 3, uintptr(0), uintptr(count))
}
