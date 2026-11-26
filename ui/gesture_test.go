// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"testing"
	"time"
)

// gestureZone 造一个 200×150 的手势区（居中偏左上，坐标 20/20 必命中）。
func gestureZone(o GestureOptions) *Node {
	n := Column().Tap(func() {}).St(St().W(200).H(150)).Gestures(o)
	return n
}

// TestGestureLongPress 按住不动超时触发一次，抬起后不再触发。
func TestGestureLongPress(t *testing.T) {
	fired := 0
	var gx, gy float32
	a := trapFixture(t, Column(gestureZone(GestureOptions{
		LongPressMs: 300,
		OnLongPress: func(x, y float32) { fired++; gx, gy = x, y },
	})))
	defer ResetGestures()

	base := time.Now()
	gestureClock = func() time.Time { return base }
	defer func() { gestureClock = time.Now }()

	gesturePointer(a, 20, 20, 0, 1)
	gestureTick()
	if fired != 0 {
		t.Fatal("未到长按时长不应触发")
	}
	gestureClock = func() time.Time { return base.Add(320 * time.Millisecond) }
	gestureTick()
	if fired != 1 {
		t.Fatalf("应触发一次长按，实际 %d 次", fired)
	}
	if gx != 20 || gy != 20 {
		t.Fatalf("长按坐标应取按下点，实际 (%v,%v)", gx, gy)
	}
	gestureTick()
	if fired != 1 {
		t.Fatalf("长按应只触发一次，实际 %d 次", fired)
	}
}

// TestGestureSwipeDirection 抬起时按主轴判定方向并给位移。
func TestGestureSwipeDirection(t *testing.T) {
	var dir SwipeDir
	var dx, dy float32
	a := trapFixture(t, Column(gestureZone(GestureOptions{
		SwipeMin: 30,
		OnSwipe:  func(d SwipeDir, x, y float32) { dir, dx, dy = d, x, y },
	})))
	defer ResetGestures()

	gesturePointer(a, 20, 20, 0, 1)
	gesturePointer(a, 120, 25, 1, 1)
	gesturePointer(a, 120, 25, 2, 1)

	if dir != SwipeRight {
		t.Fatalf("应识别为右滑，实际 %v", dir)
	}
	if dx != 100 || dy != 5 {
		t.Fatalf("位移应为 (100,5)，实际 (%v,%v)", dx, dy)
	}
}

// TestGestureSwipeVertical 垂直方向优先判定上下。
func TestGestureSwipeVertical(t *testing.T) {
	var dir SwipeDir
	a := trapFixture(t, Column(gestureZone(GestureOptions{
		OnSwipe: func(d SwipeDir, x, y float32) { dir = d },
	})))
	defer ResetGestures()

	gesturePointer(a, 20, 100, 0, 1)
	gesturePointer(a, 25, 20, 1, 1)
	gesturePointer(a, 25, 20, 2, 1)
	if dir != SwipeUp {
		t.Fatalf("应识别为上滑，实际 %v", dir)
	}
}

// TestGesturePan 拖动过程给相对按下点的位移。
func TestGesturePan(t *testing.T) {
	var pdx, pdy float32
	a := trapFixture(t, Column(gestureZone(GestureOptions{
		OnPan: func(dx, dy float32) { pdx, pdy = dx, dy },
	})))
	defer ResetGestures()

	gesturePointer(a, 20, 20, 0, 1)
	gesturePointer(a, 45, 70, 1, 1)
	if pdx != 25 || pdy != 50 {
		t.Fatalf("拖动位移应为 (25,50)，实际 (%v,%v)", pdx, pdy)
	}
}

// TestGesturePinch 双指拉开时 scale > 1。
func TestGesturePinch(t *testing.T) {
	var scale float32
	a := trapFixture(t, Column(gestureZone(GestureOptions{
		OnPinch: func(s float32) { scale = s },
	})))
	defer ResetGestures()

	gesturePointer(a, 20, 20, 0, 1)
	gesturePointer(a, 100, 20, 0, 2) // 指距 80
	gesturePointer(a, 180, 20, 1, 2) // 指距 160 → scale 2
	if scale < 1.99 || scale > 2.01 {
		t.Fatalf("捏合比例应约为 2，实际 %v", scale)
	}
}

// TestGestureIgnoredWithoutDeclaration 未挂手势的节点不产生任何回调，
// 且识别器整体处于关闭状态（有声明过手势才会打开）。
func TestGestureIgnoredWithoutDeclaration(t *testing.T) {
	ResetGestures()
	fired := false
	plain := Column().Tap(func() {}).St(St().W(200).H(150))
	root := Column(plain)

	// 在另一个节点上声明手势后又移除，确保 gestureUsed 不会被旧状态残留影响
	root.Children = append(root.Children, gestureZone(GestureOptions{OnSwipe: func(SwipeDir, float32, float32) {}}))
	a := trapFixture(t, root)
	ResetGestures() // 模拟「本帧没有节点声明手势」

	gesturePointer(a, 20, 20, 0, 1)
	gesturePointer(a, 120, 20, 1, 1)
	gesturePointer(a, 120, 20, 2, 1)
	gestureTick()
	if fired {
		t.Fatal("未声明手势时不应有回调")
	}
	if SwipeDir(0).String() != "none" || SwipeLeft.String() != "left" {
		t.Fatal("SwipeDir.String 输出不符合约定")
	}
}
