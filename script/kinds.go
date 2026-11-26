// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

import (
	"fmt"
	"strings"

	"guis/ui/uidsl"
)
// 脚本可用的节点类型清单：与 ui/uidsl 的 build 分支一一对应。
// 这里是唯一来源——构造器注册（bindUI）与未知类型校验（validateKinds）都读它。

var (
	// 容器
	containerKinds = []string{"box", "col", "row", "center", "stack", "grid", "wrap", "scroll", "spacer"}
	// 叶子
	leafKinds = []string{"text", "button", "icon", "image"}
	// 表单 / 值控件
	formKinds = []string{"input", "textarea", "checkbox", "switch", "slider", "progress"}
	// 扩展组件（卡片、富文本、图表、代码编辑器、对话框…）
	extraKinds = []string{
		"divider", "card", "badge", "avatar", "chip", "spinner", "skeleton",
		"markdown", "richtext", "masonry", "arrow", "segmented",
		"select", "chart", "codeeditor",
		"alert", "confirm", "drawer",
		"term", "video", "webview", "menubar", "viewport", "canvas",
	}
)

// allKinds 全部节点类型（按注册顺序）。
var allKinds = func() []string {
	out := make([]string, 0, 64)
	out = append(out, containerKinds...)
	out = append(out, leafKinds...)
	out = append(out, formKinds...)
	out = append(out, extraKinds...)
	return out
}()

// knownKinds 类型名 → 是否存在。
var knownKinds = func() map[string]bool {
	m := make(map[string]bool, len(allKinds)*2)
	for _, k := range allKinds {
		m[k] = true
	}
	// uidsl 层接受的别名（描述文件里可直接写）
	for _, k := range []string{"vbox", "hbox", "column", "textfield", "field", "check", "toggle", "bar"} {
		m[k] = true
	}
	return m
}()

// 体积上限默认值。
const (
	defaultHTTPMaxBytes   int64 = 8 << 20
	defaultFSMaxReadBytes int64 = 8 << 20
)

// validateKinds 走一遍声明树，遇到脚本层不支持的类型直接报错，不再静默降级。
func validateKinds(n *uidsl.Node) error {
	if n == nil {
		return nil
	}
	t := strings.ToLower(strings.TrimSpace(n.Type))
	if t != "" && !knownKinds[t] {
		return fmt.Errorf("节点类型 %q 不受支持（可用类型见 docs/script-lua.md）", t)
	}
	for i := range n.Children {
		if err := validateKinds(&n.Children[i]); err != nil {
			return err
		}
	}
	return nil
}
