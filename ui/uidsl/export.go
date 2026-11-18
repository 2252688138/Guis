// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package uidsl

// 节点树 → DSL 反向导出。
//
// 用途：调试面板里选中一个节点就能复制出它的 DSL 片段，改写界面时不必从零手写；
// 也便于把「代码里写死的界面」迁到可热改的 .ui 文件。
//
// 导出是「尽力而为」的：DSL 只覆盖 box/row/col/center/stack/grid/wrap/scroll/
// spacer/text/button/icon/image 这些类型与常用样式，其余节点输出一行注释占位
// （不会丢失结构，但需要人工补写）。半透明颜色无法用 #rrggbb 表达，也跳过。
//
//	dsl := uidsl.FromNode(view())   // 缩进嵌套、一行一个节点

import (
	"fmt"
	"strconv"
	"strings"

	"guis/internal/gpu"
	"guis/ui"
)

// FromNode 把一棵节点树导出成 DSL 文本。
func FromNode(n *ui.Node) string {
	if n == nil {
		return ""
	}
	var sb strings.Builder
	writeNode(&sb, n, 0)
	return sb.String()
}

// FromNodes 导出多个同级节点（各占一行，保持同级缩进）。
func FromNodes(ns []*ui.Node) string {
	var sb strings.Builder
	for _, n := range ns {
		writeNode(&sb, n, 0)
	}
	return sb.String()
}

func writeNode(sb *strings.Builder, n *ui.Node, depth int) {
	if n == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	typ, ok := dslType(n)
	if !ok {
		sb.WriteString(indent + "# [不支持导出的节点：" + kindLabel(n.Kind) + "]\n")
		return
	}

	var parts []string
	parts = append(parts, typ)

	// 文本 / 图标 / 图片的内容放引号里，紧跟类型名（DSL 解析规则）。
	switch typ {
	case "text", "button":
		if n.Text != "" {
			parts = append(parts, quoteDSL(n.Text))
		}
	case "icon":
		if n.IconName != "" {
			parts = append(parts, quoteDSL(n.IconName))
			parts = parts[:len(parts)-1]
			parts = append(parts, "icon="+n.IconName)
		}
	case "image":
		if n.Src != "" {
			parts = append(parts, quoteDSL(n.Src))
		}
	}

	if n.Key != "" {
		parts = append(parts, "key="+n.Key)
	}
	if n.OnClick != nil {
		// 回调无法还原成事件名，留一个待填的占位，避免静默丢交互。
		parts = append(parts, "onClick="+placeholderAction)
	}
	parts = append(parts, styleProps(n)...)

	sb.WriteString(indent + strings.Join(parts, " ") + "\n")
	for _, c := range n.Children {
		writeNode(sb, c, depth+1)
	}
}

// placeholderAction 导出时占住事件位的事件名。
const placeholderAction = "TODO"

// styleProps 把样式翻译成 key=value 列表（只导出非零值）。
func styleProps(n *ui.Node) []string {
	st := &n.Style
	var out []string
	addF := func(name string, v float32) {
		if v != 0 {
			out = append(out, name+"="+num(v))
		}
	}
	addI := func(name string, v int) {
		if v != 0 {
			out = append(out, name+"="+strconv.Itoa(v))
		}
	}

	addF("w", st.Width)
	addF("h", st.Height)
	addF("wp", st.WidthPct)
	addF("hp", st.HeightPct)
	addF("minW", st.MinW)
	addF("maxW", st.MaxW)
	addF("minH", st.MinH)
	addF("maxH", st.MaxH)

	// 内边距：四边相等用 pad，左右/上下成对用 padX/padY，其余丢弃（DSL 无单边属性）。
	if st.Pad.Top != 0 || st.Pad.Right != 0 || st.Pad.Bottom != 0 || st.Pad.Left != 0 {
		switch {
		case st.Pad.Top == st.Pad.Bottom && st.Pad.Left == st.Pad.Right && st.Pad.Top == st.Pad.Left:
			addF("pad", st.Pad.Top)
		case st.Pad.Top == st.Pad.Bottom && st.Pad.Left == st.Pad.Right:
			addF("padX", st.Pad.Left)
			addF("padY", st.Pad.Top)
		}
	}
	if st.Margin.Top != 0 || st.Margin.Right != 0 || st.Margin.Bottom != 0 || st.Margin.Left != 0 {
		switch {
		case st.Margin.Top == st.Margin.Bottom && st.Margin.Left == st.Margin.Right && st.Margin.Top == st.Margin.Left:
			addF("mar", st.Margin.Top)
		case st.Margin.Top == st.Margin.Bottom && st.Margin.Left == st.Margin.Right:
			addF("marX", st.Margin.Left)
			addF("marY", st.Margin.Top)
		}
	}

	addF("gap", st.Gap)
	addF("grow", st.Grow)
	addI("cols", st.Cols)
	addF("colW", st.ColW)

	if st.BgSet {
		if hex, ok := paintHex(st.Bg); ok {
			out = append(out, "bg="+hex)
		}
	}
	if st.BgToSet {
		if hex, ok := paintHex(st.BgTo); ok {
			out = append(out, "bgTo="+hex)
		}
		addF("bgAngle", st.BgAngle)
	}
	if st.Radius[0] != 0 && st.Radius[0] == st.Radius[1] && st.Radius[0] == st.Radius[2] && st.Radius[0] == st.Radius[3] {
		addF("r", st.Radius[0])
	}
	addF("size", st.FontSize)
	addI("weight", int(st.FontWeight))
	if c, ok := paintHex(st.Color); ok {
		out = append(out, "color="+c)
	}
	if st.Clip {
		out = append(out, "clip=true")
	}
	if st.Opacity > 0 && st.Opacity < 1 {
		addF("opacity", st.Opacity)
	}
	if s := alignName(st.AlignSelf); s != "" {
		out = append(out, "align="+s)
	}
	return out
}

// dslType 地图节点类型；不支持的类型返回 false。
func dslType(n *ui.Node) (string, bool) {
	switch n.Kind {
	case ui.KBox:
		switch n.Dir {
		case ui.Horizontal:
			return "row", true
		default:
			return "col", true
		}
	case ui.KText:
		return "text", true
	case ui.KButton:
		return "button", true
	case ui.KScroll:
		return "scroll", true
	case ui.KImage:
		return "image", true
	case ui.KStack:
		return "stack", true
	case ui.KGrid:
		return "grid", true
	case ui.KWrap:
		return "wrap", true
	case ui.KIcon:
		return "icon", true
	}
	return "", false
}

func kindLabel(k ui.Kind) string {
	return fmt.Sprintf("Kind(%d)", int(k))
}

func alignName(a ui.Align) string {
	switch a {
	case ui.AlignStart:
		return ""
	case ui.AlignCenter:
		return "center"
	case ui.AlignEnd:
		return "end"
	case ui.AlignStretch:
		return "stretch"
	}
	return ""
}

// paintHex 把颜色转成 #rrggbb；全透明或半透明（DSL 表达不了）时返回 false。
func paintHex(p gpu.Paint) (string, bool) {
	if p.A <= 0.004 || p.A < 0.999 {
		return "", false
	}
	r := clamp255(p.R)
	g := clamp255(p.G)
	b := clamp255(p.B)
	if r == 0 && g == 0 && b == 0 {
		return "", false // 纯黑多为零值兜底，导出它只会制造噪声
	}
	return fmt.Sprintf("#%02x%02x%02x", r, g, b), true
}

func clamp255(v float32) int {
	n := int(v*255 + 0.5)
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

// num 输出不带多余小数点的数字。
func num(v float32) string {
	if v == float32(int(v)) {
		return strconv.Itoa(int(v))
	}
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}

// quoteDSL 加引号并转义（DSL 里文本统一用双引号）。
func quoteDSL(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString("\\\"")
		case '\\':
			sb.WriteString("\\\\")
		case '\n':
			sb.WriteString("\\n")
		case '\t':
			sb.WriteString("\\t")
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
