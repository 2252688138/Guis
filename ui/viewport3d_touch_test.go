package ui

import "testing"

// TestViewport3DTouchPhases 视口触摸回调与鼠标模拟。
func TestViewport3DTouchPhases(t *testing.T) {
	var phases []int
	var mouseDown []bool
	n := Viewport3D(&Viewport3DOpts{
		Src: &fakeViewport{},
		OnTouch: func(x, y float32, phase int, id int) {
			phases = append(phases, phase)
		},
		OnMouse: func(x, y float32, button int, down bool) {
			mouseDown = append(mouseDown, down)
		},
	})
	LayoutForTest(n, 320, 200, "", 1)
	vp := n.Viewport
	vp.touch(10, 10, 0, 0)
	vp.touch(20, 12, 1, 0)
	vp.touch(20, 12, 2, 0)
	if len(phases) != 3 || phases[0] != 0 || phases[1] != 1 || phases[2] != 2 {
		t.Fatalf("触摸阶段 = %v，期望 [0 1 2]", phases)
	}
	if len(mouseDown) != 3 || !mouseDown[0] || !mouseDown[1] || mouseDown[2] {
		t.Fatalf("鼠标模拟序列 = %v，期望 [true true false]", mouseDown)
	}
}

// TestDispatchTouchHitsViewport 触摸事件应由命中测试落到视口节点。
func TestDispatchTouchHitsViewport(t *testing.T) {
	got := 0
	n := Viewport3D(&Viewport3DOpts{
		Src:     &fakeViewport{},
		OnTouch: func(x, y float32, phase int, id int) { got++ },
	})
	root := Column(Text("头部"), n.Grow(1))
	LayoutForTest(root, 400, 300, "", 1)
	a := &App{root: root}
	if !a.dispatchTouch(20, 100, 0, 0) {
		t.Fatal("应命中视口")
	}
	if got != 1 {
		t.Fatalf("回调次数 = %d，期望 1", got)
	}
	if a.dispatchTouch(20, 1, 0, 0) {
		t.Fatal("点在文本上不应命中视口")
	}
}
