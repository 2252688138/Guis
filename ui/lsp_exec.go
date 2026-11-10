//go:build !js

// guis — LSP 进程启动（桌面 / 移动端：os/exec）
package ui

import (
	"io"
	"os"
	"os/exec"
)

// startLSPProcess 启动语言服务器，返回 stdin / stdout 与停止函数。
func startLSPProcess(command string, args []string, dir string, env []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	cmd := exec.Command(command, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), env...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	// stderr 合并进 stdout 会破坏分帧协议，这里单独丢弃 / 透传到日志。
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}
	stop := func() error {
		if cmd.Process == nil {
			return nil
		}
		_ = cmd.Process.Kill()
		return cmd.Wait()
	}
	return stdin, stdout, stop, nil
}
