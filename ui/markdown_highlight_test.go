// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"strings"
	"testing"
)

// TestHighlightCJKCommentNoPanic 回归：注释里出现中文时不能让高亮器越界崩溃。
//
// 历史 bug：块注释用 strings.Index 求 "*/" / "-->" 的位置（字节偏移），
// 却直接当 []rune 下标去切，注释含多字节字符时 high 边界大于容量，
// 触发 "slice bounds out of range" 并把整个窗口打挂（API 文档页首屏即复现）。
func TestHighlightCJKCommentNoPanic(t *testing.T) {
	cases := []struct{ lang, line string }{
		{"go", `/* 中文注释 */ var a = 1`},
		{"go", `x := 1 /* 收尾注释，含中文 */`},
		{"go", `// 行注释也含中文`},
		{"js", "/* 🎉 emoji 与中英混排 comment */ fn()"},
		{"css", `/* 主题色：主色 */ .btn { color: #fff }`},
		{"html", `<!-- 文档说明注释 --> <div>中文内容</div>`},
		{"html", `<!-- 无结束符的中文注释`},
		{"go", `/* 未闭合的中文注释`},
		{"go", ``},
	}
	for _, c := range cases {
		lines := mdHighlight(c.lang, c.line)
		if len(lines) != 1 {
			t.Fatalf("lang=%s 行数=%d，期望 1", c.lang, len(lines))
		}
		var sb strings.Builder
		for _, s := range lines[0] {
			sb.WriteString(s.Text)
		}
		if got := sb.String(); got != c.line {
			t.Fatalf("lang=%s 高亮丢字: got=%q want=%q", c.lang, got, c.line)
		}
	}
}
