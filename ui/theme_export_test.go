// guis — 主题 token 导出用例

package ui

import (
	"strings"
	"testing"
)

func TestTokenPairsNonEmpty(t *testing.T) {
	pairs := TokenPairs()
	if len(pairs) == 0 {
		t.Fatal("TokenPairs 为空")
	}
	found := false
	for _, p := range pairs {
		if p.Name == "" || p.Value == "" {
			t.Fatalf("token 有空字段：%+v", p)
		}
		if p.Name == "surface" {
			found = true
		}
	}
	if !found {
		t.Fatalf("缺少 surface token：%+v", pairs)
	}
}

func TestTokensCSSShape(t *testing.T) {
	css := TokensCSS()
	if !strings.HasPrefix(css, ":root {") || !strings.HasSuffix(css, "}\n") {
		t.Fatalf("CSS 结构不对：\n%s", css)
	}
	if !strings.Contains(css, "--guis-surface:") {
		t.Fatalf("缺少 --guis-surface：\n%s", css)
	}
	if !strings.Contains(css, "--guis-surface-muted:") {
		t.Fatalf("kebab 化失败：\n%s", css)
	}
}

func TestTokensTailwindShape(t *testing.T) {
	tw := TokensTailwind()
	if !strings.Contains(tw, "colors: {") || !strings.Contains(tw, "guis: {") {
		t.Fatalf("Tailwind 结构不对：\n%s", tw)
	}
	if !strings.Contains(tw, "surfaceMuted:") {
		t.Fatalf("camel 化失败：\n%s", tw)
	}
}

func TestTokensGoShape(t *testing.T) {
	code := TokensGo()
	if !strings.HasPrefix(code, "ui.Tokens{") || !strings.HasSuffix(code, "}\n") {
		t.Fatalf("Go 字面量结构不对：\n%s", code)
	}
	if !strings.Contains(code, `SurfaceMuted:`) {
		t.Fatalf("字段名没有还原成大写开头：\n%s", code)
	}
}

func TestCaseConv(t *testing.T) {
	cases := []struct{ in, kebab, camel string }{
		{"Surface", "surface", "surface"},
		{"SurfaceMuted", "surface-muted", "surfaceMuted"},
		{"OnSurfaceFaint", "on-surface-faint", "onSurfaceFaint"},
		{"FocusRing", "focus-ring", "focusRing"},
	}
	for _, c := range cases {
		if got := kebabCase(c.in); got != c.kebab {
			t.Fatalf("kebabCase(%q) = %q，期望 %q", c.in, got, c.kebab)
		}
		if got := camelCase(c.kebab); got != c.camel {
			t.Fatalf("camelCase(%q) = %q，期望 %q", c.kebab, got, c.camel)
		}
	}
}
