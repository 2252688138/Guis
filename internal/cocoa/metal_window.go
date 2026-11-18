//go:build darwin && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package cocoa

import (
	"errors"

	"guis/internal/objc"
)

// 本文件提供 Metal 渲染所需的「图层背衬窗口」：与 NewWindow 共用同一套
// NSWindow / GoguiView（事件、拖入、尺寸轮询完全一致），区别只在于
//
//  1. 视图走 layer-backed（setWantsLayer:），由调用方把 CAMetalLayer 装上去；
//  2. 不创建 NSOpenGLContext / NSOpenGLPixelFormat（Metal 与 GL 不能混用同一视图）。
//
// Metal 设备与图层本身由 internal/gpu 的 Metal 后端创建（见 gpu.MetalAttach）。

// NewWindowLayerBacked 创建图层背衬窗口（无 GL 上下文）。
func NewWindowLayerBacked(title string, w, h int) (*Window, error) {
	if err := InitApp(); err != nil {
		return nil, err
	}
	if w <= 0 {
		w = 1024
	}
	if h <= 0 {
		h = 700
	}
	cls := ensureViewClass()
	if cls == 0 {
		return nil, errors.New("cocoa: 无法创建视图子类（NSView 不可用）")
	}

	winObj := objc.Send(objc.Class("NSWindow"), "alloc")
	style := uintptr(styleTitled | styleClosable | styleMiniaturizable | styleResizable)
	win := objc.SendRect3(winObj, "initWithContentRect:styleMask:backing:defer:",
		objc.NewRect(120, 120, float64(w), float64(h)), style, backingBuffered, 0)
	if win == 0 {
		return nil, errors.New("cocoa: NSWindow 创建失败")
	}
	objc.Send1(win, "setTitle:", objc.NSString(title))
	objc.SendB(win, "setAcceptsMouseMovedEvents:", true)
	objc.Send1(win, "setReleasedWhenClosed:", 0)

	view := objc.Send(cls, "alloc")
	view = objc.SendRect(view, "initWithFrame:", objc.NewRect(0, 0, float64(w), float64(h)))
	// 图层背衬：CAMetalLayer 随后由 gpu.MetalAttach 安装到 view.layer。
	objc.SendB(view, "setWantsLayer:", true)
	objc.Send1(win, "setContentView:", view)
	applyDropTypes(view)

	b := objc.RectOf(view, "bounds")
	bs := objc.RectConvert(view, "convertRectToBacking:", b)
	scale := scaleOf(win)
	if scale <= 0 {
		scale = 1
	}

	wd := &Window{
		win:    win,
		view:   view,
		title:  title,
		width:  float32(b.Size.W),
		height: float32(b.Size.H),
		pw:     int32(bs.Size.W),
		ph:     int32(bs.Size.H),
		scale:  scale,
		live:   true,
	}
	mu.Lock()
	windows[view] = wd
	mu.Unlock()

	objc.Send1(win, "makeKeyAndOrderFront:", 0)
	objc.Send1(win, "makeFirstResponder:", view)
	return wd, nil
}
