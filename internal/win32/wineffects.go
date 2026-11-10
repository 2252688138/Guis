//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"syscall"
	"unsafe"
)

// 窗口特效：透明度、鼠标穿透、置底、隐藏任务栏按钮、防截屏、深色标题栏、材质。
//
// 全部是「改扩展样式 + SetWindowPos / DwmSetWindowAttribute」这一类轻量调用，
// 不支持时静默返回（例如 Win10 之前的 DWM 属性会返回非 0 HRESULT）。

var (
	dwmapiL = syscall.NewLazyDLL("dwmapi.dll")

	procSetLayeredWindowAttributes = user32L.NewProc("SetLayeredWindowAttributes")
	procGetWindowLongPtrW          = user32L.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW          = user32L.NewProc("SetWindowLongPtrW")
	procSetWindowDisplayAffinity   = user32L.NewProc("SetWindowDisplayAffinity")
	procSetWindowPosW              = user32L.NewProc("SetWindowPos")
	procShowWindowAsync            = user32L.NewProc("ShowWindowAsync")
	procSetForegroundWindow        = user32L.NewProc("SetForegroundWindow")
	procBringWindowToTop           = user32L.NewProc("BringWindowToTop")
	procFlashWindowEx              = user32L.NewProc("FlashWindowEx")
	procDwmSetWindowAttribute      = dwmapiL.NewProc("DwmSetWindowAttribute")
	procDwmExtendFrameIntoClient   = dwmapiL.NewProc("DwmExtendFrameIntoClientArea")
)

const (
	// GWL_EXSTYLE 的 uintptr 形式（-20）。
	gwlExStyle = ^uintptr(19)

	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExToolWindow  = 0x00000080
	wsExAppWindow   = 0x00040000

	lwaAlpha = 0x00000002

	hwndBottom = 1

	// SetWindowDisplayAffinity
	wdaNone               = 0x00000000
	wdaExcludeFromCapture = 0x00000011

	// DwmSetWindowAttribute
	dwmwaUseImmersiveDarkMode = 20
	dwmwaSystemBackdropType   = 38

	// DWM_SYSTEMBACKDROP_TYPE
	dwmBackdropMica   = 2
	dwmBackdropAcrylic = 3
	dwmBackdropTabbed = 4
)

// BackdropKindSet 材质类型（与 ui.BackdropKind 对应）。
type BackdropKindSet int

const (
	// BackdropOff 关闭材质。
	BackdropOff BackdropKindSet = iota
	// BackdropMica Mica。
	BackdropMica
	// BackdropAcrylic 亚克力。
	BackdropAcrylic
	// BackdropTabbed Mica Alt。
	BackdropTabbed
)

// SetWindowOpacity 设置整窗不透明度（0~1，1 = 不透明）。
//
// 走 WS_EX_LAYERED + SetLayeredWindowAttributes(LWA_ALPHA)：DWM 合成下这是硬件
// 混合，不影响 GL 渲染路径。
func (w *Window) SetWindowOpacity(f float32) {
	if w == nil || w.Hwnd == 0 {
		return
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	ex := w.exStyle()
	if f >= 0.999 {
		// 完全恢复：撤掉 layered 样式，避免多层合成带来的轻微模糊
		if ex&wsExLayered != 0 && f >= 1 {
			w.setExStyle(ex &^ wsExLayered)
			w.opacity = 1
		}
		w.opacity = 1
		return
	}
	if ex&wsExLayered == 0 {
		w.setExStyle(ex | wsExLayered)
	}
	alpha := byte(f*255 + 0.5)
	procSetLayeredWindowAttributes.Call(w.Hwnd, 0, uintptr(alpha), lwaAlpha)
	w.opacity = f
}

// WindowOpacity 当前不透明度（未设置过为 1）。
func (w *Window) WindowOpacity() float32 {
	if w == nil || w.opacity == 0 {
		return 1
	}
	return w.opacity
}

// SetClickThrough 鼠标穿透：加 WS_EX_TRANSPARENT + WS_EX_LAYERED。
// 穿透后窗口收不到鼠标消息，只能靠全局热键 / 托盘恢复。
func (w *Window) SetClickThrough(on bool) {
	if w == nil || w.Hwnd == 0 {
		return
	}
	ex := w.exStyle()
	if on {
		if ex&wsExLayered == 0 {
			procSetLayeredWindowAttributes.Call(w.Hwnd, 0, 255, lwaAlpha)
		}
		w.setExStyle(ex | wsExTransparent | wsExLayered)
		w.clickThrough = true
		return
	}
	w.setExStyle(ex &^ wsExTransparent)
	w.clickThrough = false
	if w.opacity == 0 || w.opacity >= 1 {
		// 透明度不是业务设置的，就撤掉 layered，恢复普通窗口
		if w.opacity <= 0 {
			w.setExStyle(w.exStyle() &^ wsExLayered)
		}
	}
}

// ClickThrough 当前是否鼠标穿透。
func (w *Window) ClickThrough() bool { return w != nil && w.clickThrough }

// SetKeepOnBottom 把窗口压到 Z 序最底（壁纸 / 桌面挂件）。
func (w *Window) SetKeepOnBottom(on bool) {
	if w == nil || w.Hwnd == 0 {
		return
	}
	if on {
		procSetWindowPosW.Call(w.Hwnd, hwndBottom, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
		w.bottom = true
		return
	}
	// 恢复：把置顶状态重新应用一次，否则窗口会一直贴在 Z 序最底
	w.SetAlwaysOnTop(w.topmost)
	w.bottom = false
}

// SetSkipTaskbar 是否隐藏任务栏按钮（加 WS_EX_TOOLWINDOW，去掉 WS_EX_APPWINDOW）。
// Windows 要求样式变化后隐藏再显示才生效。
func (w *Window) SetSkipTaskbar(on bool) {
	if w == nil || w.Hwnd == 0 {
		return
	}
	ex := w.exStyle()
	visible, _, _ := procIsWindowVisible.Call(w.Hwnd)
	if on {
		ex = (ex | wsExToolWindow) &^ wsExAppWindow
	} else {
		ex = (ex | wsExAppWindow) &^ wsExToolWindow
	}
	w.setExStyle(ex)
	if visible != 0 {
		procShowWindowAsync.Call(w.Hwnd, SW_HIDE)
		procShowWindowAsync.Call(w.Hwnd, SW_SHOW)
	}
	w.skipTaskbar = on
}

// SkipTaskbar 当前是否隐藏了任务栏按钮。
func (w *Window) SkipTaskbar() bool { return w != nil && w.skipTaskbar }

// SetCaptureExcluded 是否禁止被截屏 / 录屏捕获。
func (w *Window) SetCaptureExcluded(on bool) {
	if w == nil || w.Hwnd == 0 || procSetWindowDisplayAffinity.Find() != nil {
		return
	}
	aff := uintptr(wdaNone)
	if on {
		aff = wdaExcludeFromCapture
	}
	procSetWindowDisplayAffinity.Call(w.Hwnd, aff)
	w.captureExcluded = on
}

// SetDarkTitleBar 系统标题栏是否使用深色（Win10 1809+ / Win11）。
func (w *Window) SetDarkTitleBar(on bool) {
	if w == nil || w.Hwnd == 0 || procDwmSetWindowAttribute.Find() != nil {
		return
	}
	var v int32
	if on {
		v = 1
	}
	procDwmSetWindowAttribute.Call(w.Hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&v)), 4)
	w.darkTitle = on
}

// SetBackdrop 设置窗口背景材质（Win11 22H2+）。
//
// 注意：材质显示在**窗口透明的地方**。自绘窗口默认整窗铺底色，要让材质可见需要
// Options.Background 用半透明色（例如 "rgba(0,0,0,0.35)"）或让根节点留出透明区。
func (w *Window) SetBackdrop(kind BackdropKindSet) {
	if w == nil || w.Hwnd == 0 || procDwmSetWindowAttribute.Find() != nil {
		return
	}
	var v int32
	switch kind {
	case BackdropMica:
		v = dwmBackdropMica
	case BackdropAcrylic:
		v = dwmBackdropAcrylic
	case BackdropTabbed:
		v = dwmBackdropTabbed
	default:
		v = 1 // DWMSBT_NONE 的取值在 1 号位（0 会被系统当成「不支持」）
	}
	procDwmSetWindowAttribute.Call(w.Hwnd, dwmwaSystemBackdropType, uintptr(unsafe.Pointer(&v)), 4)
	w.backdrop = int(kind)
}

// SetMinSize 设置最小客户区尺寸（逻辑像素，0 = 不限制）。
func (w *Window) SetMinSize(wd, ht int) {
	if w == nil {
		return
	}
	s := w.scale
	if s <= 0 {
		s = 1
	}
	w.minW = int32(float32(wd) * s)
	w.minH = int32(float32(ht) * s)
	// 当前窗口已经更小时立即纠正（否则要等用户拖一下才生效）
	rect := w.clientRect()
	if w.minW > 0 && rect.Right-rect.Left < w.minW {
		w.resizeClient(w.minW, rect.Bottom-rect.Top)
	}
}

// SetMaxSize 设置最大客户区尺寸（逻辑像素，0 = 不限制）。
func (w *Window) SetMaxSize(wd, ht int) {
	if w == nil {
		return
	}
	s := w.scale
	if s <= 0 {
		s = 1
	}
	w.maxW = int32(float32(wd) * s)
	w.maxH = int32(float32(ht) * s)
}

// Activate 把窗口拉到前台并抢焦点。
//
// Windows 有前台锁：后台进程直接 SetForegroundWindow 会被系统忽略（只闪任务栏）。
// 这里先 ShowWindowAsync(SW_RESTORE) + BringWindowToTop，再补一次 SetForegroundWindow，
// 成功率明显高于单次调用。
func (w *Window) Activate() {
	if w == nil || w.Hwnd == 0 {
		return
	}
	if ic, _, _ := procIsIconic.Call(w.Hwnd); ic != 0 {
		procShowWindowAsync.Call(w.Hwnd, SW_RESTORE)
	}
	procBringWindowToTop.Call(w.Hwnd)
	procSetForegroundWindow.Call(w.Hwnd)
}

// FlashTaskbar 闪烁任务栏图标提醒（count 次；untilFocused=true 时一直闪到获得焦点）。
func (w *Window) FlashTaskbar(count int, untilFocused bool) {
	if w == nil || w.Hwnd == 0 || procFlashWindowEx.Find() != nil {
		return
	}
	type flashwinfo struct {
		CbSize    uint32
		Hwnd      uintptr
		DwFlags   uint32
		UCount    uint32
		DwTimeout uint32
	}
	fi := flashwinfo{CbSize: uint32(unsafe.Sizeof(flashwinfo{})), Hwnd: w.Hwnd}
	const (
		flashwStop       = 0x0
		flashwAll        = 0x3
		flashwTimerNoFG  = 0xC
	)
	if count <= 0 && untilFocused {
		fi.DwFlags = flashwAll | flashwTimerNoFG
	} else if count <= 0 {
		fi.DwFlags = flashwStop
	} else {
		fi.DwFlags = flashwAll
		fi.UCount = uint32(count)
	}
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&fi)))
}

// ---- 内部辅助 ----

func (w *Window) exStyle() uintptr {
	if w == nil || w.Hwnd == 0 || procGetWindowLongPtrW.Find() != nil {
		return 0
	}
	r, _, _ := procGetWindowLongPtrW.Call(w.Hwnd, uintptr(gwlExStyle))
	return r
}

func (w *Window) setExStyle(style uintptr) {
	if w == nil || w.Hwnd == 0 || procSetWindowLongPtrW.Find() != nil {
		return
	}
	procSetWindowLongPtrW.Call(w.Hwnd, gwlExStyle, style)
}

func (w *Window) clientRect() RECT {
	var rc RECT
	if GetClientRect != nil && w != nil && w.Hwnd != 0 {
		GetClientRect(w.Hwnd, &rc)
	}
	return rc
}

// resizeClient 把客户区调整到指定物理像素尺寸（保持左上角不动）。
func (w *Window) resizeClient(pw, ph int32) {
	if w == nil || w.Hwnd == 0 {
		return
	}
	var rc RECT
	if GetWindowRect != nil {
		GetWindowRect(w.Hwnd, &rc)
	}
	cur := w.clientRect()
	frameW := (rc.Right - rc.Left) - (cur.Right - cur.Left)
	frameH := (rc.Bottom - rc.Top) - (cur.Bottom - cur.Top)
	procSetWindowPosW.Call(w.Hwnd, 0, 0, 0, uintptr(pw+frameW), uintptr(ph+frameH),
		SWP_NOMOVE|SWP_NOZORDER|SWP_NOACTIVATE)
}

// procIsWindowVisible / procIsIconic 延迟解析（user32 一定有，但保持风格一致）。
var (
	procIsWindowVisible = user32L.NewProc("IsWindowVisible")
	procIsIconic        = user32L.NewProc("IsIconic")
)