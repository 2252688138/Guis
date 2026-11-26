// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// Fragment 自定义片元着色器（Material.FragmentShader）的输入。
//
// 回调里可以按需采样纹理、读光源与阴影，返回最终颜色；
// 返回 A=0 的像素会被丢弃（相当于 discard），其余结果照常经过
// 雾、材质 alpha 与混合，所以不需要自己处理这些。
type Fragment struct {
	// World 世界坐标。
	World Vec3
	// Normal 世界空间法线（已归一化）。
	Normal Vec3
	// Tangent, Bitangent 三角形切线基（无 UV 的网格为零值）。
	Tangent, Bitangent Vec3
	// UV 纹理坐标。
	UV Vec2
	// Color 顶点色。
	Color RGBA
	// MipLod 当前片元的 mipmap 层级（屏幕空间 UV 导数估算）。
	MipLod float32

	ctx *shadeCtx
	mat *Material
}

// Material 当前材质（只读语义，修改会影响后续片元）。
func (f *Fragment) Material() *Material {
	if f == nil {
		return nil
	}
	return f.mat
}

// Lights 场景光源列表。
func (f *Fragment) Lights() []Light {
	if f == nil || f.ctx == nil {
		return nil
	}
	return f.ctx.lights
}

// Ambient 环境光颜色。
func (f *Fragment) Ambient() RGBA {
	if f == nil || f.ctx == nil {
		return RGBA{}
	}
	return f.ctx.ambient
}

// CameraPos 相机世界坐标。
func (f *Fragment) CameraPos() Vec3 {
	if f == nil || f.ctx == nil {
		return Vec3{}
	}
	return f.ctx.camPos
}

// ViewDir 从片元指向相机的单位向量。
func (f *Fragment) ViewDir() Vec3 {
	if f == nil || f.ctx == nil {
		return V3(0, 0, 1)
	}
	d := f.ctx.camPos.Sub(f.World)
	if l := d.Len(); l > 1e-6 {
		return d.Scale(1 / l)
	}
	return V3(0, 0, 1)
}

// Shadow 该片元的受光系数（1 = 完全受光；无阴影时恒为 1）。
func (f *Fragment) Shadow() float32 {
	if f == nil {
		return 1
	}
	return shadowFactor(f.ctx, f.World)
}

// Sky 天空环境（未设置天空时为 nil）。
func (f *Fragment) Sky() *Sky {
	if f == nil || f.ctx == nil {
		return nil
	}
	return f.ctx.sky
}

// Sample 按材质的过滤设置采样一张贴图。
func (f *Fragment) Sample(t *Texture, uv Vec2) RGBA {
	if f == nil {
		return RGB(255, 255, 255)
	}
	near, mip, lod := false, false, float32(0)
	if f.mat != nil {
		near, mip = f.mat.Nearest, f.mat.Mipmap
	}
	lod = f.MipLod
	return sampleTexture(t, uv, near, mip, lod)
}
