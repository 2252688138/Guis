//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

// 快捷方式（IShellLink + IPersistFile）与文件关联（HKCU\Software\Classes）。

var (
	procSHChangeNotify = shell32L.NewProc("SHChangeNotify")

	clsidShellLink = guid{0x00021401, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidShellLinkW  = guid{0x000214F9, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidPersistFile2 = guid{0x0000010B, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

// IShellLinkW 槽位。
const (
	slSetDescription   = 7
	slSetWorkingDir    = 9
	slSetArguments     = 11
	slSetHotkey        = 13
	slSetShowCmd       = 15
	slSetIconLocation  = 17
	slSetPath          = 20
)

// IPersistFile 槽位。
const ipfSave = 6

// comCoCreate 创建 COM 对象（失败返回 0）。
func comCoCreate(clsid, iid *guid) uintptr {
	if procCoCreateInstance.Find() != nil {
		return 0
	}
	var obj uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(clsid)), 0, 0x1, // CLSCTX_INPROC_SERVER
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&obj)),
	)
	if int32(hr) < 0 {
		return 0
	}
	return obj
}

// CreateShortcut 创建 .lnk 快捷方式。
func CreateShortcut(lnkPath, target string, args []string, workDir, iconPath string,
	iconIndex int, description, hotkey string, minimized bool) error {
	if procOleInitialize.Find() == nil {
		procOleInitialize.Call(0)
	}
	link := comCoCreate(&clsidShellLink, &iidShellLinkW)
	if link == 0 {
		return errors.New("win32: 创建 IShellLink 失败")
	}
	defer comRelease(link)

	setStr := func(slot int, s string) {
		if s == "" {
			return
		}
		p, err := syscall.UTF16PtrFromString(s)
		if err != nil {
			return
		}
		comCall(link, slot, uintptr(unsafe.Pointer(p)))
	}
	setStr(slSetPath, target)
	if len(args) > 0 {
		quoted := make([]string, 0, len(args))
		for _, a := range args {
			if strings.ContainsAny(a, " \t") && !strings.HasPrefix(a, "\"") {
				quoted = append(quoted, "\""+a+"\"")
			} else {
				quoted = append(quoted, a)
			}
		}
		setStr(slSetArguments, strings.Join(quoted, " "))
	}
	setStr(slSetWorkingDir, workDir)
	setStr(slSetDescription, description)
	if iconPath != "" {
		p, err := syscall.UTF16PtrFromString(iconPath)
		if err == nil {
			comCall(link, slSetIconLocation, uintptr(unsafe.Pointer(p)), uintptr(iconIndex))
		}
	}
	if hk := parseShellHotkey(hotkey); hk != 0 {
		comCall(link, slSetHotkey, hk)
	}
	show := uintptr(1) // SW_SHOWNORMAL
	if minimized {
		show = 7 // SW_SHOWMINNOACTIVE
	}
	comCall(link, slSetShowCmd, show)

	// 取 IPersistFile 保存
	var pf uintptr
	if hr := comCall(link, 0, uintptr(unsafe.Pointer(&iidPersistFile2)), uintptr(unsafe.Pointer(&pf))); comFail(hr) || pf == 0 {
		return errors.New("win32: 取 IPersistFile 失败")
	}
	defer comRelease(pf)
	path, err := syscall.UTF16PtrFromString(lnkPath)
	if err != nil {
		return err
	}
	if hr := comCall(pf, ipfSave, uintptr(unsafe.Pointer(path)), 1); comFail(hr) {
		return errors.New("win32: 保存快捷方式失败")
	}
	return nil
}

// parseShellHotkey 把 "Ctrl+Alt+K" 解析成 IShellLink 的热键 WORD（低字节 VK、高字节修饰键）。
func parseShellHotkey(s string) uintptr {
	if s == "" {
		return 0
	}
	var mods uintptr
	var vk uintptr
	for _, part := range strings.Split(s, "+") {
		p := strings.ToLower(strings.TrimSpace(part))
		switch p {
		case "ctrl", "control":
			mods |= 0x02 // HOTKEYF_CONTROL
		case "alt":
			mods |= 0x04 // HOTKEYF_ALT
		case "shift":
			mods |= 0x01 // HOTKEYF_SHIFT
		case "":
			continue
		default:
			r := []rune(p)
			if len(r) == 1 {
				c := r[0]
				if c >= 'a' && c <= 'z' {
					c = c - 'a' + 'A'
				}
				vk = uintptr(c)
			} else if len(p) >= 2 && p[0] == 'f' {
				// F1..F24
				n := 0
				for _, ch := range p[1:] {
					if ch < '0' || ch > '9' {
						n = 0
						break
					}
					n = n*10 + int(ch-'0')
				}
				if n >= 1 && n <= 24 {
					vk = uintptr(0x70 + n - 1)
				}
			}
		}
	}
	if vk == 0 {
		return 0
	}
	return (mods << 8) | vk
}

// SetFileAssociation 把扩展名关联到指定程序（写 HKCU，无需管理员）。
//
//	win32.SetFileAssociation(".myapp", "MyApp.Document", `"C:\App\app.exe" "%1"`)
func SetFileAssociation(ext, progID, command string) error {
	if RegCreateKeyExW == nil || RegSetValueExW == nil {
		return errors.New("win32: 注册表 API 不可用")
	}
	base := `Software\Classes\`
	// 1) HKCU\Software\Classes\<progID>\shell\open\command = command
	cmdKey := base + progID + `\shell\open\command`
	if err := regSetString(cmdKey, "", command); err != nil {
		return err
	}
	// 2) HKCU\Software\Classes\<ext> = progID
	if err := regSetString(base+ext, "", progID); err != nil {
		return err
	}
	// 3) 通知资源管理器刷新关联
	if procSHChangeNotify.Find() == nil {
		const shcneAssocChanged = 0x08000000
		procSHChangeNotify.Call(shcneAssocChanged, 0, 0, 0)
	}
	return nil
}

// regSetString 创建/写入 HKCU 下的字符串值。
func regSetString(subKey, name, value string) error {
	if RegCreateKeyExW == nil || RegSetValueExW == nil {
		return errors.New("win32: 注册表 API 不可用")
	}
	sub, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return err
	}
	var key uintptr
	const keyAllAccess = 0xF003F
	if r := RegCreateKeyExW(HKEY_CURRENT_USER, sub, 0, nil, 0, keyAllAccess, 0, &key, nil); r != 0 {
		return errors.New("win32: 打开注册表键失败")
	}
	defer RegCloseKey(key)
	val, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return err
	}
	var namePtr *uint16
	if name != "" {
		np, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			return err
		}
		namePtr = np
	}
	data := (*byte)(unsafe.Pointer(val))
	size := uint32((len(value) + 1) * 2)
	if r := RegSetValueExW(key, namePtr, 0, REG_SZ, data, size); r != 0 {
		return errors.New("win32: 写注册表失败")
	}
	return nil
}
