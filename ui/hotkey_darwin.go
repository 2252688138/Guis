//go:build darwin && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// macOS 全局热键：CGEventTap（CoreGraphics）
//
// 与 Windows 的 RegisterHotKey 不同，事件 tap 是**监听**而不是独占：
// 热键触发后按键仍会传给前台 App（ListenOnly），也不会因为组合被别的程序
// 占着就收不到。代价是需要「输入监控 / 辅助功能」授权，未授权时
// CGEventTapCreate 返回 nil → 注册失败（返回 0），业务侧按不支持降级。
package ui

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// CoreGraphics 常量。
const (
	cgSessionEventTap    = 1
	cgHeadInsertEventTap = 0
	cgTapListenOnly      = 1
	cgEventKeyDown       = 10
	cgKeyboardKeycode    = 9 // kCGKeyboardEventKeycode

	cgFlagShift   = 0x00020000
	cgFlagControl = 0x00040000
	cgFlagOption  = 0x00080000
	cgFlagCommand = 0x00100000
)

var (
	cgOnce      sync.Once
	cgReady     bool
	cgTapCreate func(tap, place, options uint32, mask uint64, cb, refcon uintptr) uintptr
	cgSrcCreate func(port uintptr) uintptr
	cgLoopAdd   func(loop, src, mode uintptr)
	cgLoopMain  func() uintptr
	cgEvFlags   func(ev uintptr) uint64
	cgEvInt     func(ev uintptr, field uint32) int64
	// cgModeCommon kCFRunLoopCommonModes 的 CFStringRef 值（不是函数，得解引用取）
	cgModeCommon uintptr
)

// loadCG 动态加载 CoreGraphics 符号（失败后重复返回 false）。
func loadCG() bool {
	cgOnce.Do(func() {
		var h uintptr
		for _, name := range []string{
			"/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices",
			"/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics",
		} {
			if lib, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
				h = lib
				break
			}
		}
		if h == 0 {
			return
		}
		bind := func(dst any, name string) bool {
			if addr, err := purego.Dlsym(h, name); err == nil && addr != 0 {
				purego.RegisterFunc(dst, addr)
				return true
			}
			return false
		}
		ok := bind(&cgTapCreate, "CGEventTapCreate") &&
			bind(&cgSrcCreate, "CFMachPortCreateRunLoopSource") &&
			bind(&cgLoopAdd, "CFRunLoopAddSource") &&
			bind(&cgLoopMain, "CFRunLoopGetMain") &&
			bind(&cgEvFlags, "CGEventGetFlags") &&
			bind(&cgEvInt, "CGEventGetIntegerValueField")
		if !ok {
			return
		}
		// kCFRunLoopCommonModes 是导出的 CFStringRef **变量**：
		// Dlsym 给的是变量地址，真正的 CFStringRef 存在那里，必须解引用。
		if addr, err := purego.Dlsym(h, "kCFRunLoopCommonModes"); err == nil && addr != 0 {
			cgModeCommon = *(*uintptr)(unsafe.Pointer(addr))
		}
		cgReady = cgModeCommon != 0
	})
	return cgReady
}

// darwinHotkey 一条已注册热键。
type darwinHotkey struct {
	id    int32
	code  int64
	flags uint64
	fn    func()
}

var (
	darwinHotkeyMu  sync.Mutex
	darwinHotkeySeq int32
	darwinHotkeys   = map[int32]*darwinHotkey{}
	darwinTap       uintptr
	darwinTapCB     uintptr // 回调必须常驻，否则 C 侧函数指针失效
)

// macKeycodes ANSI 布局的字母键码。
var macKeycodes = map[uint32]int64{
	'A': 0, 'S': 1, 'D': 2, 'F': 3, 'H': 4, 'G': 5, 'Z': 6, 'X': 7, 'C': 8, 'V': 9,
	'B': 11, 'Q': 12, 'W': 13, 'E': 14, 'R': 15, 'Y': 16, 'T': 17, 'O': 31, 'U': 32,
	'I': 34, 'P': 35, 'L': 37, 'J': 38, 'K': 40, 'N': 45, 'M': 46,
}

// macDigitCodes 主键盘数字键码。
var macDigitCodes = map[uint32]int64{
	'1': 18, '2': 19, '3': 20, '4': 21, '6': 22, '5': 23,
	'9': 25, '7': 26, '8': 28, '0': 29,
}

// vkToMacKeycode 框架 VK → macOS 虚拟键码。
func vkToMacKeycode(vk uint32) (int64, bool) {
	if c, ok := macKeycodes[vk]; ok {
		return c, true
	}
	if c, ok := macDigitCodes[vk]; ok {
		return c, true
	}
	switch vk {
	case VK_RETURN:
		return 36, true
	case VK_TAB:
		return 48, true
	case VK_SPACE:
		return 49, true
	case VK_BACK:
		return 51, true
	case VK_ESCAPE:
		return 53, true
	case VK_LEFT:
		return 123, true
	case VK_RIGHT:
		return 124, true
	case VK_DOWN:
		return 125, true
	case VK_UP:
		return 126, true
	case VK_DELETE:
		return 117, true
	}
	if vk >= 0x70 && vk <= 0x7B { // VK_F1..VK_F12
		f := []int64{122, 120, 99, 118, 96, 97, 98, 100, 101, 109, 103, 111}
		return f[vk-0x70], true
	}
	return 0, false
}

// darwinMods 框架 Hot* 修饰键 → CGEventFlags（实现见 hotkeymods.go）。
func darwinMods(mods uint32) uint64 { return hotkeyMacFlags(mods) }

// registerDarwinHotkey 注册全局热键；返回 id（0 = 失败）。
func registerDarwinHotkey(mods, vk uint32, fn func()) int32 {
	if fn == nil || !loadCG() {
		return 0
	}
	code, ok := vkToMacKeycode(vk)
	if !ok {
		return 0
	}
	darwinHotkeyMu.Lock()
	if darwinTap == 0 {
		darwinHotkeyMu.Unlock()
		startDarwinTap()
		darwinHotkeyMu.Lock()
	}
	if darwinTap == 0 {
		darwinHotkeyMu.Unlock()
		return 0
	}
	hk := &darwinHotkey{id: atomic.AddInt32(&darwinHotkeySeq, 1), code: code, flags: darwinMods(mods), fn: fn}
	darwinHotkeys[hk.id] = hk
	id := hk.id
	darwinHotkeyMu.Unlock()
	return id
}

// startDarwinTap 建立（仅一次）事件 tap 并挂到主 run loop。
func startDarwinTap() {
	cb := purego.NewCallback(func(proxy, typ, ev, refcon uintptr) uintptr {
		if int(typ) == cgEventKeyDown && ev != 0 {
			code := cgEvInt(ev, cgKeyboardKeycode)
			flags := cgEvFlags(ev) & (cgFlagShift | cgFlagControl | cgFlagOption | cgFlagCommand)
			darwinHotkeyMu.Lock()
			var hit *darwinHotkey
			for _, hk := range darwinHotkeys {
				if hk.code == code && hk.flags == flags {
					hit = hk
					break
				}
			}
			darwinHotkeyMu.Unlock()
			if hit != nil && hit.fn != nil {
				hit.fn()
			}
		}
		return ev // ListenOnly：原样返回事件
	})
	mask := uint64(1) << cgEventKeyDown
	tap := cgTapCreate(cgSessionEventTap, cgHeadInsertEventTap, cgTapListenOnly, mask, cb, 0)
	if tap == 0 {
		return
	}
	src := cgSrcCreate(tap)
	if src == 0 {
		return
	}
	cgLoopAdd(cgLoopMain(), src, cgModeCommon)
	darwinHotkeyMu.Lock()
	darwinTap = tap
	darwinTapCB = cb
	darwinHotkeyMu.Unlock()
}

// unregisterDarwinHotkey 摘掉热键表项（tap 常驻，不单独销毁）。
func unregisterDarwinHotkey(id int32) {
	darwinHotkeyMu.Lock()
	delete(darwinHotkeys, id)
	darwinHotkeyMu.Unlock()
}
