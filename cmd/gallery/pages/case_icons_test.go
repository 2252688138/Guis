// guis — gallery 图标库页布局冒烟测试
// 只做无窗口布局：确认网格 / 组合场景 / 自定义图标区块都排出了有效尺寸。

package pages

import (
	"testing"

	"guis/ui"
)

// TestIconsPageRenders 图标库页：关键节点存在且尺寸有效。
func TestIconsPageRenders(t *testing.T) {
	prevQuery, prevPick := iconQuery, iconPicked
	iconQuery, iconPicked = "", "search"
	defer func() { iconQuery, iconPicked = prevQuery, prevPick }()

	root := iconsCase()
	if root == nil {
		t.Fatal("iconsCase 返回 nil")
	}
	ui.LayoutForTest(root, 1000, 2400, "", 1)

	names := []string{"search", "play", "file", "settings", "guis-logo"}
	for _, name := range names {
		n := findKeyNode(root, "icon-cell-"+name)
		if n == nil {
			t.Fatalf("图标网格缺少 %s", name)
		}
		if n.Frame.W <= 0 || n.Frame.H <= 0 {
			t.Fatalf("%s 尺寸无效: %.1fx%.1f", name, n.Frame.W, n.Frame.H)
		}
	}

	// 组合场景与搜索框都要真实排出来
	for _, k := range []string{"icon-search", "icon-demo-search"} {
		if n := findKeyNode(root, k); n == nil || n.Frame.W <= 0 {
			t.Fatalf("缺少输入控件 %s", k)
		}
	}
}

// TestIconFilter 关键字过滤：英文名 / 中文名都能命中，空串命中全部。
func TestIconFilter(t *testing.T) {
	prev := iconQuery
	defer func() { iconQuery = prev }()

	iconQuery = ""
	all := len(iconFilteredNames())
	if all < 70 {
		t.Fatalf("空关键字应命中全部图标，实际 %d", all)
	}

	iconQuery = "SEARCH"
	got := iconFilteredNames()
	if len(got) == 0 || got[0] != "search" {
		t.Fatalf("按英文名过滤失败：%v", got)
	}

	iconQuery = "删除"
	got = iconFilteredNames()
	found := false
	for _, n := range got {
		if n == "trash" {
			found = true
		}
	}
	if !found {
		t.Fatalf("按中文名过滤未命中 trash：%v", got)
	}

	iconQuery = "no-such-icon-xyz"
	if n := len(iconFilteredNames()); n != 0 {
		t.Fatalf("不存在的关键字应无结果，实际 %d", n)
	}
}

// TestCustomIconRegistered 演示用自定义图标注册后可见（重复调用保持幂等）。
func TestCustomIconRegistered(t *testing.T) {
	ensureCustomIcon()
	ensureCustomIcon()
	if !ui.IconExists("guis-logo") {
		t.Fatal("自定义图标未注册")
	}
	def, _ := ui.IconDefOf("guis-logo")
	if len(def.Paths) == 0 || def.Label == "" {
		t.Fatalf("自定义图标定义不完整：%+v", def)
	}
}
