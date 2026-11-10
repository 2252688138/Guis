package ui

import "testing"

// TestCodeEditorTabWidth Tab 按 tabSize 个空格测量（光标 / 选区 / 缩进参考线才能对齐）。
//
// 历史问题：Tab 交给字体测量只有很窄的字形，而缩进参考线按「4 空格宽」定位，
// 于是竖线会穿进代码正文里。
func TestCodeEditorTabWidth(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("tab", "\tx = 1")
	if got := e.xOfCol(0, 1); got != 28 {
		t.Fatalf("Tab 应占 4 个空格宽（28px），实际 %.1f", got)
	}
	if got := e.colAtX("\tx = 1", 20); got != 0 {
		t.Fatalf("Tab 内点击应归到第 0 列，实际 %d", got)
	}
	if got := e.colAtX("\tx = 1", 30); got != 1 {
		t.Fatalf("Tab 之后点击应归到第 1 列，实际 %d", got)
	}
	if got := e.expandTabs("\tx"); got != "    x" {
		t.Fatalf("expandTabs 结果错误：%q", got)
	}
	if got := e.expandTabs("no-tab"); got != "no-tab" {
		t.Fatalf("无 Tab 时不应改写：%q", got)
	}
	// 缩进参考线宽度与 Tab 展开一致
	tabPx := float32(e.tabSize()) * 7
	if got := e.indentWidthPx("\tx", tabPx); got != 28 {
		t.Fatalf("缩进宽度应与 Tab 展开一致：%.1f", got)
	}
}
