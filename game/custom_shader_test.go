// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "testing"

// 自定义片元着色器：按 uv 画棋盘（2 色），并能读光源 / 阴影 / 相机。
func TestCustomFragmentShader(t *testing.T) {
	red, blue := RGB(255, 0, 0), RGB(0, 0, 255)
	var calls int
	mat := DefaultMaterial()
	mat.Shade = ShadeCustom
	mat.FragmentShader = func(f *Fragment) RGBA {
		calls++
		// 这些访问器都必须安全可读
		_ = f.Lights()
		_ = f.Ambient()
		_ = f.CameraPos()
		_ = f.ViewDir()
		_ = f.Shadow()
		_ = f.Sky()
		_ = f.Material()
		if int(f.UV.X*4)%2 == int(f.UV.Y*4)%2 {
			return red
		}
		return blue
	}

	ctx := &shadeCtx{ambient: RGB(40, 40, 40), camPos: V3(0, 0, 5)}
	ctx.lights = append(ctx.lights, SunLight(V3(0, -1, 0)))

	got := shadeFragment(ctx, mat, V3(0, 0, 0), V3(0, 1, 0), V3(1, 0, 0), V3(0, 0, 1), Vec2{X: 0.1, Y: 0.1}, RGB(255, 255, 255))
	if calls != 1 {
		t.Fatalf("回调应被调用 1 次，实际 %d", calls)
	}
	if got.R < 200 || got.G > 60 {
		t.Fatalf("uv(0.1,0.1) 应取到红色，实际 %v", got)
	}
	got = shadeFragment(ctx, mat, V3(0, 0, 0), V3(0, 1, 0), V3(1, 0, 0), V3(0, 0, 1), Vec2{X: 0.35, Y: 0.1}, RGB(255, 255, 255))
	if got.B < 200 || got.R > 60 {
		t.Fatalf("uv(0.35,0.1) 应取到蓝色，实际 %v", got)
	}
}

// 回调返回 A=0 的像素被丢弃。
func TestCustomShaderDiscard(t *testing.T) {
	mat := DefaultMaterial()
	mat.Shade = ShadeCustom
	mat.FragmentShader = func(f *Fragment) RGBA { return RGBA{} }
	ctx := &shadeCtx{}
	got := shadeFragment(ctx, mat, V3(0, 0, 0), V3(0, 1, 0), Vec3{}, Vec3{}, Vec2{}, RGB(255, 255, 255))
	if got.A != 0 {
		t.Fatalf("A=0 应丢弃，实际 %v", got)
	}
}

// 没设置回调时回退 Phong，不能崩也不能全黑。
func TestCustomShaderFallbackPhong(t *testing.T) {
	mat := DefaultMaterial()
	mat.Shade = ShadeCustom
	mat.FragmentShader = nil
	ctx := &shadeCtx{ambient: RGB(120, 120, 120)}
	ctx.lights = append(ctx.lights, SunLight(V3(0, -1, 0)))
	got := shadeFragment(ctx, mat, V3(0, 0, 0), V3(0, 1, 0), Vec3{}, Vec3{}, Vec2{}, RGB(255, 255, 255))
	if got.A == 0 {
		t.Fatalf("回退 Phong 不应透明：%v", got)
	}
	if got.R < 30 {
		t.Fatalf("回退 Phong 应有光照：%v", got)
	}
	// 材质的 Shade 不能被回退路径改掉
	if mat.Shade != ShadeCustom {
		t.Fatalf("回退不应修改调用方材质，实际 %v", mat.Shade)
	}
}

// 自定义着色器结果照常走雾。
func TestCustomShaderFogApplied(t *testing.T) {
	mat := DefaultMaterial()
	mat.Shade = ShadeCustom
	mat.FragmentShader = func(f *Fragment) RGBA { return RGB(255, 255, 255) }
	ctx := &shadeCtx{
		camPos: V3(0, 0, 0),
		fog:    Fog{On: true, Color: RGB(0, 0, 0), Near: 1, Far: 11},
	}
	near := shadeFragment(ctx, mat, V3(0, 0, 2), V3(0, 1, 0), Vec3{}, Vec3{}, Vec2{}, RGB(255, 255, 255))
	far := shadeFragment(ctx, mat, V3(0, 0, 10), V3(0, 1, 0), Vec3{}, Vec3{}, Vec2{}, RGB(255, 255, 255))
	if far.R >= near.R {
		t.Fatalf("远处的雾应更浓（更暗）：near=%v far=%v", near, far)
	}
}
