// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"testing"
)

func TestVirtualStickAxis(t *testing.T) {
	ResetVirtualStick("t")
	var axis [2]float32
	n := VirtualStick(&VirtualStickOpts{
		Key: "t", Size: 100, Deadzone: 0.2,
		OnAxis: func(x, y float32) { axis[0], axis[1] = x, y },
	})
	if n == nil {
		t.Fatal("VirtualStick 返回 nil")
	}

	// 按下在中心 → 仍为 0
	n.OnTouch(50, 50, 0, 7)
	if x, y := VirtualStickAxis("t"); x != 0 || y != 0 {
		t.Fatalf("中心按下轴 = (%v,%v)，期望 0,0", x, y)
	}

	// 向右拖到边缘 → x 接近 1
	n.OnTouch(100, 50, 1, 7)
	x, y := VirtualStickAxis("t")
	if x < 0.99 || math.Abs(float64(y)) > 1e-3 {
		t.Fatalf("右拖满轴 = (%v,%v)，期望 (≈1,0)", x, y)
	}
	if axis[0] != x || axis[1] != y {
		t.Fatalf("OnAxis 回调值 (%v,%v) 与读取值 (%v,%v) 不一致", axis[0], axis[1], x, y)
	}

	// 另一根手指移动不应改变已有轴
	n.OnTouch(0, 0, 1, 9)
	if x2, _ := VirtualStickAxis("t"); x2 != x {
		t.Fatalf("第二根手指干扰了轴值: %v -> %v", x, x2)
	}

	// 死区内的小幅拖动仍为 0
	ResetVirtualStick("t")
	n = VirtualStick(&VirtualStickOpts{Key: "t", Size: 100, Deadzone: 0.5})
	n.OnTouch(50, 50, 0, 1)
	n.OnTouch(60, 50, 1, 1) // 偏移 10% < 50% 死区
	if x, y := VirtualStickAxis("t"); x != 0 || y != 0 {
		t.Fatalf("死区内轴 = (%v,%v)，期望 0,0", x, y)
	}

	// 抬手归零
	n.OnTouch(50, 50, 2, 1)
	if x, y := VirtualStickAxis("t"); x != 0 || y != 0 {
		t.Fatalf("抬手后轴 = (%v,%v)，期望 0,0", x, y)
	}
}

func TestVirtualStickFollow(t *testing.T) {
	ResetVirtualStick("f")
	n := VirtualStick(&VirtualStickOpts{Key: "f", Size: 100, Deadzone: 0, Follow: true})
	// Follow：按下位置即中心，向外拖 50px 满格
	n.OnTouch(20, 20, 0, 3)
	n.OnTouch(70, 20, 1, 3)
	x, y := VirtualStickAxis("f")
	if x < 0.99 || math.Abs(float64(y)) > 1e-3 {
		t.Fatalf("Follow 模式轴 = (%v,%v)，期望 (≈1,0)", x, y)
	}
}

func TestVirtualButton(t *testing.T) {
	ResetVirtualStick("b")
	var events []bool
	n := VirtualButton(&VirtualButtonOpts{
		Key: "b", Size: 60, Label: "跳",
		OnPress: func(down bool) { events = append(events, down) },
	})
	n.OnTouch(30, 30, 0, 1)
	if !VirtualButtonDown("b") {
		t.Fatal("按下后应为 true")
	}
	n.OnTouch(30, 30, 1, 1) // 移动不重复触发
	n.OnTouch(30, 30, 2, 1)
	if VirtualButtonDown("b") {
		t.Fatal("抬手后应为 false")
	}
	if len(events) != 2 || !events[0] || events[1] {
		t.Fatalf("OnPress 事件 = %v，期望 [true false]", events)
	}
}
