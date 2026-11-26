//go:build linux || darwin || freebsd || netbsd || openbsd || android || ios

package plugins

import (
	"errors"
	"os/exec"
	"syscall"
)

// procGroup Unix 下用独立进程组表示：插件进程跑在 Setpgid 出来的组长进程里，
// kill(-pgid) 一次带走整棵进程树。
type procGroup struct {
	pid int
}

// errNoProcGroup 没拿到进程组。
var errNoProcGroup = errors.New("进程组未建立")

// prepareProc 启动前把子进程放进独立进程组（必须早于 Start）。
func prepareProc(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// attachProc 记录组长 pid，供整组终止。
func attachProc(cmd *exec.Cmd) *procGroup {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return &procGroup{pid: cmd.Process.Pid}
}

// Kill 结束整个进程组。
func (g *procGroup) Kill() error {
	if g == nil || g.pid <= 0 {
		return errNoProcGroup
	}
	return syscall.Kill(-g.pid, syscall.SIGKILL)
}

// SetMemoryLimit Unix 下暂不支持内存限额（RSS 需 cgroup）。
func (g *procGroup) SetMemoryLimit(bytes int64) error { return nil }

// Close Unix 下无额外句柄需要释放。
func (g *procGroup) Close() {}
