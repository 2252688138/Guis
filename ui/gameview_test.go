// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"testing"
	"time"

	"guis/game"
)

// 构建一个带 3D 场景的引擎。
func newTestGameEngine(w, h int) *game.Engine {
	eng := game.NewEngine(w, h)
	eng.Scene.Sky = game.GradientSky(game.RGB(40, 80, 200), game.RGB(180, 210, 240), game.RGB(20, 20, 30))
	eng.Scene.AddSun(game.V3(-0.4, -1, -0.6), 1)
	cube := game.NewMeshNode("cube", game.NewBox(1, 1, 1), game.DefaultMaterial())
	cube.Pos = game.V3(0, 0.5, 0)
	eng.Scene.Add(game.NewMeshNode("ground", game.NewPlane(10, 10, 1), game.DefaultMaterial()), cube)
	eng.Camera.Pos = game.V3(0, 2, 5)
	eng.Camera.Target = game.V3(0, 0.5, 0)
	return eng
}

func TestGameViewRenderViewport(t *testing.T) {
	eng := newTestGameEngine(160, 90)
	st := gameViewStateFor("gv-test", &GameViewOpts{Engine: eng, ShowStats: true})
	defer ResetGameView("gv-test")

	pix, w, h := st.RenderViewport(160, 90, 1.0/60)
	if len(pix) < w*h*4 || w <= 0 || h <= 0 {
		t.Fatalf("RenderViewport 返回异常：len=%d w=%d h=%d", len(pix), w, h)
	}
	// ShowStats 会在左上角写像素（与纯天空背景不同）
	if pix[0] == 0 && pix[1] == 0 && pix[2] == 0 {
		t.Fatalf("左上角像素应含 FPS 文本")
	}

	// 未设置 Engine 时安全返回
	empty := gameViewStateFor("gv-empty", &GameViewOpts{})
	if p, tw, th := empty.RenderViewport(32, 32, 0); p != nil || tw != 0 || th != 0 {
		t.Fatalf("无引擎时不应渲染")
	}
	ResetGameView("gv-empty")
}

func TestGameViewIdlePause(t *testing.T) {
	eng := newTestGameEngine(80, 45)
	st := gameViewStateFor("gv-idle", &GameViewOpts{Engine: eng, IdleTimeout: 0.01})
	defer ResetGameView("gv-idle")

	st.markInput()
	st.RenderViewport(80, 45, 1.0/60)
	if eng.Paused {
		t.Fatalf("刚有输入时不应暂停")
	}
	time.Sleep(25 * time.Millisecond)
	st.RenderViewport(80, 45, 1.0/60)
	if !eng.Paused {
		t.Fatalf("超过 IdleTimeout 应自动暂停")
	}
}

func TestGameViewNode(t *testing.T) {
	eng := newTestGameEngine(120, 68)
	n := GameView(&GameViewOpts{Engine: eng, Key: "gv-node", Resolution: [2]int{120, 68}, ShowFPS: true})
	if n == nil {
		t.Fatalf("GameView 应返回节点")
	}
	if GameView(nil) == nil {
		t.Fatalf("空参数也应返回可用节点")
	}
	ResetGameView("gv-node")
}
