//go:build !windows && !js && !android && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "os/exec"

// hotExeSuffix 热重载编译产物的后缀（类 Unix 平台没有 .exe）。
func hotExeSuffix() string { return "" }

// hotCmdAttrs 子进程启动属性：类 Unix 平台沿用默认（同一进程组，Ctrl+C 一起收到）。
func hotCmdAttrs(cmd *exec.Cmd) {}

// hotKillProcess 强杀子进程。
func hotKillProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
