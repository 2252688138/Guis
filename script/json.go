// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// json 命名空间：脚本侧 JSON 编解码，方便直接消费 http.* 返回的 JSON。
//
//	local t = json.decode(body)
//	local s = json.encode{ ok = true, list = {1, 2, 3} }

import (
	"encoding/json"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// bindJSON 注入 json 命名空间。
func (e *Engine) bindJSON() {
	m := e.L.NewTable()
	m.RawSetString("encode", e.L.NewFunction(e.luaJSONEncode))
	m.RawSetString("decode", e.L.NewFunction(e.luaJSONDecode))
	e.L.SetGlobal("json", m)
}

// luaJSONEncode json.encode(v) → 字符串。
func (e *Engine) luaJSONEncode(L *lua.LState) int {
	v, err := luaToAny(L.Get(1), 0)
	if err != nil {
		L.RaiseError("json.encode: %v", err)
		return 0
	}
	b, err := json.Marshal(v)
	if err != nil {
		L.RaiseError("json.encode: %v", err)
		return 0
	}
	L.Push(lua.LString(string(b)))
	return 1
}

// luaJSONDecode json.decode(s) → 值；解析失败抛错。
func (e *Engine) luaJSONDecode(L *lua.LState) int {
	s := L.CheckString(1)
	var v any
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		L.RaiseError("json.decode: %v", err)
		return 0
	}
	L.Push(anyToLua(L, v, 0))
	return 1
}

// anyToLua Go 值 → Lua 值（JSON 解码结果用）。
func anyToLua(L *lua.LState, v any, depth int) lua.LValue {
	if depth > maxDepth {
		return lua.LNil
	}
	switch x := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(x)
	case string:
		return lua.LString(x)
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return lua.LNumber(i)
		}
		if f, err := x.Float64(); err == nil {
			return lua.LNumber(f)
		}
		return lua.LString(x.String())
	case float64:
		return lua.LNumber(x)
	case int:
		return lua.LNumber(x)
	case int64:
		return lua.LNumber(x)
	case []any:
		t := L.NewTable()
		for i, item := range x {
			t.RawSetInt(i+1, anyToLua(L, item, depth+1))
		}
		return t
	case map[string]any:
		t := L.NewTable()
		for k, item := range x {
			t.RawSetString(k, anyToLua(L, item, depth+1))
		}
		return t
	default:
		return lua.LString("[unsupported]")
	}
}
