// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// PadButton 手柄按钮。
type PadButton int

// 手柄按钮常量（与 XInput 位掩码映射）。
const (
	PadUp PadButton = iota
	PadDown
	PadLeft
	PadRight
	PadStart
	PadBack
	PadL3
	PadR3
	PadLB
	PadRB
	PadA
	PadB
	PadX
	PadY
)

// 手柄轴下标。
const (
	PadAxisLX = iota
	PadAxisLY
	PadAxisRX
	PadAxisRY
	PadAxisLT
	PadAxisRT
	padAxisCount
)

// Gamepad 手柄状态（先调用 Update 轮询，再读 Button/Pressed/Axes）。
type Gamepad struct {
	// Index 设备序号（0~3）。
	Index int
	// Connected 是否已连接。
	Connected bool
	// Axes 轴值（LX/LY/RX/RY 为 -1~1，LT/RT 为 0~1）。
	Axes [padAxisCount]float32
	// Deadzone 摇杆死区（默认 0.15）。
	Deadzone float32

	cur  map[PadButton]bool
	prev map[PadButton]bool
}

// NewGamepad 创建手柄状态（默认死区 0.15）。
func NewGamepad(index int) *Gamepad {
	return &Gamepad{
		Index:    index,
		Deadzone: 0.15,
		cur:      map[PadButton]bool{},
		prev:     map[PadButton]bool{},
	}
}

// Update 轮询设备状态。
func (g *Gamepad) Update() {
	if g == nil {
		return
	}
	if g.cur == nil {
		g.cur = map[PadButton]bool{}
		g.prev = map[PadButton]bool{}
	}
	for k, v := range g.cur {
		g.prev[k] = v
	}
	if g.Deadzone <= 0 {
		g.Deadzone = 0.15
	}
	st, ok := padPoll(g.Index)
	g.Connected = ok
	if !ok {
		for k := range g.cur {
			delete(g.cur, k)
		}
		g.Axes = [padAxisCount]float32{}
		return
	}
	for k := range g.cur {
		delete(g.cur, k)
	}
	for btn, bit := range padButtonBits {
		if st.buttons&bit != 0 {
			g.cur[btn] = true
		}
	}
	g.Axes[PadAxisLX] = applyDeadzone(st.lx, g.Deadzone)
	g.Axes[PadAxisLY] = applyDeadzone(st.ly, g.Deadzone)
	g.Axes[PadAxisRX] = applyDeadzone(st.rx, g.Deadzone)
	g.Axes[PadAxisRY] = applyDeadzone(st.ry, g.Deadzone)
	g.Axes[PadAxisLT] = float32(st.lt) / 255
	g.Axes[PadAxisRT] = float32(st.rt) / 255
	if g.Axes[PadAxisLT] < 0.1 {
		g.Axes[PadAxisLT] = 0
	}
	if g.Axes[PadAxisRT] < 0.1 {
		g.Axes[PadAxisRT] = 0
	}
}

// Button 按钮是否按住。
func (g *Gamepad) Button(b PadButton) bool {
	if g == nil || !g.Connected {
		return false
	}
	return g.cur[b]
}

// Pressed 按钮是否刚按下。
func (g *Gamepad) Pressed(b PadButton) bool {
	if g == nil || !g.Connected {
		return false
	}
	return g.cur[b] && !g.prev[b]
}

// Axis 取轴值（越界返回 0）。
func (g *Gamepad) Axis(i int) float32 {
	if g == nil || i < 0 || i >= padAxisCount {
		return 0
	}
	return g.Axes[i]
}

// applyDeadzone 径向死区（归一化后重新映射到 0~1）。
func applyDeadzone(v, dz float32) float32 {
	if v >= -dz && v <= dz {
		return 0
	}
	s := 1
	if v < 0 {
		s = -1
	}
	mag := v * float32(s)
	mag = (mag - dz) / (1 - dz)
	if mag > 1 {
		mag = 1
	}
	return float32(s) * mag
}

// padState 平台无关的原始状态。
type padState struct {
	buttons        uint16
	lt, rt         uint8
	lx, ly, rx, ry float32
}

// padButtonBits 按钮 → XInput 位掩码。
var padButtonBits = map[PadButton]uint16{
	PadUp:    0x0001,
	PadDown:  0x0002,
	PadLeft:  0x0004,
	PadRight: 0x0008,
	PadStart: 0x0010,
	PadBack:  0x0020,
	PadL3:    0x0040,
	PadR3:    0x0080,
	PadLB:    0x0100,
	PadRB:    0x0200,
	PadA:     0x1000,
	PadB:     0x2000,
	PadX:     0x4000,
	PadY:     0x8000,
}
