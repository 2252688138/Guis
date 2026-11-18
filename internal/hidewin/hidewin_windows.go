//go:build windows

// Package hidewin 让子进程静默启动。
//
// GUI 子系统（-H windowsgui）的二进制本身没有控制台，直接 spawn 控制台程序
// （cmd / powershell / certutil / tzutil / route / pip …）时系统会为子进程
// 分配一个新控制台，表现为一闪而过的黑色窗口。加上 HideWindow + CREATE_NO_WINDOW
// 后子进程彻底不分配控制台。
package hidewin

import (
	"os/exec"
	"syscall"
)

// CreateNoWindow 对应 Win32 CREATE_NO_WINDOW：子进程不分配控制台窗口。
const CreateNoWindow = 0x08000000

// Apply 给 cmd 加上隐藏窗口的启动参数（幂等，可重复调用）。
func Apply(cmd *exec.Cmd) *exec.Cmd {
	if cmd == nil {
		return nil
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= CreateNoWindow
	return cmd
}
