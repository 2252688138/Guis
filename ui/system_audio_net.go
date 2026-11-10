// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// 系统音量与网络状态。
//
//	ui.SetSystemVolume(0.4)     // 0~1
//	ui.ToggleSystemMute()
//	if !ui.Online() { 提示离线 }
//
// Windows 走 Core Audio（IAudioEndpointVolume），Linux 走 wpctl / pactl / amixer，
// macOS 走 osascript。找不到可用的混音器命令时返回「不支持」错误。

var (
	volumeGetNative func() (float32, bool)
	volumeSetNative func(v float32) error
	muteToggleNative func() (bool, error)
)

// SystemVolume 读取系统主音量 0~1（读不到返回 -1）。
func SystemVolume() float32 {
	if volumeGetNative != nil {
		if v, ok := volumeGetNative(); ok {
			return v
		}
	}
	return -1
}

// SetSystemVolume 设置系统主音量（0~1，自动夹取）。
func SetSystemVolume(v float32) error {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	if volumeSetNative == nil {
		return errUnsupported("设置系统音量")
	}
	return volumeSetNative(v)
}

// ToggleSystemMute 切换系统静音，返回切换后的静音状态。
func ToggleSystemMute() (bool, error) {
	if muteToggleNative == nil {
		return false, errUnsupported("系统静音")
	}
	return muteToggleNative()
}

// ---- 网络状态 ----

// Online 粗粒度判断当前是否联网：尝试解析域名（不发起真实业务请求）。
//
// 这是「离线就隐藏下载按钮」这类场景够用的实现；需要精确到「某个服务是否可达」
// 请自己发 HEAD 请求。
func Online() bool {
	if runtime.GOOS == "js" {
		return true
	}
	c := exec.Command("ping", "-n", "1", "-w", "600", "223.5.5.5")
	if runtime.GOOS != "windows" {
		c = exec.Command("ping", "-c", "1", "-W", "1", "223.5.5.5")
	}
	hideConsole(c)
	if err := c.Run(); err == nil {
		return true
	}
	// 回退：DNS 解析（某些网络禁 ICMP）
	c2 := exec.Command("nslookup", "example.com")
	if runtime.GOOS != "windows" {
		c2 = exec.Command("getent", "hosts", "example.com")
	}
	hideConsole(c2)
	return c2.Run() == nil
}

// OnlineAsync 在后台判断联网状态，结果回 UI 线程（不阻塞界面）。
func OnlineAsync(fn func(online bool)) {
	if fn == nil {
		return
	}
	RunAsync(func() {
		ok := Online()
		postToUI(func() { fn(ok) })
	})
}

// SetOnNetworkChange 定期检查联网状态，变化时回调（返回停止函数）。
//
// 内部是轮询（默认 5 秒），因为各平台「网络变化」通知的接入成本远高于收益；
// 需要毫秒级感知请自己用 WebView / socket 保活。
func SetOnNetworkChange(fn func(online bool)) func() {
	if fn == nil {
		return func() {}
	}
	quit := make(chan struct{})
	last := Online()
	go func() {
		defer recoverReport("network-watch")
		tk := time.NewTicker(5 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-quit:
				return
			case <-tk.C:
				cur := Online()
				if cur == last {
					continue
				}
				last = cur
				v := cur
				postToUI(func() { fn(v) })
			}
		}
	}()
	return func() { close(quit) }
}

// ---- Linux / macOS 命令行音量后端（编译期无关，运行时探测） ----

// audioBackendCommand 依次尝试常见混音器命令，返回第一个可用的。
func audioBackendCommand(candidates ...string) string {
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// volumeCmdSupported 当前平台是否存在可用的 CLI 音量后端（供能力表使用）。
func volumeCmdSupported() bool {
	if runtime.GOOS == "darwin" {
		return audioBackendCommand("osascript") != ""
	}
	return audioBackendCommand("wpctl", "pactl", "amixer") != ""
}

// runQuiet 执行命令并返回去掉首尾空白的 stdout。
func runQuiet(name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	hideConsole(c)
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
