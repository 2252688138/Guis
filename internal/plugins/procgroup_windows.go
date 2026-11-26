//go:build windows

package plugins

import (
	"errors"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// procGroup 把插件子进程绑到 Windows Job Object 上，保证退出时整棵进程树被清理。
type procGroup struct {
	job windows.Handle
}

// errNoProcGroup 没拿到进程组（绑定失败）。
var errNoProcGroup = errors.New("进程组未建立")

// prepareProc Windows 下启动前无需准备（Job 在 Start 之后绑定）。
func prepareProc(cmd *exec.Cmd) {}

// attachProc 在 cmd.Start() 之后调用，把子进程放进新建的 Job Object。
func attachProc(cmd *exec.Cmd) *procGroup {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return nil
	}
	proc, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		windows.CloseHandle(job)
		return nil
	}
	err = windows.AssignProcessToJobObject(job, proc)
	windows.CloseHandle(proc)
	if err != nil {
		windows.CloseHandle(job)
		return nil
	}
	return &procGroup{job: job}
}

// Kill 结束 Job 内全部进程。
func (g *procGroup) Kill() error {
	if g == nil || g.job == 0 {
		return errNoProcGroup
	}
	return windows.TerminateJobObject(g.job, 1)
}

// SetMemoryLimit 限制 Job 内进程的内存上限（bytes，0 = 不限制）。
func (g *procGroup) SetMemoryLimit(bytes int64) error {
	if g == nil || g.job == 0 {
		return errNoProcGroup
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if bytes > 0 {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY
		info.ProcessMemoryLimit = uintptr(bytes)
	}
	_, err := windows.SetInformationJobObject(
		g.job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	return err
}

// Close 释放 Job 句柄。
func (g *procGroup) Close() {
	if g == nil || g.job == 0 {
		return
	}
	windows.CloseHandle(g.job)
	g.job = 0
}
