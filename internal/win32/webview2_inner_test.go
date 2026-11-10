//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import "testing"

// TestFindInnerWindowReusesCallback 子窗口枚举回调必须复用。
// 回归背景：purego.NewCallback 每次都 mmap 一块可执行内存且无释放接口，
// 旧实现每次 findInnerWindow 都新建回调，网页窗口反复缩放会持续泄漏虚拟内存。
func TestFindInnerWindowReusesCallback(t *testing.T) {
	// 传 0（桌面）也会枚举顶层窗口，返回值取决于机器上是否开着 Chromium 窗口，
	// 因此这里只校验回调句柄的复用，不校验能不能找到子窗口。
	findInnerWindow(0)
	first := innerFindCB
	if first == 0 {
		t.Fatal("回调句柄未创建")
	}
	for i := 0; i < 500; i++ {
		findInnerWindow(0)
	}
	if innerFindCB != first {
		t.Fatalf("回调被重建: 0x%x -> 0x%x", first, innerFindCB)
	}
}
