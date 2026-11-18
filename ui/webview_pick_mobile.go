//go:build android || ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 移动端后端：Android WebView / WKWebView 需要 cgo 或平台通道，当前统一走降级后端
// （占位卡片 + OpenExternal 交给系统浏览器）。

// webViewCapable 移动端暂不支持内嵌浏览器。
func webViewCapable() bool { return false }

func newWebViewBackend(a *App, opts WebViewOptions) (webViewBackend, error) {
	return newStubWebView(a, opts)
}

// openExternalURL 移动端暂不主动拉起浏览器（由业务侧自行调用平台 API）。
func openExternalURL(url string) {}
