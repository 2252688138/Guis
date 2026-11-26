// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

func TestActionsBindAndPress(t *testing.T) {
	m := game.NewActions()
	m.Bind("jump", "space", "w")
	in := game.NewInput()
	if m.Down(in, "jump") {
		t.Fatal("未按键不应触发")
	}
	in.HandleKey("space", true)
	if !m.Down(in, "jump") || !m.Pressed(in, "jump") {
		t.Fatal("空格应触发 jump")
	}
	in.EndFrame()
	if m.Pressed(in, "jump") {
		t.Fatal("EndFrame 后不应再 Pressed")
	}
	if !m.Down(in, "jump") {
		t.Fatal("按住状态应保持 Down")
	}
	in.HandleKey("space", false)
	if !m.Released(in, "jump") || m.Down(in, "jump") {
		t.Fatal("抬起应触发 Released 且 Down=false")
	}
}

func TestActionsCombo(t *testing.T) {
	m := game.NewActions()
	m.Bind("save", "ctrl+s")
	in := game.NewInput()
	in.HandleKey("ctrl", true)
	if m.Pressed(in, "save") || m.Down(in, "save") {
		t.Fatal("只按 ctrl 不应触发 ctrl+s")
	}
	in.HandleKey("s", true)
	if !m.Pressed(in, "save") {
		t.Fatal("ctrl+s 应触发")
	}
}

func TestActionsAxisAndConflicts(t *testing.T) {
	m := game.NewActions()
	m.Bind("left", "a")
	m.Bind("right", "d")
	in := game.NewInput()
	in.HandleKey("d", true)
	if v := m.Axis(in, "left", "right"); v != 1 {
		t.Fatalf("axis = %v，期望 1", v)
	}
	in.HandleKey("a", true)
	if v := m.Axis(in, "left", "right"); v != 0 {
		t.Fatalf("双向同按时 axis = %v，期望 0", v)
	}
	m.Bind("other", "a")
	if c := m.Conflicts(); len(c) != 1 {
		t.Fatalf("应检出 1 个冲突，实际 %v", c)
	}
	if acts := m.Actions(); len(acts) != 3 {
		t.Fatalf("动作数 = %d，期望 3", len(acts))
	}
}

func TestGamepadDeadzoneAndMapping(t *testing.T) {
	g := game.NewGamepad(0)
	g.Connected = true
	game.TestGamepadSetState(g, map[game.PadButton]bool{game.PadA: true}, map[game.PadButton]bool{})
	if !g.Button(game.PadA) || !g.Pressed(game.PadA) {
		t.Fatal("A 键应为按住 + 刚按下")
	}
	game.TestGamepadPrev(g)[game.PadA] = true
	if g.Pressed(game.PadA) {
		t.Fatal("上一帧已按下则不应 Pressed")
	}
	// 死区
	if v := game.TestApplyDeadzone(0.1, 0.15); v != 0 {
		t.Fatalf("死区内应为 0，实际 %v", v)
	}
	if v := game.TestApplyDeadzone(1, 0.15); v < 0.99 {
		t.Fatalf("满幅应为 1，实际 %v", v)
	}
	if v := game.TestApplyDeadzone(-1, 0.15); v > -0.99 {
		t.Fatalf("反向满幅应为 -1，实际 %v", v)
	}
	// 未连接时不 panic
	dis := game.NewGamepad(1)
	dis.Update()
	if dis.Button(game.PadA) || dis.Axis(game.PadAxisLX) != 0 {
		t.Fatal("未连接时应为零值")
	}
}

func TestGamepadButtonBits(t *testing.T) {
	if game.TestPadButtonBits()[game.PadA] != 0x1000 || game.TestPadButtonBits()[game.PadY] != 0x8000 {
		t.Fatal("按钮位映射错误")
	}
}
