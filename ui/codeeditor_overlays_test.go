package ui

import (
	"strings"
	"testing"

	"guis/internal/text"
)

// TestClipTextWidth 浮层文字按像素宽截断：中文不会溢出容器。
func TestClipTextWidth(t *testing.T) {
	f := text.NewFont("Microsoft YaHei UI", 12, 400, 1)
	if f == nil {
		t.Skip("测试环境没有可用字体")
	}
	if f.Measure("中") <= f.Measure("a") {
		t.Skip("字体度量不可用（中文应与英文不同宽）")
	}
	long := strings.Repeat("中文测试", 20)
	got := clipTextWidth(f, long, 120)
	if got == "" || !strings.HasSuffix(got, "…") {
		t.Fatalf("超宽文本应截断并补省略号，实际 %q", got)
	}
	if w := f.Measure(got); w > 120 {
		t.Fatalf("截断后仍超宽：%.1f > 120（%q）", w, got)
	}
	if short := "abc"; clipTextWidth(f, short, 500) != short {
		t.Fatal("未超宽的文字应原样返回")
	}
	if clipTextWidth(f, "", 100) != "" {
		t.Fatal("空串应返回空串")
	}
}
