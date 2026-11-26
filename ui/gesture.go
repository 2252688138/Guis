// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// 手势识别器：长按 / 滑动 / 拖动 / 双指捏合。
//
// 定位：触摸端（Android / iOS / Web 触屏）与鼠标拖动共用一套识别，做成
// **观察者**——只观察指针流、不消费事件，所以挂上手势不会改变既有的点击、
// 滚动、拖拽行为，可以随时加上去。
//
// 用法：
//
//	Card(...).Gestures(ui.GestureOptions{
//	    OnLongPress: func(x, y float32) { ... },              // 默认 500ms
//	    OnSwipe:     func(dir ui.SwipeDir, dx, dy float32) { ... }, // 默认阈值 30px
//	    OnPan:       func(dx, dy float32) { ... },
//	    OnPinch:     func(scale float32) { ... },
//	})
//
// 命中规则：按下点命中节点（或它的任一祖先）声明了手势即归属该节点；同一时刻
// 只有一个手势会话，多指落到同一会话里做捏合。

// SwipeDir 滑动方向（按主轴判定）。
type SwipeDir int

const (
	SwipeNone SwipeDir = iota
	SwipeLeft
	SwipeRight
	SwipeUp
	SwipeDown
)

// String 便于日志与测试断言。
func (d SwipeDir) String() string {
	switch d {
	case SwipeLeft:
		return "left"
	case SwipeRight:
		return "right"
	case SwipeUp:
		return "up"
	case SwipeDown:
		return "down"
	}
	return "none"
}

// GestureOptions 手势回调。只填用得到的字段即可，零值即关闭该手势。
type GestureOptions struct {
	// OnLongPress 按住不动超过 LongPressMs 触发（坐标相对窗口）。
	OnLongPress func(x, y float32)
	// LongPressMs 长按阈值，<=0 用默认 500ms。
	LongPressMs int
	// OnSwipe 抬起时按主轴方向触发（一次滑动只触发一次，长按已触发则不触发）。
	OnSwipe func(dir SwipeDir, dx, dy float32)
	// SwipeMin 判定滑动的最小主轴位移，<=0 用默认 30 逻辑像素。
	SwipeMin float32
	// OnPan 拖动过程回调（位移相对按下点）。
	OnPan func(dx, dy float32)
	// OnPinch 双指捏合回调，scale 为当前指距 / 起始指距（>1 放大，<1 缩小）。
	OnPinch func(scale float32)
}

// empty 未声明任何手势（用来跳过未挂手势的节点）。
func (o GestureOptions) empty() bool {
	return o.OnLongPress == nil && o.OnSwipe == nil && o.OnPan == nil && o.OnPinch == nil
}

const (
	gestureDefaultLongPressMs = 500
	gestureDefaultSwipeMin    = 30
	gestureMoveSlop           = 6
	gestureMousePointer       = -1
)

type gesturePoint struct{ x, y float32 }

type gestureRun struct {
	opts      GestureOptions
	startX    float32
	startY    float32
	curX      float32
	curY      float32
	startAt   time.Time
	longFired bool
	moved     bool
	pointers  map[int]gesturePoint
	pinchBase float32
}

var (
	gestureMu    sync.Mutex
	gestureCur   *gestureRun
	gestureUsed  atomic.Bool
	gestureClock = time.Now
)

// Gestures 给节点挂手势识别（观察者模式，不改变既有事件行为）。
func (n *Node) Gestures(o GestureOptions) *Node {
	n.GestureOpts = o
	if !o.empty() {
		gestureUsed.Store(true)
	}
	return n
}

// ResetGestures 清空当前手势会话与声明标志（窗口重建 / 测试收尾用）。
func ResetGestures() {
	gestureMu.Lock()
	gestureCur = nil
	gestureMu.Unlock()
	gestureUsed.Store(false)
}

// gestureLookup 从命中节点向上找最近一个声明了手势的节点。
func gestureLookup(a *App, x, y float32) (GestureOptions, bool) {
	if a == nil || a.root == nil {
		return GestureOptions{}, false
	}
	n := hitTest(a.root, x, y, 0, a)
	for n != nil {
		if !n.GestureOpts.empty() {
			return n.GestureOpts, true
		}
		n = n.Parent
	}
	return GestureOptions{}, false
}

// gestureSpan 多指之间的最大指距（用于捏合比例）。
func gestureSpan(ps map[int]gesturePoint) float32 {
	var best float32
	for i, p := range ps {
		for j, q := range ps {
			if i >= j {
				continue
			}
			d := hypotF(p.x-q.x, p.y-q.y)
			if d > best {
				best = d
			}
		}
	}
	return best
}

func hypotF(dx, dy float32) float32 {
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

// gestureSwipeDir 按主轴判定方向。
func gestureSwipeDir(dx, dy float32) SwipeDir {
	if absF(dx) >= absF(dy) {
		if dx < 0 {
			return SwipeLeft
		}
		return SwipeRight
	}
	if dy < 0 {
		return SwipeUp
	}
	return SwipeDown
}

// gesturePointer 喂入一个指针事件（phase：0 按下 / 1 移动 / 2 抬起）。
// 只能从主循环调用；回调在锁外执行，回调里调用 SetFocus 等不会死锁。
func gesturePointer(a *App, x, y float32, phase, id int) {
	if !gestureUsed.Load() {
		return
	}

	var (
		longFn   func(x, y float32)
		swipeFn  func(SwipeDir, float32, float32)
		panFn    func(dx, dy float32)
		pinchFn  func(scale float32)
		fx, fy   float32
		dx, dy   float32
		scale    float32
		swipeDir SwipeDir
	)

	gestureMu.Lock()
	switch phase {
	case 0:
		if gestureCur == nil {
			if opts, ok := gestureLookup(a, x, y); ok {
				gestureCur = &gestureRun{
					opts: opts, startX: x, startY: y, curX: x, curY: y,
					startAt: gestureClock(), pointers: map[int]gesturePoint{},
				}
			}
		}
		if gestureCur != nil {
			gestureCur.pointers[id] = gesturePoint{x: x, y: y}
			if len(gestureCur.pointers) >= 2 && gestureCur.pinchBase == 0 {
				gestureCur.pinchBase = gestureSpan(gestureCur.pointers)
			}
		}
	case 1:
		if r := gestureCur; r != nil {
			r.pointers[id] = gesturePoint{x: x, y: y}
			r.curX, r.curY = x, y
			if !r.moved && hypotF(x-r.startX, y-r.startY) > gestureMoveSlop {
				r.moved = true
			}
			if r.moved && r.opts.OnPan != nil {
				panFn = r.opts.OnPan
				dx, dy = x-r.startX, y-r.startY
			}
			if r.pinchBase > 0 && r.opts.OnPinch != nil && len(r.pointers) >= 2 {
				if cur := gestureSpan(r.pointers); cur > 0 {
					pinchFn = r.opts.OnPinch
					scale = cur / r.pinchBase
				}
			}
		}
	case 2:
		if r := gestureCur; r != nil {
			delete(r.pointers, id)
			if len(r.pointers) == 0 {
				gestureCur = nil
				if !r.longFired && r.opts.OnSwipe != nil {
					sdx, sdy := x-r.startX, y-r.startY
					min := r.opts.SwipeMin
					if min <= 0 {
						min = gestureDefaultSwipeMin
					}
					if absF(sdx) >= min || absF(sdy) >= min {
						swipeFn = r.opts.OnSwipe
						swipeDir, fx, fy = gestureSwipeDir(sdx, sdy), sdx, sdy
					}
				}
			}
		}
	}
	gestureMu.Unlock()

	if panFn != nil {
		panFn(dx, dy)
	}
	if pinchFn != nil {
		pinchFn(scale)
	}
	if swipeFn != nil {
		swipeFn(swipeDir, fx, fy)
	}
	if longFn != nil {
		longFn(x, y)
	}
}

// gestureTick 每帧调用：按住不动超时则触发长按（长按只触发一次）。
func gestureTick() {
	if !gestureUsed.Load() {
		return
	}
	var (
		fn     func(x, y float32)
		fx, fy float32
	)
	gestureMu.Lock()
	if r := gestureCur; r != nil && !r.longFired && !r.moved && r.opts.OnLongPress != nil {
		ms := r.opts.LongPressMs
		if ms <= 0 {
			ms = gestureDefaultLongPressMs
		}
		if gestureClock().Sub(r.startAt) >= time.Duration(ms)*time.Millisecond {
			r.longFired = true
			fn, fx, fy = r.opts.OnLongPress, r.startX, r.startY
		}
	}
	gestureMu.Unlock()
	if fn != nil {
		fn(fx, fy)
	}
}


