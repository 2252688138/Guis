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

// hotExeSuffix 热重载编译产物的后缀。
func hotExeSuffix() string { return ".exe" }

// hotCmdAttrs 子进程启动属性：GUI 程序本身没有控制台，这里只是兜底不弹黑框。
func hotCmdAttrs(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// hotKillProcess 强杀子进程（Windows 没有进程组语义，直接 Kill 主进程）。
func hotKillProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
