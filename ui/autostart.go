// guis — 开机自启（跨平台统一入口）
//
// 一套代码三端落地，全部是「当前用户级」，不需要管理员 / root：
//
//	Windows    HKCU\Software\Microsoft\Windows\CurrentVersion\Run 键值
//	Linux/BSD  ~/.config/autostart/<name>.desktop（XDG Autostart 规范）
//	macOS      ~/Library/LaunchAgents/<name>.plist（RunAtLoad）
//	Web / 移动端 不支持（HasCap(CapAutoStart) 为假）
//
// 约定与托盘一致：符号永不缺失，不支持时返回错误；业务先用 HasCap 判断
// 是否展示入口，再用 SetAutoStart / AutoStartEnabled 读写真实系统状态
// （不落本地配置：用户在系统设置里手工删掉也能正确反映）。
//
// 典型用法（关闭到托盘的程序：开机后直接驻留托盘，不弹主窗口）：
//
//	spec := ui.AutoStartSpec{Name: "MyApp", Args: []string{"--tray"}}
//	if ui.HasCap(ui.CapAutoStart) {
//		_ = ui.SetAutoStart(true, spec)
//	}
//	on := ui.AutoStartEnabled(spec)
//
// 绿色版 / 便携版可设环境变量 GUIS_AUTOSTART_CMD（"路径" 参数…）覆盖
// 注册用的命令行，避免把临时目录里的 exe 写进自启项。
package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// AutoStartSpec 开机自启的注册信息。
type AutoStartSpec struct {
	// Name 注册项名称：Windows 注册表值名 / Linux .desktop 文件名 / macOS plist 文件名。
	// 为空时取可执行文件名（不含扩展名）；非安全字符会归一化成 '-'。
	Name string
	// Args 启动参数（例如 "--tray"：开机后直接驻留托盘，不弹主窗口）。
	Args []string
	// Comment 描述：Linux .desktop 的 Name 列 / macOS plist 的用途说明，可空。
	Comment string
}

// autoStartTarget 归一化后的自启目标（路径已解析为绝对路径）。
type autoStartTarget struct {
	Name    string   // 文件名安全的注册名
	Exe     string   // 可执行文件绝对路径
	Args    []string // 启动参数
	Comment string   // 展示名 / 说明
}

// errNoAutoStart 当前平台不支持开机自启。
var errNoAutoStart = errors.New("ui: 当前平台不支持开机自启（可用 HasCap(CapAutoStart) 判断）")

// AutoStartSupported 当前平台是否支持开机自启（等价 HasCap(CapAutoStart)）。
func AutoStartSupported() bool { return HasCap(CapAutoStart) }

// SetAutoStart 开启 / 关闭开机自启（当前用户级，无需管理员权限）。
func SetAutoStart(on bool, spec AutoStartSpec) error {
	if !AutoStartSupported() {
		return errNoAutoStart
	}
	t, err := resolveAutoStartSpec(spec)
	if err != nil {
		return err
	}
	return platformSetAutoStart(t, on)
}

// AutoStartEnabled 是否已注册开机自启（直接读注册表 / .desktop / plist）。
func AutoStartEnabled(spec AutoStartSpec) bool {
	if !AutoStartSupported() {
		return false
	}
	t, err := resolveAutoStartSpec(spec)
	if err != nil {
		return false
	}
	return platformAutoStartEnabled(t)
}

// resolveAutoStartSpec 补齐默认值：可执行文件绝对路径、注册名、展示名。
func resolveAutoStartSpec(spec AutoStartSpec) (autoStartTarget, error) {
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		return autoStartTarget{}, errors.New("无法定位可执行文件，无法设置开机自启")
	}
	args := append([]string(nil), spec.Args...)

	// 环境变量覆盖：绿色版 / 便携版 / 容器里把自启项指向真实安装路径。
	if v := strings.TrimSpace(os.Getenv("GUIS_AUTOSTART_CMD")); v != "" {
		if e, a := splitCommandLine(v); e != "" {
			exe, args = e, a
		}
	}

	if p, err := filepath.EvalSymlinks(exe); err == nil && p != "" {
		exe = p
	}
	if abs, err := filepath.Abs(exe); err == nil && abs != "" {
		exe = abs
	}

	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	}
	name = autoStartFileName(name)
	if name == "" {
		return autoStartTarget{}, errors.New("开机自启项名称为空")
	}

	comment := strings.TrimSpace(spec.Comment)
	if comment == "" {
		comment = name
	}

	out := make([]string, 0, len(args))
	for _, a := range args {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return autoStartTarget{Name: name, Exe: exe, Args: out, Comment: comment}, nil
}

// autoStartFileName 归一化成文件名安全的注册名：ASCII 字母数字与 . _ - 保留，
// 其余字符（空格 / 斜杠 / 中文）折叠成一个 '-'，避免路径拼接与 plist 解析出问题。
func autoStartFileName(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(s) {
		safe := r == '.' || r == '_' || r == '-' ||
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if safe {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-.")
}

// splitCommandLine 把 "C:\Path With Space\app.exe" --tray 拆成路径与参数
// （只处理双引号，够用于自启命令）。
func splitCommandLine(s string) (exe string, args []string) {
	var cur strings.Builder
	quoted, has := false, false
	flush := func() {
		if has {
			if exe == "" {
				exe = cur.String()
			} else {
				args = append(args, cur.String())
			}
		}
		cur.Reset()
		has = false
	}
	for _, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
			has = true
		case (r == ' ' || r == '\t') && !quoted:
			flush()
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	flush()
	return exe, args
}
