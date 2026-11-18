// guis — gallery 网络请求页的布局冒烟：5 个页签都能排出有效尺寸
package pages

import (
	"testing"

	"guis/ui"
)

// TestNetPanelsLayout 逐个页签构建页面并做无窗口布局，确认控件都拿到有效尺寸。
func TestNetPanelsLayout(t *testing.T) {
	keys := map[string][]string{
		"basic":    {"netp-path", "netp-params", "netp-hdr-key", "netp-hdr-val"},
		"retry":    {"netp-retry-max", "netp-retry-backoff", "netp-timeout", "netp-retry-path"},
		"transfer": {"netp-up-field", "netp-up-name"},
		"advanced": {"netp-int", "netp-cookie"},
	}
	oldTab := netTab
	defer func() { netTab = oldTab }()

	for tab, want := range keys {
		netTab = tab
		root := netCase()
		if root == nil {
			t.Fatalf("%s 页返回 nil", tab)
		}
		ui.LayoutForTest(root, 1000, 1600, "", 1)
		for _, k := range want {
			n := findKeyNode(root, k)
			if n == nil {
				t.Fatalf("%s 页缺少节点 %s", tab, k)
			}
			if n.Frame.W <= 0 || n.Frame.H <= 0 {
				t.Fatalf("%s 页 %s 尺寸无效: %.1fx%.1f", tab, k, n.Frame.W, n.Frame.H)
			}
		}
	}

	// 代码页没有输入框，只确认能构建出非空节点树
	netTab = "code"
	root := netCase()
	ui.LayoutForTest(root, 1000, 1600, "", 1)
	if len(root.Children) == 0 {
		t.Fatal("示例代码页节点树为空")
	}
}
