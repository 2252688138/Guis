// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// AnimEvent 动画事件（到达该帧时触发一次）。
type AnimEvent struct {
	Frame int
	Name  string
}

// AnimClip 精灵帧动画片段（帧来自图集子矩形）。
type AnimClip struct {
	// Frames 逐帧源矩形（通常来自 Atlas.Cell）。
	Frames []Rect2
	// FPS 播放速率（≤ 0 时按 12）。
	FPS float32
	// Loop 是否循环播放。
	Loop bool
	// Events 事件帧（下标基于 Frames）。
	Events []AnimEvent
}

// NewAnimClip 用图集连续帧构造片段。
func NewAnimClip(atlas *Atlas, from, to int, fps float32, loop bool) *AnimClip {
	c := &AnimClip{FPS: fps, Loop: loop}
	if atlas == nil {
		return c
	}
	for i := from; i <= to; i++ {
		r, ok := atlas.Cell(i)
		if !ok {
			break
		}
		c.Frames = append(c.Frames, r)
	}
	return c
}

// Count 帧数。
func (c *AnimClip) Count() int {
	if c == nil {
		return 0
	}
	return len(c.Frames)
}

// Duration 一轮时长（秒）。
func (c *AnimClip) Duration() float32 {
	if c == nil || len(c.Frames) == 0 {
		return 0
	}
	fps := c.FPS
	if fps <= 0 {
		fps = 12
	}
	return float32(len(c.Frames)) / fps
}

// SpriteAnim 帧动画播放器（挂在精灵上逐帧推进）。
type SpriteAnim struct {
	Clip    *AnimClip
	T       float32 // 当前累计时间（秒）
	Playing bool
	// Done 非循环片段播放完毕。
	Done bool

	last int
}

// Play 切换到新片段并从头播放。
func (a *SpriteAnim) Play(clip *AnimClip) {
	if a == nil {
		return
	}
	a.Clip = clip
	a.T = 0
	a.last = 0
	a.Done = false
	a.Playing = clip != nil && clip.Count() > 0
}

// Reset 回到首帧并继续播放。
func (a *SpriteAnim) Reset() {
	a.T = 0
	a.last = 0
	a.Done = false
}

// Stop 暂停播放。
func (a *SpriteAnim) Stop() { a.Playing = false }

// Update 推进时间，返回本次触发的事件帧下标（已去重）。
func (a *SpriteAnim) Update(dt float32) []int {
	if a == nil || a.Clip == nil || a.Clip.Count() == 0 || !a.Playing || a.Done {
		return nil
	}
	if dt < 0 {
		dt = 0
	}
	fps := a.Clip.FPS
	if fps <= 0 {
		fps = 12
	}
	n := a.Clip.Count()
	a.T += dt
	idx := int(a.T * fps)
	var fired []int
	if a.Clip.Loop {
		idx %= n
		if n > 0 && dt > 0 {
			fired = a.eventsBetween(a.last, idx+n)
		}
	} else {
		if idx >= n {
			idx = n - 1
			a.Done = true
			fired = a.eventsBetween(a.last, n-1)
			a.last = n - 1
			a.T = float32(n) / fps
			return fired
		}
		fired = a.eventsBetween(a.last, idx)
	}
	a.last = idx
	return fired
}

// eventsBetween 返回 (from, to] 区间内的事件帧（to 可能超过 n，按回绕取模）。
func (a *SpriteAnim) eventsBetween(from, to int) []int {
	n := a.Clip.Count()
	if n == 0 || len(a.Clip.Events) == 0 || to <= from {
		return nil
	}
	var out []int
	for _, e := range a.Clip.Events {
		f := e.Frame
		if f < 0 {
			continue
		}
		// 把事件帧展开到 (from, to] 区间
		k := from / n
		cand := k*n + f
		for cand <= from {
			cand += n
		}
		if cand <= to {
			out = append(out, f)
		}
	}
	return out
}

// Index 当前帧下标。
func (a *SpriteAnim) Index() int {
	if a == nil || a.Clip == nil || a.Clip.Count() == 0 {
		return 0
	}
	fps := a.Clip.FPS
	if fps <= 0 {
		fps = 12
	}
	n := a.Clip.Count()
	i := int(a.T * fps)
	if a.Clip.Loop {
		return i % n
	}
	if i >= n {
		return n - 1
	}
	return i
}

// Frame 当前帧源矩形。
func (a *SpriteAnim) Frame() Rect2 {
	if a == nil || a.Clip == nil || a.Clip.Count() == 0 {
		return Rect2{}
	}
	return a.Clip.Frames[a.Index()]
}
