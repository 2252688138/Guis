//go:build !windows && !js && !android && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// 类 Unix 端把「能用命令行实现」的系统能力挂到跨平台钩子上。
//
// 原则：不改系统设置、不弹授权框，能用就用（zenity / loginctl / wpctl…），
// 缺命令就返回「不支持」，业务侧用 HasCap 判断后隐藏入口。

func init() {
	// ---- 原生消息框 ----
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("osascript"); err == nil {
			messageBoxNative = macMessageBox
		}
	} else if p := audioBackendCommand("zenity", "kdialog"); p != "" {
		messageBoxNative = func(title, body string, kind MsgKind, buttons MsgButtons) (MsgResult, bool) {
			return linuxMessageBox(p, title, body, kind, buttons)
		}
	}

	// ---- 会话控制 ----
	initSessionHooks()

	// ---- 系统音量 ----
	initVolumeHooks()

	// ---- 富文本剪贴板（Linux 需要 xclip） ----
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" {
		if _, err := exec.LookPath("xclip"); err == nil {
			clipboardRichGetNative = func(format string) (string, bool) {
				target := "text/html"
				if format == ClipRTF {
					target = "text/rtf"
				}
				out, err := runQuiet("xclip", "-selection", "clipboard", "-t", target, "-o")
				if err != nil || out == "" {
					return "", false
				}
				return out, true
			}
			clipboardRichSetNative = func(format, data string) bool {
				target := "text/html"
				if format == ClipRTF {
					target = "text/rtf"
				}
				c := exec.Command("xclip", "-selection", "clipboard", "-t", target, "-i")
				c.Stdin = strings.NewReader(data)
				hideConsole(c)
				return c.Run() == nil
			}
		}
	}
}

// ---- 消息框 ----

// linuxMessageBox 用 zenity / kdialog 弹原生对话框。
func linuxMessageBox(bin, title, body string, kind MsgKind, buttons MsgButtons) (MsgResult, bool) {
	text := body
	if title != "" {
		text = title + "\n\n" + body
	}
	var args []string
	if strings.Contains(bin, "zenity") {
		switch kind {
		case MsgError:
			args = []string{"--error"}
		case MsgWarn:
			args = []string{"--warning"}
		case MsgQuestion:
			args = []string{"--question"}
		default:
			args = []string{"--info"}
		}
		if title != "" {
			args = append(args, "--title", title)
		}
		switch buttons {
		case MsgBoxOKCancel:
			args = append(args, "--ok-label", "确定", "--cancel-label", "取消")
		case MsgBoxYesNo:
			args = append(args, "--ok-label", "是", "--cancel-label", "否")
		case MsgBoxRetryCancel:
			args = append(args, "--ok-label", "重试", "--cancel-label", "取消")
		}
		args = append(args, "--text", body)
	} else {
		switch kind {
		case MsgError:
			args = []string{"--error"}
		case MsgWarn:
			args = []string{"--warning"}
		case MsgQuestion:
			args = []string{"--yesno"}
		default:
			args = []string{"--msgbox"}
		}
		if title != "" {
			args = append(args, "--title", title)
		}
		args = append(args, text)
	}
	c := exec.Command(bin, args...)
	hideConsole(c)
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			// 用户点了「否 / 取消 / 关闭」
			if buttons == MsgBoxYesNo {
				return MsgResNo, true
			}
			return MsgResCancel, true
		}
		return MsgResOK, true
	}
	if buttons == MsgBoxYesNo {
		return MsgResYes, true
	}
	return MsgResOK, true
}

// macMessageBox 用 osascript 弹系统对话框。
func macMessageBox(title, body string, kind MsgKind, buttons MsgButtons) (MsgResult, bool) {
	icon := "note"
	switch kind {
	case MsgError:
		icon = "stop"
	case MsgWarn:
		icon = "caution"
	}
	labels := "{\"确定\"}"
	if buttons == MsgBoxOKCancel || buttons == MsgBoxRetryCancel {
		labels = "{\"确定\", \"取消\"}"
		if buttons == MsgBoxRetryCancel {
			labels = "{\"重试\", \"取消\"}"
		}
	} else if buttons == MsgBoxYesNo {
		labels = "{\"是\", \"否\"}"
	}
	script := "display dialog " + asQuote(body) + " with title " + asQuote(title) +
		" buttons " + labels + " default button 1 with icon " + icon
	c := exec.Command("osascript", "-e", script)
	hideConsole(c)
	out, err := c.Output()
	if err != nil {
		return MsgResCancel, true
	}
	s := string(out)
	switch {
	case strings.Contains(s, "取消"):
		return MsgResCancel, true
	case strings.Contains(s, "否"):
		return MsgResNo, true
	case strings.Contains(s, "重试"), strings.Contains(s, "是"):
		return MsgResYes, true
	}
	return MsgResOK, true
}

// asQuote 把字符串转成 AppleScript 字面量（转义反斜杠与引号）。
func asQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// ---- 会话控制 ----

func initSessionHooks() {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("pmset"); err == nil {
			lockScreenNative = func() error {
				return runCmd("osascript", "-e",
					`tell application "System Events" to keystroke "q" using {control down, command down}`)
			}
			suspendNative = func(hibernate bool) error { return runCmd("pmset", "sleepnow") }
			powerOffNative = func(reboot bool, delay int, force bool) error {
				verb := "shut down"
				if reboot {
					verb = "restart"
				}
				return runCmd("osascript", "-e", `tell application "System Events" to `+verb)
			}
			signOutNative = func(force bool) error {
				return runCmd("osascript", "-e", `tell application "System Events" to log out`)
			}
		}
	default:
		hasLoginctl := false
		if _, err := exec.LookPath("loginctl"); err == nil {
			hasLoginctl = true
			lockScreenNative = func() error { return runCmd("loginctl", "lock-session") }
			signOutNative = func(force bool) error { return runCmd("loginctl", "terminate-user", "") }
		}
		if _, err := exec.LookPath("systemctl"); err == nil {
			suspendNative = func(hibernate bool) error {
				if hibernate {
					return runCmd("systemctl", "hibernate")
				}
				return runCmd("systemctl", "suspend")
			}
			powerOffNative = func(reboot bool, delay int, force bool) error {
				args := []string{}
				if force {
					args = append(args, "--force")
				}
				if reboot {
					args = append(args, "reboot")
				} else {
					args = append(args, "poweroff")
				}
				if delay > 0 {
					// systemd 的 --when 需要绝对时间，这里退化成「先等再执行」
					go func(d int) {
						runCmd("sh", "-c", "sleep "+strconv.Itoa(d)+" && systemctl "+strings.Join(args, " "))
					}(delay)
					return nil
				}
				return runCmd("systemctl", args...)
			}
		}
		if !hasLoginctl {
			if _, err := exec.LookPath("xdg-screensaver"); err == nil {
				lockScreenNative = func() error { return runCmd("xdg-screensaver", "lock") }
			}
		}
	}
}

// runCmd 执行命令并等待结束。
func runCmd(name string, args ...string) error {
	c := exec.Command(name, args...)
	hideConsole(c)
	return c.Run()
}

// ---- 系统音量 ----

func initVolumeHooks() {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("osascript"); err != nil {
			return
		}
		volumeGetNative = func() (float32, bool) {
			out, err := runQuiet("osascript", "-e", "output volume of (get volume settings)")
			if err != nil {
				return 0, false
			}
			n, err := strconv.Atoi(strings.TrimSpace(out))
			if err != nil {
				return 0, false
			}
			return float32(n) / 100, true
		}
		volumeSetNative = func(v float32) error {
			lvl := int(v*100 + 0.5)
			return runCmd("osascript", "-e", "set volume output volume "+strconv.Itoa(lvl))
		}
		muteToggleNative = func() (bool, error) {
			out, err := runQuiet("osascript", "-e", "output muted of (get volume settings)")
			if err != nil {
				return false, err
			}
			muted := strings.TrimSpace(out) == "true"
			script := "set volume without output muted"
			if !muted {
				script = "set volume with output muted"
			}
			return !muted, runCmd("osascript", "-e", script)
		}
		return
	}

	if p := audioBackendCommand("wpctl"); p != "" {
		const sink = "@DEFAULT_AUDIO_SINK@"
		volumeGetNative = func() (float32, bool) {
			out, err := runQuiet(p, "get-volume", sink)
			if err != nil {
				return 0, false
			}
			// "Volume: 0.45 [MUTED]"
			f := strings.Fields(out)
			if len(f) < 2 {
				return 0, false
			}
			v, err := strconv.ParseFloat(f[1], 32)
			if err != nil {
				return 0, false
			}
			return float32(v), true
		}
		volumeSetNative = func(v float32) error {
			return runCmd(p, "set-volume", sink, strconv.Itoa(int(v*100+0.5))+"%")
		}
		muteToggleNative = func() (bool, error) {
			if err := runCmd(p, "set-mute", sink, "toggle"); err != nil {
				return false, err
			}
			out, _ := runQuiet(p, "get-volume", sink)
			return strings.Contains(out, "[MUTED]"), nil
		}
		return
	}
	if p := audioBackendCommand("pactl"); p != "" {
		volumeGetNative = func() (float32, bool) {
			out, err := runQuiet(p, "get-sink-volume", "@DEFAULT_SINK@")
			if err != nil {
				return 0, false
			}
			// "... /  45% ..."
			i := strings.Index(out, "/")
			if i < 0 {
				return 0, false
			}
			f := strings.Fields(out[i+1:])
			if len(f) == 0 {
				return 0, false
			}
			n, err := strconv.Atoi(strings.TrimSuffix(f[0], "%"))
			if err != nil {
				return 0, false
			}
			return float32(n) / 100, true
		}
		volumeSetNative = func(v float32) error {
			return runCmd(p, "set-sink-volume", "@DEFAULT_SINK@", strconv.Itoa(int(v*100+0.5))+"%")
		}
		muteToggleNative = func() (bool, error) {
			if err := runCmd(p, "set-sink-mute", "@DEFAULT_SINK@", "toggle"); err != nil {
				return false, err
			}
			out, _ := runQuiet(p, "get-sink-mute", "@DEFAULT_SINK@")
			return strings.Contains(out, "yes"), nil
		}
		return
	}
	if p := audioBackendCommand("amixer"); p != "" {
		volumeGetNative = func() (float32, bool) {
			out, err := runQuiet(p, "get", "Master")
			if err != nil {
				return 0, false
			}
			if i := strings.Index(out, "["); i >= 0 {
				if j := strings.Index(out[i:], "%]"); j > 0 {
					n, err := strconv.Atoi(out[i+1 : i+j])
					if err == nil {
						return float32(n) / 100, true
					}
				}
			}
			return 0, false
		}
		volumeSetNative = func(v float32) error {
			return runCmd(p, "sset", "Master", strconv.Itoa(int(v*100+0.5))+"%")
		}
		muteToggleNative = func() (bool, error) {
			if err := runCmd(p, "sset", "Master", "toggle"); err != nil {
				return false, err
			}
			out, _ := runQuiet(p, "get", "Master")
			return strings.Contains(out, "[off]"), nil
		}
	}
}
