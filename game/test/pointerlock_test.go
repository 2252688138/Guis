// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

func TestInputPointerLock(t *testing.T) {
	in := game.NewInput()
	in.HandleMouse(10, 10, -1, false)
	in.HandleMouse(30, 30, -1, false)
	if in.MouseDX != 20 || in.MouseDY != 20 {
		t.Fatalf("普通模式差分 = (%v,%v)，期望 (20,20)", in.MouseDX, in.MouseDY)
	}

	in.EndFrame()
	in.LockMouse = true
	// 锁定后绝对坐标不再产生差分（否则光标回位会变成反向位移）
	in.HandleMouse(100, 100, -1, false)
	in.HandleMouse(140, 120, -1, false)
	if in.MouseDX != 0 || in.MouseDY != 0 {
		t.Fatalf("锁定模式不应从绝对坐标算差分，得到 (%v,%v)", in.MouseDX, in.MouseDY)
	}
	if in.MouseX != 140 || in.MouseY != 120 {
		t.Fatalf("锁定模式仍应更新绝对坐标，得到 (%v,%v)", in.MouseX, in.MouseY)
	}

	// 锁定后只认注入的原始位移
	in.AddMouseDelta(5, -3)
	in.AddMouseDelta(-1, 1)
	if in.MouseDX != 4 || in.MouseDY != -2 {
		t.Fatalf("注入位移累加 = (%v,%v)，期望 (4,-2)", in.MouseDX, in.MouseDY)
	}
	in.EndFrame()
	if in.MouseDX != 0 || in.MouseDY != 0 {
		t.Fatalf("EndFrame 后位移应清零，得到 (%v,%v)", in.MouseDX, in.MouseDY)
	}
}

func TestEnginePointerLock(t *testing.T) {
	e := game.NewEngine(320, 200)
	e.SetPointerLock(true)
	e.HandleMouse(10, 10, -1, false)
	e.HandleMouse(50, 50, -1, false)
	if e.Input.MouseDX != 0 {
		t.Fatalf("锁定后绝对坐标不应产生差分，得到 %v", e.Input.MouseDX)
	}
	e.HandleMouseMove(7, 2)
	if e.Input.MouseDX != 7 || e.Input.MouseDY != 2 {
		t.Fatalf("HandleMouseMove = (%v,%v)，期望 (7,2)", e.Input.MouseDX, e.Input.MouseDY)
	}
	e.SetPointerLock(false)
	e.Input.EndFrame()
	e.HandleMouse(10, 10, -1, false)
	e.HandleMouse(12, 15, -1, false)
	if e.Input.MouseDX != 2 || e.Input.MouseDY != 5 {
		t.Fatalf("解除锁定后应恢复绝对坐标差分，得到 (%v,%v)", e.Input.MouseDX, e.Input.MouseDY)
	}
}
