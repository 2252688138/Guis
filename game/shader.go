// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// ShadeModel 着色模型。
type ShadeModel uint8

const (
	// ShadeUnlit 不受光照影响（纯色或纹理）。
	ShadeUnlit ShadeModel = iota
	// ShadeFlat 面法线光照（低频，像素风常用）。
	ShadeFlat
	// ShadeGouraud 顶点光照后插值（最便宜）。
	ShadeGouraud
	// ShadePhong 逐像素光照（默认，效果最好）。
	ShadePhong
	// ShadeToon 色阶量化（卡通渲染）。
	ShadeToon
	// ShadePBR 基于物理的金属度/粗糙度着色（Cook-Torrance GGX）。
	// 配合 Material.Metallic / Roughness / NormalMap 使用，见 pbr.go。
	ShadePBR
	// ShadeCustom 调用 Material.FragmentShader 自定义片元着色器（仅 soft 后端）。
	// 回调为 nil 时自动回退 ShadePhong。
	ShadeCustom
)

// Material 材质。
// AlphaMode 材质透明度处理方式。
type AlphaMode uint8

const (
	// AlphaBlend 半透明混色（默认）：进入透明通道，按深度从远到近排序后绘制。
	AlphaBlend AlphaMode = iota
	// AlphaOpaque 忽略 alpha 一律当不透明画（写深度，适合“不想要透明却误传了 alpha”的情形）。
	AlphaOpaque
	// AlphaCutout 镂空：按 AlphaCutoff 逐像素裁剪，剩下的不透明写深度（树叶 / 栅栏 / 草）。
	AlphaCutout
)

type Material struct {
	BaseColor   RGBA
	Texture     *Texture
	Shade       ShadeModel
	Specular    float32
	Shininess   float32
	Emissive    RGBA
	Alpha       float32
	DoubleSided bool
	Wireframe   bool
	Nearest     bool
	// Mipmap 启用 mipmap 三线性过滤（需先 Texture.BuildMips，远距离纹理不闪烁）。
	Mipmap bool
	// MipBias 手动 mip 偏移（正 = 更模糊，负 = 更锐利）。
	MipBias float32
	// AlphaMode 透明度处理方式（零值 = AlphaBlend，与旧行为一致）。
	AlphaMode AlphaMode
	// AlphaCutoff Cutout 模式的丢弃阈值（0~1，≤0 时按 0.5）。
	AlphaCutoff float32

	// Metallic 金属度 0~1（只对 ShadePBR 生效）：1 = 纯金属，无漫反射、高光带自身颜色。
	Metallic float32
	// Roughness 粗糙度 0.04~1（只对 ShadePBR 生效，≤0 时按 0.5）：越大高光越散。
	Roughness float32
	// NormalMap 切线空间法线贴图（只对 ShadePBR 生效）。
	// 切线由三角形世界坐标与 UV 推导，不需要网格自带切线数据。
	NormalMap *Texture
	// NormalScale 法线强度（≤0 时按 1；<1 减弱凹凸，>1 夸张）。
	NormalScale float32
	// MetallicRoughnessMap glTF 打包贴图：G = 粗糙度、B = 金属度（与 Roughness/Metallic 相乘）。
	MetallicRoughnessMap *Texture
	// AOMap 环境遮蔽贴图（取 R 通道，只影响环境光项）。
	AOMap *Texture
	// FragmentShader 自定义片元着色器（ShadeCustom 时调用）：
	// 返回 A=0 表示丢弃该像素；结果仍会走雾 / 材质 alpha。
	FragmentShader func(f *Fragment) RGBA
}

// DefaultMaterial 默认材质（白灰、Phong 光照）。
func DefaultMaterial() *Material {
	return &Material{
		BaseColor: RGB(220, 220, 225),
		Shade:     ShadePhong,
		Shininess: 32,
		Alpha:     1,
	}
}

// WithColor 设置基础色并返回自身（链式）。
func (m *Material) WithColor(c RGBA) *Material { m.BaseColor = c; return m }

// WithTexture 设置纹理并返回自身（链式）。
func (m *Material) WithTexture(t *Texture) *Material { m.Texture = t; return m }

// WithShade 设置着色模型并返回自身（链式）。
func (m *Material) WithShade(s ShadeModel) *Material { m.Shade = s; return m }

// Clone 深拷贝材质（纹理共享）。
func (m *Material) Clone() *Material {
	if m == nil {
		return DefaultMaterial()
	}
	c := *m
	return &c
}

// LightKind 光源类型。
type LightKind uint8

const (
	// LightDirectional 平行光（如太阳）。
	LightDirectional LightKind = iota
	// LightPoint 点光源（带距离衰减）。
	LightPoint
)

// Light 光源。
type Light struct {
	Kind      LightKind
	Dir       Vec3
	Pos       Vec3
	Color     RGBA
	Intensity float32
	Range     float32
}

// SunLight 构造默认强度的方向光（方向为光线照射方向）。
func SunLight(dir Vec3) Light {
	return Light{Kind: LightDirectional, Dir: dir.Normalize(), Color: RGB(255, 255, 255), Intensity: 1}
}

// PointLight 构造点光源。
func PointLight(pos Vec3, rng, intensity float32) Light {
	return Light{Kind: LightPoint, Pos: pos, Color: RGB(255, 255, 255), Intensity: intensity, Range: rng}
}

// Fog 线性距离雾。
type Fog struct {
	On    bool
	Color RGBA
	Near  float32
	Far   float32
}

// shadeCtx 一次渲染的着色上下文（光源、环境光、雾、相机位置）。
type shadeCtx struct {
	lights  []Light
	ambient RGBA
	fog     Fog
	camPos  Vec3
	// shadow 方向光阴影图（nil / 未就绪时不做阴影）。
	shadow *ShadowMap
	// cascades 级联阴影（nil / 未就绪时回退 shadow）。
	cascades *CSMShadow
	// sky 天空环境（IBL 近似用，nil = 没设天空、环境光用常数）。
	sky *Sky
	// mipLod 当前片元的 mipmap 层级（由光栅器按屏幕空间 UV 导数估算）。
	mipLod float32
}

// shadowFactor 返回世界坐标处的受光系数（1 = 完全受光，1-Strength = 全阴影）。
//
// 优先用级联阴影（ctx.cascades），否则回退单张阴影图；都没有时恒为 1。
func shadowFactor(ctx *shadeCtx, world Vec3) float32 {
	if ctx == nil {
		return 1
	}
	if ctx.cascades != nil {
		return ctx.cascades.FactorCSM(world, ctx.camPos)
	}
	if ctx.shadow != nil {
		return ctx.shadow.Factor(world)
	}
	return 1
}

// sampleTexture 按过滤方式采样（nearest / 双线性 / 三线性 mipmap）。
func sampleTexture(t *Texture, uv Vec2, nearest, mip bool, lod float32) RGBA {
	if t == nil {
		return RGB(255, 255, 255)
	}
	if nearest {
		return t.SampleNearest(uv.X, uv.Y)
	}
	if mip && len(t.mips) > 0 {
		return t.SampleTrilinear(uv.X, uv.Y, lod)
	}
	return t.SampleLinear(uv.X, uv.Y)
}

// shadeFragment 片元着色：材质色 × 顶点色 × 纹理，再叠加光源与雾。
//
// n 为世界空间法线（调用方保证已归一化），world 为世界坐标；
// tangent / bitangent 是三角形切线基（只有 ShadePBR 的法线贴图用到，其余材质可传零值）。
func shadeFragment(ctx *shadeCtx, m *Material, world, n, tangent, bitangent Vec3, uv Vec2, vcol RGBA) RGBA {
	if m.Shade == ShadeCustom {
		if m.FragmentShader == nil {
			// 没回调就是普通 Phong（浅拷贝，不改调用方材质）
			fallback := *m
			fallback.Shade = ShadePhong
			m = &fallback
		} else {
			f := Fragment{
				World: world, Normal: n, Tangent: tangent, Bitangent: bitangent,
				UV: uv, Color: vcol, MipLod: ctx.mipLod, ctx: ctx, mat: m,
			}
			c := m.FragmentShader(&f)
			if c.A == 0 {
				return RGBA{}
			}
			c = applyFog(ctx, c, world)
			if m.Alpha < 1 {
				c.A = uint8(float32(c.A) * Clamp(m.Alpha, 0, 1))
			}
			return c
		}
	}
	if m.Shade == ShadePBR {
		return pbrShade(ctx, m, world, n, tangent, bitangent, uv, vcol)
	}
	base := m.BaseColor.Mul(vcol)
	if m.Texture != nil {
		base = base.Mul(sampleTexture(m.Texture, uv, m.Nearest, m.Mipmap, ctx.mipLod+m.MipBias))
	}

	var lit RGBA
	if m.Shade == ShadeUnlit {
		lit = base
	} else {
		// 环境光
		ar := float32(ctx.ambient.R) / 255
		ag := float32(ctx.ambient.G) / 255
		ab := float32(ctx.ambient.B) / 255
		lr, lg, lb := float32(base.R)/255*ar, float32(base.G)/255*ag, float32(base.B)/255*ab

		view := Vec3{}
		if m.Specular > 0 {
			view = ctx.camPos.Sub(world).Normalize()
		}

		// 阴影：整片元采样一次，场景内所有光源共用同一张阴影图
		shadowK := shadowFactor(ctx, world)

		for i := range ctx.lights {
			lt := &ctx.lights[i]
			var ldir Vec3 // 指向光源的方向
			var atten float32 = 1
			if lt.Kind == LightDirectional {
				ldir = lt.Dir.Normalize().Neg()
			} else {
				d := lt.Pos.Sub(world)
				dist := d.Len()
				if dist <= 0 {
					continue
				}
				ldir = d.Scale(1 / dist)
				if lt.Range > 0 {
					atten = Clamp(1-dist/lt.Range, 0, 1)
					atten *= atten
				}
			}
			diff := n.Dot(ldir)
			if diff <= 0 {
				if lt.Kind == LightPoint {
					continue
				}
				diff = 0
			}
			k := diff * lt.Intensity * atten * shadowK
			if m.Shade == ShadeToon {
				// 3 阶色阶量化
				switch {
				case diff > 0.66:
					k = lt.Intensity * atten
				case diff > 0.33:
					k = lt.Intensity * atten * 0.6
				default:
					k = 0
				}
			}
			if k > 0 {
				lr += float32(base.R) / 255 * float32(lt.Color.R) / 255 * k
				lg += float32(base.G) / 255 * float32(lt.Color.G) / 255 * k
				lb += float32(base.B) / 255 * float32(lt.Color.B) / 255 * k
			}
			// 高光
			if m.Specular > 0 && k > 0 {
				h := ldir.Add(view).Normalize()
				spec := n.Dot(h)
				if spec > 0 {
					s := float32(math.Pow(float64(spec), float64(maxf(m.Shininess, 1))))
					s *= m.Specular * lt.Intensity * atten * shadowK
					lr += float32(lt.Color.R) / 255 * s
					lg += float32(lt.Color.G) / 255 * s
					lb += float32(lt.Color.B) / 255 * s
				}
			}
		}
		lit = RGBA{
			R: clamp255(lr*255 + float32(m.Emissive.R)),
			G: clamp255(lg*255 + float32(m.Emissive.G)),
			B: clamp255(lb*255 + float32(m.Emissive.B)),
			A: base.A,
		}
	}

	lit = applyFog(ctx, lit, world)

	if m.Alpha < 1 {
		lit.A = uint8(float32(lit.A) * Clamp(m.Alpha, 0, 1))
	}
	return lit
}

// applyFog 按相机距离对片元颜色做线性雾混合（.On 为 false 时原样返回）。
func applyFog(ctx *shadeCtx, c RGBA, world Vec3) RGBA {
	if ctx == nil || !ctx.fog.On {
		return c
	}
	d := ctx.camPos.Dist(world)
	t := float32(0)
	if ctx.fog.Far > ctx.fog.Near {
		t = Clamp((d-ctx.fog.Near)/(ctx.fog.Far-ctx.fog.Near), 0, 1)
	}
	return c.Lerp(ctx.fog.Color.WithAlpha(c.A), t)
}

// blendSrcOver 将 src 叠加到 dst 上（直通 alpha）。
func blendSrcOver(dst, src RGBA) RGBA {
	if src.A == 0 {
		return dst
	}
	if src.A == 255 {
		return src
	}
	sa := float32(src.A) / 255
	da := float32(dst.A) / 255
	oa := sa + da*(1-sa)
	if oa <= 0 {
		return RGBA{}
	}
	mix := func(s, d uint8) uint8 {
		v := (float32(s)*sa + float32(d)*da*(1-sa)) / oa
		return clamp255(v)
	}
	return RGBA{mix(src.R, dst.R), mix(src.G, dst.G), mix(src.B, dst.B), clamp255(oa * 255)}
}

func clamp255(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}
