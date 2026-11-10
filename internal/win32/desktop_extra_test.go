//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import "testing"

// TestMonitorsRepeat 反复枚举显示器：EnumDisplayMonitors 的回调句柄必须复用。
// 回归背景：旧实现每次 Monitors() 都调 syscall.NewCallback，回调表（约 2000 槽）
// 耗尽后 runtime 直接 fatal error: too many callback functions；
// UI 里该函数可能每帧调用，几千帧后整个进程崩溃，且 recover 拦不住。
func TestMonitorsRepeat(t *testing.T) {
	first := Monitors()
	if len(first) == 0 {
		t.Skip("当前环境没有枚举到显示器")
	}
	if !first[0].Primary {
		t.Fatalf("主屏应排在最前: %+v", first[0])
	}
	for i, m := range first {
		if m.Index != i {
			t.Fatalf("Index 不连续: 第 %d 个显示器 Index=%d", i, m.Index)
		}
		if m.W <= 0 || m.H <= 0 {
			t.Fatalf("显示器尺寸非法: %+v", m)
		}
	}
	const n = 3000
	for i := 0; i < n; i++ {
		got := Monitors()
		if len(got) != len(first) {
			t.Fatalf("第 %d 次枚举数量变化: got %d want %d", i+1, len(got), len(first))
		}
		if got[0].W != first[0].W || got[0].H != first[0].H {
			t.Fatalf("第 %d 次枚举结果不稳定: got %+v want %+v", i+1, got[0], first[0])
		}
	}
}
