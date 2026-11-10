//go:build js && wasm

// guis — 开机自启（浏览器端）
//
// 浏览器沙箱没有「登录自启」概念，符号保留为空实现，业务用
// HasCap(CapAutoStart) 判断后隐藏入口即可。
package ui

// platformSetAutoStart 浏览器端不支持开机自启。
func platformSetAutoStart(t autoStartTarget, on bool) error { return errNoAutoStart }

// platformAutoStartEnabled 浏览器端永远未开启。
func platformAutoStartEnabled(t autoStartTarget) bool { return false }
