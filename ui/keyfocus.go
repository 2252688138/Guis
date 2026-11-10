// guis — 代码编辑器：焦点按键拦截器注册表
//
// 框架的按键分发顺序是：修饰键 → Ctrl+K 命令面板 → 文本编辑（Ctrl+A/C/X/V）
// → 快捷键 → Tab 焦点切换 → 焦点控件 OnKey。代码编辑器需要在 Tab 焦点切换、
// 文本编辑分发之前拿到原始按键（Tab 缩进、Shift+Tab 反缩进、Alt+↑↓ 移动行、
// Ctrl+S 保存），因此提供一个「焦点控件优先拦截」的注册表。
//
// 用法（组件内部已自动登记）：
//
//	ui.RegisterKeyHandler("my-key", func(vk uint32) bool { return true }) // true = 已消费
package ui

// focusKeyHandlers 持有焦点的控件注册的按键拦截器（按节点 Key，每帧构建时覆盖）。
var focusKeyHandlers = map[string]func(vk uint32) bool{}

// RegisterKeyHandler 登记按键拦截器：焦点在该 Key 上时优先收到 KeyDown。
// 返回 true 表示已消费，框架不再把按键分发给其它环节（焦点切换 / 文本编辑 / 全局快捷键）。
func RegisterKeyHandler(key string, fn func(vk uint32) bool) {
	if key == "" || fn == nil {
		return
	}
	focusKeyHandlers[key] = fn
}

// UnregisterKeyHandler 移除按键拦截器。
func UnregisterKeyHandler(key string) {
	delete(focusKeyHandlers, key)
}
