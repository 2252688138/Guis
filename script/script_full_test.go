// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 完善版能力测试：缺陷修复（A）、json/http/fs（B）、事件（C）、组件（D）、
// 系统能力与 require（E）。

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"

	"guis/ui"
	"guis/ui/uidsl"
)

// ---------- A. 缺陷修复 ----------

// A1：ui.set 必须置脏（否则新调用路径下改了属性不刷新）。
func TestSetMarksDirty(t *testing.T) {
	e := newEngine(t, Options{})
	if err := e.DoString(`function view() return ui.text{ key = "t", text = "a" } end`); err != nil {
		t.Fatal(err)
	}
	_ = e.Root()
	e.mu.Lock()
	e.dirty.Store(false)
	L := e.L
	L.Push(lua.LString("t"))
	L.Push(lua.LString("text"))
	L.Push(lua.LString("b"))
	e.luaSet(L)
	e.mu.Unlock()
	if !e.dirty.Load() {
		t.Fatal("ui.set 之后 dirty 未置位")
	}
}

// A2：文本属性写数字 / 布尔不报错，按 tostring 处理。
func TestNumericTextCoerced(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`function view() return ui.text{ text = 42 } end`, "42"},
		{`function view() return ui.text{ 42 } end`, "42"},
		{`function view() return ui.text(42) end`, "42"},
		{`function view() return ui.button{ text = 1.5 } end`, "1.5"},
		{`function view() return ui.text{ text = true } end`, "true"},
	}
	for _, c := range cases {
		e := newEngine(t, Options{})
		if err := e.DoString(c.src); err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		root := e.Root()
		if err := e.LastError(); err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got := firstText(root); got != c.want {
			t.Fatalf("%s: text=%q want %q", c.src, got, c.want)
		}
	}

	// ui.set 同样做 coerc
	e := newEngine(t, Options{})
	if err := e.DoString(`function view() return ui.text{ key = "t", text = "a" } end`); err != nil {
		t.Fatal(err)
	}
	_ = e.Root()
	if err := e.DoString(`ui.set("t", "text", 42)`); err != nil {
		t.Fatal(err)
	}
	if got := firstText(e.Root()); got != "42" {
		t.Fatalf("ui.set 数字文本未 coerc，text=%q", got)
	}
}

// A3：事件白名单与实现一一对应（events.go 是唯一来源）。
func TestEventWhitelistWired(t *testing.T) {
	e := newEngine(t, Options{})
	if err := e.DoString(`function h() end
function view() return ui.text{ text = "ok" } end`); err != nil {
		t.Fatal(err)
	}
	for prop := range eventSpecs {
		n := &uidsl.Node{Type: "text", Text: "x"}
		switch prop {
		case "onClick":
			n.OnClick = "h"
		case "onChange":
			n.OnChange = "h"
		case "onSubmit":
			n.OnSubmit = "h"
		case "onKey":
			n.OnKey = "h"
		case "onKeyUp":
			n.OnKeyUp = "h"
		case "onChar":
			n.OnChar = "h"
		case "onHover":
			n.OnHover = "h"
		case "onScroll":
			n.OnScroll = "h"
		case "onDrag":
			n.OnDrag = "h"
		case "onDrop":
			n.OnDrop = "h"
		case "onPress":
			n.OnPress = "h"
		case "onActivate":
			n.OnActivate = "h"
		case "onClickAt":
			n.OnClickAt = "h"
		}
		h := e.collectHandlers(n)
		if !handlerBound(h, prop) {
			t.Fatalf("事件 %s 在 eventSpecs 里有，但 collectHandlers 没有收集", prop)
		}
	}
}

// A4：未知类型必须报错，不再静默降级成提示文本。
func TestUnknownTypeRejected(t *testing.T) {
	e := newEngine(t, Options{})
	if err := e.DoString(`function view() return ui.col{ { type = "nope" } } end`); err != nil {
		t.Fatal(err)
	}
	out := e.Root()
	_ = out
	err := e.LastError()
	if err == nil || !strings.Contains(err.Error(), "不受支持") {
		t.Fatalf("未知类型未报错，LastError=%v", err)
	}
}

// ---------- B. json / http / fs ----------

// B1：json 往返。
func TestJSONRoundTrip(t *testing.T) {
	e := newEngine(t, Options{})
	src := `
local src = { ok = true, n = 3, s = "hi", list = { 1, 2, 3 }, nested = { a = { b = 2 } } }
local s = json.encode(src)
local back = json.decode(s)
assert(back.ok == true, "bool 丢失")
assert(back.n == 3, "数字丢失: " .. tostring(back.n))
assert(back.s == "hi", "字符串丢失")
assert(#back.list == 3 and back.list[3] == 3, "数组丢失")
assert(back.nested.a.b == 2, "嵌套丢失")
local bad = pcall(function() return json.decode("{oops") end)
assert(bad == false, "非法 JSON 应报错")
function view() return ui.text{ text = "ok" } end`
	if err := e.DoString(src); err != nil {
		t.Fatalf("json 往返失败: %v", err)
	}
}

// B2：http 自定义 headers 与响应体上限。
func TestHTTPHeadersAndMaxBytes(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Token")
		w.Header().Set("X-Reply", "yes")
		w.Write([]byte(strings.Repeat("a", 200)))
	}))
	defer srv.Close()

	e := newEngine(t, Options{})
	_ = e.DoString(`
local url = ...
function view() return ui.text{ text = "ok" } end`)
	e.SetArgs([]string{"", srv.URL})
	if err := e.DoString(`
local st, body, hd = http.get(arg[1], { headers = { ["X-Token"] = "t-1" }, max_bytes = 10 })
assert(st == 0 or st == nil, "超过 max_bytes 应当失败")`); err == nil {
		t.Fatal("超过 max_bytes 应报错")
	}
	if gotHeader != "t-1" {
		t.Fatalf("服务端未收到自定义 header，实际 %q", gotHeader)
	}

	e2 := newEngine(t, Options{})
	e2.SetArgs([]string{"", srv.URL})
	if err := e2.DoString(`
local st, body, hd = http.get(arg[1], { headers = { ["X-Token"] = "t-2" } })
assert(st == 200, "状态码 " .. tostring(st))
assert(#body == 200, "响应体长度 " .. tostring(#body))
assert(hd["x-reply"] == "yes", "响应头丢失")
function view() return ui.text{ text = "ok" } end`); err != nil {
		t.Fatalf("http.get 失败: %v", err)
	}
}

// B3：http.async 回调在 Lua 线程执行。
func TestHTTPAsyncCallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	e := newEngine(t, Options{})
	e.SetArgs([]string{"", srv.URL})
	if err := e.DoString(`
got = "pending"
http.async("GET", arg[1], function(st, body)
  got = tostring(st) .. ":" .. body
end)
function view() return ui.text{ text = got } end`); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = e.Root() // 每帧 pump 一次投递队列
		if strings.Contains(firstText(e.root.Load()), "200:hello") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("http.async 回调未执行，当前界面文本=%q", firstText(e.root.Load()))
}

// B4：fs 扩展 + 读取上限。
func TestFSExtras(t *testing.T) {
	dir := t.TempDir()
	e := newEngine(t, Options{WorkDir: dir, FSMaxReadBytes: 16})
	if err := e.DoString(`
fs.mkdir("sub/deep")
fs.write("sub/deep/a.txt", "hello")
assert(fs.exists("sub/deep/a.txt"), "写文件失败")
fs.copy("sub/deep/a.txt", "sub/b.txt")
assert(fs.exists("sub/b.txt"), "复制失败")
fs.rename("sub/b.txt", "sub/c.txt")
assert(fs.exists("sub/c.txt") and not fs.exists("sub/b.txt"), "重命名失败")
local st = fs.stat("sub/c.txt")
assert(st.size == 5, "size 不对: " .. tostring(st.size))
assert(st.dir == false, "文件不应是目录")
assert(fs.read("sub/c.txt") == "hello", "读内容不对")
local ok = pcall(function() return fs.read("../evil.txt") end)
assert(ok == false, "越界路径应报错")
fs.write("big.txt", string.rep("x", 32))
local ok2 = pcall(function() return fs.read("big.txt") end)
assert(ok2 == false, "超过读取上限应报错")
function view() return ui.text{ text = "ok" } end`); err != nil {
		t.Fatalf("fs 扩展失败: %v", err)
	}
}

// ---------- C. 事件 ----------

// C1：新增事件能收到正确参数。
func TestKeyHoverAndDragEvents(t *testing.T) {
	e := newEngine(t, Options{})
	if err := e.DoString(`
gotKey, gotChar, gotHover, gotDrag = 0, "", false, ""
function on_key(k) gotKey = k end
function on_char(c) gotChar = c end
function on_hover(inn) gotHover = inn end
function on_drag(x, y) gotDrag = tostring(x) .. "," .. tostring(y) end
function view()
  return ui.col{
    ui.input{ key = "i", onKey = "on_key", onChar = "on_char" },
    ui.box{ key = "b", onHover = "on_hover", onDrag = "on_drag" },
  }
end`); err != nil {
		t.Fatal(err)
	}
	root := e.Root()
	if err := e.LastError(); err != nil {
		t.Fatal(err)
	}
	fireAll(t, root, "key")
	e.mu.Lock()
	k := lua.LVAsNumber(e.L.GetGlobal("gotKey"))
	e.mu.Unlock()
	if k != 13 {
		t.Fatalf("onKey 未收到键码，got %v", k)
	}
}

// C2：事件名不存在时只记错误、不崩。
func TestMissingHandlerReported(t *testing.T) {
	e := newEngine(t, Options{})
	if err := e.DoString(`function view() return ui.button{ text="x", onClick="不存在" } end`); err != nil {
		t.Fatal(err)
	}
	_ = e.Root()
	if err := e.LastError(); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("缺失事件名未报错: %v", err)
	}
}

// ---------- D. 组件扩容 ----------

// D1：新增组件都能构树。
func TestNewWidgetsBuild(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"divider", `return ui.divider{}`},
		{"card", `return ui.card{ ui.text{ text = "in" } }`},
		{"badge", `return ui.badge{ count = 3 }`},
		{"avatar", `return ui.avatar{ text = "S" }`},
		{"chip", `return ui.chip{ label = "tag" }`},
		{"spinner", `return ui.spinner{ size = 20 }`},
		{"skeleton", `return ui.skeleton{ w = 40, h = 8 }`},
		{"markdown", `return ui.markdown{ src = "# hi" }`},
		{"richtext", `return ui.richtext{ spans = { { text = "a" } } }`},
		{"masonry", `return ui.masonry{ ui.text{ text = "m" } }`},
		{"arrow", `return ui.arrow{ color = "#fff" }`},
		{"segmented", `return ui.segmented{ items = { { value = "a", label = "A" } }, active = "a" }`},
		{"select", `return ui.select{ key = "sel", items = { { value = "a", label = "A" } } }`},
		{"chart", `return ui.chart{ ctype = "bar", labels = { "a", "b" }, series = { { name = "s", values = { 1, 2 } } } }`},
		{"codeeditor", `return ui.codeeditor{ key = "ed", src = "local x = 1" }`},
		{"alert", `return ui.alert{ key = "al", open = true, title = "t", body = "b" }`},
		{"confirm", `return ui.confirm{ key = "cf", open = true, title = "t", body = "b" }`},
		{"drawer", `return ui.drawer{ key = "dr", open = true, title = "t", body = "b" }`},
	}
	for _, c := range cases {
		e := newEngine(t, Options{})
		if err := e.DoString("function view() " + c.src + " end"); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		_ = e.Root()
		if err := e.LastError(); err != nil {
			t.Fatalf("%s 构树失败: %v", c.name, err)
		}
	}
}

// D2：chart 真的生成了 Chart 数据。
func TestChartFromScript(t *testing.T) {
	e := newEngine(t, Options{})
	if err := e.DoString(`
function view()
  return ui.chart{ ctype = "line", labels = { "a", "b" }, series = { { name = "s1", values = { 1, 2 } } } }
end`); err != nil {
		t.Fatal(err)
	}
	root := e.Root()
	if err := e.LastError(); err != nil {
		t.Fatal(err)
	}
	var ch *ui.Chart
	var walk func(n *ui.Node)
	walk = func(n *ui.Node) {
		if n == nil {
			return
		}
		if n.Chart != nil {
			ch = n.Chart
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	if ch == nil {
		t.Fatal("chart 节点没有 Chart 数据")
	}
}

// ---------- E. 系统能力与 require ----------

// E1/E2：剪贴板与通知、焦点、滚动定位不 panic。
func TestSystemAPIs(t *testing.T) {
	e := newEngine(t, Options{})
	if err := e.DoString(`
local ok = ui.clipboard.set("你好")
local txt = ui.clipboard.get()
ui.notify("done", "标题")
ui.notifyProgress("p1", 0.5, "半")
ui.notifyClose("p1")
ui.notifyClear()
ui.focus("nope")
ui.scrollTo("nope", 10)
function view() return ui.text{ text = "ok" } end`); err != nil {
		t.Fatalf("系统能力调用失败: %v", err)
	}
}

// E4：require 只执行一次、越界报错、热重载清缓存。
func TestRequireModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "util.lua"), []byte(`
count = (count or 0) + 1
return { n = count }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "main.lua")
	if err := os.WriteFile(main, []byte(`
local a = require("util")
local b = require("util")
assert(a.n == 1, "模块应只执行一次，得到 " .. tostring(a.n))
assert(a == b, "缓存未命中")
local ok = pcall(function() return require("../evil") end)
assert(ok == false, "越界 require 应报错")
function view() return ui.text{ text = "ok" } end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := Attach(main, Options{Interval: -1})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	_ = e.Root()
	if err := e.LastError(); err != nil {
		t.Fatalf("require 失败: %v", err)
	}
}

// B5：http.post 第二个参数既可以是请求体字符串，也可以是 opts 表。
func TestHTTPPostForms(t *testing.T) {
	var gotBody, gotCT, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotCT = r.Header.Get("Content-Type")
		gotToken = r.Header.Get("X-Token")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	e := newEngine(t, Options{})
	e.SetArgs([]string{"", srv.URL})
	if err := e.DoString(`
local st1 = http.post(arg[1], '{"a":1}')
local st2 = http.post(arg[1], { body = '{"b":2}', headers = { ["X-Token"] = "tk" } })
assert(st1 == 200, "字符串请求体失败: " .. tostring(st1))
assert(st2 == 200, "opts 表请求失败: " .. tostring(st2))
function view() return ui.text{ text = "ok" } end`); err != nil {
		t.Fatalf("http.post 失败: %v", err)
	}
	if gotBody != `{"b":2}` {
		t.Fatalf("请求体不对: %q", gotBody)
	}
	if gotToken != "tk" {
		t.Fatalf("opts.headers 未生效: %q", gotToken)
	}
	if gotCT == "" {
		t.Fatal("Content-Type 未设置")
	}
}

// ---------- 辅助 ----------

func firstText(n *ui.Node) string {
	if n == nil {
		return ""
	}
	if n.Text != "" {
		return n.Text
	}
	for _, c := range n.Children {
		if s := firstText(c); s != "" {
			return s
		}
	}
	return ""
}

// handlerBound 事件名是否被收集进对应的 Handlers 表。
func handlerBound(h uidsl.Handlers, prop string) bool {
	switch prop {
	case "onClick":
		return len(h.Click) > 0
	case "onChange":
		return len(h.Change) > 0
	case "onSubmit":
		return len(h.Submit) > 0
	case "onKey":
		return len(h.Key) > 0
	case "onKeyUp":
		return len(h.KeyUp) > 0
	case "onChar":
		return len(h.Char) > 0
	case "onHover":
		return len(h.Hover) > 0
	case "onScroll":
		return len(h.Scroll) > 0
	case "onDrag":
		return len(h.Drag) > 0
	case "onDrop":
		return len(h.Drop) > 0
	case "onPress":
		return len(h.Press) > 0
	case "onActivate":
		return len(h.Activate) > 0
	case "onClickAt":
		return len(h.ClickAt) > 0
	}
	return false
}

// fireAll 在构建好的界面上找到第一个带键盘回调的节点并触发回车键。
func fireAll(t *testing.T, n *ui.Node, kind string) {
	t.Helper()
	var walk func(x *ui.Node) bool
	walk = func(x *ui.Node) bool {
		if x == nil {
			return false
		}
		if x.OnKey != nil {
			x.OnKey(uint32(13))
			return true
		}
		for _, c := range x.Children {
			if walk(c) {
				return true
			}
		}
		return false
	}
	if !walk(n) {
		t.Fatalf("界面上没有找到 %s 回调节点", kind)
	}
}
