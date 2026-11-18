package ui

import "testing"

// TestTooltipLayerScopedToWindow 悬停提示必须只画在触发它的窗口上。
//
// 回归背景：提示状态曾是包级全局变量，数字人独立窗口悬停后提示被主窗口的
// TooltipLayer 读走，气泡出现在主软件里且坐标错乱。
func TestTooltipLayerScopedToWindow(t *testing.T) {
	prev := currentApp.Load()
	t.Cleanup(func() { currentApp.Store(prev) })

	mainWin, dhWin := &App{}, &App{}

	currentApp.Store(mainWin)
	SetTooltip("数字人：实时语音对话", 100, 200)
	if TooltipLayer() == nil {
		t.Fatal("主窗口自己设的提示应该能画出来")
	}

	// 切到数字人窗口：它没设提示，不该借到主窗口那条
	currentApp.Store(dhWin)
	if n := TooltipLayer(); n != nil {
		t.Fatal("别的窗口的提示串到本窗口了")
	}

	// 数字人窗口设自己的提示后，只它可见
	SetTooltip("重新连接画面", 30, 40)
	if n := TooltipLayer(); n == nil {
		t.Fatal("本窗口的提示没画出来")
	}
	currentApp.Store(mainWin)
	if n := TooltipLayer(); n == nil || n.Text != "数字人：实时语音对话" {
		t.Fatalf("主窗口提示被覆盖了：%+v", n)
	}

	// 清空只影响当前窗口
	SetTooltip("", 0, 0)
	if n := TooltipLayer(); n != nil {
		t.Fatal("清空后本窗口仍有提示")
	}
}
