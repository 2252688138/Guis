//go:build android && cgo

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package android

/*
#include <stdlib.h>
#include <android/native_activity.h>

void gg_ime_set_activity(ANativeActivity *a);
int  gg_ime_ready(void);
int  gg_ime_init(const char *cls_name);
void gg_ime_show(const char *text, int mode);
void gg_ime_hide(void);
void gg_ime_sync(const char *text);
*/
import "C"

import (
	"sync"
	"unsafe"
)

// Android 输入法（软键盘）桥。
//
// 背景：NativeActivity 没有 InputConnection，系统输入法提交的 commitText
// （中文候选上屏就是这一步）进不了 AInputQueue —— 原生输入框点上去键盘不弹，
// 就算弹了也只能收到硬键盘的 key event。这里用 Java 侧的 ImeBridge
//（隐藏的透明 EditText）承接输入法，上屏后把整段文本回调给 Go。
//
// Java 类在 APK 的 classes.dex 里（mobile/gallery/java），所以 APK 的
// AndroidManifest 需要 android:hasCode="true"。

// ImeBridgeClass APK 里的 JNI 桥类名。
//
// 注意用点号：走的是 ClassLoader.loadClass（二进制名），
// 不是 FindClass 的斜杠形式 —— 写错只会报“class not found”。
const ImeBridgeClass = "com.guis.gallery.ImeBridge"

// IME 的输入类型（与 Java 侧常量一致）。
const (
	ImeModeText      = 0
	ImeModeMultiline = 1
	ImeModePassword  = 2
)

var (
	imeMu     sync.Mutex
	imeOnText func(string)
	imeInited bool
)

// SetOnImeText 注册输入法上屏回调（参数是输入框的整段文本）。
//
// 回调来自 Java UI 线程，实现方必须自己切回 UI 线程（ui 包用 pending 变量 +
// RequestRedraw 交给渲染帧处理）。
func SetOnImeText(fn func(string)) {
	imeMu.Lock()
	imeOnText = fn
	imeMu.Unlock()
}

// ImeInit 创建输入法桥（Activity 就绪后调用；可重复调用，失败会重试）。
//
// 注意必须在 Activity attach 完成之后才能拿到 ClassLoader：NativeActivity 的
// onCreate 回调早于窗口内容就绪，第一次调用往往得到 null ClassLoader，
// 所以这里不做 once，由调用方按帧重试直到成功。
//
// 返回 false 表示桥不可用（类没打进 APK、JNI 失败等），调用方应降级为
// 「不弹键盘」，不要因此影响界面。
func ImeInit() bool {
	if imeInited {
		return true
	}
	if a := CurrentActivity(); a != 0 {
		C.gg_ime_set_activity((*C.ANativeActivity)(unsafe.Pointer(a)))
	}
	// 正常路径：APK 里的 GalleryActivity 在 onCreate 里已经把 ImeBridge 交给 native。
	// native 线程自己 loadClass 拿不到应用 dex（无情 ClassLoader），所以不做回退尝试。
	if C.gg_ime_ready() != 0 {
		imeInited = true
		Log("ime: bridge attached")
	}
	return imeInited
}

// ImeShow 弹出系统输入法，text 为输入框当前内容（供输入法联想 / 续写）。
func ImeShow(text string, mode int) {
	if !imeInited {
		return
	}
	c := C.CString(text)
	defer C.free(unsafe.Pointer(c))
	C.gg_ime_show(c, C.int(mode))
}

// ImeHide 收起系统输入法。
func ImeHide() {
	if !imeInited {
		return
	}
	C.gg_ime_hide()
}

// ImeSync 把程序改动后的文本同步进隐藏输入框（例如提交后清空）。
func ImeSync(text string) {
	if !imeInited {
		return
	}
	c := C.CString(text)
	defer C.free(unsafe.Pointer(c))
	C.gg_ime_sync(c)
}

//export ggGoImeText
func ggGoImeText(text *C.char) {
	if text == nil {
		return
	}
	s := C.GoString(text)
	imeMu.Lock()
	fn := imeOnText
	imeMu.Unlock()
	if fn != nil {
		fn(s)
	}
}
