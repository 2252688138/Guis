// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// Package script 让 Lua 脚本驱动 guis 界面：脚本里写 function view() 返回节点表，
// Go 侧每帧把它转成 ui.Node；事件、定时器、JSON、文件、HTTP、模块加载都在同一个 Lua VM 里跑。
//
//	script.RunFile("ui.lua", ui.Options{Title: "脚本", Width: 900, Height: 620}, script.Options{})
//
// 脚本写法：
//
//	n = 0
//	function inc() n = n + 1; ui.set("cnt", "text", tostring(n)) end
//	function view()
//	  return ui.col{ pad=16, gap=12,
//	    ui.text{ key="cnt", text=tostring(n), size=22 },
//	    ui.button{ text="+1", onClick="inc" },
//	  }
//	end
//
// 设计要点：
//   - 单 VM + 单锁：Lua 不是并发安全的，装载 / 事件 / 定时器 / 构树全部串行；
//   - 树缓存 + dirty：没有脚本状态变化时空闲帧不进 Lua（Root 直接返回上一棵树）；
//   - 沙箱：不加载 io/package/debug，去掉 dofile/loadfile/load/require/os.execute；
//     fs.* 与 http.* 分别由 Options.NoFS / NoNet 关闭，路径限制在 WorkDir 内。
package script

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	lua "github.com/yuin/gopher-lua"

	"guis/ui"
	"guis/ui/uidsl"
)

// Options 引擎配置。零值即可用（默认 10 秒 HTTP 超时、200ms 热重载轮询）。
type Options struct {
	// WorkDir fs.* 的根目录，也是相对路径的基准（默认脚本所在目录）。
	WorkDir string
	// NoFS 关闭 fs.*（脚本调用直接报错）。
	NoFS bool
	// NoNet 关闭 http.*（脚本调用直接报错）。
	NoNet bool
	// HttpTimeout 单次 HTTP 请求超时（默认 10s）。
	HttpTimeout time.Duration
	// Interval 热重载轮询间隔（默认 200ms，负数关闭）。
	Interval time.Duration
	// OnError 脚本错误出口（默认打 stderr）。语法错误、运行时错误、构树错误都会走这里。
	OnError func(error)
	// Logger 脚本 log.info/warn/error 的出口（默认打 stderr）。
	Logger func(level, msg string)
	// OnQuit 脚本调用 ui.quit() 时的处理（默认 os.Exit(0)）。宿主想优雅退出时覆盖它。
	OnQuit func()
	// OnTitle 脚本调用 ui.title(s) 时的处理（ui 包的改标题只有 App.SetTitle，需要宿主转一下）。
	OnTitle func(string)
	// ExecTimeout 单次脚本执行（装载 / 构树 / 事件 / 定时器回调）的时间上限，
	// 超时通过 Lua 的 ctx 中断，脚本里的 while true 不会把界面锁死。默认 5 秒，负数不限。
	ExecTimeout time.Duration
	// OnExit 应用退出前的收尾钩子（先跑脚本里的 on_exit()，再调它）。
	OnExit func()
	// Args 传给脚本 arg 表的参数（arg[0] 自动为脚本绝对路径）。
	Args []string

	// FSMaxReadBytes fs.read 的单次读取上限（默认 8MB，<=0 用默认）。
	FSMaxReadBytes int64
	// HTTPMaxBytes http.* 响应体上限（默认 8MB，<=0 用默认）。
	HTTPMaxBytes int64
	// ModulePath require 的额外搜索目录（在 WorkDir 之外时由宿主显式开放）。
	ModulePath []string
	// OnViewport3D 3D 视口渲染源提供者：脚本写 ui.viewport{ key=... } 时按 key 取渲染源，
	// 返回 nil 表示该 key 不支持（界面降级成提示文本）。
	OnViewport3D func(key string) *ui.Viewport3DOpts
}

// DefaultExecTimeout 单次脚本执行的默认时间上限。
const DefaultExecTimeout = 5 * time.Second

// Engine 一个 Lua 引擎实例（并发安全的对外壳，内部串行）。
type Engine struct {
	mu  sync.Mutex
	L   *lua.LState
	opt Options

	root  atomic.Pointer[ui.Node] // 最近一次构好的树（空闲帧直接返回）
	dirty atomic.Bool             // 脚本状态是否变化，下次 Root 需要重新构树

	baseGlobals map[string]bool           // 初始化时的全局名，重载时据此清理用户符号
	handlers    map[string]*lua.LFunction // onClick 直接写函数时的匿名名 → 函数
	handlerSeq  int
	overrides   map[string]map[string]any // ui.set 的覆盖：key → prop → 值
	timers      map[int]*timerEntry       // 定时器
	timerSeq    int
	path        string // 当前脚本文件（热重载用）
	lastErr     error
	closed      bool
	stopWatch   chan struct{}

	qmu      sync.Mutex    // 事件队列锁（UI 线程与定时器都往里塞，不碰 Lua 锁）
	queue    []queuedEvent // 锁被占时排队的事件
	httpOnce sync.Once
	httpCli  *http.Client

	modules map[string]lua.LValue // require 已加载的模块（热重载时清空）
	loadedT *lua.LTable           // package.loaded 镜像
	modStack []string             // require 调用栈（当前模块路径，用于 ./ 相对解析）
	reqSeq  int                   // http.async 请求序号
}

// queuedEvent 一个排队等锁的事件。
type queuedEvent struct {
	name string
	args []any
	fn   func(*lua.LState) // 非 nil：任意投递任务（http 异步回调等）
}

const (
	// maxQueued 事件队列上限：脚本卡住时旧事件先丢，避免无限堆积。
	maxQueued = 64
	// maxPump 每次持锁最多补跑多少个排队事件。
	maxPump = 16
)

// timerEntry 一个定时器。
type timerEntry struct {
	fn   *lua.LFunction
	once bool
	stop chan struct{}
}

// New 创建引擎（已注入 ui/timer/log/fs/http 命名空间）。
func New(opt Options) (*Engine, error) {
	if opt.HttpTimeout <= 0 {
		opt.HttpTimeout = 10 * time.Second
	}
	if opt.Interval == 0 {
		opt.Interval = 200 * time.Millisecond
	}
	if opt.ExecTimeout == 0 {
		opt.ExecTimeout = DefaultExecTimeout
	}
	if opt.OnError == nil {
		opt.OnError = func(err error) { fmt.Fprintf(os.Stderr, "[lua] %v\n", err) }
	}
	if opt.Logger == nil {
		opt.Logger = func(level, msg string) { fmt.Fprintf(os.Stderr, "[lua] %s %s\n", level, msg) }
	}
	if opt.OnQuit == nil {
		opt.OnQuit = func() { os.Exit(0) }
	}
	e := &Engine{
		opt:       opt,
		handlers:  map[string]*lua.LFunction{},
		overrides: map[string]map[string]any{},
		 timers:    map[int]*timerEntry{},
		modules:   map[string]lua.LValue{},
		stopWatch: make(chan struct{}),
	}
	e.L = lua.NewState(lua.Options{SkipOpenLibs: true})
	e.openLibs()
	e.bindUI()
	e.bindTimer()
	e.bindSys()
	e.bindJSON()
	e.bindModule()
	if opt.OnViewport3D != nil {
		uidsl.RegisterViewport3DProvider(opt.OnViewport3D)
	}
	// 描记完所有注入后的全局，重载脚本时据此只清用户符号（ui/timer/fs/http 必须保留）。
	e.baseGlobals = e.snapshotGlobals()
	return e, nil
}

// openLibs 只开安全的基础库，并摘掉危险函数。
func (e *Engine) openLibs() {
	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
		{lua.CoroutineLibName, lua.OpenCoroutine},
		{lua.OsLibName, lua.OpenOs},
	} {
		e.L.Push(e.L.NewFunction(lib.fn))
		e.L.Push(lua.LString(lib.name))
		e.L.Call(1, 0)
	}
	// 未加载 io / package / debug：没有 io.open、require、loadlib、debug。
	for _, name := range []string{"dofile", "loadfile", "load", "loadstring", "require"} {
		e.L.SetGlobal(name, lua.LNil)
	}
	if osv, ok := e.L.GetGlobal("os").(*lua.LTable); ok {
		for _, name := range []string{"execute", "exit", "remove", "rename", "getenv", "setenv", "tmpname"} {
			osv.RawSetString(name, lua.LNil)
		}
	}
}

// snapshotGlobals 记下当前所有全局名（重载脚本时据此只清用户的符号）。
func (e *Engine) snapshotGlobals() map[string]bool {
	set := map[string]bool{}
	e.L.G.Global.ForEach(func(k, _ lua.LValue) {
		if s, ok := k.(lua.LString); ok {
			set[string(s)] = true
		}
	})
	return set
}

// resetScriptState 热重载前清掉上一版脚本的运行期状态：
// 全局符号、匿名回调、定时器、ui.set 覆盖全部丢弃，否则旧闭包会继续跑。
func (e *Engine) resetScriptState() {
	e.resetGlobals()
	for id, t := range e.timers {
		close(t.stop)
		delete(e.timers, id)
	}
	e.overrides = map[string]map[string]any{}
	e.modules = map[string]lua.LValue{}
	e.modStack = nil
	if e.loadedT != nil {
		e.loadedT = e.L.NewTable()
		e.L.SetGlobal("package", func() lua.LValue {
			pkg := e.L.NewTable()
			pkg.RawSetString("loaded", e.loadedT)
			pkg.RawSetString("path", lua.LString(e.modulePathString()))
			return pkg
		}())
	}
	uidsl.ResetExtraStates()
}

// resetGlobals 清掉脚本自己定义的全局（保留标准库与注入的命名空间）。
func (e *Engine) resetGlobals() {
	// 旧版脚本里的匿名 onClick 函数引用一并丢掉，避免重载后越积越多。
	e.handlers = map[string]*lua.LFunction{}
	e.handlerSeq = 0
	var drop []string
	e.L.G.Global.ForEach(func(k, _ lua.LValue) {
		if s, ok := k.(lua.LString); ok && !e.baseGlobals[string(s)] {
			drop = append(drop, string(s))
		}
	})
	for _, name := range drop {
		e.L.SetGlobal(name, lua.LNil)
	}
}

// DoString 执行一段脚本源码（保留之前定义的全局与 state 表）。
func (e *Engine) DoString(src string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.doStringLocked(src, "")
}

// DoFile 执行脚本文件（同时记为热重载目标），失败返回错误并记进 LastError()。
func (e *Engine) DoFile(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		e.setErr(fmt.Errorf("读取脚本 %s: %w", path, err))
		return e.lastErr
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.opt.WorkDir == "" {
		e.opt.WorkDir = filepath.Dir(path)
	}
	if e.path != "" {
		e.resetScriptState() // 热重载：清掉上一版脚本的符号 / 定时器 / 覆盖，再执行新版
	}
	e.path = path
	return e.doStringLocked(string(src), path)
}

func (e *Engine) doStringLocked(src, path string) error {
	if e.closed {
		return errors.New("script: 引擎已关闭")
	}
	err := e.run(func() error {
		return e.L.DoString(src)
	})
	if err != nil {
		if path != "" {
			err = fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		e.setErr(err)
		return err
	}
	if _, ok := e.L.GetGlobal("view").(*lua.LFunction); !ok {
		err = fmt.Errorf("脚本未定义 view() 函数（返回界面节点表）")
		e.setErr(err)
		return err
	}
	e.lastErr = nil
	e.dirty.Store(true)
	ui.RequestRedraw()
	return nil
}

// run 执行一段会进 Lua 的代码：套超时 ctx、兜住 panic、复位 Lua 栈。
//
// 调用方必须已持锁（Lua 单线程）。ctx 只在本次调用期间挂在 LState 上，
// 超时后 gopher-lua 会在下一条指令处抛错，这里统一翻译成「执行超时」。
func (e *Engine) run(fn func() error) error {
	top := e.L.GetTop()
	ctx, cancel := context.WithCancel(context.Background())
	if e.opt.ExecTimeout > 0 {
		cancel()
		ctx, cancel = context.WithTimeout(context.Background(), e.opt.ExecTimeout)
	}
	defer cancel()
	e.L.SetContext(ctx)
	defer e.L.RemoveContext()
	err := e.safe(fn)
	e.L.SetTop(top) // 出错时清掉残留的栈，避免多帧后栈溢出
	if err != nil && ctx.Err() != nil {
		err = fmt.Errorf("脚本执行超时（超过 %s，死循环？）: %w", e.opt.ExecTimeout, err)
	}
	return err
}

// safe 执行 fn 并兜住 panic（脚本里死循环之外的崩溃不会带走进程）。
func (e *Engine) safe(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("脚本 panic: %v", r)
		}
	}()
	return fn()
}

func (e *Engine) setErr(err error) {
	if err == nil {
		return
	}
	e.lastErr = err
	if e.opt.OnError != nil {
		e.opt.OnError(err)
	}
}

// LastError 最近一次脚本错误（成功执行后清空）。
func (e *Engine) LastError() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastErr
}

// Path 当前脚本文件路径。
func (e *Engine) Path() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.path
}

// Invalidate 标记脚本状态已变化（宿主改了脚本共享的数据后调用），下次 Root 重新构树。
func (e *Engine) Invalidate() {
	e.dirty.Store(true)
	ui.RequestRedraw()
}

// Close 停掉定时器与热重载并释放 VM（幂等）。
func (e *Engine) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	close(e.stopWatch)
	for id, t := range e.timers {
		close(t.stop)
		delete(e.timers, id)
	}
	L := e.L
	e.mu.Unlock()
	if L != nil {
		L.Close()
	}
}

// logf 走 Logger 出口。
func (e *Engine) logf(level, format string, args ...any) {
	if e.opt.Logger != nil {
		e.opt.Logger(level, fmt.Sprintf(format, args...))
	}
}

// SetArgs 注入脚本可见的命令行参数表 arg（arg[0] 为脚本路径，1..n 为参数）。
// 参数表算「基础全局」，热重载时不会被清掉。
func (e *Engine) SetArgs(args []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	t := e.L.NewTable()
	for i, a := range args {
		t.RawSetInt(i, lua.LString(a)) // Lua 习惯：arg[0] 是脚本名
	}
	e.L.SetGlobal("arg", t)
	e.baseGlobals["arg"] = true
}

// shutdown 退出前跑脚本的 on_exit()（带超时），再调宿主钩子。
func (e *Engine) shutdown() {
	e.mu.Lock()
	if !e.closed {
		if fn, ok := e.L.GetGlobal("on_exit").(*lua.LFunction); ok {
			if err := e.run(func() error {
				return e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true})
			}); err != nil {
				e.setErr(fmt.Errorf("on_exit: %w", err))
			}
		}
	}
	e.mu.Unlock()
	if e.opt.OnExit != nil {
		e.opt.OnExit()
	}
}

// httpClient 脚本用的 HTTP 客户端（一个引擎一个，连接池复用）。
func (e *Engine) httpClient() *http.Client {
	e.httpOnce.Do(func() {
		e.httpCli = &http.Client{Timeout: e.opt.HttpTimeout}
	})
	return e.httpCli
}

// enqueue 把事件排进队列（Lua 锁被别人占着时）。
func (e *Engine) enqueue(name string, args []any) {
	e.qmu.Lock()
	defer e.qmu.Unlock()
	if len(e.queue) >= maxQueued {
		e.queue = e.queue[1:]
	}
	e.queue = append(e.queue, queuedEvent{name: name, args: args})
}

// post 把任意任务投递到 Lua 线程执行（异步回调结果回填用）。
func (e *Engine) post(fn func(*lua.LState)) {
	e.qmu.Lock()
	defer e.qmu.Unlock()
	if len(e.queue) >= maxQueued {
		e.queue = e.queue[1:]
	}
	e.queue = append(e.queue, queuedEvent{fn: fn})
}

// hasQueued 是否还有排队事件等待执行。
func (e *Engine) hasQueued() bool {
	e.qmu.Lock()
	defer e.qmu.Unlock()
	return len(e.queue) > 0
}

// pumpLocked 补跑排队事件（调用方已持 Lua 锁）。
func (e *Engine) pumpLocked() {
	for i := 0; i < maxPump; i++ {
		e.qmu.Lock()
		if len(e.queue) == 0 {
			e.qmu.Unlock()
			return
		}
		ev := e.queue[0]
		e.queue = e.queue[1:]
		e.qmu.Unlock()
		if ev.fn != nil {
			if err := e.run(func() error { ev.fn(e.L); return nil }); err != nil {
				e.setErr(err)
			}
			e.dirty.Store(true)
			ui.RequestRedraw()
			continue
		}
		e.deliverLocked(ev.name, ev.args)
	}
}
