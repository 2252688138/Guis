// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "image"

// ParallaxLayer 视差背景层。
//
// Factor 控制相对相机的滚动速度：0 = 完全不动（远景天空），1 = 与世界同速（前景）。
// 常用：远景 0.1~0.2，中景 0.4，近景 0.7。
type ParallaxLayer struct {
	// Texture 平铺纹理（必填）。
	Texture *Texture
	// Factor 滚动系数（X/Y 可分别设置，天空背景一般 Y 设 0）。
	Factor Vec2
	// Offset 额外偏移（像素，正值向右/向下）。
	Offset Vec2
	// Scale 纹理放大倍数（默认 1）。
	Scale float32
	// Tint 乘色（A=0 按不透明处理）。
	Tint RGBA
	// NoRepeat 不重复平铺（只画一块，其余留空）。
	NoRepeat bool
}

// Parallax 多层视差背景（Draw 时按层序从远到近绘制）。
type Parallax struct {
	Layers []ParallaxLayer
}

// Add 追加一层（链式）。
func (p *Parallax) Add(l ParallaxLayer) *Parallax {
	if p == nil {
		return nil
	}
	p.Layers = append(p.Layers, l)
	return p
}

// Clear 清空所有层。
func (p *Parallax) Clear() {
	if p == nil {
		return
	}
	p.Layers = p.Layers[:0]
}

// Draw 按相机位置绘制所有层（在场景精灵之前调用）。
func (p *Parallax) Draw(dst *image.RGBA, cam *Camera2D) {
	if p == nil || dst == nil {
		return
	}
	var cx, cy float32
	if cam != nil {
		cx, cy = cam.Center()
	}
	w := dst.Rect.Dx()
	h := dst.Rect.Dy()
	if w <= 0 || h <= 0 {
		return
	}
	for _, l := range p.Layers {
		if l.Texture == nil || l.Texture.W <= 0 || l.Texture.H <= 0 {
			continue
		}
		s := l.Scale
		if s <= 0 {
			s = 1
		}
		tw := float32(l.Texture.W) * s
		th := float32(l.Texture.H) * s
		if tw <= 0 || th <= 0 {
			continue
		}
		// 视差滚动偏移（相机中心 × 系数），取负使其反向移动
		ox := -cx*l.Factor.X + l.Offset.X
		oy := -cy*l.Factor.Y + l.Offset.Y
		startX := modf32(ox, tw)
		startY := modf32(oy, th)
		if startX > 0 {
			startX -= tw
		}
		if startY > 0 {
			startY -= th
		}
		maxX, maxY := w, h
		if l.NoRepeat {
			maxX = int(tw) + 1
			maxY = int(th) + 1
		}
		for y := startY; y < float32(maxY); y += th {
			for x := startX; x < float32(maxX); x += tw {
				DrawSprite(dst, Sprite{
					Pos:     Vec2{X: x, Y: y},
					Size:    Vec2{X: tw, Y: th},
					Texture: l.Texture,
					Color:   l.Tint,
				})
			}
		}
	}
}

// modf32 取模并把结果归一到 [0, m)。
func modf32(v, m float32) float32 {
	if m <= 0 {
		return 0
	}
	r := v - float32(int(v/m))*m
	if r < 0 {
		r += m
	}
	return r
}

// StarfieldTexture 生成一张星点背景纹理（无美术资源也能做视差星空）。
func StarfieldTexture(size, stars int, bg RGBA) *Texture {
	if size < 8 {
		size = 8
	}
	t := NewTexture(size, size)
	t.Fill(bg)
	// 确定性伪随机（同一参数每次生成结果一致）
	seed := uint32(size*7919 + stars*104729 + 1)
	for i := 0; i < stars; i++ {
		seed = seed*1664525 + 1013904223
		x := int(seed>>8) % size
		seed = seed*1664525 + 1013904223
		y := int(seed>>8) % size
		b := uint8(140 + (seed>>16)%116)
		t.Set(x, y, RGBA{R: b, G: b, B: 255, A: 255})
	}
	return t
}
