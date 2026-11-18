//go:build !js

package ui

// 系统浏览器（CDP）后端的端到端测试：真启动本机 Chrome / Chromium，
// 验证「加载页面 → 取帧 → 缩放像素比 → 鼠标点击转发 → 字符输入转发 → 覆盖 HTML」。
// 本机没有 Chromium 系浏览器时自动跳过。

import (
	"strings"
	"testing"
	"time"

	"guis/internal/geom"
	"guis/internal/webcdp"
)

const cdpTestHTML = `<html><body style="margin:0;background:#0b1220;color:#fff">
<div id="cnt" style="padding:10px">0</div>
<input id="q" style="margin:10px;width:200px">
<script>let n=0;document.body.addEventListener('click',()=>{n++;document.getElementById('cnt').textContent=String(n)});</script>
</body></html>`

func TestCDPWebViewRendersAndForwardsInput(t *testing.T) {
	if webcdp.LocateBrowser() == "" {
		t.Skip("本机没有 Chromium 系浏览器，跳过")
	}
	be, err := newCDPWebView(nil, WebViewOptions{HTML: cdpTestHTML})
	if err != nil {
		t.Fatalf("创建后端失败：%v", err)
	}
	defer be.Close()
	if got := be.Platform(); got != "system-browser-cdp" {
		t.Fatalf("Platform() = %q", got)
	}

	be.Place(geom.Rect{W: 400, H: 300}, 1)
	if !waitCDPFrame(be, 400, 300, 20*time.Second) {
		t.Fatalf("没取到第一帧（尺寸 %dx%d，期望 400x300），后端错误：%v", be.fw, be.fh, be.Err())
	}

	// 缩放：走页面 CSS zoom（取帧尺寸不变，纹理不重建）
	be.SetZoom(2)
	if got := evalWithTick(be, "document.documentElement.style.zoom", 15*time.Second); strings.TrimSpace(got) != "2" {
		t.Fatalf("缩放未生效：document.zoom = %q", got)
	}
	be.mu.Lock()
	fw, fh := be.fw, be.fh
	be.mu.Unlock()
	if fw != 400 || fh != 300 {
		t.Fatalf("缩放不该改变取帧尺寸，实际 %dx%d", fw, fh)
	}
	be.SetZoom(1)

	// 尺寸变化：视图区跟着节点尺寸走（这里是 500x200）
	be.Place(geom.Rect{W: 500, H: 200}, 1)
	if !waitCDPFrame(be, 500, 200, 15*time.Second) {
		t.Fatalf("改尺寸后帧尺寸 %dx%d，期望 500x200", be.fw, be.fh)
	}
	be.Place(geom.Rect{W: 400, H: 300}, 1)
	if !waitCDPFrame(be, 400, 300, 15*time.Second) {
		t.Fatalf("恢复尺寸后帧尺寸 %dx%d，期望 400x300", be.fw, be.fh)
	}

	// 鼠标：点击转发（页面脚本统计点击次数）
	be.PointerMove(60, 30)
	be.PointerDown()
	be.PointerUp()
	if got := evalWithTick(be, "document.getElementById('cnt').textContent", 15*time.Second); strings.TrimSpace(got) != "1" {
		t.Fatalf("点击转发失败：计数 = %q，期望 1", got)
	}

	// 字符：输入转发
	be.Exec("document.getElementById('q').focus()")
	be.Char('a')
	be.Char('b')
	if got := evalWithTick(be, "document.getElementById('q').value", 15*time.Second); got != "ab" {
		t.Fatalf("字符转发失败：输入框 = %q，期望 ab", got)
	}

	// 具名按键：回车不报错（事件真的发出去了）
	be.Key(13)
	if got := evalWithTick(be, "1+1", 10*time.Second); got != "2" {
		t.Fatalf("按键后脚本通道异常：1+1 = %q", got)
	}

	// 离线 HTML 覆盖
	be.LoadHTML(`<p id="x">offline-ok</p>`)
	if got := evalWithTick(be, "document.getElementById('x') && document.getElementById('x').textContent", 15*time.Second); got != "offline-ok" {
		t.Fatalf("LoadHTML 失败：%q", got)
	}

	// 隐藏后停止取帧，重新显示能恢复
	be.Hide()
	gen0 := be.gen
	time.Sleep(400 * time.Millisecond)
	if be.gen != gen0 {
		t.Fatalf("隐藏后仍在取帧（gen %d → %d）", gen0, be.gen)
	}
	be.Place(geom.Rect{W: 400, H: 300}, 1)
	if !waitCDPFrame(be, 400, 300, 15*time.Second) {
		t.Fatalf("重新显示后没恢复取帧")
	}
}

// waitCDPFrame 等一帧尺寸符合预期的纹理。
func waitCDPFrame(be *cdpWebView, w, h int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		be.mu.Lock()
		ok := be.pix != nil && be.fw == w && be.fh == h
		be.mu.Unlock()
		if ok {
			return true
		}
		time.Sleep(80 * time.Millisecond)
	}
	return false
}

// evalWithTick 在页面里求值（回调走 UI 线程队列，测试里手动 Tick）。
func evalWithTick(be *cdpWebView, js string, timeout time.Duration) string {
	out := make(chan string, 1)
	be.ExecAsync(js, func(res string) {
		select {
		case out <- res:
		default:
		}
	})
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		be.Tick()
		select {
		case res := <-out:
			return res
		case <-time.After(50 * time.Millisecond):
		}
	}
	return ""
}
