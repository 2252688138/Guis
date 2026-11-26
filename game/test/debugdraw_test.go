// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image"
	"testing"

	"guis/game"
)

func testCam() *game.Camera {
	c := game.NewCamera()
	c.Pos = game.V3(0, 0, -5)
	c.Target = game.V3(0, 0, 0)
	c.Up = game.V3(0, 1, 0)
	return c
}

func TestDebugDrawLinePixels(t *testing.T) {
	d := game.NewDebugDraw(testCam())
	d.Clear()
	d.Line(game.V3(-1, 0, 0), game.V3(1, 0, 0), game.RGB(255, 0, 0))
	if d.Lines != 1 {
		t.Fatalf("线段计数 = %d", d.Lines)
	}
	dst := image.NewRGBA(image.Rect(0, 0, 160, 120))
	d.DrawSprites(dst, 160, 120)
	if d.Pixels == 0 {
		t.Fatal("应写入像素")
	}
	// 中点附近应有红色像素
	found := false
	for y := 50; y < 70 && !found; y++ {
		for x := 60; x < 100; x++ {
			px := dst.RGBAAt(x, y)
			if px.R > 200 && px.G < 60 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("屏幕中心附近应有红线")
	}
}

func TestDebugDrawCounts(t *testing.T) {
	d := game.NewDebugDraw(testCam())
	d.Clear()
	d.Box(game.V3(-1, -1, -1), game.V3(1, 1, 1), game.RGBA{})
	if d.Lines != 12 {
		t.Fatalf("AABB 边数 = %d，期望 12", d.Lines)
	}
	d.Clear()
	d.Sphere(game.V3(0, 0, 0), 1, game.RGB(0, 0, 255))
	if d.Lines != 48 {
		t.Fatalf("球体线段数 = %d，期望 48", d.Lines)
	}
	d.Clear()
	d.Axes(game.V3(0, 0, 0), 2)
	if d.Lines != 3 {
		t.Fatalf("坐标轴线段数 = %d，期望 3", d.Lines)
	}
	d.Clear()
	d.Ray(game.V3(0, 0, 0), game.V3(1, 1, 0), 3, game.RGBA{})
	if d.Lines != 1 {
		t.Fatalf("射线线段数 = %d", d.Lines)
	}
}

func TestDebugDrawBehindCameraSkipped(t *testing.T) {
	d := game.NewDebugDraw(testCam()) // 相机在 z=-5 看 +Z
	d.Clear()
	d.Line(game.V3(-1, 0, -20), game.V3(1, 0, -20), game.RGB(0, 255, 0))
	dst := image.NewRGBA(image.Rect(0, 0, 80, 60))
	d.DrawSprites(dst, 80, 60)
	if d.Pixels != 0 {
		t.Fatalf("相机后方线段不应绘制，实际写入 %d", d.Pixels)
	}
}

func TestDebugDrawNilSafe(t *testing.T) {
	var d *game.DebugDraw
	d.Clear()
	d.Line(game.V3(0, 0, 0), game.V3(1, 1, 1), game.RGBA{})
	d.Box(game.V3(0, 0, 0), game.V3(1, 1, 1), game.RGBA{})
	d.DrawSprites(image.NewRGBA(image.Rect(0, 0, 4, 4)), 4, 4)
}
