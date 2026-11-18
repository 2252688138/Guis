//go:build !((linux || freebsd) && !android)

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 非 Linux/X11 平台没有「把系统浏览器窗口挂进来」的能力（macOS 需要 Cocoa 重挂载，
// 暂未实现），统一返回不可用，由 CDP 后端或降级后端接手。

// embedBrowserAvailable 本平台能否真窗口嵌入。
func embedBrowserAvailable() bool { return false }

// newEmbeddedBrowserBackend 真窗口嵌入后端（不可用平台直接返回 false）。
func newEmbeddedBrowserBackend(a *App, opts WebViewOptions) (webViewBackend, bool) {
	return nil, false
}
