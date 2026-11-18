//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// Windows 后端选择：WebView2（子窗口，性能最好）> 系统浏览器 CDP（纹理，可裁剪）> 降级。
//
// 可用环境变量 GUIS_WEBVIEW_BACKEND 强制指定：webview2 / cdp / stub。

import (
	"os/exec"
	"strings"

	"guis/internal/webcdp"
	"guis/internal/win32"
)

// webViewCapable 有 WebView2 运行时或本机有 Chromium 系浏览器即为可用。
func webViewCapable() bool {
	return win32.WebView2RuntimeAvailable() || webcdp.LocateBrowser() != ""
}

// newWebViewBackend 按平台能力选后端。
func newWebViewBackend(a *App, opts WebViewOptions) (webViewBackend, error) {
	pref := backendPref()
	if pref != "cdp" && pref != "stub" && win32.WebView2RuntimeAvailable() {
		if be, err := newWebView2Backend(a, opts); err == nil {
			return be, nil
		} else if pref == "webview2" {
			return nil, err
		}
	}
	if pref != "stub" {
		if be, err := newCDPWebView(a, opts); err == nil {
			return be, nil
		} else if pref == "cdp" {
			return nil, err
		}
	}
	return newStubWebView(a, opts)
}

// openExternalURL 用系统默认浏览器打开地址（降级路径）。
func openExternalURL(url string) {
	url = strings.TrimSpace(url)
	if url == "" || url == "about:blank" {
		return
	}
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
		_ = exec.Command("cmd", "/c", "start", "", url).Start()
	}
}
