// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "image"

// Engine 把场景、相机、渲染器、输入、固定步长循环串在一起的引擎门面。
//
// 典型用法（ui 侧）：
//
// e := game.NewEngine(480, 270)
// n := ui.Viewport3D(&ui.Viewport3DOpts{Src: e})
//
// Engine 实现了 ui.ViewportSource（RenderViewport），但 game 包不依赖 ui。
type Engine struct {
	// Scene 场景（必填）。
	Scene *Scene
	// Renderer 渲染器（必填）。
	Renderer *Renderer
	// Camera 相机（同步到 Scene.Camera）。
	Camera *Camera
	// Input 输入状态表（可为空）。
	Input *Input
	// Loop 固定步长循环（可为空）。
	Loop *Loop
	// Paused 暂停逻辑更新（仍会渲染）。
	Paused bool
	// AutoClearInput 每次 Tick 末尾自动 EndFrame（默认 true）。
	AutoClearInput bool
	// KeepAspect 按控件宽高比调整内部渲染比例，避免画面拉伸（默认 true）。
	KeepAspect bool
	// BaseW, BaseH 内部渲染基准分辨率。
	BaseW, BaseH int

	// gpu / gpuScene 硬件渲染后端（AttachGPU 后非空；nil = 走软件 Renderer）。
	gpu      *GPU
	gpuScene *GPUScene

	lastW, lastH     int
	clientW, clientH int
}

// NewEngine 创建引擎（默认 60 帧固定步长、保持宽高比）。
func NewEngine(w, h int) *Engine {
	if w < 16 {
		w = 16
	}
	if h < 16 {
		h = 16
	}
	sc := NewScene()
	return &Engine{
		Scene:          sc,
		Renderer:       NewRenderer(w, h),
		Camera:         sc.Camera,
		Input:          NewInput(),
		AutoClearInput: true,
		KeepAspect:     true,
		BaseW:          w,
		BaseH:          h,
	}
}

// SetGame 挂载游戏逻辑（创建固定步长循环）。
func (e *Engine) SetGame(g Game) *Engine {
	e.Loop = NewLoop(g)
	return e
}

// Resize 调整内部渲染基准分辨率。
func (e *Engine) Resize(w, h int) {
	if w < 16 {
		w = 16
	}
	if h < 16 {
		h = 16
	}
	e.BaseW, e.BaseH = w, h
	e.Renderer.Resize(w, h)
}

// Tick 推进一帧逻辑（dt 单位秒）。
func (e *Engine) Tick(dt float32) {
	if e == nil {
		return
	}
	if dt <= 0 {
		dt = 1.0 / 60.0
	}
	if dt > 0.25 {
		dt = 0.25
	}
	if e.Camera != nil {
		if e.Scene == nil {
			e.Scene = NewScene()
		}
		e.Scene.Camera = e.Camera
	}
	if !e.Paused && e.Loop != nil {
		e.Loop.Step(dt, e)
		if e.Loop.Game != nil {
			e.Loop.Game.Draw(e.Scene)
		}
	}
	if e.AutoClearInput && e.Input != nil {
		e.Input.EndFrame()
	}
}

// Render 渲染一帧并返回图像。
func (e *Engine) Render() *image.RGBA {
	if e == nil || e.Renderer == nil {
		return nil
	}
	return e.Renderer.Frame(e.Scene)
}

// RenderViewport 实现 ui.ViewportSource：按控件尺寸渲染并返回 RGBA 像素（长度 w*h*4）。
//
// w/h 是控件像素尺寸（用于保持宽高比），dt 是距上一帧的秒数（≤0 时按 60fps 计）。
func (e *Engine) RenderViewport(w, h int, dt float32) ([]byte, int, int) {
	if e == nil || e.Renderer == nil {
		return nil, 0, 0
	}
	e.Tick(dt)
	if tw, th := e.targetSize(w, h); tw > 0 {
		e.Renderer.Resize(tw, th)
	}
	img := e.Renderer.Frame(e.Scene)
	if img == nil {
		return nil, 0, 0
	}
	e.lastW, e.lastH = img.Rect.Dx(), img.Rect.Dy()
	e.clientW, e.clientH = w, h
	return img.Pix, e.lastW, e.lastH
}

// PickClient 用控件像素坐标拾取（自动按 KeepAspect 的缩放比例映射到内部渲染坐标）。
func (e *Engine) PickClient(px, py float32, nodes ...*Node) (*Node, float32, bool) {
	if e == nil || e.lastW <= 0 || e.lastH <= 0 {
		return nil, 0, false
	}
	if e.clientW > 0 && e.clientH > 0 {
		px *= float32(e.lastW) / float32(e.clientW)
		py *= float32(e.lastH) / float32(e.clientH)
	}
	return e.Pick(px, py, e.lastW, e.lastH, nodes...)
}

// ClientSize 最近一次 RenderViewport 收到的控件尺寸。
func (e *Engine) ClientSize() (int, int) {
	if e == nil {
		return 0, 0
	}
	return e.clientW, e.clientH
}

// FrameStats 上一帧统计。
func (e *Engine) FrameStats() FrameStats {
	if e == nil || e.Renderer == nil {
		return FrameStats{}
	}
	return e.Renderer.Stats()
}

// Size 最近一次渲染的内部尺寸。
func (e *Engine) Size() (int, int) {
	if e == nil {
		return 0, 0
	}
	return e.lastW, e.lastH
}

// HandleKey 转发按键事件（配合 ui.Viewport3DOpts.OnKey）。
func (e *Engine) HandleKey(name string, down bool) {
	if e == nil || e.Input == nil {
		return
	}
	e.Input.HandleKey(name, down)
}

// HandleMouse 转发鼠标事件（btn < 0 表示仅移动）。
func (e *Engine) HandleMouse(x, y float32, btn int, down bool) {
	if e == nil || e.Input == nil {
		return
	}
	e.Input.HandleMouse(x, y, btn, down)
}

// HandleMouseMove 注入鼠标相对位移（指针锁定 / 原始输入用，不参与绝对坐标差分）。
func (e *Engine) HandleMouseMove(dx, dy float32) {
	if e == nil || e.Input == nil {
		return
	}
	e.Input.AddMouseDelta(dx, dy)
}

// SetPointerLock 开关指针锁定：开启后只认 HandleMouseMove 的位移，
// 绝对坐标不再参与差分（配合视口全屏使用，转视角不会因为光标撞到屏幕边缘而卡住）。
func (e *Engine) SetPointerLock(on bool) {
	if e == nil || e.Input == nil {
		return
	}
	e.Input.LockMouse = on
}

// HandleTouch 转发触摸事件（phase：0 按下 / 1 移动 / 2 抬起）。
func (e *Engine) HandleTouch(x, y float32, phase int, id int) {
	if e == nil || e.Input == nil {
		return
	}
	p := TouchMove
	switch phase {
	case 0:
		p = TouchBegin
	case 2:
		p = TouchEnd
	}
	e.Input.HandleTouch(id, x, y, p)
}

// HandleWheel 转发滚轮事件。
func (e *Engine) HandleWheel(dy float32) {
	if e == nil || e.Input == nil {
		return
	}
	e.Input.HandleWheel(dy)
}

// Pick 从控件像素坐标发射射线并返回最近的命中物体（mesh 为空则用全部节点）。
func (e *Engine) Pick(px, py float32, w, h int, nodes ...*Node) (hit *Node, dist float32, ok bool) {
	if e == nil || e.Scene == nil || e.Camera == nil {
		return nil, 0, false
	}
	// 拾取依赖世界矩阵，不能依赖本帧是否已渲染。
	e.Scene.BuildWorld()
	origin, dir := e.Camera.Ray(px, py, w, h)
	list := nodes
	if len(list) == 0 {
		e.Scene.Each(func(n *Node) {
			if n.Mesh != nil && n.Visible {
				list = append(list, n)
			}
		})
	}
	best := float32(0)
	for _, n := range list {
		if n == nil || n.Mesh == nil {
			continue
		}
		var t float32
		var hitOK bool
		// 有 BVH 就走加速结构（与 RayMesh 结果一致，只是跳过不相交的子树）
		if bvh := n.Mesh.BVH(); bvh != nil {
			t, _, hitOK = bvh.Ray(origin, dir, n.WorldMatrix())
		} else {
			t, _, hitOK = RayMesh(origin, dir, n.Mesh, n.WorldMatrix())
		}
		if !hitOK {
			continue
		}
		if hit == nil || t < best {
			hit, best = n, t
		}
	}
	return hit, best, hit != nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
