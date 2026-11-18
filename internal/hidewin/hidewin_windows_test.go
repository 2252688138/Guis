//go:build windows

package hidewin

import (
	"os/exec"
	"syscall"
	"testing"
)

// TestApplyHidesConsole Apply 应同时设置 SW_HIDE 与 CREATE_NO_WINDOW，并返回同一个 cmd。
func TestApplyHidesConsole(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo")
	if got := Apply(cmd); got != cmd {
		t.Fatal("Apply 应返回同一个 *exec.Cmd")
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow {
		t.Error("应设置 HideWindow")
	}
	if cmd.SysProcAttr.CreationFlags&CreateNoWindow == 0 {
		t.Errorf("应设置 CREATE_NO_WINDOW，实际 0x%X", cmd.SysProcAttr.CreationFlags)
	}
}

// TestApplyKeepsExistingFlags 已有 CreationFlags 时按位或追加，不能覆盖。
func TestApplyKeepsExistingFlags(t *testing.T) {
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
	Apply(cmd)
	if want := uint32(0x00000008 | CreateNoWindow); cmd.SysProcAttr.CreationFlags != want {
		t.Errorf("CreationFlags 应叠加，want 0x%X got 0x%X", want, cmd.SysProcAttr.CreationFlags)
	}
}

// TestApplyNil 空指针安全。
func TestApplyNil(t *testing.T) {
	if Apply(nil) != nil {
		t.Error("nil 应原样返回 nil")
	}
}
