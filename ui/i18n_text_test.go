// guis — 框架级文本翻译钩子的测试
package ui

import (
	"strings"
	"testing"
)

// TestTextTranslatorHook 钩子生效范围：Text / Button 翻译，TextRaw 与 NoTranslate 透传。
func TestTextTranslatorHook(t *testing.T) {
	old := TextTranslator()
	t.Cleanup(func() { SetTextTranslator(old) })
	SetTextTranslator(func(s string) string {
		if s == "设置" {
			return "Settings"
		}
		return ""
	})

	if got := Text("设置").Text; got != "Settings" {
		t.Fatalf("Text 应翻译，实际 %q", got)
	}
	if got := Button("设置", nil).Text; got != "Settings" {
		t.Fatalf("Button 应翻译，实际 %q", got)
	}
	if got := TextRaw("设置").Text; got != "设置" {
		t.Fatalf("TextRaw 不应翻译，实际 %q", got)
	}
	// 词表里没有的文案：钩子返回空串 → 原文。
	if got := Text("未收录文案").Text; got != "未收录文案" {
		t.Fatalf("缺译文应回退原文，实际 %q", got)
	}

	NoTranslate(func() {
		if got := Text("设置").Text; got != "设置" {
			t.Fatalf("NoTranslate 作用域内不应翻译，实际 %q", got)
		}
	})
	if got := Text("设置").Text; got != "Settings" {
		t.Fatalf("作用域结束应恢复翻译，实际 %q", got)
	}
}

// TestMarkdownNotTranslated Markdown 正文是自然内容，不受翻译钩子影响。
func TestMarkdownNotTranslated(t *testing.T) {
	old := TextTranslator()
	t.Cleanup(func() { SetTextTranslator(old) })
	SetTextTranslator(func(s string) string {
		if s == "设置" {
			return "Settings"
		}
		return ""
	})
	MarkdownResetState()
	n := Markdown("i18n-md-test", "设置")
	if n == nil {
		t.Fatal("Markdown 返回 nil")
	}
	if texts := collectTexts(n); !strings.Contains(strings.Join(texts, ""), "设置") {
		t.Fatalf("Markdown 正文应保留原文，实际 %v", texts)
	} else if contains(texts, "Settings") {
		t.Fatalf("Markdown 正文不应被翻译，实际 %v", texts)
	}
}

// collectTexts 收集子树里的所有文本节点内容。
func collectTexts(n *Node) []string {
	var out []string
	var walk func(*Node)
	walk = func(x *Node) {
		if x == nil {
			return
		}
		if x.Kind == KText || x.Kind == KButton {
			out = append(out, x.Text)
		}
		for _, c := range x.Children {
			walk(c)
		}
	}
	walk(n)
	return out
}

// contains 字符串是否在列表里。
func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
