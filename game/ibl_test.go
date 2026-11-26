// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "testing"

// lumOf 亮度（测试用，0.299R+0.587G+0.114B）。
func lumOf(c RGBA) float32 {
	return 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
}

// 渐变天空：天顶偏蓝、天底偏暗，方向采样要能区分开。
func TestSkyColorGradient(t *testing.T) {
	sky := GradientSky(RGB(20, 30, 220), RGB(200, 120, 60), RGB(30, 30, 30))
	up := skyColor(sky, V3(0, 1, 0))
	horizon := skyColor(sky, V3(1, 0, 0))
	down := skyColor(sky, V3(0, -1, 0))
	if up.B <= horizon.B {
		t.Fatalf("天顶应更蓝：up=%v horizon=%v", up, horizon)
	}
	if down.R >= horizon.R {
		t.Fatalf("天底应更暗：down=%v horizon=%v", down, horizon)
	}
}

func TestSkyColorDisabled(t *testing.T) {
	if c := skyColor(Sky{}, V3(0, 1, 0)); c != (RGBA{}) {
		t.Fatalf("未启用天空应返回零色，实际 %v", c)
	}
}

// pbrShade 在设了天空时应按法线方向取环境色：朝上的面比朝下的面更亮（天顶更亮）。
func TestPBRUsesSkyIrradiance(t *testing.T) {
	sky := GradientSky(RGB(240, 240, 255), RGB(120, 140, 160), RGB(10, 10, 10))
	mat := DefaultMaterial()
	mat.Shade = ShadePBR
	mat.Metallic = 0
	mat.Roughness = 0.9
	mat.BaseColor = RGB(255, 255, 255)

	ctx := &shadeCtx{ambient: RGB(0, 0, 0), sky: &sky}
	up := pbrShade(ctx, mat, V3(0, 0, 0), V3(0, 1, 0), V3(1, 0, 0), V3(0, 0, 1), Vec2{}, RGB(255, 255, 255))
	down := pbrShade(ctx, mat, V3(0, 0, 0), V3(0, -1, 0), V3(1, 0, 0), V3(0, 0, 1), Vec2{}, RGB(255, 255, 255))
	if lumOf(up) <= lumOf(down) {
		t.Fatalf("朝上的面应比朝下的面亮：up=%v down=%v", up, down)
	}
}

// 没有天空时必须与原来的常数环境路径一致（回归）。
func TestPBRFallsBackToAmbient(t *testing.T) {
	mat := DefaultMaterial()
	mat.Shade = ShadePBR
	mat.Metallic = 0
	mat.Roughness = 0.5
	mat.BaseColor = RGB(255, 255, 255)

	amb := RGB(90, 90, 90)
	ctx := &shadeCtx{ambient: amb}
	got := pbrShade(ctx, mat, V3(0, 0, 0), V3(0, 1, 0), V3(1, 0, 0), V3(0, 0, 1), Vec2{}, RGB(255, 255, 255))
	if got.A == 0 {
		t.Fatal("没有天空时不应返回全透明")
	}
	if lumOf(got) < 20 {
		t.Fatalf("常数环境光下应可见，实际 %v", got)
	}
	// 与手算的常数近似一致性：环境项约等于 amb * albedo(kd=1) + amb*F0(1-rough)
	want := float32(90) * (1 - 0.04*0.5)
	if diff := lumOf(got) - want; diff > 25 || diff < -25 {
		t.Fatalf("常数环境路径偏离手算过多：got=%.1f want≈%.1f", lumOf(got), want)
	}
}
