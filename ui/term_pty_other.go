//go:build !windows && !linux

// guis — 伪终端组件（五）：其它平台（macOS / BSD / wasm / 移动端）
//
// 这些平台暂不提供内置本地 PTY。终端组件本身照常可用：把远程数据
// （SSH / WebSocket / 容器日志）喂给 term.Feed 即可渲染与交互。
// macOS 后续可用 posix_openpt + ptsname 补齐（需 cgo 或 purego 绑定 libc）。
package ui

import (
	"runtime"
)

// defaultShell 默认 shell（stub 保留可编译）。
func defaultShell() (string, []string) {
	return "/bin/sh", nil
}

// startPTY 当前平台不支持本地伪终端。
func startPTY(cols, rows int, opts ShellOptions) (*TermProc, error) {
	return nil, ErrPTYUnsupported
}

// 保留 runtime 引用，便于错误信息里标注平台（wasm 下 GOOS=js）。
var _ = runtime.GOOS
