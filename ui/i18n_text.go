// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "sync/atomic"

// 界面文案翻译钩子：零调用点改造的多语言支持。
//
// 业务把「中文字面量 → 译文」查表函数注册进来（见 cmd/gallery/pages/locale.go 的
// 键值式做法，或 cmd/deepseek/pages/locale.go 的整串查表做法），框架在**创建节点**
// 时把每条界面文案过一遍钩子，因此 Text / Button / Chip / Tabs / Table / 菜单 /
// 提示 / Toast 等全部组件都自动获得多语言，不需要逐个改调用点：
//
//	ui.SetTextTranslator(func(s string) string { return myDict[s] })  // 启动时一次
//	ui.Text("设置")            // 中文界面 → "设置"，英文界面 → "Settings"
//	ui.TextRaw(session.Title)  // 用户 / 模型产生的自然内容，显式跳过翻译
//	ui.NoTranslate(func() {    // 构造自然内容（Markdown、输入框正文…）时整段跳过
//		ui.Text(msg)
//	})
//
// 设计取舍：翻译发生在「节点创建」而不是「绘制」，这样测量、折行、缓存哈希、
// 选区复制全部天然一致；切换语言后界面每帧重建，文案自动跟着变（框架还会
// 丢弃一次栅格缓存，见 SetTextTranslator）。

// textTranslator 当前翻译钩子（nil 表示不翻译）。
var textTranslator atomic.Value // func(string) string

// textTrDepth 「不翻译」作用域嵌套深度（>0 时 Text 透传原串）。
var textTrDepth atomic.Int32

// textTrGen 钩子代数：变化时丢弃栅格缓存，避免旧语言的纹理被复用。
var textTrGen atomic.Uint64

// SetTextTranslator 注册界面文案翻译钩子（传 nil 取消）。
//
// 钩子返回空串表示「这条不翻译」，框架会回退到原串。钩子必须廉价（纯查表），
// 它在每次界面构建时对每条文案调用一次；不要在里面做 IO、正则或格式化。
func SetTextTranslator(fn func(string) string) {
	if fn == nil {
		textTranslator = atomic.Value{}
	} else {
		textTranslator.Store(fn)
	}
	textTrGen.Add(1)
	// 组件栅格缓存里存的是旧语言的已渲染文本，换代后必须重建。
	DropCachedLayers()
	RequestRedraw()
}

// TextTranslator 当前翻译钩子（未注册返回 nil）。
func TextTranslator() func(string) string {
	if v, ok := textTranslator.Load().(func(string) string); ok {
		return v
	}
	return nil
}

// TextGeneration 文案翻译代数：钩子每变一次 +1，业务可据此让缓存失效。
func TextGeneration() uint64 { return textTrGen.Load() }

// Tr 应用翻译钩子：空串 / 无钩子 / 「不翻译」作用域内 / 钩子无译文时原样返回。
//
// 业务自己拼装的非节点文案（窗口标题、托盘提示、通知）也可以调它。
func Tr(s string) string {
	if s == "" || textTrDepth.Load() > 0 {
		return s
	}
	fn := TextTranslator()
	if fn == nil {
		return s
	}
	if out := fn(s); out != "" && out != s {
		return out
	}
	return s
}

// NoTranslate 在 fn 内构造的节点不做文案翻译（自然内容：AI 回复、用户输入、代码）。
//
// 用法：把整段自然内容的构造包起来即可，嵌套安全：
//
//	ui.NoTranslate(func() { out = ui.Markdown(key, src) })
func NoTranslate(fn func()) {
	if fn == nil {
		return
	}
	textTrDepth.Add(1)
	defer textTrDepth.Add(-1)
	fn()
}

// NoTranslateScope 打开一个「不翻译」作用域，返回恢复函数（适合 defer）：
//
//	defer ui.NoTranslateScope()()
func NoTranslateScope() func() {
	textTrDepth.Add(1)
	return func() { textTrDepth.Add(-1) }
}

// TextRaw 不过翻译钩子的文本节点（用户 / 模型产生的自然内容用它，避免被误翻）。
func TextRaw(s string) *Node {
	return &Node{Kind: KText, Text: s, Style: Style{Opacity: 1}}
}
