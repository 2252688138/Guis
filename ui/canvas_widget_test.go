// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// Canvas 光栅与缓存测试（不需要窗口：只走 CPU 光栅路径）。

import "testing"

// pxAt 取逻辑坐标像素的 RGBA。
func pxAt(pix []byte, w, x, y int) (r, g, b, a byte) {
	i := (y*w + x) * 4
	if i+3 >= len(pix) {
		return 0, 0, 0, 0
	}
	return pix[i], pix[i+1], pix[i+2], pix[i+3]
}

// 矩形填充：区域内是红色，区域外透明。
func TestCanvasRasterRect(t *testing.T) {
	st := NewCanvasState("t")
	n := Canvas(st, []CanvasShape{
		{Kind: "rect", X: 10, Y: 10, W: 20, H: 10, Fill: "#ff0000"},
	}, CanvasOptions{W: 64, H: 32})
	if n.Kind != KCanvas || n.Canvas != st {
		t.Fatalf("Canvas 节点不对: kind=%v canvas=%v", n.Kind, n.Canvas)
	}
	if w, h := st.Size(); w != 64 || h != 32 {
		t.Fatalf("尺寸 %dx%d", w, h)
	}
	pix := st.Raster()
	if len(pix) != 64*32*4 {
		t.Fatalf("像素长度 %d", len(pix))
	}
	r, _, _, a := pxAt(pix, 64, 20, 15)
	if a == 0 || r < 200 {
		t.Fatalf("矩形内应为不透明红色，得到 r=%d a=%d", r, a)
	}
	if _, _, _, a2 := pxAt(pix, 64, 2, 2); a2 != 0 {
		t.Fatalf("矩形外应为透明，得到 a=%d", a2)
	}
}

// 各形状都能画出不透明像素，未知 kind 被忽略。
func TestCanvasRasterShapes(t *testing.T) {
	cases := []struct {
		name  string
		shape CanvasShape
		probe [2]int
	}{
		{"line", CanvasShape{Kind: "line", X: 4, Y: 16, W: 60, H: 16, Stroke: "#ffffff", StrokeW: 4}, [2]int{30, 16}},
		{"circle", CanvasShape{Kind: "circle", CX: 32, CY: 16, Radius: 12, Fill: "#00ff00"}, [2]int{32, 16}},
		{"ring", CanvasShape{Kind: "ring", CX: 32, CY: 16, Radius: 12, Stroke: "#00ffff", StrokeW: 4}, [2]int{32, 5}},
		{"polygon", CanvasShape{Kind: "polygon", Points: []CanvasPoint{{10, 4}, {54, 4}, {32, 28}}, Fill: "#ffff00"}, [2]int{32, 12}},
		{"roundrect", CanvasShape{Kind: "roundrect", X: 8, Y: 6, W: 48, H: 20, R: 8, Fill: "#ff00ff"}, [2]int{32, 16}},
		{"arc", CanvasShape{Kind: "arc", CX: 32, CY: 16, Radius: 10, A0: 0, A1: 180, Stroke: "#ffffff", StrokeW: 3}, [2]int{42, 16}},
	}
	for _, c := range cases {
		st := NewCanvasState(c.name)
		Canvas(st, []CanvasShape{c.shape}, CanvasOptions{W: 64, H: 32})
		pix := st.Raster()
		_, _, _, a := pxAt(pix, 64, c.probe[0], c.probe[1])
		if a == 0 {
			t.Fatalf("%s 没画出像素（probe=%v）", c.name, c.probe)
		}
	}

	// 未知 kind 不影响其它形状
	st := NewCanvasState("u")
	Canvas(st, []CanvasShape{
		{Kind: "nope", X: 0, Y: 0, W: 10, H: 10, Fill: "#ff0000"},
		{Kind: "rect", X: 20, Y: 10, W: 10, H: 10, Fill: "#ff0000"},
	}, CanvasOptions{W: 64, H: 32})
	pix := st.Raster()
	if _, _, _, a := pxAt(pix, 64, 25, 15); a == 0 {
		t.Fatal("未知 kind 影响了后续形状")
	}
}

// 颜色与透明度：hex alpha 与 opacity 都生效。
func TestCanvasOpacity(t *testing.T) {
	st := NewCanvasState("o")
	Canvas(st, []CanvasShape{
		{Kind: "rect", X: 0, Y: 0, W: 32, H: 32, Fill: "#00ff0080"},
	}, CanvasOptions{W: 32, H: 32})
	pix := st.Raster()
	_, _, _, a := pxAt(pix, 32, 16, 16)
	if a == 0 || a == 255 {
		t.Fatalf("半透明填充未生效，a=%d", a)
	}

	st2 := NewCanvasState("o2")
	Canvas(st2, []CanvasShape{
		{Kind: "rect", X: 0, Y: 0, W: 32, H: 32, Fill: "#00ff00", Opacity: 0.5},
	}, CanvasOptions{W: 32, H: 32})
	_, _, _, a2 := pxAt(st2.Raster(), 32, 16, 16)
	if a2 == 0 || a2 > 200 {
		t.Fatalf("opacity 未生效，a=%d", a2)
	}

	// 底色
	st3 := NewCanvasState("bg")
	Canvas(st3, nil, CanvasOptions{W: 16, H: 16, BG: "#102030"})
	r, g, b, a3 := pxAt(st3.Raster(), 16, 8, 8)
	if a3 == 0 || r > 40 || g > 60 || b < 30 {
		t.Fatalf("底色未生效: %d,%d,%d,%d", r, g, b, a3)
	}
}

// 指纹缓存：相同形状不重复光栅，变了才重光栅。
func TestCanvasCache(t *testing.T) {
	st := NewCanvasState("c")
	Canvas(st, []CanvasShape{{Kind: "rect", X: 1, Y: 1, W: 8, H: 8, Fill: "#ff0000"}},
		CanvasOptions{W: 32, H: 32})
	if !st.needRaster() {
		t.Fatal("首次应需要光栅")
	}
	st.Raster()
	if st.needRaster() {
		t.Fatal("形状未变不应重复光栅")
	}
	first := st.Rasters()

	// 同一份形状再走一遍 Canvas()：指纹相同，不该重绘
	Canvas(st, []CanvasShape{{Kind: "rect", X: 1, Y: 1, W: 8, H: 8, Fill: "#ff0000"}},
		CanvasOptions{W: 32, H: 32})
	if st.needRaster() {
		t.Fatal("相同形状（重新声明）不应重复光栅")
	}

	// 改形状 → 要重绘
	Canvas(st, []CanvasShape{{Kind: "rect", X: 2, Y: 1, W: 8, H: 8, Fill: "#ff0000"}},
		CanvasOptions{W: 32, H: 32})
	st.Raster()
	if st.Rasters() != first+1 {
		t.Fatalf("光栅次数 %d，期望 %d", st.Rasters(), first+1)
	}
	// 同一内容再画一帧：不应重复光栅
	st.Raster()
	if st.Rasters() != first+1 {
		t.Fatalf("相同内容重复光栅了: %d", st.Rasters())
	}

	// Invalidate 强制重绘
	st.Invalidate()
	st.Raster()
	if st.Rasters() != first+2 {
		t.Fatalf("Invalidate 后未重光栅: %d", st.Rasters())
	}
}
