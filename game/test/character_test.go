// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

func TestCharacterJumpAndLand(t *testing.T) {
	c := game.NewCharacter(game.V3(0, 0, 0))
	floor := []game.AABB{{Min: game.V3(-20, -1, -20), Max: game.V3(20, 0, 20)}}
	in := game.NewInput()
	// 先落地
	for i := 0; i < 5; i++ {
		c.Update(in, 1.0/60, floor, nil)
	}
	if !c.Body.Grounded {
		t.Fatal("起始应踩在地面上")
	}
	in.HandleKey("space", true)
	c.Update(in, 1.0/60, floor, nil)
	if c.Body.Vel.Y <= 0 {
		t.Fatalf("跳跃后应有向上速度，实际 %.2f", c.Body.Vel.Y)
	}
	peak := c.Body.Pos.Y
	for i := 0; i < 200; i++ {
		c.Update(in, 1.0/60, floor, nil)
		if c.Body.Pos.Y > peak {
			peak = c.Body.Pos.Y
		}
		if c.Body.Grounded && i > 10 {
			break
		}
	}
	if peak < 0.8 || peak > 1.6 {
		t.Fatalf("跳跃峰值 = %.2f，期望约 1.1", peak)
	}
	if !c.Body.Grounded {
		t.Fatal("最终应落回地面")
	}
}

func TestCharacterMoveYaw(t *testing.T) {
	c := game.NewCharacter(game.V3(0, 0, 0))
	c.Yaw = 0 // 面向 +Z
	in := game.NewInput()
	in.HandleKey("w", true)
	for i := 0; i < 30; i++ {
		c.Update(in, 1.0/60, nil, nil)
	}
	if c.Body.Pos.Z <= 0.5 {
		t.Fatalf("按 W 应向 +Z 移动，实际 z=%.2f", c.Body.Pos.Z)
	}
	if c.Body.Pos.X > 0.01 || c.Body.Pos.X < -0.01 {
		t.Fatalf("按 W 不应横向漂移，x=%.3f", c.Body.Pos.X)
	}
}

func TestCharacterFly(t *testing.T) {
	c := game.NewCharacter(game.V3(0, 0, 0))
	c.Fly = true
	in := game.NewInput()
	in.HandleKey("space", true)
	for i := 0; i < 30; i++ {
		c.Update(in, 1.0/60, nil, nil)
	}
	if c.Body.Pos.Y <= 0.2 {
		t.Fatalf("飞行模式按空格应上升，y=%.2f", c.Body.Pos.Y)
	}
}

func TestCharacterApplyCamera(t *testing.T) {
	c := game.NewCharacter(game.V3(1, 2, 3))
	c.Yaw = 0
	cam := game.NewCamera()
	c.ApplyCamera(cam)
	if cam.Pos != game.V3(1, 2+c.EyeHeight, 3) {
		t.Fatalf("相机位置 = %v", cam.Pos)
	}
	// 朝向 +Z
	if d := cam.Target.Sub(cam.Pos).Normalize(); d.Z < 0.9 {
		t.Fatalf("相机前向 = %v，期望近似 +Z", d)
	}
}
