//go:build !windows

package hidewin

import "os/exec"

// Apply 非 Windows 平台无控制台窗口问题，原样返回。
func Apply(cmd *exec.Cmd) *exec.Cmd { return cmd }
