// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// Canvas 从脚本构树 + 状态复用。

import (
	"testing"

	"guis/ui"
	"guis/ui/uidsl"
)

func TestCanvasFromScript(t *testing.T) {
	uidsl.ResetExtraStates()
	e := newEngine(t, Options{})
	if err := e.DoString(`
function view()
  return ui.canvas{ key = "c", w = 200, h = 80, bg = "#0b1020", shapes = {
    { kind = "rect", x = 10, y = 10, w = 60, h = 30, fill = "#4c8dff" },
    { kind = "circle", cx = 140, cy = 40, radius = 20, fill = "#ff0000" },
    { kind = "line", x = 10, y = 70, w = 190, h = 70, stroke = "#ffffff88", strokeW = 2 },
  } }
end`); err != nil {
		t.Fatal(err)
	}
	root := e.Root()
	if err := e.LastError(); err != nil {
		t.Fatalf("canvas 构树失败: %v", err)
	}
	n := findKind(root, ui.KCanvas)
	if n == nil || n.Canvas == nil {
		t.Fatal("未构出 KCanvas 节点")
	}
	if w, h := n.Canvas.Size(); w != 200 || h != 80 {
		t.Fatalf("画布尺寸 %dx%d", w, h)
	}
	first := n.Canvas
	if pix := first.Raster(); len(pix) != 200*80*4 {
		t.Fatalf("光栅像素长度 %d", len(pix))
	}

	// 同 key 复用同一状态
	n2 := findKind(e.Root(), ui.KCanvas)
	if n2 == nil || n2.Canvas != first {
		t.Fatal("同 key 应复用同一个 CanvasState")
	}

	// ui.set 改 shapes → 内容变化（光栅次数增加）
	before := first.Rasters()
	if err := e.DoString(`ui.set("c", "shapes", { { kind = "rect", x = 0, y = 0, w = 20, h = 20, fill = "#00ff00" } })`); err != nil {
		t.Fatal(err)
	}
	n3 := findKind(e.Root(), ui.KCanvas)
	if n3 == nil {
		t.Fatal("改 shapes 后丢了 canvas 节点")
	}
	n3.Canvas.Raster()
	if n3.Canvas.Rasters() <= before {
		t.Fatalf("shapes 变化后没有重新光栅: %d -> %d", before, n3.Canvas.Rasters())
	}

	// 热重载（ResetExtraStates）后缓存清空
	uidsl.ResetExtraStates()
	e.dirty.Store(true) // 强制重建（真实场景里由 ui.set / 热重载置脏）
	n4 := findKind(e.Root(), ui.KCanvas)
	if n4 == nil || n4.Canvas == first {
		t.Fatal("ResetExtraStates 后应重建 CanvasState")
	}
}

// 描述文件形态（uidsl 直接 Build）也能构出画布。
func TestCanvasFromDecl(t *testing.T) {
	uidsl.ResetExtraStates()
	defer uidsl.ResetExtraStates()
	out := uidsl.BuildWith(uidsl.Node{
		Type:  "canvas",
		Key:   "decl",
		W:     120,
		H:     60,
		BG:    "#102030",
		Shapes: []uidsl.CanvasShapeDef{
			{Kind: "rect", X: 4, Y: 4, W: 20, H: 20, Fill: "#ffffff"},
		},
	}, uidsl.Handlers{})
	if out == nil || out.Kind != ui.KCanvas || out.Canvas == nil {
		t.Fatalf("声明式 canvas 构造失败: %+v", out)
	}
	// 降级信息不应产生
	if msgs := uidsl.DrainDegrades(); len(msgs) > 0 {
		t.Fatalf("canvas 构树产生了降级: %v", msgs)
	}
}
