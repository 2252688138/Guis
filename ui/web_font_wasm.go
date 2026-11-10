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

// 浏览器端字体热注入：页面可以在首帧之后再把字体数据推给 Go（window.__guisAddFont），
// 用于
//   - 粗体延后加载：首屏只等正文字体，粗体到货后注入并重绘（不阻塞启动）；
//   - 缺字分片按需加载：论坛等用户可输入任意文字的场景，按需追加字形分片。
//
// 注入后必须丢弃已建好的字体实例（每份字体自带字形图集与缓存），否则旧 face 仍然生效。

// HotAddFont 注册/替换一份 Web 字体数据，并让界面按新字体重建。
func HotAddFont(name string, data []byte) {
	if name == "" || len(data) == 0 {
		return
	}
	text.RegisterFont(name, data)
	if a := currentApp.Load(); a != nil {
		a.ResetFonts()
	}
}

// ResetFonts 丢弃全部字体实例（含折行缓存与图集纹理），下一帧按新字体重建并重绘。
func (a *App) ResetFonts() {
	if a == nil {
		return
	}
	if a.fonts != nil {
		a.fonts.reset()
	}
	if mdFallbackFonts != nil {
		mdFallbackFonts.reset()
	}
	a.dirty = true
}

// installWebFontHook 暴露 window.__guisAddFont(name, Uint8Array|ArrayBuffer) 给页面脚本。
func installWebFontHook() {
	js.Global().Set("__guisAddFont", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 2 || args[0].IsUndefined() || args[1].IsUndefined() {
			return false
		}
		u8 := js.Global().Get("Uint8Array").New(args[1])
		if u8.Length() == 0 {
			return false
		}
		buf := make([]byte, u8.Length())
		js.CopyBytesToGo(buf, u8)
		HotAddFont(args[0].String(), buf)
		return true
	}))
	// 页面可主动清空字体缓存（例如切换字体后）
	js.Global().Set("__guisResetFonts", js.FuncOf(func(this js.Value, args []js.Value) any {
		if a := currentApp.Load(); a != nil {
			a.ResetFonts()
		}
		return true
	}))
}
