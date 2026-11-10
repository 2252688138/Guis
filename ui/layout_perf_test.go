// guis — 稳定字体缓存下的整树排版基准
//
// LayoutForTest 每次调用都新建 fontCache（字体实例 / 字形 / 折行缓存全丢），
// 只能测出「冷启动」成本。真实运行时字体是长驻的，这里复用同一份缓存。

package ui

import (
	"strings"
	"testing"
)

// perfMarkdown 造一篇含标题 / 段落 / 列表 / 表格 / 代码块的 Markdown。
func perfMarkdown(paras int) string {
	var sb strings.Builder
	sb.WriteString("## 排查结论\n\n")
	for i := 0; i < paras; i++ {
		sb.WriteString("这一段用于压测折行与字形测量：包含 **加粗**、`行内代码`、数字 12345 与中英文混排 mixed text。\n\n")
	}
	sb.WriteString("| 列 A | 列 B | 列 C |\n| --- | --- | --- |\n")
	for i := 0; i < 6; i++ {
		sb.WriteString("| 数据 1 | 数据 2 | 数据 3 |\n")
	}
	sb.WriteString("\n- 第一项\n- 第二项\n- 第三项\n\n```go\nfunc demo() error {\n\tfor i := 0; i < 10; i++ {\n\t\tprintln(i)\n\t}\n\treturn nil\n}\n```\n")
	return sb.String()
}

// BenchmarkMarkdownLayoutStable 同一份字体缓存下的整篇 Markdown 排版（稳态）。
func BenchmarkMarkdownLayoutStable(b *testing.B) {
	MarkdownResetState()
	fc := newFontCache("Microsoft YaHei UI", 1)
	root := Column(Markdown("md-perf", perfMarkdown(24),
		MarkdownOptions{Width: 780, BaseSize: 14, LineHeight: 14 * 1.72})).
		St(St().W(820).Padding(6))
	Layout(root, 900, 4000, fc) // 预热：字体 / 字形 / 折行缓存都建好
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Layout(root, 900, 4000, fc)
	}
}

// BenchmarkChatRowsLayoutStable 10 条消息并排的整列排版（稳态）。
func BenchmarkChatRowsLayoutStable(b *testing.B) {
	MarkdownResetState()
	fc := newFontCache("Microsoft YaHei UI", 1)
	doc := perfMarkdown(10)
	rows := make([]*Node, 0, 10)
	for i := 0; i < 10; i++ {
		rows = append(rows, Markdown("md-"+string(rune('a'+i)), doc,
			MarkdownOptions{Width: 780, BaseSize: 14, LineHeight: 14 * 1.72}))
	}
	root := Column(rows...).St(St().W(820).GapOf(4).Padding(6))
	Layout(root, 900, 800, fc)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Layout(root, 900, 800, fc)
	}
}
