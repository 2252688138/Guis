//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"runtime"
	"syscall"
	"unsafe"
)

// 系统级操作原生实现：原生消息框、标准目录、系统信息、电源状态、会话控制。
//
// 与 api.go 里的 purego 绑定居分开：这里用 syscall.NewLazyDLL（延迟解析，失败只
// 影响对应能力），避免老系统上缺符号导致整个包 init panic。

var (
	user32L   = syscall.NewLazyDLL("user32.dll")
	kernel32L = syscall.NewLazyDLL("kernel32.dll")
	shell32L  = syscall.NewLazyDLL("shell32.dll")
	advapi32L = syscall.NewLazyDLL("advapi32.dll")
	ntdllL    = syscall.NewLazyDLL("ntdll.dll")
	powrprofL = syscall.NewLazyDLL("powrprof.dll")

	procMessageBoxW              = user32L.NewProc("MessageBoxW")
	procSHGetKnownFolderPath     = shell32L.NewProc("SHGetKnownFolderPath")
	procGetUserDefaultUILanguage = kernel32L.NewProc("GetUserDefaultUILanguage")
	procLCIDToLocaleName         = kernel32L.NewProc("LCIDToLocaleName")
	procGetUserNameW             = advapi32L.NewProc("GetUserNameW")
	procGetComputerNameExW       = kernel32L.NewProc("GetComputerNameExW")
	procRtlGetVersion            = ntdllL.NewProc("RtlGetVersion")
	procGetSystemPowerStatus     = kernel32L.NewProc("GetSystemPowerStatus")
	procLockWorkStation          = user32L.NewProc("LockWorkStation")
	procExitWindowsEx            = user32L.NewProc("ExitWindowsEx")
	procInitiateShutdownW        = advapi32L.NewProc("InitiateSystemShutdownExW")
	procAbortSystemShutdownW     = advapi32L.NewProc("AbortSystemShutdownW")
	procSetSuspendState          = powrprofL.NewProc("SetSuspendState")
	procOpenProcessToken         = advapi32L.NewProc("OpenProcessToken")
	procGetTokenInformation      = advapi32L.NewProc("GetTokenInformation")
	procLookupPrivilegeValueW    = advapi32L.NewProc("LookupPrivilegeValueW")
	procAdjustTokenPrivileges    = advapi32L.NewProc("AdjustTokenPrivileges")
	procCloseHandle              = kernel32L.NewProc("CloseHandle")
	procGetCurrentProcessID      = kernel32L.NewProc("GetCurrentProcessId")
)

// ---- 原生消息框 ----

// MessageBoxKind 消息框图标（对应 MB_ICON*）。
type MessageBoxKind int

const (
	// MBInfo 信息图标。
	MBInfo MessageBoxKind = iota
	// MBWarn 警告图标。
	MBWarn
	// MBError 错误图标。
	MBError
	// MBQuestion 询问图标。
	MBQuestion
)

// MessageBoxButtons 按钮组合（对应 MB_OK* / MB_YESNO*）。
type MessageBoxButtons int

const (
	// MBOK 只有确定。
	MBOK MessageBoxButtons = iota
	// MBOKCancel 确定 / 取消。
	MBOKCancel
	// MBYesNo 是 / 否。
	MBYesNo
	// MBRetryCancel 重试 / 取消。
	MBRetryCancel
)

// MessageBoxResult 返回值（对应 ID*）。
type MessageBoxResult int

const (
	// MBResultOK 确定。
	MBResultOK MessageBoxResult = iota
	// MBResultCancel 取消 / 关闭。
	MBResultCancel
	// MBResultYes 是。
	MBResultYes
	// MBResultNo 否。
	MBResultNo
)

// MessageBox 弹出系统原生消息框（模态）。返回用户选择。
func MessageBox(hwnd uintptr, title, body string, kind MessageBoxKind, buttons MessageBoxButtons) MessageBoxResult {
	t, _ := syscall.UTF16PtrFromString(title)
	b, _ := syscall.UTF16PtrFromString(body)
	uType := uintptr(0)
	switch kind {
	case MBWarn:
		uType |= 0x30 // MB_ICONWARNING
	case MBError:
		uType |= 0x10 // MB_ICONERROR
	case MBQuestion:
		uType |= 0x20 // MB_ICONQUESTION
	default:
		uType |= 0x40 // MB_ICONINFORMATION
	}
	switch buttons {
	case MBOKCancel:
		uType |= 0x1 // MB_OKCANCEL
	case MBYesNo:
		uType |= 0x4 // MB_YESNO
	case MBRetryCancel:
		uType |= 0x5 // MB_RETRYCANCEL
	default:
		uType |= 0x0 // MB_OK
	}
	// 顶层窗口：让对话框归到本进程（避免弹到别的桌面/被 z-order 吞掉）
	uType |= 0x10000 // MB_SETFOREGROUND
	if hwnd == 0 {
		uType |= 0x40000 // MB_TOPMOST（无主窗口时保证可见）
	}
	r, _, _ := procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(b)), uType)
	switch r {
	case 1, 2: // IDOK / IDCANCEL
		if r == 2 {
			return MBResultCancel
		}
		return MBResultOK
	case 6:
		return MBResultYes
	case 7:
		return MBResultNo
	case 4: // IDRETRY
		return MBResultOK
	}
	return MBResultCancel
}

// ---- 标准目录（SHGetKnownFolderPath）----

// guid 简单 GUID 表示（与 Windows GUID 内存布局一致）。
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// 常用已知文件夹 GUID。
var (
	folderDesktop     = guid{0xB4BFCC3A, 0xDB2C, 0x424C, [8]byte{0xB0, 0x29, 0x7F, 0xE9, 0x9A, 0x87, 0xC6, 0x41}}
	folderDocuments   = guid{0xFDD39AD0, 0x238F, 0x46AF, [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}}
	folderDownloads   = guid{0x374DE290, 0x123F, 0x4565, [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B}}
	folderPictures    = guid{0x33E28130, 0x4E1E, 0x4676, [8]byte{0x83, 0x5A, 0x98, 0x39, 0x5C, 0x3B, 0xC3, 0xBB}}
	folderMusic       = guid{0x4BD8D571, 0x6D19, 0x48D3, [8]byte{0xBE, 0x97, 0x42, 0x22, 0x20, 0x08, 0x0E, 0x43}}
	folderVideos      = guid{0x18989B1D, 0x99B5, 0x455B, [8]byte{0x84, 0x1C, 0xAB, 0x7C, 0x74, 0xE4, 0xDD, 0xFC}}
	folderRoaming     = guid{0x3EB685DB, 0x65F9, 0x4CF6, [8]byte{0xA0, 0x3A, 0xE3, 0xEF, 0x65, 0x72, 0x9F, 0x3D}}
	folderLocalApp    = guid{0xF1B32785, 0x6FBA, 0x4FCF, [8]byte{0x9D, 0x55, 0x7B, 0x8E, 0x7F, 0x15, 0x70, 0x91}}
	folderProfile     = guid{0x5E6C858F, 0x0E22, 0x4760, [8]byte{0x9A, 0xFE, 0xEA, 0x33, 0x17, 0xB6, 0x71, 0x73}}
	folderProgramFile = guid{0x905E63B6, 0xC1BF, 0x494E, [8]byte{0xB2, 0x9C, 0x65, 0xB7, 0x32, 0xD3, 0xD2, 0x1A}}
)

// KnownFolderKind 与 ui.KnownDir 一一对应的目录类别。
type KnownFolderKind int

const (
	// KFHome 用户主目录。
	KFHome KnownFolderKind = iota
	// KFDesktop 桌面。
	KFDesktop
	// KFDocuments 文档。
	KFDocuments
	// KFDownloads 下载。
	KFDownloads
	// KFPictures 图片。
	KFPictures
	// KFMusic 音乐。
	KFMusic
	// KFVideos 视频。
	KFVideos
	// KFConfig Roaming AppData。
	KFConfig
	// KFCache Local AppData（缓存）。
	KFCache
	// KFData Local AppData（数据）。
	KFData
	// KFTemp 临时目录。
	KFTemp
	// KFPrograms Program Files。
	KFPrograms
)

// KnownFolder 用 SHGetKnownFolderPath 查询标准目录（支持用户重定向到别的盘）。
// 失败返回空串。
func KnownFolder(kind KnownFolderKind) string {
	var g *guid
	switch kind {
	case KFDesktop:
		g = &folderDesktop
	case KFDocuments:
		g = &folderDocuments
	case KFDownloads:
		g = &folderDownloads
	case KFPictures:
		g = &folderPictures
	case KFMusic:
		g = &folderMusic
	case KFVideos:
		g = &folderVideos
	case KFConfig:
		g = &folderRoaming
	case KFCache, KFData:
		g = &folderLocalApp
	case KFHome:
		g = &folderProfile
	case KFPrograms:
		g = &folderProgramFile
	case KFTemp:
		return tempPath()
	}
	if g == nil || procSHGetKnownFolderPath.Find() != nil {
		return ""
	}
	var psz uintptr
	hr, _, _ := procSHGetKnownFolderPath.Call(uintptr(unsafe.Pointer(g)), 0, 0, uintptr(unsafe.Pointer(&psz)))
	if int32(hr) < 0 || psz == 0 {
		return ""
	}
	defer func() {
		if CoTaskMemFree != nil {
			CoTaskMemFree(psz)
		}
	}()
	return utf16PtrToGoString(psz)
}

// tempPath 取临时目录（GetTempPathW 比 SHGetKnownFolderPath 更稳）。
func tempPath() string {
	var buf [261]uint16
	proc := kernel32L.NewProc("GetTempPathW")
	n, _, _ := proc.Call(uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// utf16PtrToGoString 把 UTF-16 指针读成 Go 字符串（读到 NUL 为止）。
func utf16PtrToGoString(p uintptr) string {
	if p == 0 {
		return ""
	}
	var n int
	for {
		c := *(*uint16)(unsafe.Pointer(p + uintptr(n)*2))
		if c == 0 {
			break
		}
		n++
		if n > 32768 {
			break
		}
	}
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(p)), n))
}

// ---- 系统信息 ----

// osVersionInfoExW RtlGetVersion 的结构。
type osVersionInfoExW struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformID        uint32
	CSDVersion        [128]uint16
	ServicePackMajor  uint16
	ServicePackMinor  uint16
	SuiteMask         uint16
	ProductType       byte
	Reserved          byte
}

// SysInfoResult 原生系统信息。
type SysInfoResult struct {
	Locale     string
	LocaleName string
	UserName   string
	HostName   string
	OS         string
	OSVersion  string
	Arch       string
	Elevated   bool
}

// SysInfo 读取系统信息（locale 走 GetUserDefaultUILanguage + LCIDToLocaleName）。
func SysInfo() SysInfoResult {
	var out SysInfoResult
	out.Arch = runtime.GOARCH

	if procGetUserDefaultUILanguage.Find() == nil {
		if langid, _, _ := procGetUserDefaultUILanguage.Call(); langid != 0 {
			// LANGID → LCID：直接用 LANGID 作 LCID（低 16 位相同语义）
			out.Locale = lcidToName(uint32(langid))
		}
	}
	out.LocaleName = localeDisplayName(out.Locale)

	if procGetUserNameW.Find() == nil {
		var buf [256]uint16
		n := uint32(len(buf))
		var size = n
		if r, _, _ := procGetUserNameW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r != 0 {
			out.UserName = syscall.UTF16ToString(buf[:size-1])
		}
	}
	if procGetComputerNameExW.Find() == nil {
		var buf [256]uint16
		size := uint32(len(buf))
		// ComputerNameDnsHostname = 1
		if r, _, _ := procGetComputerNameExW.Call(1, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r != 0 && size > 0 {
			out.HostName = syscall.UTF16ToString(buf[:size])
		}
	}
	if procRtlGetVersion.Find() == nil {
		var vi osVersionInfoExW
		vi.OSVersionInfoSize = uint32(unsafe.Sizeof(vi))
		if r, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&vi))); r == 0 {
			out.OSVersion = itoa(int(vi.MajorVersion)) + "." + itoa(int(vi.MinorVersion)) + "." + itoa(int(vi.BuildNumber))
			out.OS = windowsProductName(int(vi.MajorVersion), int(vi.MinorVersion), int(vi.BuildNumber))
		}
	}
	out.Elevated = IsElevated()
	return out
}

// lcidToName LCID → BCP-47 名称（LCIDToLocaleName，Win Vista+）。
func lcidToName(lcid uint32) string {
	if procLCIDToLocaleName.Find() != nil {
		return ""
	}
	var buf [85]uint16
	n, _, _ := procLCIDToLocaleName.Call(uintptr(lcid), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n-1])
}

// localeDisplayName 常见语言的本地名称（Windows 未提供「语言自称」API，
// 这里覆盖使用率最高的几种，其余回退到 locale 代码本身）。
func localeDisplayName(locale string) string {
	switch locale {
	case "zh-CN", "zh-Hans-CN":
		return "中文（简体）"
	case "zh-TW", "zh-HK", "zh-Hant-TW":
		return "中文（繁體）"
	case "en-US":
		return "English (United States)"
	case "en-GB":
		return "English (United Kingdom)"
	case "ja-JP":
		return "日本語"
	case "ko-KR":
		return "한국어"
	case "de-DE":
		return "Deutsch"
	case "fr-FR":
		return "Français"
	case "ru-RU":
		return "Русский"
	case "es-ES":
		return "Español"
	}
	return ""
}

// windowsProductName 用主次版本号拼一个可读的系统名。
func windowsProductName(major, minor, build int) string {
	switch {
	case major == 10 && build >= 22000:
		return "Windows 11 (build " + itoa(build) + ")"
	case major == 10:
		return "Windows 10 (build " + itoa(build) + ")"
	case major == 6 && minor == 3:
		return "Windows 8.1 (build " + itoa(build) + ")"
	case major == 6 && minor == 2:
		return "Windows 8 (build " + itoa(build) + ")"
	case major == 6 && minor == 1:
		return "Windows 7 (build " + itoa(build) + ")"
	}
	return "Windows " + itoa(major) + "." + itoa(minor) + " (build " + itoa(build) + ")"
}

// itoa 极简整数转字符串（避免为几处日志引入 strconv 依赖）。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// tokenElevation TOKEN_ELEVATION（GetTokenInformation 的 TokenElevation=20）。
type tokenElevation struct{ TokenIsElevated uint32 }

// IsElevated 当前进程是否已提权（管理员）。
func IsElevated() bool {
	if procOpenProcessToken.Find() != nil || GetCurrentProcess == nil {
		return false
	}
	var tok uintptr
	if r, _, _ := procOpenProcessToken.Call(GetCurrentProcess(), 0x0008 /*TOKEN_QUERY*/, uintptr(unsafe.Pointer(&tok))); r == 0 {
		return false
	}
	defer procCloseHandle.Call(tok)
	var elev tokenElevation
	var n uint32
	if r, _, _ := procGetTokenInformation.Call(tok, 20, uintptr(unsafe.Pointer(&elev)), unsafe.Sizeof(elev), uintptr(unsafe.Pointer(&n))); r == 0 {
		return false
	}
	return elev.TokenIsElevated != 0
}

// ---- 电源状态 ----

// systemPowerStatus GetSystemPowerStatus 的结构。
type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

// PowerState 读取电源 / 电池状态。
func PowerState() (onBattery bool, percent int, charging bool, acOnline bool, ok bool) {
	if procGetSystemPowerStatus.Find() != nil {
		return false, -1, false, true, false
	}
	var st systemPowerStatus
	if r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&st))); r == 0 {
		return false, -1, false, true, false
	}
	acOnline = st.ACLineStatus == 1
	onBattery = st.ACLineStatus == 0
	percent = -1
	if st.BatteryLifePercent != 255 {
		percent = int(st.BatteryLifePercent)
	}
	// BatteryFlag 8 = 充电中
	charging = st.BatteryFlag&0x08 != 0
	return onBattery, percent, charging, acOnline, true
}

// ---- 会话控制 ----

// enablePrivilege 打开当前进程的指定特权（关机 / 重启需要 SeShutdownPrivilege）。
func enablePrivilege(name string) bool {
	if procOpenProcessToken.Find() != nil || procLookupPrivilegeValueW.Find() != nil || procAdjustTokenPrivileges.Find() != nil {
		return false
	}
	var tok uintptr
	// TOKEN_ADJUST_PRIVILEGES | TOKEN_QUERY
	if r, _, _ := procOpenProcessToken.Call(GetCurrentProcess(), 0x20|0x8, uintptr(unsafe.Pointer(&tok))); r == 0 {
		return false
	}
	defer procCloseHandle.Call(tok)
	n, _ := syscall.UTF16PtrFromString(name)
	var luid struct {
		LowPart  uint32
		HighPart int32
	}
	if r, _, _ := procLookupPrivilegeValueW.Call(0, uintptr(unsafe.Pointer(n)), uintptr(unsafe.Pointer(&luid))); r == 0 {
		return false
	}
	type luidAndAttrs struct {
		Luid       [2]uint32
		Attributes uint32
	}
	type tokenPrivileges struct {
		PrivilegeCount uint32
		_              uint32
		Privileges     luidAndAttrs
	}
	tp := tokenPrivileges{PrivilegeCount: 1}
	tp.Privileges.Luid = [2]uint32{luid.LowPart, uint32(luid.HighPart)}
	tp.Privileges.Attributes = 0x2 // SE_PRIVILEGE_ENABLED
	r, _, _ := procAdjustTokenPrivileges.Call(tok, 0, uintptr(unsafe.Pointer(&tp)), 0, 0, 0)
	return r != 0
}

// LockScreen 锁定工作站。
func LockScreen() error {
	if procLockWorkStation.Find() != nil {
		return syscall.EWINDOWS
	}
	if r, _, err := procLockWorkStation.Call(); r == 0 {
		return err
	}
	return nil
}

// SuspendSystem 睡眠（hibernate=true 为休眠）。
func SuspendSystem(hibernate bool) error {
	if procSetSuspendState.Find() != nil {
		return syscall.EWINDOWS
	}
	h := uintptr(0)
	if hibernate {
		h = 1
	}
	// Hibernate, ForceCritical=0, DisableWakeEvent=0
	r, _, err := procSetSuspendState.Call(h, 0, 0)
	if r == 0 {
		return err
	}
	return nil
}

// ShutdownSystem 关机 / 重启。
//
// delay>0 走 InitiateSystemShutdownExW（系统自己倒计时，可用 AbortSystemShutdown 取消）；
// delay==0 走 ExitWindowsEx（立即生效，需要 SeShutdownPrivilege —— 开机会自动尝试提升）。
func ShutdownSystem(reboot bool, delay int, force bool) error {
	enablePrivilege("SeShutdownPrivilege")
	const ewxShutdown = 0x00000001
	const ewxReboot = 0x00000002
	const ewxForceIfHung = 0x00000010
	const ewxForce = 0x00000004
	if delay > 0 && procInitiateShutdownW.Find() == nil {
		var rebootArg uintptr
		if reboot {
			rebootArg = 1
		}
		var forceArg uintptr
		if force {
			forceArg = 1
		}
		// 参数：机器名(NULL=本机) / 提示消息 / 超时秒 / 强制关应用 / 重启 / 原因码
		r, _, err := procInitiateShutdownW.Call(0, 0, uintptr(delay), forceArg, rebootArg, 0)
		if r == 0 {
			return err
		}
		return nil
	}
	flags := uintptr(ewxShutdown)
	if reboot {
		flags = ewxReboot
	}
	if force {
		flags |= ewxForce | ewxForceIfHung
	} else {
		flags |= ewxForceIfHung
	}
	if r, _, err := procExitWindowsEx.Call(flags, 0); r == 0 {
		return err
	}
	return nil
}

// SignOut 注销当前用户。
func SignOut(force bool) error {
	enablePrivilege("SeShutdownPrivilege")
	const ewxLogoff = 0x00000000
	flags := uintptr(ewxLogoff) | 0x00000010 // EWX_FORCEIFHUNG
	if force {
		flags |= 0x00000004 // EWX_FORCE
	}
	if r, _, err := procExitWindowsEx.Call(flags, 0); r == 0 {
		return err
	}
	return nil
}

// CancelShutdown 取消已排队的关机。
func CancelShutdown() error {
	if procAbortSystemShutdownW.Find() != nil {
		return syscall.EWINDOWS
	}
	enablePrivilege("SeShutdownPrivilege")
	if r, _, err := procAbortSystemShutdownW.Call(0); r == 0 {
		return err
	}
	return nil
}
