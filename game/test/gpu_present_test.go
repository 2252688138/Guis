package game_test

import (
	"testing"

	"guis/game"
)

// TestZeroCopyPresent 零回读呈现通道：硬件后端给出真实显存纹理句柄，
// 软件后端明确报「不支持」，让调用方回退像素路径。
func TestZeroCopyPresent(t *testing.T) {
	g, err := game.NewGPU(game.WithGPUBackend("auto"))
	if err != nil {
		t.Skipf("无可用 GPU 后端: %v", err)
	}
	defer g.Close()

	sc := game.NewScene()
	sc.Root.Add(game.NewMeshNode("cube", game.NewBox(1, 1, 1), game.DefaultMaterial()))
	gs, err := game.NewGPUScene(g, 64, 48)
	if err != nil {
		t.Fatalf("NewGPUScene: %v", err)
	}
	defer gs.Close()

	fr, err := gs.RenderNative(sc, nil)
	if !g.ZeroCopy() {
		if err == nil {
			t.Fatalf("软件后端应返回不支持零回读")
		}
		img, err2 := gs.Render(sc, nil)
		if err2 != nil || img == nil {
			t.Fatalf("软件后端 Render 回退失败: %v", err2)
		}
		return
	}
	if err != nil {
		t.Fatalf("RenderNative: %v", err)
	}
	if fr.TextureID == 0 || fr.W != 64 || fr.H != 48 {
		t.Fatalf("帧句柄非法: %+v", fr)
	}
	if !fr.FlipY {
		t.Fatalf("GL 纹理需 FlipY=true 才能正着显示")
	}
	if gs.Color().NativeID() != fr.TextureID {
		t.Fatalf("句柄应来自 FBO 颜色附件: %d vs %d", gs.Color().NativeID(), fr.TextureID)
	}
}

// TestEngineRenderViewportNative 引擎级零回读：AttachGPU 后可取句柄，卸载后回到软件路径。
func TestEngineRenderViewportNative(t *testing.T) {
	g, err := game.NewGPU(game.WithGPUBackend("auto"))
	if err != nil {
		t.Skipf("无可用 GPU 后端: %v", err)
	}
	defer g.Close()

	eng := game.NewEngine(64, 48)
	eng.Scene.Root.Add(game.NewMeshNode("cube", game.NewBox(1, 1, 1), game.DefaultMaterial()))
	if err := eng.AttachGPU(g); err != nil {
		t.Fatalf("AttachGPU: %v", err)
	}
	if eng.GPUScene() == nil || eng.GPU() == nil {
		t.Fatalf("AttachGPU 后应持有硬件场景")
	}
	tex, w, h, ok := eng.RenderViewportNative(96, 72, 1.0/60)
	if g.ZeroCopy() {
		if !ok || tex == 0 || w <= 0 || h <= 0 {
			t.Fatalf("native 渲染失败 tex=%d %dx%d ok=%v", tex, w, h, ok)
		}
	} else {
		if ok {
			t.Fatalf("软件后端不应返回 native 帧")
		}
		if eng.ZeroCopy() {
			t.Fatalf("软件后端 ZeroCopy 应为 false")
		}
	}

	if err := eng.AttachGPU(nil); err != nil {
		t.Fatalf("卸载 AttachGPU: %v", err)
	}
	if eng.GPU() != nil || eng.GPUScene() != nil || eng.ZeroCopy() {
		t.Fatalf("卸载后硬件字段应清空")
	}
	pix, tw, th := eng.RenderViewport(96, 72, 1.0/60)
	if len(pix) == 0 || tw <= 0 || th <= 0 {
		t.Fatalf("卸载后软件渲染路径应正常")
	}
}
