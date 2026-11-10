// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// TestHotkeyMods 修饰键换算：框架 Hot* → X11 掩码 / macOS CGEventFlags。
//
// 这两张表错一位就表现为「热键注册成功但按了没反应」，是最难排查的一类问题，
// 所以单独锁死。
func TestHotkeyMods(t *testing.T) {
	type row struct {
		in    uint32
		x11   uint32
		macos uint64
	}
	rows := []row{
		{0, 0, 0},
		{HotCtrl, 1 << 2, 0x00040000},
		{HotShift, 1 << 0, 0x00020000},
		{HotAlt, 1 << 3, 0x00080000},
		{HotWin, 1 << 6, 0x00100000},
		{HotCtrl | HotShift, 1<<2 | 1<<0, 0x00060000},
		{HotCtrl | HotAlt, 1<<2 | 1<<3, 0x000C0000},
		{HotCtrl | HotAlt | HotShift | HotWin, 1<<2 | 1<<3 | 1<<0 | 1<<6, 0x001E0000},
		// HotNoRepeat 只对 Windows 有意义，不应影响掩码
		{HotCtrl | HotNoRepeat, 1 << 2, 0x00040000},
	}
	for _, c := range rows {
		if got := hotkeyX11Mods(c.in); got != c.x11 {
			t.Fatalf("hotkeyX11Mods(%#x) = %#x, want %#x", c.in, got, c.x11)
		}
		if got := hotkeyMacFlags(c.in); got != c.macos {
			t.Fatalf("hotkeyMacFlags(%#x) = %#x, want %#x", c.in, got, c.macos)
		}
	}
}
