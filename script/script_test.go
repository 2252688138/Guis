// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"guis/ui"
)

// newEngine 建一个测试用引擎（自动关闭）。
func newEngine(t *testing.T, opt Options) *Engine {
	t.Helper()
	e, err := New(opt)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(e.Close)
	return e
}

// findKey 在节点树里按 Key 找节点。
func findKey(n *ui.Node, key string) *ui.Node {
	if n == nil {
		return nil
	}
	if n.Key == key {
		return n
	}
	for _, c := range n.Children {
		if got := findKey(c, key); got != nil {
			return got
		}
	}
	return nil
}

func TestViewBuildAndEvent(t *testing.T) {
	e := newEngine(t, Options{})
	src := `
count = 0
function inc() count = count + 1 end
function view()
  return ui.col{ pad=8, gap=4,
    ui.text{ key="cnt", text=tostring(count), size=20 },
    ui.button{ key="plus", text="+1", onClick="inc" },
  }
end`
	if err := e.DoString(src); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	root := e.Root()
	if root == nil {
		t.Fatal("Root() 为空")
	}
	if got := findKey(root, "cnt"); got == nil || got.Text != "0" {
		t.Fatalf("cnt 文本 = %v，期望 0", got)
	}
	btn := findKey(root, "plus")
	if btn == nil {
		t.Fatal("找不到 plus 按钮")
	}
	if btn.OnClick == nil {
		t.Fatal("按钮没有绑定 onClick")
	}
	btn.OnClick()
	root = e.Root()
	if got := findKey(root, "cnt"); got == nil || got.Text != "1" {
		t.Fatalf("点击后 cnt = %v，期望 1", got)
	}
}

func TestSetOverrideAndReset(t *testing.T) {
	e := newEngine(t, Options{})
	src := `
function bump() ui.set("t", "text", "改过了") end
function view() return ui.text{ key="t", text="原文", size=18 } end`
	if err := e.DoString(src); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if got := findKey(e.Root(), "t"); got == nil || got.Text != "原文" {
		t.Fatalf("初始文本 = %v", got)
	}
	if err := e.DoString(`bump()`); err != nil {
		t.Fatalf("调用 bump: %v", err)
	}
	if got := findKey(e.Root(), "t"); got == nil || got.Text != "改过了" {
		t.Fatalf("覆盖后文本 = %v，期望 改过了", got)
	}
	if err := e.DoString(`ui.reset("t")`); err != nil {
		t.Fatalf("ui.reset: %v", err)
	}
	if got := findKey(e.Root(), "t"); got == nil || got.Text != "原文" {
		t.Fatalf("撤销后文本 = %v，期望 原文", got)
	}
}

func TestSyntaxErrorKeepsOldTree(t *testing.T) {
	e := newEngine(t, Options{OnError: func(error) {}})
	if err := e.DoString(`function view() return ui.text{ text="v1" } end`); err != nil {
		t.Fatalf("首次装载: %v", err)
	}
	if got := e.Root(); got == nil || got.Text != "v1" {
		t.Fatalf("首次构树 = %v", got)
	}
	if err := e.DoString(`function view( return ui.text{ text="v2" } end`); err == nil {
		t.Fatal("语法错误应该报错")
	}
	if e.LastError() == nil {
		t.Fatal("LastError 应为非空")
	}
	if got := e.Root(); got == nil || got.Text != "v1" {
		t.Fatalf("失败后应保留旧树，实际 = %v", got)
	}
}

func TestSandbox(t *testing.T) {
	e := newEngine(t, Options{})
	src := `
assert(io == nil, "io 不应存在")
assert(dofile == nil, "dofile 不应存在")
assert(loadfile == nil, "loadfile 不应存在")
assert(require ~= nil, "require 应当是受限版（只能加载 WorkDir 内的 .lua）")
assert(type(require) == "function", "require 应为函数")
assert(os.execute == nil, "os.execute 不应存在")
function view() return ui.text{ text="ok" } end`
	if err := e.DoString(src); err != nil {
		t.Fatalf("沙箱断言失败: %v", err)
	}
}

func TestFSBoundaryAndSwitch(t *testing.T) {
	dir := t.TempDir()
	e := newEngine(t, Options{WorkDir: dir})
	src := `
fs.write("a/b.txt", "hello")
assert(fs.read("a/b.txt") == "hello", "读回不匹配")
local ok = pcall(function() return fs.read("../escape.txt") end)
assert(ok == false, "越界路径应该报错")
function view() return ui.text{ text="fs" } end`
	if err := e.DoString(src); err != nil {
		t.Fatalf("fs 测试失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "b.txt")); err != nil {
		t.Fatalf("文件未写入: %v", err)
	}

	off := newEngine(t, Options{WorkDir: dir, NoFS: true, NoNet: true})
	src2 := `
local ok = pcall(function() return fs.read("a/b.txt") end)
assert(ok == false, "NoFS 时应报错")
local ok2 = pcall(function() return http.get("https://example.com") end)
assert(ok2 == false, "NoNet 时应报错")
function view() return ui.text{ text="off" } end`
	if err := off.DoString(src2); err != nil {
		t.Fatalf("开关测试失败: %v", err)
	}
}

func TestTimerEveryAndCancel(t *testing.T) {
	e := newEngine(t, Options{})
	src := `
ticks = 0
id = timer.every(15, function() ticks = ticks + 1 end)
function view() return ui.text{ key="t", text=tostring(ticks) } end`
	if err := e.DoString(src); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if n := readTicks(t, e); n < 2 {
		t.Fatalf("定时器触发次数 %d，期望 >=2", n)
	}
	if err := e.DoString(`timer.cancel(id)`); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	base := readTicks(t, e)
	time.Sleep(200 * time.Millisecond)
	if after := readTicks(t, e); after > base+1 {
		t.Fatalf("cancel 后仍在触发: %d → %d", base, after)
	}
}

// readTicks 反复取树直到拿到最新的定时器计数（Root 碰上定时器持锁时会返回旧树）。
func readTicks(t *testing.T, e *Engine) int {
	t.Helper()
	best := 0
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if n := findKey(e.Root(), "t"); n != nil {
			if v := atoiOrZero(n.Text); v > best {
				best = v
				if v >= 2 {
					return v
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return best
}

func TestTimerAfterOnce(t *testing.T) {
	e := newEngine(t, Options{})
	src := `
hits = 0
timer.after(10, function() hits = hits + 1 end)
function view() return ui.text{ key="h", text=tostring(hits) } end`
	if err := e.DoString(src); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := atoiOrZero(e.Root().Text); n != 1 {
		t.Fatalf("after 触发 %d 次，期望 1", n)
	}
}

func TestHotReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.lua")
	writeFile(t, path, "function view() return ui.text{ key=\"t\", text=\"v1\" } end")
	e, err := Attach(path, Options{Interval: 20 * time.Millisecond, OnError: func(error) {}})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	t.Cleanup(e.Close)
	if got := e.Root(); got == nil || got.Text != "v1" {
		t.Fatalf("初始 = %v", got)
	}
	time.Sleep(30 * time.Millisecond)
	writeFile(t, path, "function view() return ui.text{ key=\"t\", text=\"v2\" } end")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := e.Root(); got != nil && got.Text == "v2" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("热重载未生效，当前 = %v，错误 = %v", e.Root(), e.LastError())
}

func TestAnonymousHandler(t *testing.T) {
	e := newEngine(t, Options{})
	src := `
hits = 0
function view()
  return ui.button{ key="b", text="点", onClick=function() hits = hits + 1; ui.set("b", "text", "点了 "..hits) end }
end`
	if err := e.DoString(src); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	btn := findKey(e.Root(), "b")
	if btn == nil || btn.OnClick == nil {
		t.Fatal("匿名 onClick 没有绑定")
	}
	btn.OnClick()
	if got := findKey(e.Root(), "b"); got == nil || got.Text != "点了 1" {
		t.Fatalf("文本 = %v，期望 点了 1", got)
	}
}

func TestDeepNestRejected(t *testing.T) {
	e := newEngine(t, Options{OnError: func(error) {}})
	if err := e.DoString(`function view()
  local t = { type="text", text="x" }
  return t
end`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	// 正常叶子可以构树
	if got := e.Root(); got == nil || got.Text != "x" {
		t.Fatalf("叶子构树 = %v", got)
	}
	// 造一个自引用表，应该报错但进程不挂
	if err := e.DoString(`function view()
  local t = { type="text", text="y" }
  local c = {}
  for i = 1, 70 do c = { c } end
  t.children = c
  return t
end`); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	if got := e.Root(); got == nil || got.Text != "x" {
		t.Fatalf("过深嵌套应保留旧树，实际 = %v", got)
	}
}

// ---- 测试小工具 ----

func atoiOrZero(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 %s: %v", path, err)
	}
	// 确保 mtime 变化（部分文件系统精度粗）
	future := time.Now().Add(time.Duration(len(content)) * time.Millisecond)
	_ = os.Chtimes(path, future, future)
}
