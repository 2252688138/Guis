// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"fmt"
	"sort"
	"strings"
)

// DebugStats 一帧的可展示统计（调试面板 / 日志用）。
type DebugStats struct {
	FPS          float64
	Ms           float64
	Triangles    int
	Culled       int
	Pixels       int
	Objects      int
	Sprites      int
	W, H         int
	Supersample  int
	Nodes        int
	PadConnected bool
	PadAxes      [6]float32
	Recording    bool
	ReplayFrames int
}

// countNodes 递归数节点。
func countNodes(n *Node) int {
	if n == nil {
		return 0
	}
	c := 1
	for _, ch := range n.Children {
		c += countNodes(ch)
	}
	return c
}

// DebugStats 取当前统计（不修改引擎状态）。
func (e *Engine) DebugStats() DebugStats {
	if e == nil {
		return DebugStats{}
	}
	var d DebugStats
	if e.Renderer != nil {
		s := e.Renderer.Stats()
		d.Ms = s.Ms
		d.Triangles = s.Triangles
		d.Culled = s.Culled
		d.Pixels = s.Pixels
		d.Objects = s.Objects
		d.Sprites = s.Sprites
		d.W, d.H = e.Renderer.W, e.Renderer.H
		d.Supersample = e.Renderer.Supersample
	}
	if e.Scene != nil {
		d.Nodes = countNodes(e.Scene.Root)
	}
	return d
}

// DebugHUD 调试面板：平滑 FPS + 文本格式化（零分配复用缓冲）。
type DebugHUD struct {
	fps float64
	// Pad 非 nil 时面板会显示手柄状态。
	Pad *Gamepad
	// Rec 非 nil 时面板会显示录制/回放状态。
	Rec *Recorder
}

// NewDebugHUD 创建调试面板。
func NewDebugHUD() *DebugHUD { return &DebugHUD{} }

// Update 推进 FPS 统计并返回带 FPS/手柄/录制信息的快照。
func (h *DebugHUD) Update(e *Engine, dt float32) DebugStats {
	d := e.DebugStats()
	if h != nil {
		if dt > 0 {
			inst := 1 / float64(dt)
			if h.fps == 0 {
				h.fps = inst
			} else {
				h.fps += (inst - h.fps) * 0.1
			}
		}
		d.FPS = h.fps
		if h.Pad != nil {
			d.PadConnected = h.Pad.Connected
			d.PadAxes = h.Pad.Axes
		}
		if h.Rec != nil {
			d.Recording = h.Rec.Recording()
			d.ReplayFrames = h.Rec.FrameCount()
		}
	}
	return d
}

// Lines 格式化为多行文本（直接喂给 UI 文本节点）。
func (d DebugStats) Lines() []string {
	out := []string{
		fmt.Sprintf("FPS %.0f  %.2fms", d.FPS, d.Ms),
		fmt.Sprintf("三角 %d  剔除 %d  像素 %d", d.Triangles, d.Culled, d.Pixels),
		fmt.Sprintf("对象 %d  节点 %d  精灵 %d", d.Objects, d.Nodes, d.Sprites),
		fmt.Sprintf("分辨率 %d×%d  超采 %d×", d.W, d.H, d.Supersample),
	}
	if d.PadConnected {
		out = append(out, fmt.Sprintf("手柄 L %.2f,%.2f  R %.2f,%.2f", d.PadAxes[0], d.PadAxes[1], d.PadAxes[2], d.PadAxes[3]))
	} else {
		out = append(out, "手柄 未连接")
	}
	if d.Recording {
		out = append(out, fmt.Sprintf("录制中 %d 帧", d.ReplayFrames))
	} else if d.ReplayFrames > 0 {
		out = append(out, fmt.Sprintf("回放数据 %d 帧", d.ReplayFrames))
	}
	return out
}

// String 单行摘要（日志用）。
func (d DebugStats) String() string { return strings.Join(d.Lines(), " | ") }

// DebugAABB 线框盒绘制时用的边索引（12 条边）。
var debugBoxEdges = [12][2]int{
	{0, 1}, {1, 3}, {3, 2}, {2, 0},
	{4, 5}, {5, 7}, {7, 6}, {6, 4},
	{0, 4}, {1, 5}, {2, 6}, {3, 7},
}

// AABBPoints 返回盒子的 8 个角点（顺序与 debugBoxEdges 匹配）。
func AABBPoints(b AABB) [8]Vec3 {
	return [8]Vec3{
		{X: b.Min.X, Y: b.Min.Y, Z: b.Min.Z},
		{X: b.Max.X, Y: b.Min.Y, Z: b.Min.Z},
		{X: b.Min.X, Y: b.Max.Y, Z: b.Min.Z},
		{X: b.Max.X, Y: b.Max.Y, Z: b.Min.Z},
		{X: b.Min.X, Y: b.Min.Y, Z: b.Max.Z},
		{X: b.Max.X, Y: b.Min.Y, Z: b.Max.Z},
		{X: b.Min.X, Y: b.Max.Y, Z: b.Max.Z},
		{X: b.Max.X, Y: b.Max.Y, Z: b.Max.Z},
	}
}

// SortedKeys 当前按住的键（排序，便于展示与回放比对）。
func (in *Input) SortedKeys() []string {
	if in == nil {
		return nil
	}
	out := sortedKeys(in.Keys)
	sort.Strings(out)
	return out
}
