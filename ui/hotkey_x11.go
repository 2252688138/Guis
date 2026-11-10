//go:build (linux || freebsd) && !android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// X11 全局热键：XGrabKey + 根窗口 KeyPress 匹配。
//
// 与 Windows 端 RegisterHotKey 语义一致：拿到 id（0 = 注册失败），
// 注销用同一个 id；回调在 UI 线程执行，可直接改状态后 RequestRedraw。
package ui

import (
	"sync"
	"sync/atomic"

	"guis/internal/x11"
)

// x11Hotkey 一个已注册的全局热键。
type x11Hotkey struct {
	id      int32
	keycode int
	xmods   uint32 // X11 修饰键掩码（已按 ui 层的 Hot* 常量换算）
	fn      func()
}

var (
	x11HotkeyMu  sync.Mutex
	x11HotkeySeq int32
	// x11Hotkeys 按 id 索引（注销用）。
	x11Hotkeys = map[int32]*x11Hotkey{}
	// x11HotkeyByKey 按「修饰键<<16 | 键码」索引（事件匹配用）。
	x11HotkeyByKey = map[uint32]*x11Hotkey{}
)

// x11Mods 把框架的 Hot* 修饰键换算成 X11 掩码（映射表在 hotkeymods.go，可单测）。
func x11Mods(mods uint32) uint32 { return hotkeyX11Mods(mods) }

// vkToKeysym 框架 VK 码 → X11 KeySym。
//
// 只覆盖常见可注册的键：字母 / 数字 / F1~F24 / 方向键 / 编辑键。
// 返回 0 表示不支持（调用方按注册失败处理，与 Windows 行为一致）。
func vkToKeysym(vk uint32) uintptr {
	switch {
	case vk >= 'A' && vk <= 'Z': // 字母：X11 keysym 用小写，XKeysymToKeycode 会按键盘映射换算
		return uintptr(vk + 32)
	case vk >= '0' && vk <= '9':
		return uintptr(vk)
	}
	switch vk {
	case VK_F11:
		return 0xFFC8
	case VK_ESCAPE:
		return 0xFF1B
	case VK_RETURN:
		return 0xFF0D
	case VK_TAB:
		return 0xFF09
	case VK_BACK:
		return 0xFF08
	case VK_DELETE:
		return 0xFFFF
	case VK_SPACE:
		return 0x0020
	case VK_LEFT:
		return 0xFF51
	case VK_UP:
		return 0xFF52
	case VK_RIGHT:
		return 0xFF53
	case VK_DOWN:
		return 0xFF54
	}
	if vk >= 0x70 && vk <= 0x87 { // VK_F1..VK_F24
		return uintptr(0xFFBE + (vk - 0x70))
	}
	return 0
}

// RegisterHotKey 注册系统级热键（窗口失焦 / 最小化时同样触发）。
func (p *platformWin) RegisterHotKey(mods, vk uint32, fn func()) int32 {
	x11Mu.Lock()
	dpy := x11Disp
	x11Mu.Unlock()
	if dpy == nil || fn == nil {
		return 0
	}
	ks := vkToKeysym(vk)
	if ks == 0 {
		return 0
	}
	code := dpy.KeysymToKeycode(ks)
	if code == 0 {
		return 0
	}
	xmods := x11Mods(mods)
	if !dpy.GrabKey(code, xmods) {
		return 0
	}
	hk := &x11Hotkey{keycode: code, xmods: xmods, fn: fn}
	x11HotkeyMu.Lock()
	hk.id = atomic.AddInt32(&x11HotkeySeq, 1)
	id := hk.id
	x11Hotkeys[id] = hk
	x11HotkeyByKey[hotkeyKey(xmods, code)] = hk
	x11HotkeyMu.Unlock()
	return id
}

// UnregisterHotkey 注销热键（幂等）。
func (p *platformWin) UnregisterHotkey(id int32) {
	x11HotkeyMu.Lock()
	hk := x11Hotkeys[id]
	delete(x11Hotkeys, id)
	if hk != nil {
		delete(x11HotkeyByKey, hotkeyKey(hk.xmods, hk.keycode))
	}
	x11HotkeyMu.Unlock()
	if hk == nil {
		return
	}
	x11Mu.Lock()
	dpy := x11Disp
	x11Mu.Unlock()
	if dpy != nil {
		dpy.UngrabKey(hk.keycode, hk.xmods)
	}
}

// hotkeyKey 事件匹配键：修饰键放高位，键码放低位。
func hotkeyKey(xmods uint32, code int) uint32 {
	return xmods<<16 | uint32(code&0xFFFF)
}

// x11DispatchHotkey 尝试把一次 KeyPress 当作全局热键处理。
//
// 只在「根窗口收到 KeyPress」时匹配：XGrabKey 抓的是根窗口，
// 事件里的 window 字段就是根窗口；普通窗口内的键盘输入不应该被吞。
func x11DispatchHotkey(ev *x11.Event) bool {
	if ev.Type() != x11.KeyPress {
		return false
	}
	state := ev.State() & (x11.ShiftMask | x11.ControlMask | x11.Mod1Mask | x11.Mod4Mask)
	code := int(ev.Keycode())
	x11HotkeyMu.Lock()
	hk := x11HotkeyByKey[hotkeyKey(state, code)]
	x11HotkeyMu.Unlock()
	if hk == nil || hk.fn == nil {
		return false
	}
	hk.fn()
	return true
}

// x11UngrabAllHotkeys 窗口销毁时清理所有抓取（避免残留全局热键）。
func x11UngrabAllHotkeys() {
	x11HotkeyMu.Lock()
	list := make([]*x11Hotkey, 0, len(x11Hotkeys))
	for _, hk := range x11Hotkeys {
		list = append(list, hk)
	}
	x11Hotkeys = map[int32]*x11Hotkey{}
	x11HotkeyByKey = map[uint32]*x11Hotkey{}
	x11HotkeyMu.Unlock()
	x11Mu.Lock()
	dpy := x11Disp
	x11Mu.Unlock()
	if dpy == nil {
		return
	}
	for _, hk := range list {
		dpy.UngrabKey(hk.keycode, hk.xmods)
	}
}
