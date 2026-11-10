//go:build !js && !android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"os"
)

// entryFn 应用入口（由 SetEntry 在 init() 阶段注册）。
var entryFn func() (Options, func() *Node)

// SetEntry 注册应用入口：返回窗口选项与整页构建函数。
//
// 为什么不直接 Main(opts, view)：Android 的 NativeActivity 在 dlopen 之后
// 立刻回调 ANativeActivity_onCreate，可能早于 main() 被调度。把参数放进
// init() 注册的闭包里，三端（桌面 / 浏览器 / 安卓）才能共用同一个 main()。
func SetEntry(fn func() (Options, func() *Node)) { entryFn = fn }

// Main 三端统一的应用入口（桌面版：Windows / Linux / macOS）。
//
// 业务代码只需要两段：
//
//	func init() { ui.SetEntry(func() (ui.Options, func() *ui.Node) { return opts, view }) }
//	func main() { ui.Main() }
//
// 平台差异全部在 ui 包内消化：
//   - 桌面（本文件）：直接进 Run；
//   - 浏览器（boot_wasm.go）：先 fetch 字体，再进 Run；
//   - Android（main_entry_android.go）：挂到 NativeActivity 的 OnCreate 之后再进 Run。
func Main() {
	fn := entryFn
	if fn == nil {
		fmt.Fprintln(os.Stderr, "[guis] 未注册入口：请在 init() 里调用 ui.SetEntry")
		return
	}
	opts, view := fn()
	if err := Run(opts, view); err != nil {
		fmt.Fprintln(os.Stderr, "[guis] 启动失败:", err)
	}
}
