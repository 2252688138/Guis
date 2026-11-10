// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// 全局热键的修饰键换算（纯函数，无平台依赖，可在任何系统上单测）。
//
// 框架沿用 Windows 的 Hot* 常量（MOD_ALT=1 / MOD_CONTROL=2 / MOD_SHIFT=4 / MOD_WIN=8），
// 而 X11 与 macOS 的掩码完全不同，必须显式映射——直接透传会出现
// 「注册成功但永远不触发」这种最难查的问题。
package ui

// X11 修饰键掩码（与 internal/x11 的 ShiftMask/ControlMask/Mod1Mask/Mod4Mask 一致）。
const (
	x11MaskShift   = 1 << 0
	x11MaskControl = 1 << 2
	x11MaskMod1    = 1 << 3 // Alt
	x11MaskMod4    = 1 << 6 // Super / Win
)

// macOS CGEventFlags。
const (
	cfMaskShift   = 0x00020000
	cfMaskControl = 0x00040000
	cfMaskOption  = 0x00080000
	cfMaskCommand = 0x00100000
)

// hotkeyX11Mods 框架 Hot* 修饰键 → X11 掩码。
func hotkeyX11Mods(mods uint32) uint32 {
	var m uint32
	if mods&HotShift != 0 {
		m |= x11MaskShift
	}
	if mods&HotCtrl != 0 {
		m |= x11MaskControl
	}
	if mods&HotAlt != 0 {
		m |= x11MaskMod1
	}
	if mods&HotWin != 0 {
		m |= x11MaskMod4
	}
	return m
}

// hotkeyMacFlags 框架 Hot* 修饰键 → CGEventFlags。
func hotkeyMacFlags(mods uint32) uint64 {
	var f uint64
	if mods&HotShift != 0 {
		f |= cfMaskShift
	}
	if mods&HotCtrl != 0 {
		f |= cfMaskControl
	}
	if mods&HotAlt != 0 {
		f |= cfMaskOption
	}
	if mods&HotWin != 0 {
		f |= cfMaskCommand
	}
	return f
}
