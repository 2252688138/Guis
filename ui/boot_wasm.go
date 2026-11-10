//go:build js && wasm

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"syscall/js"

	"guis/internal/text"
)

// 浏览器端启动自举：先把字体取回来再建窗口。
//
// 为什么要放在 ui 包里：浏览器没有系统字体，必须 fetch 同目录的 font.ttf 后注册，
// 这一步是**平台细节**。下沉到这里之后，业务代码（cmd/gallery/main.go）
// 在所有平台都只有一个 main()、一份 opts，不需要为 Web 单独写启动文件。
//
// 约定：页面同目录放 font.ttf（必需）+ font-bold.ttf（可选粗体）。

// 启动阶段（写到 window.__guisStage，供页面加载动画与冒烟测试读取）
const (
	stageStart = "start"
	stageFont  = "font"
	stagePanic = "panic"
)

func setStage(s string) {
	if g := js.Global(); !g.IsUndefined() {
		g.Set("__guisStage", s)
	}
}

// prepareRun 在任何平台上都在 newApp 之前调用；Web 端在这里加载字体。
func prepareRun(opts *Options) {
	setStage(stageStart)

	// 页面可通过 window.__guisAddFont 在运行期注入字体（粗体延后 / 缺字分片）
	installWebFontHook()

	// 深链接直达需要全量字体的页面（如 ?page=api）时，页面会在启动前先等
	// font-full.ttf（web/site.html）。这里发现它已到货就直接当主字体，
	// 免得首帧用子集排版后马上又因注入全量字体而重排一次。
	if buf := prefetchedFont("font-full.ttf"); buf != nil {
		text.RegisterFont("GUISWeb", buf)
	} else if !registerFont("font.ttf", "GUISWeb", false, 4000) {
		showPageError("字体加载失败：请把 font.ttf 放到 index.html 同目录")
		setStage("nofont")
		return
	}
	// 粗体不再阻塞启动：页面预取完成后（首帧之后）用 __guisAddFont 注入，
	// 这期间文字用常规字重渲染；若预取已就绪则直接注册，省一次往返。
	if buf := prefetchedFont("font-bold.ttf"); buf != nil {
		text.RegisterFont("GUISWeb/Bold", buf)
	}

	// 浏览器端只能用注册进来的字体
	opts.FontFamily = "GUISWeb"
	setStage(stageFont)
}

// prefetchedFont 立即读取页面并行预取的字体数据（没有则返回 nil，不等待、不发请求）。
func prefetchedFont(name string) []byte {
	g := js.Global().Get("__guisFonts")
	if g.IsUndefined() || g.IsNull() {
		return nil
	}
	v := g.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return nil
	}
	u8 := js.Global().Get("Uint8Array").New(v)
	if u8.Length() == 0 {
		return nil
	}
	buf := make([]byte, u8.Length())
	js.CopyBytesToGo(buf, u8)
	return buf
}

// registerFont 优先使用页面与 main.wasm 并行预取好的字体数据，拿不到再退回 fetch。
//
// 页面约定（web/site.html 已实现）：把 fetch 结果写进 window.__guisFonts[文件名]（Uint8Array）。
// 这样字体和 wasm 同时下载，首屏不用等 wasm 编译完才发起字体请求；
// waitMs 是“等预取完成”的上限，超时就直接 fetch（此时浏览器缓存多半已命中）。
func registerFont(name, family string, bold bool, waitMs int) bool {
	deadline := waitMs / 20
	for i := 0; i < deadline; i++ {
		g := js.Global().Get("__guisFonts")
		if !g.IsUndefined() && !g.IsNull() {
			if v := g.Get(name); !v.IsUndefined() && !v.IsNull() {
				u8 := js.Global().Get("Uint8Array").New(v)
				buf := make([]byte, u8.Length())
				js.CopyBytesToGo(buf, u8)
				if len(buf) > 0 {
					if bold {
						text.RegisterFont(family+"/Bold", buf)
					} else {
						text.RegisterFont(family, buf)
					}
					return true
				}
			}
		}
		sleepJS(20)
	}
	return fetchFont(name, family, bold)
}

// sleepJS 让出 20ms 并等定时器回调，期间 Go 运行时会把控制权交回浏览器事件循环。
func sleepJS(ms int) {
	done := make(chan bool, 1)
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		done <- true
		return nil
	})
	js.Global().Call("setTimeout", cb, ms)
	<-done
	cb.Release()
}

// fetchFont 同步等待 fetch 完成（阻塞在 channel 上，Go 的 wasm 运行时会把
// 控制权交回事件循环，因此回调能正常执行）。
func fetchFont(url, name string, bold bool) bool {
	done := make(chan bool, 1)

	onBody := js.FuncOf(func(this js.Value, args []js.Value) any {
		u8 := js.Global().Get("Uint8Array").New(args[0])
		buf := make([]byte, u8.Length())
		js.CopyBytesToGo(buf, u8)
		if bold {
			text.RegisterFont(name+"/Bold", buf)
		} else {
			text.RegisterFont(name, buf)
		}
		done <- true
		return nil
	})
	onResp := js.FuncOf(func(this js.Value, args []js.Value) any {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			done <- false
			return nil
		}
		resp.Call("arrayBuffer").Call("then", onBody)
		return nil
	})
	onErr := js.FuncOf(func(this js.Value, args []js.Value) any {
		done <- false
		return nil
	})

	js.Global().Call("fetch", url).Call("then", onResp).Call("catch", onErr)
	return <-done
}

// showPageError 把错误写到页面上的 #err 浮层（同时打 console）。
// 同时写到 window.__guisPanic，页面自身脚本覆盖 #err 时仍能查到原始信息。
func showPageError(msg string) {
	doc := js.Global().Get("document")
	js.Global().Set("__guisPanic", msg)
	if el := doc.Call("getElementById", "err"); !el.IsNull() && !el.IsUndefined() {
		el.Set("textContent", msg)
		el.Get("style").Set("display", "block")
	}
	js.Global().Get("console").Call("error", msg)
}

// entryFn 应用入口（由 SetEntry 在 init() 阶段注册）。
var entryFn func() (Options, func() *Node)

// SetEntry 注册应用入口：返回窗口选项与整页构建函数。
//
// 与桌面版同签名：浏览器同样需要先注册再进 Run（字体在 prepareRun 里自举）。
func SetEntry(fn func() (Options, func() *Node)) { entryFn = fn }

// Main 三端统一的应用入口（浏览器版）。
//
// 字体已在 prepareRun 里 fetch + 注册，这里与桌面走同一个 Run。
func Main() {
	fn := entryFn
	if fn == nil {
		showPageError("未注册入口：请在 init() 里调用 ui.SetEntry")
		return
	}
	opts, view := fn()
	if err := Run(opts, view); err != nil {
		showPageError("启动失败：" + err.Error())
	}
}

// reportPanic Web 端把 panic 显示到页面上，避免只剩一片黑屏。
func reportPanic(msg string) {
	setStage(stagePanic)
	showPageError(msg)
}
