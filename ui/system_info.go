// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os"
	"runtime"
)

// 系统信息与电源状态。
//
//	info := ui.SystemInfo()
//	fmt.Println(info.Locale, info.UserName, info.OS, info.Elevated)
//
//	pw := ui.PowerState()
//	if pw.OnBattery && pw.Percent >= 0 && pw.Percent < 20 { 提示电量 }
//
// 这些信息只依赖系统 API / sysfs，不需要额外权限；读不到的字段留零值
//（Percent 用 -1 表示未知），业务侧据此隐藏展示即可。

// SysInfo 一份系统信息快照。
type SysInfo struct {
	// Locale 系统 UI 语言（BCP-47 风格，如 "zh-CN" / "en-US"）。
	Locale string
	// LocaleName 语言的本地名称（如 "中文（简体）"）；未知为空。
	LocaleName string
	// UserName 当前登录用户名。
	UserName string
	// HostName 计算机名。
	HostName string
	// OS 人类可读的系统描述（如 "Windows 11 (build 22631)" / "Ubuntu 24.04"）。
	OS string
	// OSVersion 系统版本号（如 "10.0.22631"）。
	OSVersion string
	// Arch 处理器架构（amd64 / arm64 ...）。
	Arch string
	// Elevated 当前进程是否以管理员 / root 运行。
	Elevated bool
}

// String 便于日志输出的单行描述。
func (s SysInfo) String() string {
	return s.OS + " / " + s.Arch + " / " + s.Locale + " / " + s.UserName
}

// sysInfoNative 平台原生实现（Windows 读 RtlGetVersion / GetUserDefaultUILanguage）。
var sysInfoNative func() (SysInfo, bool)

// SystemInfo 读取系统信息（读取失败的字段留零值）。
func SystemInfo() SysInfo {
	info := SysInfo{Arch: runtime.GOARCH}
	if sysInfoNative != nil {
		if got, ok := sysInfoNative(); ok {
			if got.Arch == "" {
				got.Arch = runtime.GOARCH
			}
			return got
		}
	}
	// 兜底：只填 Go 能直接拿到的几项
	info.OS = runtime.GOOS
	info.Locale = systemLocaleFallback()
	if h, err := os.Hostname(); err == nil {
		info.HostName = h
	}
	info.UserName = sysUserName()
	return info
}

// SystemLocale 系统 UI 语言（读不到时返回 "en-US"）。
func SystemLocale() string {
	if l := SystemInfo().Locale; l != "" {
		return l
	}
	return "en-US"
}

// IsElevated 当前进程是否以管理员 / root 身份运行。
func IsElevated() bool { return SystemInfo().Elevated }

// PowerStatus 电源与电池状态。
type PowerStatus struct {
	// OnBattery 是否在用电池（台式机恒 false）。
	OnBattery bool
	// Percent 电量百分比 0~100；未知为 -1。
	Percent int
	// Charging 是否正在充电。
	Charging bool
	// ACOnline 是否接了交流电（笔记本拔掉电源为 false）。
	ACOnline bool
	// SaverOn 是否开启系统省电模式。
	SaverOn bool
}

// powerStateNative 平台原生实现（Windows: GetSystemPowerStatus）。
var powerStateNative func() (PowerStatus, bool)

// PowerState 读取电源状态。没有电池的机器返回 {ACOnline: true, Percent: -1}。
func PowerState() PowerStatus {
	if powerStateNative != nil {
		if st, ok := powerStateNative(); ok {
			return st
		}
	}
	return PowerStatus{ACOnline: true, Percent: -1}
}

// HasBattery 本机是否有电池（笔记本 / 平板）。
func HasBattery() bool {
	st := PowerState()
	return st.OnBattery || st.Percent >= 0
}

// sysUserName 跨平台取当前用户名（Windows 用 USERNAME，类 Unix 用 USER）。
func sysUserName() string {
	for _, k := range []string{"USERNAME", "USER", "LOGNAME"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
