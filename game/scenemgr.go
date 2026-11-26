// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"fmt"
	"image"
)

// Level 一个关卡 / 场景定义（由 SceneManager 管理）。
type Level struct {
	// Name 关卡名。
	Name string
	// Build 切入时调用，返回该关卡的 Game（nil = 只保留静态场景）。
	Build func(w *World) Game
	// Enter 切入前的钩子（搭场景 / 重置状态）。
	Enter func(w *World)
	// Exit 切出时的钩子（停音频 / 释放资源）。
	Exit func(w *World)
}

// SceneManager 关卡 / 场景切换管理器（含淡入淡出过渡）。
//
// 把它放在游戏逻辑里，一行完成"淡出 → 清场景 → 重建 → 淡入"：
//
//	mgr := game.NewSceneManager()
//	mgr.Register("menu", buildMenu, nil, nil).Register("level1", buildLevel1, nil, nil)
//	mgr.FadeTo("level1")
//	// Update 中：mgr.Update(dt, w)
//	// Draw 中：mgr.Draw(w.Renderer.Image())
//
// 淡入淡出遮罩在精灵之后绘制，会盖住整个画面；不需要过渡时可把 FadeTime 设为 0。
type SceneManager struct {
	levels  map[string]*Level
	order   []string
	current string
	pending string
	// FadeTime 过渡时长（秒，默认 0.25；0 = 立即切换）。
	FadeTime float32
	// FadeColor 过渡遮罩色（默认黑，A 由过渡自动计算）。
	FadeColor RGBA

	fade    float32 // 0 = 透明，1 = 全遮
	fadeDir int     // +1 淡出，-1 淡入，0 静止
}

// NewSceneManager 创建关卡管理器。
func NewSceneManager() *SceneManager {
	return &SceneManager{levels: map[string]*Level{}, FadeTime: 0.25, FadeColor: RGB(0, 0, 0)}
}

// Register 注册关卡（重复注册同名时覆盖）。
func (m *SceneManager) Register(name string, build func(w *World) Game, enter, exit func(w *World)) *SceneManager {
	if m == nil || name == "" {
		return m
	}
	if _, ok := m.levels[name]; !ok {
		m.order = append(m.order, name)
	}
	m.levels[name] = &Level{Name: name, Build: build, Enter: enter, Exit: exit}
	return m
}

// Levels 返回已注册的关卡名（按注册顺序）。
func (m *SceneManager) Levels() []string {
	if m == nil {
		return nil
	}
	out := make([]string, len(m.order))
	copy(out, m.order)
	return out
}

// Current 当前关卡名。
func (m *SceneManager) Current() string {
	if m == nil {
		return ""
	}
	return m.current
}

// Busy 是否正在过渡中。
func (m *SceneManager) Busy() bool {
	return m != nil && m.fadeDir != 0
}

// Switch 立即切换到指定关卡（无过渡）。
func (m *SceneManager) Switch(name string, w *World) error {
	if m == nil || w == nil {
		return fmt.Errorf("game: SceneManager 或 World 为空")
	}
	lv, ok := m.levels[name]
	if !ok {
		return fmt.Errorf("game: 未注册的关卡 %q", name)
	}
	if cur, ok := m.levels[m.current]; ok && cur.Exit != nil {
		cur.Exit(w)
	}
	// 重置场景（保留相机，重建根节点）
	w.Scene.Root = NewNode("root")
	w.Scene.Lights = nil
	w.Scene.Fog = Fog{}
	w.Scene.Sky = Sky{}
	w.Paused = false
	var g Game
	if lv.Build != nil {
		g = lv.Build(w)
	}
	w.SetGame(g)
	if lv.Enter != nil {
		lv.Enter(w)
	}
	m.current = name
	m.pending = ""
	m.fadeDir = 0
	m.fade = 0
	return nil
}

// FadeTo 带过场动画切换关卡（FadeTime <= 0 时退化为 Switch）。
func (m *SceneManager) FadeTo(name string, w *World) error {
	if m == nil || w == nil {
		return fmt.Errorf("game: SceneManager 或 World 为空")
	}
	if _, ok := m.levels[name]; !ok {
		return fmt.Errorf("game: 未注册的关卡 %q", name)
	}
	if m.FadeTime <= 0 {
		return m.Switch(name, w)
	}
	m.pending = name
	m.fadeDir = 1
	return nil
}

// Update 推进过渡（每帧在游戏逻辑里调用）。
func (m *SceneManager) Update(dt float32, w *World) {
	if m == nil || w == nil || m.fadeDir == 0 || dt <= 0 {
		return
	}
	step := dt / m.FadeTime
	if m.fadeDir > 0 {
		m.fade += step
		if m.fade >= 1 {
			m.fade = 1
			if err := m.Switch(m.pending, w); err == nil {
				m.fadeDir = -1
				m.fade = 1
			} else {
				m.fadeDir, m.fade = 0, 0
			}
		}
		return
	}
	m.fade -= step
	if m.fade <= 0 {
		m.fade = 0
		m.fadeDir = 0
	}
}

// Fade 当前遮罩不透明度（0~1，调试 / 自定义绘制用）。
func (m *SceneManager) Fade() float32 {
	if m == nil {
		return 0
	}
	return m.fade
}

// Draw 把过渡遮罩画到帧上（在精灵绘制之后调用）。
func (m *SceneManager) Draw(dst *image.RGBA) {
	if m == nil || dst == nil || m.fade <= 0 {
		return
	}
	a := uint8(Clamp(m.fade, 0, 1)*255 + 0.5)
	c := m.FadeColor.WithAlpha(a)
	FillRect(dst, 0, 0, float32(dst.Rect.Dx()), float32(dst.Rect.Dy()), c)
}

// FillRect 用纯色填充矩形（直通 alpha 混合，截断在图像范围内）。
func FillRect(dst *image.RGBA, x, y, w, h float32, c RGBA) {
	if dst == nil || w <= 0 || h <= 0 || c.A == 0 {
		return
	}
	bw, bh := dst.Rect.Dx(), dst.Rect.Dy()
	x0 := int(x)
	y0 := int(y)
	x1 := int(x + w + 0.99)
	y1 := int(y + h + 0.99)
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > bw {
		x1 = bw
	}
	if y1 > bh {
		y1 = bh
	}
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			j := py*dst.Stride + px*4
			if j+3 >= len(dst.Pix) {
				return
			}
			if c.A == 255 {
				dst.Pix[j], dst.Pix[j+1], dst.Pix[j+2], dst.Pix[j+3] = c.R, c.G, c.B, 255
				continue
			}
			blended := blendSrcOver(RGBA{dst.Pix[j], dst.Pix[j+1], dst.Pix[j+2], dst.Pix[j+3]}, c)
			dst.Pix[j], dst.Pix[j+1], dst.Pix[j+2], dst.Pix[j+3] = blended.R, blended.G, blended.B, blended.A
		}
	}
}
