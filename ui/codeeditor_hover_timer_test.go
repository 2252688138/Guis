package ui

import (
	"testing"
	"time"
)

// TestCodeEditorMouseHoverTimer 真实定时器链路：鼠标停留 450ms 后，
// time.AfterFunc → pushLSP(lspHoverDue) → drainLSP → hoverDue → 弹出提示。
func TestCodeEditorMouseHoverTimer(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("hover-timer", "package main\n\nfunc foo() int { return 1 }\n")
	e.SetLanguage("go")
	viewSetup(e, 800, 400)

	const line = 2
	x := e.gutterTotal() - e.ScrollX + e.xOfCol(line, 6)
	y := codeTopPad + (float32(line)+0.5)*e.lineH() - e.ScrollY
	e.mouseMoveAt(x, y)
	if e.hoverKeyNow == "" {
		t.Fatal("未识别到悬停符号")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		e.drainLSP() // 真实程序里由每帧的 CodeEditor() 调用
		if e.hoverOpen {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !e.hoverOpen {
		t.Fatal("悬停 450ms 后应弹出提示（定时器 → drainLSP 链路断了）")
	}
}
