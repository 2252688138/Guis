//go:build windows

// guis — 持久化环境变量（当前用户 / 系统级）
//
// 把变量写进系统自带的「环境变量」存储，重登 / 重启后仍在：
//   - 当前用户：HKCU\Environment（无需管理员）
//   - 系统级  ：HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment（需要管理员）
//
// 写完后广播 WM_SETTINGCHANGE("Environment")，资源管理器等程序会重新读取；
// 同时 os.Setenv，本进程立刻生效，不用等下次启动。

package win32

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	// EnvScopeUser 当前用户环境变量（默认）。
	EnvScopeUser = "user"
	// EnvScopeSystem 系统级环境变量（全机可见）。
	EnvScopeSystem = "system"
)

// errNoRegistry 注册表 API 不可用（非 Windows 或绑定失败）。
var errNoRegistry = errors.New("当前系统不支持写注册表")

// envKey 返回作用域对应的根键与子键路径。
func envKey(scope string) (uintptr, string) {
	if scope == EnvScopeSystem {
		return HKEY_LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	}
	return HKEY_CURRENT_USER, `Environment`
}

// SetEnvVar 写入持久化环境变量（不广播；批量写完后调 BroadcastEnvChange）。
func SetEnvVar(name, value, scope string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("环境变量名为空")
	}
	if strings.Contains(name, "=") {
		return errors.New("环境变量名不能包含 =")
	}
	if RegOpenKeyExW == nil || RegSetValueExW == nil {
		return errNoRegistry
	}
	root, sub := envKey(scope)
	subp, err := syscall.UTF16PtrFromString(sub)
	if err != nil {
		return err
	}
	var key uintptr
	if rc := RegOpenKeyExW(root, subp, 0, KEY_SET_VALUE, &key); rc != 0 {
		return regErr(rc, scope)
	}
	defer RegCloseKey(key)

	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	// StringToUTF16 结尾已带 NUL；REG_SZ 的长度含结尾 NUL。
	buf := syscall.StringToUTF16(value)
	if rc := RegSetValueExW(key, namep, 0, REG_SZ, (*byte)(unsafe.Pointer(&buf[0])), uint32(len(buf)*2)); rc != 0 {
		return regErr(rc, scope)
	}
	_ = os.Setenv(name, value)
	return nil
}

// DeleteEnvVar 删除持久化环境变量（不存在视为成功，不广播）。
func DeleteEnvVar(name, scope string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if RegOpenKeyExW == nil || RegDeleteValueW == nil {
		return errNoRegistry
	}
	root, sub := envKey(scope)
	subp, err := syscall.UTF16PtrFromString(sub)
	if err != nil {
		return err
	}
	var key uintptr
	if rc := RegOpenKeyExW(root, subp, 0, KEY_SET_VALUE, &key); rc != 0 {
		if rc == 2 { // ERROR_FILE_NOT_FOUND
			return nil
		}
		return regErr(rc, scope)
	}
	defer RegCloseKey(key)
	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	if rc := RegDeleteValueW(key, namep); rc != 0 && rc != 2 {
		return regErr(rc, scope)
	}
	_ = os.Unsetenv(name)
	return nil
}

// GetEnvPersistent 直接从注册表读取持久化变量（当前进程环境块可能尚未刷新）。
func GetEnvPersistent(name, scope string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || RegOpenKeyExW == nil || RegQueryValueExW == nil {
		return "", false
	}
	root, sub := envKey(scope)
	subp, err := syscall.UTF16PtrFromString(sub)
	if err != nil {
		return "", false
	}
	var key uintptr
	if rc := RegOpenKeyExW(root, subp, 0, KEY_READ, &key); rc != 0 {
		return "", false
	}
	defer RegCloseKey(key)
	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, 512)
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

// BroadcastEnvChange 通知系统环境变量已变化（资源管理器 / 新进程会重新读取）。
func BroadcastEnvChange() {
	if SendMessageTimeoutW == nil {
		return
	}
	msg, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var res uintptr
	SendMessageTimeoutW(HWND_BROADCAST, WM_SETTINGCHANGE, 0,
		uintptr(unsafe.Pointer(msg)), SMTO_ABORTIFHUNG, 3000, &res)
}

// regErr 把注册表错误码翻译成用户能看懂的提示。
func regErr(rc int32, scope string) error {
	switch rc {
	case 5: // ERROR_ACCESS_DENIED
		if scope == EnvScopeSystem {
			return errors.New("需要以管理员身份运行才能写系统级环境变量")
		}
		return errors.New("没有权限写入环境变量")
	case 2:
		return errors.New("环境变量注册表项不存在")
	}
	return fmt.Errorf("写入环境变量失败（错误码 %d）", rc)
}
