// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// Body 运动学刚体（竖直圆柱 + AABB 碰撞，Y 轴向上）。
//
// Pos 是脚底中心（角色站立时 Pos.Y 为地面高度）。
type Body struct {
	Pos, Vel, Acc Vec3
	// Gravity 重力加速度（负数向下，默认 -18）。
	Gravity float32
	// Drag 每秒速度衰减比例（0~1）。
	Drag float32
	// MaxSpeed 水平最大速度（≤ 0 不限）。
	MaxSpeed float32
	// Radius 水平半径（默认 0.35）。
	Radius float32
	// Height 高度（默认 1.7）。
	Height float32
	// Grounded 上一步是否踩在地面/实体上。
	Grounded bool
	// Mass 质量（≤ 0 视为 1）。只影响 PhysicsWorld 里刚体互推时的位移/冲量分配。
	Mass float32
	// Friction 地面摩擦（0~1，每秒水平速度衰减比例；0 = 无摩擦）。
	Friction float32
	// Restitution 弹性（0~1）；0 = 撞上即停，1 = 完全弹性。
	Restitution float32
	// Static 静止体：不受力、不被推动，只充当其它刚体的障碍。
	Static bool
}

// NewBody 创建刚体（默认重力 -18、半径 0.35、高 1.7）。
func NewBody(pos Vec3) *Body {
	return &Body{Pos: pos, Gravity: -18, MaxSpeed: 12, Radius: 0.35, Height: 1.7, Mass: 1}
}

// AABB 刚体包围盒。
func (b *Body) AABB() AABB {
	r := b.Radius
	if r <= 0 {
		r = 0.35
	}
	h := b.Height
	if h <= 0 {
		h = 1.7
	}
	return AABB{
		Min: Vec3{X: b.Pos.X - r, Y: b.Pos.Y, Z: b.Pos.Z - r},
		Max: Vec3{X: b.Pos.X + r, Y: b.Pos.Y + h, Z: b.Pos.Z + r},
	}
}

// ApplyForces 只更新速度（加速度/重力/阻尼/限速），不移动位置。
func (b *Body) ApplyForces(dt float32) {
	if b == nil || dt <= 0 {
		return
	}
	b.Vel.X += b.Acc.X * dt
	b.Vel.Y += b.Acc.Y * dt
	b.Vel.Z += b.Acc.Z * dt
	b.Vel.Y += b.Gravity * dt
	if b.Drag > 0 {
		k := 1 - b.Drag*dt
		if k < 0 {
			k = 0
		}
		b.Vel.X *= k
		b.Vel.Z *= k
	}
	if b.MaxSpeed > 0 {
		h := float32(math.Hypot(float64(b.Vel.X), float64(b.Vel.Z)))
		if h > b.MaxSpeed {
			s := b.MaxSpeed / h
			b.Vel.X *= s
			b.Vel.Z *= s
		}
	}
}

// Integrate 积分速度与位置（无碰撞场景用；带碰撞请用 ApplyForces + MoveAndSlide）。
func (b *Body) Integrate(dt float32) {
	if b == nil || dt <= 0 {
		return
	}
	b.ApplyForces(dt)
	b.Pos.X += b.Vel.X * dt
	b.Pos.Y += b.Vel.Y * dt
	b.Pos.Z += b.Vel.Z * dt
}

// MoveResult 移动与碰撞结果。
type MoveResult struct {
	Hit    bool
	Hits   int
	Normal Vec3
}

// OverlapAABB 两个包围盒是否相交（接触不算相交）。
func OverlapAABB(a, b AABB) bool {
	if a.Max.X <= b.Min.X || a.Min.X >= b.Max.X {
		return false
	}
	if a.Max.Y <= b.Min.Y || a.Min.Y >= b.Max.Y {
		return false
	}
	if a.Max.Z <= b.Min.Z || a.Min.Z >= b.Max.Z {
		return false
	}
	return true
}

// blocked 刚体包围盒是否被静态盒或网格阻挡。
func blocked(b *Body, solids []AABB, grid *CollisionGrid) bool {
	box := b.AABB()
	for i := range solids {
		if OverlapAABB(box, solids[i]) {
			return true
		}
	}
	if grid != nil && grid.OverlapBox(box) {
		return true
	}
	return false
}

// MoveAndSlide 按 X→Z→Y 分轴推进，撞上就回退该轴并清零对应速度（贴墙滑动）。
//
// solids 为静态盒体，grid 可为 nil。必须在 Integrate 之后或直接用 Vel 调用。
func MoveAndSlide(b *Body, dt float32, solids []AABB, grid *CollisionGrid) MoveResult {
	res := MoveResult{}
	if b == nil || dt <= 0 {
		return res
	}
	sx, sz := b.Vel.X*dt, b.Vel.Z*dt

	if sx != 0 {
		b.Pos.X += sx
		if blocked(b, solids, grid) {
			b.Pos.X -= sx
			b.Vel.X = 0
			res.Hit, res.Hits = true, res.Hits+1
			if sx > 0 {
				res.Normal = V3(-1, 0, 0)
			} else {
				res.Normal = V3(1, 0, 0)
			}
		}
	}
	if sz != 0 {
		b.Pos.Z += sz
		if blocked(b, solids, grid) {
			b.Pos.Z -= sz
			b.Vel.Z = 0
			res.Hit, res.Hits = true, res.Hits+1
			if sz > 0 {
				res.Normal = V3(0, 0, -1)
			} else {
				res.Normal = V3(0, 0, 1)
			}
		}
	}

	sy := b.Vel.Y * dt
	if sy != 0 {
		b.Pos.Y += sy
		if blocked(b, solids, grid) {
			b.Pos.Y -= sy
			if sy < 0 {
				b.Grounded = true
				res.Normal = V3(0, 1, 0)
			} else {
				res.Normal = V3(0, -1, 0)
			}
			b.Vel.Y = 0
			res.Hit, res.Hits = true, res.Hits+1
		}
	}

	// 地面探测：向下探 2cm，撞到即为踩实
	save := b.Pos.Y
	b.Pos.Y -= 0.02
	b.Grounded = blocked(b, solids, grid)
	b.Pos.Y = save
	return res
}

// SweepAABB 扫掠盒体与静态盒的最早接触时间（t ∈ [0,1]）。
//
// 使用分离轴滑动法；无接触时返回 t=1。
func SweepAABB(a AABB, delta Vec3, solids []AABB) (t float32, normal Vec3, hit bool) {
	t = 1
	for _, s := range solids {
		st, sn, ok := sweepOne(a, delta, s)
		if ok && st < t {
			t, normal, hit = st, sn, true
		}
	}
	return
}

func sweepOne(a AABB, delta Vec3, s AABB) (float32, Vec3, bool) {
	// 逐轴计算进入/离开时间
	enter, exit := float32(0), float32(1)
	axis := -1
	sign := float32(1)
	check := func(minA, maxA, minB, maxB, d float32) bool {
		if d == 0 {
			if maxA <= minB || minA >= maxB {
				return false
			}
			return true
		}
		t0 := (minB - maxA) / d
		t1 := (maxB - minA) / d
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		if t0 > enter {
			enter = t0
			axis = 1
			sign = d
		} else if t0 == enter && axis < 0 {
			axis = 1
			sign = d
		}
		if t1 < exit {
			exit = t1
		}
		return enter < exit
	}

	if !check(a.Min.X, a.Max.X, s.Min.X, s.Max.X, delta.X) {
		return 0, Vec3{}, false
	}
	if !check(a.Min.Y, a.Max.Y, s.Min.Y, s.Max.Y, delta.Y) {
		return 0, Vec3{}, false
	}
	if !check(a.Min.Z, a.Max.Z, s.Min.Z, s.Max.Z, delta.Z) {
		return 0, Vec3{}, false
	}
	if enter > 1 || enter < 0 {
		return 0, Vec3{}, false
	}
	if axis < 0 {
		return 0, Vec3{}, false
	}
	n := Vec3{}
	if sign > 0 {
		n.X = -1
	} else {
		n.X = 1
	}
	// 根据最早进入轴确定法线（用 delta 的主轴近似，已足够做碰撞响应）
	if math.Abs(float64(delta.X)) >= math.Abs(float64(delta.Y)) && math.Abs(float64(delta.X)) >= math.Abs(float64(delta.Z)) {
		n = Vec3{X: -signFloat(delta.X)}
	} else if math.Abs(float64(delta.Y)) >= math.Abs(float64(delta.Z)) {
		n = Vec3{Y: -signFloat(delta.Y)}
	} else {
		n = Vec3{Z: -signFloat(delta.Z)}
	}
	return enter, n, true
}

func signFloat(v float32) float32 {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}
