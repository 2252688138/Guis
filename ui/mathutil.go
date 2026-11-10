// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// 平台无关的小工具（原来 minF / maxF 只定义在 webview.go（windows）与
// webview_wasm.go（js）里，导致 Linux/macOS 端的 office.go 引用 minF 时编译失败）。
package ui

// minF32 两个 float32 取小（跨平台，代替只在 webview*.go 里定义的 minF，
// 否则 Linux/macOS 端的 office.go 编译不过）。
func minF32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// maxF32 两个 float32 取大。
func maxF32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
