// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "image"

// NineSlice 九宫格拉伸（UI 面板 / 对话框 / 按钮 / 血条底图常用）。
//
//	game.NineSlice{
//		Texture: uiTex,
//		Dst:     game.Rect2{X: 20, Y: 20, W: 320, H: 120},
//		Border:  game.Rect2{X: 8, Y: 8, W: 8, H: 8}, // 左 / 上 / 右 / 下
//	}.Draw(img)
//
// 四角保持原像素不拉伸，四条边单向拉伸，中区双向拉伸；
// 目标尺寸小于边框总和时按比例收缩边框，不会破图。
type NineSlice struct {
	// Texture 纹理（必填）。
	Texture *Texture
	// Src 源矩形（W/H ≤ 0 时用整张纹理）。
	Src Rect2
	// Dst 目标矩形（屏幕像素）。
	Dst Rect2
	// Border 四边宽度：X=左、Y=上、W=右、H=下。
	Border Rect2
	// Tint 乘色（A=0 按不透明处理）。
	Tint RGBA
	// SkipCenter 只画边框、不画中区。
	SkipCenter bool
}

// seg2 一维分段（源起点 / 长度、目标起点 / 长度）。
type seg2 struct{ src, srcLen, dst, dstLen float32 }

// Draw 把九宫格混合到目标图像。
func (n NineSlice) Draw(dst *image.RGBA) {
	if dst == nil || n.Texture == nil || n.Dst.W <= 0 || n.Dst.H <= 0 {
		return
	}
	src := n.Src
	if src.W <= 0 || src.H <= 0 {
		src = Rect2{0, 0, float32(n.Texture.W), float32(n.Texture.H)}
	}
	b := n.Border
	if b.X < 0 {
		b.X = 0
	}
	if b.Y < 0 {
		b.Y = 0
	}
	if b.W < 0 {
		b.W = 0
	}
	if b.H < 0 {
		b.H = 0
	}
	if b.X+b.W > src.W {
		k := src.W / (b.X + b.W)
		b.X *= k
		b.W *= k
	}
	if b.Y+b.H > src.H {
		k := src.H / (b.Y + b.H)
		b.Y *= k
		b.H *= k
	}

	// 目标侧边框按比例收缩
	kl, kr := float32(1), float32(1)
	if sum := b.X + b.W; sum > 0 && n.Dst.W < sum {
		k := n.Dst.W / sum
		kl, kr = k, k
	}
	kt, kb := float32(1), float32(1)
	if sum := b.Y + b.H; sum > 0 && n.Dst.H < sum {
		k := n.Dst.H / sum
		kt, kb = k, k
	}

	dl, dr := b.X*kl, b.W*kr
	dt, db := b.Y*kt, b.H*kb
	cols := [3]seg2{
		{src.X, b.X, n.Dst.X, dl},
		{src.X + b.X, src.W - b.X - b.W, n.Dst.X + dl, n.Dst.W - dl - dr},
		{src.X + src.W - b.W, b.W, n.Dst.X + n.Dst.W - dr, dr},
	}
	rows := [3]seg2{
		{src.Y, b.Y, n.Dst.Y, dt},
		{src.Y + b.Y, src.H - b.Y - b.H, n.Dst.Y + dt, n.Dst.H - dt - db},
		{src.Y + src.H - b.H, b.H, n.Dst.Y + n.Dst.H - db, db},
	}

	for ri, row := range rows {
		if row.dstLen <= 0 || row.srcLen <= 0 {
			continue
		}
		for ci, col := range cols {
			if col.dstLen <= 0 || col.srcLen <= 0 {
				continue
			}
			if n.SkipCenter && ri == 1 && ci == 1 {
				continue
			}
			DrawSprite(dst, Sprite{
				Pos:     Vec2{X: col.dst, Y: row.dst},
				Size:    Vec2{X: col.dstLen, Y: row.dstLen},
				Texture: n.Texture,
				Src:     Rect2{X: col.src, Y: row.src, W: col.srcLen, H: row.srcLen},
				Color:   n.Tint,
			})
		}
	}
}

// DrawAt 在指定位置按目标尺寸绘制（链式便利方法）。
func (n NineSlice) DrawAt(dst *image.RGBA, x, y, w, h float32) {
	n.Dst = Rect2{X: x, Y: y, W: w, H: h}
	n.Draw(dst)
}

// NineSliceTexture 生成一张带圆角与边框的九宫格纹理（无外部美术资源也能用）。
//
// size 为纹理边长（建议 24），fill 为填充色，border 为边框色，radius 为圆角半径（像素）。
func NineSliceTexture(size int, fill, border RGBA, radius int) *Texture {
	if size < 6 {
		size = 6
	}
	if radius < 0 {
		radius = 0
	}
	if radius > size/2 {
		radius = size / 2
	}
	t := NewTexture(size, size)
	t.Fill(fill)
	if border.A == 0 {
		return t
	}
	r := float32(radius)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// 圆角外的像素置空
			var dx, dy float32
			if x < radius {
				dx = r - float32(x) - 0.5
			} else if x >= size-radius {
				dx = float32(x) - float32(size-radius) + 0.5
			}
			if y < radius {
				dy = r - float32(y) - 0.5
			} else if y >= size-radius {
				dy = float32(y) - float32(size-radius) + 0.5
			}
			if dx != 0 || dy != 0 {
				if dx*dx+dy*dy > r*r {
					t.Set(x, y, RGBA{})
					continue
				}
			}
			// 边缘 1 像素画边框
			edge := x == 0 || y == 0 || x == size-1 || y == size-1
			if !edge && radius > 0 {
				if x < radius && y < radius && dx*dx+dy*dy > (r-1)*(r-1) {
					edge = true
				} else if x >= size-radius && y < radius && dx*dx+dy*dy > (r-1)*(r-1) {
					edge = true
				} else if x < radius && y >= size-radius && dx*dx+dy*dy > (r-1)*(r-1) {
					edge = true
				} else if x >= size-radius && y >= size-radius && dx*dx+dy*dy > (r-1)*(r-1) {
					edge = true
				}
			}
			if edge {
				t.Set(x, y, border)
			}
		}
	}
	return t
}
