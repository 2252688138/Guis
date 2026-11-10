//go:build js && wasm

package ui

import "syscall/js"

// setGateDiag 把本帧的重绘闸门标志写到 window.__gate（诊断用）。
func setGateDiag(v string) {
	if g := js.Global(); !g.IsUndefined() {
		g.Set("__gate", v)
	}
}
