package ui

import "testing"

// TestViewportFollowsCurrentApp 视口必须跟随「正在渲染的窗口」：
// 多窗口时若沿用上一个窗口的宽高，主界面会被独立窗口（数字人）的窄尺寸压扁。
func TestViewportFollowsCurrentApp(t *testing.T) {
	old := currentApp.Load()
	defer func() { currentApp.Store(old) }()

	main := &App{winW: 1200, winH: 800}
	dh := &App{winW: 320, winH: 640}

	currentApp.Store(main)
	if got := ViewportW(); got != 1200 {
		t.Fatalf("主窗口视口宽应为 1200，实际 %.0f", got)
	}
	if got := ViewportH(); got != 800 {
		t.Fatalf("主窗口视口高应为 800，实际 %.0f", got)
	}

	currentApp.Store(dh)
	if got := ViewportW(); got != 320 {
		t.Fatalf("数字人窗口视口宽应为 320，实际 %.0f", got)
	}

	// 切回主窗口后必须立即恢复，不能残留小窗口尺寸
	currentApp.Store(main)
	if got := ViewportW(); got != 1200 {
		t.Fatalf("切回主窗口后应为 1200，实际 %.0f（跳窗口污染）", got)
	}
}

// TestEnterViewport 无 currentApp 时回退到 enterViewport 灌入的全局值。
func TestEnterViewport(t *testing.T) {
	old := currentApp.Load()
	defer func() { currentApp.Store(old) }()

	currentApp.Store(nil)
	enterViewport(900, 500)
	if got := ViewportW(); got != 900 {
		t.Fatalf("回退视口宽应为 900，实际 %.0f", got)
	}
	if got := ViewportH(); got != 500 {
		t.Fatalf("回退视口高应为 500，实际 %.0f", got)
	}
	// 非法尺寸不覆盖
	enterViewport(0, 0)
	if got := ViewportW(); got != 900 {
		t.Fatalf("非法尺寸不应覆盖，实际 %.0f", got)
	}
}
