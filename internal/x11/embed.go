//go:build (linux || freebsd) && !android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package x11

// 窗口嵌入相关的少量 Xlib 绑定：重挂载父窗口、枚举子窗口、读 32 位属性。
//
// 这些函数只在「把系统浏览器窗口挂进 GUIS 窗口」时才用到，所以单独放一个文件、
// 单独一次 Dlopen（用的还是同一个 libX11，重复加载不会出错）。

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	embedOnce sync.Once
	embedErr  error

	xReparentWindow func(uintptr, uintptr, uintptr, int, int) int
	xQueryTree      func(uintptr, uintptr, *uintptr, *uintptr, *uintptr, *uint) int
)

// loadEmbed 解析嵌入相关符号。
func loadEmbed() error {
	embedOnce.Do(func() {
		var h uintptr
		for _, name := range []string{"libX11.so.6", "libX11.so", "/usr/lib/libX11.so.6", "/usr/local/lib/libX11.so"} {
			if lib, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
				h = lib
				break
			}
		}
		if h == 0 {
			embedErr = errors.New("x11: 找不到 libX11")
			return
		}
		bind := func(dst any, name string) {
			if addr, err := purego.Dlsym(h, name); err == nil && addr != 0 {
				purego.RegisterFunc(dst, addr)
			}
		}
		bind(&xReparentWindow, "XReparentWindow")
		bind(&xQueryTree, "XQueryTree")
		if xReparentWindow == nil || xQueryTree == nil {
			embedErr = errors.New("x11: libX11 缺少 XReparentWindow / XQueryTree")
		}
	})
	return embedErr
}

// Reparent 把窗口 w 重新挂到父窗口 parent 下，位置 (x, y)。
func (d *Display) Reparent(w, parent Window, x, y int) error {
	if err := loadEmbed(); err != nil {
		return err
	}
	if d == nil || d.p == 0 || w == 0 || parent == 0 {
		return errors.New("x11: Reparent 参数无效")
	}
	xReparentWindow(d.p, uintptr(w), uintptr(parent), x, y)
	xFlush(d.p)
	return nil
}

// QueryTree 列出窗口 w 的直接子窗口。
func (d *Display) QueryTree(w Window) []Window {
	if loadEmbed() != nil || d == nil || d.p == 0 || w == 0 {
		return nil
	}
	var root, parent, children uintptr
	var n uint
	if xQueryTree(d.p, uintptr(w), &root, &parent, &children, &n) == 0 || n == 0 || children == 0 {
		return nil
	}
	out := make([]Window, 0, n)
	slice := unsafe.Slice((*uintptr)(unsafe.Pointer(children)), n)
	for i := uint(0); i < n; i++ {
		out = append(out, Window(slice[i]))
	}
	if xFree != nil {
		xFree(children)
	}
	return out
}

// GetProp32 读取 32 位属性（例如 _NET_WM_PID）；读不到返回 nil。
func (d *Display) GetProp32(w Window, prop Atom) []uint32 {
	if xGetWindowProperty == nil || d == nil || d.p == 0 || w == 0 {
		return nil
	}
	var actualType uintptr
	var actualFormat int
	var nitems, bytesAfter uintptr
	var data uintptr
	rc := xGetWindowProperty(d.p, uintptr(w), uintptr(prop), 0, 4, 1, 0,
		&actualType, &actualFormat, &nitems, &bytesAfter, &data)
	if rc != 0 || data == 0 || nitems == 0 || actualFormat != 32 {
		if data != 0 && xFree != nil {
			xFree(data)
		}
		return nil
	}
	defer func() {
		if xFree != nil {
			xFree(data)
		}
	}()
	src := unsafe.Slice((*uint32)(unsafe.Pointer(data)), nitems)
	out := make([]uint32, nitems)
	copy(out, src)
	return out
}

// WindowPID 读 _NET_WM_PID（0 = 没有这个属性）。
func (d *Display) WindowPID(w Window) int {
	vals := d.GetProp32(w, Atom(d.Atom("_NET_WM_PID")))
	if len(vals) == 0 {
		return 0
	}
	return int(vals[0])
}

// FindWindowByPID 在 root 之下按 _NET_WM_PID 找窗口（深度优先，最多 4 层）。
func (d *Display) FindWindowByPID(pid int) Window {
	if pid <= 0 {
		return 0
	}
	type item struct {
		w     Window
		depth int
	}
	queue := []item{{w: d.Root(), depth: 0}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth > 4 {
			continue
		}
		for _, child := range d.QueryTree(cur.w) {
			if d.WindowPID(child) == pid {
				return child
			}
			queue = append(queue, item{w: child, depth: cur.depth + 1})
		}
	}
	return 0
}
