// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

func newBillboardScene(t *testing.T) (*game.Renderer, *game.Scene) {
	t.Helper()
	r := &game.Renderer{W: 80, H: 60, BgColor: game.Color(0, 0, 0, 255)}
	sc := game.NewScene()
	sc.Root.Children = nil
	sc.Camera.Pos = game.Vec3{}
	sc.Camera.Target = game.Vec3{Z: -1}
	sc.Billboards = game.NewBillboardSet()
	return r, sc
}

func TestBillboardIconDraws(t *testing.T) {
	r, sc := newBillboardScene(t)
	sc.Billboards.Add(game.Billboard{
		Pos:   game.Vec3{Z: -3},
		Kind:  game.BillboardIcon,
		Width: 2, Height: 2,
		Color: game.Color(255, 0, 0, 255),
	})
	img := r.Frame(sc)
	c := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	if c.R < 200 || c.G > 40 || c.B > 40 {
		t.Fatalf("画面中心像素 = %v，期望红色广告牌", c)
	}
	if stats := r.Stats(); stats.Billboards != 1 {
		t.Fatalf("广告牌计数 = %d，期望 1", stats.Billboards)
	}
}

func TestBillboardBarFill(t *testing.T) {
	r, sc := newBillboardScene(t)
	sc.Billboards.Add(game.Billboard{
		Pos:       game.Vec3{Z: -2},
		Kind:      game.BillboardBar,
		Width:     4, Height: 0.4,
		Fill:      1,
		Color:     game.Color(0, 255, 0, 255),
		BackColor: game.Color(0, 0, 0, 255),
	})
	img := r.Frame(sc)
	c := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	if c.G < 200 {
		t.Fatalf("满血条中心像素 = %v，期望绿色", c)
	}
}

func TestBillboardBehindCameraSkipped(t *testing.T) {
	r, sc := newBillboardScene(t)
	// 广告牌在相机背后（+Z）→ 不应绘制
	sc.Billboards.Add(game.Billboard{
		Pos:   game.Vec3{Z: 5},
		Kind:  game.BillboardIcon,
		Width: 4, Height: 4,
		Color: game.Color(255, 0, 0, 255),
	})
	img := r.Frame(sc)
	c := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	if c.R > 40 {
		t.Fatalf("相机背后广告牌不应绘制，中心像素 = %v", c)
	}
	if stats := r.Stats(); stats.Billboards != 0 {
		t.Fatalf("广告牌计数 = %d，期望 0", stats.Billboards)
	}
}

func TestBillboardTextNoPanic(t *testing.T) {
	r, sc := newBillboardScene(t)
	sc.Billboards.Add(game.Billboard{
		Pos:  game.Vec3{Z: -2},
		Kind: game.BillboardText,
		Text: "99",
		Color: game.Color(255, 255, 0, 255),
	})
	if img := r.Frame(sc); img == nil {
		t.Fatal("渲染返回 nil")
	}
	if stats := r.Stats(); stats.Billboards != 1 {
		t.Fatalf("广告牌计数 = %d，期望 1", stats.Billboards)
	}
}
