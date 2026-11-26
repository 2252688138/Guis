// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// ui 命名空间：脚本里搭界面的构造器 + 运行期改属性 / 重绘 / 退出，
// 属性名与 uidsl.Node 的 json tag 一致（text / key / onClick / w / pad / gap …）。
//
//	ui.col{ gap=12, pad=16, bg="#0f1626",
//	  ui.text{ text="标题", size=22, weight=700 },
//	  ui.button{ text="确定", onClick="ok" },
//	}

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"guis/ui"
)

// bindUI 往全局注入 ui 命名空间。
func (e *Engine) bindUI() {
	m := e.L.NewTable()
	// 全部节点类型（清单在 kinds.go，与 uidsl 的 build 分支一一对应）
	for _, kind := range allKinds {
		m.RawSetString(kind, e.ctor(kind))
	}
	m.RawSetString("set", e.L.NewFunction(e.luaSet))
	m.RawSetString("reset", e.L.NewFunction(e.luaReset))
	m.RawSetString("redraw", e.L.NewFunction(e.luaRedraw))
	m.RawSetString("quit", e.L.NewFunction(e.luaQuit))
	m.RawSetString("toast", e.L.NewFunction(e.luaToast))
	m.RawSetString("title", e.L.NewFunction(e.luaTitle))
	m.RawSetString("error", e.L.NewFunction(e.luaLastError))
	m.RawSetString("key", keyTable(e.L))
	e.bindSystem(m)
	e.L.SetGlobal("ui", m)
}

// ctor 造一个 ui.xxx 构造器。
//
// 参数可以是：文本（面向 text/button/icon/image 的位置参数）、属性表、子节点表。
func (e *Engine) ctor(kind string) *lua.LFunction {
	return e.L.NewFunction(func(L *lua.LState) int {
		t := L.NewTable()
		t.RawSetString("type", lua.LString(kind))
		kids := L.NewTable()
		n := 0
		for i := 1; i <= L.GetTop(); i++ {
			switch v := L.Get(i).(type) {
			case lua.LString:
				if kind == "input" || kind == "textarea" {
					t.RawSetString("value", v) // 输入框的位置参数是当前值
				} else if t.RawGetString("text") == lua.LNil {
					t.RawSetString("text", v)
				}
			case lua.LNumber, lua.LBool:
				// 文本位置参数：数字 / 布尔按 Lua 语义 tostring（否则构树时 JSON 类型对不上）。
				if t.RawGetString("text") == lua.LNil {
					t.RawSetString("text", lua.LString(v.String()))
				}
			case *lua.LTable:
				if isNodeTable(v) { // ui.col{ ui.text{}, ui.button{} }：子节点
					n++
					kids.RawSetInt(n, v)
				} else { // 属性表（里面数字键且是节点表的，一并当子节点）
					n += e.mergeTable(t, kids, v)
				}
			case *lua.LFunction: // 位置参数函数 → 当作 onClick
				t.RawSetString("onClick", lua.LString(e.registerHandler(v)))
			default:
				L.RaiseError("%s: 不支持的参数类型 %s", kind, v.Type().String())
				return 0
			}
		}
		coerceStringProps(t)
		if n > 0 {
			if prev, ok := t.RawGetString("children").(*lua.LTable); ok {
				prev.ForEach(func(_, val lua.LValue) { n++; kids.RawSetInt(n, val) })
			}
			t.RawSetString("children", kids)
		}
		L.Push(t)
		return 1
	})
}

// isNodeTable 判断表是不是一个节点（带 type 字段）。
func isNodeTable(t *lua.LTable) bool {
	v := t.RawGetString("type")
	if v == lua.LNil {
		return false
	}
	_, ok := v.(lua.LString)
	return ok
}

// mergeTable 把属性表里的键拷到节点表上：
// 字符串键是属性（onClick 写函数时自动注册为匿名名），数字键或节点表是子节点。
// 返回追加的子节点个数。
func (e *Engine) mergeTable(dst, kids, src *lua.LTable) int {
	n := 0
	src.ForEach(func(k, v lua.LValue) {
		if f, ok := v.(*lua.LFunction); ok && isEventKey(k.String()) {
			dst.RawSetString(k.String(), lua.LString(e.registerHandler(f)))
			return
		}
		if _, ok := k.(lua.LNumber); ok { // ui.col{ ui.text{...}, ui.button{...} } 的数组部分
			if tv, ok := v.(*lua.LTable); ok && isNodeTable(tv) {
				n++
				kids.RawSetInt(n, v)
				return
			}
			// 非节点表的数字位置参数：当文本（ui.text{42} 不应该变成非法子节点）。
			if dst.RawGetString("text") == lua.LNil {
				dst.RawSetString("text", lua.LString(v.String()))
				return
			}
			n++
			kids.RawSetInt(n, v)
			return
		}
		dst.RawSet(k, v)
	})
	return n
}

// isEventKey 这些键写函数时自动登记成匿名事件名（白名单在 events.go）。
func isEventKey(k string) bool { return eventNameOf(k) }

// stringProps uidsl 里是字符串、但脚本常写数字/布尔的属性：自动 tostring。
var stringProps = map[string]bool{
	"text": true, "label": true, "value": true, "placeholder": true, "icon": true,
	"title": true, "body": true, "src": true, "active": true, "okLabel": true,
	"dlgKind": true, "lang": true, "ctype": true, "color": true, "bg": true, "fg": true,
	"key": true, "align": true, "justify": true, "dir": true,
	"strokeColor": true, "shadowColor": true,
}

// isStringProp 该属性是否按字符串处理。
func isStringProp(p string) bool { return stringProps[p] }

// coerceStringProps 把节点表里的数字 / 布尔文本属性转成字符串（避免构树时 JSON 类型错）。
func coerceStringProps(t *lua.LTable) {
	for k := range stringProps {
		switch v := t.RawGetString(k).(type) {
		case lua.LNumber, lua.LBool:
			t.RawSetString(k, lua.LString(v.String()))
		}
	}
}

// registerHandler 把匿名函数登记成 __fnN 形式的事件名。
func (e *Engine) registerHandler(fn *lua.LFunction) string {
	e.handlerSeq++
	name := fmt.Sprintf("__fn%d", e.handlerSeq)
	e.handlers[name] = fn
	return name
}

// luaSet ui.set(key, prop, value)：运行期改属性（value 为 nil 时撤销）。
func (e *Engine) luaSet(L *lua.LState) int {
	key := L.CheckString(1)
	prop := L.CheckString(2)
	val := L.Get(3)
	if val == lua.LNil {
		e.clearOverride(key, prop)
		e.dirty.Store(true)
		ui.RequestRedraw()
		return 0
	}
	v, err := luaToAny(val, 0)
	if err != nil {
		L.RaiseError("ui.set: %v", err)
		return 0
	}
	if isStringProp(prop) {
		switch tv := v.(type) {
		case float64, bool:
			v = fmt.Sprint(tv)
		}
	}
	if err := checkProp(prop, v); err != nil {
		L.RaiseError("ui.set: %v", err)
		return 0
	}
	e.setOverride(key, prop, v)
	e.dirty.Store(true)
	ui.RequestRedraw()
	return 0
}

// luaReset ui.reset(key)：清掉某个 key 的所有运行期覆盖。
func (e *Engine) luaReset(L *lua.LState) int {
	key := L.CheckString(1)
	delete(e.overrides, key)
	e.dirty.Store(true)
	ui.RequestRedraw()
	return 0
}

// luaRedraw ui.redraw()：状态在 Go 侧变了（如回调外改共享数据）时手动刷新。
func (e *Engine) luaRedraw(L *lua.LState) int {
	e.dirty.Store(true)
	ui.RequestRedraw()
	return 0
}

// luaQuit ui.quit()：请求退出（默认 os.Exit(0)，宿主可用 Options.OnQuit 接管）。
func (e *Engine) luaQuit(L *lua.LState) int {
	// 放到独立协程里，避免在 Lua 调用栈里直接退进程导致 LState 处于半途状态。
	quit := e.opt.OnQuit
	go func() {
		defer func() { _ = recover() }()
		if quit != nil {
			quit()
		}
	}()
	return 0
}

// luaToast ui.toast(msg)：右下角轻提示。
func (e *Engine) luaToast(L *lua.LState) int {
	msg := L.CheckString(1)
	ui.ToastShow(msg)
	return 0
}

// luaTitle ui.title(s)：改窗口标题（ui 包只有 App.SetTitle，这里交给宿主钩子）。
func (e *Engine) luaTitle(L *lua.LState) int {
	t := L.CheckString(1)
	if e.opt.OnTitle != nil {
		e.opt.OnTitle(t)
		return 0
	}
	e.logf("warn", "ui.title(%q): 宿主未设置 Options.OnTitle", t)
	return 0
}

// luaLastError ui.error()：最近一次脚本错误（字符串，无错误时为空）。
func (e *Engine) luaLastError(L *lua.LState) int {
	if e.lastErr == nil {
		L.Push(lua.LString(""))
		return 1
	}
	L.Push(lua.LString(e.lastErr.Error()))
	return 1
}
