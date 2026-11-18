package ui

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// waitDrain 反复 drainLSP 直到 cond 成立或超时（模拟每帧应用后台结果）。
func waitDrain(e *CodeEditorState, cond func() bool) bool {
	for i := 0; i < 200; i++ {
		if cond() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
		e.drainLSP()
	}
	return cond()
}

// TestCodeTranslateHover 悬停：先按原文显示，译文回来后重排成中文。
func TestCodeTranslateHover(t *testing.T) {
	e := NewCodeEditorState("tr-hover", "")
	e.OnTranslate = func(text string, done func(string)) {
		go done("包 main 实现入口。")
	}
	e.setHoverMarkdown("Package main implements the entry.")
	if len(e.hoverLines) == 0 || !strings.Contains(strings.Join(e.hoverLines, "\n"), "Package main") {
		t.Fatalf("译文回来前应显示原文，实际 %v", e.hoverLines)
	}
	ok := waitDrain(e, func() bool {
		return strings.Contains(strings.Join(e.hoverLines, "\n"), "包 main")
	})
	if !ok {
		t.Fatalf("悬停译文未生效，实际 %v", e.hoverLines)
	}
	if v, cached := e.translateCached("Package main implements the entry."); !cached || v != "包 main 实现入口。" {
		t.Fatalf("译文应进缓存，实际 %q %v", v, cached)
	}
}

// TestCodeTranslateDiags 诊断：原文先显示，译文落地后换中文，且同一原文只请求一次。
func TestCodeTranslateDiags(t *testing.T) {
	e := NewCodeEditorState("tr-diags", "package main\n")
	var mu sync.Mutex
	calls := 0
	e.OnTranslate = func(text string, done func(string)) {
		mu.Lock()
		calls++
		mu.Unlock()
		go done("未定义的标识符：foo")
	}
	ds := []CodeDiagnostic{{Line: 0, Col: 0, Severity: 1, Message: "undefined: foo"}}
	first := e.translateDiags(ds)
	if first[0].Message != "undefined: foo" {
		t.Fatalf("首帧应显示原文，实际 %q", first[0].Message)
	}
	ok := waitDrain(e, func() bool {
		return len(e.lspDiags) == 1 && e.lspDiags[0].Message == "未定义的标识符：foo"
	})
	if !ok {
		t.Fatalf("诊断译文未生效，实际 %+v", e.lspDiags)
	}
	// 再应用同一批原文：命中缓存，不再发起翻译。
	again := e.translateDiags(e.diagRaw)
	if again[0].Message != "未定义的标识符：foo" {
		t.Fatalf("缓存命中应直接给译文，实际 %q", again[0].Message)
	}
	mu.Lock()
	n := calls
	mu.Unlock()
	if n != 1 {
		t.Fatalf("同一原文只应请求一次翻译，实际 %d 次", n)
	}
}

// TestCodeTranslateDisabled 未设置 OnTranslate 时原样返回，不改变任何显示。
func TestCodeTranslateDisabled(t *testing.T) {
	e := NewCodeEditorState("tr-off", "")
	ds := []CodeDiagnostic{{Message: "cannot use x (variable of type int) as string"}}
	got := e.translateDiags(ds)
	if got[0].Message != ds[0].Message {
		t.Fatalf("未设置翻译钩子时应原样返回，实际 %q", got[0].Message)
	}
	if e.hoverLines != nil && len(e.hoverLines) > 0 {
		t.Fatal("未设置翻译钩子时不应产生悬停内容")
	}
}
