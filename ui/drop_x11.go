//go:build (linux || freebsd) && !android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// X11 文件拖入（XDND 协议，freedesktop）
//
// 协议流程：
//
//	拖拽源 ──XdndEnter────▶ 我们的窗口（声明 XdndAware=5）
//	       ──XdndPosition─▶ 我们回 XdndStatus(accept)
//	       ──XdndDrop─────▶ 我们 ConvertSelection(XdndSelection → text/uri-list)
//	       ◀─SelectionNotify─ 读属性拿到 URL 列表 → 解析成路径
//	       ◀────────────── 回 XdndFinished
//
// 与 Windows 的 WM_DROPFILES 一样，只有在**松手**时才会拿到内容，
// 因此同样没有 dragenter 悬停高亮（要做高亮得持续处理 XdndPosition 并重绘）。
package ui

import (
	"fmt"
	"os"
	"sync"

	"guis/internal/x11"
)

// XDND 原子名。
const (
	xdndAwareAtom     = "XdndAware"
	xdndEnterAtom     = "XdndEnter"
	xdndPositionAtom  = "XdndPosition"
	xdndStatusAtom    = "XdndStatus"
	xdndLeaveAtom     = "XdndLeave"
	xdndDropAtom      = "XdndDrop"
	xdndFinishedAtom  = "XdndFinished"
	xdndSelectionAtom = "XdndSelection"
	xdndURIsAtom      = "text/uri-list"
	xdndPropAtom      = "GUIS_XDND_DATA"
	xdndVersion       = 5
)

// dndTarget 一个窗口上的拖拽会话状态。
type dndTarget struct {
	src  x11.Window // 拖拽源窗口（回执要用）
	root bool       // 是否已收到 XdndEnter
	ver  int
	x, y int // 最近一次 XdndPosition 的根坐标（物理像素）
	ti   uintptr
}

var (
	dndMu   sync.Mutex
	dndWins = map[x11.Window]*dndTarget{}
)

// initDropX11 给窗口声明支持 XDND（必须在窗口创建后、显示前调用）。
func initDropX11(dpy *x11.Display, w x11.Window) {
	if dpy == nil || w == 0 {
		return
	}
	// XdndAware：类型 XA_ATOM，值 = 我们支持的协议版本
	dpy.ChangeProp32(w, dpy.Atom(xdndAwareAtom), dpy.Atom("ATOM"), []uint32{xdndVersion})
	dndMu.Lock()
	dndWins[w] = &dndTarget{}
	dndMu.Unlock()
}

// dropX11Release 窗口销毁时清理状态。
func dropX11Release(w x11.Window) {
	dndMu.Lock()
	delete(dndWins, w)
	dndMu.Unlock()
}

// x11DispatchDND 处理 XDND 相关事件；返回 true 表示事件已消费。
//
// onDrop 拿到的是**物理像素**的落点（调用方除以 scale 换算成逻辑像素），
// 与 Windows 端 App.SetOnDropFiles 的 (x, y) 语义保持一致。
func x11DispatchDND(dpy *x11.Display, ev *x11.Event, onDrop func(paths []string, x, y float32)) bool {
	if dpy == nil {
		return false
	}
	win := x11.Window(ev.WindowOf())
	dndMu.Lock()
	st := dndWins[win]
	dndMu.Unlock()
	if st == nil {
		return false
	}

	switch {
	case ev.IsClientMessage():
		mt := ev.MessageType()
		switch mt {
		case dpy.Atom(xdndEnterAtom):
			dndMu.Lock()
			st.src = x11.Window(ev.Data(0))
			st.root = true
			st.ver = int(ev.Data(1)>>24) & 0xFF
			dndMu.Unlock()
			if os.Getenv("GUIS_X11_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[x11] XdndEnter from=%d v=%d\n", st.src, st.ver)
			}
			return true
		case dpy.Atom(xdndPositionAtom):
			rect := ev.Data(2)
			rootX := int(int32(rect >> 16))
			rootY := int(int32(rect & 0xFFFF))
			action := ev.Data(4) & 0xFFFF
			wx, wy := dpy.TranslateCoords(dpy.Root(), win, rootX, rootY)
			dndMu.Lock()
			st.x, st.y = wx, wy
			st.ti = ev.TimeOf()
			src := st.src
			dndMu.Unlock()
			// XdndStatus: flags bit0=accept，rect 全 0 表示「整个窗口都算接受区」
			dpy.SendClientMessage(src, dpy.Atom(xdndStatusAtom), 0, [5]int64{
				int64(win), 1, 0, 0, action,
			})
			return true
		case dpy.Atom(xdndDropAtom):
			dndMu.Lock()
			st.ti = ev.TimeOf()
			src := st.src
			dndMu.Unlock()
			// 请求内容：XdndSelection → text/uri-list，落到我们的属性里
			dpy.ConvertSelection(dpy.Atom(xdndSelectionAtom), dpy.Atom(xdndURIsAtom),
				dpy.Atom(xdndPropAtom), win)
			if os.Getenv("GUIS_X11_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[x11] XdndDrop from=%d\n", src)
			}
			return true
		case dpy.Atom(xdndLeaveAtom):
			dndMu.Lock()
			st.root = false
			st.src = 0
			dndMu.Unlock()
			return true
		}
		return false
	case ev.IsSelectionNotify():
		if ev.SelProperty() != dpy.Atom(xdndPropAtom) {
			return false
		}
		data := dpy.GetStringProperty(win, dpy.Atom(xdndPropAtom))
		paths := ParseURIList(data)
		dndMu.Lock()
		src, x, y := st.src, st.x, st.y
		st.root = false
		st.src = 0
		dndMu.Unlock()
		if src != 0 {
			// XdndFinished: flags bit0=成功，action=copy(1)
			dpy.SendClientMessage(src, dpy.Atom(xdndFinishedAtom), 0, [5]int64{
				int64(win), 1, 1, 0, 0,
			})
		}
		if len(paths) > 0 && onDrop != nil {
			onDrop(paths, float32(x), float32(y))
		}
		return true
	}
	return false
}
