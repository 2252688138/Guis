// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// BuildMips 构建 mipmap 链（2×2 盒式平均，逐级减半直到 1×1）。
//
// 已构建时直接返回，重复调用无开销；纹理像素被外部改写后需先 FreeMips。
func (t *Texture) BuildMips() {
	if t == nil || t.W <= 0 || t.H <= 0 || len(t.Pix) < t.W*t.H {
		return
	}
	if len(t.mips) > 0 {
		return
	}
	for w, h := t.W, t.H; w > 1 || h > 1; {
		nw, nh := w/2, h/2
		if nw < 1 {
			nw = 1
		}
		if nh < 1 {
			nh = 1
		}
		prev := t.Pix
		if n := len(t.mips); n > 0 {
			prev = t.mips[n-1].Pix
		}
		m := &Texture{W: nw, H: nh, Pix: make([]RGBA, nw*nh)}
		for y := 0; y < nh; y++ {
			for x := 0; x < nw; x++ {
				// 取上一级的 2×2 四点平均（奇数边时复用边缘像素）
				c00 := atClamp(prev, w, h, x*2, y*2)
				c10 := atClamp(prev, w, h, x*2+1, y*2)
				c01 := atClamp(prev, w, h, x*2, y*2+1)
				c11 := atClamp(prev, w, h, x*2+1, y*2+1)
				a := c00.Lerp(c10, 0.5)
				b := c01.Lerp(c11, 0.5)
				m.Pix[y*nw+x] = a.Lerp(b, 0.5)
			}
		}
		t.mips = append(t.mips, m)
		w, h = nw, nh
	}
}

// atClamp 取像素（越界钳制到边缘）。
func atClamp(pix []RGBA, w, h, x, y int) RGBA {
	if x >= w {
		x = w - 1
	}
	if y >= h {
		y = h - 1
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if len(pix) <= y*w+x {
		return RGBA{}
	}
	return pix[y*w+x]
}

// FreeMips 释放 mipmap 链（纹理像素被改写后调用，下次采样会按需重建）。
func (t *Texture) FreeMips() {
	if t == nil {
		return
	}
	t.mips = nil
}

// MipCount 返回 mipmap 层数（含 0 级）；未构建时为 0。
func (t *Texture) MipCount() int {
	if t == nil {
		return 0
	}
	if len(t.mips) == 0 {
		return 0
	}
	return len(t.mips) + 1
}

// Mip 返回第 level 级纹理（0 = 原图），越界返回 nil。
func (t *Texture) Mip(level int) *Texture {
	if t == nil || level < 0 {
		return nil
	}
	if level == 0 {
		return t
	}
	if level > len(t.mips) {
		return nil
	}
	return t.mips[level-1]
}

// SampleTrilinear 三线性采样：按 lod 在相邻两级 mip 之间线性插值。
//
// mipmap 链未构建时自动退化为第 0 级双线性采样，因此可以安全地在任意纹理上调用。
// lod 为「以 2 为底」的细节层级（0 = 原图，1 = 半分辨率，依次类推）。
func (t *Texture) SampleTrilinear(u, v, lod float32) RGBA {
	if t == nil || t.W == 0 || t.H == 0 {
		return RGB(255, 255, 255)
	}
	if len(t.mips) == 0 || lod <= 0 {
		return t.SampleLinear(u, v)
	}
	maxLod := float32(len(t.mips))
	if lod >= maxLod {
		// 收敛到最小一级（1×1 或 1×2），避免过度模糊
		return t.mips[len(t.mips)-1].SampleLinear(u, v)
	}
	i := int(math.Floor(float64(lod)))
	frac := lod - float32(i)
	a := t.Mip(i)
	b := t.Mip(i + 1)
	if a == nil || b == nil {
		return t.SampleLinear(u, v)
	}
	ca := a.SampleLinear(u, v)
	if frac <= 0 || b == nil {
		return ca
	}
	return ca.Lerp(b.SampleLinear(u, v), frac)
}

// TexelLod 按屏幕空间的 UV 变化率估算 mip 层级。
//
// du、dv 为相邻像素的 UV 增量（透视校正后），返回可直接传给 SampleTrilinear 的 lod。
func (t *Texture) TexelLod(du, dv float32) float32 {
	if t == nil || t.W == 0 || t.H == 0 {
		return 0
	}
	du = float32(math.Abs(float64(du))) * float32(t.W)
	dv = float32(math.Abs(float64(dv))) * float32(t.H)
	d := du
	if dv > d {
		d = dv
	}
	if d <= 1 {
		return 0
	}
	return float32(math.Log2(float64(d)))
}
