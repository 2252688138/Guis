//go:build windows

// guis — 开机自启注册项（当前用户级）
//
// 写在 HKCU\Software\Microsoft\Windows\CurrentVersion\Run：
//
//   - 当前用户登录即启动，**不需要管理员权限**，不触发 UAC；
//   - 值名 = 应用注册名，值 = "exe 路径" 参数…（含空格的路径必须带引号）；
//   - 任务管理器的「启动应用」页会列出该项，用户可在系统里自行禁用/删除，
//     因此读取时始终以注册表真实内容为准，不缓存。
package win32

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

// runKeyPath 当前用户的登录自启项注册表路径。
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// SetRunAtLogin 写入 / 删除当前用户的登录自启项。
//
// on 为 false 时删除（不存在视为成功）；on 为 true 时写入 command。
func SetRunAtLogin(name, command string, on bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("开机自启项名称为空")
	}
	if !on {
		return deleteRunValue(name)
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("开机自启命令为空")
	}
	if RegCreateKeyExW == nil || RegSetValueExW == nil {
		return errNoRegistry
	}
	subp, err := syscall.UTF16PtrFromString(runKeyPath)
	if err != nil {
		return err
	}
	var key uintptr
	var disp uint32
	// REG_OPTION_NON_VOLATILE = 0；安全属性传 0 表示默认（当前用户上下文）。
	if rc := RegCreateKeyExW(HKEY_CURRENT_USER, subp, 0, nil, 0, KEY_SET_VALUE, 0, &key, &disp); rc != 0 {
		return regErr(rc, EnvScopeUser)
	}
	defer RegCloseKey(key)

	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	buf := syscall.StringToUTF16(command) // 结尾已带 NUL
	if rc := RegSetValueExW(key, namep, 0, REG_SZ, (*byte)(unsafe.Pointer(&buf[0])), uint32(len(buf)*2)); rc != 0 {
		return regErr(rc, EnvScopeUser)
	}
	return nil
}

// RunAtLogin 读取当前用户登录自启项的注册命令（不存在返回 "", false）。
func RunAtLogin(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || RegOpenKeyExW == nil || RegQueryValueExW == nil {
		return "", false
	}
	subp, err := syscall.UTF16PtrFromString(runKeyPath)
	if err != nil {
		return "", false
	}
	var key uintptr
	if rc := RegOpenKeyExW(HKEY_CURRENT_USER, subp, 0, KEY_READ, &key); rc != 0 {
		return "", false
	}
	defer RegCloseKey(key)
	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, 1024)
	cb := uint32(len(buf) * 2)
	for {
		rc := RegQueryValueExW(key, namep, nil, nil, (*byte)(unsafe.Pointer(&buf[0])), &cb)
		if rc == 234 && int(cb/2) > len(buf) { // ERROR_MORE_DATA
			buf = make([]uint16, cb/2+1)
			continue
		}
		if rc != 0 {
			return "", false
		}
		n := int(cb / 2)
		if n > len(buf) {
			n = len(buf)
		}
		s := syscall.UTF16ToString(buf[:n])
		if s == "" {
			return "", false
		}
		return s, true
	}
}

// deleteRunValue 删除注册表值（不存在 / 键不存在都视为成功）。
func deleteRunValue(name string) error {
	if RegOpenKeyExW == nil || RegDeleteValueW == nil {
		return errNoRegistry
	}
	subp, err := syscall.UTF16PtrFromString(runKeyPath)
	if err != nil {
		return err
	}
	var key uintptr
	if rc := RegOpenKeyExW(HKEY_CURRENT_USER, subp, 0, KEY_SET_VALUE, &key); rc != 0 {
		if rc == 2 { // ERROR_FILE_NOT_FOUND
			return nil
		}
		return regErr(rc, EnvScopeUser)
	}
	defer RegCloseKey(key)
	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	if rc := RegDeleteValueW(key, namep); rc != 0 && rc != 2 {
		return regErr(rc, EnvScopeUser)
	}
	return nil
}
