package ui

import (
	"fmt"
	"strings"
	"testing"
)

// bigDoc 生成 n 行文档（用于大文件基准）。
func bigDoc(n int) string {
	var b strings.Builder
	b.Grow(n * 40)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "func fn%d(a int) int { return a + %d }\n", i, i)
	}
	return b.String()
}

// BenchmarkCodeEditorOpen 打开 10 万行文档（SetText + 首次布局）。
func BenchmarkCodeEditorOpen(b *testing.B) {
	doc := bigDoc(100000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := NewCodeEditorState("bench-open", "")
		e.SetLanguage("go")
		e.SetText(doc)
		e.viewW, e.viewH = 900, 700
		_ = e.sumRows()
	}
}

// BenchmarkCodeEditorScroll 10 万行文档连续滚动（含视觉行缓存推进）。
func BenchmarkCodeEditorScroll(b *testing.B) {
	e := NewCodeEditorState("bench-scroll", bigDoc(100000))
	e.SetLanguage("go")
	e.viewW, e.viewH = 900, 700
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.scrollBy(200)
		first, last := e.rowRange(e.viewH)
		_ = first + last
	}
}

// BenchmarkCodeEditorInput 10 万行文档中间连续输入（触发增量失效）。
func BenchmarkCodeEditorInput(b *testing.B) {
	e := NewCodeEditorState("bench-input", bigDoc(100000))
	e.SetLanguage("go")
	e.viewW, e.viewH = 900, 700
	e.ScrollToLine(50000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.InsertText("x")
	}
}

// BenchmarkCodeEditorFind 10 万行文档查找（纯文本命中集）。
func BenchmarkCodeEditorFind(b *testing.B) {
	e := NewCodeEditorState("bench-find", bigDoc(100000))
	e.SetLanguage("go")
	e.findOpen = true
	e.findText = "fn99999"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.changeSeq++ // 模拟文本变化，强制重算
		_ = e.MatchCount()
	}
}

// BenchmarkCodeEditorLongLine 超长行（2 万字符）高亮判定与列宽计算。
func BenchmarkCodeEditorLongLine(b *testing.B) {
	e := NewCodeEditorState("bench-long", strings.Repeat("var abcdefghij = 123 + ", 900)+"\n")
	e.SetLanguage("go")
	e.viewW, e.viewH = 900, 700
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !e.isLongLine(0) {
			b.Fatal("应为超长行")
		}
		_ = e.colXOf(0)
	}
}
