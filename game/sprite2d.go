// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"image"
	"math"
	"sort"
)

// Rect2 纹理子矩形（像素）。
type Rect2 struct {
	X, Y, W, H float32
}

// Sprite 屏幕空间精灵（像素坐标）。
type Sprite struct {
	Pos, Size Vec2
	Texture   *Texture
	// Src 源矩形（零值时用整张纹理）。
	Src Rect2
	// Color 乘色（A=0 时按不透明处理）。
	Color RGBA
	// Rot 绕锚点的旋转（弧度）；锚点默认矩形中心，见 Origin / UseOrigin。
	Rot float32
	// FlipX / FlipY 渲染时水平 / 垂直翻转（源矩形不变）。
	FlipX bool
	FlipY bool
	// Origin 锚点（0~1，相对 Size：0,0 = 左上，0.5,0.5 = 中心，1,1 = 右下）。
	// 只有 UseOrigin = true 时生效。
	Origin Vec2
	// UseOrigin 启用 Origin 锚点；默认 false = 锚点为几何中心（与旧版一致）。
	UseOrigin bool
	// Layer 绘制层（小者先画），SortY 第一关键字。
	Layer int
	// Z 层内深度（小者先画），SortY 第二关键字；同 Z 再按脚底 Y。
	Z float32
}

// SpriteBatch 精灵批（逐帧 Clear 后 Add，再交给 Renderer.SetSprite2D）。
type SpriteBatch struct {
	list  []Sprite
	count int
}

// NewSpriteBatch 创建空批。
func NewSpriteBatch() *SpriteBatch { return &SpriteBatch{} }

// Add 添加一个精灵。
func (b *SpriteBatch) Add(s Sprite) {
	b.list = append(b.list, s)
	b.count = len(b.list)
}

// Clear 清空（保留底层容量）。
func (b *SpriteBatch) Clear() {
	b.list = b.list[:0]
	b.count = 0
}

// Sprites 当前精灵列表（只读）。
func (b *SpriteBatch) Sprites() []Sprite { return b.list }

// SpriteCount 绘制数量（供 FrameStats 使用）。
func (b *SpriteBatch) SpriteCount() int { return b.count }

// FeetY 脚底 Y 坐标：UseOrigin 时按 Origin.Y，否则取矩形底边。SortY 的最终关键字。
func (s Sprite) FeetY() float32 {
	ay := float32(0.5)
	if s.UseOrigin {
		ay = s.Origin.Y
	}
	return s.Pos.Y + s.Size.Y*(1-ay)
}

// SortY 按 Layer → Z → 脚底 Y 稳定排序（先画的更靠后/更远）。
//
// 典型用法：先把角色、道具、敌人全部 Add 进批，再调用 SortY，最后交给
// Renderer.SetSprite2D；同层同 Z 时脚底 Y 小者先画，实现前后遮挡。
func (b *SpriteBatch) SortY() {
	if b == nil || len(b.list) < 2 {
		return
	}
	sort.SliceStable(b.list, func(i, j int) bool {
		a, c := &b.list[i], &b.list[j]
		if a.Layer != c.Layer {
			return a.Layer < c.Layer
		}
		if a.Z != c.Z {
			return a.Z < c.Z
		}
		return a.FeetY() < c.FeetY()
	})
}

// SortFunc 用自定义比较器稳定排序（a 应先绘制时 less 返回 true）。
func (b *SpriteBatch) SortFunc(less func(a, c *Sprite) bool) {
	if b == nil || less == nil || len(b.list) < 2 {
		return
	}
	sort.SliceStable(b.list, func(i, j int) bool { return less(&b.list[i], &b.list[j]) })
}

// DrawSprites 实现 Sprite2D。
func (b *SpriteBatch) DrawSprites(dst *image.RGBA, w, h int) {
	if b == nil || dst == nil {
		return
	}
	for i := range b.list {
		DrawSprite(dst, b.list[i])
	}
}

// DrawSprite 把单个精灵混合到目标帧（直通 alpha 的 src-over）。
func DrawSprite(dst *image.RGBA, s Sprite) {
	if dst == nil || s.Texture == nil || s.Size.X <= 0 || s.Size.Y <= 0 {
		return
	}
	tint := s.Color
	if tint.A == 0 {
		tint = RGB(255, 255, 255)
	}
	src := s.Src
	if src.W <= 0 || src.H <= 0 {
		src = Rect2{0, 0, float32(s.Texture.W), float32(s.Texture.H)}
	}
	bw := dst.Rect.Dx()
	bh := dst.Rect.Dy()

	// 旋转变换：先转回本地坐标再采样
	cos, sin := float32(1), float32(0)
	if s.Rot != 0 {
		cos = float32(math.Cos(float64(s.Rot)))
		sin = float32(math.Sin(float64(s.Rot)))
	}

	// ax/ay 锚点（相对 Size 的比例）：UseOrigin 时取 Origin，否则为几何中心。
	ax, ay := float32(0.5), float32(0.5)
	if s.UseOrigin {
		ax, ay = s.Origin.X, s.Origin.Y
	}
	cx, cy := s.Pos.X+s.Size.X*ax, s.Pos.Y+s.Size.Y*ay

	minX := int(math.Floor(float64(s.Pos.X)))
	minY := int(math.Floor(float64(s.Pos.Y)))
	maxX := int(math.Ceil(float64(s.Pos.X + s.Size.X)))
	maxY := int(math.Ceil(float64(s.Pos.Y + s.Size.Y)))
	if s.Rot != 0 {
		// 旋转时四角相对锚点旋转，求精确包围盒（旧版按 max(W,H) 半径粗扩）
		loX, loY := cx, cy
		hiX, hiY := cx, cy
		for i := 0; i < 4; i++ {
			ox, oy := -s.Size.X*ax, -s.Size.Y*ay
			if i&1 != 0 {
				ox = s.Size.X * (1 - ax)
			}
			if i&2 != 0 {
				oy = s.Size.Y * (1 - ay)
			}
			rx := ox*cos - oy*sin
			ry := ox*sin + oy*cos
			loX, loY = minf(loX, cx+rx), minf(loY, cy+ry)
			hiX, hiY = maxf(hiX, cx+rx), maxf(hiY, cy+ry)
		}
		minX = int(math.Floor(float64(loX)))
		minY = int(math.Floor(float64(loY)))
		maxX = int(math.Ceil(float64(hiX)))
		maxY = int(math.Ceil(float64(hiY)))
	}
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > bw-1 {
		maxX = bw - 1
	}
	if maxY > bh-1 {
		maxY = bh - 1
	}

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			// 像素中心相对精灵中心
			px := float32(x) + 0.5 - cx
			py := float32(y) + 0.5 - cy
			if s.Rot != 0 {
				px, py = px*cos+py*sin, -px*sin+py*cos
			}
			lx := px/s.Size.X + ax
			ly := py/s.Size.Y + ay
			if s.FlipX {
				lx = 1 - lx
			}
			if s.FlipY {
				ly = 1 - ly
			}
			if lx < 0 || lx >= 1 || ly < 0 || ly >= 1 {
				continue
			}
			te := s.Texture.SampleNearest((src.X+lx*src.W)/float32(s.Texture.W), (src.Y+ly*src.H)/float32(s.Texture.H))
			col := te.Mul(tint)
			if col.A == 0 {
				continue
			}
			j := y*dst.Stride + x*4
			dstPix := RGBA{dst.Pix[j], dst.Pix[j+1], dst.Pix[j+2], dst.Pix[j+3]}
			out := blendSrcOver(dstPix, col)
			dst.Pix[j], dst.Pix[j+1], dst.Pix[j+2], dst.Pix[j+3] = out.R, out.G, out.B, out.A
		}
	}
}

// Atlas 等尺寸网格图集（按行列布局自动切分）。
type Atlas struct {
	Tex   *Texture
	CellW int
	CellH int
	Cols  int
	Rows  int
	// Overflow 越界时是否换行到下一行（通常保持 false）
	Clamp bool
}

// NewAtlas 按固定格子尺寸切分图集（Cols=0 时按纹理宽度推导）。
func NewAtlas(t *Texture, cellW, cellH, cols int) *Atlas {
	a := &Atlas{Tex: t, CellW: cellW, CellH: cellH, Cols: cols}
	if cellW <= 0 {
		cellW = 1
		a.CellW = 1
	}
	if cellH <= 0 {
		cellH = 1
		a.CellH = 1
	}
	if t != nil {
		if a.Cols <= 0 {
			a.Cols = t.W / cellW
		}
		a.Rows = t.H / cellH
	}
	return a
}

// Cell 按下标取格子源矩形（行主序）。
func (a *Atlas) Cell(i int) (Rect2, bool) {
	if a == nil || a.Cols <= 0 || i < 0 {
		return Rect2{}, false
	}
	col := i % a.Cols
	row := i / a.Cols
	if a.Rows > 0 && row >= a.Rows {
		return Rect2{}, false
	}
	return Rect2{float32(col * a.CellW), float32(row * a.CellH), float32(a.CellW), float32(a.CellH)}, true
}

// Tilemap 瓦片地图（TileW/H 为每个瓦片在屏幕上的像素尺寸）；Tiles 中 -1 表示空。
type Tilemap struct {
	Atlas  *Atlas
	W, H   int
	TileW  int
	TileH  int
	Tiles  []int
	Origin Vec2
	Scale  float32
}

// NewTilemap 创建空瓦片地图。
func NewTilemap(w, h, tw, th int, atlas *Atlas) *Tilemap {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	tiles := make([]int, w*h)
	for i := range tiles {
		tiles[i] = -1
	}
	return &Tilemap{Atlas: atlas, W: w, H: h, TileW: tw, TileH: th, Tiles: tiles, Scale: 1}
}

// Set 设置某格瓦片（越界忽略）。
func (t *Tilemap) Set(tx, ty, tile int) {
	if t == nil || tx < 0 || ty < 0 || tx >= t.W || ty >= t.H {
		return
	}
	t.Tiles[ty*t.W+tx] = tile
}

// SpriteCount 非空瓦片数量（供 FrameStats 使用）。
func (t *Tilemap) SpriteCount() int {
	if t == nil {
		return 0
	}
	n := 0
	for _, v := range t.Tiles {
		if v >= 0 {
			n++
		}
	}
	return n
}

// DrawSprites 实现 Sprite2D（只画屏幕内瓦片）。
func (t *Tilemap) DrawSprites(dst *image.RGBA, w, h int) {
	if t == nil || t.Atlas == nil || dst == nil {
		return
	}
	sc := t.Scale
	if sc <= 0 {
		sc = 1
	}
	tw := float32(t.TileW) * sc
	th := float32(t.TileH) * sc
	for ty := 0; ty < t.H; ty++ {
		for tx := 0; tx < t.W; tx++ {
			tile := t.Tiles[ty*t.W+tx]
			if tile < 0 {
				continue
			}
			cell, ok := t.Atlas.Cell(tile)
			if !ok {
				continue
			}
			sx := t.Origin.X + float32(tx)*tw
			sy := t.Origin.Y + float32(ty)*th
			if sx+tw < 0 || sy+th < 0 || sx > float32(w) || sy > float32(h) {
				continue
			}
			DrawSprite(dst, Sprite{
				Pos: V2(sx, sy), Size: V2(tw, th),
				Texture: t.Atlas.Tex, Src: cell, Color: RGB(255, 255, 255),
			})
		}
	}
}
