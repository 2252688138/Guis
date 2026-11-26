package ui

import "testing"

// ttCollectKeys 收集节点树上所有非空 Key（供结构断言复用）。
func ttCollectKeys(n *Node, out map[string]bool) {
	if n == nil {
		return
	}
	if n.Key != "" {
		out[n.Key] = true
	}
	for _, c := range n.Children {
		ttCollectKeys(c, out)
	}
}

func ttSampleRows() []TreeTableRow {
	return []TreeTableRow{
		{Key: "a", Texts: []string{"分组 A", "2"}, Children: []TreeTableRow{
			{Key: "a1", Texts: []string{"条目 A1", "开"}},
			{Key: "a2", Texts: []string{"条目 A2", "关"}, Children: []TreeTableRow{
				{Key: "a21", Texts: []string{"条目 A21", "关"}},
			}},
		}},
		{Key: "b", Texts: []string{"分组 B", "0"}},
	}
}

func TestTreeTableFoldAndExpand(t *testing.T) {
	key := "t-tree-table-fold"
	TreeTableReset(key)
	defer TreeTableReset(key)

	nodes := func() map[string]bool {
		m := map[string]bool{}
		ttCollectKeys(TreeTable(ttSampleRows(), TreeTableOptions{
			Key: key, Headers: []string{"名称", "状态"}, Widths: []float32{220},
		}), m)
		return m
	}

	// 全收起：只有顶层两行
	if got := nodes(); !got["/a"] || !got["/b"] || got["/a/a1"] {
		t.Fatalf("收起状态应只有顶层行，实际 a=%v b=%v a1=%v", got["/a"], got["/b"], got["/a/a1"])
	}

	// 展开 a：应多出 a1、a2（a21 仍收起）
	TreeTableSetOpen(key, "a", true)
	if got := nodes(); !got["/a/a1"] || !got["/a/a2"] || got["/a/a2/a21"] {
		t.Fatalf("展开 a 后应只出现一层子行，实际 a1=%v a2=%v a21=%v",
			got["/a/a1"], got["/a/a2"], got["/a/a2/a21"])
	}

	// 再展开 a2：孙子行出现
	TreeTableToggle(key, "a/a2")
	if !TreeTableIsOpen(key, "a/a2") {
		t.Fatal("a2 应处于展开状态")
	}
	if got := nodes(); !got["/a/a2/a21"] {
		t.Fatal("展开 a2 后应出现 a21")
	}

	// 收全：回到两行
	TreeTableSetAllOpen(key, ttSampleRows(), false)
	if got := nodes(); got["/a/a1"] || got["/a/a2"] {
		t.Fatal("全部收起后不应再有子行")
	}
}

func TestTreeTableDefaultOpenAndSelect(t *testing.T) {
	key := "t-tree-table-open"
	TreeTableReset(key)
	defer TreeTableReset(key)

	m := map[string]bool{}
	ttCollectKeys(TreeTable(ttSampleRows(), TreeTableOptions{
		Key: key, Headers: []string{"名称"}, DefaultOpen: true,
	}), m)
	for _, k := range []string{"/a", "/b", "/a/a1", "/a/a2", "/a/a2/a21"} {
		if !m[k] {
			t.Fatalf("DefaultOpen 应展开全部层级，缺少 %s", k)
		}
	}

	var clicked string
	TreeTable(ttSampleRows(), TreeTableOptions{Key: key, Headers: []string{"名称"},
		OnClick: func(path []string, row TreeTableRow) { clicked = row.Key }})
	TreeTableSetSelected(key, "a/a1")
	if got := TreeTableSelected(key); got != "a/a1" {
		t.Fatalf("选中行应为 a/a1，实际 %q", got)
	}
	TreeTableSetSelected(key, "")
	if TreeTableSelected(key) != "" {
		t.Fatal("清空后应无选中行")
	}
	_ = clicked
}
