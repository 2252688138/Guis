// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/binary"
	"errors"
	"math"
)

// 本文件负责「游戏资源 → GPU 资源」的落地：
//   - Mesh 打包成顶点缓冲 / 索引缓冲（与内置着色器属性约定一致）
//   - 内置着色程序库（软件后端 Go 回调 + 硬件后端 GLSL，两端同语义）

// gpuMeshStride 内置网格顶点步长（pos3 + normal3 + uv2 + rgba4 = 36 字节）。
const gpuMeshStride = 36

// MeshVertexData 把 Mesh 打包成内置顶点布局的字节流（可单测，不依赖设备）。
func MeshVertexData(m *Mesh) ([]byte, *VertexLayout) {
	if m == nil {
		return nil, MeshLayout(gpuMeshStride)
	}
	n := len(m.Positions)
	buf := make([]byte, n*gpuMeshStride)
	for i := 0; i < n; i++ {
		off := i * gpuMeshStride
		p := m.Positions[i]
		binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(p.X))
		binary.LittleEndian.PutUint32(buf[off+4:], math.Float32bits(p.Y))
		binary.LittleEndian.PutUint32(buf[off+8:], math.Float32bits(p.Z))
		if i < len(m.Normals) {
			nv := m.Normals[i]
			binary.LittleEndian.PutUint32(buf[off+12:], math.Float32bits(nv.X))
			binary.LittleEndian.PutUint32(buf[off+16:], math.Float32bits(nv.Y))
			binary.LittleEndian.PutUint32(buf[off+20:], math.Float32bits(nv.Z))
		} else {
			binary.LittleEndian.PutUint32(buf[off+20:], math.Float32bits(1))
		}
		if i < len(m.UVs) {
			uv := m.UVs[i]
			binary.LittleEndian.PutUint32(buf[off+24:], math.Float32bits(uv.X))
			binary.LittleEndian.PutUint32(buf[off+28:], math.Float32bits(uv.Y))
		}
		var c RGBA
		if i < len(m.Colors) && m.Colors[i].A != 0 {
			c = m.Colors[i]
		} else {
			c = RGBA{255, 255, 255, 255}
		}
		buf[off+32] = c.R
		buf[off+33] = c.G
		buf[off+34] = c.B
		buf[off+35] = c.A
	}
	return buf, MeshLayout(gpuMeshStride)
}

// MeshIndexData 把 Mesh 索引转成 uint32 字节流。
func MeshIndexData(m *Mesh) []byte {
	if m == nil {
		return nil
	}
	buf := make([]byte, len(m.Indices)*4)
	for i, v := range m.Indices {
		binary.LittleEndian.PutUint32(buf[i*4:], v)
	}
	return buf
}

// NewMeshBuffers 上传网格到 GPU，返回顶点缓冲 / 索引缓冲 / 顶点布局。
func (g *GPU) NewMeshBuffers(m *Mesh) (*GPUBuffer, *GPUBuffer, *VertexLayout, error) {
	if m == nil || len(m.Positions) == 0 {
		return nil, nil, nil, errors.New("game: 空网格")
	}
	vdata, layout := MeshVertexData(m)
	vb, err := g.NewBuffer(BufDesc{Size: len(vdata), Usage: BufVertex, Data: vdata})
	if err != nil {
		return nil, nil, nil, err
	}
	if len(m.Indices) == 0 {
		return vb, nil, layout, nil
	}
	idata := MeshIndexData(m)
	ib, err := g.NewBuffer(BufDesc{Size: len(idata), Usage: BufIndex, Data: idata})
	if err != nil {
		return vb, nil, layout, err
	}
	return vb, ib, layout, nil
}

// NewTextureFromGame 把 Texture 上传为 GPU 纹理（含 mip 链时会一并上传）。
func (g *GPU) NewTextureFromGame(t *Texture) (*GPUTexture, error) {
	if t == nil || t.W <= 0 || t.H <= 0 {
		return nil, errors.New("game: 空纹理")
	}
	img, err := g.NewTexture(TexDesc{W: t.W, H: t.H, Format: PixRGBA8, Usage: TexSampled | TexRenderTarget})
	if err != nil {
		return nil, err
	}
	if err := img.SetPixels(t.Pix); err != nil {
		return nil, err
	}
	return img, nil
}

// mulU8 颜色相乘（归一化）。
func mulU8(a, b uint8) uint8 { return uint8(uint16(a) * uint16(b) / 255) }

// mulRGBA 颜色逐通道相乘。
func mulRGBA(a, b RGBA) RGBA {
	if b.A == 0 {
		b.A = 255
	}
	return RGBA{mulU8(a.R, b.R), mulU8(a.G, b.G), mulU8(a.B, b.B), mulU8(a.A, b.A)}
}

// normalizeV3 归一化（零向量返回 +Y）。
func normalizeV3(v Vec3) Vec3 {
	l := v.X*v.X + v.Y*v.Y + v.Z*v.Z
	if l < 1e-12 {
		return Vec3{0, 1, 0}
	}
	inv := 1 / float32(math.Sqrt(float64(l)))
	return Vec3{v.X * inv, v.Y * inv, v.Z * inv}
}

// ---------- 内置着色程序 ----------

// unlitGLSL 无光照 GLSL（硬件后端）。
const unlitGLSLVert = `#version 330 core
layout(location=0) in vec3 a_pos;
layout(location=1) in vec3 a_normal;
layout(location=2) in vec2 a_uv;
layout(location=3) in vec4 a_color;
uniform mat4 u_mvp;
out vec3 v_normal;
out vec2 v_uv;
out vec4 v_color;
void main() {
    gl_Position = u_mvp * vec4(a_pos, 1.0);
    v_normal = a_normal;
    v_uv = a_uv;
    v_color = a_color;
}`

const unlitGLSLFrag = `#version 330 core
in vec3 v_normal;
in vec2 v_uv;
in vec4 v_color;
uniform sampler2D u_tex;
uniform vec4 u_tint;
uniform int u_hasTex;
out vec4 fragColor;
void main() {
    vec4 c = v_color * u_tint;
    if (u_hasTex == 1) c *= texture(u_tex, v_uv);
    fragColor = c;
}`

const phongGLSLFrag = `#version 330 core
in vec3 v_normal;
in vec2 v_uv;
in vec4 v_color;
uniform sampler2D u_tex;
uniform vec4 u_tint;
uniform int u_hasTex;
uniform vec3 u_lightDir;
uniform vec4 u_lightColor;
uniform float u_ambient;
out vec4 fragColor;
void main() {
    vec4 c = v_color * u_tint;
    if (u_hasTex == 1) c *= texture(u_tex, v_uv);
    vec3 n = normalize(v_normal);
    float diff = max(dot(n, normalize(u_lightDir)), 0.0);
    vec3 lit = c.rgb * (u_ambient + diff * u_lightColor.rgb);
    fragColor = vec4(lit, c.a);
}`

// ShaderUnlit 无光照着色：纹理(slot 0) × 顶点色 × 材质染色 u_tint。
func ShaderUnlit() *ShaderProgram {
	return &ShaderProgram{
		Name: "unlit",
		Vertex: func(in *VertexIn, out *Varyings) {
			out.Pos = in.MVP.TransformVec4(Vec4{in.Position.X, in.Position.Y, in.Position.Z, 1})
			out.UV = in.UV
			out.Color = in.Color
			out.Normal = in.Normal
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			c := in.Color
			if s := in.Tex[0]; s != nil {
				c = mulRGBA(c, s.RGBA8(in.UV.X, in.UV.Y))
			}
			out.Color[0] = mulRGBA(c, uniformTint(in))
		},
		GLSL: &GLSLProgram{
			Vertex:   unlitGLSLVert,
			Fragment: unlitGLSLFrag,
			Uniforms: []string{"u_tint", "u_hasTex"},
		},
	}
}

// uniformTint 取材质染色（未设时白色）。
func uniformTint(in *FragmentIn) RGBA {
	if in.Tint.A == 0 && in.Tint.R == 0 && in.Tint.G == 0 && in.Tint.B == 0 {
		return RGBA{255, 255, 255, 255}
	}
	return in.Tint
}

// ShaderPhong 逐像素 Phong 着色（Lambert 漫反射 + 可选 Blinn-Phong 高光）。
//
// 光照参数由 uniform 提供（u_lightDir / u_lightColor / u_ambient /
// u_specular / u_shininess），所以整个场景只需编译一次。
func ShaderPhong() *ShaderProgram {
	return &ShaderProgram{
		Name: "phong",
		Vertex: func(in *VertexIn, out *Varyings) {
			out.Pos = in.MVP.TransformVec4(Vec4{in.Position.X, in.Position.Y, in.Position.Z, 1})
			out.UV = in.UV
			out.Color = in.Color
			out.Normal = in.Model.TransformDir(in.Normal)
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			c := in.Color
			if s := in.Tex[0]; s != nil {
				c = mulRGBA(c, s.RGBA8(in.UV.X, in.UV.Y))
			}
			c = mulRGBA(c, uniformTint(in))
			n := normalizeV3(in.Normal)
			diff := n.X*in.LightDir.X + n.Y*in.LightDir.Y + n.Z*in.LightDir.Z
			if diff < 0 {
				diff = 0
			}
			light := in.Ambient + diff*float32(in.LightColor.R)/255
			r := clampf(float32(c.R)/255*light, 0, 1)
			g := clampf(float32(c.G)/255*light, 0, 1)
			b := clampf(float32(c.B)/255*light, 0, 1)
			if in.Specular > 0 && diff > 0 {
				sh := in.Shininess
				if sh <= 0 {
					sh = 32
				}
				spec := float32(math.Pow(float64(diff), float64(sh))) * in.Specular
				r = clampf(r+spec, 0, 1)
				g = clampf(g+spec, 0, 1)
				b = clampf(b+spec, 0, 1)
			}
			out.Color[0] = RGBA{floatToU8(r), floatToU8(g), floatToU8(b), c.A}
		},
		GLSL: &GLSLProgram{
			Vertex:   unlitGLSLVert,
			Fragment: phongGLSLFrag,
			Uniforms: []string{"u_tint", "u_hasTex", "u_lightDir", "u_lightColor", "u_ambient"},
		},
	}
}

// ShaderToon 色阶量化着色（卡通渲染），levels 为色阶数（默认 3）。
func ShaderToon(levels int) *ShaderProgram {
	if levels < 2 {
		levels = 3
	}
	return &ShaderProgram{
		Name: "toon",
		Vertex: func(in *VertexIn, out *Varyings) {
			out.Pos = in.MVP.TransformVec4(Vec4{in.Position.X, in.Position.Y, in.Position.Z, 1})
			out.UV = in.UV
			out.Color = in.Color
			out.Normal = in.Model.TransformDir(in.Normal)
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			c := in.Color
			if s := in.Tex[0]; s != nil {
				c = mulRGBA(c, s.RGBA8(in.UV.X, in.UV.Y))
			}
			c = mulRGBA(c, uniformTint(in))
			n := normalizeV3(in.Normal)
			diff := clampf(n.X*in.LightDir.X+n.Y*in.LightDir.Y+n.Z*in.LightDir.Z, 0, 1)
			step := float32(int(diff*float32(levels))) / float32(levels)
			light := in.Ambient + step*(float32(in.LightColor.R)/255)
			out.Color[0] = RGBA{
				floatToU8(float32(c.R) / 255 * light),
				floatToU8(float32(c.G) / 255 * light),
				floatToU8(float32(c.B) / 255 * light),
				c.A,
			}
		},
		GLSL: &GLSLProgram{
			Vertex:   unlitGLSLVert,
			Fragment: phongGLSLFrag,
			Uniforms: []string{"u_tint", "u_hasTex", "u_lightDir", "u_lightColor", "u_ambient"},
		},
	}
}

// ShaderDepthOnly 只写深度（深度预通道 / 阴影贴图），颜色输出白色。
func ShaderDepthOnly() *ShaderProgram {
	return &ShaderProgram{
		Name: "depth-only",
		Vertex: func(in *VertexIn, out *Varyings) {
			out.Pos = in.MVP.TransformVec4(Vec4{in.Position.X, in.Position.Y, in.Position.Z, 1})
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			out.Color[0] = RGBA{255, 255, 255, 255}
		},
		GLSL: &GLSLProgram{
			Vertex: unlitGLSLVert,
			Fragment: `#version 330 core
out vec4 fragColor;
void main() { fragColor = vec4(1.0); }`,
		},
	}
}

// ShaderNormalDebug 法线可视化（世界空间法线 → RGB）。
func ShaderNormalDebug() *ShaderProgram {
	return &ShaderProgram{
		Name: "normal",
		Vertex: func(in *VertexIn, out *Varyings) {
			out.Pos = in.MVP.TransformVec4(Vec4{in.Position.X, in.Position.Y, in.Position.Z, 1})
			out.Normal = in.Model.TransformDir(in.Normal)
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			n := normalizeV3(in.Normal)
			out.Color[0] = RGBA{
				floatToU8(n.X*0.5 + 0.5),
				floatToU8(n.Y*0.5 + 0.5),
				floatToU8(n.Z*0.5 + 0.5),
				255,
			}
		},
		GLSL: &GLSLProgram{
			Vertex: unlitGLSLVert,
			Fragment: `#version 330 core
in vec3 v_normal;
out vec4 fragColor;
void main() {
    vec3 n = normalize(v_normal);
    fragColor = vec4(n * 0.5 + 0.5, 1.0);
}`,
		},
	}
}

// ShaderFullscreen 全屏后处理着色：把 in 参数当作屏幕 UV 自定义。
//
// 配合 DrawArrays(3, 1) 的「全屏三角形」使用（顶点位置由 out.Pos 直接给裁剪空间）。
// fn 收到屏幕 UV（0..1，左上原点）与绑定的 slot 0 采样器，返回颜色。
func ShaderFullscreen(name string, fn func(uv Vec2, tex *TextureSampler, coord Vec2) RGBA) *ShaderProgram {
	positions := [3]Vec2{{-1, -1}, {3, -1}, {-1, 3}}
	return &ShaderProgram{
		Name: name,
		Vertex: func(in *VertexIn, out *Varyings) {
			p := positions[in.VertexID%3]
			out.Pos = Vec4{p.X, p.Y, 0, 1}
			out.UV = Vec2{p.X*0.5 + 0.5, 0.5 - p.Y*0.5}
		},
		Fragment: func(in *FragmentIn, out *FragmentOut) {
			out.Color[0] = fn(in.UV, in.Tex[0], in.Coord)
		},
	}
}

// ShaderBlit 全屏拷贝（后处理链的终点 / 上式法回读）。
func ShaderBlit() *ShaderProgram {
	return ShaderFullscreen("blit", func(uv Vec2, tex *TextureSampler, _ Vec2) RGBA {
		if tex == nil {
			return RGBA{255, 0, 255, 255}
		}
		return tex.RGBA8(uv.X, uv.Y)
	})
}
