// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// Camera2D 的增强能力：旋转、抖屏、平滑缩放、世界↔屏幕互转与批量绘制。
//
// 基础字段与 Follow / Clamp / WorldToScreen 在 platform2d.go，这里补齐做 2D 游戏
// 常需的"手感"类功能：
//
//	cam := game.NewCamera2D(640, 360)
//	cam.Follow(playerPos.X, playerPos.Y, 0.2)
//	cam.Shake(6, 0.25)          // 受击抖屏
//	cam.ZoomTo(1.35, 0.4)       // 平滑推近
//	cam.Update(dt)              // 每帧推进抖屏 / 缩放过渡
//	for i := range actors {
//		batch.Add(cam.Apply(actors[i].Sprite))
//	}

// Update 推进抖屏与平滑缩放（每帧调用一次；不调用则抖屏不会衰减）。
func (c *Camera2D) Update(dt float32) {
	if c == nil || dt <= 0 {
		return
	}
	if c.shakeT > 0 {
		c.shakeT -= dt
		if c.shakeT < 0 {
			c.shakeT = 0
		}
	}
	if c.zDur > 0 {
		c.zT += dt / c.zDur
		if c.zT >= 1 {
			c.Zoom = c.zTo
			c.zT = 0
			c.zDur = 0
		} else {
			c.Zoom = c.zFrom + (c.zTo-c.zFrom)*easeOut(c.zT)
		}
	}
}

// easeOut 缓出曲线（t ∈ [0,1]）。
func easeOut(t float32) float32 {
	t = Clamp(t, 0, 1)
	return 1 - (1-t)*(1-t)
}

// Shake 触发抖屏（amp 为像素幅度，dur 为持续秒数）。
func (c *Camera2D) Shake(amp, dur float32) {
	if c == nil || amp <= 0 || dur <= 0 {
		return
	}
	c.shakeAmp = amp
	c.shakeDur = dur
	c.shakeT = dur
}

// Shaking 当前是否处于抖屏状态。
func (c *Camera2D) Shaking() bool { return c != nil && c.shakeT > 0 }

// ZoomTo 平滑缩放到目标倍率（dur <= 0 时立即生效）。
func (c *Camera2D) ZoomTo(to, dur float32) {
	if c == nil {
		return
	}
	if to <= 0 {
		to = 0.01
	}
	if dur <= 0 {
		c.Zoom = to
		c.zDur, c.zT = 0, 0
		return
	}
	c.zFrom, c.zTo, c.zT, c.zDur = c.effectiveZoom(), to, 0, dur
}

// effectiveZoom 实际缩放（≤0 时按 1 处理，避免除零）。
func (c *Camera2D) effectiveZoom() float32 {
	if c == nil || c.Zoom <= 0 {
		return 1
	}
	return c.Zoom
}

// shakeOffset 当前抖屏偏移（确定性正弦组合，不需要随机数种子）。
func (c *Camera2D) shakeOffset() (float32, float32) {
	if c == nil || c.shakeT <= 0 || c.shakeAmp <= 0 {
		return 0, 0
	}
	k := float32(1)
	if c.shakeDur > 0 {
		k = c.shakeT / c.shakeDur
	}
	amp := c.shakeAmp * k * k
	t := c.shakeDur - c.shakeT
	return float32(math.Sin(float64(t*57.3))) * amp,
		float32(math.Cos(float64(t*41.7))) * amp
}

// Center 返回含抖屏偏移的视野中心（世界坐标）。
func (c *Camera2D) Center() (float32, float32) {
	if c == nil {
		return 0, 0
	}
	ox, oy := c.shakeOffset()
	return c.X + ox/c.effectiveZoom(), c.Y + oy/c.effectiveZoom()
}

// Screen 世界坐标 → 屏幕像素（含缩放、旋转、抖屏）。
func (c *Camera2D) Screen(x, y float32) (float32, float32) {
	if c == nil {
		return x, y
	}
	cx, cy := c.Center()
	dx, dy := x-cx, y-cy
	if c.Rotation != 0 {
		cos, sin := float32(math.Cos(float64(c.Rotation))), float32(math.Sin(float64(c.Rotation)))
		dx, dy = dx*cos+dy*sin, -dx*sin+dy*cos
	}
	z := c.effectiveZoom()
	return float32(c.W)*0.5 + dx*z, float32(c.H)*0.5 + dy*z
}

// World 屏幕像素 → 世界坐标（Screen 的逆变换）。
func (c *Camera2D) World(sx, sy float32) (float32, float32) {
	if c == nil {
		return sx, sy
	}
	z := c.effectiveZoom()
	dx := (sx - float32(c.W)*0.5) / z
	dy := (sy - float32(c.H)*0.5) / z
	if c.Rotation != 0 {
		cos, sin := float32(math.Cos(float64(c.Rotation))), float32(math.Sin(float64(c.Rotation)))
		dx, dy = dx*cos-dy*sin, dx*sin+dy*cos
	}
	cx, cy := c.Center()
	return cx + dx, cy + dy
}

// Apply 把世界坐标的精灵（Pos 为左上角，世界单位）转换为屏幕坐标精灵。
//
// 位置、尺寸、旋转都按相机变换；Tint / Src / Flip 等属性原样保留。
func (c *Camera2D) Apply(s Sprite) Sprite {
	if c == nil {
		return s
	}
	z := c.effectiveZoom()
	ccx, ccy := s.Pos.X+s.Size.X*0.5, s.Pos.Y+s.Size.Y*0.5
	sx, sy := c.Screen(ccx, ccy)
	s.Size = s.Size.Scale(z)
	s.Pos = V2(sx-s.Size.X*0.5, sy-s.Size.Y*0.5)
	s.Rot += c.Rotation
	return s
}

// ApplyBatch 批量把世界坐标精灵转换到屏幕坐标并写入批处理（自动视口剔除）。
func (c *Camera2D) ApplyBatch(dst *SpriteBatch, src []Sprite) {
	if c == nil || dst == nil {
		return
	}
	for i := range src {
		w := src[i]
		if !c.VisibleRect(w.Pos.X, w.Pos.Y, w.Pos.X+w.Size.X, w.Pos.Y+w.Size.Y) {
			continue
		}
		dst.Add(c.Apply(w))
	}
}

// VisibleRect 世界矩形是否与当前视口相交（用于对象剔除）。
func (c *Camera2D) VisibleRect(minX, minY, maxX, maxY float32) bool {
	if c == nil {
		return true
	}
	x0, y0 := c.Screen(minX, minY)
	x1, y1 := c.Screen(minX, maxY)
	x2, y2 := c.Screen(maxX, minY)
	x3, y3 := c.Screen(maxX, maxY)
	sminX := minf(minf(x0, x1), minf(x2, x3))
	sminY := minf(minf(y0, y1), minf(y2, y3))
	smaxX := maxf(maxf(x0, x1), maxf(x2, x3))
	smaxY := maxf(maxf(y0, y1), maxf(y2, y3))
	return smaxX > 0 && smaxY > 0 && sminX < float32(c.W) && sminY < float32(c.H)
}

// VisibleBounds 返回当前视口对应的世界包围盒（AABB2）。
func (c *Camera2D) VisibleBounds() AABB2 {
	if c == nil {
		return AABB2{}
	}
	x0, y0 := c.World(0, 0)
	x1, y1 := c.World(0, float32(c.H))
	x2, y2 := c.World(float32(c.W), 0)
	x3, y3 := c.World(float32(c.W), float32(c.H))
	return AABB2{
		Min: V2(minf(minf(x0, x1), minf(x2, x3)), minf(minf(y0, y1), minf(y2, y3))),
		Max: V2(maxf(maxf(x0, x1), maxf(x2, x3)), maxf(maxf(y0, y1), maxf(y2, y3))),
	}
}

// ZoomBy 在当前缩放基础上乘一个系数（立即生效）。
func (c *Camera2D) ZoomBy(k float32) {
	if c == nil || k <= 0 {
		return
	}
	c.Zoom = c.effectiveZoom() * k
	c.zDur, c.zT = 0, 0
}

// SetBounds 设置相机活动范围（世界坐标，Max <= Min 表示该轴不限制）。
func (c *Camera2D) SetBounds(minX, minY, maxX, maxY float32) {
	if c == nil {
		return
	}
	c.MinX, c.MinY, c.MaxX, c.MaxY = minX, minY, maxX, maxY
	c.Clamp()
}
