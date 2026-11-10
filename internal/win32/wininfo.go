//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// 剪贴板变化通知（AddClipboardFormatListener）与前台窗口查询。
//
// 剪贴板以前只能 500ms 轮询序号；接上 WM_CLIPBOARDUPDATE 后是「零延迟 + 零轮询」，
// 复制后立即触发回调（window.go 的 wndProc 把消息转发到 dispatchClipboardChange）。

var (
	procAddClipboardFormatListener    = user32L.NewProc("AddClipboardFormatListener")
	procRemoveClipboardFormatListener = user32L.NewProc("RemoveClipboardFormatListener")
	procGetForegroundWindow           = user32L.NewProc("GetForegroundWindow")
	procGetWindowTextW                = user32L.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId      = user32L.NewProc("GetWindowThreadProcessId")
	procQueryFullProcessImageNameW    = kernel32L.NewProc("QueryFullProcessImageNameW")
)

// clipboardListenerMu 保护监听器集合。
var (
	clipboardListenerMu sync.RWMutex
	clipboardListeners  []func()
)

// RegisterClipboardListener 注册剪贴板变化回调（进程内全局，窗口需要已开启监听）。
// 返回注销函数。
func RegisterClipboardListener(fn func()) func() {
	if fn == nil {
		return func() {}
	}
	clipboardListenerMu.Lock()
	clipboardListeners = append(clipboardListeners, fn)
	idx := len(clipboardListeners) - 1
	clipboardListenerMu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			clipboardListenerMu.Lock()
			if idx < len(clipboardListeners) {
				clipboardListeners[idx] = nil
			}
			clipboardListenerMu.Unlock()
		})
	}
}

// dispatchClipboardChange 由窗口过程在收到 WM_CLIPBOARDUPDATE 时调用。
func dispatchClipboardChange() {
	clipboardListenerMu.RLock()
	list := append([]func(){}, clipboardListeners...)
	clipboardListenerMu.RUnlock()
	for _, fn := range list {
		if fn != nil {
			fn()
		}
	}
}

// WatchClipboardNative 注册「窗口级」剪贴板监听；失败返回 false（调用方回退轮询）。
func WatchClipboardNative(hwnd uintptr, fn func()) func() {
	if hwnd == 0 || fn == nil || procAddClipboardFormatListener.Find() != nil {
		return nil
	}
	if r, _, _ := procAddClipboardFormatListener.Call(hwnd); r == 0 {
		return nil
	}
	stop := RegisterClipboardListener(fn)
	return func() {
		stop()
		if procRemoveClipboardFormatListener.Find() == nil {
			procRemoveClipboardFormatListener.Call(hwnd)
		}
	}
}

// WindowInfo 前台窗口信息。
type WindowInfo struct {
	Handle  uintptr
	Title   string
	Process string
	PID     uint32
}

// ForegroundWindowInfo 返回当前前台窗口的信息（取不到时 ok=false）。
func ForegroundWindowInfo() (WindowInfo, bool) {
	if procGetForegroundWindow.Find() != nil {
		return WindowInfo{}, false
	}
	h, _, _ := procGetForegroundWindow.Call()
	if h == 0 {
		return WindowInfo{}, false
	}
	info := WindowInfo{Handle: h}

	// 标题
	var buf [512]uint16
	n, _, _ := procGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n > 0 {
		info.Title = syscall.UTF16ToString(buf[:n])
	}

	// 进程
	var pid uint32
	if procGetWindowThreadProcessId.Find() == nil {
		procGetWindowThreadProcessId.Call(h, uintptr(unsafe.Pointer(&pid)))
	}
	info.PID = pid
	if pid != 0 && procOpenProcess != nil && procQueryFullProcessImageNameW.Find() == nil {
		const processQueryLimitedInformation = 0x1000
		ph, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
		if ph != 0 {
			defer procCloseHandle.Call(ph)
			var path [520]uint16
			size := uint32(len(path))
			if r, _, _ := procQueryFullProcessImageNameW.Call(ph, 0,
				uintptr(unsafe.Pointer(&path[0])), uintptr(unsafe.Pointer(&size))); r != 0 && size > 0 {
				full := syscall.UTF16ToString(path[:size])
				info.Process = filepath.Base(full)
			}
		}
	}
	if info.Process == "" && info.Title == "" {
		// 极少数受保护进程（如 UWP 壳）取不到信息
		return info, true
	}
	return info, true
}

// TrimProcessName 去掉 .exe 后缀（日志 / 展示更好看）。
func TrimProcessName(name string) string {
	return strings.TrimSuffix(name, ".exe")
}

var procOpenProcess = kernel32L.NewProc("OpenProcess")
