//go:build !windows && !js && !android && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os/exec"
	"sync"

	"guis/internal/audio"
)

// platformCaps 类 Unix 端能力表。
//
// 与 Windows 的差别（全部体现在这里，业务代码可用 HasCap 分支）：
//   - 视频/音频：只有 ffmpeg 一条解码路径（无系统解码器，见 system_decoder_other.go），
//     音频还要求本机有播放后端（paplay / aplay / pw-play）；
//   - 内嵌浏览器：暂无（Linux 需 WebKitGTK，macOS 需 WKWebView）；
//   - 托盘：暂无（Linux 需 StatusNotifierItem，macOS 需 NSStatusBar）；
//   - 文件拖入：X11 需 XDND 协议，尚未接入；
//   - 全局热键：X11 需 XGrabKey，尚未接入；
//   - 剪贴板/文件对话框：依赖系统命令（xclip/wl-paste/zenity/pbpaste…），存在才报告支持。
var (
	capOnce  sync.Once
	capCache []Cap
)

func platformCaps() []Cap {
	capOnce.Do(func() {
		// 开机自启：Linux/BSD 走 XDG autostart、macOS 走 LaunchAgents，均为用户级。
		caps := []Cap{CapSystemDark, CapKeepAwake, CapModernNotify, CapAutoStart}
		// 纯 Go 实现（与桌面环境无关）：标准目录、文件监听、系统信息。
		caps = append(caps, CapKnownFolders, CapFileWatch, CapSystemInfo)
		// 电源与电池：Linux 读 /sys/class/power_supply，macOS 走 pmset。
		caps = append(caps, powerCaps()...)
		if commandExists("xclip", "xsel", "wl-paste", "pbpaste") {
			caps = append(caps, CapClipboard, CapClipboardWatch)
		}
		// 富文本剪贴板：xclip 支持按 target 读写 text/html。
		if commandExists("xclip") {
			caps = append(caps, CapClipboardRich)
		}
		if commandExists("zenity", "kdialog", "osascript") {
			caps = append(caps, CapDialogs, CapNativeDialog)
		}
		// 会话控制：logind / systemctl / pmset 可用时才能锁屏与注销关机。
		if commandExists("loginctl", "systemctl", "pmset") {
			caps = append(caps, CapSession)
		}
		// 单实例：flock 实现，类 Unix 全平台可用。
		caps = append(caps, CapSingleInstance)
		// 显示器枚举：xrandr（X11）或 displayplacer（macOS）。
		if commandExists("xrandr", "displayplacer", "osascript") {
			caps = append(caps, CapMultiMonitor)
		}
		if commandExists("gio") {
			caps = append(caps, CapRecycleBin)
		}
		// 媒体：只有 ffmpeg 一条解码路径；音频还需本机有可用播放后端。
		if commandExists("ffmpeg") && commandExists("ffprobe") {
			caps = append(caps, CapVideo)
			if audio.Devices() > 0 {
				caps = append(caps, CapAudio)
			}
		}
		// 系统集成（热键 / 托盘 / 拖入）：由各平台的 caps_integration_*.go 声明
		caps = append(caps, platformIntegrationCaps()...)
		capCache = caps
	})
	return capCache
}

// commandExists 任一命令存在即视为可用。
func commandExists(names ...string) bool {
	for _, n := range names {
		if _, err := exec.LookPath(n); err == nil {
			return true
		}
	}
	return false
}
