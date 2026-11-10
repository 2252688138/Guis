//go:build darwin && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// platformIntegrationCaps macOS 端的「系统集成」能力。
//
//   - 全局热键：CGEventTap（hotkey_darwin.go），首次使用时系统会要求
//     「辅助功能 / 输入监控」授权；
//   - 托盘：NSStatusItem（tray_darwin.go）；
//   - 文件拖入：NSDraggingDestination（cocoa 里注册拖动类型）。
func platformIntegrationCaps() []Cap {
	return []Cap{CapHotkey, CapTray, CapDragDrop}
}
