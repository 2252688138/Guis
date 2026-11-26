// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 本轮补齐的能力：执行超时、ui.set 零值 / 未知属性、带参事件、arg 表、
// on_exit 钩子、热重载清定时器、值控件（input / checkbox / switch / slider）。

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"guis/ui/uidsl"
)

func mustDo(t *testing.T, e *Engine, src string) {
	t.Helper()
	if err := e.DoString(src); err != nil {
		t.Fatalf("DoString: %v", err)
	}
}

// TestExecTimeout 死循环被 ExecTimeout 打断，不会永久卡住。
func TestExecTimeout(t *testing.T) {
	e := newEngine(t, Options{ExecTimeout: 80 * time.Millisecond})
	err := e.DoString(`while true do end`)
	if err == nil {
		t.Fatal("死循环应被超时打断")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("错误里应说明超时，实际: %v", err)
	}
}

// TestAssignPropsZeroValue ui.set 能写零值，且未知属性报错。
func TestAssignPropsZeroValue(t *testing.T) {
	n := uidsl.Node{Type: "text", Key: "t", Text: "hi", Size: 20}
	if err := assignProps(&n, map[string]any{"size": float64(0), "text": ""}); err != nil {
		t.Fatalf("assignProps: %v", err)
	}
	if n.Size != 0 || n.Text != "" {
		t.Fatalf("零值没写进去: size=%v text=%q", n.Size, n.Text)
	}
	if err := assignProps(&n, map[string]any{"nope": 1}); err == nil {
		t.Fatal("未知属性应该报错")
	}
}

// TestSetUnknownPropRaise ui.set 写错属性名时脚本立刻拿到错误。
func TestSetUnknownPropRaise(t *testing.T) {
	e := newEngine(t, Options{})
	err := e.DoString(`ui.set("t", "nope", 1)`)
	if err == nil || !strings.Contains(err.Error(), "ui.set") {
		t.Fatalf("应该报 ui.set 错误，实际: %v", err)
	}
}

// TestFireValuePassesArg 带参事件把 bool / string / number 传进 Lua。
func TestFireValuePassesArg(t *testing.T) {
	e := newEngine(t, Options{})
	mustDo(t, e, `got = nil
function on_ch(v) got = v end
function view()
  return ui.col{ ui.checkbox{ key="c", checked=true, onChange="on_ch" } }
end`)
	e.Root()
	e.fireValue("on_ch", true)
	e.mu.Lock()
	v := e.L.GetGlobal("got")
	e.mu.Unlock()
	if fmt.Sprint(v) != "true" {
		t.Fatalf("Lua 没收到参数，got=%v", v)
	}
}

// TestValueWidgetsBuild 值控件都能构出节点（key 自动兜底）。
func TestValueWidgetsBuild(t *testing.T) {
	e := newEngine(t, Options{})
	mustDo(t, e, `function view()
  return ui.col{
    ui.input{ key="q", value="hi", onChange="on_ch" },
    ui.textarea{ key="ta", value="multi" },
    ui.checkbox{ key="c", text="启用", checked=true },
    ui.switch{ key="s", checked=true },
    ui.slider{ key="sl", pct=0.5 },
    ui.progress{ pct=0.3 },
  }
end
function on_ch(v) end`)
	root := e.Root()
	if root == nil {
		t.Fatal("构树失败")
	}
	for _, k := range []string{"q", "ta", "c", "s", "sl"} {
		if findKey(root, k) == nil {
			t.Fatalf("缺少控件 %q", k)
		}
	}
}

// TestArgsTable arg[0] 是脚本路径，arg[1..] 是参数。
func TestArgsTable(t *testing.T) {
	e := newEngine(t, Options{})
	e.SetArgs([]string{"ui.lua", "hello"})
	mustDo(t, e, `function view() return ui.text{ key="t", text=arg[1] } end`)
	n := findKey(e.Root(), "t")
	if n == nil || n.Text != "hello" {
		t.Fatalf("arg 表没生效: %+v", n)
	}
}

// TestShutdownRunsOnExit on_exit() 与 Options.OnExit 都会执行。
func TestShutdownRunsOnExit(t *testing.T) {
	hostCalled := false
	e := newEngine(t, Options{OnExit: func() { hostCalled = true }})
	mustDo(t, e, `exit_ran = false
function on_exit() exit_ran = true end
function view() return ui.text{ text="x" } end`)
	e.shutdown()
	e.mu.Lock()
	v := e.L.GetGlobal("exit_ran")
	e.mu.Unlock()
	if fmt.Sprint(v) != "true" || !hostCalled {
		t.Fatalf("on_exit 未执行: lua=%v host=%v", v, hostCalled)
	}
}

// TestResetScriptStateClearsTimers 热重载前把旧定时器全停掉。
func TestResetScriptStateClearsTimers(t *testing.T) {
	e := newEngine(t, Options{})
	mustDo(t, e, `function tick() end
function view() return ui.text{ text="x" } end
timer.every(1000, tick)`)
	e.mu.Lock()
	n := len(e.timers)
	e.mu.Unlock()
	if n != 1 {
		t.Fatalf("定时器应已注册，实际 %d", n)
	}
	e.mu.Lock()
	e.resetScriptState()
	left := len(e.timers)
	e.mu.Unlock()
	if left != 0 {
		t.Fatalf("重载后定时器应清空，实际 %d", left)
	}
}
