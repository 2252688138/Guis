//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os/exec"

	"guis/internal/hidewin"
)

// createNoWindow 对应 Win32 CREATE_NO_WINDOW：子进程不分配控制台窗口。
const createNoWindow = hidewin.CreateNoWindow

// hideConsole 隐藏子进程控制台窗口（发布构建下不闪黑框）。
// 非 Windows 平台见 proc_unix.go（空实现：Unix 没有「控制台窗口」概念）。
//
// 具体实现统一在 internal/hidewin：同时设 SW_HIDE 与 CREATE_NO_WINDOW，
// 后者让子进程干脆不分配控制台（只设 HideWindow 时仍会先建窗口再隐藏，黑框一闪）。
func hideConsole(cmd *exec.Cmd) {
	hidewin.Apply(cmd)
}
