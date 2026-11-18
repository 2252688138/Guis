//go:build (linux || darwin) && !android && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// Linux / macOS 后端选择：真窗口嵌入（Linux/X11，性能最好）> 系统浏览器 CDP（纹理）> 降级。
//
// 可用环境变量 GUIS_WEBVIEW_BACKEND 强制指定：embed / cdp / stub。

import (
	"os/exec"
	"runtime"
	"strings"

	"guis/internal/webcdp"
)

// webViewCapable 能嵌入 / 能连上系统浏览器即为可用。
func webViewCapable() bool {
	return embedBrowserAvailable() || webcdp.LocateBrowser() != ""
}

// newWebViewBackend 按平台能力选后端。
func newWebViewBackend(a *App, opts WebViewOptions) (webViewBackend, error) {
	pref := backendPref()
	if pref != "cdp" && pref != "stub" {
		if be, ok := newEmbeddedBrowserBackend(a, opts); ok {
			return be, nil
		} else if pref == "embed" {
			return nil, errNoWebView
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

// openExternalURL 用系统默认浏览器打开地址（xdg-open / open）。
func openExternalURL(url string) {
	url = strings.TrimSpace(url)
	if url == "" || url == "about:blank" {
		return
	}
	cmd := "xdg-open"
	if runtime.GOOS == "darwin" {
		cmd = "open"
	}
	_ = exec.Command(cmd, url).Start()
}
