// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 事件表：属性名 → 参数个数与类型。这里是**唯一**的事件白名单，
// ctor 的匿名函数自动登记、handlersFor 的收集、文档都以此为准，
// 避免出现「白名单里有、实现里没有」的静默失效。

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"guis/ui/uidsl"
)

// eventSpec 一个事件的规格。
type eventSpec struct {
	argc int    // 回调参数个数
	desc string // 参数说明（文档 / 错误提示用）
}

// eventSpecs 事件白名单。
var eventSpecs = map[string]eventSpec{
	"onClick":    {0, ""},
	"onPress":    {0, ""},
	"onActivate": {0, ""},
	"onChange":   {1, "新值"},
	"onSubmit":   {1, "提交的文本"},
	"onKey":      {1, "键码"},
	"onKeyUp":    {1, "键码"},
	"onChar":     {1, "字符"},
	"onHover":    {1, "是否进入"},
	"onScroll":   {1, "滚动量 dy"},
	"onDrag":     {2, "x, y"},
	"onDrop":     {1, "文件路径数组"},
	"onClickAt":  {2, "x, y"},
}

// eventNameOf 事件属性名是否合法。
func eventNameOf(k string) bool {
	_, ok := eventSpecs[k]
	return ok
}

// collectHandlers 按声明树收集事件回调（未定义的事件名只报错、不阻断）。
func (e *Engine) collectHandlers(n *uidsl.Node) uidsl.Handlers {
	h := uidsl.Handlers{
		Click:    map[string]func(){},
		Change:   map[string]func(any){},
		Submit:   map[string]func(string){},
		Key:      map[string]func(uint32){},
		KeyUp:    map[string]func(uint32){},
		Char:     map[string]func(rune){},
		Hover:    map[string]func(bool){},
		Scroll:   map[string]func(float32){},
		Drag:     map[string]func(x, y float32){},
		Drop:     map[string]func([]string){},
		Press:    map[string]func(){},
		Activate: map[string]func(){},
		ClickAt:  map[string]func(x, y float32){},
	}
	check := func(name, prop string) bool {
		if e.hasHandler(name) {
			return true
		}
		e.setErr(fmt.Errorf("节点事件 %s=%q 在脚本里不存在", prop, name))
		return false
	}
	var walk func(node *uidsl.Node)
	walk = func(node *uidsl.Node) {
		if name := node.OnClick; name != "" && check(name, "onClick") {
			h.Click[name] = func() { e.fireArgs(name) }
		}
		if name := node.OnPress; name != "" && check(name, "onPress") {
			h.Press[name] = func() { e.fireArgs(name) }
		}
		if name := node.OnActivate; name != "" && check(name, "onActivate") {
			h.Activate[name] = func() { e.fireArgs(name) }
		}
		if name := node.OnChange; name != "" && check(name, "onChange") {
			h.Change[name] = func(v any) { e.fireArgs(name, v) }
		}
		if name := node.OnSubmit; name != "" && check(name, "onSubmit") {
			h.Submit[name] = func(v string) { e.fireArgs(name, v) }
		}
		if name := node.OnKey; name != "" && check(name, "onKey") {
			h.Key[name] = func(k uint32) { e.fireArgs(name, float64(k)) }
		}
		if name := node.OnKeyUp; name != "" && check(name, "onKeyUp") {
			h.KeyUp[name] = func(k uint32) { e.fireArgs(name, float64(k)) }
		}
		if name := node.OnChar; name != "" && check(name, "onChar") {
			h.Char[name] = func(r rune) { e.fireArgs(name, string(r)) }
		}
		if name := node.OnHover; name != "" && check(name, "onHover") {
			h.Hover[name] = func(in bool) { e.fireArgs(name, in) }
		}
		if name := node.OnScroll; name != "" && check(name, "onScroll") {
			h.Scroll[name] = func(dy float32) { e.fireArgs(name, float64(dy)) }
		}
		if name := node.OnDrag; name != "" && check(name, "onDrag") {
			h.Drag[name] = func(x, y float32) { e.fireArgs(name, float64(x), float64(y)) }
		}
		if name := node.OnDrop; name != "" && check(name, "onDrop") {
			h.Drop[name] = func(paths []string) {
				list := make([]any, 0, len(paths))
				for _, p := range paths {
					list = append(list, p)
				}
				e.fireArgs(name, list)
			}
		}
		if name := node.OnClickAt; name != "" && check(name, "onClickAt") {
			h.ClickAt[name] = func(x, y float32) { e.fireArgs(name, float64(x), float64(y)) }
		}
		for i := range node.Children {
			walk(&node.Children[i])
		}
	}
	walk(n)
	return h
}

// keyTable 键码常量表（ui.key.enter / ui.key.esc …）。
func keyTable(L *lua.LState) *lua.LTable {
	t := L.NewTable()
	for name, code := range map[string]int{		"enter": 13, "esc": 27, "escape": 27, "space": 32, "tab": 9,
		"backspace": 8, "delete": 46, "left": 37, "up": 38, "right": 39, "down": 40,
		"home": 36, "end": 35, "pageup": 33, "pagedown": 34,
		"f1": 112, "f2": 113, "f3": 114, "f4": 115, "f5": 116, "f6": 117,
		"f7": 118, "f8": 119, "f9": 120, "f10": 121, "f11": 122, "f12": 123,
		"shift": 16, "ctrl": 17, "alt": 18,
	} {
		t.RawSetString(name, lua.LNumber(code))
	}
	return t
}
