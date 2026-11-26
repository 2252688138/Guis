// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"errors"
	"fmt"
	"image"
	"runtime"
	"sync"
)

// 本文件是 GPU 抽象层：一套与后端无关的图形 API（设备 / 缓冲 / 纹理 / 管线 /
// 帧缓冲 / 命令列表 / 计算内核），语义对齐 WebGPU、Metal 的最小可用子集。
//
// 后端分两类：
//   - soft：纯 Go 多线程 tile 光栅化（零依赖，六端一致，可单测）
//   - gl  ：OpenGL 离屏渲染（purego 动态加载驱动，失败自动回退 soft）
//
// 用户代码只依赖这里导出的类型，换后端不改编排逻辑。

// PixFormat 像素格式。
type PixFormat uint8

const (
	// PixRGBA8 8 位归一化 RGBA，默认颜色目标格式。
	PixRGBA8 PixFormat = iota
	// PixRGBA8SRGB sRGB 颜色空间的 8 位 RGBA，用于最终输出。
	PixRGBA8SRGB
	// PixRGBA16F 半精度浮点 RGBA（软件后端以 float32 存储）。
	PixRGBA16F
	// PixRGBA32F 单精度浮点 RGBA，HDR 场景与后处理中间缓冲。
	PixRGBA32F
	// PixR8 单通道 8 位，遮罩 / SDF / 灰度。
	PixR8
	// PixDepth32 32 位浮点深度，阴影贴图与深度预通道。
	PixDepth32
	// PixDepth24Stencil8 深度 + 模板。
	PixDepth24Stencil8
)

// BytesPerPixel 单像素占用字节数。
func (f PixFormat) BytesPerPixel() int {
	switch f {
	case PixRGBA8, PixRGBA8SRGB, PixDepth32, PixDepth24Stencil8:
		return 4
	case PixRGBA16F:
		return 8
	case PixRGBA32F:
		return 16
	case PixR8:
		return 1
	}
	return 4
}

// IsDepth 是否深度类格式。
func (f PixFormat) IsDepth() bool {
	return f == PixDepth32 || f == PixDepth24Stencil8
}

// IsFloat 是否浮点颜色格式（HDR）。
func (f PixFormat) IsFloat() bool { return f == PixRGBA16F || f == PixRGBA32F }

// Channels 通道数。
func (f PixFormat) Channels() int {
	switch f {
	case PixR8:
		return 1
	case PixRGBA8, PixRGBA8SRGB, PixRGBA16F, PixRGBA32F:
		return 4
	}
	return 4
}

func (f PixFormat) String() string {
	switch f {
	case PixRGBA8:
		return "rgba8"
	case PixRGBA8SRGB:
		return "rgba8-srgb"
	case PixRGBA16F:
		return "rgba16f"
	case PixRGBA32F:
		return "rgba32f"
	case PixR8:
		return "r8"
	case PixDepth32:
		return "depth32f"
	case PixDepth24Stencil8:
		return "depth24s8"
	}
	return fmt.Sprintf("pix(%d)", uint8(f))
}

// TexUsage 纹理用途（位标志，可组合）。
type TexUsage uint8

const (
	// TexSampled 可作为着色器采样源。
	TexSampled TexUsage = 1 << iota
	// TexRenderTarget 可作为颜色渲染目标。
	TexRenderTarget
	// TexDepthStencil 可作为深度/模板附件。
	TexDepthStencil
	// TexStorage 可作为计算内核的读写存储（软件后端）。
	TexStorage
	// TexCube 立方体贴图（6 面按 z 层存储）。
	TexCube
)

// Has 是否包含指定标志位。
func (u TexUsage) Has(f TexUsage) bool { return u&f != 0 }

// TexDesc 纹理描述。
type TexDesc struct {
	// W, H 像素尺寸（必须 > 0）。
	W, H int
	// Depth 层数（数组纹理 / 立方体的 6 层）。
	Depth int
	// Mips mip 级数，0 或 1 表示无 mip 链。
	Mips int
	// Format 像素格式。
	Format PixFormat
	// Usage 用途标志，为 0 时按「可采样 + 可渲染」处理。
	Usage TexUsage
}

func (d TexDesc) norm() (TexDesc, error) {
	if d.W <= 0 || d.H <= 0 {
		return d, fmt.Errorf("game: 纹理尺寸非法 %dx%d", d.W, d.H)
	}
	if d.Depth <= 0 {
		d.Depth = 1
	}
	if d.Mips <= 0 {
		d.Mips = 1
	}
	if d.Format == PixDepth32 || d.Format == PixDepth24Stencil8 {
		d.Usage |= TexDepthStencil
	}
	if d.Usage == 0 {
		d.Usage = TexSampled | TexRenderTarget
	}
	return d, nil
}

// LevelCount 有效 mip 级数。
func (d TexDesc) LevelCount() int {
	if d.Mips <= 1 {
		return 1
	}
	return d.Mips
}

// MipSize 第 level 级尺寸。
func (d TexDesc) MipSize(level int) (int, int) {
	w, h := d.W, d.H
	for i := 0; i < level; i++ {
		w >>= 1
		h >>= 1
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
	}
	return w, h
}

// BufferUsage 缓冲用途（位标志）。
type BufferUsage uint8

const (
	// BufVertex 顶点数据。
	BufVertex BufferUsage = 1 << iota
	// BufIndex 索引数据（uint16 或 uint32）。
	BufIndex
	// BufUniform 统一常量。
	BufUniform
	// BufStorage 计算内核读写存储。
	BufStorage
)

// Has 是否包含指定标志位。
func (u BufferUsage) Has(f BufferUsage) bool { return u&f != 0 }

// BufDesc 缓冲描述。
type BufDesc struct {
	// Size 字节数。
	Size int
	// Usage 用途标志，0 按 BufVertex 处理。
	Usage BufferUsage
	// Data 初始数据（可为空，长度不得超过 Size）。
	Data []byte
}

// VertexFormat 顶点属性格式。
type VertexFormat uint8

const (
	// VF32 单精度标量。
	VF32 VertexFormat = iota
	// VF32x2 二维向量。
	VF32x2
	// VF32x3 三维向量。
	VF32x3
	// VF32x4 四维向量。
	VF32x4
	// VU8x4N 归一化 8 位四通道（顶点色）。
	VU8x4N
)

// Size 该属性占用的字节数。
func (f VertexFormat) Size() int {
	switch f {
	case VF32:
		return 4
	case VF32x2:
		return 8
	case VF32x3:
		return 12
	case VF32x4, VU8x4N:
		return 16
	}
	return 4
}

// Count 分量个数。
func (f VertexFormat) Count() int {
	switch f {
	case VF32:
		return 1
	case VF32x2:
		return 2
	case VF32x3:
		return 3
	}
	return 4
}

// VertexAttr 单个顶点属性。
//
// Location 语义约定（与内置着色器一致）：
//
//	0 = 位置(xyz) 1 = 法线(xyz) 2 = UV(xy) 3 = 顶点色(rgba)
//	4..7 = 自定义 Attr[0..3]，attrN 按 Vec4 传入
type VertexAttr struct {
	// Location 属性槽位。
	Location int
	// Offset 在顶点步长内的字节偏移。
	Offset int
	// Format 数据格式。
	Format VertexFormat
}

// VertexLayout 顶点布局。
type VertexLayout struct {
	// Stride 单个顶点（或单个实例）字节步长。
	Stride int
	// Attrs 属性表。
	Attrs []VertexAttr
	// Instanced 该流是逐实例数据（Location 从 0 重新计为 Inst[0..]）。
	Instanced bool
}

// MeshLayout 与 Mesh 内存布局匹配的顶点布局（Positions/Normals/UVs/Colors）。
//
// 配合 NewVertexBufferFromMesh 使用。
func MeshLayout(stride int) *VertexLayout {
	return &VertexLayout{
		Stride: stride,
		Attrs: []VertexAttr{
			{Location: 0, Offset: 0, Format: VF32x3},
			{Location: 1, Offset: 12, Format: VF32x3},
			{Location: 2, Offset: 24, Format: VF32x2},
			{Location: 3, Offset: 32, Format: VU8x4N},
		},
	}
}

// BlendMode 颜色混合模式。
type BlendMode uint8

const (
	// BlendNone 直接覆盖（不透明几何体）。
	BlendNone BlendMode = iota
	// BlendAlpha 标准 alpha 混合：src.a*src + (1-src.a)*dst。
	BlendAlpha
	// BlendPremultiplied 预乘 alpha：src + (1-src.a)*dst。
	BlendPremultiplied
	// BlendAdd 加法混合：src + dst（发光 / 粒子）。
	BlendAdd
	// BlendMultiply 乘法混合：src * dst（阴影 / 染色）。
	BlendMultiply
)

// DepthFunc 深度比较函数。
type DepthFunc uint8

const (
	// DepthLess 小于通过（默认）。
	DepthLess DepthFunc = iota
	// DepthLEqual 小于等于通过。
	DepthLEqual
	// DepthGreater 大于通过（反向 Z）。
	DepthGreater
	// DepthAlways 总是通过（仍可写深度）。
	DepthAlways
)

// CullMode 剔除模式。
type CullMode uint8

const (
	// CullBack 剔除背面（默认）。
	CullBack CullMode = iota
	// CullNone 双面绘制（草 / 布料 / 粒子面片）。
	CullNone
	// CullFront 剔除正面（阴影体 / 内表面）。
	CullFront
)

// VertexIn 顶点着色器输入。
type VertexIn struct {
	// Position 模型空间位置（Location 0）。
	Position Vec3
	// Normal 模型空间法线（Location 1）。
	Normal Vec3
	// UV 纹理坐标（Location 2）。
	UV Vec2
	// Color 顶点色（Location 3）。
	Color RGBA
	// Attr 自定义属性（Location 4..7）。
	Attr [4]Vec4
	// Inst 实例属性（实例流 Location 0..3，按 Vec4 填充）。
	Inst [4]Vec4
	// VertexID 当前顶点序号。
	VertexID int
	// InstanceID 当前实例序号。
	InstanceID int
	// MVP 视图投影矩阵，取自 uniform「u_mvp」（缺省 Identity）。
	MVP Mat4
	// Model 模型矩阵，取自 uniform「u_model」（缺省 Identity）。
	Model Mat4
}

// Varyings 顶点着色器输出，光栅化后按透视校正插值成 FragmentIn。
type Varyings struct {
	// Pos 裁剪空间位置（必须写，w > 0）。
	Pos Vec4
	// UV 纹理坐标。
	UV Vec2
	// Normal 世界空间法线。
	Normal Vec3
	// Color 顶点色。
	Color RGBA
	// Var 自定义插值量（最多 8 个 Vec4）。
	Var [8]Vec4
}

// FragmentIn 片元着色器输入。
type FragmentIn struct {
	// Coord 屏幕像素坐标。
	Coord Vec2
	// UV 插值后的纹理坐标。
	UV Vec2
	// Normal 插值后的法线。
	Normal Vec3
	// Color 插值后的顶点色。
	Color RGBA
	// Var 自定义插值量。
	Var [8]Vec4
	// Depth 当前片元深度（0..1，0 = 近）。
	Depth float32
	// Tint 材质染色，取自 uniform「u_tint」（缺省白色）。
	Tint RGBA
	// LightDir 指向光源的方向（uniform「u_lightDir」）。
	LightDir Vec3
	// LightColor 光色（uniform「u_lightColor」）。
	LightColor RGBA
	// Ambient 环境光强度（uniform「u_ambient」）。
	Ambient float32
	// Specular 高光强度（uniform「u_specular」）。
	Specular float32
	// Shininess 高光指数（uniform「u_shininess」）。
	Shininess float32
	// Tex 绑定在 0..7 号槽的采样器，未绑定为 nil。
	Tex [8]*TextureSampler
}

// FragmentOut 片元着色器输出。
type FragmentOut struct {
	// Color 最多 4 个颜色目标（MRT / G-Buffer）。
	Color [4]RGBA
	// Depth 可选深度写入，< 0 表示沿用插值深度（多数场景保持 -1）。
	Depth float32
	// Discard 丢弃该片元（镂空 / alpha test）。
	Discard bool
}

// GLSLProgram 硬件后端（OpenGL）使用的 GLSL 源码。
//
// Uniform 名约定（引擎自动填充）：
//
//	u_mvp（mat4）、u_model（mat4）、u_view（mat4）、u_proj（mat4）
//
// 顶点属性名约定：a_pos / a_normal / a_uv / a_color / a_attr0..a_attr3。
type GLSLProgram struct {
	// Vertex 顶点着色器源码。
	Vertex string
	// Fragment 片元着色器源码。
	Fragment string
	// Uniforms 需要显式 SetUniform 填充的 uniform 名。
	Uniforms []string
}

// ShaderProgram 跨后端着色程序。
//
// 软件后端执行 Go 回调（Vertex / Fragment）；硬件后端执行 GLSL。
// 只填 Go 回调时硬件后端会拒绝编译该管线（ErrShaderNoGLSL），
// 此时用 WithGPUBackend("soft") 或给 GLSL 提供等价实现。
type ShaderProgram struct {
	// Name 程序名（日志 / 统计）。
	Name string
	// Vertex 顶点着色回调（软件后端）。
	Vertex func(in *VertexIn, out *Varyings)
	// Fragment 片元着色回调（软件后端）。
	Fragment func(in *FragmentIn, out *FragmentOut)
	// GLSL 硬件后端实现（可为空）。
	GLSL *GLSLProgram
}

// PipelineDesc 管线描述（固定渲染状态）。
type PipelineDesc struct {
	// Program 着色程序。
	Program *ShaderProgram
	// DepthTest 是否启用深度测试。
	DepthTest bool
	// DepthWrite 是否写入深度。
	DepthWrite bool
	// DepthFunc 深度比较函数。
	DepthFunc DepthFunc
	// Cull 剔除模式。
	Cull CullMode
	// Blend 颜色混合模式。
	Blend BlendMode
	// ColorTargets 颜色目标数量（1..4）。
	ColorTargets int
	// BlendTargets 每个颜色目标各自的混合模式，为空时全部用 Blend。
	BlendTargets []BlendMode
	// Uniforms 初始 uniform 表（"u_mvp" 等 16 个 float）。
	Uniforms map[string][]float32
	// Wireframe 线框绘制（软件后端调试用）。
	Wireframe bool
}

// Pipeline 已编译的管线对象。
type Pipeline struct {
	desc PipelineDesc
	name string
	// uniforms 当前 uniform 表（命令 SetUniform 会更新）。
	uniforms map[string][]float32
	// glProgram 硬件后端的 GL 程序对象，0 表示未编译。
	glProgram uint32
	// glAttrib 硬件后端属性/统一位置缓存。
	glAttrib map[string]int32
}

// Name 管线名。
func (p *Pipeline) Name() string { return p.name }

// Uniform 读取 uniform 副本。
func (p *Pipeline) Uniform(name string) []float32 { return p.uniforms[name] }

// GPUFramebuffer 帧缓冲：1..4 个颜色附件 + 可选深度附件。
type GPUFramebuffer struct {
	// Colors 颜色附件。
	Colors []*GPUTexture
	// Depth 深度附件（可为 nil）。
	Depth *GPUTexture
	// W, H 尺寸（取首个附件）。
	W, H int
	// glID 硬件后端帧缓冲对象（内部使用）。
	glID uint32
}

// Size 返回帧缓冲尺寸。
func (fb *GPUFramebuffer) Size() (int, int) {
	if fb == nil {
		return 0, 0
	}
	return fb.W, fb.H
}

// GPUTexture GPU 纹理。
//
// 软件后端持有像素内存（可被 CPU 直接读写，便于测试与回读）；
// 硬件后端持有驱动纹理对象。
type GPUTexture struct {
	desc TexDesc

	pix8 [][]RGBA
	pixF [][]float32
	pixR [][]uint8
	// depth 深度值（每层一份）。
	depth []float32

	glID   uint32
	dev    *GPU
	closed bool
}

// Desc 纹理描述。
func (t *GPUTexture) Desc() TexDesc { return t.desc }

// Size 纹理尺寸。
func (t *GPUTexture) Size() (int, int) { return t.desc.W, t.desc.H }

// SetPixels 上传第 0 级像素（RGBA8 格式，长度须为 W*H）。
func (t *GPUTexture) SetPixels(pix []RGBA) error {
	if t.desc.Format != PixRGBA8 && t.desc.Format != PixRGBA8SRGB {
		return fmt.Errorf("game: SetPixels 只支持 rgba8 纹理，当前 %s", t.desc.Format)
	}
	if len(pix) < t.desc.W*t.desc.H {
		return fmt.Errorf("game: 像素不足 %d < %d", len(pix), t.desc.W*t.desc.H)
	}
	if len(t.pix8) == 0 || len(t.pix8[0]) == 0 {
		return errors.New("game: 该纹理不持有 CPU 像素（硬件后端请用 GPU.UploadTexture）")
	}
	copy(t.pix8[0], pix[:t.desc.W*t.desc.H])
	t.dirtyGPU()
	return nil
}

// Pixels 返回第 0 级像素（仅软件后端，硬件后端请用 GPU.ReadPixels）。
func (t *GPUTexture) Pixels() ([]RGBA, error) {
	if len(t.pix8) == 0 || len(t.pix8[0]) == 0 {
		return nil, errors.New("game: 该纹理不持有 CPU 像素（硬件后端请用 GPU.ReadPixels）")
	}
	return t.pix8[0], nil
}

// SetFloatPixels 上传浮点颜色第 0 级（RGBA 顺序，长度 W*H*4）。
func (t *GPUTexture) SetFloatPixels(v []float32) error {
	if !t.desc.Format.IsFloat() {
		return fmt.Errorf("game: 格式 %s 不是浮点颜色", t.desc.Format)
	}
	w, h := t.desc.W, t.desc.H
	if len(v) < w*h*4 {
		return errors.New("game: 浮点像素数量不足")
	}
	if len(t.pixF) == 0 {
		t.pixF = make([][]float32, t.desc.LevelCount())
	}
	if len(t.pixF[0]) != w*h*4 {
		t.pixF[0] = make([]float32, w*h*4)
	}
	copy(t.pixF[0], v[:w*h*4])
	return nil
}

// SetDepth 上传深度数据（长度 W*H）。
func (t *GPUTexture) SetDepth(d []float32) error {
	w, h := t.desc.W, t.desc.H
	if len(d) < w*h {
		return errors.New("game: 深度数据数量不足")
	}
	if len(t.depth) != w*h {
		t.depth = make([]float32, w*h)
	}
	copy(t.depth, d[:w*h])
	return nil
}

// DepthData 返回深度数据（仅软件后端）。
func (t *GPUTexture) DepthData() []float32 { return t.depth }

// GPUBuffer GPU 缓冲。
type GPUBuffer struct {
	desc BufDesc
	data []byte
	glID uint32
	dev  *GPU
}

// Desc 缓冲描述。
func (b *GPUBuffer) Desc() BufDesc { return b.desc }

// Size 字节数。
func (b *GPUBuffer) Size() int { return b.desc.Size }

// Bytes 返回底层字节（仅软件后端，可原地修改后调用 GPU.UploadBuffer）。
func (b *GPUBuffer) Bytes() []byte { return b.data }

// Write 写入数据到 offset（软件后端立即生效；硬件后端标记脏并随下一帧上传）。
func (b *GPUBuffer) Write(offset int, data []byte) error {
	if offset < 0 || offset+len(data) > b.desc.Size {
		return fmt.Errorf("game: 缓冲越界写入 %d+%d > %d", offset, len(data), b.desc.Size)
	}
	if len(b.data) < b.desc.Size {
		return errors.New("game: 硬件缓冲不可直接写，请用 GPU.UploadBuffer")
	}
	copy(b.data[offset:], data)
	return nil
}

// TextureSampler 纹理 + 采样状态，供软件后端着色器采样。
type TextureSampler struct {
	// Tex 源纹理。
	Tex *GPUTexture
	// Linear 是否双线性过滤（false = 最近邻）。
	Linear bool
	// Mip 是否启用 mipmap 三线性过滤。
	Mip bool
	// Repeat 是否 UV 环绕（false = clamp）。
	Repeat bool
}

// RGBA8 采样并返回 8 位颜色。
func (s *TextureSampler) RGBA8(u, v float32) RGBA {
	if s == nil || s.Tex == nil {
		return RGBA{255, 255, 255, 255}
	}
	t := s.Tex
	w, h := t.desc.W, t.desc.H
	if w <= 0 || h <= 0 {
		return RGBA{}
	}
	if s.Repeat {
		u -= float32(int(u))
		v -= float32(int(v))
		if u < 0 {
			u++
		}
		if v < 0 {
			v++
		}
	} else {
		u = clampf(u, 0, 1)
		v = clampf(v, 0, 1)
	}
	if t.desc.Format.IsFloat() {
		f := s.Vec4(u, v)
		return RGBA{floatToU8(f.X), floatToU8(f.Y), floatToU8(f.Z), floatToU8(f.W)}
	}
	if len(t.pix8) == 0 || len(t.pix8[0]) == 0 {
		return RGBA{255, 255, 255, 255}
	}
	pix := t.pix8[0]
	if !s.Linear {
		x := int(u * float32(w))
		y := int(v * float32(h))
		return pix[clampi(y, 0, h-1)*w+clampi(x, 0, w-1)]
	}
	fx := u*float32(w) - 0.5
	fy := v*float32(h) - 0.5
	x0 := int(floorf(fx))
	y0 := int(floorf(fy))
	tx := fx - float32(x0)
	ty := fy - float32(y0)
	c00 := pix[clampi(y0, 0, h-1)*w+clampi(x0, 0, w-1)]
	c10 := pix[clampi(y0, 0, h-1)*w+clampi(x0+1, 0, w-1)]
	c01 := pix[clampi(y0+1, 0, h-1)*w+clampi(x0, 0, w-1)]
	c11 := pix[clampi(y0+1, 0, h-1)*w+clampi(x0+1, 0, w-1)]
	return RGBA{
		lerpU8(c00.R, c10.R, c01.R, c11.R, tx, ty),
		lerpU8(c00.G, c10.G, c01.G, c11.G, tx, ty),
		lerpU8(c00.B, c10.B, c01.B, c11.B, tx, ty),
		lerpU8(c00.A, c10.A, c01.A, c11.A, tx, ty),
	}
}

// Vec4 采样并返回 0..1 浮点颜色。
func (s *TextureSampler) Vec4(u, v float32) Vec4 {
	if s == nil || s.Tex == nil {
		return Vec4{1, 1, 1, 1}
	}
	t := s.Tex
	if t.desc.Format.IsFloat() {
		if len(t.pixF) == 0 || len(t.pixF[0]) == 0 {
			return Vec4{1, 1, 1, 1}
		}
		w, h := t.desc.W, t.desc.H
		if s.Repeat {
			u -= float32(int(u))
			v -= float32(int(v))
			if u < 0 {
				u++
			}
			if v < 0 {
				v++
			}
		} else {
			u = clampf(u, 0, 1)
			v = clampf(v, 0, 1)
		}
		x := clampi(int(u*float32(w)), 0, w-1)
		y := clampi(int(v*float32(h)), 0, h-1)
		p := t.pixF[0]
		i := (y*w + x) * 4
		return Vec4{p[i], p[i+1], p[i+2], p[i+3]}
	}
	c := s.RGBA8(u, v)
	return Vec4{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}

// GPUCaps 后端能力自述。
type GPUCaps struct {
	// Backend 后端名（"soft" / "gl"）。
	Backend string
	// Hardware 是否硬件加速。
	Hardware bool
	// MaxTextureSize 单边最大纹理尺寸。
	MaxTextureSize int
	// MaxColorTargets 最大颜色附件数（MRT）。
	MaxColorTargets int
	// MaxVertexAttrs 最大顶点属性数。
	MaxVertexAttrs int
	// Compute 是否支持计算内核。
	Compute bool
	// Instancing 是否支持实例化绘制。
	Instancing bool
	// ShadowMap 是否支持深度纹理采样。
	ShadowMap bool
	// FloatTarget 是否支持浮点渲染目标。
	FloatTarget bool
	// Threads 软件后端工作线程数（硬件后端为 0）。
	Threads int
	// Readback 是否支持像素回读到 CPU。
	Readback bool
}

// GPUStats GPU 统计（每次 Submit 后累加，ResetGPUStats 清零）。
type GPUStats struct {
	// DrawCalls 绘制调用数。
	DrawCalls int
	// Instances 实例总数。
	Instances int
	// Vertices 顶点着色器执行次数。
	Vertices int
	// Triangles 提交到光栅化的三角形数。
	Triangles int
	// Clipped 被裁剪剔除的三角形数。
	Culled int
	// Fragments 片元着色器执行次数。
	Fragments int
	// Shaded 通过深度/模板测试的片元数。
	Shaded int
	// Clears 清屏次数。
	Clears int
	// Dispatches 计算内核派发次数。
	Dispatches int
	// UploadBytes 累计上传字节数。
	UploadBytes int64
	// SubmitMs 最近一次 Submit 耗时（毫秒）。
	SubmitMs float64
	// TotalMs 累计 Submit 耗时（毫秒）。
	TotalMs float64
}

var (
	errGPUBackendUnknown = errors.New("game: 未知 GPU 后端")
	errGPUClosed         = errors.New("game: GPU 设备已关闭")
	// ErrShaderNoGLSL 硬件后端遇到没有 GLSL 实现的着色程序。
	ErrShaderNoGLSL = errors.New("game: 该着色程序没有 GLSL 实现，硬件后端无法编译（请提供 GLSL 或改用 soft 后端）")
)

// gpuBackend 后端实现契约（soft / gl）。
type gpuBackend interface {
	// name 后端标识。
	name() string
	// caps 能力自述。
	caps() GPUCaps
	// newTexture 创建纹理资源。
	newTexture(t *GPUTexture) error
	// newBuffer 创建缓冲资源。
	newBuffer(b *GPUBuffer) error
	// compilePipeline 编译管线。
	compilePipeline(p *Pipeline) error
	// submit 执行命令列表。
	submit(c *GPUCommands) error
	// readPixels 回读颜色附件。
	readPixels(fb *GPUFramebuffer, target int, dst []RGBA) error
	// close 释放资源。
	close() error
}

// GPU 图形设备句柄（非并发安全，应在渲染线程使用）。
type GPU struct {
	bk      gpuBackend
	stats   GPUStats
	mu      sync.Mutex
	closed  bool
	workers int
}

// GPUOption 设备创建选项。
type GPUOption func(*gpuOptions)

type gpuOptions struct {
	backend  string
	workers  int
	fallback bool
}

// WithGPUBackend 指定后端（"soft" / "gl" / "auto"）。
func WithGPUBackend(name string) GPUOption {
	return func(o *gpuOptions) { o.backend = name }
}

// WithGPUWorkers 指定软件后端工作线程数，0 = 逻辑 CPU 数。
func WithGPUWorkers(n int) GPUOption {
	return func(o *gpuOptions) { o.workers = n }
}

// WithGPUFallback 硬件后端创建失败时是否回退软件后端（默认开启）。
func WithGPUFallback(enable bool) GPUOption {
	return func(o *gpuOptions) { o.fallback = enable }
}

var (
	gpuRegistryMu sync.Mutex
	gpuRegistry   = map[string]func(*GPU) (gpuBackend, error){}
)

// RegisterGPUBackend 注册 GPU 后端工厂（后端实现文件在 init 中调用）。
func RegisterGPUBackend(name string, factory func(*GPU) (gpuBackend, error)) {
	gpuRegistryMu.Lock()
	defer gpuRegistryMu.Unlock()
	gpuRegistry[name] = factory
}

// GPUBackends 已注册的后端名。
func GPUBackends() []string {
	gpuRegistryMu.Lock()
	defer gpuRegistryMu.Unlock()
	out := make([]string, 0, len(gpuRegistry))
	for k := range gpuRegistry {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

// NewGPU 创建 GPU 设备。
//
// 默认后端顺序：gl（若可用）→ soft。用 WithGPUBackend 显式指定时不做回退
// （除非也开了 WithGPUFallback）。
func NewGPU(opts ...GPUOption) (*GPU, error) {
	o := gpuOptions{backend: "auto", fallback: true}
	for _, fn := range opts {
		fn(&o)
	}
	g := &GPU{workers: o.workers}
	if g.workers <= 0 {
		g.workers = defaultGPUWorkers()
	}
	try := func(name string) (gpuBackend, error) {
		gpuRegistryMu.Lock()
		f, ok := gpuRegistry[name]
		gpuRegistryMu.Unlock()
		if !ok {
			return nil, errGPUBackendUnknown
		}
		return f(g)
	}
	if o.backend == "auto" {
		for _, name := range []string{"gl", "soft"} {
			if bk, err := try(name); err == nil {
				g.bk = bk
				return g, nil
			}
		}
		return nil, errors.New("game: 没有可用的 GPU 后端")
	}
	bk, err := try(o.backend)
	if err != nil && o.fallback && o.backend != "soft" {
		if bk2, err2 := try("soft"); err2 == nil {
			g.bk = bk2
			return g, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("game: 创建 GPU 后端 %s 失败: %w", o.backend, err)
	}
	g.bk = bk
	return g, nil
}

// Backend 后端名。
func (g *GPU) Backend() string {
	if g == nil || g.bk == nil {
		return ""
	}
	return g.bk.name()
}

// Caps 能力自述。
func (g *GPU) Caps() GPUCaps {
	if g == nil || g.bk == nil {
		return GPUCaps{}
	}
	return g.bk.caps()
}

// Workers 工作线程数。
func (g *GPU) Workers() int { return g.workers }

// Stats 统计快照。
func (g *GPU) Stats() GPUStats {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stats
}

// ResetGPUStats 清零统计。
func (g *GPU) ResetGPUStats() {
	g.mu.Lock()
	g.stats = GPUStats{}
	g.mu.Unlock()
}

// NewTexture 创建纹理。
func (g *GPU) NewTexture(d TexDesc) (*GPUTexture, error) {
	if g == nil || g.bk == nil {
		return nil, errGPUClosed
	}
	nd, err := d.norm()
	if err != nil {
		return nil, err
	}
	if mx := g.bk.caps().MaxTextureSize; mx > 0 && (nd.W > mx || nd.H > mx) {
		return nil, fmt.Errorf("game: 纹理超限 %dx%d > %d", nd.W, nd.H, mx)
	}
	t := &GPUTexture{desc: nd, dev: g}
	if err := g.bk.newTexture(t); err != nil {
		return nil, err
	}
	return t, nil
}

// NewBuffer 创建缓冲。
func (g *GPU) NewBuffer(desc BufDesc) (*GPUBuffer, error) {
	if g == nil || g.bk == nil {
		return nil, errGPUClosed
	}
	if desc.Size <= 0 {
		return nil, fmt.Errorf("game: 缓冲尺寸非法 %d", desc.Size)
	}
	if len(desc.Data) > desc.Size {
		return nil, fmt.Errorf("game: 初始数据 %d 超过缓冲尺寸 %d", len(desc.Data), desc.Size)
	}
	if desc.Usage == 0 {
		desc.Usage = BufVertex
	}
	b := &GPUBuffer{desc: desc, dev: g}
	if err := g.bk.newBuffer(b); err != nil {
		return nil, err
	}
	return b, nil
}

// NewPipeline 编译管线。
func (g *GPU) NewPipeline(desc PipelineDesc) (*Pipeline, error) {
	if g == nil || g.bk == nil {
		return nil, errGPUClosed
	}
	if desc.Program == nil || (desc.Program.Vertex == nil && desc.Program.GLSL == nil) {
		return nil, errors.New("game: 管线缺少顶点着色")
	}
	if desc.ColorTargets <= 0 {
		desc.ColorTargets = 1
	}
	if desc.ColorTargets > 4 {
		return nil, fmt.Errorf("game: 颜色目标 %d 超过上限 4", desc.ColorTargets)
	}
	p := &Pipeline{
		desc:     desc,
		name:     desc.Program.Name,
		uniforms: map[string][]float32{},
		glAttrib: map[string]int32{},
	}
	if p.name == "" {
		p.name = "pipeline"
	}
	for k, v := range desc.Uniforms {
		cp := make([]float32, len(v))
		copy(cp, v)
		p.uniforms[k] = cp
	}
	if err := g.bk.compilePipeline(p); err != nil {
		return nil, err
	}
	return p, nil
}

// NewFramebuffer 创建帧缓冲（colors 至少 1 个，尺寸必须一致）。
func (g *GPU) NewFramebuffer(colors []*GPUTexture, depth *GPUTexture) (*GPUFramebuffer, error) {
	if g == nil || g.bk == nil {
		return nil, errGPUClosed
	}
	if len(colors) == 0 {
		return nil, errors.New("game: 帧缓冲至少需要 1 个颜色附件")
	}
	if len(colors) > 4 {
		return nil, fmt.Errorf("game: 颜色附件 %d 超过上限 4", len(colors))
	}
	w, h := colors[0].desc.W, colors[0].desc.H
	for _, c := range colors {
		if c == nil {
			return nil, errors.New("game: 颜色附件为 nil")
		}
		if c.desc.W != w || c.desc.H != h {
			return nil, errors.New("game: 颜色附件尺寸不一致")
		}
	}
	if depth != nil && (depth.desc.W != w || depth.desc.H != h) {
		return nil, errors.New("game: 深度附件尺寸与颜色附件不一致")
	}
	return &GPUFramebuffer{Colors: colors, Depth: depth, W: w, H: h}, nil
}

// NewSampler 构造采样器便捷方法。
func (g *GPU) NewSampler(t *GPUTexture, linear, mip, repeat bool) *TextureSampler {
	return &TextureSampler{Tex: t, Linear: linear, Mip: mip, Repeat: repeat}
}

// Begin 开始录制命令列表。
func (g *GPU) Begin() *GPUCommands { return &GPUCommands{dev: g} }

// Submit 提交并执行命令列表。
func (g *GPU) Submit(c *GPUCommands) error {
	if g == nil || g.bk == nil || g.closed {
		return errGPUClosed
	}
	if c == nil || len(c.ops) == 0 {
		return nil
	}
	return g.bk.submit(c)
}

// ReadPixels 回读帧缓冲第 target 个颜色附件到 image.RGBA。
func (g *GPU) ReadPixels(fb *GPUFramebuffer, target int) (*image.RGBA, error) {
	if g == nil || g.bk == nil {
		return nil, errGPUClosed
	}
	if fb == nil || target < 0 || target >= len(fb.Colors) {
		return nil, errors.New("game: 帧缓冲或颜色目标序号非法")
	}
	t := fb.Colors[target]
	if t == nil {
		return nil, errors.New("game: 颜色附件为空")
	}
	buf := make([]RGBA, fb.W*fb.H)
	if err := g.bk.readPixels(fb, target, buf); err != nil {
		return nil, err
	}
	return rgbaImageFrom(buf, fb.W, fb.H), nil
}

// rgbaImageFrom 把 RGBA 像素切片转成 image.RGBA。
func rgbaImageFrom(pix []RGBA, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	n := w * h
	if len(pix) < n {
		n = len(pix)
	}
	for i := 0; i < n; i++ {
		c := pix[i]
		j := i * 4
		img.Pix[j] = c.R
		img.Pix[j+1] = c.G
		img.Pix[j+2] = c.B
		img.Pix[j+3] = c.A
	}
	return img
}

// defaultGPUWorkers 默认工作线程数（上限 16，避免线程远多于 tile 造成空转）。
func defaultGPUWorkers() int {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	if n > 16 {
		n = 16
	}
	return n
}

// Close 释放设备资源。
func (g *GPU) Close() error {
	if g == nil || g.bk == nil {
		return nil
	}
	if g.closed {
		return nil
	}
	g.closed = true
	return g.bk.close()
}

// dirtyGPU 标记纹理像素被 CPU 改动，硬件后端下一帧需要重传。
func (t *GPUTexture) dirtyGPU() {
	if t != nil && t.dev != nil {
		t.dev.mu.Lock()
		t.dev.stats.UploadBytes += int64(t.desc.W * t.desc.H * t.desc.Format.BytesPerPixel())
		t.dev.mu.Unlock()
	}
}

// ---------- 小工具（避免与包内同名函数冲突，统一 gpu 前缀） ----------

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func floorf(v float32) float32 {
	i := float32(int(v))
	if v < 0 && i != v {
		i--
	}
	return i
}

func floatToU8(v float32) uint8 {
	v = clampf(v, 0, 1)
	return uint8(v*255 + 0.5)
}

func lerpU8(a, b, c, d uint8, tx, ty float32) uint8 {
	top := float32(a) + (float32(b)-float32(a))*tx
	bot := float32(c) + (float32(d)-float32(c))*tx
	return uint8(top + (bot-top)*ty + 0.5)
}

func lerpf(a, b, t float32) float32 { return a + (b-a)*t }

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
