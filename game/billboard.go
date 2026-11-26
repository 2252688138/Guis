// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"image"
	"math"
)

// BillboardKind 广告牌类型。
type BillboardKind int

const (
	// BillboardBar 血条 / 进度条（底 + 按 Fill 比例的填充）。
	BillboardBar BillboardKind = iota
	// BillboardText 文字（伤害数字、名字、提示）。
	BillboardText
	// BillboardIcon 纯色方块 / 图标。
	BillboardIcon
)

// Billboard 世界空间广告牌（始终面向相机的颜文字 / 血条）。
//
// 不参与 3D 光栅化：在所有三角形之后以屏幕空间叠加的方式画到画面上，
// 深度测试读光栅器的 invW 缓冲，所以能被墙、地形挡住。
type Billboard struct {
	// Pos 世界坐标（米）。
	Pos Vec3
	// Offset 额外偏移（比如跑到头顶上方）。
	Offset Vec3
	Kind   BillboardKind
	Text   string
	// Width / Height 世界尺寸（米）；Height=0 时条高默认 0.12、图标边默认 0.5。
	Width, Height float32
	// Fill 进度条填充比例（0~1）。
	Fill float32
	// Color 主色（文字 / 填充 / 图标），BackColor 底色（仅 Bar）。
	Color, BackColor RGBA
	// Scale 文字像素缩放（每字素宽/高倍数）；0 = 按 Height 自动推算。
	Scale float32
	// DepthTest 是否被 3D 几何遮挡（默认 true 更符合直觉，需要永远置顶时显式关掉）。
	DepthTest bool
	// MaxDist 超过这个距离不画（0 = 不限）。
	MaxDist float32
}

// BillboardSet 广告牌集合（每帧重建，读时直接取切片）。
type BillboardSet struct {
	items []Billboard
}

// NewBillboardSet 创建空集合。
func NewBillboardSet() *BillboardSet { return &BillboardSet{} }

// Add 追加一个广告牌。
func (b *BillboardSet) Add(x Billboard) *BillboardSet {
	if b == nil {
		return nil
	}
	b.items = append(b.items, x)
	return b
}

// Reset 清空（帧循环开头调用）。
func (b *BillboardSet) Reset() {
	if b == nil {
		return
	}
	b.items = b.items[:0]
}

// Len 当前数量。
func (b *BillboardSet) Len() int {
	if b == nil {
		return 0
	}
	return len(b.items)
}

// Items 当前广告牌切片。
func (b *BillboardSet) Items() []Billboard {
	if b == nil {
		return nil
	}
	return b.items
}

// drawBillboards 把广告牌叠加到已输出的帧缓冲（在 Post / 精灵之前），返回实际绘制数。
func (r *Renderer) drawBillboards(sc *Scene) int {
	if r == nil || r.img == nil || sc == nil || sc.Camera == nil || sc.Billboards.Len() == 0 {
		return 0
	}
	cam := sc.Camera
	aspect := float32(r.W) / float32(r.H)
	vp := cam.ViewProj(aspect)
	tanHalf := float32(math.Tan(float64(Radians(cam.FOVY)) / 2))
	if tanHalf <= 0 {
		tanHalf = 0.5
	}
	ss := r.ss()
	font := BuiltinFont()
	drawn := 0

	for _, it := range sc.Billboards.Items() {
		p := it.Pos.Add(it.Offset)
		clip := vp.TransformVec4(Vec4{X: p.X, Y: p.Y, Z: p.Z, W: 1})
		if clip.W <= 0.05 {
			continue // 相机背后
		}
		invW := 1 / clip.W
		ndcX := clip.X * invW
		ndcY := clip.Y * invW
		if ndcX < -1.1 || ndcX > 1.1 || ndcY < -1.1 || ndcY > 1.1 {
			continue
		}
		if it.MaxDist > 0 {
			dx := p.X - cam.Pos.X
			dy := p.Y - cam.Pos.Y
			dz := p.Z - cam.Pos.Z
			if float32(math.Sqrt(float64(dx*dx+dy*dy+dz*dz))) > it.MaxDist {
				continue
			}
		}
		sx := (ndcX*0.5 + 0.5) * float32(r.W)
		sy := (0.5 - ndcY*0.5) * float32(r.H)
		if it.DepthTest && r.fb != nil {
			dx := int(sx) * ss
			dy := int(sy) * ss
			if dx >= 0 && dy >= 0 && dx < r.fb.w && dy < r.fb.h {
				if r.fb.depth[dy*r.fb.w+dx] > invW*1.02 {
					continue // 被更近的几何挡住
				}
			}
		}
		// 世界单位 → 像素（透视投影下的近似比例）
		pxPerUnit := float32(r.H) * 0.5 / (tanHalf * clip.W)
		drawn++
		switch it.Kind {
		case BillboardText:
			r.drawBillboardText(font, it, sx, sy, pxPerUnit)
		case BillboardIcon:
			h := it.Height
			if h <= 0 {
				h = 0.5
			}
			w := it.Width
			if w <= 0 {
				w = h
			}
			hw := int(w * pxPerUnit / 2)
			hh := int(h * pxPerUnit / 2)
			if hw < 1 {
				hw = 1
			}
			if hh < 1 {
				hh = 1
			}
			blendRect(r.img, int(sx)-hw, int(sy)-hh, int(sx)+hw, int(sy)+hh, it.Color)
		default: // BillboardBar
			h := it.Height
			if h <= 0 {
				h = 0.12
			}
			w := it.Width
			if w <= 0 {
				w = 1
			}
			hw := int(w * pxPerUnit / 2)
			hh := int(h * pxPerUnit / 2)
			if hw < 2 {
				hw = 2
			}
			if hh < 1 {
				hh = 1
			}
			x0, y0 := int(sx)-hw, int(sy)-hh
			x1, y1 := int(sx)+hw, int(sy)+hh
			blendRect(r.img, x0, y0, x1, y1, it.BackColor)
			fill := it.Fill
			if fill < 0 {
				fill = 0
			} else if fill > 1 {
				fill = 1
			}
			xf := x0 + int(float32(x1-x0)*fill)
			blendRect(r.img, x0, y0, xf, y1, it.Color)
		}
	}
	return drawn
}

// drawBillboardText 以屏幕像素画一行文字（带描边阴影，避免贴图背景上看不见）。
func (r *Renderer) drawBillboardText(font *BitmapFont, it Billboard, sx, sy, pxPerUnit float32) {
	if font == nil || it.Text == "" {
		return
	}
	scale := it.Scale
	if scale <= 0 {
		h := it.Height
		if h <= 0 {
			h = 0.35
		}
		_, gh := font.Measure("X", 1)
		if gh <= 0 {
			gh = 8
		}
		want := h * pxPerUnit
		if want < 6 {
			want = 6 // 太远就直接画小字，避免退化到 0 像素
		}
		scale = want / gh
	}
	w, h := font.Measure(it.Text, scale)
	x := int(sx - w/2)
	y := int(sy - h/2)
	shadow := it.Color
	shadow.A = 160
	font.DrawTextShadow(r.img, it.Text, float32(x)+1, float32(y)+1, scale, it.Color, shadow)
}

// blendRect 在 RGBA 图上做 alpha 混合矩形（自动裁剪到边界）。
func blendRect(dst *image.RGBA, x0, y0, x1, y1 int, c RGBA) {
	if dst == nil || c.A == 0 || x1 <= x0 || y1 <= y0 {
		return
	}
	b := dst.Bounds()
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	if x1 <= x0 || y1 <= y0 {
		return
	}
	a := int(c.A)
	ia := 255 - a
	for y := y0; y < y1; y++ {
		row := dst.Pix[y*dst.Stride:]
		for x := x0; x < x1; x++ {
			i := x * 4
			row[i] = uint8((int(row[i])*ia + int(c.R)*a) / 255)
			row[i+1] = uint8((int(row[i+1])*ia + int(c.G)*a) / 255)
			row[i+2] = uint8((int(row[i+2])*ia + int(c.B)*a) / 255)
			row[i+3] = 255
		}
	}
}
