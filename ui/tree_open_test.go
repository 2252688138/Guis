package ui

import (
	"testing"
	"time"
)

// treeOpenTestRow 构建一行并返回 (row, picks, opens 计数指针)。
func treeOpenTestRow(t *testing.T, key string, node TreeNode) (*Node, *int, *int) {
	t.Helper()
	picks, opens := 0, 0
	row := buildTreeRow(TreeOptions{
		Key: key, RowH: 30, Indent: 16,
		OnPick: func(n TreeNode) { picks++ },
		OnOpen: func(n TreeNode) { opens++ },
	}, node, 0, false, "")
	return row, &picks, &opens
}

// TestFileTreeDoubleClickOnOpen 同节点 600ms 内二次点击：只触发 OnOpen，且不再触发 OnPick。
func TestFileTreeDoubleClickOnOpen(t *testing.T) {
	const key = "tree-open-test"
	t.Cleanup(func() { delete(treeSel, key); treeResetClickState() })
	treeResetClickState()

	row, picks, opens := treeOpenTestRow(t, key, TreeNode{Key: "a.go", Label: "a.go", Kind: TreeFile})
	row.OnClick()
	if *picks != 1 || *opens != 0 {
		t.Fatalf("首次点击应只触发 OnPick：picks=%d opens=%d", *picks, *opens)
	}
	row.OnClick()
	if *picks != 1 || *opens != 1 {
		t.Fatalf("二次点击应只触发 OnOpen：picks=%d opens=%d", *picks, *opens)
	}
	// 双击后记账已清空：再一次点击重新按单击处理。
	row.OnClick()
	if *picks != 2 || *opens != 1 {
		t.Fatalf("双击后再点击应重新算单击：picks=%d opens=%d", *picks, *opens)
	}
}

// TestFileTreeDoubleClickDifferentNode 连续点不同节点不构成双击。
func TestFileTreeDoubleClickDifferentNode(t *testing.T) {
	const key = "tree-open-node-test"
	t.Cleanup(func() { delete(treeSel, key); treeResetClickState() })
	treeResetClickState()

	rowA, picksA, opensA := treeOpenTestRow(t, key, TreeNode{Key: "a.go", Label: "a.go", Kind: TreeFile})
	rowB, picksB, opensB := treeOpenTestRow(t, key, TreeNode{Key: "b.go", Label: "b.go", Kind: TreeFile})
	rowA.OnClick()
	rowB.OnClick()
	if *picksA != 1 || *picksB != 1 || *opensA != 0 || *opensB != 0 {
		t.Fatalf("不同节点不算双击：A picks=%d opens=%d B picks=%d opens=%d",
			*picksA, *opensA, *picksB, *opensB)
	}
}

// TestFileTreeDoubleClickWindow 超出双击窗口的二次点击仍是两次单击。
func TestFileTreeDoubleClickWindow(t *testing.T) {
	const key = "tree-open-window-test"
	t.Cleanup(func() { delete(treeSel, key); treeResetClickState() })
	treeResetClickState()

	row, picks, opens := treeOpenTestRow(t, key, TreeNode{Key: "a.go", Label: "a.go", Kind: TreeFile})
	row.OnClick()
	// 把记账时间推到窗口之外，模拟用户慢慢点两下。
	treeLastClick.at = time.Now().Add(-(treeDoubleClickMs + 100) * time.Millisecond)
	row.OnClick()
	if *picks != 2 || *opens != 0 {
		t.Fatalf("超窗口的两次点击应各算一次单击：picks=%d opens=%d", *picks, *opens)
	}
}

// TestFileTreeNoOnOpenKeepsPick 未配置 OnOpen 时，连点两下仍各自走 OnPick（行为不变）。
func TestFileTreeNoOnOpenKeepsPick(t *testing.T) {
	const key = "tree-no-open-test"
	t.Cleanup(func() { delete(treeSel, key); treeResetClickState() })
	treeResetClickState()

	picks := 0
	row := buildTreeRow(TreeOptions{
		Key: key, RowH: 30, Indent: 16,
		OnPick: func(n TreeNode) { picks++ },
	}, TreeNode{Key: "a.go", Label: "a.go", Kind: TreeFile}, 0, false, "")
	row.OnClick()
	row.OnClick()
	if picks != 2 {
		t.Fatalf("未配置 OnOpen 时不应启用双击判定，picks=%d", picks)
	}
}
