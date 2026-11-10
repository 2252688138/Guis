//go:build js

// guis — LSP 进程启动（Web / wasm：没有子进程）
package ui

import (
	"fmt"
	"io"
)

// startLSPProcess 浏览器端不能启动本地进程：返回可读错误，页面据此回退内置解析器。
func startLSPProcess(command string, args []string, dir string, env []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	return nil, nil, nil, fmt.Errorf("浏览器环境无法启动语言服务器 %q（请使用桌面版）", command)
}
