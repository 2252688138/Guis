//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os/exec"
	"syscall"
	"testing"
)

// TestHideConsolePreventsConsoleWindow 确保框架层 hideConsole 同时设置
// HideWindow 与 CREATE_NO_WINDOW：只设前者时控制台程序仍会先建窗口再隐藏（黑框一闪）。
func TestHideConsolePreventsConsoleWindow(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit")
	hideConsole(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr 未设置")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("应设置 HideWindow")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Errorf("应设置 CREATE_NO_WINDOW，实际 CreationFlags=0x%X", cmd.SysProcAttr.CreationFlags)
	}
}

// TestHideConsoleKeepsExistingFlags 已有 CreationFlags 时按位或追加，不能覆盖。
func TestHideConsoleKeepsExistingFlags(t *testing.T) {
	const createNewConsole = 0x00000010
	cmd := exec.Command("cmd", "/c", "exit")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
	hideConsole(cmd)
	if cmd.SysProcAttr.CreationFlags&createNewConsole == 0 {
		t.Error("原有 CreationFlags 位被覆盖")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Error("未追加 CREATE_NO_WINDOW")
	}
}
