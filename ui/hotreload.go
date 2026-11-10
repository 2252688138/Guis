//go:build !js && !android && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 热重载（Hot Reload）——业务侧 API 与子进程侧实现。
//
// 目标：`go run ./cmd/gallery` 跑起来的窗口，在改完 .go 源码后自动重编译并
// 重新出图，不用手动 Ctrl+C 再跑一遍。
//
// Go 不支持在同一进程内替换已编译的代码（plugin 在 Windows 上不可用，且无法卸载），
// 所以这里采用与 air / realize 同构、但**由框架自身托管**的做法：
//
//	父进程（go run 出来的临时 exe）：守护进程，不建窗口。
//	  监听源码 → go build → 停旧子进程 → 起新子进程（见 hotreload_super.go）。
//	子进程（守护进程用新编译出的 exe 启动，GUIS_HOT_CHILD=1）：真正的应用。
//	  退出前把窗口几何 + 业务状态写盘，下一个子进程启动时恢复。
//
// 视觉上等价于「同一扇窗口的内容被刷新」，代价是窗口会重建一次（通常 < 1 秒）。
//
// 触发条件（hotReloadEnabled）：
//   - 可执行文件位于 go 的临时构建目录（路径含 go-build），即 go run 的产物；
//   - 或显式设置 GUIS_HOT=1 强制开启；
//   - 设置 GUIS_HOT=0 可关闭（打包后的正式 exe 本来就不会命中）。
//
// 业务侧可选接入状态保留（例如当前页面、滚动位置）：
//
//	func init() {
//	    ui.SetHotState(
//	        func() []byte { return []byte(currentPage) },
//	        func(b []byte) { currentPage = string(b) },
//	    )
//	}
//
// 未接入时只保留窗口位置 / 尺寸 / 最大化状态。

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	// hotEnvChild 子进程标记：设置后表示当前进程由热重载守护进程启动，不再派生孙进程。
	hotEnvChild = "GUIS_HOT_CHILD"
	// hotEnvState 子进程读写现场（窗口几何 + 业务状态）的文件路径。
	hotEnvState = "GUIS_HOT_STATE"
	// hotEnvMode 手动开关：GUIS_HOT=0 关闭，GUIS_HOT=1 强制开启。
	hotEnvMode = "GUIS_HOT"
)

// hotSaved 一次热重载重启需要跨进程携带的现场。
type hotSaved struct {
	X, Y, W, H int
	Max        bool
	// State 业务通过 SetHotState 序列化的状态（可为空）。
	State []byte
	// ok 是否真的读到了上次现场（首次启动为 false）。
	ok bool
}

// hotRestored 子进程启动时读到的现场（在 Run 之前载入，newApp 里应用）。
var hotRestored hotSaved

// hotStop 子进程收到「保存并退出」后置位，runLoop 据此干净退出。
var hotStop atomic.Bool

// hotStopRequested runLoop 每轮检查：热重载要求退出时为 true。
func hotStopRequested() bool { return hotStop.Load() }

var (
	hotStateMu      sync.Mutex
	hotStateSave    func() []byte
	hotStateRestore func([]byte)
)

// SetHotState 注册热重载现场保存 / 恢复回调（可选）。
//
// save 在重启前、旧进程退出前调用，返回业务自己的序列化结果；
// restore 在新进程建好窗口后调用一次，入参即上次 save 的返回值。
// 两者都可以传 nil。非热重载运行时这两个回调永远不会被调用。
//
// 注意：save 在后台 goroutine 里执行（收到守护进程指令时），涉及 UI 状态时
// 请自行加锁，或用 ui.Post 投递后尽快返回。
func SetHotState(save func() []byte, restore func([]byte)) {
	hotStateMu.Lock()
	hotStateSave, hotStateRestore = save, restore
	hotStateMu.Unlock()
}

// HotReloadActive 当前进程是否运行在热重载模式下（由守护进程启动的子进程）。
func HotReloadActive() bool { return os.Getenv(hotEnvChild) != "" }

// hotReloadEnterRun Run 启动时调用。
//
// 返回 true 表示本次 Run 已被热重载守护进程接管（父进程只负责监听与编译，
// 不创建窗口），err 为守护进程的退出原因。
func hotReloadEnterRun() (bool, error) {
	if os.Getenv(hotEnvChild) != "" {
		hotStartChild()
		return false, nil
	}
	if !hotReloadEnabled() {
		return false, nil
	}
	return true, hotReloadSupervise()
}

// hotReloadEnabled 是否需要以守护进程身份启动。
func hotReloadEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(hotEnvMode))) {
	case "0", "off", "false", "no":
		return false
	case "1", "on", "true", "yes":
		return true
	}
	return hotIsGoRun()
}

// hotIsGoRun 可执行文件是否由 go run 编译（位于 go 的临时构建目录）。
func hotIsGoRun() bool {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return false
	}
	p := strings.ToLower(strings.ReplaceAll(exe, "\\", "/"))
	return strings.Contains(p, "/go-build")
}

// ---------------------------------------------------------------- 子进程侧

// hotStartChild 子进程初始化：读上次现场、启动控制通道读取。
func hotStartChild() {
	if p := os.Getenv(hotEnvState); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			var s hotSaved
			if json.Unmarshal(b, &s) == nil && s.W > 0 && s.H > 0 {
				s.ok = true
				hotRestored = s
			}
		}
	}
	go hotReadControl()
	go hotHandleInterrupt()
}

// hotHandleInterrupt 子进程侧 Ctrl+C：与守护进程一致，先保存现场再退出。
//
// 不做任何处理时，控制台 Ctrl+C 会同时送到守护进程与子进程，子进程会被
// 默认处理直接杀掉（表现为 exit status 0xc000013a、窗口几何与业务状态丢失）。
func hotHandleInterrupt() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	hotSaveStateNow()
	hotStop.Store(true)
}

// hotReadControl 从 stdin 读取守护进程指令（父进程通过管道下发）。
//
// 协议：一行一条指令，save = 保存现场后退出，quit = 直接退出。
// 保存后向 stdout 回一行 saved，父进程据此判断可以开始重新编译。
func hotReadControl() {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		switch strings.TrimSpace(sc.Text()) {
		case "save":
			hotSaveStateNow()
			fmt.Fprintln(os.Stdout, "saved")
			hotStop.Store(true)
			return
		case "quit":
			hotStop.Store(true)
			return
		}
	}
	// 控制管道关闭 = 守护进程已退出（Ctrl+C / 被强杀 / 控制台关闭）：跟着退出，
	// 否则留下一个没有父进程、关不掉的孤儿窗口。
	hotStop.Store(true)
}

// hotSaveStateNow 把窗口几何 + 业务状态写入现场文件（原子替换）。
func hotSaveStateNow() {
	var s hotSaved
	if a := currentApp.Load(); a != nil && a.win != nil {
		if b, ok := a.win.(hotBoundsBackend); ok {
			s.X, s.Y, s.W, s.H, s.Max = b.Bounds()
		}
	}
	hotStateMu.Lock()
	save := hotStateSave
	hotStateMu.Unlock()
	if save != nil {
		func() {
			defer func() { _ = recover() }() // 业务回调 panic 不能连累重启
			s.State = save()
		}()
	}
	if s.W <= 0 || s.H <= 0 {
		return // 窗口还没建出来（或被销毁）：不覆盖上一次的现场
	}
	path := os.Getenv(hotEnvState)
	if path == "" {
		return
	}
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// hotBoundsBackend 可选后端能力：读取 / 设置窗口在屏幕上的位置与尺寸。
// 目前 Windows 后端实现（见 backend_windows.go），其余平台缺失时只丢失几何信息。
type hotBoundsBackend interface {
	Bounds() (x, y, w, h int, maximized bool)
	SetBounds(x, y, w, h int)
}

// hotApplyRestore 新窗口建好后恢复上次现场（非热重载时为空操作）。
func hotApplyRestore(a *App) {
	if a == nil || a.win == nil || !hotRestored.ok {
		return
	}
	if b, ok := a.win.(hotBoundsBackend); ok && hotRestored.W > 0 && hotRestored.H > 0 {
		b.SetBounds(hotRestored.X, hotRestored.Y, hotRestored.W, hotRestored.H)
	}
	if hotRestored.Max && !a.win.IsMaximized() {
		// 此时窗口还没显示，只能先记下来：首帧呈现后再最大化，
		// 否则 ShowWindow(SW_MAXIMIZE) 会提前把窗口亮出来（启动白闪）。
		hotPendingMax = true
	}
	hotStateMu.Lock()
	restore := hotStateRestore
	hotStateMu.Unlock()
	if restore != nil && len(hotRestored.State) > 0 {
		func() {
			defer func() { _ = recover() }()
			restore(hotRestored.State)
		}()
	}
	a.dirty = true
}

// hotPendingMax 恢复现场时窗口尚未显示，最大化留到首帧之后再做。
var hotPendingMax bool

// hotAfterReveal 首帧呈现、窗口显示之后调用：补上最大化状态。
func hotAfterReveal(a *App) {
	if a == nil || !hotPendingMax {
		return
	}
	hotPendingMax = false
	if a.win != nil && !a.win.IsMaximized() {
		a.win.ToggleMaximize()
	}
	a.dirty = true
}
