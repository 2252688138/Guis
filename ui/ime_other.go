//go:build !android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// syncIME 非 Android 端不需要输入法桥：
// 桌面由系统把 IME 组合串通过 EvComposition 送进来（app.go 里已有处理），
// 浏览器由 canvas 的输入事件负责。这里保持空实现，让 app.go 的调用点跨平台一致。
func (a *App) syncIME() {}
