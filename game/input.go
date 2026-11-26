// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "strings"

// TouchPhase 触摸阶段。
type TouchPhase uint8

const (
	// TouchBegin 按下。
	TouchBegin TouchPhase = iota
	// TouchMove 移动。
	TouchMove
	// TouchEnd 抬起。
	TouchEnd
)

// Touch 单个触摸点。
type Touch struct {
	ID    int
	X, Y  float32
	Phase TouchPhase
}

// Input 输入状态表（键鼠 + 触摸）。
//
// 键名一律小写：字母 "w"/"a"，方向键 "arrowup"，修饰键 "shift"/"ctrl"，
// 其余用 ui 侧传入的名称。
type Input struct {
	// Keys 当前按住的键。
	Keys map[string]bool
	// Pressed 本帧按下的键（EndFrame 后清空）。
	Pressed map[string]bool
	// Released 本帧抬起的键（EndFrame 后清空）。
	Released map[string]bool

	// MouseX, MouseY 鼠标在控件内的像素坐标。
	MouseX, MouseY float32
	// MouseDX, MouseDY 本帧鼠标位移。
	MouseDX, MouseDY float32
	// LockMouse 指针锁定：为 true 时 HandleMouse 不再用绝对坐标算差分，
	// 位移全部由 AddMouseDelta 注入（FPS 转视角用）。
	LockMouse bool
	// Wheel 本帧滚轮量。
	Wheel float32
	// Buttons 鼠标按键（0 左、1 右、2 中）是否按住。
	Buttons [3]bool
	// BtnPressed 本帧按下的鼠标键。
	BtnPressed [3]bool

	// Touches 本帧触摸事件列表。
	Touches []Touch

	hasMouse   bool
	lastX      float32
	lastY      float32
	wheelAccum float32
}

// NewInput 创建空的输入状态表。
func NewInput() *Input {
	return &Input{
		Keys:     map[string]bool{},
		Pressed:  map[string]bool{},
		Released: map[string]bool{},
	}
}

func normKey(k string) string { return strings.ToLower(strings.TrimSpace(k)) }

// HandleKey 记录一次按键事件。
func (i *Input) HandleKey(name string, down bool) {
	if i == nil {
		return
	}
	k := normKey(name)
	if k == "" {
		return
	}
	if down {
		if !i.Keys[k] {
			i.Pressed[k] = true
		}
		i.Keys[k] = true
		return
	}
	i.Keys[k] = false
	i.Released[k] = true
}

// HandleMouse 记录鼠标移动/按键（btn < 0 表示仅移动）。
func (i *Input) HandleMouse(x, y float32, btn int, down bool) {
	if i == nil {
		return
	}
	if i.LockMouse {
		// 锁定模式：绝对坐标位移会因光标回位而抖动，只认 AddMouseDelta
		i.MouseX, i.MouseY = x, y
		if btn >= 0 && btn < 3 {
			if down && !i.Buttons[btn] {
				i.BtnPressed[btn] = true
			}
			i.Buttons[btn] = down
		}
		return
	}
	if i.hasMouse {
		i.MouseDX += x - i.lastX
		i.MouseDY += y - i.lastY
	}
	i.lastX, i.lastY = x, y
	i.hasMouse = true
	i.MouseX, i.MouseY = x, y
	if btn >= 0 && btn < 3 {
		if down && !i.Buttons[btn] {
			i.BtnPressed[btn] = true
		}
		i.Buttons[btn] = down
	}
}

// AddMouseDelta 直接累加本帧鼠标位移（指针锁定 / 原始输入设备用）。
func (i *Input) AddMouseDelta(dx, dy float32) {
	if i == nil {
		return
	}
	i.MouseDX += dx
	i.MouseDY += dy
}

// HandleWheel 记录滚轮。
func (i *Input) HandleWheel(dy float32) {
	if i == nil {
		return
	}
	i.Wheel += dy
	i.wheelAccum += dy
}

// HandleTouch 记录触摸事件。
func (i *Input) HandleTouch(id int, x, y float32, phase TouchPhase) {
	if i == nil {
		return
	}
	i.Touches = append(i.Touches, Touch{ID: id, X: x, Y: y, Phase: phase})
}

// KeyDown 键是否按住。
func (i *Input) KeyDown(name string) bool {
	if i == nil {
		return false
	}
	return i.Keys[normKey(name)]
}

// KeyPressed 本帧是否刚按下。
func (i *Input) KeyPressed(name string) bool {
	if i == nil {
		return false
	}
	return i.Pressed[normKey(name)]
}

// KeyReleased 本帧是否刚抬起。
func (i *Input) KeyReleased(name string) bool {
	if i == nil {
		return false
	}
	return i.Released[normKey(name)]
}

// Axis 返回 -1 / 0 / 1 的轴值（neg 键为负方向，pos 键为正方向）。
func (i *Input) Axis(neg, pos string) float32 {
	var v float32
	if i.KeyDown(neg) {
		v -= 1
	}
	if i.KeyDown(pos) {
		v += 1
	}
	return v
}

// EndFrame 帧末清理一次性状态（必须在每帧逻辑处理完之后调用）。
func (i *Input) EndFrame() {
	if i == nil {
		return
	}
	for k := range i.Pressed {
		delete(i.Pressed, k)
	}
	for k := range i.Released {
		delete(i.Released, k)
	}
	i.BtnPressed = [3]bool{}
	i.MouseDX, i.MouseDY = 0, 0
	i.Wheel = 0
	i.wheelAccum = 0
	i.Touches = i.Touches[:0]
}

// Reset 清空全部输入状态（失焦时调用，避免按键卡住）。
func (i *Input) Reset() {
	if i == nil {
		return
	}
	for k := range i.Keys {
		delete(i.Keys, k)
	}
	i.EndFrame()
	i.Buttons = [3]bool{}
}
