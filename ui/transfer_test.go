package ui

import "testing"

func transferSample() []TransferItem {
	return []TransferItem{
		{Key: "r", Label: "只读"},
		{Key: "w", Label: "可写", Hint: "写"},
		{Key: "x", Label: "可执行"},
	}
}

func TestTransferMoveBothWays(t *testing.T) {
	s := NewTransfer("t-transfer-move")
	if s.Key != "t-transfer-move" || TransferOf("t-transfer-move") != s {
		t.Fatal("NewTransfer 应按 key 注册并可取回")
	}

	s.SelLeft["r"] = true
	s.SelLeft["x"] = true
	s.MoveRight()
	if got := s.RightKeys(transferSample()); len(got) != 2 || got[0] != "r" || got[1] != "x" {
		t.Fatalf("右侧应按 items 顺序为 [r x]，实际 %v", got)
	}
	if len(s.SelLeft) != 0 {
		t.Fatal("移动后左侧勾选应清空")
	}

	s.SelRight["r"] = true
	s.MoveLeft()
	if got := s.RightKeys(transferSample()); len(got) != 1 || got[0] != "x" {
		t.Fatalf("移回后右侧应只剩 x，实际 %v", got)
	}

	s.Reset()
	if len(s.RightKeys(transferSample())) != 0 {
		t.Fatal("Reset 后右侧应为空")
	}
}

func TestTransferRenderAndFilter(t *testing.T) {
	s := NewTransfer("t-transfer-render")
	s.FilterL = "可写"
	var changed bool
	n := Transfer(s, transferSample(), TransferOptions{
		Key: "t-transfer-render", Filterable: true, Height: 200, PanelWidth: 160,
		OnChange: func(left, right []string) { changed = true },
	})
	if n == nil || n.Key != "t-transfer-render" {
		t.Fatal("Transfer 应返回带 Key 的节点")
	}
	if len(n.Children) != 3 {
		t.Fatalf("应有左面板 + 中间按钮 + 右面板三段，实际 %d", len(n.Children))
	}
	s.SelLeft["w"] = true
	s.MoveRight()
	if !changed {
		t.Fatal("OnChange 应在转移后触发")
	}
	if got := s.RightLabels(transferSample()); len(got) != 1 || got[0] != "可写" {
		t.Fatalf("右侧文案应为[可写]，实际 %v", got)
	}
}
