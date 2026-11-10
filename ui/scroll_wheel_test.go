// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"testing"
)
// TestWheelInstantDrag 触摸拖动（instant）必须立即到位、不留缓动目标。
//
// 手机滑动「黏手」的根因就是拖动也走缓动：每帧只追 30% 剩余距离，
// 手指移动 100px 内容要好几帧才跟上。拖动应当 1:1 跟手。
func TestWheelInstantDrag(t *testing.T) {
	root := Column(
		Scroll(Box().St(St().H(1000))).St(St().W(100).H(100)).K("page"),
	)
	Layout(root, 100, 100, newTestCache())
	a := &App{
		root:          root,
		scrolls:       map[string]float32{},
		scrollTargets: map[string]float32{},
	}
	a.handleWheelAt(50, 50, -1, true)

	if got := a.scrolls["page"]; math.Abs(float64(got-wheelStepSize)) > 0.5 {
		t.Fatalf("拖动应立即滚动一格 %.0f，实际 %.1f", float32(wheelStepSize), got)
	}
	if len(a.scrollTargets) != 0 {
		t.Fatalf("拖动后不应留下缓动目标: %+v", a.scrollTargets)
	}
}
// TestWheelFirstScroll 滚轮首次滚动：偏移表 scrolls 是懒建的，首次滚轮时还没有
// 该 key 的条目。若 stepScrollAnim 把「条目不存在」当成无效目标丢弃，滚轮就完全
// 不响应，直到拖动一次滚动条（会直接写 scrolls）后才恢复正常。
func TestWheelFirstScroll(t *testing.T) {
	root := Column(
		Scroll(Box().St(St().H(1000))).St(St().W(100).H(100)).K("page"),
	)
	Layout(root, 100, 100, newTestCache())

	a := &App{
		root:          root,
		scrolls:       map[string]float32{},
		scrollTargets: map[string]float32{},
	}
	a.animDt = 1.0 / 60
	a.handleWheel(50, 50, -1) // 向下滚一格

	if _, ok := a.scrollTargets["page"]; !ok {
		t.Fatalf("滚轮未设置滚动目标: %+v", a.scrollTargets)
	}
	if _, ok := a.scrolls["page"]; !ok {
		t.Fatal("setScrollTarget 应惰性建出 scrolls 条目")
	}

	for i := 0; i < 90; i++ {
		a.stepScrollAnim()
	}
	if got := a.scrolls["page"]; got < wheelStepSize-1 {
		t.Fatalf("缓动后偏移 = %.1f, want ≈%.0f", got, float32(wheelStepSize))
	}
}
