package ui

import "testing"

// TestFileTreeContextMenuHook 右键回调挂在行上，触发前先把节点设为选中。
func TestFileTreeContextMenuHook(t *testing.T) {
	const key = "ctx-tree-test"
	t.Cleanup(func() { delete(treeSel, key) })

	var got TreeNode
	row := buildTreeRow(TreeOptions{
		Key: key, RowH: 30, Indent: 16,
		OnContextMenu: func(n TreeNode, x, y float32) { got = n },
	}, TreeNode{Key: "a.go", Label: "a.go", Kind: TreeFile}, 0, false, "")

	if row.OnRightClickAt == nil {
		t.Fatal("配置 OnContextMenu 后行节点应绑定右键回调")
	}
	row.OnRightClickAt(3, 4)
	if got.Key != "a.go" {
		t.Fatalf("右键回调节点不对：%+v", got)
	}
	if sel, ok := treeSel[key]; !ok || sel != "a.go" {
		t.Fatalf("右键应先选中该节点，实际 %q", sel)
	}
}

// TestFileTreeContextMenuDisabled 禁用节点（占位 / 空目录）不绑定右键。
func TestFileTreeContextMenuDisabled(t *testing.T) {
	row := buildTreeRow(TreeOptions{
		Key: "ctx-tree-disabled", RowH: 30,
		OnContextMenu: func(TreeNode, float32, float32) {},
	}, TreeNode{Key: "x", Label: "x", Disabled: true}, 0, false, "")
	if row.OnRightClickAt != nil {
		t.Fatal("禁用节点不应绑定右键回调")
	}
}

// TestFileTreeNoContextMenu 未配置回调时不挂右键（保持原行为）。
func TestFileTreeNoContextMenu(t *testing.T) {
	row := buildTreeRow(TreeOptions{Key: "ctx-tree-none", RowH: 30},
		TreeNode{Key: "a.go", Label: "a.go", Kind: TreeFile}, 0, false, "")
	if row.OnRightClickAt != nil {
		t.Fatal("未配置 OnContextMenu 时不应绑定右键")
	}
}
