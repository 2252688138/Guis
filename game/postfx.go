// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"image"
	"math"
)

// PostFX 屏幕空间后处理效果链（挂在 Renderer.Post 上，作用于 3D 画面）。
//
// 全部为 CPU 像素运算，成本与渲染分辨率成正比；建议把最常用的几项开着，
// bloom 与 noise 按需求临时打开。零值字段表示「不生效」，只需填要用的项：
//
//	renderer.Post = &game.PostFX{On: true, Vignette: 0.35, Saturation: 1.1}
//
// 游戏中常用的受击闪红 / 回血闪绿，直接改 Flash 即可：
//
//	post.Flash = game.Color(220, 40, 40, 140) // 之后每帧减速衰减 A
type PostFX struct {
	// On 是否启用后处理。
	On bool
	// Brightness 亮度偏移（-1~1，0 = 不变）。
	Brightness float32
	// Contrast 对比度（1 = 不变，>1 加强）。
	Contrast float32
	// Saturation 饱和度（1 = 不变，0 = 灰度）。
	Saturation float32
	// Gamma 伽马校正（1 = 不变，>1 提亮暗部）。
	Gamma float32
	// Tint 乘色染色（A=0 时不生效）。
	Tint RGBA
	// Flash 叠加闪光色（用 A 作强度，0 = 不生效）。
	Flash RGBA
	// Grayscale 灰度混合量（0~1）。
	Grayscale float32
	// Vignette 暗角强度（0~1）。
	Vignette float32
	// Posterize 色阶量化级数（0 = 关闭，常用 8~16 做复古风）。
	Posterize int
	// Scanlines 扫描线强度（0~1）。
	Scanlines float32
	// Noise 噪点强度（0~1）。
	Noise float32
	// Bloom 泛光开关。
	Bloom bool
	// BloomThreshold 泛光亮度阈值（0~1，默认 0.7）。
	BloomThreshold float32
	// BloomIntensity 泛光强度（默认 1.0）。
	BloomIntensity float32
	// BloomRadius 泛光模糊半径（像素，默认 3）。
	BloomRadius int

	frame int
}

// NewPostFX 创建后处理链（默认关闭，只打开要用的效果）。
func NewPostFX() *PostFX {
	return &PostFX{Contrast: 1, Saturation: 1, Gamma: 1, BloomThreshold: 0.7, BloomIntensity: 1, BloomRadius: 3}
}

// 把颜色量化到 Posterize 级。
func posterizeChannel(v float32, levels int) float32 {
	if levels < 2 {
		return v
	}
	l := float32(levels - 1)
	return float32(math.Floor(float64(v*l+0.5))) / l
}

// Apply 对一帧图像应用后处理（原地修改）。
func (p *PostFX) Apply(img *image.RGBA) {
	if p == nil || !p.On || img == nil {
		return
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w <= 0 || h <= 0 || len(img.Pix) < w*h*4 {
		return
	}
	p.frame++

	contrast := p.Contrast
	if contrast <= 0 {
		contrast = 1
	}
	gamma := p.Gamma
	if gamma <= 0 {
		gamma = 1
	}
	sat := p.Saturation
	if sat < 0 {
		sat = 0
	}
	gray := Clamp(p.Grayscale, 0, 1)

	// 亮度 / 对比 / 伽马用 256 项查表，逐像素只查表
	var lut [256]uint8
	for i := 0; i < 256; i++ {
		v := float64(i)/255 + float64(p.Brightness)
		v = (v-0.5)*float64(contrast) + 0.5
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		if gamma != 1 {
			v = math.Pow(v, 1/float64(gamma))
		}
		lut[i] = uint8(v*255 + 0.5)
	}
	tR, tG, tB, tA := p.Tint.R, p.Tint.G, p.Tint.B, p.Tint.A
	tint := tA > 0
	levels := p.Posterize

	// 1) 色调映射
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			j := i * 4
			r := lut[img.Pix[j]]
			g := lut[img.Pix[j+1]]
			b := lut[img.Pix[j+2]]
			if tint {
				r = uint8(uint32(r) * uint32(tR) / 255)
				g = uint8(uint32(g) * uint32(tG) / 255)
				b = uint8(uint32(b) * uint32(tB) / 255)
			}
			if sat != 1 || gray > 0 {
				luma := 0.299*float32(r) + 0.587*float32(g) + 0.114*float32(b)
				s := sat * (1 - gray)
				r = clamp255(luma + (float32(r)-luma)*s)
				g = clamp255(luma + (float32(g)-luma)*s)
				b = clamp255(luma + (float32(b)-luma)*s)
			}
			if levels >= 2 {
				r = uint8(posterizeChannel(float32(r)/255, levels)*255 + 0.5)
				g = uint8(posterizeChannel(float32(g)/255, levels)*255 + 0.5)
				b = uint8(posterizeChannel(float32(b)/255, levels)*255 + 0.5)
			}
			img.Pix[j], img.Pix[j+1], img.Pix[j+2] = r, g, b
		}
	}

	// 2) 泛光
	if p.Bloom {
		p.applyBloom(img, w, h)
	}

	// 3) 暗角 / 扫描线 / 闪光 / 噪点
	vig := Clamp(p.Vignette, 0, 1)
	scan := Clamp(p.Scanlines, 0, 1)
	noise := Clamp(p.Noise, 0, 1)
	flash := p.Flash.A > 0
	fa := float32(p.Flash.A) / 255
	if vig > 0 || scan > 0 || flash || noise > 0 {
		cx, cy := float32(w)*0.5, float32(h)*0.5
		maxD := float32(math.Hypot(float64(cx), float64(cy)))
		if maxD <= 0 {
			maxD = 1
		}
		seed := uint32(p.frame)*2654435761 + 1
		for y := 0; y < h; y++ {
			var scanK float32 = 1
			if scan > 0 && y%2 == 1 {
				scanK = 1 - scan*0.35
			}
			for x := 0; x < w; x++ {
				j := (y*w + x) * 4
				k := scanK
				if vig > 0 {
					dx, dy := float32(x)+0.5-cx, float32(y)+0.5-cy
					d := float32(math.Hypot(float64(dx), float64(dy))) / maxD
					k *= 1 - vig*d*d
				}
				if k < 0 {
					k = 0
				}
				r := clamp255(float32(img.Pix[j]) * k)
				g := clamp255(float32(img.Pix[j+1]) * k)
				b := clamp255(float32(img.Pix[j+2]) * k)
				if flash {
					r = clamp255(float32(r)*(1-fa) + float32(p.Flash.R)*fa)
					g = clamp255(float32(g)*(1-fa) + float32(p.Flash.G)*fa)
					b = clamp255(float32(b)*(1-fa) + float32(p.Flash.B)*fa)
				}
				if noise > 0 {
					seed = seed*1664525 + 1013904223
					n := (float32(seed>>16&0xff)/127.5 - 1) * noise * 48
					r = clamp255(float32(r) + n)
					g = clamp255(float32(g) + n)
					b = clamp255(float32(b) + n)
				}
				img.Pix[j], img.Pix[j+1], img.Pix[j+2] = r, g, b
			}
		}
	}
}

// applyBloom 降采样提取亮部 → 两遍盒式模糊 → 加回原图。
func (p *PostFX) applyBloom(img *image.RGBA, w, h int) {
	bw, bh := (w+3)/4, (h+3)/4
	if bw < 1 {
		bw = 1
	}
	if bh < 1 {
		bh = 1
	}
	if bw*bh <= 0 {
		return
	}
	thr := p.BloomThreshold
	if thr <= 0 {
		thr = 0.7
	}
	inten := p.BloomIntensity
	if inten <= 0 {
		inten = 1
	}
	if p.BloomRadius <= 0 {
		p.BloomRadius = 3
	}

	r := make([]float32, bw*bh)
	g := make([]float32, bw*bh)
	b := make([]float32, bw*bh)
	for y := 0; y < bh; y++ {
		for x := 0; x < bw; x++ {
			// 4×4 平均
			var sr, sg, sb float32
			var cnt float32
			for sy := 0; sy < 4; sy++ {
				py := y*4 + sy
				if py >= h {
					break
				}
				for sx := 0; sx < 4; sx++ {
					px := x*4 + sx
					if px >= w {
						break
					}
					j := (py*w + px) * 4
					sr += float32(img.Pix[j])
					sg += float32(img.Pix[j+1])
					sb += float32(img.Pix[j+2])
					cnt++
				}
			}
			if cnt <= 0 {
				continue
			}
			rr, gg, bb := sr/cnt/255, sg/cnt/255, sb/cnt/255
			luma := 0.299*rr + 0.587*gg + 0.114*bb
			if luma <= thr {
				continue
			}
			scale := (luma - thr) / (1 - thr + 1e-6)
			i := y*bw + x
			r[i] = rr * scale
			g[i] = gg * scale
			b[i] = bb * scale
		}
	}

	tmp := make([]float32, len(r))
	boxBlur32(r, tmp, bw, bh, p.BloomRadius)
	boxBlur32(g, tmp, bw, bh, p.BloomRadius)
	boxBlur32(b, tmp, bw, bh, p.BloomRadius)

	for y := 0; y < h; y++ {
		by := y / 4
		if by >= bh {
			by = bh - 1
		}
		for x := 0; x < w; x++ {
			bx := x / 4
			if bx >= bw {
				bx = bw - 1
			}
			s := by*bw + bx
			j := (y*w + x) * 4
			img.Pix[j] = clamp255(float32(img.Pix[j]) + r[s]*inten*255)
			img.Pix[j+1] = clamp255(float32(img.Pix[j+1]) + g[s]*inten*255)
			img.Pix[j+2] = clamp255(float32(img.Pix[j+2]) + b[s]*inten*255)
		}
	}
}

// boxBlur32 对单通道浮点图做两遍（水平 + 垂直）可分离盒式模糊。
func boxBlur32(src, tmp []float32, w, h, radius int) {
	if radius < 1 || len(src) < w*h {
		return
	}
	if radius > 16 {
		radius = 16
	}
	d := float32(radius*2 + 1)
	// 水平
	for y := 0; y < h; y++ {
		row := y * w
		for x := 0; x < w; x++ {
			var sum float32
			for k := -radius; k <= radius; k++ {
				sx := x + k
				if sx < 0 {
					sx = 0
				}
				if sx >= w {
					sx = w - 1
				}
				sum += src[row+sx]
			}
			tmp[row+x] = sum / d
		}
	}
	// 垂直
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			var sum float32
			for k := -radius; k <= radius; k++ {
				sy := y + k
				if sy < 0 {
					sy = 0
				}
				if sy >= h {
					sy = h - 1
				}
				sum += tmp[sy*w+x]
			}
			src[y*w+x] = sum / d
		}
	}
}
