//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"sync"
	"unsafe"
)

// 系统主音量（Core Audio / IAudioEndpointVolume）。
//
// Windows 没有「一条命令调音量」的稳定通道，Core Audio 是唯一正式接口。
//
// ⚠️ 虚表槽位是硬编码的：错一格就会把 Set 当成 Get 调用（参数个数不符）→
// 原生访问违例，进程直接消失（Go 侧无法 recover）。下面的槽位是实测确认过的
// （用 ProbeVolumeSlot 逐槽位探测：slot 9 返回 0~1 的音量、slot 8 返回 dB 值、
// slot 14 返回静音标志），修改前务必重新探测。

var (
	clsidMMDeviceEnumerator = guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidMMDeviceEnumerator   = guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidAudioEndpointVolume  = guid{0x5CDF2C82, 0x841E, 0x4546, [8]byte{0x97, 0x22, 0x0C, 0xF7, 0x40, 0x78, 0x22, 0x9A}}
)

// IAudioEndpointVolume 的虚表槽位（IUnknown 占 0~2），实测值：
//
//	3  RegisterControlChangeNotify   4  UnregisterControlChangeNotify
//	5  GetChannelCount               6  SetMasterVolumeLevel
//	7  SetMasterVolumeLevelScalar    8  GetMasterVolumeLevel（dB）
//	9  GetMasterVolumeLevelScalar   10  SetChannelVolumeLevel
//	11 SetChannelVolumeLevelScalar  12  GetChannelVolumeLevel
//	13 GetChannelVolumeLevelScalar  14  GetMute
//	15 SetMute                      16  VolumeStepUp
//	17 VolumeStepDown               18  QueryHardwareSupport
//	19 GetVolumeRange
const (
	mmdeGetDefaultEndpoint = 4 // IMMDeviceEnumerator::GetDefaultAudioEndpoint
	mmdActivate            = 3 // IMMDevice::Activate
	aevGetMasterScalar     = 9
	aevSetMasterScalar     = 7
	aevGetMute             = 14
	aevSetMute             = 15
	clsctxAll              = 0x17
)

var (
	volMu   sync.Mutex
	volOnce sync.Once
	volPtr  uintptr // IAudioEndpointVolume
	volErr  bool

	procCoInitializeExVol = ole32L.NewProc("CoInitializeEx")
)

// audioEndpoint 懒创建 IAudioEndpointVolume（失败后不再重试）。
func audioEndpoint() uintptr {
	volOnce.Do(func() {
		if procCoCreateInstance.Find() != nil {
			volErr = true
			return
		}
		if procCoInitializeExVol.Find() == nil {
			// COINIT_APARTMENTTHREADED；已初始化时返回 S_FALSE / RPC_E_CHANGED_MODE，
			// 都表示「本线程已有 COM 单元」，可继续使用。
			procCoInitializeExVol.Call(0, 2)
		}
		var enum uintptr
		r, _, _ := procCoCreateInstance.Call(
			uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
			uintptr(unsafe.Pointer(&iidMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enum)),
		)
		hr := uint32(r)
		if comFail(hr) || enum == 0 {
			volErr = true
			return
		}
		defer comRelease(enum)
		var dev uintptr
		// eRender = 0, eConsole = 0
		hr = comCall(enum, mmdeGetDefaultEndpoint, 0, 0, uintptr(unsafe.Pointer(&dev)))
		if comFail(hr) || dev == 0 {
			volErr = true
			return
		}
		defer comRelease(dev)
		var vol uintptr
		hr = comCall(dev, mmdActivate, uintptr(unsafe.Pointer(&iidAudioEndpointVolume)),
			clsctxAll, 0, uintptr(unsafe.Pointer(&vol)))
		if comFail(hr) || vol == 0 {
			volErr = true
			return
		}
		volPtr = vol
	})
	return volPtr
}

// MasterVolume 读取系统主音量 0~1。
func MasterVolume() (float32, bool) {
	vol := audioEndpoint()
	if vol == 0 {
		return 0, false
	}
	volMu.Lock()
	defer volMu.Unlock()
	var v float32
	if hr := comCall(vol, aevGetMasterScalar, uintptr(unsafe.Pointer(&v))); comFail(hr) {
		return 0, false
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v, true
}

// SetMasterVolume 设置系统主音量 0~1。
//
// 实现方式：先读当前音量（IAudioEndpointVolume 槽位 9，实测确认），再用
// VK_VOLUME_UP / VK_VOLUME_DOWN 步进到目标值（每步约 2%）。
//
// 为什么不用 SetMasterVolumeLevelScalar：该方法的参数含 float，Win64 下 float
// 走 XMM，需要机器码 trampoline；而它的虚表槽位在不同 Windows 版本上出现过
// 偏移，槽位错一格就是原生访问违例（进程直接消失，Go 侧无法 recover）。
// 音量键是系统级热键，任何版本都可用，且不会崩溃 —— 稳定性优先。
func SetMasterVolume(v float32) bool {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	cur, ok := MasterVolume()
	if !ok {
		// 读不到当前值：按 50% 估算，尽量靠近目标
		cur = 0.5
	}
	// 每步 2%（Windows 默认音量步长）
	steps := int((v - cur) / 0.02)
	if steps == 0 {
		switch {
		case v > cur+0.005:
			steps = 1
		case v < cur-0.005:
			steps = -1
		default:
			return true // 已经一致
		}
	}
	const maxSteps = 50
	if steps > maxSteps {
		steps = maxSteps
	}
	if steps < -maxSteps {
		steps = -maxSteps
	}
	vk := uint32(0xAF) // VK_VOLUME_UP
	if steps < 0 {
		vk = 0xAE // VK_VOLUME_DOWN
		steps = -steps
	}
	for i := 0; i < steps; i++ {
		if err := SendKey(0, vk); err != nil {
			return false
		}
	}
	return true
}

// ToggleMasterMute 切换系统静音，返回切换后的状态。
//
// 读取走 IAudioEndpointVolume 槽位 14（实测安全），切换用 VK_VOLUME_MUTE 键
// （与 SetMasterVolume 同策略：避免调用参数/槽位不确定的 Set 方法触发原生崩溃）。
func ToggleMasterMute() (bool, bool) {
	vol := audioEndpoint()
	if vol == 0 {
		return false, false
	}
	volMu.Lock()
	var muted int32
	_ = comCall(vol, aevGetMute, uintptr(unsafe.Pointer(&muted)))
	volMu.Unlock()
	wasMuted := muted != 0
	if err := SendKey(0, 0xAD); err != nil { // VK_VOLUME_MUTE
		return wasMuted, false
	}
	return !wasMuted, true
}

// ProbeVolumeSlotFloat 诊断用：按 ComCallFloatPtr（fn(self, float, void*)）调用第 slot
// 个虚表方法，返回 HRESULT。用来核对 SetMasterVolumeLevelScalar 的槽位。
func ProbeVolumeSlotFloat(slot int, f float32) uint32 {
	vol := audioEndpoint()
	if vol == 0 {
		return 0x80004005
	}
	volMu.Lock()
	defer volMu.Unlock()
	return ComCallFloatPtr(vol, slot, f, 0)
}

// ProbeVolumeSlot 诊断用：按第 slot 个虚表方法调用「取音量」签名，返回 (hr, value)。
//
// 用来核对 IAudioEndpointVolume 的槽位（写错槽位会直接让进程访问违例）。
// 正确槽位返回 hr=0 且 value 落在 0~1。
func ProbeVolumeSlot(slot int) (uint32, float32) {
	vol := audioEndpoint()
	if vol == 0 {
		return 0x80004005, 0
	}
	volMu.Lock()
	defer volMu.Unlock()
	var v float32
	hr := comCall(vol, slot, uintptr(unsafe.Pointer(&v)))
	return hr, v
}
