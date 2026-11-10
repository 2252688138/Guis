//go:build (linux || freebsd) && !android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// platformIntegrationCaps X11 端的「系统集成」能力。
//
// 三项都在本包实现（运行时若缺宿主会安全失败）：
//   - 全局热键：XGrabKey（hotkey_x11.go）
//   - 托盘：XEmbed 协议（tray_x11.go）；没有托盘宿主时 NewTray 返回错误
//   - 文件拖入：XDND（drop_x11.go）
func platformIntegrationCaps() []Cap {
	return []Cap{CapHotkey, CapTray, CapDragDrop}
}
