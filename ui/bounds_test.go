package ui

import "testing"

// TestAppBoundsNilSafe 窗口句柄为空时，几何读写必须是安全的空操作。
//
// Bounds / SetBounds 是「记住独立子窗口位置」的基础（见 SetBounds 的注释），
// 业务代码在窗口可能已关闭的路径上调用它们，不能 panic。
func TestAppBoundsNilSafe(t *testing.T) {
	var a *App
	if _, _, _, _, ok := a.Bounds(); ok {
		t.Fatal("nil App 不应报告有效窗口几何")
	}
	a.SetBounds(1, 2, 3, 4) // 不应 panic

	empty := &App{}
	if _, _, _, _, ok := empty.Bounds(); ok {
		t.Fatal("没有后端的 App 不应报告有效窗口几何")
	}
	empty.SetBounds(10, 20, 30, 40)
}
