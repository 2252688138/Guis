// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

// renderQuad 渲染一张面向相机、指定材质的面片，返回中心像素。
func renderQuad(t *testing.T, mode game.AlphaMode, alpha uint8, cutoff float32) game.RGBA {
	t.Helper()
	mesh := game.NewMesh([]game.Vec3{
		{X: -1, Y: -1, Z: -3},
		{X: 1, Y: -1, Z: -3},
		{X: 1, Y: 1, Z: -3},
		{X: -1, Y: 1, Z: -3},
	}, []uint32{0, 1, 2, 0, 2, 3})
	mat := game.DefaultMaterial()
	mat.BaseColor = game.Color(255, 0, 0, alpha)
	mat.Shade = game.ShadeUnlit
	mat.AlphaMode = mode
	mat.AlphaCutoff = cutoff

	n := game.NewNode("quad")
	n.Mesh = mesh
	n.Material = mat

	sc := game.NewScene()
	sc.Root.Children = nil
	sc.Root.Children = append(sc.Root.Children, n)
	sc.Camera.Pos = game.Vec3{}
	sc.Camera.Target = game.Vec3{Z: -1}

	r := &game.Renderer{W: 64, H: 48, BgColor: game.Color(0, 0, 0, 255)}
	img := r.Frame(sc)
	c := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	return game.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func TestAlphaModeOpaqueIgnoresAlpha(t *testing.T) {
	c := renderQuad(t, game.AlphaOpaque, 40, 0)
	if c.R < 200 {
		t.Fatalf("AlphaOpaque 应忽略 alpha 直接画，中心像素 = %v", c)
	}
}

func TestAlphaModeCutout(t *testing.T) {
	// 低于阈值的像素被丢弃（看到背景黑）
	low := renderQuad(t, game.AlphaCutout, 40, 0.5)
	if low.R > 40 {
		t.Fatalf("低于阈值的像素应镂空，中心像素 = %v", low)
	}
	// 高于阈值的像素正常画
	high := renderQuad(t, game.AlphaCutout, 230, 0.5)
	if high.R < 200 {
		t.Fatalf("高于阈值的像素应不透明画出，中心像素 = %v", high)
	}
}

func TestAlphaModeBlend(t *testing.T) {
	// 半透明：黑白背景叠加红 alpha=128 → R 约 128，仍在两栖之间
	c := renderQuad(t, game.AlphaBlend, 128, 0)
	if c.R < 60 || c.R > 200 {
		t.Fatalf("半透明混色应得到中间值，中心像素 = %v", c)
	}
}
