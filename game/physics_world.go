// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// PhysicsWorld 轻量刚体世界：在 MoveAndSlide 的静态碰撞之上，
// 补齐刚体之间的水平推挤、叠放（站到彼此头顶）与摩擦/弹性材质。
//
// 用法（固定步长里调用）：
//
//	w := game.NewPhysicsWorld()
//	w.Add(game.NewBody(game.V3(0, 0.5, 0)), game.NewBody(game.V3(2, 0.5, 0)))
//	w.Solids = append(w.Solids, game.AABB{Min: game.V3(-8, -1, -8), Max: game.V3(8, 0, 8)})
//	// 每帧：
//	w.Step(dt)
//
// 注意：Character 自带 Update（内部已 MoveAndSlide），不要再把同一个 Body
// 加进 PhysicsWorld 的 Bodies，否则会重复积分；Character 想把别的刚体推开时，
// 把那些刚体放进 Bodies、角色包围盒临时塞进 Solids 即可。
type PhysicsWorld struct {
	// Bodies 参与动力学的刚体（Static 的只作障碍）。
	Bodies []*Body
	// Solids 静态盒（地面、墙）。
	Solids []AABB
	// Grid 静态碰撞网格（Tiled 关卡，可空）。
	Grid *CollisionGrid
	// Iterations 刚体间推挤的迭代次数（默认 2）。
	Iterations int
}

// NewPhysicsWorld 创建空世界（推挤迭代 2 次）。
func NewPhysicsWorld() *PhysicsWorld { return &PhysicsWorld{Iterations: 2} }

// Add 加入刚体并返回自身（链式）。
func (w *PhysicsWorld) Add(bs ...*Body) *PhysicsWorld {
	w.Bodies = append(w.Bodies, bs...)
	return w
}

// Remove 移除刚体，返回是否找到。
func (w *PhysicsWorld) Remove(b *Body) bool {
	for i, x := range w.Bodies {
		if x == b {
			w.Bodies = append(w.Bodies[:i], w.Bodies[i+1:]...)
			return true
		}
	}
	return false
}

// Step 推进一个固定步长：受力 → 静态碰撞移动 → 刚体互推 → 叠放 → 地面摩擦。
func (w *PhysicsWorld) Step(dt float32) {
	if w == nil || dt <= 0 {
		return
	}
	for _, b := range w.Bodies {
		if b == nil || b.Static {
			continue
		}
		b.ApplyForces(dt)
		MoveAndSlide(b, dt, w.Solids, w.Grid)
	}
	iters := w.Iterations
	if iters <= 0 {
		iters = 2
	}
	for i := 0; i < iters; i++ {
		w.resolvePairs()
	}
	w.resolveStacking(dt)
	for _, b := range w.Bodies {
		if b == nil || b.Static {
			continue
		}
		w.applyGroundFriction(b, dt)
	}
}

// applyGroundFriction 踩实后按 Friction 衰减水平速度（模拟地面阻尼）。
func (w *PhysicsWorld) applyGroundFriction(b *Body, dt float32) {
	if b.Friction <= 0 || !b.Grounded {
		return
	}
	k := 1 - b.Friction*dt
	if k < 0 {
		k = 0
	}
	b.Vel.X *= k
	b.Vel.Z *= k
}

// bodyMass 质量（≤0 视为 1）。
func bodyMass(b *Body) float32 {
	if b == nil || b.Mass <= 0 {
		return 1
	}
	return b.Mass
}

// bodyHeight 刚体高度（≤0 视为 1.7，与 AABB 保持一致）。
func bodyHeight(b *Body) float32 {
	if b == nil || b.Height <= 0 {
		return 1.7
	}
	return b.Height
}

// horizontalOverlap a 与 b 在 X/Z 平面上是否投影重叠。
func horizontalOverlap(a, b *Body) bool {
	ra, rb := a.Radius, b.Radius
	if ra <= 0 {
		ra = 0.35
	}
	if rb <= 0 {
		rb = 0.35
	}
	return float32(math.Abs(float64(a.Pos.X-b.Pos.X))) < ra+rb &&
		float32(math.Abs(float64(a.Pos.Z-b.Pos.Z))) < ra+rb
}

// resolvePairs 处理刚体间的水平推挤（位置分离 + 法向冲量）。
func (w *PhysicsWorld) resolvePairs() {
	bs := w.Bodies
	for i := 0; i < len(bs); i++ {
		a := bs[i]
		if a == nil {
			continue
		}
		for j := i + 1; j < len(bs); j++ {
			b := bs[j]
			if b == nil || (a.Static && b.Static) {
				continue
			}
			ba, bb := a.AABB(), b.AABB()
			if !OverlapAABB(ba, bb) {
				continue
			}
			ox := minF(ba.Max.X, bb.Max.X) - maxF(ba.Min.X, bb.Min.X)
			oy := minF(ba.Max.Y, bb.Max.Y) - maxF(ba.Min.Y, bb.Min.Y)
			oz := minF(ba.Max.Z, bb.Max.Z) - maxF(ba.Min.Z, bb.Min.Z)
			// 竖直重叠远小于水平重叠时判定为“上下叠放”，交给 resolveStacking。
			if oy <= minF(ox, oz)*0.5 {
				continue
			}

			var n Vec3
			pen := ox
			if oz < ox {
				pen = oz
				if b.Pos.Z >= a.Pos.Z {
					n = V3(0, 0, 1)
				} else {
					n = V3(0, 0, -1)
				}
			} else if b.Pos.X >= a.Pos.X {
				n = V3(1, 0, 0)
			} else {
				n = V3(-1, 0, 0)
			}

			invA, invB := float32(0), float32(0)
			if !a.Static {
				invA = 1 / bodyMass(a)
			}
			if !b.Static {
				invB = 1 / bodyMass(b)
			}
			sum := invA + invB
			if sum <= 0 {
				continue
			}

			// 位置分离：按逆向质量分配，避免轻的刚体被重的挤飞。
			push := n.Scale(pen / sum)
			if !a.Static {
				a.Pos = a.Pos.Sub(push.Scale(invA))
			}
			if !b.Static {
				b.Pos = b.Pos.Add(push.Scale(invB))
			}

			// 法向冲量：只处理相向运动，e 取两者弹性的平均。
			rv := b.Vel.Sub(a.Vel)
			vn := rv.Dot(n)
			if vn >= 0 {
				continue
			}
			e := (a.Restitution + b.Restitution) * 0.5
			if e < 0 {
				e = 0
			}
			if e > 1 {
				e = 1
			}
			jImp := -(1 + e) * vn / sum
			if !a.Static {
				a.Vel = a.Vel.Sub(n.Scale(jImp * invA))
			}
			if !b.Static {
				b.Vel = b.Vel.Add(n.Scale(jImp * invB))
			}
		}
	}
}

// resolveStacking 让刚体能站在彼此头顶：下落穿越顶面时吸附并按弹性反弹。
func (w *PhysicsWorld) resolveStacking(dt float32) {
	bs := w.Bodies
	for _, a := range bs {
		if a == nil || a.Static || a.Vel.Y > 0 {
			continue
		}
		for _, b := range bs {
			if b == nil || a == b || !horizontalOverlap(a, b) {
				continue
			}
			top := b.Pos.Y + bodyHeight(b)
			if a.Pos.Y > top {
				continue // 还在顶面之上
			}
			if a.Pos.Y+bodyHeight(a) <= b.Pos.Y {
				continue // 已落到 b 下方
			}
			tol := float32(0.08)
			if step := -a.Vel.Y * dt; step > tol {
				tol = step + 0.02
			}
			// 本帧起点（prevy）在顶面之上、现在不超过顶面 => 本帧穿越了顶面；
			// 否则要求当前位置贴着顶面，避免半空中误吸附。
			prevy := a.Pos.Y - a.Vel.Y*dt
			crossed := prevy >= top && a.Pos.Y <= top
			nearTop := a.Pos.Y >= top-tol
			if !crossed && !nearTop {
				continue
			}
			a.Pos.Y = top
			a.Grounded = true
			if a.Restitution > 0 && -a.Vel.Y > 1 {
				a.Vel.Y = -a.Vel.Y * a.Restitution
			} else {
				a.Vel.Y = 0
			}
		}
	}
}

// minF / maxF float32 版取小/取大（避免每处导入 math 的浮点转换）。
func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
