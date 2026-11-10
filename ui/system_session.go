// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os"
	"runtime"
	"strings"
)

// 会话与系统控制。
//
//	ui.LockScreen()                       // 锁屏（Win+L 等价）
//	if ui.ConfirmShutdown() { ui.Shutdown(0) }
//	ui.Suspend()                          // 睡眠
//
// 设计约束：**默认不需要二次确认，但关机/重启这类不可逆操作建议业务自己弹
// 确认框**（框架不阻止，只是不替你决定）。所有调用失败返回 error，不 panic。

// sessionNative 平台原生实现（Windows: LockWorkStation / ExitWindowsEx）。
var (
	lockScreenNative  func() error
	suspendNative     func(hibernate bool) error
	powerOffNative    func(reboot bool, delay int, force bool) error
	signOutNative     func(force bool) error
	cancelShutdownNative func() error
)

// LockScreen 锁定当前会话（Windows 为 LockWorkStation，Linux 走 loginctl）。
func LockScreen() error {
	if lockScreenNative == nil {
		return errUnsupported("锁屏")
	}
	return lockScreenNative()
}

// Suspend 让系统进入睡眠（hibernate=true 时进入休眠）。
func Suspend(hibernate bool) error {
	if suspendNative == nil {
		return errUnsupported("睡眠")
	}
	return suspendNative(hibernate)
}

// Shutdown 关机。delay>0 时延迟指定秒数（用户可在此期间调用 CancelShutdown 取消）。
// force=true 会强制关闭未保存的程序，**慎用**。
func Shutdown(delay int, force bool) error {
	if powerOffNative == nil {
		return errUnsupported("关机")
	}
	return powerOffNative(false, delay, force)
}

// Reboot 重启（参数同 Shutdown）。
func Reboot(delay int, force bool) error {
	if powerOffNative == nil {
		return errUnsupported("重启")
	}
	return powerOffNative(true, delay, force)
}

// SignOut 注销当前用户。
func SignOut(force bool) error {
	if signOutNative == nil {
		return errUnsupported("注销")
	}
	return signOutNative(force)
}

// CancelShutdown 取消已经排队的关机 / 重启。
func CancelShutdown() error {
	if cancelShutdownNative == nil {
		return errUnsupported("取消关机")
	}
	return cancelShutdownNative()
}

// errUnsupported 统一的「当前平台不支持」错误。
func errUnsupported(what string) error {
	return &unsupportedError{what: what}
}

type unsupportedError struct{ what string }

func (e *unsupportedError) Error() string {
	if runtime.GOOS == "js" {
		return "ui: 浏览器端不支持" + e.what
	}
	return "ui: 当前平台不支持" + e.what
}

// IsUnsupported 判断错误是否来自「平台不支持」（业务侧据此隐藏入口）。
func IsUnsupported(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*unsupportedError); ok {
		return true
	}
	return strings.Contains(err.Error(), "不支持")
}

// systemLocaleFallback 粗粒度语言探测（精确值由各平台原生实现提供）。
func systemLocaleFallback() string {
	for _, k := range []string{"LANG", "LC_ALL", "LC_MESSAGES"} {
		v := os.Getenv(k)
		if v == "" {
			continue
		}
		// zh_CN.UTF-8 → zh-CN
		v = strings.TrimSuffix(v, ".UTF-8")
		v = strings.TrimSuffix(v, ".utf8")
		v = strings.ReplaceAll(v, "_", "-")
		if i := strings.IndexByte(v, '@'); i > 0 {
			v = v[:i]
		}
		if v != "" && v != "C" && v != "POSIX" {
			return v
		}
	}
	return "en-US"
}
