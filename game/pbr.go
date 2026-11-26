// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// 基于物理的着色（PBR，金属度 / 粗糙度工作流）。
//
//	mat := game.DefaultMaterial()
//	mat.Shade = game.ShadePBR
//	mat.Metallic = 1      // 金属（无漫反射，高光带自身颜色）
//	mat.Roughness = 0.35  // 越光滑高光越集中
//	mat.NormalMap = normal
//
// 实现是 Cook-Torrance：GGX 法线分布 + Smith 几何项 + Schlick 菲涅尔；
// 环境光用「漫反射 albedo + 常数 F0」近似（没有 IBL 预滤波，不追求电影级）。
// 软件光栅下片元成本约为 Phong 的 3 倍，只在需要质感的物体上开，
// 大面积背景仍用 ShadePhong / ShadeGouraud。
const (
	// pbrDielectric 电介质（非金属）基础反射率。
	pbrDielectric = 0.04
	// pbrMinRough 最小粗糙度（0 会让高光退化成单像素噪点）。
	pbrMinRough = 0.04
	// pbrInvPi 1/π（漫反射积分的常量，避免逐片元除法）。
	pbrInvPi = 0.3183098861837907
)

// pbrShade ShadePBR 材质的片元着色。
//
// n 为世界空间法线；tangent / bitangent 是三角形切线基（由光栅器按世界坐标与
// UV 推导，退化时内部自动换成任意正交基）。
func pbrShade(ctx *shadeCtx, m *Material, world, n, tangent, bitangent Vec3, uv Vec2, vcol RGBA) RGBA {
	// 基础色 = 材质色 × 顶点色 × 贴图（alpha 一并相乘，与 Phong 路径一致）
	base := m.BaseColor.Mul(vcol)
	if m.Texture != nil {
		base = base.Mul(sampleTexture(m.Texture, uv, m.Nearest, m.Mipmap, ctx.mipLod+m.MipBias))
	}
	albedo := pbrColor(base)

	metallic := Clamp(m.Metallic, 0, 1)
	rough := m.Roughness
	if rough <= 0 {
		rough = 0.5
	}
	rough = Clamp(rough, pbrMinRough, 1)
	ao := float32(1)

	if m.MetallicRoughnessMap != nil {
		c := sampleTexture(m.MetallicRoughnessMap, uv, m.Nearest, m.Mipmap, ctx.mipLod+m.MipBias)
		rough = Clamp(rough*float32(c.G)/255, pbrMinRough, 1)
		metallic = Clamp(metallic*float32(c.B)/255, 0, 1)
	}
	if m.AOMap != nil {
		c := sampleTexture(m.AOMap, uv, m.Nearest, m.Mipmap, ctx.mipLod+m.MipBias)
		ao = float32(c.R) / 255
	}
	if m.NormalMap != nil {
		n = pbrNormal(m, ctx, n, tangent, bitangent, uv)
	}
	if l := n.Len(); l > 1e-6 {
		n = n.Scale(1 / l)
	} else {
		n = V3(0, 1, 0)
	}

	// 视线方向与 N·V
	view := ctx.camPos.Sub(world)
	if l := view.Len(); l > 1e-6 {
		view = view.Scale(1 / l)
	} else {
		view = V3(0, 0, 1)
	}
	nDotV := maxf(n.Dot(view), 0.001)

	// F0 = mix(0.04, albedo, metallic)；kd 为漫反射占比
	f0 := V3(pbrDielectric, pbrDielectric, pbrDielectric).Lerp(albedo, metallic)
	kd := 1 - metallic
	// 漫反射项与光源无关，提前算好（每片元只算一次）
	diffBase := albedo.Scale(kd * pbrInvPi)

	// 环境光：有天空时按方向采样（IBL 近似），否则用常数环境色
	var out Vec3
	if ctx.sky != nil && ctx.sky.Enabled() {
		inten := ctx.sky.Intensity
		if inten <= 0 {
			inten = 1
		}
		irr := pbrColor(skyColor(*ctx.sky, n)).Scale(inten)
		// 镜面环境：按反射方向采样，粗糙度越高越向漫反射平均靠拢（粗糙化）
		rv := n.Scale(2 * n.Dot(view)).Sub(view)
		envSpec := pbrColor(skyColor(*ctx.sky, rv)).Scale(inten).Lerp(irr, rough)
		out = irr.Mul(albedo.Scale(kd * ao)).Add(envSpec.Mul(f0.Scale(ao * (1 - rough))))
	} else {
		amb := pbrColor(ctx.ambient)
		out = amb.Mul(albedo.Scale(kd * ao)).Add(amb.Mul(f0.Scale(ao * (1 - rough))))
	}

	shadowK := float32(1)
	if f := shadowFactor(ctx, world); f > 0 {
		shadowK = f
	}

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
		nDotL := n.Dot(ldir)
		if nDotL <= 0 {
			continue
		}
		rad := pbrColor(lt.Color).Scale(lt.Intensity * atten * shadowK)
		if rad.X <= 0 && rad.Y <= 0 && rad.Z <= 0 {
			continue
		}
		h := ldir.Add(view)
		if l := h.Len(); l > 1e-6 {
			h = h.Scale(1 / l)
		}
		nDotH := maxf(n.Dot(h), 0)
		vDotH := maxf(view.Dot(h), 0)

		d := ggxNDF(nDotH, rough)
		g := smithGGX(nDotV, nDotL, rough)
		spec := schlickFresnel(vDotH, f0).Scale(d * g / (4 * nDotV * nDotL))
		out = out.Add(diffBase.Add(spec).Mul(rad).Scale(nDotL))
	}

	// 自发光
	out = out.Add(pbrColor(m.Emissive))

	lit := pbrRGBA(out, base.A)
	if ctx.fog.On {
		d := ctx.camPos.Dist(world)
		t := float32(0)
		if ctx.fog.Far > ctx.fog.Near {
			t = Clamp((d-ctx.fog.Near)/(ctx.fog.Far-ctx.fog.Near), 0, 1)
		}
		lit = lit.Lerp(ctx.fog.Color.WithAlpha(lit.A), t)
	}
	if m.Alpha < 1 {
		lit.A = uint8(float32(lit.A) * Clamp(m.Alpha, 0, 1))
	}
	return lit
}

// triangleTangent 由三角形世界坐标与 UV 推导几何切线基（T = ∂P/∂u，B = ∂P/∂v）。
// UV 退化（行列式为 0，或网格没有 UV）时返回零向量，由 pbrNormal 兑底成正交基。
func triangleTangent(a, b, c scrVtx) (Vec3, Vec3) {
	duv1 := b.uv.Sub(a.uv)
	duv2 := c.uv.Sub(a.uv)
	det := duv1.X*duv2.Y - duv1.Y*duv2.X
	if math.Abs(float64(det)) < 1e-8 {
		return Vec3{}, Vec3{}
	}
	dp1 := b.world.Sub(a.world)
	dp2 := c.world.Sub(a.world)
	inv := 1 / det
	t := V3(
		dp1.X*duv2.Y-dp2.X*duv1.Y,
		dp1.Y*duv2.Y-dp2.Y*duv1.Y,
		dp1.Z*duv2.Y-dp2.Z*duv1.Y,
	).Scale(inv)
	bt := V3(
		dp2.X*duv1.X-dp1.X*duv2.X,
		dp2.Y*duv1.X-dp1.Y*duv2.X,
		dp2.Z*duv1.X-dp1.Z*duv2.X,
	).Scale(inv)
	return t, bt
}

// pbrNormal 采样切线空间法线贴图并变换到世界空间。
func pbrNormal(m *Material, ctx *shadeCtx, n, tangent, bitangent Vec3, uv Vec2) Vec3 {
	c := sampleTexture(m.NormalMap, uv, m.Nearest, m.Mipmap, ctx.mipLod+m.MipBias)
	scale := m.NormalScale
	if scale <= 0 {
		scale = 1
	}
	nx := (float32(c.R)/255*2 - 1) * scale
	ny := (float32(c.G)/255*2 - 1) * scale
	nz := float32(c.B)/255*2 - 1

	t, b := tangent, bitangent
	if t.Len() < 1e-6 || b.Len() < 1e-6 {
		t, b = pbrBasis(n)
	} else {
		// Gram-Schmidt 正交化，副切线方向跟随原推导结果
		t = t.Sub(n.Scale(n.Dot(t)))
		if l := t.Len(); l > 1e-6 {
			t = t.Scale(1 / l)
		} else {
			t, b = pbrBasis(n)
		}
		if t.Len() > 1e-6 {
			want := n.Cross(t)
			if l := want.Len(); l > 1e-6 {
				want = want.Scale(1 / l)
			}
			if want.Dot(b) < 0 {
				want = want.Neg()
			}
			b = want
		}
	}
	r := t.Scale(nx).Add(b.Scale(ny)).Add(n.Scale(nz))
	if r.Len() < 1e-6 {
		return n
	}
	return r
}

// pbrBasis 由法线构造任意正交基（网格没有切线数据时兜底）。
func pbrBasis(n Vec3) (Vec3, Vec3) {
	up := V3(0, 1, 0)
	if float32(math.Abs(float64(n.Y))) > 0.95 {
		up = V3(1, 0, 0)
	}
	t := up.Cross(n)
	if l := t.Len(); l > 1e-6 {
		t = t.Scale(1 / l)
	} else {
		t = V3(1, 0, 0)
	}
	return t, n.Cross(t)
}

// pbrColor RGBA → 线性 0~1 颜色向量。
func pbrColor(c RGBA) Vec3 {
	return V3(float32(c.R)/255, float32(c.G)/255, float32(c.B)/255)
}

// pbrRGBA 颜色向量 → RGBA（每通道钳到 0~255）。
func pbrRGBA(v Vec3, a uint8) RGBA {
	return RGBA{R: clamp255(v.X * 255), G: clamp255(v.Y * 255), B: clamp255(v.Z * 255), A: a}
}

// ggxNDF Trowbridge-Reitz（GGX）法线分布函数。
func ggxNDF(nDotH, rough float32) float32 {
	a := rough * rough
	a2 := a * a
	d := nDotH*nDotH*(a2-1) + 1
	if d <= 1e-9 {
		return 0
	}
	return a2 / (math.Pi * d * d)
}

// smithGGX Smith 几何项（Schlick-GGX 近似，直接光）。
func smithGGX(nDotV, nDotL, rough float32) float32 {
	a := rough * rough
	k := a * 0.5
	return smithG1(nDotV, k) * smithG1(nDotL, k)
}

// smithG1 单方向几何遮蔽。
func smithG1(n, k float32) float32 {
	if n <= 0 {
		return 0
	}
	return n / (n*(1-k) + k)
}

// schlickFresnel Schlick 菲涅尔项（用乘法算 5 次幂，比 math.Pow 快很多）。
func schlickFresnel(vDotH float32, f0 Vec3) Vec3 {
	x := 1 - vDotH
	x2 := x * x
	p := x2 * x2 * x
	return f0.Add(V3(1, 1, 1).Sub(f0).Scale(p))
}
