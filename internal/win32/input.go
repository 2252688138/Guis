//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"errors"
	"syscall"
	"unsafe"
)

// 键鼠模拟输入（SendInput）。
//
// ⚠️ 这类调用等于替用户操作，安全软件会标记；框架只在业务显式调用时执行。

var procSendInput = user32L.NewProc("SendInput")

const (
	inputMouse    = 0
	inputKeyboard = 1

	keyeventfKeyup   = 0x0002
	keyeventfUnicode = 0x0004

	mouseeventfMove       = 0x0001
	mouseeventfLeftDown   = 0x0002
	mouseeventfLeftUp     = 0x0004
	mouseeventfRightDown  = 0x0008
	mouseeventfRightUp    = 0x0010
	mouseeventfMiddleDown = 0x0020
	mouseeventfMiddleUp   = 0x0040
	mouseeventfWheel      = 0x0800
	mouseeventfAbsolute   = 0x8000
)

// sendInputRaw 一条 INPUT（amd64 下 40 字节：type + 4 字节对齐 + 32 字节联合体）。
type sendInputRaw struct {
	Type uint32
	_    uint32
	Data [32]byte
}

// sendInputs 一次性提交一批输入事件。
func sendInputs(list []sendInputRaw) bool {
	if len(list) == 0 || procSendInput.Find() != nil {
		return false
	}
	r, _, _ := procSendInput.Call(uintptr(len(list)), uintptr(unsafe.Pointer(&list[0])), unsafe.Sizeof(sendInputRaw{}))
	return r == uintptr(len(list))
}

// keyEvent 生成一条键盘事件。
func keyEvent(vk uint16, scan uint16, flags uint32) sendInputRaw {
	in := sendInputRaw{Type: inputKeyboard}
	type kbd struct {
		Vk        uint16
		Scan      uint16
		Flags     uint32
		Time      uint32
		_         uint32
		ExtraInfo uintptr
	}
	k := (*kbd)(unsafe.Pointer(&in.Data[0]))
	k.Vk = vk
	k.Scan = scan
	k.Flags = flags
	return in
}

// mouseEvent 生成一条鼠标事件。
func mouseEvent(dx, dy int32, data uint32, flags uint32) sendInputRaw {
	in := sendInputRaw{Type: inputMouse}
	type ms struct {
		Dx        int32
		Dy        int32
		MouseData uint32
		Flags     uint32
		Time      uint32
		_         uint32
		ExtraInfo uintptr
	}
	m := (*ms)(unsafe.Pointer(&in.Data[0]))
	m.Dx = dx
	m.Dy = dy
	m.MouseData = data
	m.Flags = flags
	return in
}

// vkFromHotMods 把 ui.HotCtrl 这类修饰位转成虚拟键码列表。
func vkFromHotMods(mods uint32) []uint16 {
	var out []uint16
	const (
		hotAlt   = 0x0001
		hotCtrl  = 0x0002
		hotShift = 0x0004
		hotWin   = 0x0008
	)
	if mods&hotCtrl != 0 {
		out = append(out, 0x11) // VK_CONTROL
	}
	if mods&hotAlt != 0 {
		out = append(out, 0x12) // VK_MENU
	}
	if mods&hotShift != 0 {
		out = append(out, 0x10) // VK_SHIFT
	}
	if mods&hotWin != 0 {
		out = append(out, 0x5B) // VK_LWIN
	}
	return out
}

// SendKey 模拟按下并释放一个按键（mods 为 ui.HotXxx 组合）。
func SendKey(mods uint32, vk uint32) error {
	if vk == 0 {
		return errors.New("win32: 空键码")
	}
	mk := vkFromHotMods(mods)
	list := make([]sendInputRaw, 0, len(mk)*2+2)
	for _, m := range mk {
		list = append(list, keyEvent(m, 0, 0))
	}
	list = append(list, keyEvent(uint16(vk), 0, 0))
	list = append(list, keyEvent(uint16(vk), 0, keyeventfKeyup))
	for i := len(mk) - 1; i >= 0; i-- {
		list = append(list, keyEvent(mk[i], 0, keyeventfKeyup))
	}
	if !sendInputs(list) {
		return errors.New("win32: SendInput 失败")
	}
	return nil
}

// SendText 逐字符输入文本（Unicode 扫描码，绕过输入法）。
func SendText(s string) error {
	if s == "" {
		return nil
	}
	list := make([]sendInputRaw, 0, 64)
	flush := func() error {
		if len(list) == 0 {
			return nil
		}
		if !sendInputs(list) {
			return errors.New("win32: SendInput 失败")
		}
		list = list[:0]
		return nil
	}
	for _, r := range s {
		if r == '\n' {
			list = append(list, keyEvent(0x0D, 0, 0), keyEvent(0x0D, 0, keyeventfKeyup))
		} else if r < 0x10000 {
			list = append(list,
				keyEvent(0, uint16(r), keyeventfUnicode),
				keyEvent(0, uint16(r), keyeventfUnicode|keyeventfKeyup))
		} else {
			// 非 BMP：拆成 UTF-16 代理对
			hi, lo := utf16Surrogates(r)
			list = append(list,
				keyEvent(0, hi, keyeventfUnicode),
				keyEvent(0, lo, keyeventfUnicode),
				keyEvent(0, lo, keyeventfUnicode|keyeventfKeyup),
				keyEvent(0, hi, keyeventfUnicode|keyeventfKeyup))
		}
		if len(list) >= 48 {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

// utf16Surrogates 把非 BMP 码点拆成代理对。
func utf16Surrogates(r rune) (uint16, uint16) {
	v := uint32(r) - 0x10000
	return uint16(0xD800 + (v >> 10)), uint16(0xDC00 + (v & 0x3FF))
}

// MouseMoveTo 移动鼠标到屏幕绝对坐标（物理像素）。
func MouseMoveTo(x, y int) error {
	return mouseClick(x, y, -1, 0)
}

// MouseClickAt 在屏幕坐标单击（button: 0 左 / 1 右 / 2 中）。
func MouseClickAt(x, y int, button int) error {
	return mouseClick(x, y, button, 0)
}

// MouseWheel 滚轮（delta 为正向下）。
func MouseWheel(delta int) error {
	if procSendInput.Find() != nil {
		return errors.New("win32: SendInput 不可用")
	}
	if !sendInputs([]sendInputRaw{mouseEvent(0, 0, uint32(int32(delta)), mouseeventfWheel)}) {
		return errors.New("win32: SendInput 失败")
	}
	return nil
}

// mouseClick 移动（button<0 时只移动）+ 点击。
func mouseClick(x, y int, button int, _ int) error {
	if procSendInput.Find() != nil {
		return errors.New("win32: SendInput 不可用")
	}
	sw, _, _ := procGetSystemMetrics.Call(uintptr(smCxScreen))
	sh, _, _ := procGetSystemMetrics.Call(uintptr(smCyScreen))
	if int32(sw) <= 0 || int32(sh) <= 0 {
		return errors.New("win32: 取屏幕尺寸失败")
	}
	ax := int32(int64(x) * 65535 / int64(int32(sw)-1))
	ay := int32(int64(y) * 65535 / int64(int32(sh)-1))
	list := []sendInputRaw{mouseEvent(ax, ay, 0, mouseeventfMove|mouseeventfAbsolute)}
	var down, up uint32
	switch button {
	case 0:
		down, up = mouseeventfLeftDown, mouseeventfLeftUp
	case 1:
		down, up = mouseeventfRightDown, mouseeventfRightUp
	case 2:
		down, up = mouseeventfMiddleDown, mouseeventfMiddleUp
	default:
		if !sendInputs(list) {
			return errors.New("win32: SendInput 失败")
		}
		return nil
	}
	list = append(list, mouseEvent(0, 0, 0, down), mouseEvent(0, 0, 0, up))
	if !sendInputs(list) {
		return errors.New("win32: SendInput 失败")
	}
	return nil
}

// 保证 syscall 被引用（部分构建下只在错误分支使用）。
var _ = syscall.GetCurrentProcess
