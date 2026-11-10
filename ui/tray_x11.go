//go:build (linux || freebsd) && !android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// X11 系统托盘（XEmbed 协议，freedesktop）
//
// 流程：
//  1. 找 `_NET_SYSTEM_TRAY_S<screen>` 选择的所有者（托盘宿主，如 xfce4-panel / stalonetray）；
//  2. 建一个 22x22 的 override-redirect 窗口当图标，写 `_XEMBED_INFO`（version=0, flags=MAPped）；
//  3. 给宿主发 `_NET_SYSTEM_TRAY_OPCODE / SYSTEM_TRAY_REQUEST_DOCK`，宿主把我们嵌进托盘；
//  4. 在图标窗口上收 ButtonPress/ButtonRelease，左键回调 onClick、右键回调 onMenu(-1)。
//
// 不支持的部分（如实记录）：
//   - 悬停提示：XEmbed 图标没有 tooltip 通道，SetTooltip 只保存文本不显示；
//   - 自绘弹出菜单：需要在图标窗口里画自家菜单（还要抓住指针），本轮不做，
//     右键只回调 onMenu(-1) 让业务自己用 ui.MenuPanel 之类弹面板。
package ui

import (
	"fmt"
	"sync"

	"guis/internal/x11"
)

// XEmbed / 托盘协议常量（freedesktop 规范）。
const (
	xembedInfoAtom       = "_XEMBED_INFO"
	systemTrayOpcodeAtom = "_NET_SYSTEM_TRAY_OPCODE"
	xembedVersion        = 0
	xembedMapped         = 1
	systemTrayRequestDock = 0
	trayIconSize         = 22
)

var (
	x11TrayMu sync.Mutex
	x11Trays  = map[x11.Window]*Tray{}
)

// Tray X11 托盘图标。
type Tray struct {
	win     x11.Window
	tooltip string
	items   []TrayItem
	onClick func()
	onMenu  func(id int)
}

// NewTray 创建托盘图标。
//
// 桌面没有 XEmbed 托盘宿主（如纯 wlroots 的 sway 只提供 StatusNotifierItem）时返回错误，
// 调用方应先查 HasCap(CapTray) 再创建，创建失败也不会影响主窗口。
func NewTray(tooltip string, onClick func(), items []TrayItem, onMenu func(id int)) (*Tray, error) {
	x11Mu.Lock()
	dpy := x11Disp
	x11Mu.Unlock()
	if dpy == nil {
		return nil, errNoTray
	}
	selName := fmt.Sprintf("_NET_SYSTEM_TRAY_S%d", dpy.Screen())
	sel := dpy.Atom(selName)
	owner := dpy.SelectionOwner(sel)
	if owner == 0 {
		return nil, fmt.Errorf("ui: 当前桌面没有 XEmbed 系统托盘（缺 %s 所有者）", selName)
	}

	// 图标底色：主色，宿主没给主题时装图标也能看见
	icon := dpy.CreateSimpleChild(dpy.Root(), 0, 0, trayIconSize, trayIconSize, true,
		x11.AllocColor(0x63, 0x66, 0xf1))
	if icon == 0 {
		return nil, errNoTray
	}
	// 订阅鼠标事件（图标窗口自己要收点击）
	dpy.SelectInput(icon, x11.ButtonPressMask|x11.ButtonReleaseMask|x11.ExposureMask)

	t := &Tray{win: icon, tooltip: tooltip, items: items, onClick: onClick, onMenu: onMenu}

	// _XEMBED_INFO: 两个 32 位整数（version, flags）
	dpy.ChangeProp32(icon, dpy.Atom(xembedInfoAtom), dpy.Atom("ATOM"),
		[]uint32{xembedVersion, xembedMapped})
	dpy.Map(icon)
	dpy.ClearWindow(icon)

	// 请求宿主 dock
	dpy.SendClientMessage(owner, dpy.Atom(systemTrayOpcodeAtom), 0, [5]int64{
		0, // CurrentTime
		systemTrayRequestDock,
		int64(icon),
		0, 0,
	})

	x11TrayMu.Lock()
	x11Trays[icon] = t
	x11TrayMu.Unlock()
	return t, nil
}

// SetTooltip 更新托盘提示（XEmbed 图标没有 tooltip 通道，仅保存文本）。
func (t *Tray) SetTooltip(s string) {
	if t != nil {
		t.tooltip = s
	}
}

// SetItems 更新右键菜单数据（点击时通过 onMenu 回传给业务）。
func (t *Tray) SetItems(items []TrayItem) {
	if t != nil {
		t.items = items
	}
}

// Notify 系统通知：X11 走 org.freedesktop.Notifications（D-Bus），本期未接入。
func (t *Tray) Notify(title, msg string) {
	if title == "" && msg == "" {
		return
	}
	// 退化为应用内轻提示，保证调用方不会「什么都没发生」
	ToastShow(msg)
}

// Remove 移除托盘图标（幂等）。
func (t *Tray) Remove() {
	if t == nil || t.win == 0 {
		return
	}
	x11TrayMu.Lock()
	delete(x11Trays, t.win)
	x11TrayMu.Unlock()
	win := t.win
	t.win = 0
	x11Mu.Lock()
	dpy := x11Disp
	x11Mu.Unlock()
	if dpy != nil {
		dpy.Destroy(win)
	}
}

// x11DispatchTray 处理托盘图标上的鼠标事件；命中返回 true（事件被消费）。
func x11DispatchTray(ev *x11.Event) bool {
	win := x11.Window(ev.WindowOf())
	x11TrayMu.Lock()
	t := x11Trays[win]
	x11TrayMu.Unlock()
	if t == nil {
		return false
	}
	switch ev.Type() {
	case x11.ButtonPress:
		switch ev.Button() {
		case 1:
			if t.onClick != nil {
				t.onClick()
			}
		case 3:
			// -1 表示「请业务弹自己的菜单」（XEmbed 不做自绘弹出菜单）
			if t.onMenu != nil {
				t.onMenu(-1)
			}
		}
		return true
	case x11.Expose:
		x11Mu.Lock()
		dpy := x11Disp
		x11Mu.Unlock()
		if dpy != nil {
			dpy.ClearWindow(win)
		}
		return true
	}
	return false
}
