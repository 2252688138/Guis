// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"testing"

	"guis/game"
)

type fakeViewport struct {
	w, h int
	dt   float32
}

func (f *fakeViewport) RenderViewport(w, h int, dt float32) ([]byte, int, int) {
	f.w, f.h, f.dt = w, h, dt
	return make([]byte, 4*4*4), 4, 4
}

type vpEvent struct {
	kind string
	x, y float32
	btn  int
	down bool
}

func TestViewport3DNode(t *testing.T) {
	src := &fakeViewport{}
	var got []vpEvent
	n := Viewport3D(&Viewport3DOpts{
		Src:        src,
		AutoFocus:  true,
		Resolution: [2]int{320, 180},
		OnKey:      func(k string, down bool) { got = append(got, vpEvent{kind: "key:" + k, down: down}) },
		OnMouse: func(x, y float32, btn int, down bool) {
			got = append(got, vpEvent{kind: "mouse", x: x, y: y, btn: btn, down: down})
		},
		OnWheel: func(dy float32) { got = append(got, vpEvent{kind: "wheel", y: dy}) },
	})

	if n.Kind != KViewport3D {
		t.Fatalf("Kind = %v，期望 KViewport3D", n.Kind)
	}
	if n.Viewport == nil {
		t.Fatal("Viewport 状态未初始化")
	}
	if n.Key == "" {
		t.Fatal("Key 应自动生成（焦点与拖动依赖它）")
	}
	if !n.Style.Focusable {
		t.Fatal("应可获取焦点")
	}
	if n.Style.Width != 320 || n.Style.Height != 180 {
		t.Fatalf("Resolution 未生效: %v×%v", n.Style.Width, n.Style.Height)
	}

	// 事件转发
	n.OnHoverAt(7, 8)
	n.OnPress()
	n.OnDrag(11, 22)
	n.OnDragEnd()
	n.OnScroll(3)
	n.OnRightClickAt(5, 6)
	n.OnKey(0x41) // A
	n.OnKeyUp(0x41)

	var kinds []string
	for _, e := range got {
		kinds = append(kinds, e.kind)
	}
	want := []string{"mouse", "mouse", "mouse", "mouse", "wheel", "mouse", "mouse", "key:a", "key:a"}
	if len(kinds) != len(want) {
		t.Fatalf("事件数 = %d (%v)，期望 %d (%v)", len(kinds), kinds, len(want), want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("第 %d 个事件 = %s，期望 %s", i, kinds[i], want[i])
		}
	}
	if got[4].y != 3 {
		t.Fatalf("wheel 值 = %v，期望 3", got[4].y)
	}
	if !got[8].down == false {
		t.Fatal("OnKeyUp 应传递 down=false")
	}
}

func TestViewport3DSourceContract(t *testing.T) {
	// game.Engine 必须满足 ui.ViewportSource（编译期断言）
	eng := game.NewEngine(64, 36)
	var src ViewportSource = eng
	pix, w, h := src.RenderViewport(128, 72, 1.0/60.0)
	if w <= 0 || h <= 0 {
		t.Fatalf("尺寸 = %d×%d", w, h)
	}
	if len(pix) != w*h*4 {
		t.Fatalf("像素长度 = %d，期望 %d", len(pix), w*h*4)
	}
}

func TestGameKeyName(t *testing.T) {
	cases := map[uint32]string{
		0x41: "a", 0x5A: "z", 0x30: "0", 0x39: "9",
		0x26: "arrowup", 0x28: "arrowdown", 0x25: "arrowleft", 0x27: "arrowright",
		0x0D: "enter", 0x20: "space", 0x10: "shift", 0x11: "ctrl", 0x12: "alt",
		0x1B: "escape", 0x70: "f1", 0x7B: "f12", 0x60: "num0", 0x08: "backspace",
	}
	for vk, want := range cases {
		if got := GameKeyName(vk); got != want {
			t.Fatalf("GameKeyName(%#x) = %q，期望 %q", vk, got, want)
		}
	}
}
