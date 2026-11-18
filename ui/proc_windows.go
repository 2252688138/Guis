//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os/exec"
	"syscall"
)

// createNoWindow 对应 Win32 CREATE_NO_WINDOW：子进程不分配控制台窗口。
const createNoWindow = 0x08000000

// hideConsole 隐藏子进程控制台窗口（发布构建下不闪黑框）。
// 非 Windows 平台见 proc_unix.go（空实现：Unix 没有「控制台窗口」概念）。
//
// 只设 HideWindow 时，系统会先给控制台程序分配窗口再隐藏，仍可能一闪；
// 因此同时设 CREATE_NO_WINDOW（0x08000000）：子进程干脆不分配控制台。
func hideConsole(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
