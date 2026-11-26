// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
)

// RGBA 直通 alpha（非预乘）颜色。
type RGBA struct {
	R, G, B, A uint8
}

// Color 构造颜色。
func Color(r, g, b, a uint8) RGBA { return RGBA{r, g, b, a} }

// RGB 不透明颜色。
func RGB(r, g, b uint8) RGBA { return RGBA{r, g, b, 255} }

// WithAlpha 返回替换 alpha 后的副本。
func (c RGBA) WithAlpha(a uint8) RGBA { c.A = a; return c }

// Lerp 两色线性插值（t ∈ [0,1]）。
func (c RGBA) Lerp(o RGBA, t float32) RGBA {
	t = Clamp(t, 0, 1)
	mix := func(a, b uint8) uint8 {
		return uint8(float32(a) + (float32(b)-float32(a))*t + 0.5)
	}
	return RGBA{mix(c.R, o.R), mix(c.G, o.G), mix(c.B, o.B), mix(c.A, o.A)}
}

// Scale 按系数缩放亮度（含 alpha）。
func (c RGBA) Scale(k float32) RGBA {
	cl := func(v uint8) uint8 {
		x := float32(v) * k
		if x <= 0 {
			return 0
		}
		if x >= 255 {
			return 255
		}
		return uint8(x + 0.5)
	}
	return RGBA{cl(c.R), cl(c.G), cl(c.B), cl(c.A)}
}

// Mul 颜色相乘（分量归一化后相乘，常用于纹理 × 材质色）。
func (c RGBA) Mul(o RGBA) RGBA {
	m := func(a, b uint8) uint8 { return uint8(uint32(a) * uint32(b) / 255) }
	return RGBA{m(c.R, o.R), m(c.G, o.G), m(c.B, o.B), m(c.A, o.A)}
}

// Opaque 是否完全不透明。
func (c RGBA) Opaque() bool { return c.A == 255 }

// Texture 直通 alpha 的 RGBA 纹理（长度 = W*H）。
//
// 需要三线性过滤时先调 BuildMips 构建 mipmap 链（见 texture_mip.go）。
type Texture struct {
	W, H int
	Pix  []RGBA
	// mips mipmap 链（不含 0 级，惰性构建）。
	mips []*Texture
}

// NewTexture 创建尺寸为 w×h 的纹理（初始全透明）。
func NewTexture(w, h int) *Texture {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &Texture{W: w, H: h, Pix: make([]RGBA, w*h)}
}

// Fill 用单色填满。
func (t *Texture) Fill(c RGBA) {
	for i := range t.Pix {
		t.Pix[i] = c
	}
}

// At 取像素（越界返回透明黑）。
func (t *Texture) At(x, y int) RGBA {
	if t == nil || x < 0 || y < 0 || x >= t.W || y >= t.H {
		return RGBA{}
	}
	return t.Pix[y*t.W+x]
}

// Set 写像素（越界忽略）。
func (t *Texture) Set(x, y int, c RGBA) {
	if t == nil || x < 0 || y < 0 || x >= t.W || y >= t.H {
		return
	}
	t.Pix[y*t.W+x] = c
}

// SampleNearest 最近邻采样（UV 在 [0,1]，超出按重复平铺）。
func (t *Texture) SampleNearest(u, v float32) RGBA {
	if t == nil || t.W == 0 || t.H == 0 {
		return RGBA{255, 255, 255, 255}
	}
	x := int(u * float32(t.W))
	y := int(v * float32(t.H))
	x %= t.W
	y %= t.H
	if x < 0 {
		x += t.W
	}
	if y < 0 {
		y += t.H
	}
	return t.Pix[y*t.W+x]
}

// SampleLinear 双线性采样（UV 在 [0,1]，超出按重复平铺）。
func (t *Texture) SampleLinear(u, v float32) RGBA {
	if t == nil || t.W == 0 || t.H == 0 {
		return RGBA{255, 255, 255, 255}
	}
	fx := u*float32(t.W) - 0.5
	fy := v*float32(t.H) - 0.5
	x0 := int(math.Floor(float64(fx)))
	y0 := int(math.Floor(float64(fy)))
	tx := fx - float32(x0)
	ty := fy - float32(y0)
	wrap := func(x, y int) RGBA {
		x %= t.W
		y %= t.H
		if x < 0 {
			x += t.W
		}
		if y < 0 {
			y += t.H
		}
		return t.Pix[y*t.W+x]
	}
	c00 := wrap(x0, y0)
	c10 := wrap(x0+1, y0)
	c01 := wrap(x0, y0+1)
	c11 := wrap(x0+1, y0+1)
	a := c00.Lerp(c10, tx)
	b := c01.Lerp(c11, tx)
	return a.Lerp(b, ty)
}

// Clone 深拷贝。
func (t *Texture) Clone() *Texture {
	if t == nil {
		return nil
	}
	c := NewTexture(t.W, t.H)
	copy(c.Pix, t.Pix)
	return c
}

// CheckerTexture 生成棋盘纹理。
func CheckerTexture(cell int, a, b RGBA) *Texture {
	if cell < 1 {
		cell = 1
	}
	t := NewTexture(cell*2, cell*2)
	for y := 0; y < t.H; y++ {
		for x := 0; x < t.W; x++ {
			if (x/cell+y/cell)%2 == 0 {
				t.Pix[y*t.W+x] = a
			} else {
				t.Pix[y*t.W+x] = b
			}
		}
	}
	return t
}

// LoadTexture 从文件加载 PNG/JPEG 纹理。
func LoadTexture(path string) (*Texture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadTextureBytes(b)
}

// LoadTextureBytes 从内存字节加载 PNG/JPEG 纹理。
func LoadTextureBytes(b []byte) (*Texture, error) {
	var (
		img image.Image
		err error
	)
	if img, err = png.Decode(bytes.NewReader(b)); err != nil {
		if img, err = jpeg.Decode(bytes.NewReader(b)); err != nil {
			return nil, fmt.Errorf("game: 无法识别的图片格式: %w", err)
		}
	}
	return TextureFromImage(img), nil
}

// TextureFromImage 把 image.Image 转成直通 alpha 的 Texture。
func TextureFromImage(img image.Image) *Texture {
	if img == nil {
		return NewTexture(0, 0)
	}
	bd := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, bd.Dx(), bd.Dy()))
	draw.Draw(n, n.Bounds(), img, bd.Min, draw.Src)
	t := NewTexture(bd.Dx(), bd.Dy())
	for y := 0; y < t.H; y++ {
		for x := 0; x < t.W; x++ {
			i := n.PixOffset(x, y)
			t.Pix[y*t.W+x] = RGBA{n.Pix[i], n.Pix[i+1], n.Pix[i+2], n.Pix[i+3]}
		}
	}
	return t
}
