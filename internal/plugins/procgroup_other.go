//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !android && !ios

package plugins

import (
	"errors"
	"os/exec"
)

// procGroup 该平台无进程组能力（如 js/wasm），仅做占位。
type procGroup struct{}

// errNoProcGroup 没拿到进程组。
var errNoProcGroup = errors.New("进程组未建立")

// prepareProc 该平台无需准备。
func prepareProc(cmd *exec.Cmd) {}

// attachProc 该平台返回 nil，调用方退化为单进程终止。
func attachProc(cmd *exec.Cmd) *procGroup { return nil }

// Kill 无法整组终止。
func (g *procGroup) Kill() error { return errNoProcGroup }

// SetMemoryLimit 该平台不支持内存限额。
func (g *procGroup) SetMemoryLimit(bytes int64) error { return nil }

// Close 无句柄需要释放。
func (g *procGroup) Close() {}
