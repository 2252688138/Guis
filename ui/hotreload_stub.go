//go:build js || android || ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 热重载仅桌面端可用：浏览器 / 安卓 / iOS 上这些入口全部是空实现，
// 保证业务代码在任意平台都能直接调用 ui.SetHotState / ui.HotReloadActive。

// SetHotState 注册热重载现场保存 / 恢复回调（移动端与浏览器端为空操作）。
func SetHotState(save func() []byte, restore func([]byte)) {}

// HotReloadActive 当前进程是否运行在热重载模式下（移动端与浏览器端恒为 false）。
func HotReloadActive() bool { return false }

// hotReloadEnterRun Run 启动时调用：非桌面平台不接管启动流程。
func hotReloadEnterRun() (bool, error) { return false, nil }

// hotStopRequested 移动端与浏览器端不会收到热重载退出请求。
func hotStopRequested() bool { return false }

// hotApplyRestore 非桌面平台没有跨进程现场可恢复。
func hotApplyRestore(a *App) {}

// hotAfterReveal 非桌面平台没有待恢复的最大化状态。
func hotAfterReveal(a *App) {}
