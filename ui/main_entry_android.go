//go:build android && cgo

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"sync"
	"time"

	"guis/internal/android"
)

var (
	entryMu sync.Mutex
	entryFn func() (Options, func() *Node)
	// entryOnce 保证 UI 只启动一次（OnCreate 与 Main 都可能触发启动）。
	entryOnce sync.Once
)

// init 在 .so 被 dlopen 期间执行，早于系统的 ANativeActivity_onCreate 回调。
//
// 入口必须在这里挂上：NativeActivity 一加载完就回调，若等 main() 再赋值，
// 会错过回调（进程活着、CPU 0%、全黑屏、logcat 一条都没有）。
func init() {
	android.OnCreate = func() { go startEntry() }
}

// SetEntry 注册应用入口（应用 init() 里调用，先于 NativeActivity 回调）。
func SetEntry(fn func() (Options, func() *Node)) {
	entryMu.Lock()
	entryFn = fn
	entryMu.Unlock()
}

// Main 三端统一入口的 Android 版：UI 在 OnCreate 派生的 goroutine 里跑，
// 这里只负责常驻（-buildmode=c-shared 下 main() 的调度时机不稳定）。
func Main() {
	go startEntry()
	select {}
}

// startEntry 等应用把入口注册上来后启动一次 UI。
//
// 应用包的 init（ui.SetEntry）与本包的 init（挂 OnCreate）谁先执行不由我们决定，
// 所以这里做最长 5 秒的等待，不假设顺序。
func startEntry() {
	deadline := time.Now().Add(5 * time.Second)
	var fn func() (Options, func() *Node)
	for {
		entryMu.Lock()
		fn = entryFn
		entryMu.Unlock()
		if fn != nil {
			break
		}
		if !time.Now().Before(deadline) {
			android.Log("ui.startEntry: 未注册入口（ui.SetEntry 必须在 init 里调用）")
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	entryOnce.Do(func() {
		opts, view := fn()
		android.Log("ui.startEntry: launching")
		runAndroid(opts, view)
	})
}

// runAndroid 等原生窗口交出尺寸 → 按屏幕宽度算 UI 缩放 → 进入统一渲染循环。
//
// 与桌面共用同一个 view：布局自适应（窄屏收窄侧栏 / 隐藏菜单栏）由 view 自己
// 按 ui.ViewportW() 决定，这里不做任何界面分支。
func runAndroid(opts Options, view func() *Node) {
	w, h := waitWindow()
	android.Log(fmt.Sprintf("runAndroid: window=%dx%d ready=%v", w, h, android.Ready()))

	// Android 后端按物理像素布局（win.Scale() 恒为 1），桌面字号直接搬到手机上
	// 会小到看不清，这里按屏幕宽度反推缩放：目标逻辑宽度 480px（≈ 手机竖屏
	// 可用 dp 宽度）。720p → 1.5，1080p → 2.25，1440p 及以上 → 2.5（SetUIScale 上限）。
	if w > 0 {
		SetUIScale(float32(w) / 480)
	}
	android.Log(fmt.Sprintf("uiScale=%v viewport=%dx%d",
		UIScale(), int(float32(w)/UIScale()), int(float32(h)/UIScale())))

	err := Run(opts, view)
	if err != nil {
		android.Log("ui.Run returned error: " + err.Error())
		return
	}
	android.Log("ui.Run returned nil")
}

// waitWindow 等原生窗口交出尺寸（最多 3 秒）。
//
// 只为了把界面缩放算准：newApp 内部还会等 GL 上下文就绪，这里不重复那一步。
func waitWindow() (int32, int32) {
	deadline := time.Now().Add(3 * time.Second)
	for {
		if w, h := android.WindowSize(); w > 0 && h > 0 {
			return w, h
		}
		if !time.Now().Before(deadline) {
			return 0, 0
		}
		time.Sleep(10 * time.Millisecond)
	}
}
