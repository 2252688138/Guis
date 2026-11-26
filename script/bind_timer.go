// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// timer 命名空间：脚本里的定时器。
//
//	timer.every(1000, function() ticks = ticks + 1 end)
//	local id = timer.after(2000, function() ui.toast("2 秒到了") end)
//	timer.cancel(id)
//
// 回调在独立协程里跑，但真正进 Lua 前先拿锁（Lua 不并发安全），
// 拿不到锁就直接跳过这一拍，绝不卡住 UI；回调完置脏并请求重绘。

import (
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"

	"guis/ui"
)

// bindTimer 注入 timer 命名空间。
func (e *Engine) bindTimer() {
	m := e.L.NewTable()
	m.RawSetString("after", e.L.NewFunction(e.luaAfter))
	m.RawSetString("every", e.L.NewFunction(e.luaEvery))
	m.RawSetString("cancel", e.L.NewFunction(e.luaCancel))
	e.L.SetGlobal("timer", m)
}

// luaAfter timer.after(ms, fn) → id。
func (e *Engine) luaAfter(L *lua.LState) int {
	ms := L.CheckInt(1)
	fn := L.CheckFunction(2)
	L.Push(lua.LNumber(e.addTimer(fn, true, time.Duration(ms)*time.Millisecond)))
	return 1
}

// luaEvery timer.every(ms, fn) → id。
func (e *Engine) luaEvery(L *lua.LState) int {
	ms := L.CheckInt(1)
	fn := L.CheckFunction(2)
	if ms <= 0 {
		L.RaiseError("timer.every: 间隔必须大于 0")
		return 0
	}
	L.Push(lua.LNumber(e.addTimer(fn, false, time.Duration(ms)*time.Millisecond)))
	return 1
}

// luaCancel timer.cancel(id)。调用方已持锁（DoString / timer 回调都在锁内）。
func (e *Engine) luaCancel(L *lua.LState) int {
	id := L.CheckInt(1)
	if t, ok := e.timers[id]; ok {
		close(t.stop)
		delete(e.timers, id)
	}
	return 0
}

// addTimer 启动一个定时器（调用方已持锁）。
func (e *Engine) addTimer(fn *lua.LFunction, once bool, d time.Duration) int {
	e.timerSeq++
	id := e.timerSeq
	t := &timerEntry{fn: fn, once: once, stop: make(chan struct{})}
	e.timers[id] = t
	go e.runTimer(id, t, d)
	return id
}

// runTimer 定时器主体。
func (e *Engine) runTimer(id int, t *timerEntry, d time.Duration) {
	if t.once {
		select {
		case <-time.After(d):
		case <-t.stop:
			return
		}
		e.callTimer(id, t)
		return
	}
	tk := time.NewTicker(d)
	defer tk.Stop()
	for {
		select {
		case <-tk.C:
			e.callTimer(id, t)
		case <-t.stop:
			return
		}
	}
}

// callTimer 进 Lua 跑一次回调（拿不到锁就跳过这一拍）。
func (e *Engine) callTimer(id int, t *timerEntry) {
	if !e.mu.TryLock() {
		return
	}
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	if _, ok := e.timers[id]; !ok { // 已被 cancel
		return
	}
	if t.once {
		delete(e.timers, id)
	}
	if err := e.run(func() error {
		return e.L.CallByParam(lua.P{Fn: t.fn, NRet: 0, Protect: true})
	}); err != nil {
		e.setErr(fmt.Errorf("定时器 %d: %w", id, err))
		return
	}
	e.dirty.Store(true)
	ui.RequestRedraw()
}
