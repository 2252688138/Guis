// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "strconv"

// 无障碍静态检查（A11yLint）。
//
// a11y.go 只做「声明 + 导出」（A11ySnapshot），本文件补上「检查」这一环：
// 在开发期扫一遍界面树，把「点了没名字」「可聚焦但没角色」「Key 重复导致跨帧状态
// 互相覆盖」这类问题以清单形式返回，供调试面板 / 单测 / CI 使用。
//
//	// 页面构建后立刻自检（开发期）：
//	if issues := ui.A11yLint(view()); len(issues) > 0 {
//	    for _, it := range issues {
//	        log.Println(it.String())
//	    }
//	}
//
// 注意：本检查是纯静态的（只看节点声明），不代表系统级无障碍已可用——
// 向 UIA / AT-SPI 注册 Provider 是另一件事，见 a11y.go 顶部说明。

// A11yIssue 一条无障碍问题。
type A11yIssue struct {
	// Path 节点在树中的位置（形如 /col/button[2]，根为 /）。
	Path string
	// Code 问题代码（见 A11y* 常量），便于 CI 里按类型过滤。
	Code string
	// Key 节点 Key（可能为空）。
	Key string
	// Msg 面向人的说明。
	Msg string
}

// String 便于日志直接打印。
func (i A11yIssue) String() string {
	return "a11y[" + i.Code + "] " + i.Path + ": " + i.Msg
}

// 问题代码。
const (
	// A11yIssueMissingRole 可交互节点没有声明角色。
	A11yIssueMissingRole = "missing-role"
	// A11yIssueMissingName 可交互节点没有可读名称（既无 A11y.Name 也无文本）。
	A11yIssueMissingName = "missing-name"
	// A11yIssueFocusNoKey 节点可聚焦但没有 Key：焦点跨帧按 Key 继承，无 Key 会每帧丢失。
	A11yIssueFocusNoKey = "focus-without-key"
	// A11yIssueDuplicateKey 同一棵树里 Key 重复：交互状态会互相覆盖。
	A11yIssueDuplicateKey = "duplicate-key"
	// A11yIssueImageNoName 图片没有替代文本。
	A11yIssueImageNoName = "image-without-name"
)

// A11yLint 检查一棵节点树的无障碍问题（先序遍历，问题顺序与树顺序一致）。
//
// 规则：
//   - 可交互（有 OnClick / OnClickAt / OnActivate / OnChar / OnKey / OnDrop）的节点
//     必须声明 A11yRole，且要有名称（A11y.Name 或 Text 非空）；
//   - Style.Focusable 的节点必须带 Key（否则焦点无法跨帧保持）；
//   - KImage 必须有 A11y.Name（替代文本）；
//   - 全局 Key 不得重复。
func A11yLint(root *Node) []A11yIssue {
	if root == nil {
		return nil
	}
	var out []A11yIssue
	seen := make(map[string]int, 64)

	var walk func(n *Node, path string, idx int)
	walk = func(n *Node, path string, idx int) {
		if n == nil {
			return
		}
		seg := nodePathSeg(n, idx)
		p := path + "/" + seg

		if n.Key != "" {
			seen[n.Key]++
			if seen[n.Key] == 2 {
				out = append(out, A11yIssue{
					Path: p, Code: A11yIssueDuplicateKey, Key: n.Key,
					Msg: "Key \"" + n.Key + "\" 在树中重复，交互状态会互相覆盖",
				})
			}
		}

		interactive := n.OnClick != nil || n.OnClickAt != nil || n.OnActivate != nil ||
			n.OnChar != nil || n.OnKey != nil || n.OnDrop != nil
		if interactive || n.Style.Focusable {
			if n.A11y.Role == A11yNone {
				out = append(out, A11yIssue{
					Path: p, Code: A11yIssueMissingRole, Key: n.Key,
					Msg: "可交互/可聚焦节点未声明角色（用 .A11yOn(ui.A11yButton, \"名称\", \"\")）",
				})
			}
			if n.A11y.Name == "" && n.Text == "" {
				out = append(out, A11yIssue{
					Path: p, Code: A11yIssueMissingName, Key: n.Key,
					Msg: "可交互节点没有可读名称",
				})
			}
		}
		if n.Style.Focusable && n.Key == "" {
			out = append(out, A11yIssue{
				Path: p, Code: A11yIssueFocusNoKey,
				Msg: "可聚焦节点没有 Key，焦点无法跨帧保持",
			})
		}
		if n.Kind == KImage && n.A11y.Name == "" {
			out = append(out, A11yIssue{
				Path: p, Code: A11yIssueImageNoName, Key: n.Key,
				Msg: "图片缺少替代文本（A11y.Name）",
			})
		}
		for i, c := range n.Children {
			walk(c, p, i)
		}
	}
	walk(root, "", 0)
	return out
}

// nodePathSeg 生成路径片段：有 Key 用 Key，否则用「类型[序号]」。
func nodePathSeg(n *Node, idx int) string {
	if n.Key != "" {
		return n.Key
	}
	name := kindName(n.Kind)
	if idx > 0 {
		name += "[" + strconv.Itoa(idx) + "]"
	}
	return name
}

// kindName 节点类型名（调试路径用，未列出的类型归入 box）。
func kindName(k Kind) string {
	switch k {
	case KText:
		return "text"
	case KButton:
		return "button"
	case KScroll:
		return "scroll"
	case KImage:
		return "image"
	case KStack:
		return "stack"
	case KGrid:
		return "grid"
	case KWrap:
		return "wrap"
	case KMasonry:
		return "masonry"
	case KVideo:
		return "video"
	case KWebView:
		return "webview"
	case KArrow:
		return "arrow"
	case KChart:
		return "chart"
	case KRichText:
		return "richtext"
	case KIcon:
		return "icon"
	case KTerm:
		return "term"
	}
	return "box"
}
