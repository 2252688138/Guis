// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image"
	"image/color"
	"testing"

	"guis/game"
)

// colored 生成一张纯色小图。
func colored(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestPackAtlasNoOverlap(t *testing.T) {
	imgs := []game.PackImage{
		{Name: "a", Image: colored(8, 8, color.RGBA{R: 255, A: 255})},
		{Name: "b", Image: colored(16, 4, color.RGBA{G: 255, A: 255})},
		{Name: "c", Image: colored(4, 16, color.RGBA{B: 255, A: 255})},
		{Name: "d", Image: colored(8, 8, color.RGBA{R: 255, G: 255, A: 255})},
		{Name: "e", Image: colored(32, 2, color.RGBA{A: 255})},
	}
	res, err := game.PackAtlas(imgs, game.PackOptions{Padding: 2, MaxSize: 256})
	if err != nil {
		t.Fatalf("打包失败: %v", err)
	}
	if len(res.Rects) != len(imgs) {
		t.Fatalf("打包矩形数 = %d，期望 %d", len(res.Rects), len(imgs))
	}
	for i := 0; i < len(res.Rects); i++ {
		for j := i + 1; j < len(res.Rects); j++ {
			a, b := res.Rects[i], res.Rects[j]
			if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
				t.Fatalf("矩形重叠: %v 与 %v", a, b)
			}
		}
		if res.Rects[i].X+res.Rects[i].W > res.W || res.Rects[i].Y+res.Rects[i].H > res.H {
			t.Fatalf("矩形越界: %v 图集 %d×%d", res.Rects[i], res.W, res.H)
		}
	}
	// 每个矩形内像素应与原图一致（纯色校验）
	for _, r := range res.Rects {
		c := res.Image.RGBAAt(r.X+r.W/2, r.Y+r.H/2)
		if c.A == 0 {
			t.Fatalf("帧 %q 在 (%d,%d) 处没有像素", r.Name, r.X, r.Y)
		}
	}
}

func TestPackAtlasPadding(t *testing.T) {
	imgs := []game.PackImage{
		{Name: "a", Image: colored(8, 8, color.RGBA{R: 255, A: 255})},
		{Name: "b", Image: colored(8, 8, color.RGBA{G: 255, A: 255})},
	}
	res, err := game.PackAtlas(imgs, game.PackOptions{Padding: 4, MaxSize: 128})
	if err != nil {
		t.Fatalf("打包失败: %v", err)
	}
	a, b := res.Rects[0], res.Rects[1]
	gapX := absInt(a.X - b.X)
	gapY := absInt(a.Y - b.Y)
	if gapX < a.W+4 && gapY < a.H+4 {
		t.Fatalf("留白不足 4px: %v %v", a, b)
	}
}

func TestPackAtlasEmpty(t *testing.T) {
	res, err := game.PackAtlas(nil, game.PackOptions{})
	if err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	if res.W != 1 || res.H != 1 {
		t.Fatalf("空输入应返回 1×1，实际 %d×%d", res.W, res.H)
	}
}

func TestPackAtlasOverflow(t *testing.T) {
	imgs := []game.PackImage{
		{Name: "big", Image: colored(64, 64, color.RGBA{A: 255})},
		{Name: "big2", Image: colored(64, 64, color.RGBA{A: 255})},
	}
	if _, err := game.PackAtlas(imgs, game.PackOptions{MaxSize: 32}); err == nil {
		t.Fatal("超出最大尺寸时应返回错误")
	}
}

func TestAtlasMapJSONRoundTrip(t *testing.T) {
	a := game.NewAtlasMap(nil)
	a.Add("hero_idle_0", game.Rect2{X: 0, Y: 0, W: 16, H: 16})
	a.Add("hero_run_3", game.Rect2{X: 16, Y: 0, W: 16, H: 16})
	b, err := a.Marshal()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	back, err := game.LoadAtlasMap(nil, b)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if back.Len() != 2 {
		t.Fatalf("回读帧数 = %d，期望 2", back.Len())
	}
	r, ok := back.Rect("hero_run_3")
	if !ok || r.X != 16 || r.W != 16 {
		t.Fatalf("回读坐标不正确: %+v ok=%v", r, ok)
	}
	if _, ok := back.Rect("missing"); ok {
		t.Fatal("不存在的帧不应命中")
	}
}

func TestAtlasMapSprite(t *testing.T) {
	tex := solidTex(16, 16, color.RGBA{R: 255, A: 255})
	a := game.NewAtlasMap(tex)
	a.Add("a", game.Rect2{X: 0, Y: 0, W: 8, H: 8})
	s, ok := a.Sprite("a", 10, 20, 0, 0)
	if !ok {
		t.Fatal("取精灵失败")
	}
	if s.Size.X != 8 || s.Size.Y != 8 || s.Pos.X != 10 || s.Pos.Y != 20 {
		t.Fatalf("精灵 = %+v", s)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
