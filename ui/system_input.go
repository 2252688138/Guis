// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 键鼠模拟输入（SendInput）。
//
// 用于「自动填充表单」「软件引导演示」「无障碍替代输入」这类场景：
//
//	ui.SendText("张三")                  // 逐字符输入（不依赖输入法）
//	ui.SendKeyCombo(ui.HotCtrl, 'S')     // Ctrl+S
//	ui.MouseClickAt(320, 240, 0)         // 屏幕坐标左键单击
//
// ⚠️ 安全说明：模拟输入等于替用户操作键盘鼠标，会被安全软件标记为可疑行为。
// 仅在你自己的应用里、由用户明确触发的场景使用；不要用它绕过任何系统确认。
// 支持情况见 HasCap(CapInputSim)（目前仅 Windows）。

// 鼠标按键编号（与常见约定一致）。
const (
	// MouseLeft 左键。
	MouseLeft = 0
	// MouseRight 右键。
	MouseRight = 1
	// MouseMiddle 中键。
	MouseMiddle = 2
)

var (
	sendKeyNative   func(mods uint32, vk uint32) error
	sendTextNative  func(s string) error
	mouseMoveNative func(x, y int) error
	mouseClickNative func(x, y int, button int) error
	mouseWheelNative func(delta int) error
)

// SendKey 模拟按下并释放一个按键（vk 为虚拟键码，如 'A' / VK_RETURN）。
func SendKey(vk uint32) error {
	if sendKeyNative == nil {
		return errUnsupported("模拟按键")
	}
	return sendKeyNative(0, vk)
}

// SendKeyCombo 模拟组合键（mods 用 HotCtrl / HotAlt / HotShift / HotWin）。
func SendKeyCombo(mods uint32, vk uint32) error {
	if sendKeyNative == nil {
		return errUnsupported("模拟按键")
	}
	return sendKeyNative(mods, vk)
}

// SendText 逐字符模拟输入一段文本（用 Unicode 扫描码，中文也能直接输入，
// 不经过输入法）。文本较长时会自动分片，避免一次塞满输入队列。
func SendText(s string) error {
	if s == "" {
		return nil
	}
	if sendTextNative == nil {
		return errUnsupported("模拟输入文本")
	}
	return sendTextNative(s)
}

// MouseMoveTo 把鼠标移动到屏幕绝对坐标（物理像素）。
func MouseMoveTo(x, y int) error {
	if mouseMoveNative == nil {
		return errUnsupported("模拟鼠标")
	}
	return mouseMoveNative(x, y)
}

// MouseClickAt 在屏幕绝对坐标单击（button 见 MouseLeft / MouseRight / MouseMiddle）。
func MouseClickAt(x, y int, button int) error {
	if mouseClickNative == nil {
		return errUnsupported("模拟鼠标")
	}
	return mouseClickNative(x, y, button)
}

// MouseWheel 模拟滚轮（delta 为正向下滚；一格通常是 120）。
func MouseWheel(delta int) error {
	if mouseWheelNative == nil {
		return errUnsupported("模拟滚轮")
	}
	return mouseWheelNative(delta)
}

// WaitIdle 是「模拟输入后等一帧」的便捷封装：自动填充时插在人机之间更稳。
func WaitIdle() { RequestRedraw() }
