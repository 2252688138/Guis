// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"math"
	"testing"

	"guis/game"
)

func TestBodyFreeFall(t *testing.T) {
	b := game.NewBody(game.V3(0, 10, 0))
	for i := 0; i < 60; i++ {
		b.Integrate(1.0 / 60)
	}
	if b.Vel.Y > -16 || b.Vel.Y < -20 {
		t.Fatalf("1 秒后 vel.Y = %.2f，期望约 -18", b.Vel.Y)
	}
	if b.Pos.Y > 2 || b.Pos.Y < 0.5 {
		t.Fatalf("1 秒后高度 = %.2f，期望约 1", b.Pos.Y)
	}
}

func TestBodyMaxSpeed(t *testing.T) {
	b := game.NewBody(game.V3(0, 0, 0))
	b.Gravity = 0
	b.MaxSpeed = 5
	b.Acc = game.V3(100, 0, 100)
	b.Integrate(0.5)
	if h := float32(math.Hypot(float64(b.Vel.X), float64(b.Vel.Z))); h > 5.01 {
		t.Fatalf("限速失效，水平速度 = %.2f", h)
	}
}

func TestMoveAndSlideWall(t *testing.T) {
	wall := []game.AABB{{Min: game.V3(2, -1, -20), Max: game.V3(3, 4, 20)}}
	b := game.NewBody(game.V3(0, 0, 0))
	b.Gravity = 0
	b.Vel = game.V3(10, 0, 5)
	for i := 0; i < 40; i++ {
		b.Vel.X, b.Vel.Z = 10, 5
		game.MoveAndSlide(b, 0.05, wall, nil)
	}
	if b.Pos.X > 1.66 {
		t.Fatalf("应被墙挡住（x≈1.65），实际 %.3f", b.Pos.X)
	}
	if b.Pos.X < 1.5 {
		t.Fatalf("未贴近墙面，x=%.3f", b.Pos.X)
	}
	if b.Pos.Z < 5 {
		t.Fatalf("应沿墙滑动，z=%.3f", b.Pos.Z)
	}
}

func TestMoveAndSlideGround(t *testing.T) {
	floor := []game.AABB{{Min: game.V3(-10, -1, -10), Max: game.V3(10, 0, 10)}}
	b := game.NewBody(game.V3(0, 2, 0))
	b.Vel.Y = -5
	landed := false
	for i := 0; i < 200; i++ {
		b.Vel.Y += b.Gravity * 0.016
		game.MoveAndSlide(b, 0.016, floor, nil)
		if b.Grounded {
			landed = true
			break
		}
	}
	if !landed {
		t.Fatal("应能落地")
	}
	if math.Abs(float64(b.Pos.Y)) > 0.05 {
		t.Fatalf("落地高度 = %.3f，期望 ≈0", b.Pos.Y)
	}
}

func TestMoveAndSlideGrid(t *testing.T) {
	g := game.NewCollisionGrid(4, 4, 1)
	g.SetSolid(2, 0, true)
	g.SetSolid(2, 1, true)
	g.SetSolid(2, 2, true)
	g.SetSolid(2, 3, true)
	b := game.NewBody(game.V3(0.5, 0, 0.5))
	b.Gravity = 0
	for i := 0; i < 60; i++ {
		b.Vel = game.V3(6, 0, 0)
		game.MoveAndSlide(b, 1.0/60, nil, g)
	}
	if b.Pos.X > 1.66 {
		t.Fatalf("网格墙未挡住，x=%.3f", b.Pos.X)
	}
}

func TestSweepAABB(t *testing.T) {
	a := game.AABB{Min: game.V3(0, 0, 0), Max: game.V3(1, 1, 1)}
	wall := []game.AABB{{Min: game.V3(3, -1, -1), Max: game.V3(4, 2, 2)}}
	tHit, n, hit := game.SweepAABB(a, game.V3(5, 0, 0), wall)
	if !hit {
		t.Fatal("应检测到碰撞")
	}
	if tHit < 0.35 || tHit > 0.45 {
		t.Fatalf("接触时间 = %.3f，期望 ≈0.4", tHit)
	}
	if n.X != -1 {
		t.Fatalf("法线 = %v，期望 x=-1", n)
	}
	if _, _, hit := game.SweepAABB(a, game.V3(5, 0, 0), nil); hit {
		t.Fatal("无静态体不应碰撞")
	}
}

func TestOverlapAABB(t *testing.T) {
	a := game.AABB{Min: game.V3(0, 0, 0), Max: game.V3(1, 1, 1)}
	if !game.OverlapAABB(a, game.AABB{Min: game.V3(0.5, 0.5, 0.5), Max: game.V3(2, 2, 2)}) {
		t.Fatal("相交盒应返回 true")
	}
	if game.OverlapAABB(a, game.AABB{Min: game.V3(1, 0, 0), Max: game.V3(2, 1, 1)}) {
		t.Fatal("相切不应算相交")
	}
}
