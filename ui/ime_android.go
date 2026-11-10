//go:build android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"sync/atomic"

	"guis/internal/android"
)

// Android 软键盘（IME）接线。
//
// 组件库的输入框是自绘的，系统看不到真正的 EditText，因此输入法要绕一圈：
//
//	焦点进入输入框 → internal/android 的 Java 桥弹键盘（隐藏的透明 EditText 承接）
//	输入法上屏     → Java 回调整段文本（UI 线程）→ 存进 imePending
//	下一帧         → 按「影子文本 → 新文本」的差异注入焦点控件（End + 退格 + 逐字插入）
//
// 为什么用差异注入而不是直接替换：输入框的值在组件闭包里，帧外既读不到也写不进；
// 而 OnChar / OnKey 是唯一稳定的写入通道（桌面键盘、wasm 也都是走它们）。
//
// 影子文本由 Input / Textarea 渲染时写入（inputShadow / inputKind）。

var (
	imePending  atomic.Value // string：等待注入的上屏文本
	imeFocusKey string       // 已弹键盘的输入框 Key（"" = 未弹）
	imeBase     string       // 与该输入框最后一次同步的文本
	imeBridgeOK bool
	imeRetry    int
)

func init() {
	// 输入法上屏回调（Java UI 线程）→ 交给渲染帧处理，避免跨线程改界面状态
	android.SetOnImeText(func(text string) {
		imePending.Store(text)
		RequestRedraw()
	})
}

// syncIME 每帧调用：处理「焦点切换 → 弹/收键盘」与上屏文本注入。
func (a *App) syncIME() {
	if !imeBridgeOK {
		// 桥要等 Activity attach 完成才能拿到 ClassLoader：前几帧多半失败，
		// 每隔 10 帧重试一次（约 0.2s），直到成功。
		if imeRetry%10 == 0 {
			imeBridgeOK = android.ImeInit()
		}
		imeRetry++
		if !imeBridgeOK {
			return
		}
	}

	// 1) 上屏文本注入（只注入给仍然持有焦点的那个输入框）
	if v := imePending.Swap(nil); v != nil {
		if s, ok := v.(string); ok && imeFocusKey != "" && a.focusKey == imeFocusKey {
			a.applyIMEText(imeFocusKey, s)
		}
	}

	// 2) 焦点切换 → 弹 / 收键盘
	key, mode, text, ok := a.imeTarget()
	if !ok {
		if imeFocusKey != "" {
			android.ImeHide()
			imeFocusKey, imeBase = "", ""
		}
		return
	}
	if key != imeFocusKey {
		if imeFocusKey != "" {
			android.ImeHide()
		}
		imeFocusKey, imeBase = key, text
		android.ImeShow(text, mode)
		android.Log("ime: show key=" + key)
		return
	}
	// 3) 文本被程序改动（提交后清空 / 选择器回填）→ 同步给隐藏输入框
	if text != imeBase {
		imeBase = text
		android.ImeSync(text)
	}
}

// imeTarget 返回当前焦点对应的输入控件：Key / 键盘类型 / 当前文本。
func (a *App) imeTarget() (key string, mode int, text string, ok bool) {
	key = a.focusKey
	if key == "" {
		return "", 0, "", false
	}
	text, isInput := inputShadow[key]
	if !isInput {
		return "", 0, "", false
	}
	return key, inputKind[key], text, true
}

// applyIMEText 把输入法上屏的整段文本按差异注入焦点控件。
//
// 做法：先把光标移到末尾（所有输入框都支持 End），再删掉与新版本文本不一致的尾部，
// 最后逐字插入新增内容。手机输入法几乎都是「末尾追加 / 末尾退格」，
// 这个策略对中文上屏（拼音串整体替换为汉字）同样成立。
func (a *App) applyIMEText(key, next string) {
	n := a.focusedNode()
	if n == nil || n.Key != key || (n.OnChar == nil && n.OnKey == nil) {
		return
	}
	cur := imeBase
	if v, ok := inputShadow[key]; ok {
		cur = v
	}
	cr, nr := []rune(cur), []rune(next)
	p := 0
	for p < len(cr) && p < len(nr) && cr[p] == nr[p] {
		p++
	}
	if n.OnKey != nil {
		n.OnKey(0x23) // VK_END：光标移到末尾（单行 = 末尾，多行 = 行尾）
	}
	if n.OnChar != nil {
		for i := 0; i < len(cr)-p; i++ {
			n.OnChar(8) // Backspace
		}
		for _, r := range nr[p:] {
			n.OnChar(r)
		}
	}
	imeBase = next
	a.dirty = true
	android.Log("ime: text=" + next)
}
