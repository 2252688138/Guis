// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"strings"
	"testing"

	"guis/game"
)

func TestDebugStatsAndHUD(t *testing.T) {
	e := game.NewEngine(320, 180)
	e.Scene.Add(game.NewMeshNode("a", game.NewBox(1, 1, 1), game.DefaultMaterial()))
	d := e.DebugStats()
	if d.W != 320 || d.H != 180 {
		t.Fatalf("分辨率 = %dx%d", d.W, d.H)
	}
	if d.Nodes < 2 {
		t.Fatalf("节点数 = %d，期望 ≥2", d.Nodes)
	}

	hud := game.NewDebugHUD()
	hud.Pad = game.NewGamepad(0)
	hud.Rec = game.NewRecorder()
	s1 := hud.Update(e, 1.0/60)
	if s1.FPS < 55 || s1.FPS > 65 {
		t.Fatalf("FPS = %.1f，期望 ≨60", s1.FPS)
	}
	s2 := hud.Update(e, 1.0/30)
	if s2.FPS <= s1.FPS-25 || s2.FPS > s1.FPS {
		t.Fatalf("FPS 平滑异常: %.1f → %.1f", s1.FPS, s2.FPS)
	}
	lines := s2.Lines()
	if len(lines) < 5 || !strings.Contains(lines[0], "FPS") {
		t.Fatalf("调试行 = %v", lines)
	}
	if !strings.Contains(s2.String(), "手柄") {
		t.Fatalf("摘要缺少手柄信息: %s", s2.String())
	}
}

func TestAABBPoints(t *testing.T) {
	p := game.AABBPoints(game.AABB{Min: game.V3(-1, -2, -3), Max: game.V3(1, 2, 3)})
	if p[0] != game.V3(-1, -2, -3) || p[7] != game.V3(1, 2, 3) {
		t.Fatalf("角点 = %v … %v", p[0], p[7])
	}
}

func TestInputSortedKeys(t *testing.T) {
	in := game.NewInput()
	in.HandleKey("b", true)
	in.HandleKey("a", true)
	ks := in.SortedKeys()
	if len(ks) != 2 || ks[0] != "a" || ks[1] != "b" {
		t.Fatalf("排序键 = %v", ks)
	}
}
