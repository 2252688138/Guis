//go:build android && cgo

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package android

/*
#include <android/native_activity.h>
#include <android/native_window.h>
#include <android/input.h>

#include "bridge.h"
*/
import "C"

import (
	"sync"
	"time"
	"unsafe"
)

// ANativeActivity 生命周期桥接。
//
// Android 的 NativeActivity 会在 lib 加载后调用导出符号 `ANativeActivity_onCreate`。
// 我们在这里把系统回调装好（见 bridge.c），并把三件事交给 Go 侧：
//
//	窗口创建     → SetWindow（随后建立 EGL 表面与 GLES3 上下文）
//	输入队列就绪 → SetInputQueue（轮询取触摸/按键）
//	窗口销毁     → 释放 EGL 资源
//
// UI 主循环由调用方通过 OnCreate 注入（见 mobile/android/main.go），
// 因为「启动哪个界面」属于应用层的事。

var (
	createOnce sync.Once
	createMu   sync.Mutex
	// OnCreate 应用主循环入口（由应用层 main 设置）。
	OnCreate func()

	// onCreateWait 等待应用层注入 OnCreate 的最长时间（测试可改成 0 跳过等待）。
	// 见 startOnCreate 的时序说明。
	onCreateWait = 5 * time.Second
)

//export ANativeActivity_onCreate
func ANativeActivity_onCreate(act *C.ANativeActivity, savedState unsafe.Pointer, savedSize C.size_t) {
	if act == nil {
		return
	}
	SetActivity(uintptr(unsafe.Pointer(act)))
	// 装窗口 / 输入 / 销毁回调
	C.gg_install_callbacks(act)
	// 启动应用主循环（等待应用层注入入口，见 startOnCreate）
	startOnCreate()
}

// startOnCreate 启动应用主循环，必要时先等待应用层注入 OnCreate。
//
// 时序：NativeActivity 在 dlopen 之后立刻 dlsym 并调用 ANativeActivity_onCreate，
// 而 Go 侧 main()（在那里赋值 OnCreate）此时可能还没被调度。旧实现在这里直接读
// OnCreate，读到 nil 就永久放弃启动——表现为进程活着、CPU 0%、屏幕全黑、
// logcat 一条日志都没有。现在最多等 onCreateWait 再放弃。
func startOnCreate() {
	// 放到独立 goroutine 里等：ANativeActivity_onCreate 跑在系统的 Activity 启动线程上，
	// 在这里阻塞会拖慢启动（甚至触发 ANR）。
	go func() {
		deadline := time.Now().Add(onCreateWait)
		for {
			createMu.Lock()
			fn := OnCreate
			createMu.Unlock()
			if fn != nil {
				createOnce.Do(func() { go fn() })
				return
			}
			if onCreateWait <= 0 || !time.Now().Before(deadline) {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
}

//export ggGoWindowCreated
func ggGoWindowCreated(w *C.ANativeWindow) {
	if w == nil {
		return
	}
	_ = SetWindow(uintptr(unsafe.Pointer(w)))
}

//export ggGoWindowDestroyed
func ggGoWindowDestroyed() {
	_ = SetWindow(0)
}

//export ggGoInputQueueCreated
func ggGoInputQueueCreated(q *C.AInputQueue) {
	SetInputQueue(uintptr(unsafe.Pointer(q)))
}

//export ggGoInputQueueDestroyed
func ggGoInputQueueDestroyed() {
	SetInputQueue(0)
}

//export ggGoAppDestroyed
func ggGoAppDestroyed() {
	Destroy()
}
