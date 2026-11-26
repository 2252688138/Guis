// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 系统能力：剪贴板、通知、焦点、滚动定位、文件选择框。

import (
	lua "github.com/yuin/gopher-lua"

	"guis/ui"
)

// bindSystem 把系统级能力挂到 ui 命名空间上。
func (e *Engine) bindSystem(m *lua.LTable) {
	clip := e.L.NewTable()
	clip.RawSetString("get", e.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(ui.ClipboardText()))
		return 1
	}))
	clip.RawSetString("set", e.L.NewFunction(func(L *lua.LState) int {
		ok := ui.ClipboardSetText(L.CheckString(1))
		L.Push(lua.LBool(ok))
		return 1
	}))
	m.RawSetString("clipboard", clip)

	m.RawSetString("focus", e.L.NewFunction(func(L *lua.LState) int {
		ui.SetFocus(L.CheckString(1))
		ui.RequestRedraw()
		return 0
	}))
	m.RawSetString("scrollTo", e.L.NewFunction(func(L *lua.LState) int {
		ui.ScrollTo(L.CheckString(1), float32(L.CheckNumber(2)))
		ui.RequestRedraw()
		return 0
	}))
	m.RawSetString("notify", e.L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		title := L.OptString(2, "")
		kind := L.OptString(3, "info")
		ui.Notify(ui.Notice{Title: title, Body: msg, Kind: kind})
		return 0
	}))
	m.RawSetString("notifyProgress", e.L.NewFunction(func(L *lua.LState) int {
		id := L.CheckString(1)
		pct := float32(L.CheckNumber(2))
		body := L.OptString(3, "")
		ui.NotifyProgress(id, pct, body)
		return 0
	}))
	m.RawSetString("notifyClose", e.L.NewFunction(func(L *lua.LState) int {
		ui.NotifyClose(L.CheckString(1))
		return 0
	}))
	m.RawSetString("notifyClear", e.L.NewFunction(func(L *lua.LState) int {
		ui.NotifyClear()
		return 0
	}))
	m.RawSetString("openFile", e.L.NewFunction(func(L *lua.LState) int {
		title := L.OptString(1, "选择文件")
		filter := L.OptString(2, "")
		if p, ok := ui.OpenFileDialog(title, filter); ok {
			L.Push(lua.LString(p))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))
}
