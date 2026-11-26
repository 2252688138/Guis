// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

func TestLockedViewportLookup(t *testing.T) {
	if lockedViewport(nil) != nil {
		t.Fatal("nil 树应返回 nil")
	}
	plain := &Node{Viewport: &Viewport3DState{opts: &Viewport3DOpts{}}, Style: Style{Opacity: 1}}
	if lockedViewport(plain) != nil {
		t.Fatal("未开启 MouseLock 的视口不应被选中")
	}

	locked := &Node{Viewport: &Viewport3DState{opts: &Viewport3DOpts{MouseLock: true}}, Style: Style{Opacity: 1}}
	root := Row(plain, locked)
	if v := lockedViewport(root); v != locked.Viewport {
		t.Fatalf("应找到开启锁定的视口，得到 %v", v)
	}

	locked.Disabled = true
	if v := lockedViewport(root); v != nil {
		t.Fatal("Disabled 视口不应被选中")
	}
}

func TestViewportMouseDeltaForwards(t *testing.T) {
	src := &fakeViewportSrc{}
	v := &Viewport3DState{opts: &Viewport3DOpts{Src: src, MouseLock: true}, w: 640, h: 360}
	v.MouseDelta(3, -4)
	if src.dx != 3 || src.dy != -4 {
		t.Fatalf("位移转发 = (%v,%v)，期望 (3,-4)", src.dx, src.dy)
	}
}

type fakeViewportSrc struct{ dx, dy float32 }

func (f *fakeViewportSrc) RenderViewport(w, h int, dt float32) ([]byte, int, int) {
	return make([]byte, w*h*4), w, h
}

func (f *fakeViewportSrc) HandleMouseMove(dx, dy float32) { f.dx, f.dy = dx, dy }
