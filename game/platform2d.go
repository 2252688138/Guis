// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// AABB2 横版 / 俯视 2D 的轴对齐碰撞盒（世界坐标，Y 轴向上）。
type AABB2 struct {
	Min, Max Vec2
}

// Box2 由左上角 + 尺寸构造碰撞盒（Y 轴向上，Min.Y 是底边）。
func Box2(x, y, w, h float32) AABB2 {
	return AABB2{Min: V2(x, y), Max: V2(x+w, y+h)}
}

// Overlaps 是否与另一盒相交（接触不算）。
func (a AABB2) Overlaps(b AABB2) bool {
	return a.Min.X < b.Max.X && a.Max.X > b.Min.X && a.Min.Y < b.Max.Y && a.Max.Y > b.Min.Y
}

// Contains 点是否在盒内。
func (a AABB2) Contains(x, y float32) bool {
	return x >= a.Min.X && x <= a.Max.X && y >= a.Min.Y && y <= a.Max.Y
}

// Center 盒中心。
func (a AABB2) Center() Vec2 { return V2((a.Min.X+a.Max.X)/2, (a.Min.Y+a.Max.Y)/2) }

// W / H 盒尺寸。
func (a AABB2) W() float32 { return a.Max.X - a.Min.X }
func (a AABB2) H() float32 { return a.Max.Y - a.Min.Y }

// Move 平移后的新盒。
func (a AABB2) Move(dx, dy float32) AABB2 {
	return AABB2{Min: V2(a.Min.X+dx, a.Min.Y+dy), Max: V2(a.Max.X+dx, a.Max.Y+dy)}
}

// Camera2D 2D 相机：把世界坐标映射到视口像素（Y 轴向上，屏幕 Y 向下）。
type Camera2D struct {
	// X, Y 视野中心的世界坐标。
	X, Y float32
	// Zoom 缩放（1 = 1 世界单位占 1 像素）。
	Zoom float32
	// W, H 视口像素尺寸。
	W, H int
	// 夹紧范围（Max <= Min 表示该轴不限制）。
	MinX, MinY, MaxX, MaxY float32
	// Rotation 视野旋转（弧度，正值顺时针），配合 Apply / Screen 系列方法生效。
	Rotation float32

	// 抖屏与缩放过渡的内部状态（由 Shake / ZoomTo / Update 维护）。
	shakeAmp, shakeDur, shakeT float32
	zFrom, zTo, zT, zDur       float32
}

// NewCamera2D 创建相机（Zoom = 1）。
func NewCamera2D(w, h int) *Camera2D { return &Camera2D{W: w, H: h, Zoom: 1} }

// SetViewport 更新视口尺寸。
func (c *Camera2D) SetViewport(w, h int) {
	if c == nil {
		return
	}
	c.W, c.H = w, h
}

// Follow 平滑跟随目标（lerp ≤ 0 或 ≥ 1 时立即到位），并应用夹紧。
func (c *Camera2D) Follow(tx, ty, lerp float32) {
	if c == nil {
		return
	}
	if lerp <= 0 || lerp >= 1 {
		c.X, c.Y = tx, ty
	} else {
		c.X += (tx - c.X) * lerp
		c.Y += (ty - c.Y) * lerp
	}
	c.Clamp()
}

// Clamp 把视野中心限制在 Min/Max 范围内。
func (c *Camera2D) Clamp() {
	if c == nil {
		return
	}
	hw, hh := c.HalfSize()
	if c.MaxX > c.MinX {
		if c.X-hw < c.MinX {
			c.X = c.MinX + hw
		}
		if c.X+hw > c.MaxX {
			c.X = c.MaxX - hw
		}
	}
	if c.MaxY > c.MinY {
		if c.Y-hh < c.MinY {
			c.Y = c.MinY + hh
		}
		if c.Y+hh > c.MaxY {
			c.Y = c.MaxY - hh
		}
	}
}

// HalfSize 视野半宽 / 半高（世界单位）。
func (c *Camera2D) HalfSize() (float32, float32) {
	if c == nil {
		return 0, 0
	}
	z := c.Zoom
	if z <= 0 {
		z = 1
	}
	return float32(c.W) / 2 / z, float32(c.H) / 2 / z
}

// WorldToScreen 世界坐标 → 屏幕像素（左上原点，Y 向下）。
func (c *Camera2D) WorldToScreen(x, y float32) (float32, float32) {
	if c == nil {
		return x, y
	}
	z := c.Zoom
	if z <= 0 {
		z = 1
	}
	return (x - c.X) * z + float32(c.W)/2, float32(c.H)/2 - (y-c.Y)*z
}

// ScreenToWorld 屏幕像素 → 世界坐标。
func (c *Camera2D) ScreenToWorld(sx, sy float32) (float32, float32) {
	if c == nil {
		return sx, sy
	}
	z := c.Zoom
	if z <= 0 {
		z = 1
	}
	return (sx-float32(c.W)/2)/z + c.X, c.Y - (sy-float32(c.H)/2)/z
}

// Visible 世界坐标是否在视野内（margin 为外扩边界）。
func (c *Camera2D) Visible(x, y, margin float32) bool {
	hw, hh := c.HalfSize()
	return x >= c.X-hw-margin && x <= c.X+hw+margin && y >= c.Y-hh-margin && y <= c.Y+hh+margin
}

// PlatformerBody 横版平台跳跃刚体（世界坐标 X 右 / Y 上，Pos 为碰撞盒中心）。
type PlatformerBody struct {
	Pos, Vel Vec2
	// Size 碰撞盒尺寸（像素，Y 向上不管）。
	Size Vec2
	// Gravity 重力加速度（像素/秒²，负数向下，默认 -2200）。
	Gravity float32
	// MaxFall 最大下落速度（正数，默认 1400）。
	MaxFall float32
	// Grounded 上一步是否站在实体 / 单向平台上。
	Grounded bool
	// HitCeiling 上一步是否顶到天花板（撞砖块判定用）。
	HitCeiling bool
	// HitWall 上一步是否水平撞墙。
	HitWall bool
}

// NewPlatformerBody 创建刚体（x, y 为中心，w, h 为碰撞盒尺寸）。
func NewPlatformerBody(x, y, w, h float32) *PlatformerBody {
	return &PlatformerBody{Pos: V2(x, y), Size: V2(w, h), Gravity: -2200, MaxFall: 1400}
}

// Box 当前碰撞盒。
func (b *PlatformerBody) Box() AABB2 {
	hw, hh := b.Size.X/2, b.Size.Y/2
	return AABB2{Min: V2(b.Pos.X-hw, b.Pos.Y-hh), Max: V2(b.Pos.X+hw, b.Pos.Y+hh)}
}

// Step 推进一帧：先水平后垂直分轴解析（solids 为实体，oneWay 为单向平台）。
// 单向平台只从上方落地，可从下方穿过；下落中且上一帧底边不低于平台顶面才参与碰撞。
func (b *PlatformerBody) Step(dt float32, solids, oneWay []AABB2) {
	if b == nil || dt <= 0 {
		return
	}
	if b.MaxFall <= 0 {
		b.MaxFall = 1400
	}
	b.Vel.Y += b.Gravity * dt
	if b.Vel.Y < -b.MaxFall {
		b.Vel.Y = -b.MaxFall
	}
	b.HitWall, b.HitCeiling = false, false

	// ---- 水平轴 ----
	box := b.Box()
	dx := b.Vel.X * dt
	if dx != 0 {
		box.Min.X += dx
		box.Max.X += dx
		for _, s := range solids {
			if !box.Overlaps(s) {
				continue
			}
			if dx > 0 {
				box.Max.X = s.Min.X
				box.Min.X = box.Max.X - b.Size.X
			} else {
				box.Min.X = s.Max.X
				box.Max.X = box.Min.X + b.Size.X
			}
			b.Vel.X = 0
			b.HitWall = true
			break
		}
	}

	// ---- 垂直轴 ----
	prevBottom := box.Min.Y
	dy := b.Vel.Y * dt
	box.Min.Y += dy
	box.Max.Y += dy
	b.Grounded = false
	if dy < 0 {
		// 下落：先实体，再单向平台
		for _, s := range solids {
			if !box.Overlaps(s) {
				continue
			}
			box.Min.Y = s.Max.Y
			box.Max.Y = box.Min.Y + b.Size.Y
			b.Vel.Y = 0
			b.Grounded = true
			break
		}
		if !b.Grounded {
			for _, s := range oneWay {
				if prevBottom < s.Max.Y-0.01 {
					continue // 之前就在平台顶面以下，不阻挡
				}
				if !box.Overlaps(s) {
					continue
				}
				box.Min.Y = s.Max.Y
				box.Max.Y = box.Min.Y + b.Size.Y
				b.Vel.Y = 0
				b.Grounded = true
				break
			}
		}
	} else if dy > 0 {
		for _, s := range solids {
			if !box.Overlaps(s) {
				continue
			}
			box.Max.Y = s.Min.Y
			box.Min.Y = box.Max.Y - b.Size.Y
			b.Vel.Y = 0
			b.HitCeiling = true
			break
		}
	}

	b.Pos = V2((box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2)
}

// Bottom / Top / Left / Right 碰撞盒边界快捷值。
func (b *PlatformerBody) Bottom() float32 { return b.Pos.Y - b.Size.Y/2 }
func (b *PlatformerBody) Top() float32    { return b.Pos.Y + b.Size.Y/2 }
func (b *PlatformerBody) Left() float32   { return b.Pos.X - b.Size.X/2 }
func (b *PlatformerBody) Right() float32  { return b.Pos.X + b.Size.X/2 }

// Landed 上一帧是否从空中落到地面（业务侧在 Step 后比较）。
func (b *PlatformerBody) Landed() bool { return b.Grounded && b.Vel.Y == 0 }
