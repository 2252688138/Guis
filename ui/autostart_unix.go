//go:build !windows && !js

// guis — 开机自启（类 Unix 端）
//
// 落地方式（都是当前用户级，无需 root）：
//
//	Linux/BSD  ~/.config/autostart/<name>.desktop（XDG Autostart 规范，
//	           遵循 $XDG_CONFIG_HOME；GNOME/KDE/XFCE 等桌面通用）
//	macOS      ~/Library/LaunchAgents/<name>.plist（RunAtLoad，登录时加载）
//	android/ios 沙盒内不允许自启，返回「不支持」
package ui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// platformSetAutoStart 按 GOOS 分派：Linux/BSD 写 .desktop，macOS 写 plist。
func platformSetAutoStart(t autoStartTarget, on bool) error {
	switch runtime.GOOS {
	case "android", "ios":
		return errNoAutoStart
	case "darwin":
		return setLaunchAgent(t, on)
	}
	return setXDGAutostart(t, on)
}

// platformAutoStartEnabled 自启文件是否存在。
func platformAutoStartEnabled(t autoStartTarget) bool {
	switch runtime.GOOS {
	case "android", "ios":
		return false
	case "darwin":
		p, err := launchAgentPath(t)
		if err != nil {
			return false
		}
		_, err = os.Stat(p)
		return err == nil
	}
	p, err := xdgAutostartPath(t)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// ---- Linux / BSD：XDG Autostart ----

// xdgAutostartPath ~/.config/autostart/<name>.desktop（遵循 XDG_CONFIG_HOME）。
func xdgAutostartPath(t autoStartTarget) (string, error) {
	base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", errors.New("无法定位用户主目录，无法设置开机自启")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "autostart", t.Name+".desktop"), nil
}

// setXDGAutostart 写入 / 删除 .desktop 自启文件。
func setXDGAutostart(t autoStartTarget, on bool) error {
	p, err := xdgAutostartPath(t)
	if err != nil {
		return err
	}
	if !on {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(p, []byte(xdgDesktopEntry(t)), 0o644)
}

// xdgDesktopEntry 生成 Desktop Entry 内容（Exec 按规范加引号并转义）。
func xdgDesktopEntry(t autoStartTarget) string {
	exec := make([]string, 0, len(t.Args)+1)
	exec = append(exec, desktopQuote(t.Exe))
	for _, a := range t.Args {
		exec = append(exec, desktopQuote(a))
	}
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Version=1.0\n")
	b.WriteString("Name=" + strings.ReplaceAll(t.Comment, "\n", " ") + "\n")
	b.WriteString("Comment=开机自动启动\n")
	b.WriteString("Exec=" + strings.Join(exec, " ") + "\n")
	b.WriteString("Terminal=false\n")
	// GNOME 系桌面会把未标记的文件当「已禁用」，显式打开；
	// 延迟 3 秒等桌面会话就绪，避免登录瞬间抢不到显示服务。
	b.WriteString("X-GNOME-Autostart-enabled=true\n")
	b.WriteString("X-GNOME-Autostart-Delay=3\n")
	return b.String()
}

// desktopQuote 按 Desktop Entry 规范转义单个参数：
// 含空白 / 引号 / 反斜杠 / $ / ` 时加双引号，内部 " 与 \ 转义、% 写成 %%。
func desktopQuote(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"'\\$`") && !strings.Contains(s, "%") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%")
	return `"` + r.Replace(s) + `"`
}

// ---- macOS：LaunchAgent ----

// launchAgentPath ~/Library/LaunchAgents/<name>.plist。
func launchAgentPath(t autoStartTarget) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("无法定位用户主目录，无法设置开机自启")
	}
	return filepath.Join(home, "Library", "LaunchAgents", t.Name+".plist"), nil
}

// setLaunchAgent 写入 / 删除 LaunchAgent plist，并尽力同步 launchctl。
//
// launchctl 失败不影响结果：plist 会在下次登录时由 launchd 自动加载。
func setLaunchAgent(t autoStartTarget, on bool) error {
	p, err := launchAgentPath(t)
	if err != nil {
		return err
	}
	if !on {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		_ = exec.Command("launchctl", "unload", p).Run()
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(p, []byte(launchAgentPlist(t)), 0o644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "load", "-w", p).Run()
	return nil
}

// launchAgentPlist 生成 LaunchAgent plist（RunAtLoad + 单实例保活关闭）。
func launchAgentPlist(t autoStartTarget) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" ` +
		`"http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString("<dict>\n")
	b.WriteString("\t<key>Label</key>\n\t<string>" + xmlEscape(t.Name) + "</string>\n")
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	b.WriteString("\t\t<string>" + xmlEscape(t.Exe) + "</string>\n")
	for _, a := range t.Args {
		b.WriteString("\t\t<string>" + xmlEscape(a) + "</string>\n")
	}
	b.WriteString("\t</array>\n")
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	// 用户登录后自启，不需要 KeepAlive（崩溃后不自动拉起，避免与手动退出打架）。
	b.WriteString("\t<key>KeepAlive</key>\n\t<false/>\n")
	if c := strings.TrimSpace(t.Comment); c != "" {
		b.WriteString("\t<key>Comment</key>\n\t<string>" + xmlEscape(c) + "</string>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

// xmlEscape plist 文本节点转义。
func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// writeFileAtomic 先写临时文件再 rename：自启文件不会出现半截内容被系统读到。
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
