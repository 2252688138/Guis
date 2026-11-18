// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 主题 token 导出：把当前主题的语义色板输出成 CSS 变量 / Tailwind 配置。
//
// 目的：同一套设计语言同时驱动「GUIS 桌面界面」和「配套网页 / 分享页」，
// 不再出现桌面端改色、网页端忘了同步。字段用反射遍历，Tokens 以后新增字段
// 自动被导出，不需要维护第二份名单。
//
//	css := ui.TokensCSS()        // :root { --guis-surface: …; }
//	tw  := ui.TokensTailwind()   // colors: { guis: { surface: '…' } }

import (
	"fmt"
	"reflect"
	"strings"
)

// TokenPair 一个语义 token 的名字（kebab-case）与颜色值。
type TokenPair struct {
	// Name kebab-case 名字，例如 "surface-muted"。
	Name string
	// Field Tokens 里的原始字段名（Go 字面量导出用），例如 "SurfaceMuted"。
	Field string
	// Value 颜色字符串（#rrggbb / rgba(...) / 主题里写的任意值）。
	Value string
}

// TokenPairs 当前主题的语义 token 列表（按 Tokens 字段顺序，空值跳过）。
func TokenPairs() []TokenPair {
	t := TokensOf()
	v := reflect.ValueOf(t)
	typ := v.Type()
	out := make([]TokenPair, 0, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		if typ.Field(i).Type.Kind() != reflect.String {
			continue
		}
		val := v.Field(i).String()
		if val == "" {
			continue
		}
		out = append(out, TokenPair{Name: kebabCase(typ.Field(i).Name), Field: typ.Field(i).Name, Value: val})
	}
	return out
}

// TokensCSS 导出为 CSS 变量块（`:root { --guis-xxx: …; }`）。
func TokensCSS() string {
	var sb strings.Builder
	sb.WriteString(":root {\n")
	for _, p := range TokenPairs() {
		fmt.Fprintf(&sb, "  --guis-%s: %s;\n", p.Name, p.Value)
	}
	sb.WriteString("}\n")
	return sb.String()
}

// TokensTailwind 导出为 Tailwind v3 `theme.extend` 片段（colors.guis.*）。
//
// 粘到 tailwind.config.js 里即可用 bg-guis-surface / text-guis-on-surface 这类类名。
func TokensTailwind() string {
	var sb strings.Builder
	sb.WriteString("  colors: {\n    guis: {\n")
	for _, p := range TokenPairs() {
		// Tailwind 里用 camelCase 键（surfaceMuted），比 kebab 更贴近 JS 习惯。
		fmt.Fprintf(&sb, "      %s: '%s',\n", camelCase(p.Name), p.Value)
	}
	sb.WriteString("    },\n  },\n")
	return sb.String()
}

// TokensGo 导出为 Go 字面量（用于生成自带主题的代码片段 / 文档）。
func TokensGo() string {
	var sb strings.Builder
	sb.WriteString("ui.Tokens{\n")
	for _, p := range TokenPairs() {
		fmt.Fprintf(&sb, "\t%s: %q,\n", p.Field, p.Value)
	}
	sb.WriteString("}\n")
	return sb.String()
}

// kebabCase SurfaceMuted → surface-muted。
func kebabCase(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				sb.WriteByte('-')
			}
			sb.WriteRune(r - 'A' + 'a')
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// camelCase surface-muted → surfaceMuted。
func camelCase(s string) string {
	parts := strings.Split(s, "-")
	var sb strings.Builder
	sb.WriteString(parts[0])
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		sb.WriteString(strings.ToUpper(p[:1]))
		sb.WriteString(p[1:])
	}
	return sb.String()
}
