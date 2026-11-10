//go:build linux

// guis — 伪终端组件（五）：Linux PTY 后端
//
// 直接走 /dev/ptmx：open → 解锁（TIOCSPTLCK）→ 取从端号（TIOCGPTN）→ 打开
// /dev/pts/N → 子进程 setsid + 以从端为控制终端（Setctty）→ 其余与普通进程一致。
// 主端一个 fd 同时做读写，窗口尺寸用 TIOCSWINSZ 同步给内核。
package ui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"
)

// winSize struct winsize。
type winSize struct {
	Row, Col, Xpixel, Ypixel uint16
}

// defaultShell Linux 默认 shell（$SHELL → /bin/bash → /bin/sh）。
func defaultShell() (string, []string) {
	if sh := os.Getenv("SHELL"); sh != "" {
		if _, err := os.Stat(sh); err == nil {
			return sh, []string{"-l"}
		}
	}
	if _, err := os.Stat("/bin/bash"); err == nil {
		return "/bin/bash", []string{"-l"}
	}
	return "/bin/sh", nil
}

// ioctl 调用 ioctl(fd, req, arg)。
func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// setWinSize 设置 PTY 窗口尺寸。
func setWinSize(fd int, cols, rows int) error {
	if cols < 2 {
		cols = 2
	}
	if rows < 2 {
		rows = 2
	}
	ws := winSize{Row: uint16(rows), Col: uint16(cols)}
	return ioctl(fd, syscall.TIOCSWINSZ, unsafe.Pointer(&ws))
}

// startPTY 打开 /dev/ptmx 并启动 shell。
func startPTY(cols, rows int, opts ShellOptions) (*TermProc, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("ui: 打开 /dev/ptmx 失败：%w", err)
	}
	mfd := int(master.Fd())

	// 解锁从端
	var unlock int32
	if err := ioctl(mfd, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		master.Close()
		return nil, fmt.Errorf("ui: TIOCSPTLCK 失败：%w", err)
	}
	// 取从端编号
	var ptn uint32
	if err := ioctl(mfd, syscall.TIOCGPTN, unsafe.Pointer(&ptn)); err != nil {
		master.Close()
		return nil, fmt.Errorf("ui: TIOCGPTN 失败：%w", err)
	}
	slavePath := "/dev/pts/" + strconv.Itoa(int(ptn))
	slave, err := os.OpenFile(slavePath, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, fmt.Errorf("ui: 打开 %s 失败：%w", slavePath, err)
	}
	_ = setWinSize(mfd, cols, rows)

	app, args := opts.Command, opts.Args
	if app == "" {
		app, args = defaultShell()
	}
	cmd := exec.Command(app, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.Dir = opts.Dir
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		slave.Close()
		master.Close()
		return nil, fmt.Errorf("ui: 启动 %s 失败：%w", app, err)
	}
	// 父进程不再需要从端（子进程已继承）
	slave.Close()

	proc := &TermProc{in: master, out: master}
	proc.resizeFn = func(c, r int) error { return setWinSize(mfd, c, r) }
	proc.closeFn = func() error {
		// 关主端会让会话收到 SIGHUP；再补一刀 SIGKILL 防止子进程逃逸。
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		return nil
	}
	proc.waitFn = func() (int, error) {
		err := cmd.Wait()
		if err == nil {
			return 0, nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
				if ws.Signaled() {
					return 128 + int(ws.Signal()), nil
				}
				return ws.ExitStatus(), nil
			}
			return ee.ExitCode(), nil
		}
		return -1, err
	}
	proc.pidFn = func() int {
		if cmd.Process == nil {
			return 0
		}
		return cmd.Process.Pid
	}
	return proc, nil
}
