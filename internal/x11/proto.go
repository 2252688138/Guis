//go:build (linux || freebsd) && !android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// 本文件补齐「系统集成」所需的 Xlib 绑定：全局热键（XGrabKey）、
// 系统托盘（XEmbed 协议）、文件拖入（XDND 协议）、属性与选择读写。
//
// 与 x11.go 一样零 cgo：purego 动态解析符号，结构体一律按固定偏移读写字节，
// 不做 Go 结构体映射（Xlib 的 XEvent 是 C 联合体，对齐规则容易踩坑）。
package x11

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// Atom X11 原子（与既有 API 一致，直接用 uintptr 表示）。
type Atom = uintptr

// 修饰键掩码（X.h）。
const (
	ShiftMask   = 1 << 0
	LockMask    = 1 << 1
	ControlMask = 1 << 2
	Mod1Mask    = 1 << 3 // 通常是 Alt
	Mod2Mask    = 1 << 4 // 通常是 NumLock
	Mod4Mask    = 1 << 6 // 通常是 Super / Win
	AnyModifier = 1 << 15
)

// 额外事件掩码。
const (
	NoEventMask          = 0
	KeymapStateMask      = 1 << 14
	VisibilityChangeMask = 1 << 16
	ResizeRedirectMask   = 1 << 18
	SubstructureNotify   = 1 << 19
	SubstructureRedirect = 1 << 20
)

// 预定义 Atom 序号（X.h）。
const (
	atomAtom           = 4
	atomString         = 31
	atomCardinal       = 6
	atomXATOM          = 68
	xaWindowProperty   = 33
	propModeReplace    = 0
	anyPropertyType    = 0
	successStatus      = 0
	propClientMessage  = 33
	propSelectionNotif = 31
	propPropertyNotify = 28
)

// 额外绑定的 Xlib 函数（Load 成功后在 bindProto 里解析）。
var (
	xGrabKey           func(uintptr, int, uint32, uintptr, int, int, int) int
	xUngrabKey         func(uintptr, int, uint32, uintptr) int
	xKeysymToKeycode   func(uintptr, uintptr) byte
	xCreateWindow      func(uintptr, uintptr, int, int, uint, uint, uint, int, uint, uintptr, uintptr, unsafe.Pointer) uintptr
	xSetWindowBackground func(uintptr, uintptr, uintptr) int
	xClearWindow       func(uintptr, uintptr) int
	xSetSelectionOwner func(uintptr, uintptr, uintptr, uintptr) int
	xGetSelectionOwner func(uintptr, uintptr) uintptr
	xConvertSelection  func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr) int
	xGetWindowProperty func(uintptr, uintptr, uintptr, int64, int64, int, uintptr, *uintptr, *int, *uintptr, *uintptr, *uintptr) int
	xFree              func(uintptr) int
	xRaiseWindow       func(uintptr, uintptr) int
	xChangeSaveSet     func(uintptr, uintptr, int) int
	xDeleteProperty    func(uintptr, uintptr, uintptr) int
	xGetAtomName       func(uintptr, uintptr) *byte
	xTranslateCoords   func(uintptr, uintptr, uintptr, int, int, *int, *int, *uintptr) int
)

// bindProto 解析系统集成所需的额外符号（由 Load 调用，失败时保持 nil）。
func bindProto(h uintptr) {
	bind := func(dst any, name string) {
		if addr, err := purego.Dlsym(h, name); err == nil && addr != 0 {
			purego.RegisterFunc(dst, addr)
		}
	}
	bind(&xGrabKey, "XGrabKey")
	bind(&xUngrabKey, "XUngrabKey")
	bind(&xKeysymToKeycode, "XKeysymToKeycode")
	bind(&xCreateWindow, "XCreateWindow")
	bind(&xSetWindowBackground, "XSetWindowBackground")
	bind(&xClearWindow, "XClearWindow")
	bind(&xSetSelectionOwner, "XSetSelectionOwner")
	bind(&xGetSelectionOwner, "XGetSelectionOwner")
	bind(&xConvertSelection, "XConvertSelection")
	bind(&xGetWindowProperty, "XGetWindowProperty")
	bind(&xFree, "XFree")
	bind(&xRaiseWindow, "XRaiseWindow")
	bind(&xChangeSaveSet, "XChangeSaveSet")
	bind(&xDeleteProperty, "XDeleteProperty")
	bind(&xGetAtomName, "XGetAtomName")
	bind(&xTranslateCoords, "XTranslateCoordinates")
}

// ---- 全局热键 ----

// GrabKey 抓取一个全局热键（含 CapsLock / NumLock 组合，避免开着大写锁就失效）。
//
// keycode 为硬件键码，mods 为 X11 修饰键掩码。返回 Ok=false 表示已被其它程序占用。
func (d *Display) GrabKey(keycode int, mods uint32) bool {
	if xGrabKey == nil || d == nil || d.p == 0 || keycode <= 0 {
		return false
	}
	ok := false
	// 依次抓 无锁 / CapsLock / NumLock / 两者 四种状态：桌面环境下这些锁键状态
	// 会参与状态匹配，只抓一个组合会导致「开大写锁热键失灵」。
	for _, extra := range []uint32{0, LockMask, Mod2Mask, LockMask | Mod2Mask} {
		if xGrabKey(d.p, keycode, mods|extra, uintptr(d.root), 0, 1 /*GrabModeAsync*/, 1) != 0 {
			// XGrabKey 无返回值语义（BadAccess 走错误处理），这里用 GetSelectionOwner 之类无法判定；
			// 统一视为成功，真正失败（被占用）时 X 会异步报 BadAccess。
			ok = true
		}
	}
	xSync(d.p, 0)
	return ok
}

// UngrabKey 释放热键（mods 传注册时的原始值即可，内部会释放四种锁组合）。
func (d *Display) UngrabKey(keycode int, mods uint32) {
	if xUngrabKey == nil || d == nil || d.p == 0 || keycode <= 0 {
		return
	}
	for _, extra := range []uint32{0, LockMask, Mod2Mask, LockMask | Mod2Mask} {
		xUngrabKey(d.p, keycode, mods|extra, uintptr(d.root))
	}
	xSync(d.p, 0)
}

// KeysymToKeycode KeySym → 硬件键码。
func (d *Display) KeysymToKeycode(keysym uintptr) int {
	if xKeysymToKeycode == nil || d == nil || d.p == 0 {
		return 0
	}
	return int(xKeysymToKeycode(d.p, keysym))
}

// ---- 托盘窗口（XEmbed）----

// CreateSimpleChild 创建一个 override-redirect 之类的小窗口（托盘图标 / 菜单用）。
//
// parent 传 Root() 即为顶层窗口；overrideRedirect=true 时窗口管理器不接管
// （不画标题栏、不参与平铺），这正是托盘图标需要的行为。
// bgPixel 为背景像素（AllocColor 生成）。
func (d *Display) CreateSimpleChild(parent Window, x, y int, w, h int, overrideRedirect bool, bgPixel uintptr) Window {
	if xCreateWindow == nil || d == nil || d.p == 0 {
		return 0
	}
	// XSetWindowAttributes 按 64 位对齐的实际字段偏移：
	// background_pixmap 0 / background_pixel 8 / border_pixmap 16 / border_pixel 24 /
	// bit_gravity 32 / win_gravity 36 / backing_store 40 / backing_planes 48 /
	// backing_pixel 56 / save_under 64 / event_mask 72 / do_not_propagate_mask 80 /
	// override_redirect 88 / cursor 96。只填用到的两项，其余保持 0。
	var attrs [104]byte
	setLong(attrs[:], 8, uint64(bgPixel))
	mask := uintptr(1 << 1) // CWBackPixel
	if overrideRedirect {
		setLong(attrs[:], 88, 1)
		mask |= 1 << 9 // CWOverrideRedirect
	}
	w2 := xCreateWindow(d.p, uintptr(parent), x, y, uint(w), uint(h), 0, 0,
		1 /*InputOutput*/, 0, mask, unsafe.Pointer(&attrs[0]))
	if w2 == 0 {
		return 0
	}
	return Window(w2)
}

// SetWindowBackground 设置窗口背景像素。
func (d *Display) SetWindowBackground(w Window, pixel uintptr) {
	if xSetWindowBackground == nil || d == nil || d.p == 0 {
		return
	}
	xSetWindowBackground(d.p, uintptr(w), pixel)
}

// ClearWindow 用背景色填满窗口（重画图标底色）。
func (d *Display) ClearWindow(w Window) {
	if xClearWindow == nil || d == nil || d.p == 0 {
		return
	}
	xClearWindow(d.p, uintptr(w))
}

// RaiseWindow 把窗口抬到最上层（托盘菜单弹出前用）。
func (d *Display) RaiseWindow(w Window) {
	if xRaiseWindow == nil || d == nil || d.p == 0 {
		return
	}
	xRaiseWindow(d.p, uintptr(w))
}

// AllocColor 用 RGB 分配一个像素值（信息色，非真彩色时由 X 服务器匹配最接近色）。
func AllocColor(r, g, b uint8) uintptr {
	// 真彩色（绝大多数现代 X 服务器）下像素值即 0xRRGGBB。
	return uintptr(uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}

// ---- 选择（selection）与属性 ----

// SetSelectionOwner 成为某个选择的所有者（托盘协议 `_NET_SYSTEM_TRAY_Sn` 用）。
func (d *Display) SetSelectionOwner(selection Atom, owner Window) {
	if xSetSelectionOwner == nil || d == nil || d.p == 0 {
		return
	}
	xSetSelectionOwner(d.p, uintptr(selection), uintptr(owner), 0 /*CurrentTime*/)
	xFlush(d.p)
}

// SelectInput 订阅窗口事件（0 表示取消全部订阅）。
func (d *Display) SelectInput(w Window, mask uintptr) {
	if xSelectInput == nil || d == nil || d.p == 0 {
		return
	}
	xSelectInput(d.p, uintptr(w), mask)
}

// ChangeProp32 把一个 32 位整数数组写入窗口属性（_XEMBED_INFO / XdndAware 等）。
func (d *Display) ChangeProp32(w Window, prop, typ Atom, vals []uint32) {
	if xChangeProperty == nil || d == nil || d.p == 0 || len(vals) == 0 {
		return
	}
	buf := make([]uint32, len(vals))
	copy(buf, vals)
	xChangeProperty(d.p, uintptr(w), uintptr(prop), uintptr(typ), 32, propModeReplace,
		(*byte)(unsafe.Pointer(&buf[0])), len(buf))
	xFlush(d.p)
}

// TranslateCoords 把 src 窗口坐标系里的点换算到 dst 窗口坐标系（XDND 落点用）。
func (d *Display) TranslateCoords(src, dst Window, x, y int) (int, int) {
	if xTranslateCoords == nil || d == nil || d.p == 0 {
		return x, y
	}
	var dx, dy int
	var child uintptr
	if xTranslateCoords(d.p, uintptr(src), uintptr(dst), x, y, &dx, &dy, &child) == 0 {
		return x, y
	}
	return dx, dy
}

// SelectionOwner 查询某个选择当前的所有者窗口（无主返回 0）。
func (d *Display) SelectionOwner(selection Atom) Window {
	if xGetSelectionOwner == nil || d == nil || d.p == 0 {
		return 0
	}
	return Window(xGetSelectionOwner(d.p, uintptr(selection)))
}

// ConvertSelection 请求某个选择的指定目标格式（XDND 取 text/uri-list 用）。
func (d *Display) ConvertSelection(selection, target, property Atom, requestor Window) {
	if xConvertSelection == nil || d == nil || d.p == 0 {
		return
	}
	xConvertSelection(d.p, uintptr(selection), uintptr(target), uintptr(property), uintptr(requestor), 0)
	xFlush(d.p)
}

// GetStringProperty 读取窗口的字符串属性（读不到返回空串）。
//
// X11 的返回值是 Xlib 自己 malloc 的缓冲区，必须用 XFree 释放，否则每读一次漏一次。
func (d *Display) GetStringProperty(w Window, prop Atom) string {
	if xGetWindowProperty == nil || d == nil || d.p == 0 {
		return ""
	}
	var actualType uintptr
	var actualFormat int
	var nitems, bytesAfter uintptr
	var data uintptr
	if xGetWindowProperty(d.p, uintptr(w), uintptr(prop), 0, 1<<20, 1 /*True*/, anyPropertyType,
		&actualType, &actualFormat, &nitems, &bytesAfter, &data) != successStatus {
		return ""
	}
	if data == 0 || nitems == 0 {
		if data != 0 && xFree != nil {
			xFree(data)
		}
		return ""
	}
	var out string
	switch actualFormat {
	case 8:
		out = goBytes(data, int(nitems))
	case 32:
		// 32 位数组：逐字取低字节（常见于 atom 列表，不当作字符串用）
		buf := unsafe.Slice((*uint32)(unsafe.Pointer(data)), int(nitems))
		bs := make([]byte, 0, len(buf))
		for _, v := range buf {
			bs = append(bs, byte(v))
		}
		out = string(bs)
	}
	if xFree != nil {
		xFree(data)
	}
	return out
}

// SendClientMessage 给目标窗口发一条 32 位 ClientMessage（XEmbed / XDND 协议用）。
func (d *Display) SendClientMessage(target Window, messageType Atom, mask uintptr, data [5]int64) {
	if xSendEvent == nil || d == nil || d.p == 0 {
		return
	}
	ev := BuildClientMessage(uintptr(target), uintptr(messageType), data)
	xSendEvent(d.p, uintptr(target), 0, mask, unsafe.Pointer(&ev[0]))
	xFlush(d.p)
}

// ---- 事件字段（补充 event.go 未覆盖的部分）----

// AtomOf 事件的 Atom 字段（SelectionNotify / PropertyNotify / ClientMessage 共用偏移 40）。
func (e *Event) AtomOf() uintptr { return uintptr(at(e.Raw[:], 40)) }

// WindowOf 事件关联的窗口（偏移 32；ConfigureNotify 也是这个偏移）。
func (e *Event) WindowOf() uintptr { return uintptr(at(e.Raw[:], 32)) }

// Requestor / Target / Property 读 XSelectionEvent（type=31）。
func (e *Event) SelRequestor() uintptr { return uintptr(at(e.Raw[:], 32)) }
func (e *Event) SelSelection() uintptr { return uintptr(at(e.Raw[:], 40)) }
func (e *Event) SelTarget() uintptr    { return uintptr(at(e.Raw[:], 48)) }
func (e *Event) SelProperty() uintptr  { return uintptr(at(e.Raw[:], 56)) }

// TimeOf 事件时间戳（XDND 回执需要）。
func (e *Event) TimeOf() uintptr { return uintptr(at(e.Raw[:], 48)) }

// IsClientMessage 是否 ClientMessage 事件。
func (e *Event) IsClientMessage() bool { return e.Type() == propClientMessage }

// IsSelectionNotify 是否 SelectionNotify 事件。
func (e *Event) IsSelectionNotify() bool { return e.Type() == propSelectionNotif }

// goBytes 把 Xlib 返回的缓冲区转成 Go 字节切片（复制，调用方随后可 XFree）。
func goBytes(ptr uintptr, n int) string {
	if ptr == 0 || n <= 0 {
		return ""
	}
	bs := make([]byte, n)
	copy(bs, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), n))
	// 去掉尾部 NUL（X 返回的字符串常带结尾 0）
	for len(bs) > 0 && bs[len(bs)-1] == 0 {
		bs = bs[:len(bs)-1]
	}
	return string(bs)
}

// SelectionNotifyEventType 供外部断言用（避免魔法数字）。
const SelectionNotifyEventType = propSelectionNotif
