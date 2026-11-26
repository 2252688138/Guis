// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 脚本表树 → uidsl.Node → ui.Node 的桥梁，以及「脏了才重建」的树缓存。
//
// 属性名直接复用 uidsl.Node 的 json tag（text / key / onClick / w / pad / gap /
// grow / bg / fg / size / weight …），所以 Lua、JSON、缩进 DSL 三套写法的属性完全一致，
// uidsl 以后加字段这里自动支持。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	lua "github.com/yuin/gopher-lua"

	"guis/ui"
	"guis/ui/uidsl"
)

// maxDepth 表嵌套深度上限（防止脚本递归造表把栈打爆）。
const maxDepth = 64

// Root 供 ui.Run / SetEntry 使用：返回当前界面树。
//
// 没有脚本状态变化时直接返回上一棵树（不进 Lua）；构树中拿不到锁（有定时器/事件在跑）
// 就先用旧树顶上并请求下一帧重绘，UI 线程不会被脚本阻塞。
func (e *Engine) Root() *ui.Node {
	if e == nil {
		return nil
	}
	if !e.dirty.Load() {
		if r := e.root.Load(); r != nil && !e.hasQueued() {
			return r
		}
	}
	if !e.mu.TryLock() {
		// 已经有定时器 / 事件在跑：先用上一棵树顶上，下一帧再重建（UI 线程不等脚本）。
		// 但首次构树前必须拿到锁，否则会把 nil 交给 ui.Run。
		if r := e.root.Load(); r != nil {
			ui.RequestRedraw()
			return r
		}
		e.mu.Lock()
	}
	defer e.mu.Unlock()
	if e.closed {
		return e.root.Load()
	}
	dirty := e.dirty.Load()
	e.dirty.Store(false) // 先清：view() 里改了状态（ui.set 等）会再置脏，下一帧重建
	e.pumpLocked()       // 锁被占时排队的点击 / 输入先补上（它们会把状态改脏）
	if e.root.Load() == nil || dirty || e.dirty.Load() {
		if err := e.run(e.buildLocked); err != nil {
			e.setErr(err)
		}
	}
	return e.root.Load()
}

// buildLocked 调 Lua 的 view() 并构树（调用方已持锁）。
func (e *Engine) buildLocked() error {
	fn, ok := e.L.GetGlobal("view").(*lua.LFunction)
	if !ok {
		return errors.New("script: 脚本未定义 view() 函数")
	}
	if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
		return fmt.Errorf("view(): %w", err)
	}
	ret := e.L.Get(-1)
	e.L.Pop(1)
	t, ok := ret.(*lua.LTable)
	if !ok {
		return fmt.Errorf("view() 必须返回节点表（如 ui.col{...}），实际返回 %s", ret.Type().String())
	}
	decl, err := tableToDecl(t)
	if err != nil {
		return fmt.Errorf("view(): %w", err)
	}
	if err := validateKinds(&decl); err != nil {
		return fmt.Errorf("view(): %w", err)
	}
	e.applyOverrides(&decl)
	e.root.Store(uidsl.BuildWith(decl, e.handlersFor(&decl)))
	// 组件降级（video / webview / viewport 打不开）转成脚本错误，界面里能用 ui.error() 看到。
	for _, msg := range uidsl.DrainDegrades() {
		e.setErr(errors.New(msg))
	}
	return nil
}

// tableToDecl 把 Lua 表转成 uidsl.Node（走 JSON，属性名与 uidsl 完全一致）。
func tableToDecl(t *lua.LTable) (uidsl.Node, error) {
	var n uidsl.Node
	v, err := luaToAny(t, 0)
	if err != nil {
		return n, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return n, fmt.Errorf("节点表无法序列化: %w", err)
	}
	if err := json.Unmarshal(b, &n); err != nil {
		return n, fmt.Errorf("节点表字段不合法（%s）: %w", string(b), err)
	}
	return n, nil
}

// luaToAny 把 Lua 值转成 Go 值（表：连续整数键为数组，否则为 map）。
func luaToAny(v lua.LValue, depth int) (any, error) {
	if depth > maxDepth {
		return nil, errors.New("表嵌套超过 64 层")
	}
	switch x := v.(type) {
	case *lua.LNilType:
		return nil, nil
	case lua.LBool:
		return bool(x), nil
	case lua.LString:
		return string(x), nil
	case lua.LNumber:
		return float64(x), nil
	case *lua.LTable:
		n := x.Len()
		count := 0
		x.ForEach(func(_, _ lua.LValue) { count++ })
		if n > 0 && count == n { // 纯数组：children 列表
			arr := make([]any, 0, n)
			for i := 1; i <= n; i++ {
				item, err := luaToAny(x.RawGetInt(i), depth+1)
				if err != nil {
					return nil, err
				}
				arr = append(arr, item)
			}
			return arr, nil
		}
		m := make(map[string]any, count)
		var err error
		x.ForEach(func(k, val lua.LValue) {
			if err != nil {
				return
			}
			var key string
			switch kk := k.(type) {
			case lua.LString:
				key = string(kk)
			case lua.LNumber:
				key = strconv.FormatFloat(float64(kk), 'f', -1, 64)
			default:
				err = fmt.Errorf("表键只能是字符串或数字，实际是 %s", k.Type().String())
				return
			}
			m[key], err = luaToAny(val, depth+1)
		})
		return m, err
	default:
		return nil, fmt.Errorf("不支持的 Lua 值类型 %s", v.Type().String())
	}
}

// handlersFor 按声明树收集事件回调（点击 / 值变化 / 回车），未定义的 handler 只报错不阻断。
func (e *Engine) handlersFor(n *uidsl.Node) uidsl.Handlers {
	return e.collectHandlers(n)
}

// hasHandler 事件名是否可调用（匿名注册的函数或脚本全局函数）。
func (e *Engine) hasHandler(name string) bool {
	if e.handlers[name] != nil {
		return true
	}
	_, ok := e.L.GetGlobal(name).(*lua.LFunction)
	return ok
}

// fire 触发一个事件（UI 线程调）。
//
// 拿不到锁（定时器 / 另一个事件 / http 正在跑）时不阻塞 UI：事件进队列，
// 下一帧 Root 或定时器持锁时补跑。
func (e *Engine) fire(name string) { e.fireArgs(name) }

// fireValue 触发带参数的事件（勾选 → bool、输入 → string、滑块 → number）。
func (e *Engine) fireValue(name string, val any) { e.fireArgs(name, val) }

// fireArgs 触发任意参数个数的事件。
func (e *Engine) fireArgs(name string, args ...any) {
	if e.closed {
		return
	}
	if !e.mu.TryLock() {
		e.enqueue(name, args)
		ui.RequestRedraw()
		return
	}
	e.deliverLocked(name, args)
	e.mu.Unlock()
}

// fireEvent 兼容旧签名（单个可空参数）。
func (e *Engine) fireEvent(name string, arg any) {
	if arg == nil {
		e.fireArgs(name)
		return
	}
	e.fireArgs(name, arg)
}

// deliverLocked 在持锁状态下真正调用 Lua 回调（调用方已持锁）。
func (e *Engine) deliverLocked(name string, args []any) {
	if e.closed {
		return
	}
	fn := e.handlerOf(name)
	if fn == nil {
		e.setErr(fmt.Errorf("事件 %q 没有对应函数", name))
		return
	}
	cp := lua.P{Fn: fn, NRet: 0, Protect: true}
	// 参数逐个转成 Lua 值；空参数不传，避免脚本收到多余的 nil。
	lvs := make([]lua.LValue, 0, len(args))
	for _, a := range args {
		lvs = append(lvs, goToLua(a))
	}
	if err := e.run(func() error { return e.L.CallByParam(cp, lvs...) }); err != nil {
		e.setErr(fmt.Errorf("事件 %s: %w", name, err))
		return
	}
	e.dirty.Store(true)
	ui.RequestRedraw()
}

// handlerOf 找事件对应的 Lua 函数（匿名注册的优先，其次脚本全局）。
func (e *Engine) handlerOf(name string) *lua.LFunction {
	if fn := e.handlers[name]; fn != nil {
		return fn
	}
	fn, _ := e.L.GetGlobal(name).(*lua.LFunction)
	return fn
}

// goToLua 把 Go 值转成 Lua 值（回调用到的几种基本类型）。
func goToLua(v any) lua.LValue {
	switch x := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(x)
	case string:
		return lua.LString(x)
	case float64:
		return lua.LNumber(x)
	case float32:
		return lua.LNumber(x)
	case int:
		return lua.LNumber(x)
	case int64:
		return lua.LNumber(x)
	default:
		return lua.LString(fmt.Sprint(v))
	}
}

// setOverride 记录 ui.set(key, prop, value) 的覆盖（构树后套用）。
func (e *Engine) setOverride(key, prop string, val any) {
	m := e.overrides[key]
	if m == nil {
		m = map[string]any{}
		e.overrides[key] = m
	}
	m[prop] = val
	e.dirty.Store(true)
}

// clearOverride 删除某个覆盖项（value=nil 时用）。
func (e *Engine) clearOverride(key, prop string) {
	if m := e.overrides[key]; m != nil {
		delete(m, prop)
		if len(m) == 0 {
			delete(e.overrides, key)
		}
		e.dirty.Store(true)
	}
}

// applyOverrides 按 key 把覆盖套到声明树上。
func (e *Engine) applyOverrides(n *uidsl.Node) {
	if len(e.overrides) == 0 {
		return
	}
	var walk func(node *uidsl.Node)
	walk = func(node *uidsl.Node) {
		if props, ok := e.overrides[node.Key]; ok && node.Key != "" {
			if err := assignProps(node, props); err != nil {
				e.setErr(err)
			}
		}
		for i := range node.Children {
			walk(&node.Children[i])
		}
	}
	walk(n)
}

// checkProp 校验 ui.set 的属性名与类型（未知属性直接报错，不再静默失效）。
func checkProp(prop string, val any) error {
	var n uidsl.Node
	return assignProps(&n, map[string]any{prop: val})
}

// assignProps 把 ui.set 的覆盖写进节点。
//
// 做法：先复制现有声明，再用严格 JSON 解码覆盖——false / 0 / 空串这些零值也能写进去
// （旧实现只拷非零字段，关不掉东西），属性名拼错会报错而不是静默丢弃。
func assignProps(dst *uidsl.Node, props map[string]any) error {
	b, err := json.Marshal(props)
	if err != nil {
		return fmt.Errorf("ui.set 的值无法序列化: %w", err)
	}
	tmp := *dst
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tmp); err != nil {
		return fmt.Errorf("ui.set 属性不合法（%v）: %w", props, err)
	}
	*dst = tmp
	return nil
}
