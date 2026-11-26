// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build windows

package game

import (
	"sync"
	"syscall"
	"unsafe"
)

// xinputState XINPUT_STATE（16 字节，与 Win32 定义逐字节对齐）。
type xinputState struct {
	PacketNumber uint32
	Buttons      uint16
	LeftTrigger  uint8
	RightTrigger uint8
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

type xinputVibration struct {
	LeftMotorSpeed  uint16
	RightMotorSpeed uint16
}

var (
	xinputOnce sync.Once
	xinputGet  *syscall.LazyProc
	xinputSet  *syscall.LazyProc
)

// loadXInput 依次尝试 xinput1_4 / xinput1_3 / xinput9_1_0。
func loadXInput() {
	for _, name := range []string{"xinput1_4.dll", "xinput1_3.dll", "xinput9_1_0.dll"} {
		dll := syscall.NewLazyDLL(name)
		get := dll.NewProc("XInputGetState")
		if err := get.Find(); err != nil {
			continue
		}
		xinputGet = get
		xinputSet = dll.NewProc("XInputSetState")
		return
	}
}

func padPoll(idx int) (padState, bool) {
	xinputOnce.Do(loadXInput)
	if xinputGet == nil {
		return padState{}, false
	}
	var st xinputState
	r, _, _ := xinputGet.Call(uintptr(idx), uintptr(unsafe.Pointer(&st)))
	if r != 0 { // ERROR_SUCCESS = 0
		return padState{}, false
	}
	return padState{
		buttons: st.Buttons,
		lt:      st.LeftTrigger,
		rt:      st.RightTrigger,
		lx:      normStick(st.ThumbLX),
		ly:      normStick(st.ThumbLY),
		rx:      normStick(st.ThumbRX),
		ry:      normStick(st.ThumbRY),
	}, true
}

// normStick int16 → [-1,1]（Y 轴不翻转，调用方自行决定方向）。
func normStick(v int16) float32 {
	f := float32(v)
	if f >= 0 {
		return f / 32767
	}
	return f / 32768
}

func padRumble(idx int, left, right float32) bool {
	xinputOnce.Do(loadXInput)
	if xinputSet == nil || xinputSet.Find() != nil {
		return false
	}
	v := xinputVibration{LeftMotorSpeed: clampU16(left), RightMotorSpeed: clampU16(right)}
	r, _, _ := xinputSet.Call(uintptr(idx), uintptr(unsafe.Pointer(&v)))
	return r == 0
}

func clampU16(v float32) uint16 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 65535
	}
	return uint16(v * 65535)
}
