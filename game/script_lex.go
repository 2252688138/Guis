// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"fmt"
	"strconv"
	"strings"
)

// 嵌入式脚本语言的词法分析。
//
// 语言刻意保持小：只有 number / string / bool / nil / array / object 六种值，
// if / while / for / fn / return / break / continue 七种语句，加上 C 风格运算符。
// 目的是让策划改数值、写关卡逻辑、绑事件时不用重新编译 Go，而不是做成通用语言。

// scriptTokKind 词法单元类型。
type scriptTokKind uint8

const (
	stEOF scriptTokKind = iota
	stNum
	stStr
	stIdent
	stKeyword
	stOp
	stPunct
)

// scriptToken 一个词法单元。
type scriptToken struct {
	kind scriptTokKind
	text string
	num  float64
	line int
	col  int
}

func (t scriptToken) String() string {
	switch t.kind {
	case stNum:
		return fmt.Sprintf("数字 %g", t.num)
	case stStr:
		return fmt.Sprintf("字符串 %q", t.text)
	case stEOF:
		return "文件末尾"
	default:
		return fmt.Sprintf("%q", t.text)
	}
}

// scriptKeywords 关键字表。
var scriptKeywords = map[string]bool{
	"let": true, "fn": true, "if": true, "else": true, "while": true,
	"for": true, "in": true, "return": true, "break": true, "continue": true,
	"true": true, "false": true, "nil": true,
}

// scriptLex 把源码切成词法单元。
func scriptLex(src string) ([]scriptToken, error) {
	var out []scriptToken
	line, col := 1, 1
	i := 0
	n := len(src)
	adv := func(k int) {
		for j := 0; j < k && i < n; j++ {
			if src[i] == '\n' {
				line++
				col = 1
			} else {
				col++
			}
			i++
		}
	}
	for i < n {
		c := src[i]
		// 空白
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			adv(1)
			continue
		}
		// 注释
		if c == '/' && i+1 < n {
			if src[i+1] == '/' {
				for i < n && src[i] != '\n' {
					adv(1)
				}
				continue
			}
			if src[i+1] == '*' {
				adv(2)
				for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
					adv(1)
				}
				if i+1 >= n {
					return nil, fmt.Errorf("脚本第 %d 行：注释未闭合", line)
				}
				adv(2)
				continue
			}
		}
		// 数字
		if isDigit(c) || (c == '.' && i+1 < n && isDigit(src[i+1])) {
			start := i
			sl, sc := line, col
			seenDot := false
			for i < n {
				ch := src[i]
				if isDigit(ch) {
					adv(1)
					continue
				}
				if ch == '.' && !seenDot {
					seenDot = true
					adv(1)
					continue
				}
				if (ch == 'e' || ch == 'E') && i+1 < n {
					adv(1)
					if i < n && (src[i] == '+' || src[i] == '-') {
						adv(1)
					}
					continue
				}
				break
			}
			txt := src[start:i]
			v, err := strconv.ParseFloat(txt, 64)
			if err != nil {
				return nil, fmt.Errorf("脚本第 %d 行：非法数字 %q", sl, txt)
			}
			out = append(out, scriptToken{kind: stNum, num: v, text: txt, line: sl, col: sc})
			continue
		}
		// 字符串
		if c == '"' || c == '\'' {
			quote := c
			sl, sc := line, col
			adv(1)
			var sb strings.Builder
			closed := false
			for i < n {
				ch := src[i]
				if ch == '\\' && i+1 < n {
					adv(1)
					switch src[i] {
					case 'n':
						sb.WriteByte('\n')
					case 't':
						sb.WriteByte('\t')
					case 'r':
						sb.WriteByte('\r')
					case '\\':
						sb.WriteByte('\\')
					case '"':
						sb.WriteByte('"')
					case '\'':
						sb.WriteByte('\'')
					default:
						sb.WriteByte(src[i])
					}
					adv(1)
					continue
				}
				if ch == quote {
					adv(1)
					closed = true
					break
				}
				sb.WriteByte(ch)
				adv(1)
			}
			if !closed {
				return nil, fmt.Errorf("脚本第 %d 行：字符串未闭合", sl)
			}
			out = append(out, scriptToken{kind: stStr, text: sb.String(), line: sl, col: sc})
			continue
		}
		// 标识符 / 关键字
		if isIdentStart(c) {
			start := i
			sl, sc := line, col
			for i < n && isIdentPart(src[i]) {
				adv(1)
			}
			txt := src[start:i]
			k := stIdent
			if scriptKeywords[txt] {
				k = stKeyword
			}
			out = append(out, scriptToken{kind: k, text: txt, line: sl, col: sc})
			continue
		}
		// 运算符与标点
		sl, sc := line, col
		two := ""
		if i+1 < n {
			two = src[i : i+2]
		}
		switch two {
		case "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=", "->", "++", "--":
			out = append(out, scriptToken{kind: stOp, text: two, line: sl, col: sc})
			adv(2)
			continue
		}
		switch c {
		case '+', '-', '*', '/', '%', '<', '>', '=', '!':
			out = append(out, scriptToken{kind: stOp, text: string(c), line: sl, col: sc})
			adv(1)
			continue
		case '(', ')', '{', '}', '[', ']', ',', ';', ':', '.':
			out = append(out, scriptToken{kind: stPunct, text: string(c), line: sl, col: sc})
			adv(1)
			continue
		}
		return nil, fmt.Errorf("脚本第 %d 行：无法识别的字符 %q", sl, string(c))
	}
	out = append(out, scriptToken{kind: stEOF, line: line, col: col})
	return out, nil
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isIdentStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isIdentPart(c byte) bool  { return isIdentStart(c) || isDigit(c) }
