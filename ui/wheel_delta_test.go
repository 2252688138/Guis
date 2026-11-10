// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

type wheelCase struct {
	dy   float64
	mode int
	want float32
}

// TestWheelClicksFrom 浏览器滚轮 deltaY → 框架「格数」的折算：一格 100px、
// 反号（浏览器向下为正）、单次夹到 ±3 格。
func TestWheelClicksFrom(t *testing.T) {
	cases := []wheelCase{
		{100, 0, -1},  // 像素：一格向下
		{-100, 0, 1},  // 像素：一格向上
		{30, 0, -0.3}, // 触控板小步长保持连续
		{2000, 0, -3}, // 超大 delta 夹到 3 格
		{-2000, 0, 3}, // 反方向同样夹取
		{3, 1, -1},    // 行：3 行 = 1 格
		{1, 2, -3},    // 页：至少 3 格（夹取）
		{0, 0, 0},     // 无位移
	}
	for _, c := range cases {
		if got := wheelClicksFrom(c.dy, c.mode); got != c.want {
			t.Fatalf("wheelClicksFrom(%v, %d) = %v, want %v", c.dy, c.mode, got, c.want)
		}
	}
}

// TestTouchDragClicks 手机手指拖动 → 「格数」的折算。
//
// 关键约定：
//  1. 1:1 跟手——滑动 96px（wheelStepSize）就是 1 格，不是一格一格跳；
//  2. 手指上推（屏幕坐标 y 变小，位移为负）必须得到负格数，App 才会把内容往下滚；
//  3. 滑动还没超过阈值（点击时的微小抖动）一律返回 0，不进滚动链路。
func TestTouchDragClicks(t *testing.T) {
	cases := []struct{ acc, want float32 }{
		{0, 0},
		{2, 0},       // 阈值内：点击抖动
		{-2, 0},      // 阈值内
		{96, 1},      // 下滑 96px = 1 格
		{-96, -1},    // 上滑 96px = -1 格（内容向下滚）
		{-192, -2},   // 上滑 192px = 2 格
		{-48, -0.5},  // 半格也要发（跟手感）
	}
	for _, c := range cases {
		if got := touchDragClicks(c.acc); got != c.want {
			t.Fatalf("touchDragClicks(%v) = %v, want %v", c.acc, got, c.want)
		}
	}
}
