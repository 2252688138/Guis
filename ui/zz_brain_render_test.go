package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

// blend 把白色 + alpha 的光栅图合成到灰色背景上，返回 RGBA 图像。
func blend(px []byte, size int, bg color.RGBA, scale int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size*scale, size*scale))
	for y := 0; y < size*scale; y++ {
		for x := 0; x < size*scale; x++ {
			a := int(px[((y/scale)*size+(x/scale))*4+3])
			c := color.RGBA{
				R: uint8((int(bg.R)*(255-a) + 255*a) / 255),
				G: uint8((int(bg.G)*(255-a) + 255*a) / 255),
				B: uint8((int(bg.B)*(255-a) + 255*a) / 255),
				A: 255,
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func loadPath(t *testing.T, name string) string {
	raw, err := os.ReadFile("../.zym/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

// TestZZBrainRender 临时预览：把候选图标渲染成 PNG 便于肉眼比较。
func TestZZBrainRender(t *testing.T) {
	bg := color.RGBA{0x1b, 0x1b, 0x1d, 255}
	type cand struct {
		name string
		file string
		fill bool
	}
	cands := []cand{
		{"B1", "cand_b.txt", true},
		{"B2", "cand_b2.txt", true},
	}
	const cell, big, small = 300, 240, 22

	var imgs []*image.RGBA
	for _, c := range cands {
		d := loadPath(t, c.file)
		subs := iconFlattenPath(d)
		imgs = append(imgs, blend(iconRaster(subs, c.fill, 2.0, big), big, bg, 1))
		imgs = append(imgs, blend(iconRaster(subs, c.fill, 2.0, small), small, bg, 6))
	}

	w := cell * len(imgs)
	h := cell
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range out.Pix {
		out.Pix[i] = 0xFF
	}
	// 深色底
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.SetRGBA(x, y, bg)
		}
	}
	for i, im := range imgs {
		x0 := i*cell + (cell-im.Bounds().Dx())/2
		y0 := (cell - im.Bounds().Dy()) / 2
		for y := 0; y < im.Bounds().Dy(); y++ {
			for x := 0; x < im.Bounds().Dx(); x++ {
				out.SetRGBA(x0+x, y0+y, im.RGBAAt(x, y))
			}
		}
	}
	f, err := os.Create("../.zym/shots/cands.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, out); err != nil {
		t.Fatal(err)
	}
	f.Close()
	fmt.Println("wrote ../.zym/shots/cands.png", w, h)
}
