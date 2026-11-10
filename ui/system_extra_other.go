//go:build !windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"errors"
	"image"
	"strconv"
	"strings"
)

// 桌面增强能力的非 Windows 降级实现：符号齐全、返回零值 / 不支持。
// 业务代码应先用 HasCap 判断（CapScreenCapture / CapPrint / CapMultiMonitor …）。

// Monitor 显示器信息（非 Windows 端字段保留，值由各端自行填充或为空）。
type Monitor struct {
	Index   int
	Name    string
	X, Y    int
	W, H    int
	WorkX   int
	WorkY   int
	WorkW   int
	WorkH   int
	Scale   float32
	Primary bool
}

// monitorsNative 平台显示器枚举实现（Linux: xrandr；macOS: displayplacer / AppleScript）。
var monitorsNative func() []Monitor

// parseWxH 解析 "2560x1440"（显示器枚举共用）。
func parseWxH(s string) (int, int, bool) {
	i := strings.IndexByte(s, 'x')
	if i <= 0 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(s[:i])
	h, err2 := strconv.Atoi(s[i+1:])
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// Monitors 非 Windows 端：优先用各平台实现（Linux 走 xrandr，macOS 走 displayplacer）；
// 都不可用时返回 nil，业务侧应先用 HasCap(CapMultiMonitor) 判断。
func Monitors() []Monitor {
	if monitorsNative != nil {
		if ms := monitorsNative(); len(ms) > 0 {
			return ms
		}
	}
	return nil
}

// MonitorAt 返回包含该屏幕坐标的显示器。
func MonitorAt(x, y int) (Monitor, bool) {
	for _, m := range Monitors() {
		if x >= m.X && x < m.X+m.W && y >= m.Y && y < m.Y+m.H {
			return m, true
		}
	}
	return Monitor{}, false
}

// VirtualScreen 所有显示器并集；没有显示器信息时返回零值。
func VirtualScreen() (int, int, int, int) {
	ms := Monitors()
	if len(ms) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY := ms[0].X, ms[0].Y
	maxX, maxY := ms[0].X+ms[0].W, ms[0].Y+ms[0].H
	for _, m := range ms[1:] {
		if m.X < minX {
			minX = m.X
		}
		if m.Y < minY {
			minY = m.Y
		}
		if m.X+m.W > maxX {
			maxX = m.X + m.W
		}
		if m.Y+m.H > maxY {
			maxY = m.Y + m.H
		}
	}
	return minX, minY, maxX - minX, maxY - minY
}

// CaptureScreen 非 Windows 端暂不支持。
func CaptureScreen(x, y, w, h int, cursor bool) (*image.NRGBA, error) {
	return nil, errors.New("ui: 当前平台暂不支持屏幕捕获")
}

// CaptureFullScreen 非 Windows 端暂不支持。
func CaptureFullScreen() (*image.NRGBA, error) {
	return nil, errors.New("ui: 当前平台暂不支持屏幕捕获")
}

// ClipboardInfo 剪贴板快照。
type ClipboardInfo struct {
	Seq      uint32
	Text     string
	HasText  bool
	HasImage bool
	HasFiles bool
}

// Clipboard 读取剪贴板摘要（非 Windows 仅文本）。
func Clipboard() ClipboardInfo {
	t := ClipboardText()
	return ClipboardInfo{Text: t, HasText: t != ""}
}

// ClipboardImage 非 Windows 端不支持。
func ClipboardImage() (*image.NRGBA, error) {
	return nil, errors.New("ui: 当前平台暂不支持读取剪贴板图片")
}

// WatchClipboard 非 Windows 端用轮询文本变化近似实现。
func WatchClipboard(fn func(ClipboardInfo)) (stop func()) {
	return watchClipboardPolling(fn)
}

// 单实例锁在非 Windows 端见 system_instance_unix.go（flock）/ system_instance_stub.go。

// ---- 打印 ----

func printDocument(hwnd uintptr, title string, doc Printable) (bool, error) {
	return false, errors.New("ui: 当前平台暂不支持打印")
}

// printList / printDefault / printTo 非 Windows 端无打印机。
func printList() []string { return nil }

func printDefault() string { return "" }

func printTo(printer, title string, doc Printable) (bool, error) {
	return false, errors.New("ui: 当前平台暂不支持打印")
}

func printText(dc uintptr, x, y, size float32, s, color string)         {}
func printRect(dc uintptr, x, y, w, h, stroke float32, sc, fill string) {}
func printLine(dc uintptr, x1, y1, x2, y2, width float32, color string) {}

// postToUI 非 Windows 端直接执行。
func postToUI(fn func()) { fn() }
