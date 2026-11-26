// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// Character 角色控制器（第一/第三人称均可用）。
type Character struct {
	Body *Body
	// Yaw 水平朝向（弧度，0 指向 +Z）。
	Yaw float32
	// Pitch 俯仰（仅影响相机）。
	Pitch float32
	// MoveSpeed 行走速度（米/秒）。
	MoveSpeed float32
	// JumpSpeed 起跳初速。
	JumpSpeed float32
	// EyeHeight 眼睛离脚底的高度。
	EyeHeight float32
	// Fly 飞行模式（忽略重力，空格上升）。
	Fly bool
}

// NewCharacter 创建角色（0.35 半径、1.7 高、4.5 m/s、跳 6.2 m/s）。
func NewCharacter(pos Vec3) *Character {
	return &Character{
		Body:      NewBody(pos),
		MoveSpeed: 4.5,
		JumpSpeed: 6.2,
		EyeHeight: 1.6,
	}
}

// Update 按输入推进角色（行为受 Fly 影响），返回碰撞结果。
func (c *Character) Update(in *Input, dt float32, solids []AABB, grid *CollisionGrid) MoveResult {
	if c == nil || c.Body == nil || dt <= 0 {
		return MoveResult{}
	}
	b := c.Body
	var fwd, side float32
	if in != nil {
		fwd = in.Axis("s", "w")
		side = in.Axis("a", "d")
	}
	sin, cos := float32(math.Sin(float64(c.Yaw))), float32(math.Cos(float64(c.Yaw)))
	// 前向 (sin, 0, cos)，右向 (cos, 0, -sin)
	vx := (sin*fwd + cos*side)
	vz := (cos*fwd - sin*side)
	if l := float32(math.Hypot(float64(vx), float64(vz))); l > 1 {
		vx, vz = vx/l, vz/l
	}
	b.Vel.X = vx * c.MoveSpeed
	b.Vel.Z = vz * c.MoveSpeed

	if c.Fly {
		b.Gravity = 0
		up := float32(0)
		if in != nil {
			up = in.Axis("shift", "space")
		}
		b.Vel.Y = up * c.MoveSpeed
	} else {
		if b.Gravity == 0 {
			b.Gravity = -18
		}
		jump := in != nil && in.KeyPressed("space")
		if jump && b.Grounded {
			b.Vel.Y = c.JumpSpeed
			b.Grounded = false
		}
	}

	b.ApplyForces(dt)
	return MoveAndSlide(b, dt, solids, grid)
}

// ApplyCamera 把相机放到角色眼睛位置（第一人称）。
func (c *Character) ApplyCamera(cam *Camera) {
	if c == nil || c.Body == nil || cam == nil {
		return
	}
	eye := c.Body.Pos.Add(V3(0, c.EyeHeight, 0))
	cam.Pos = eye
	cp, sp := float32(math.Cos(float64(c.Pitch))), float32(math.Sin(float64(c.Pitch)))
	sn, cs := float32(math.Sin(float64(c.Yaw))), float32(math.Cos(float64(c.Yaw)))
	cam.Target = eye.Add(V3(sn*cp, sp, cs*cp))
}
