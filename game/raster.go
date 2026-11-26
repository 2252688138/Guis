// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// nearEps 近平面裁剪阈值（裁剪空间 w）。
const nearEps = 1e-4

// vtx 顶点着色后的顶点（世界坐标 + 裁剪空间）。
type vtx struct {
	clip   Vec4
	world  Vec3
	normal Vec3
	uv     Vec2
	color  RGBA
}

// scrVtx 投影到屏幕（像素）后的顶点，带透视校正所需数据。
type scrVtx struct {
	x, y float32
	// invW = 1/w，屏幕空间下线性，用于 Z 测试与透视校正插值。
	invW   float32
	world  Vec3
	normal Vec3
	uv     Vec2
	color  RGBA
}

// frameBuf 帧缓冲（颜色 + 反深度缓冲）。
// color 用预打包的 uint32（R|G<<8|B<<16|A<<24，小端内存布局与 image.RGBA.Pix 一致），
// 这样 blit 只需一次 memmove，不必逐像素拆字节。
type frameBuf struct {
	w, h  int
	color []uint32
	depth []float32
}

func newFrameBuf(w, h int) *frameBuf {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return &frameBuf{w: w, h: h, color: make([]uint32, w*h), depth: make([]float32, w*h)}
}

// resize 尺寸变化时重建（尺寸不变则复用）。
func (fb *frameBuf) resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if fb.w == w && fb.h == h && fb.color != nil {
		return
	}
	fb.w, fb.h = w, h
	fb.color = make([]uint32, w*h)
	fb.depth = make([]float32, w*h)
}

// clear 清屏（深度置 0，越大越近）。
func (fb *frameBuf) clear(c RGBA) {
	packed := packColor(c)
	for i := range fb.color {
		fb.color[i] = packed
	}
	for i := range fb.depth {
		fb.depth[i] = 0
	}
}

// clearRange 清除 [y0,y1) 行（并行清屏用）。
func (fb *frameBuf) clearRange(c RGBA, y0, y1 int) {
	packed := packColor(c)
	s, e := y0*fb.w, y1*fb.w
	if e > len(fb.color) {
		e = len(fb.color)
	}
	for i := s; i < e; i++ {
		fb.color[i] = packed
	}
	if e > len(fb.depth) {
		e = len(fb.depth)
	}
	for i := s; i < e; i++ {
		fb.depth[i] = 0
	}
}

// packColor 把 RGBA 打包成 uint32（字节序 R,G,B,A）。
func packColor(c RGBA) uint32 {
	return uint32(c.R) | uint32(c.G)<<8 | uint32(c.B)<<16 | uint32(c.A)<<24
}

// unpackColor packColor 的逆运算。
func unpackColor(v uint32) RGBA {
	return RGBA{R: byte(v), G: byte(v >> 8), B: byte(v >> 16), A: byte(v >> 24)}
}

// blendPack 在预打包颜色上做 src-over 混合。
func blendPack(dst, src uint32) uint32 {
	sa := float32(byte(src>>24)) / 255
	if sa >= 1 {
		return src
	}
	ia := 1 - sa
	return packColor(RGBA{
		R: clamp255(float32(byte(src))*sa + float32(byte(dst))*ia),
		G: clamp255(float32(byte(src>>8))*sa + float32(byte(dst>>8))*ia),
		B: clamp255(float32(byte(src>>16))*sa + float32(byte(dst>>16))*ia),
		A: clamp255(float32(byte(src>>24))*sa + float32(byte(dst>>24))*ia),
	})
}

// clipNear 对近平面 w > nearEps 做 Sutherland–Hodgman 裁剪。
func clipNear(poly []vtx) []vtx {
	if len(poly) == 0 {
		return nil
	}
	out := make([]vtx, 0, len(poly)+1)
	dist := func(v vtx) float32 { return v.clip.W - nearEps }
	prev := poly[len(poly)-1]
	dPrev := dist(prev)
	prevIn := dPrev > 0
	for _, cur := range poly {
		dCur := dist(cur)
		curIn := dCur > 0
		switch {
		case prevIn && curIn:
			out = append(out, cur)
		case prevIn && !curIn:
			t := dPrev / (dPrev - dCur)
			out = append(out, lerpVtx(prev, cur, t))
		case !prevIn && curIn:
			t := dPrev / (dPrev - dCur)
			out = append(out, lerpVtx(prev, cur, t), cur)
		}
		prev, dPrev, prevIn = cur, dCur, curIn
	}
	return out
}

// lerpVtx 在裁剪空间线性插值顶点属性。
func lerpVtx(a, b vtx, t float32) vtx {
	l4 := func(x, y float32) float32 { return x + (y-x)*t }
	return vtx{
		clip: Vec4{
			l4(a.clip.X, b.clip.X), l4(a.clip.Y, b.clip.Y),
			l4(a.clip.Z, b.clip.Z), l4(a.clip.W, b.clip.W),
		},
		world:  a.world.Lerp(b.world, t),
		normal: a.normal.Lerp(b.normal, t).Normalize(),
		uv:     V2(l4(a.uv.X, b.uv.X), l4(a.uv.Y, b.uv.Y)),
		color: RGBA{
			R: uint8(l4(float32(a.color.R), float32(b.color.R))),
			G: uint8(l4(float32(a.color.G), float32(b.color.G))),
			B: uint8(l4(float32(a.color.B), float32(b.color.B))),
			A: uint8(l4(float32(a.color.A), float32(b.color.A))),
		},
	}
}

// project 裁剪空间 → 屏幕像素坐标。
func project(v vtx, w, h int) scrVtx {
	invW := 1 / v.clip.W
	ndcX := v.clip.X * invW
	ndcY := v.clip.Y * invW
	return scrVtx{
		x:      (ndcX*0.5 + 0.5) * float32(w),
		y:      (1 - (ndcY*0.5 + 0.5)) * float32(h),
		invW:   invW,
		world:  v.world,
		normal: v.normal,
		uv:     v.uv,
		color:  v.color,
	}
}

// tri 待光栅化的屏幕三角形。
type tri struct {
	a, b, c     scrVtx
	mat         *Material
	faceN       Vec3
	transparent bool
	depthKey    float32 // 透明排序用（越小越远）
}

// edge 边缘函数。
func edge(ax, ay, bx, by, px, py float32) float32 {
	return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
}

// rasterTriangle 光栅化单个三角形（仅扫描 [y0, y1) 行区间）。
//
// 调用方需先用 orientTriangle 统一绕序（面积 < 0），本函数不再做背部剔除。
func rasterTriangle(fb *frameBuf, t *tri, ctx *shadeCtx, y0, y1 int, stats *FrameStats) {
	a, b, c := t.a, t.b, t.c
	area := (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x)
	if area >= 0 || math.IsNaN(float64(area)) {
		return
	}
	faceN := t.faceN
	// 包围盒
	minX := int(math.Floor(float64(minf(a.x, minf(b.x, c.x)))))
	maxX := int(math.Ceil(float64(maxf(a.x, maxf(b.x, c.x)))))
	minYf := minf(a.y, minf(b.y, c.y))
	maxYf := maxf(a.y, maxf(b.y, c.y))
	minY := int(math.Floor(float64(minYf)))
	maxY := int(math.Ceil(float64(maxYf)))
	if minX < 0 {
		minX = 0
	}
	if minY < y0 {
		minY = y0
	}
	if maxX > fb.w-1 {
		maxX = fb.w - 1
	}
	if maxY > y1-1 {
		maxY = y1 - 1
	}
	if maxY > fb.h-1 {
		maxY = fb.h - 1
	}
	if minX > maxX || minY > maxY {
		return
	}

	inv := 1 / area
	// 切线基（PBR 法线贴图用）：由顶点世界坐标与 UV 推导，退化时返回零向量
	tangent, bitangent := triangleTangent(a, b, c)
	// 顶点属性的透视校正预乘
	aw, bw, cw := a.invW, b.invW, c.invW
	aWorldW := a.world.Scale(aw)
	bWorldW := b.world.Scale(bw)
	cWorldW := c.world.Scale(cw)
	aNw := a.normal.Scale(aw)
	bNw := b.normal.Scale(bw)
	cNw := c.normal.Scale(cw)
	aUw := a.uv.Scale(aw)
	bUw := b.uv.Scale(bw)
	cUw := c.uv.Scale(cw)

	// mipmap：屏幕空间 UV 导数（dUdx 与 d(1/w)/dx 在整个三角形上是常数）
	mipOn := t.mat.Mipmap && t.mat.Texture != nil
	var dUdx Vec2
	var dchw float32
	if mipOn {
		dw0x := (b.y - c.y) * inv
		dw1x := (c.y - a.y) * inv
		dw2x := (a.y - b.y) * inv
		dUdx = aUw.Scale(dw0x).Add(bUw.Scale(dw1x)).Add(cUw.Scale(dw2x))
		dchw = aw*dw0x + bw*dw1x + cw*dw2x
	}

	for y := minY; y <= maxY; y++ {
		py := float32(y) + 0.5
		row := y * fb.w
		for x := minX; x <= maxX; x++ {
			px := float32(x) + 0.5
			w0 := edge(b.x, b.y, c.x, c.y, px, py) * inv
			w1 := edge(c.x, c.y, a.x, a.y, px, py) * inv
			w2 := edge(a.x, a.y, b.x, b.y, px, py) * inv
			if w0 < 0 || w1 < 0 || w2 < 0 {
				continue
			}
			idx := row + x
			invW := w0*aw + w1*bw + w2*cw
			if invW <= fb.depth[idx] {
				continue
			}

			// 透视校正插值
			chw := 1 / invW
			world := aWorldW.Scale(w0).Add(bWorldW.Scale(w1)).Add(cWorldW.Scale(w2)).Scale(chw)
			var n Vec3
			switch t.mat.Shade {
			case ShadeUnlit:
				n = faceN
			case ShadeFlat:
				n = faceN
			default:
				n = aNw.Scale(w0).Add(bNw.Scale(w1)).Add(cNw.Scale(w2)).Scale(chw)
				if n.Len() == 0 {
					n = faceN
				} else {
					n = n.Normalize()
				}
			}
			uv := aUw.Scale(w0).Add(bUw.Scale(w1)).Add(cUw.Scale(w2)).Scale(chw)
			vc := RGBA{
				R: clamp255((float32(a.color.R)*w0*aw + float32(b.color.R)*w1*bw + float32(c.color.R)*w2*cw) * chw),
				G: clamp255((float32(a.color.G)*w0*aw + float32(b.color.G)*w1*bw + float32(c.color.G)*w2*cw) * chw),
				B: clamp255((float32(a.color.B)*w0*aw + float32(b.color.B)*w1*bw + float32(c.color.B)*w2*cw) * chw),
				A: clamp255((float32(a.color.A)*w0*aw + float32(b.color.A)*w1*bw + float32(c.color.A)*w2*cw) * chw),
			}
			col := RGBA{}
			if mipOn {
				// 按屏幕空间 UV 变化率选 mip 层级（透视校正后随像素变化）
				rowCtx := *ctx
				duvdx := dUdx.Sub(uv.Scale(dchw)).Scale(chw)
				rowCtx.mipLod = t.mat.Texture.TexelLod(duvdx.X, duvdx.Y) + t.mat.MipBias
				col = shadeFragment(&rowCtx, t.mat, world, n, tangent, bitangent, uv, vc)
			} else {
				col = shadeFragment(ctx, t.mat, world, n, tangent, bitangent, uv, vc)
			}
			if col.A == 0 {
				continue
			}
			if t.mat.AlphaMode == AlphaCutout {
				cut := t.mat.AlphaCutoff
				if cut <= 0 {
					cut = 0.5
				}
				if float32(col.A) < cut*255 {
					continue // 镂空：低于阈值的像素直接丢弃（不写色也不写深度）
				}
				col.A = 255
			} else if t.mat.AlphaMode == AlphaOpaque {
				col.A = 255
			}
			fb.depth[idx] = invW
			if col.A == 255 {
				fb.color[idx] = packColor(col)
			} else {
				fb.color[idx] = blendPack(fb.color[idx], packColor(col))
			}
			stats.Pixels++
		}
	}
}

// rasterLine 画一条带深度测试的线（线框模式）。
func rasterLine(fb *frameBuf, x0, y0, x1, y1, invW0, invW1 float32, col RGBA) {
	steps := int(maxf(absf(x1-x0), absf(y1-y0)))
	if steps <= 0 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		t := float32(i) / float32(steps)
		x := int(x0 + (x1-x0)*t)
		y := int(y0 + (y1-y0)*t)
		if x < 0 || y < 0 || x >= fb.w || y >= fb.h {
			continue
		}
		idx := y*fb.w + x
		w := invW0 + (invW1-invW0)*t
		if w <= fb.depth[idx] {
			continue
		}
		fb.depth[idx] = w
		fb.color[idx] = blendPack(fb.color[idx], packColor(col))
	}
}

// rasterWireframe 线框三角形。
func rasterWireframe(fb *frameBuf, t *tri) {
	col := RGB(255, 255, 255)
	rasterLine(fb, t.a.x, t.a.y, t.b.x, t.b.y, t.a.invW, t.b.invW, col)
	rasterLine(fb, t.b.x, t.b.y, t.c.x, t.c.y, t.b.invW, t.c.invW, col)
	rasterLine(fb, t.c.x, t.c.y, t.a.x, t.a.y, t.c.invW, t.a.invW, col)
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// orientTriangle 统一三角形屏幕绕序：默认剔除背面（面积 > 0），
// 双面材质则交换顶点 + 翻转面法线。返回 false 表示该三角形被剔除。
func orientTriangle(t *tri) bool {
	area := (t.b.x-t.a.x)*(t.c.y-t.a.y) - (t.b.y-t.a.y)*(t.c.x-t.a.x)
	if area == 0 || math.IsNaN(float64(area)) {
		return false
	}
	if area > 0 {
		if !t.mat.DoubleSided {
			return false
		}
		t.b, t.c = t.c, t.b
		t.faceN = t.faceN.Neg()
	}
	return true
}
