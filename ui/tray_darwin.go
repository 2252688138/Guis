//go:build darwin && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// macOS 系统托盘：NSStatusBar / NSStatusItem。
//
// 行为约定（与 Windows 托盘尽量对齐）：
//   - 有菜单项时，点图标弹出 NSMenu（点菜单项回调 onMenu(id)，id 即 TrayItem.ID）；
//   - 没有菜单项时，点图标回调 onClick；
//   - SetTooltip 写按钮的 toolTip（悬停显示）；
//   - Submenu 生成二级菜单；Separator / Checked / Disabled 都映射到 NSMenuItem。
package ui

import (
	"sync"

	"guis/internal/objc"

	"github.com/ebitengine/purego"
)

// 常量（NSEvent / NSStatusBar）。
const (
	nsVariableStatusItemLength = -1 // NSVariableStatusItemLength
	nsControlStateOn           = 1  // NSControlStateValueOn
)

var (
	trayStateMu   sync.Mutex
	trayTargets   = map[uintptr]*Tray{} // GoguiTrayTarget* → Tray
	trayClass     uintptr
	trayCallbacks []uintptr
)

// Tray macOS 托盘图标。
type Tray struct {
	item   uintptr // NSStatusItem*
	button uintptr // NSButton*（statusItem.button）
	target uintptr // GoguiTrayTarget*
	tooltip string
	items  []TrayItem
	onClick func()
	onMenu  func(id int)
}

// ensureTrayClass 创建带回调方法的 target 类（只创建一次）。
func ensureTrayClass() uintptr {
	if trayClass != 0 {
		return trayClass
	}
	cls := objc.Subclass(objc.Class("NSObject"), "GoguiTrayTarget")
	if cls == 0 {
		return 0
	}
	click := purego.NewCallback(func(self, cmd, sender uintptr) uintptr {
		trayStateMu.Lock()
		t := trayTargets[self]
		trayStateMu.Unlock()
		if t != nil && t.onClick != nil {
			t.onClick()
		}
		return 0
	})
	pick := purego.NewCallback(func(self, cmd, sender uintptr) uintptr {
		trayStateMu.Lock()
		t := trayTargets[self]
		trayStateMu.Unlock()
		if t == nil || t.onMenu == nil || sender == 0 {
			return 0
		}
		t.onMenu(int(objc.IntOf(sender, "tag")))
		return 0
	})
	trayCallbacks = append(trayCallbacks, click, pick)
	objc.AddMethod(cls, "onClick:", click, "v@:@")
	objc.AddMethod(cls, "onPick:", pick, "v@:@")
	trayClass = cls
	return cls
}

// NewTray 创建状态栏图标。失败时返回 errNoTray（业务侧可忽略并隐藏入口）。
func NewTray(tooltip string, onClick func(), items []TrayItem, onMenu func(id int)) (*Tray, error) {
	if err := objc.Load(); err != nil {
		return nil, errNoTray
	}
	cls := ensureTrayClass()
	if cls == 0 {
		return nil, errNoTray
	}
	bar := objc.Send(objc.Class("NSStatusBar"), "systemStatusBar")
	if bar == 0 {
		return nil, errNoTray
	}
	item := objc.SendFRet(bar, "statusItemWithLength:", float64(nsVariableStatusItemLength))
	if item == 0 {
		return nil, errNoTray
	}
	// 不是 ARC 环境：手动 retain，避免被自动释放池回收
	objc.Send(item, "retain")

	t := &Tray{
		item:    item,
		button:  objc.Send(item, "button"),
		target:  objc.Send(objc.Send(cls, "alloc"), "init"),
		tooltip: tooltip,
		items:   items,
		onClick: onClick,
		onMenu:  onMenu,
	}
	if t.button != 0 {
		objc.Send1(t.button, "setTitle:", objc.NSString("●"))
		if tooltip != "" {
			objc.Send1(t.button, "setToolTip:", objc.NSString(tooltip))
		}
		objc.Send1(t.button, "setTarget:", t.target)
		objc.Send1(t.button, "setAction:", objc.Sel("onClick:"))
	}
	trayStateMu.Lock()
	trayTargets[t.target] = t
	trayStateMu.Unlock()
	t.SetItems(items)
	return t, nil
}

// SetTooltip 更新悬停提示。
func (t *Tray) SetTooltip(s string) {
	if t == nil || t.button == 0 {
		return
	}
	t.tooltip = s
	objc.Send1(t.button, "setToolTip:", objc.NSString(s))
}

// SetItems 重建右键 / 下拉菜单。
func (t *Tray) SetItems(items []TrayItem) {
	if t == nil || t.item == 0 {
		return
	}
	t.items = items
	if len(items) == 0 {
		// 没有菜单：点击只走 onClick
		objc.Send1(t.item, "setMenu:", 0)
		return
	}
	menu := objc.Send(objc.Send(objc.Class("NSMenu"), "alloc"), "init")
	if menu == 0 {
		return
	}
	objc.SendB(menu, "setAutoenablesItems:", false)
	addTrayMenuItems(menu, items, t.target)
	objc.Send1(t.item, "setMenu:", menu)
}

// addTrayMenuItems 递归构建菜单项（含二级菜单）。
func addTrayMenuItems(menu uintptr, items []TrayItem, target uintptr) {
	for _, it := range items {
		if it.Separator {
			sep := objc.Send(objc.Class("NSMenuItem"), "separatorItem")
			if sep != 0 {
				objc.Send1(menu, "addItem:", sep)
			}
			continue
		}
		mi := objc.Send(objc.Class("NSMenuItem"), "alloc")
		mi = objc.Send3(mi, "initWithTitle:action:keyEquivalent:",
			objc.NSString(it.Label), objc.Sel("onPick:"), objc.NSString(""))
		if mi == 0 {
			continue
		}
		objc.Send1(mi, "setTarget:", target)
		objc.SendI(mi, "setTag:", int64(it.ID))
		objc.SendB(mi, "setEnabled:", !it.Disabled)
		if it.Checked {
			objc.SendI(mi, "setState:", nsControlStateOn)
		}
		if len(it.Submenu) > 0 {
			sub := objc.Send(objc.Send(objc.Class("NSMenu"), "alloc"), "init")
			if sub != 0 {
				addTrayMenuItems(sub, it.Submenu, target)
				objc.Send1(mi, "setSubmenu:", sub)
			}
		}
		objc.Send1(menu, "addItem:", mi)
	}
}

// Notify 系统通知：macOS 需要 UNUserNotificationCenter（10.14+），
// 本期未接入（首次使用要弹授权），退化为应用内轻提示。
func (t *Tray) Notify(title, msg string) {
	if msg == "" {
		return
	}
	ToastShow(msg)
}

// Remove 移除状态栏图标（幂等）。
func (t *Tray) Remove() {
	if t == nil || t.item == 0 {
		return
	}
	trayStateMu.Lock()
	delete(trayTargets, t.target)
	trayStateMu.Unlock()
	if bar := objc.Send(objc.Class("NSStatusBar"), "systemStatusBar"); bar != 0 {
		objc.Send1(bar, "removeStatusItem:", t.item)
	}
	t.item = 0
	t.button = 0
}
