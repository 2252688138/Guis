package ui

import "testing"

func TestPopconfirmToggle(t *testing.T) {
	PopconfirmClose()
	key := "t-popconfirm"
	var confirmed bool

	n := Popconfirm(key, "删除", "确定删除这条记录？", BtnDanger, func() { confirmed = true })
	if n == nil || PopconfirmIsOpen(key) {
		t.Fatal("初始应为收起状态")
	}
	// 直接置位模拟点击展开
	popconfirmOpen[key] = true
	n = Popconfirm(key, "删除", "确定删除这条记录？", BtnDanger, func() { confirmed = true })
	if len(n.Children) != 2 {
		t.Fatalf("展开后应为「提示 + 操作」两段，实际 %d", len(n.Children))
	}
	if !confirmed == false {
		t.Fatal("展开本身不应触发确认回调")
	}

	PopconfirmClose()
	if PopconfirmIsOpen(key) {
		t.Fatal("PopconfirmClose 应把全部确认收起")
	}
}
