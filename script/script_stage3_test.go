// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 第三阶段：动画 / 终端 / 视频 / 内嵌浏览器 / 菜单栏 / 3D 视口 / 模块语义。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guis/ui"
	"guis/ui/uidsl"
)

// 节点动画：脚本里写 anim，构树无错。
func TestNodeAnimFromScript(t *testing.T) {
	uidsl.ResetExtraStates()
	e := newEngine(t, Options{})
	if err := e.DoString(`
function view()
  return ui.card{ anim = { { kind = "pop", dur = 0.2 }, { kind = "breathe", loop = true } },
    ui.text{ text = "x" },
  }
end`); err != nil {
		t.Fatal(err)
	}
	if e.Root() == nil || e.LastError() != nil {
		t.Fatalf("动画构树失败: %v", e.LastError())
	}
}

// 终端：构出 KTerm 节点，内容能更新且复用同一状态。
func TestTermFromScript(t *testing.T) {
	uidsl.ResetExtraStates()
	e := newEngine(t, Options{})
	if err := e.DoString(`
function view()
  return ui.term{ key = "t", lines = { "line-1" } }
end`); err != nil {
		t.Fatal(err)
	}
	n := findKind(e.Root(), ui.KTerm)
	if n == nil || n.Term == nil {
		t.Fatalf("未构出 term 节点: %v", e.LastError())
	}
	first := n.Term
	if err := e.DoString(`ui.set("t", "lines", { "line-1", "line-2" })`); err != nil {
		t.Fatal(err)
	}
	n2 := findKind(e.Root(), ui.KTerm)
	if n2 == nil {
		t.Fatal("第二次构树丢了 term 节点")
	}
	if n2.Term != first {
		t.Fatal("同 key 应复用同一个 TermState")
	}
	if !strings.Contains(n2.Term.Text(), "line-2") {
		t.Fatalf("新增行未写入: %q", n2.Term.Text())
	}
}

// 视频打不开：不 panic，降级文本，错误可见。
func TestVideoDegradesFromScript(t *testing.T) {
	uidsl.ResetExtraStates()
	uidsl.DrainDegrades()
	e := newEngine(t, Options{})
	if err := e.DoString(`
function view()
  return ui.video{ key = "v", src = "不存在的文件.mp4" }
end`); err != nil {
		t.Fatal(err)
	}
	root := e.Root()
	if root == nil {
		t.Fatal("video 构树返回 nil")
	}
	if err := e.LastError(); err == nil || !strings.Contains(err.Error(), "video") {
		t.Fatalf("降级信息没进 LastError: %v", err)
	}
}

// 内嵌浏览器：无后端环境下不 panic。
func TestWebviewNoPanic(t *testing.T) {
	uidsl.ResetExtraStates()
	uidsl.DrainDegrades()
	e := newEngine(t, Options{})
	if err := e.DoString(`
function view()
  return ui.webview{ key = "w", src = "https://example.com" }
end`); err != nil {
		t.Fatal(err)
	}
	if e.Root() == nil {
		t.Fatal("webview 构树返回 nil")
	}
}

// 菜单栏：脚本声明 + 点击回调。
func TestMenuBarFromScript(t *testing.T) {
	uidsl.ResetExtraStates()
	e := newEngine(t, Options{})
	if err := e.DoString(`
picked = ""
function new() picked = "new" end
function view()
  return ui.menubar{ key = "mb", menus = {
    { label = "文件", items = {
      { label = "新建", hint = "Ctrl+N", onClick = "new" },
      { separator = true },
      { label = "退出", onClick = "new", checked = true },
    } },
  } }
end`); err != nil {
		t.Fatal(err)
	}
	if e.Root() == nil || e.LastError() != nil {
		t.Fatalf("菜单栏构树失败: %v", e.LastError())
	}
}

// 3D 视口：没注入渲染源时降级；注入后按 key 调用宿主提供者。
func TestViewportFromScript(t *testing.T) {
	uidsl.ResetExtraStates()
	defer uidsl.RegisterViewport3DProvider(nil)

	got := ""
	e := newEngine(t, Options{OnViewport3D: func(key string) *ui.Viewport3DOpts {
		got = key
		return nil
	}})
	if err := e.DoString(`function view() return ui.viewport{ key = "main" } end`); err != nil {
		t.Fatal(err)
	}
	root := e.Root()
	if root == nil {
		t.Fatal("viewport 构树返回 nil")
	}
	if got != "main" {
		t.Fatalf("宿主注入未被调用，got=%q", got)
	}
	if txt := firstText(root); !strings.Contains(txt, "3D 视口") {
		t.Fatalf("未降级为提示文本: %q", txt)
	}
}

// 模块：相对路径 require。
func TestRequireRelative(t *testing.T) {
	dir := t.TempDir()
	writeModFile(t, filepath.Join(dir, "lib", "a.lua"), `
local b = require("./b")
return { name = "a", b = b }
`)
	writeModFile(t, filepath.Join(dir, "lib", "b.lua"), `return { name = "b" }`)
	e := newEngine(t, Options{WorkDir: dir})
	if err := e.DoString(`
local a = require("lib/a")
assert(a.name == "a", "a.name 不对")
assert(a.b and a.b.name == "b", "./b 相对解析失败")
function view() return ui.text{ text = "ok" } end`); err != nil {
		t.Fatalf("相对 require 失败: %v", err)
	}
}

// 模块：ModulePath 额外搜索目录。
func TestRequireModulePath(t *testing.T) {
	work := t.TempDir()
	libs := t.TempDir()
	writeModFile(t, filepath.Join(libs, "shared.lua"), `return { v = 42 }`)
	e := newEngine(t, Options{WorkDir: work, ModulePath: []string{libs}})
	if err := e.DoString(`
local s = require("shared")
assert(s.v == 42, "ModulePath 未生效")
function view() return ui.text{ text = "ok" } end`); err != nil {
		t.Fatalf("ModulePath require 失败: %v", err)
	}
}

// 模块：package.loaded 可见。
func TestPackageLoaded(t *testing.T) {
	dir := t.TempDir()
	writeModFile(t, filepath.Join(dir, "util.lua"), `return { x = 1 }`)
	e := newEngine(t, Options{WorkDir: dir})
	if err := e.DoString(`
local u = require("util")
assert(package.loaded["util"] ~= nil, "package.loaded 未记录模块")
assert(type(package.path) == "string" and #package.path > 0, "package.path 异常")
assert(type(package.loaded["util"]) == "table", "package.loaded 内容不对")
function view() return ui.text{ text = "ok" } end`); err != nil {
		t.Fatalf("package 表异常: %v", err)
	}
}

// 模块：循环 require 不死锁，返回占位。
func TestRequireCycle(t *testing.T) {
	dir := t.TempDir()
	writeModFile(t, filepath.Join(dir, "a.lua"), `
local b = require("b")
return { name = "a", b = b }
`)
	writeModFile(t, filepath.Join(dir, "b.lua"), `
local a = require("a")
return { name = "b", a = a }
`)
	e := newEngine(t, Options{WorkDir: dir})
	if err := e.DoString(`
local a = require("a")
assert(a.name == "a", "顶层模块未返回")
assert(type(a.b.a) == "boolean", "循环引用应拿到占位 true，实际 " .. type(a.b.a))
function view() return ui.text{ text = "ok" } end`); err != nil {
		t.Fatalf("循环 require 失败: %v", err)
	}
}

// ---------- 辅助 ----------

func writeModFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findKind(n *ui.Node, k ui.Kind) *ui.Node {
	if n == nil {
		return nil
	}
	if n.Kind == k {
		return n
	}
	for _, c := range n.Children {
		if got := findKind(c, k); got != nil {
			return got
		}
	}
	return nil
}
