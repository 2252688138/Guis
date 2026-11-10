//go:build windows

// guis — 开机自启（Windows 端）
//
// 落地方式：HKCU\Software\Microsoft\Windows\CurrentVersion\Run 键值
//
//   - 当前用户级、无需管理员，也不触发 UAC；
//   - 任务管理器「启动应用」页可直接看到并禁用该项；
//   - 读取时以注册表真实内容为准：用户手工删除 / 换了安装目录都能正确反映。
package ui

import (
	"strings"

	"guis/internal/win32"
)

// platformSetAutoStart 写入 / 删除当前用户登录自启项。
func platformSetAutoStart(t autoStartTarget, on bool) error {
	return win32.SetRunAtLogin(t.Name, autoStartCommand(t), on)
}

// platformAutoStartEnabled 注册表里是否已存在本程序的自启项。
//
// 同名项可能指向旧路径（换过安装目录 / 便携版挪过位置），只有命令里包含
// 当前 exe 才算「已开启」，否则视为未开启（用户重开一次即可覆盖成新路径）。
func platformAutoStartEnabled(t autoStartTarget) bool {
	v, ok := win32.RunAtLogin(t.Name)
	if !ok || strings.TrimSpace(v) == "" {
		return false
	}
	return strings.Contains(strings.ToLower(v), strings.ToLower(t.Exe))
}

// autoStartCommand 拼接 Windows 命令行：路径与参数含空格时加双引号。
func autoStartCommand(t autoStartTarget) string {
	var b strings.Builder
	b.WriteString(winQuote(t.Exe))
	for _, a := range t.Args {
		b.WriteByte(' ')
		b.WriteString(winQuote(a))
	}
	return b.String()
}

// winQuote 命令行参数转义（Windows 规则：含空格/制表符/引号才需要引号）。
func winQuote(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
