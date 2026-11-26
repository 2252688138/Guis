// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

// ground 返回一块 16×16 的静态地面（顶面 y=0）。
func ground() game.AABB {
	return game.AABB{Min: game.V3(-8, -1, -8), Max: game.V3(8, 0, 8)}
}

// floatBody 创建无重力、无速度的测试刚体（半径 0.5、高 1），便于隔离物理行为。
func floatBody(x, z float32) *game.Body {
	b := game.NewBody(game.V3(x, 0, z))
	b.Gravity = 0
	b.MaxSpeed = 0
	b.Drag = 0
	b.Radius = 0.5
	b.Height = 1
	return b
}

func TestPhysicsWorldSeparatesOverlap(t *testing.T) {
	a := floatBody(0, 0)
	b := floatBody(0.9, 0)
	b.Vel = game.V3(-6, 0, 0)

	w := game.NewPhysicsWorld().Add(a, b)
	w.Step(0.1)

	if game.OverlapAABB(a.AABB(), b.AABB()) {
		t.Fatalf("Step 后仍重叠: a=%v b=%v", a.Pos, b.Pos)
	}
	// 等质量非弹性：两者获得同一水平速度（动量守恒）。
	if !nearF(a.Vel.X, -3, 0.01) || !nearF(b.Vel.X, -3, 0.01) {
		t.Fatalf("非弹性碰撞速度分配错误: a=%v b=%v", a.Vel, b.Vel)
	}
}

func TestPhysicsWorldElasticSwap(t *testing.T) {
	a := floatBody(0, 0)
	b := floatBody(0.9, 0)
	a.Restitution, b.Restitution = 1, 1
	b.Vel = game.V3(-6, 0, 0)

	w := game.NewPhysicsWorld().Add(a, b)
	w.Step(0.1)

	if !nearF(a.Vel.X, -6, 0.01) {
		t.Fatalf("完全弹性等质量应交换速度，a.Vel.X=%v", a.Vel.X)
	}
	if !nearF(b.Vel.X, 0, 0.01) {
		t.Fatalf("完全弹性等质量应交换速度，b.Vel.X=%v", b.Vel.X)
	}
}

func TestPhysicsWorldMassWeighting(t *testing.T) {
	light := floatBody(0, 0)
	light.Mass = 1
	heavy := floatBody(0.9, 0)
	heavy.Mass = 9

	before := light.Pos.X
	w := game.NewPhysicsWorld().Add(light, heavy)
	w.Step(0.1)

	moved := light.Pos.X - before
	if moved > -0.05 {
		t.Fatalf("轻的刚体应被推得更远，实际位移 %v", moved)
	}
	if light.Pos.X >= heavy.Pos.X {
		t.Fatalf("分离方向错误: light=%v heavy=%v", light.Pos.X, heavy.Pos.X)
	}
}

func TestPhysicsWorldStaticObstacle(t *testing.T) {
	wall := floatBody(1.0, 0)
	wall.Static = true
	wall.Pos.Y = 0
	mover := floatBody(0, 0)
	mover.Vel = game.V3(5, 0, 0)

	wallBefore := wall.Pos
	w := game.NewPhysicsWorld().Add(wall, mover)
	w.Step(0.1)

	if wall.Pos != wallBefore {
		t.Fatalf("Static 刚体不应被推动: %v -> %v", wallBefore, wall.Pos)
	}
	if game.OverlapAABB(wall.AABB(), mover.AABB()) {
		t.Fatal("运动刚体应被 Static 刚体挡开")
	}
	if mover.Vel.X > 0 {
		t.Fatalf("撞上 Static 后水平速度应被吸收，实际 %v", mover.Vel.X)
	}
}

func TestPhysicsWorldGroundFriction(t *testing.T) {
	b := floatBody(0, 0)
	b.Friction = 1
	b.Vel = game.V3(10, 0, 0)

	w := game.NewPhysicsWorld()
	w.Solids = append(w.Solids, ground())
	w.Add(b)
	w.Step(0.1)

	if !b.Grounded {
		t.Fatal("应判定为踩在地面上")
	}
	if !nearF(b.Vel.X, 9, 0.02) {
		t.Fatalf("摩擦 1 每秒应衰减 10%% ，实际 %v", b.Vel.X)
	}

	// 无摩擦时速度不变。
	c := floatBody(4, 0)
	c.Vel = game.V3(10, 0, 0)
	w2 := game.NewPhysicsWorld()
	w2.Solids = append(w2.Solids, ground())
	w2.Add(c)
	w2.Step(0.1)
	if !nearF(c.Vel.X, 10, 0.02) {
		t.Fatalf("Friction=0 不应衰减速度，实际 %v", c.Vel.X)
	}
}

func TestPhysicsWorldStacking(t *testing.T) {
	base := floatBody(0, 0) // 顶面在 y=1
	top := floatBody(0, 0)
	top.Pos.Y = 1.1
	top.Vel = game.V3(0, -4, 0)

	w := game.NewPhysicsWorld().Add(base, top)
	for i := 0; i < 5; i++ {
		w.Step(1.0 / 60.0)
	}

	if top.Pos.Y < 0.9 || top.Pos.Y > 1.05 {
		t.Fatalf("上方刚体应停在底座的顶面 y≈1，实际 y=%v", top.Pos.Y)
	}
	if !top.Grounded {
		t.Fatal("站在另一个刚体上应标记 Grounded")
	}
	if top.Vel.Y < 0 {
		t.Fatalf("落到顶面后竖直速度应归零，实际 %v", top.Vel.Y)
	}
}

func TestPhysicsWorldBounceOnBody(t *testing.T) {
	base := floatBody(0, 0)
	ball := floatBody(0, 0)
	ball.Pos.Y = 1.4
	ball.Vel = game.V3(0, -6, 0)
	ball.Restitution = 1

	w := game.NewPhysicsWorld().Add(base, ball)
	w.Step(0.1)

	if ball.Pos.Y < 0.99 || ball.Pos.Y > 1.01 {
		t.Fatalf("弹性刚体应被吸附到底座顶面，实际 y=%v", ball.Pos.Y)
	}
	if !nearF(ball.Vel.Y, 6, 0.01) {
		t.Fatalf("Restitution=1 应以同样速度弹回，实际 Vel.Y=%v", ball.Vel.Y)
	}
}

func TestPhysicsWorldRemove(t *testing.T) {
	a := floatBody(0, 0)
	b := floatBody(3, 0)
	w := game.NewPhysicsWorld().Add(a, b)
	if len(w.Bodies) != 2 {
		t.Fatalf("Bodies=%d", len(w.Bodies))
	}
	if !w.Remove(a) {
		t.Fatal("Remove 应返回 true")
	}
	if len(w.Bodies) != 1 || w.Bodies[0] != b {
		t.Fatalf("移除后 Bodies=%v", w.Bodies)
	}
	if w.Remove(a) {
		t.Fatal("重复移除应返回 false")
	}
}

func TestPhysicsWorldNoDoubleIntegrate(t *testing.T) {
	// Step 内部只 ApplyForces 一次：无重力无外力时位置不应改变。
	b := floatBody(2, 2)
	w := game.NewPhysicsWorld()
	w.Add(b)
	w.Step(0.5)
	if b.Pos != game.V3(2, 0, 2) {
		t.Fatalf("静止刚体不应移动，实际 %v", b.Pos)
	}
}
