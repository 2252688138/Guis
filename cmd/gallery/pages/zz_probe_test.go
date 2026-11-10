// 临时探针：打印窄屏（手机）下基础页里所有输入框的 Key 与位置。

package pages

import (
	"fmt"
	"strings"
	"testing"

	"guis/ui"
)

func dumpKeys(n *ui.Node, depth int) {
	if n == nil {
		return
	}
	k := strings.ToLower(n.Key)
	if n.Key != "" && (strings.Contains(k, "input") || strings.Contains(k, "ctrl") || strings.Contains(k, "msg") || strings.Contains(k, "settings")) {
		fmt.Printf("key=%-28s frame=(%.0f,%.0f %.0fx%.0f)\n", n.Key, n.Frame.X, n.Frame.Y, n.Frame.W, n.Frame.H)
	}
	for _, c := range n.Children {
		dumpKeys(c, depth+1)
	}
}

func TestProbeInputPos(t *testing.T) {
	old := Current
	Current = "basic"
	defer func() { Current = old }()

	ui.LayoutForTest(View(), 480, 853, "", 1)
	root := View()
	ui.LayoutForTest(root, 480, 853, "", 1)
	dumpKeys(root, 0)
}
