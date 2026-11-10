// guis — 原生代码编辑器（七）：诊断与格式化共用的小工具
package ui

// diagSeverityName 诊断等级 → 中文名（LSP 1..4）。
func diagSeverityName(s int) string {
	switch s {
	case 1:
		return "错误"
	case 2:
		return "警告"
	case 3:
		return "信息"
	case 4:
		return "提示"
	}
	return "提示"
}

// codeBracketDelta 一行的括号净值（仅统计未被字符串/注释吃掉的括号）。
func codeBracketDelta(line string) int {
	d := 0
	for _, r := range line {
		switch r {
		case '{', '(', '[':
			d++
		case '}', ')', ']':
			d--
		}
	}
	return d
}

// codeIndentStep 当前语言的一档缩进宽度。
func codeIndentStep(lang string) int {
	if lang == "python" {
		return 4
	}
	return 4
}
