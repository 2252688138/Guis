// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"sync"
)

// VirtualStickOpts 虚拟摇杆参数。

type VirtualStickOpts struct {
	// Key 状态标识（多个摇杆区分，默认 "virtualstick"）。
	Key string
	// Size 底座直径（默认 140）。
	Size float32
	// KnobSize 摇杆球直径（默认 Size×0.42）。
	KnobSize float32
	// Left / Top 相对父容器左上角的位置（默认 24,24）。
	Left, Top float32
	// Deadzone 死区（0~1，默认 0.15）。
	Deadzone float32
	// Follow 浮动摇杆：按下位置作为中心（视觉底座仍画在原位置）。
	Follow bool
	// BaseColor / KnobColor 底座与摇杆球颜色（默认半透明白）。
	BaseColor, KnobColor string
	// OnAxis 轴变化回调（-1~1，已做死区与圆形裁切；松手时回调一次 0,0）。
	OnAxis func(x, y float32)
}

type stickState struct {
	pointer          int
	ax, ay           float32
	centerX, centerY float32
	active           bool
}

var (
	vpadMu     sync.Mutex
	vpadSticks = map[string]*stickState{}
	vpadBtns   = map[string]bool{}
)

func vpadStick(key string) *stickState {
	st := vpadSticks[key]
	if st == nil {
		st = &stickState{pointer: -1}
		vpadSticks[key] = st
	}
	return st
}

// VirtualStick 虚拟摇杆（支持多点触控：只跟踪第一个按下的手指）。
//
// 返回一个绝对定位的叠层控件，内部走 OnTouch，不会把事件漏给下层视口。
// 轴值用 VirtualStickAxis(key) 读取，或在 OnAxis 里接收。
func VirtualStick(o *VirtualStickOpts) *Node {
	if o == nil {
		o = &VirtualStickOpts{}
	}
	key := o.Key
	if key == "" {
		key = "virtualstick"
	}
	size := o.Size
	if size <= 0 {
		size = 140
	}
	knob := o.KnobSize
	if knob <= 0 {
		knob = size * 0.42
	}
	dead := o.Deadzone
	if dead <= 0 {
		dead = 0.15
	}
	if dead > 0.95 {
		dead = 0.95
	}
	baseColor := o.BaseColor
	if baseColor == "" {
		baseColor = "rgba(255,255,255,0.16)"
	}
	knobColor := o.KnobColor
	if knobColor == "" {
		knobColor = "rgba(255,255,255,0.55)"
	}
	radius := size / 2
	kr := knob / 2

	vpadMu.Lock()
	st := vpadStick(key)
	ax, ay := st.ax, st.ay
	vpadMu.Unlock()

	// 摇杆球偏移（视觉上跟手，FK 不参与输入）
	travel := radius - kr*0.6
	kx := radius - kr + ax*travel
	ky := radius - kr + ay*travel

	base := &Node{Kind: KBox, Style: Style{
		Opacity: 1, Width: size, Height: size,
		Radius: [4]float32{radius, radius, radius, radius},
		Bg:     C(baseColor), BgSet: true,
	}}
	knobStyle := Style{
		Opacity: 1, Width: knob, Height: knob,
		Radius: [4]float32{kr, kr, kr, kr},
		Bg:     C(knobColor), BgSet: true,
	}
	knobStyle.AbsTL(kx, ky)
	knobNode := &Node{Kind: KBox, Style: knobStyle}

	n := Stack(base, knobNode)
	n.Style.Width, n.Style.Height = size, size
	n.Style.AbsTL(o.Left, o.Top)
	n.OnTouch = func(x, y float32, phase int, id int) {
		vpadMu.Lock()
		st := vpadStick(key)
		if phase == 0 {
			if st.pointer >= 0 && st.pointer != id {
				vpadMu.Unlock()
				return
			}
			st.pointer = id
			st.active = true
			if o.Follow {
				st.centerX, st.centerY = x, y
			} else {
				st.centerX, st.centerY = radius, radius
			}
		} else if st.pointer != id {
			vpadMu.Unlock()
			return
		}
		if phase == 2 {
			st.pointer = -1
			st.active = false
			st.ax, st.ay = 0, 0
			cb := o.OnAxis
			vpadMu.Unlock()
			if cb != nil {
				cb(0, 0)
			}
			return
		}
		ax, ay := stickAxis(x-st.centerX, y-st.centerY, radius, dead)
		st.ax, st.ay = ax, ay
		cb := o.OnAxis
		vpadMu.Unlock()
		if cb != nil {
			cb(ax, ay)
		}
	}
	return n
}

// stickAxis 把触点偏移转换为带死区的归一化轴值（长度 ≤ 1）。
func stickAxis(dx, dy, maxR, dead float32) (float32, float32) {
	if maxR <= 0 {
		return 0, 0
	}
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l <= 0 {
		return 0, 0
	}
	dz := dead * maxR
	if l <= dz {
		return 0, 0
	}
	mag := l
	if mag > maxR {
		mag = maxR
	}
	t := (mag - dz) / (maxR - dz)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return dx / l * t, dy / l * t
}

// VirtualStickAxis 读当前摇杆轴值（不存在或未操作时为 0,0）。
func VirtualStickAxis(key string) (x, y float32) {
	if key == "" {
		key = "virtualstick"
	}
	vpadMu.Lock()
	defer vpadMu.Unlock()
	if st := vpadSticks[key]; st != nil {
		return st.ax, st.ay
	}
	return 0, 0
}

// ResetVirtualStick 重置摇杆状态（切换关卡 / 暂停时调用）。
func ResetVirtualStick(key string) {
	if key == "" {
		key = "virtualstick"
	}
	vpadMu.Lock()
	defer vpadMu.Unlock()
	delete(vpadSticks, key)
	vpadBtns = map[string]bool{}
}

// VirtualButtonOpts 触屏按钮参数。

type VirtualButtonOpts struct {
	// Key 状态标识（默认 "virtualbutton"）。
	Key string
	// Size 直径（默认 72）。
	Size float32
	// Left / Top 相对父容器左上角的位置（默认 24,24）。
	Left, Top float32
	// Label 按钮文字（可为空，纯色圆钮）。
	Label string
	// Color / TextColor 颜色（默认半透明白底 / 黑字）。
	Color, TextColor string
	// OnPress 按下 / 松手回调。
	OnPress func(down bool)
}

// VirtualButton 触屏按钮（支持多指，各按钮互不干扰）。
func VirtualButton(o *VirtualButtonOpts) *Node {
	if o == nil {
		o = &VirtualButtonOpts{}
	}
	key := o.Key
	if key == "" {
		key = "virtualbutton"
	}
	size := o.Size
	if size <= 0 {
		size = 72
	}
	color := o.Color
	if color == "" {
		color = "rgba(255,255,255,0.22)"
	}
	textColor := o.TextColor
	if textColor == "" {
		textColor = "#ffffff"
	}
	r := size / 2

	vpadMu.Lock()
	down := vpadBtns[key]
	vpadMu.Unlock()
	bg := color
	if down {
		bg = "rgba(255,255,255,0.45)"
	}

	st := Style{
		Opacity: 1, Width: size, Height: size,
		Radius: [4]float32{r, r, r, r},
		Bg:     C(bg), BgSet: true,
		TextAlign: AlignCenter, CenterY: true,
	}
	st.AbsTL(o.Left, o.Top)
	n := &Node{Kind: KBox, Text: Tr(o.Label), Style: st}
	n.Style.Color = C(textColor)
	n.OnTouch = func(x, y float32, phase int, id int) {
		if phase == 1 {
			return
		}
		vpadMu.Lock()
		prev := vpadBtns[key]
		if phase == 0 {
			vpadBtns[key] = true
		} else {
			vpadBtns[key] = false
		}
		now := vpadBtns[key]
		vpadMu.Unlock()
		if prev != now && o.OnPress != nil {
			o.OnPress(now)
		}
	}
	return n
}

// VirtualButtonDown 读按钮是否被按住。
func VirtualButtonDown(key string) bool {
	if key == "" {
		key = "virtualbutton"
	}
	vpadMu.Lock()
	defer vpadMu.Unlock()
	return vpadBtns[key]
}
