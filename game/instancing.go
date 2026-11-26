// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/binary"
	"math"
)

// 批量绘制：大型游戏里同一种网格（草 / 石块 / 子弹 / 树）往往有上千个，
// 逐个 draw 会浪费 CPU 与驱动开销。
//
//   - InstanceBatch：同网格多实例，一次 Draw 画完（GPU 实例化）
//   - MergeMeshes：静态合批，把多个网格烘成一个大网格（减少顶点缓冲切换）
//
// 实例流布局（与 ShaderInstanced 约定一致）：
//
//	Location 4：xyz = 世界偏移，w = 统一缩放
//	Location 5：rgba = 颜色

// InstanceData 单实例数据。
type InstanceData struct {
	// Offset 世界偏移。
	Offset Vec3
	// Scale 统一缩放（0 视为 1）。
	Scale float32
	// Tint 颜色染色。
	Tint RGBA
	// RotY 绕 Y 轴旋转（弧度，0 = 不旋转）。
	RotY float32
	// ScaleY 垂直缩放（0 = 与 Scale 相同）。
	ScaleY float32
}

// InstanceBatch 同网格实例批次。
type InstanceBatch struct {
	// Mesh 共享网格。
	Mesh *Mesh
	// Material 共享材质。
	Material *Material
	// Data 实例数据。
	Data []InstanceData
	// Capacity 预分配容量（0 = 不预分配）。
	Capacity int
}

// NewInstanceBatch 创建实例批次。
func NewInstanceBatch(m *Mesh, mat *Material) *InstanceBatch {
	if mat == nil {
		mat = DefaultMaterial()
	}
	return &InstanceBatch{Mesh: m, Material: mat, Data: make([]InstanceData, 0, 64)}
}

// Add 添加实例。
func (b *InstanceBatch) Add(offset Vec3, scale float32, tint RGBA) {
	if b == nil {
		return
	}
	if tint.A == 0 {
		tint.A = 255
	}
	b.Data = append(b.Data, InstanceData{Offset: offset, Scale: scale, Tint: tint})
}

// AddTransformed 按矩阵添加实例（只取平移与平均缩放）。
func (b *InstanceBatch) AddTransformed(m Mat4, tint RGBA) {
	p := m.TransformPoint(V3(0, 0, 0))
	sx := V3(m[0], m[1], m[2]).Len()
	sy := V3(m[4], m[5], m[6]).Len()
	sz := V3(m[8], m[9], m[10]).Len()
	b.Add(p, (sx+sy+sz)/3, tint)
}

// MatrixOf 返回第 i 个实例的局部变换（偏移 → 绕 Y 旋转 → 缩放）。
//
// 渲染器提交实例时用它把网格搬到实例位置；场景节点的世界矩阵会叠在外面。
func (b *InstanceBatch) MatrixOf(i int) Mat4 {
	if b == nil || i < 0 || i >= len(b.Data) {
		return Identity()
	}
	d := b.Data[i]
	s := d.Scale
	if s <= 0 {
		s = 1
	}
	sy := d.ScaleY
	if sy <= 0 {
		sy = s
	}
	m := Translate(d.Offset)
	if d.RotY != 0 {
		m = m.Mul(Rotate(V3(0, 1, 0), d.RotY))
	}
	return m.Mul(Scale(V3(s, sy, s)))
}

// Count 实例数。
func (b *InstanceBatch) Count() int {
	if b == nil {
		return 0
	}
	return len(b.Data)
}

// Reset 清空实例（保留容量）。
func (b *InstanceBatch) Reset() {
	if b == nil {
		return
	}
	b.Data = b.Data[:0]
}

// InstanceLayout 实例流布局（偏移+缩放 16 字节、颜色 16 字节）。
func InstanceLayout() *VertexLayout {
	return &VertexLayout{
		Stride:    32,
		Instanced: true,
		Attrs: []VertexAttr{
			{Location: 4, Offset: 0, Format: VF32x4},
			{Location: 5, Offset: 16, Format: VF32x4},
		},
	}
}

// Stream 生成实例流字节数据。
func (b *InstanceBatch) Stream() ([]byte, *VertexLayout) {
	layout := InstanceLayout()
	if b == nil || len(b.Data) == 0 {
		return nil, layout
	}
	buf := make([]byte, len(b.Data)*layout.Stride)
	for i, d := range b.Data {
		off := i * layout.Stride
		binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(d.Offset.X))
		binary.LittleEndian.PutUint32(buf[off+4:], math.Float32bits(d.Offset.Y))
		binary.LittleEndian.PutUint32(buf[off+8:], math.Float32bits(d.Offset.Z))
		binary.LittleEndian.PutUint32(buf[off+12:], math.Float32bits(d.Scale))
		binary.LittleEndian.PutUint32(buf[off+16:], math.Float32bits(float32(d.Tint.R)/255))
		binary.LittleEndian.PutUint32(buf[off+20:], math.Float32bits(float32(d.Tint.G)/255))
		binary.LittleEndian.PutUint32(buf[off+24:], math.Float32bits(float32(d.Tint.B)/255))
		binary.LittleEndian.PutUint32(buf[off+28:], math.Float32bits(float32(d.Tint.A)/255))
	}
	return buf, layout
}

// Upload 上传实例流为 GPU 缓冲。
func (b *InstanceBatch) Upload(g *GPU) (*GPUBuffer, *VertexLayout, error) {
	data, layout := b.Stream()
	if len(data) == 0 {
		return nil, layout, nil
	}
	buf, err := g.NewBuffer(BufDesc{Size: len(data), Usage: BufVertex, Data: data})
	if err != nil {
		return nil, layout, err
	}
	return buf, layout, nil
}

// ShaderInstanced 实例化无光照着色（偏移 + 缩放 + 染色）。
//
// 顶点色 × 实例色 × 纹理(slot 0) 作为最终颜色。
func ShaderInstanced() *ShaderProgram {
	return &ShaderProgram{
		Name: "instanced",
		Vertex: func(in *VertexIn, out *Varyings) {
			s := in.Inst[0].W
			if s == 0 {
				s = 1
			}
			p := Vec3{
				X: in.Position.X*s + in.Inst[0].X,
				Y: in.Position.Y*s + in.Inst[0].Y,
				Z: in.Position.Z*s + in.Inst[0].Z,
			}
			out.Pos = in.MVP.TransformVec4(Vec4{p.X, p.Y, p.Z, 1})
			out.UV = in.UV
			out.Normal = in.Model.TransformDir(in.Normal)
			inst := RGBA{
				floatToU8(in.Inst[1].X),
				floatToU8(in.Inst[1].Y),
				floatToU8(in.Inst[1].Z),
				floatToU8(in.Inst[1].W),
			}
			if in.Inst[1] == (Vec4{}) {
				inst = RGBA{255, 255, 255, 255}
			}
			out.Color = mulRGBA(in.Color, inst)
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			c := in.Color
			if s := in.Tex[0]; s != nil {
				c = mulRGBA(c, s.RGBA8(in.UV.X, in.UV.Y))
			}
			out.Color[0] = mulRGBA(c, uniformTint(in))
		},
		GLSL: &GLSLProgram{
			Vertex: `#version 330 core
layout(location=0) in vec3 a_pos;
layout(location=1) in vec3 a_normal;
layout(location=2) in vec2 a_uv;
layout(location=3) in vec4 a_color;
layout(location=4) in vec4 a_attr0;
layout(location=5) in vec4 a_attr1;
uniform mat4 u_mvp;
out vec2 v_uv;
out vec4 v_color;
out vec3 v_normal;
void main() {
    float s = a_attr0.w == 0.0 ? 1.0 : a_attr0.w;
    vec3 p = a_pos * s + a_attr0.xyz;
    gl_Position = u_mvp * vec4(p, 1.0);
    v_uv = a_uv;
    v_color = a_color * a_attr1;
    v_normal = a_normal;
}`,
			Fragment: `#version 330 core
in vec2 v_uv;
in vec4 v_color;
in vec3 v_normal;
uniform sampler2D u_tex;
uniform vec4 u_tint;
uniform int u_hasTex;
out vec4 fragColor;
void main() {
    vec4 c = v_color * u_tint;
    if (u_hasTex == 1) c *= texture(u_tex, v_uv);
    fragColor = c;
}`,
			Uniforms: []string{"u_tint", "u_hasTex"},
		},
	}
}

// MergeMeshes 静态合批：把多个网格按各自变换烘成一个网格。
//
// transforms 与 parts 等长，可为 nil（表示单位矩阵）。
// 材质需相同（只合并几何）。返回 nil 表示无有效输入。
func MergeMeshes(parts []*Mesh, transforms []Mat4) *Mesh {
	if len(parts) == 0 {
		return nil
	}
	var pos []Vec3
	var nrm []Vec3
	var uv []Vec2
	var col []RGBA
	var idx []uint32
	for i, m := range parts {
		if m == nil || len(m.Positions) == 0 {
			continue
		}
		var tr Mat4
		if i < len(transforms) {
			tr = transforms[i]
		} else {
			tr = Identity()
		}
		base := uint32(len(pos))
		nm := tr.NormalMatrix()
		for j, p := range m.Positions {
			pos = append(pos, tr.TransformPoint(p))
			if j < len(m.Normals) {
				nrm = append(nrm, nm.TransformDir(m.Normals[j]).Normalize())
			} else {
				nrm = append(nrm, V3(0, 1, 0))
			}
			if j < len(m.UVs) {
				uv = append(uv, m.UVs[j])
			} else {
				uv = append(uv, Vec2{})
			}
			if j < len(m.Colors) {
				col = append(col, m.Colors[j])
			} else {
				col = append(col, RGBA{255, 255, 255, 255})
			}
		}
		for _, v := range m.Indices {
			idx = append(idx, base+v)
		}
	}
	if len(pos) == 0 || len(idx) == 0 {
		return nil
	}
	out := NewMesh(pos, idx)
	out.Normals = nrm
	out.UVs = uv
	out.Colors = col
	out.RecomputeBounds()
	return out
}
