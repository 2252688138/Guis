// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package uidsl

// 紧凑 DSL：缩进决定嵌套，一行一个节点，属性写成 key=value。
//
//	# 井号开头整行是注释
//	col gap=12 pad=16 bg=#0f1626
//	  text "界面标题" size=22 weight=700
//	  row gap=8
//	    button "确定" id=ok onClick=confirm
//	    button "取消" id=cancel
//	  spacer
//	  scroll grow=1
//	    text "改这个文件，界面立刻变"
//
// 规则：
//   - 缩进（空格或 tab）比上一级深即为它的子节点；对齐即同级；
//   - 行首第一个词是类型，带引号的部分是文本，其余 key=value 是属性；
//   - 属性名大小写不敏感（onClick 与 onclick 等价），写错会直接报错并给出行号。

import (
	"fmt"
	"strconv"
	"strings"
)

type rawNode struct {
	n    Node
	kids []*rawNode
}

func parseDSL(src string) (Node, error) {
	src = strings.TrimPrefix(src, "\ufeff")
	root := &rawNode{}
	type frame struct {
		indent int
		n      *rawNode
	}
	stack := []frame{{indent: -1, n: root}}
	for i, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		ind := indentWidth(line)
		for len(stack) > 1 && ind <= stack[len(stack)-1].indent {
			stack = stack[:len(stack)-1]
		}
		n, err := parseLine(trimmed)
		if err != nil {
			return Node{}, fmt.Errorf("第 %d 行: %w", i+1, err)
		}
		parent := stack[len(stack)-1].n
		parent.kids = append(parent.kids, n)
		stack = append(stack, frame{indent: ind, n: n})
	}
	if len(root.kids) == 0 {
		return Node{}, fmt.Errorf("DSL 里没有任何节点")
	}
	if len(root.kids) > 1 {
		// 多个根节点：自动包一层纵向容器，避免用户必须记住「只能有一个根」。
		wrap := &rawNode{n: Node{Type: "col"}}
		wrap.kids = root.kids
		return flatten(wrap), nil
	}
	return flatten(root.kids[0]), nil
}

func flatten(r *rawNode) Node {
	n := r.n
	if len(r.kids) > 0 {
		n.Children = make([]Node, 0, len(r.kids))
		for _, k := range r.kids {
			n.Children = append(n.Children, flatten(k))
		}
	}
	return n
}

// indentWidth 缩进宽度：空格 1 列，tab 记 4 列（混用时仍能排出层级）。
func indentWidth(line string) int {
	w := 0
	for _, r := range line {
		switch r {
		case ' ':
			w++
		case '\t':
			w += 4
		default:
			return w
		}
	}
	return w
}

// parseLine 解析一行：类型 + 可选文本 + 属性。
func parseLine(line string) (*rawNode, error) {
	typ, rest := cutToken(line)
	if typ == "" {
		return nil, fmt.Errorf("缺少节点类型")
	}
	out := &rawNode{n: Node{Type: typ}}
	for {
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" {
			break
		}
		if rest[0] == '"' {
			s, tail, err := readQuoted(rest[1:])
			if err != nil {
				return nil, err
			}
			looseText(&out.n, s)
			rest = tail
			continue
		}
		tok, tail := cutToken(rest)
		rest = tail
		if tok == "" {
			continue
		}
		if !strings.Contains(tok, "=") {
			// 裸词当作文本补充（例如 text 标题 / icon star），不报错，方便临时写。
			looseText(&out.n, tok)
			continue
		}
		key, val := cutOnce(tok, '=')
		// 值带引号时可能含空格：从原始 rest 里继续读。
		if strings.HasPrefix(val, "\"") {
			joined := val + rest
			s, t, err := readQuoted(strings.TrimPrefix(joined, "\""))
			if err != nil {
				return nil, err
			}
			val = "\"" + s
			rest = t
		}
		if err := setAttr(&out.n, key, strings.Trim(val, "\"")); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// cutToken 取第一个空白分隔的词。
func cutToken(s string) (string, string) {
	s = strings.TrimLeft(s, " \t")
	for i, r := range s {
		if r == ' ' || r == '\t' {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

// cutOnce 按 sep 切成两段。
func cutOnce(s string, sep rune) (string, string) {
	if i := strings.IndexRune(s, sep); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// readQuoted 从 s 开头（不含引号）读到下一个未转义引号，返回内容与剩余部分。
func readQuoted(s string) (string, string, error) {
	var b strings.Builder
	esc := false
	for i, r := range s {
		switch {
		case esc:
			switch r {
			case 'n':
				b.WriteRune('\n')
			case 't':
				b.WriteRune('\t')
			default:
				b.WriteRune(r)
			}
			esc = false
		case r == '\\':
			esc = true
		case r == '"':
			return b.String(), s[i+1:], nil
		default:
			b.WriteRune(r)
		}
	}
	return "", "", fmt.Errorf("文本缺少收尾的双引号")
}

// looseText 把「没有属性名的文本」放到类型对应的字段上：
// icon 用图标名、image 用图片路径，其余都当正文。
func looseText(n *Node, s string) {
	switch strings.ToLower(strings.TrimSpace(n.Type)) {
	case "icon":
		n.Icon = s
	case "image":
		n.Text = s
	default:
		if n.Text == "" {
			n.Text = s
		} else {
			n.Text += " " + s
		}
	}
}

// setAttr 把 key=value 落到声明结构上；未知属性直接报错，避免拼写错误静默失效。
func setAttr(n *Node, key, val string) error {
	num := func() (float32, error) {
		v, err := strconv.ParseFloat(strings.TrimSpace(val), 32)
		if err != nil {
			return 0, fmt.Errorf("属性 %s 需要数字，收到 %q", key, val)
		}
		return float32(v), nil
	}
	int32v := func() (int32, error) {
		v, err := strconv.ParseFloat(strings.TrimSpace(val), 32)
		if err != nil {
			return 0, fmt.Errorf("属性 %s 需要数字，收到 %q", key, val)
		}
		return int32(v), nil
	}
	switch strings.ToLower(key) {
	case "key", "id":
		n.Key = val
	case "text":
		n.Text = val
	case "icon":
		n.Icon = val
	case "onclick", "click", "action":
		n.OnClick = val
	case "w", "width":
		return setNum(&n.W, num)
	case "h", "height":
		return setNum(&n.H, num)
	case "wp", "widthpct":
		return setNum(&n.Wp, num)
	case "hp", "heightpct":
		return setNum(&n.Hp, num)
	case "minw":
		return setNum(&n.MinW, num)
	case "maxw":
		return setNum(&n.MaxW, num)
	case "minh":
		return setNum(&n.MinH, num)
	case "maxh":
		return setNum(&n.MaxH, num)
	case "pad":
		return setNum(&n.Pad, num)
	case "padx":
		return setNum(&n.PadX, num)
	case "pady":
		return setNum(&n.PadY, num)
	case "mar":
		return setNum(&n.Mar, num)
	case "marx":
		return setNum(&n.MarX, num)
	case "mary":
		return setNum(&n.MarY, num)
	case "gap":
		return setNum(&n.Gap, num)
	case "grow":
		return setNum(&n.Grow, num)
	case "colw":
		return setNum(&n.ColW, num)
	case "cols":
		v, err := int32v()
		if err != nil {
			return err
		}
		n.Cols = int(v)
	case "bg":
		n.BG = val
	case "bgfrom":
		n.BGFrom = val
	case "bgto":
		n.BGTo = val
	case "bgangle":
		return setNum(&n.BGAngle, num)
	case "r", "radius":
		return setNum(&n.R, num)
	case "stroke":
		return setNum(&n.Stroke, num)
	case "strokecolor", "strokec":
		n.StrokeColor = val
	case "shadowblur":
		return setNum(&n.ShadowBlur, num)
	case "shadowy":
		return setNum(&n.ShadowY, num)
	case "shadowspread":
		return setNum(&n.ShadowSpread, num)
	case "shadowcolor":
		n.ShadowColor = val
	case "size":
		return setNum(&n.Size, num)
	case "weight":
		v, err := int32v()
		if err != nil {
			return err
		}
		n.Weight = v
	case "color":
		n.Color = val
	case "align":
		n.Align = val
	case "justify":
		n.Justify = val
	case "dir":
		n.Dir = val
	case "clip":
		n.Clip = val == "" || val == "true" || val == "1"
	case "opacity":
		return setNum(&n.Opacity, num)
	default:
		return fmt.Errorf("未知属性 %q（写 node.children 前先用 uidsl 文档里的属性名）", key)
	}
	return nil
}

func setNum(dst *float32, num func() (float32, error)) error {
	v, err := num()
	if err != nil {
		return err
	}
	*dst = v
	return nil
}
