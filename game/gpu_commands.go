// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// 本文件是 GPU 命令列表：录制 → Submit 后由后端一次性执行。
// 命令按录制顺序生效，语义接近 WebGPU 的 render pass 内录制。

// gpuCmdKind 命令种类。
type gpuCmdKind uint8

const (
	gpuCmdSetPipeline gpuCmdKind = iota
	gpuCmdSetVertex
	gpuCmdSetInstance
	gpuCmdSetIndex
	gpuCmdSetUniform
	gpuCmdSetTexture
	gpuCmdDraw
	gpuCmdClearColor
	gpuCmdClearDepth
	gpuCmdSetViewport
	gpuCmdCopyTexture
	gpuCmdDispatch
)

// gpuCmd 单条命令。字段按需复用，避免每种命令一个类型。
type gpuCmd struct {
	kind gpuCmdKind

	pipeline *Pipeline
	buffer   *GPUBuffer
	layout   *VertexLayout
	tex      *GPUTexture
	sampler  *TextureSampler
	kernel   *ComputeKernel

	name      string
	floats    []float32
	slot      int
	target    int
	count     int
	instances int
	indexed   bool
	color     RGBA
	depth     float32
	viewport  [4]int
	groups    [3]int
	src, dst  *GPUTexture
}

// ComputeKernel GPU 计算内核。
//
// 软件后端在 worker 池上按 (x,y,z) 三维索引并行执行 Run；
// 硬件后端需要 GLSL（OpenGL 4.3+ compute），没有 GLSL 时退化为软件执行。
type ComputeKernel struct {
	// Name 内核名（统计 / 调试）。
	Name string
	// Run 单个调用（按全局索引 x/y/z）。闭包捕获存储缓冲最方便。
	Run func(x, y, z int)
	// GLSL compute 着色器源码（可选）。
	GLSL string
}

// GPUCommands 命令列表（非并发安全，一个设备同时只有一份录制中）。
type GPUCommands struct {
	dev  *GPU
	fb   *GPUFramebuffer
	name string
	ops  []gpuCmd
}

// SetFramebuffer 指定本次提交的渲染目标。
func (c *GPUCommands) SetFramebuffer(fb *GPUFramebuffer) *GPUCommands {
	c.fb = fb
	return c
}

// Framebuffer 当前渲染目标。
func (c *GPUCommands) Framebuffer() *GPUFramebuffer { return c.fb }

// Name 命名本次提交（调试用）。
func (c *GPUCommands) Name(n string) *GPUCommands {
	c.name = n
	return c
}

// Len 已录制的命令数。
func (c *GPUCommands) Len() int { return len(c.ops) }

// Reset 清空命令，复用对象。
func (c *GPUCommands) Reset() *GPUCommands {
	c.ops = c.ops[:0]
	c.fb = nil
	return c
}

// SetPipeline 切换渲染状态。
func (c *GPUCommands) SetPipeline(p *Pipeline) *GPUCommands {
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdSetPipeline, pipeline: p})
	return c
}

// SetVertexBuffer 绑定顶点缓冲及其布局。
func (c *GPUCommands) SetVertexBuffer(b *GPUBuffer, layout *VertexLayout) *GPUCommands {
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdSetVertex, buffer: b, layout: layout})
	return c
}

// SetInstanceBuffer 绑定逐实例数据缓冲（Inst[0..3] 属性）。
func (c *GPUCommands) SetInstanceBuffer(b *GPUBuffer, layout *VertexLayout) *GPUCommands {
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdSetInstance, buffer: b, layout: layout})
	return c
}

// SetIndexBuffer 绑定索引缓冲（uint32 索引）。
func (c *GPUCommands) SetIndexBuffer(b *GPUBuffer) *GPUCommands {
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdSetIndex, buffer: b})
	return c
}

// SetUniform 写入 uniform 浮点数组。
func (c *GPUCommands) SetUniform(name string, v ...float32) *GPUCommands {
	cp := make([]float32, len(v))
	copy(cp, v)
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdSetUniform, name: name, floats: cp})
	return c
}

// SetUniformMat4 写入 mat4 uniform（列主序，与 Mat4 一致）。
func (c *GPUCommands) SetUniformMat4(name string, m Mat4) *GPUCommands {
	v := make([]float32, 16)
	copy(v, m[:])
	return c.SetUniform(name, v...)
}

// SetUniformFloat 写入标量 uniform。
func (c *GPUCommands) SetUniformFloat(name string, v float32) *GPUCommands {
	return c.SetUniform(name, v)
}

// SetTexture 绑定纹理与采样状态到 slot（0..7），t 为 nil 表示解绑。
func (c *GPUCommands) SetTexture(slot int, t *GPUTexture, s *TextureSampler) *GPUCommands {
	if slot < 0 || slot > 7 {
		return c
	}
	if s != nil && s.Tex == nil {
		s.Tex = t
	}
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdSetTexture, slot: slot, tex: t, sampler: s})
	return c
}

// Draw 索引绘制。已绑定的索引缓冲 + 顶点缓冲参与。
func (c *GPUCommands) Draw(indexCount, instanceCount int) *GPUCommands {
	if instanceCount <= 0 {
		instanceCount = 1
	}
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdDraw, count: indexCount, instances: instanceCount, indexed: true})
	return c
}

// DrawArrays 非索引绘制（顶点顺序直读）。
func (c *GPUCommands) DrawArrays(vertexCount, instanceCount int) *GPUCommands {
	if instanceCount <= 0 {
		instanceCount = 1
	}
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdDraw, count: vertexCount, instances: instanceCount})
	return c
}

// ClearColor 清空第 target 个颜色附件。
func (c *GPUCommands) ClearColor(target int, col RGBA) *GPUCommands {
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdClearColor, target: target, color: col})
	return c
}

// ClearDepth 清空深度附件（软硬件统一用 0..1，默认 1 表示最远）。
func (c *GPUCommands) ClearDepth(v float32) *GPUCommands {
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdClearDepth, depth: v})
	return c
}

// SetViewport 设置视口（像素坐标，默认整帧）。
func (c *GPUCommands) SetViewport(x, y, w, h int) *GPUCommands {
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdSetViewport, viewport: [4]int{x, y, w, h}})
	return c
}

// CopyTexture 整幅拷贝（同尺寸同格式）。
func (c *GPUCommands) CopyTexture(dst, src *GPUTexture) *GPUCommands {
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdCopyTexture, src: src, dst: dst})
	return c
}

// Dispatch 派发计算内核，工作组数 (gx, gy, gz)。
func (c *GPUCommands) Dispatch(k *ComputeKernel, gx, gy, gz int) *GPUCommands {
	if k == nil {
		return c
	}
	if gx < 1 {
		gx = 1
	}
	if gy < 1 {
		gy = 1
	}
	if gz < 1 {
		gz = 1
	}
	c.ops = append(c.ops, gpuCmd{kind: gpuCmdDispatch, kernel: k, groups: [3]int{gx, gy, gz}})
	return c
}

// ---------- 顶点数据读取 ----------

// f32At 从字节流读取小端 float32。
func f32At(b []byte, off int) float32 {
	if off < 0 || off+4 > len(b) {
		return 0
	}
	return math.Float32frombits(uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24)
}

// u32At 从字节流读取小端 uint32。
func u32At(b []byte, off int) uint32 {
	if off < 0 || off+4 > len(b) {
		return 0
	}
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
}

// readAttrs 按布局解析一条顶点/实例记录。
func readAttrs(data []byte, layout *VertexLayout, stride, index int) (pos Vec3, nrm Vec3, uv Vec2, col RGBA, attr [4]Vec4) {
	col = RGBA{255, 255, 255, 255}
	if layout == nil || len(data) == 0 {
		return
	}
	base := index * stride
	if base < 0 || base >= len(data) {
		return
	}
	for _, a := range layout.Attrs {
		off := base + a.Offset
		switch a.Location {
		case 0:
			pos = Vec3{f32At(data, off), f32At(data, off+4), f32At(data, off+8)}
		case 1:
			nrm = Vec3{f32At(data, off), f32At(data, off+4), f32At(data, off+8)}
		case 2:
			uv = Vec2{f32At(data, off), f32At(data, off+4)}
		case 3:
			if a.Format == VU8x4N {
				if off+4 <= len(data) {
					col = RGBA{data[off], data[off+1], data[off+2], data[off+3]}
				}
			} else {
				col = RGBA{
					floatToU8(f32At(data, off)),
					floatToU8(f32At(data, off+4)),
					floatToU8(f32At(data, off+8)),
					floatToU8(f32At(data, off+12)),
				}
			}
		default:
			slot := a.Location - 4
			if slot < 0 || slot > 3 {
				continue
			}
			n := a.Format.Count()
			v := Vec4{0, 0, 0, 0}
			raw := [4]float32{}
			for i := 0; i < n; i++ {
				raw[i] = f32At(data, off+i*4)
			}
			v = Vec4{raw[0], raw[1], raw[2], raw[3]}
			attr[slot] = v
		}
	}
	return
}
