// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// flingApp 构造一个只有外层滚动容器可命中的场景（指针落在内层之外）。
//
// testScrollTree 的 outer 是 800×400（内容 1000），inner 是 10,10 起的 300×200：
// 用 (700, 350) 命中 outer，避免内层吃掉这次甩动。
func flingApp() *App {
	a := testApp()
	root, _, _ := testScrollTree()
	a.root = root
	return a
}

// TestFlingDeceleratesAndStops 惯性滑动应推进偏移、速度单调衰减，并自行停下。
func TestFlingDeceleratesAndStops(t *testing.T) {
	a := flingApp()
	a.scrolls["outer"] = 0
	a.startFling(700, 350, 1800)
	if a.flingKey != "outer" {
		t.Fatalf("应从指针下的 outer 开始惯性，实际 key=%q", a.flingKey)
	}
	prev := a.flingVel
	for i := 0; i < 600 && a.isFlinging(); i++ {
		a.stepFling()
		if a.flingVel > prev {
			t.Fatalf("速度不该回升：%v → %v", prev, a.flingVel)
		}
		prev = a.flingVel
	}
	if a.isFlinging() {
		t.Fatal("速度衰减到阈值以下后应停止惯性")
	}
	if got := a.scrolls["outer"]; got <= 0 {
		t.Fatalf("惯性应把内容往前推，实际偏移 %v", got)
	}
}

// TestFlingClampsAtBottom 甩到底部时夹住并立刻结束，不会越界。
func TestFlingClampsAtBottom(t *testing.T) {
	a := flingApp()
	a.scrolls["outer"] = 590 // 底部 600 前 10px
	a.startFling(700, 350, 4000)
	for i := 0; i < 400 && a.isFlinging(); i++ {
		a.stepFling()
	}
	if got := a.scrolls["outer"]; got != 600 {
		t.Fatalf("应夹在底部 600，实际 %v", got)
	}
	if a.isFlinging() {
		t.Fatal("到边界后应停止惯性，否则会一直空转请求帧")
	}
}

// TestFlingIgnoredAtEdge 已到底时继续向同方向甩不该启动（没有可滚空间）。
func TestFlingIgnoredAtEdge(t *testing.T) {
	a := flingApp()
	a.scrolls["outer"] = 600
	a.startFling(700, 350, 1200)
	if a.flingKey != "" {
		t.Fatalf("到底后不该启动惯性，实际 key=%q", a.flingKey)
	}
}

// TestFlingBubblesToOuter 内层到顶后向下甩，应由外层接手。
func TestFlingBubblesToOuter(t *testing.T) {
	a := flingApp()
	a.scrolls["inner"] = 0
	a.scrolls["outer"] = 100
	a.startFling(20, 20, -1500) // 负速度：内容继续往下（偏移减小）
	if a.flingKey != "outer" {
		t.Fatalf("内层到顶后应冒泡给外层，实际 key=%q", a.flingKey)
	}
}

// TestFlingStoppedByWheelInput 滚轮/触摸新输入必须打断惯性，避免速度叠加。
func TestFlingStoppedByWheelInput(t *testing.T) {
	a := flingApp()
	a.scrolls["outer"] = 100
	a.startFling(700, 350, 1500)
	if !a.isFlinging() {
		t.Fatal("前置条件：应已进入惯性")
	}
	a.handleWheel(700, 350, -1)
	if a.isFlinging() {
		t.Fatal("滚轮输入后应停止惯性")
	}
}

// TestFlingStoppedByScrollTo 程序化跳转（导航 / 回到顶部）也要打断惯性。
func TestFlingStoppedByScrollTo(t *testing.T) {
	a := flingApp()
	a.scrolls["outer"] = 100
	a.startFling(700, 350, 1500)
	a.stopFling()
	if a.isFlinging() {
		t.Fatal("stopFling 后不应继续惯性")
	}
	if a.flingVel != 0 {
		t.Fatalf("stopFling 应把速度清零，实际 %v", a.flingVel)
	}
}
