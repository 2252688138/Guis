// guis — gallery 页面拆分：网络请求页的上传下载 / 高级 / 代码面板
// 本文件属于 pages 包，与其它页面文件一起由 main.go 引入。

package pages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"guis/httpc"
	"guis/ui"
)

// ---- 上传与下载 ----

// netTransferPanel 下载（流式 + 进度 + 原子落盘）与 multipart 上传。
func netTransferPanel() *ui.Node {
	netMu.Lock()
	kb, dst, field, name, text, busy := netDownKB, netDownPath, netUpField, netUpName, netUpText, netBusy
	netMu.Unlock()

	sizes := ui.Segmented(
		[]string{"64", "512", "2048"},
		[]string{"64 KiB", "512 KiB", "2 MiB"},
		fmt.Sprint(kb),
		func(k string) {
			n := netpAtoi(k, 512)
			netMu.Lock()
			netDownKB = n
			netMu.Unlock()
			ui.RequestRedraw()
		},
	)

	downBtn := ui.Btn("下载到临时目录", ui.BtnPrimary, netpDownload)
	if busy {
		downBtn = ui.Btn("传输中…", ui.BtnGhost, nil)
	}

	openBtn := ui.Btn("打开所在目录", ui.BtnGhost, func() {
		netMu.Lock()
		p := netDownPath
		netMu.Unlock()
		if p == "" {
			Toast("先下载一个文件")
			return
		}
		netOpenDir(filepath.Dir(p))
	})

	pathLine := ui.Text("保存位置：" + netpOr(dst, "（尚未下载）")).
		St(ui.St().Text(11, 400, ui.ColTextDim).WrapOn())

	downCard := ui.Card(ui.Column(
		ui.Text("下载（ResponseStream + 临时文件 + rename）").St(ui.St().Text(13, 600, "#e8ecff")),
		ui.Row(
			ui.Text("大小").St(ui.St().Text(11, 500, ui.ColTextDim).W(52)),
			sizes, downBtn, openBtn,
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		pathLine,
		netStatusBlock(),
	).St(ui.St().GapOf(10)))

	fieldIn := netpFieldInput("netp-up-field", field, "普通字段名", func(v string) {
		netMu.Lock()
		netUpField = v
		netMu.Unlock()
	})
	nameIn := netpFieldInput("netp-up-name", name, "文件名", func(v string) {
		netMu.Lock()
		netUpName = v
		netMu.Unlock()
	})
	textIn := ui.Textarea("netp-up-text", text, "文件内容", 84, func(v string) {
		netMu.Lock()
		netUpText = v
		netMu.Unlock()
	})

	upBtn := ui.Btn("multipart 上传", ui.BtnPrimary, netpUpload)
	if busy {
		upBtn = ui.Btn("传输中…", ui.BtnGhost, nil)
	}

	upCard := ui.Card(ui.Column(
		ui.Text("上传（Multipart 管道流式 + 上传进度）").St(ui.St().Text(13, 600, "#e8ecff")),
		ui.Row(ui.Text("字段").St(ui.St().Text(11, 500, ui.ColTextDim).W(52)), fieldIn, nameIn).
			St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		textIn,
		ui.Row(upBtn, ui.Text("字段值 = 文件内容，服务端会回显收到的字段与文件信息").
			St(ui.St().Text(11, 400, ui.ColTextDim))).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
	).St(ui.St().GapOf(10)))

	cards := []*ui.Node{downCard, upCard}
	if resp := netRespBlock("服务端回显 / 传输结果"); resp != nil {
		cards = append(cards, resp)
	}
	if log := netLogBlock("传输时间线", 8); log != nil {
		cards = append(cards, log)
	}
	return ui.Column(cards...).St(ui.St().GapOf(14))
}

// netpDownload 下载演示文件到临时目录。
func netpDownload() {
	netMu.Lock()
	kb := netDownKB
	netMu.Unlock()
	dst := filepath.Join(os.TempDir(), fmt.Sprintf("guis-httpc-demo-%d.bin", time.Now().UnixNano()%1e6))
	netMu.Lock()
	netDownPath = dst
	netMu.Unlock()
	netpRun(netSpec{
		label:      fmt.Sprintf("下载 %s", netpBytes(int64(kb)*1024)),
		method:     "GET",
		path:       fmt.Sprintf("/api/download?kb=%d", kb),
		timeout:    60 * time.Second,
		progress:   true,
		downloadTo: dst,
	})
}

// netpUpload multipart 上传演示。
func netpUpload() {
	netMu.Lock()
	field, name, text := netUpField, netUpName, netUpText
	netMu.Unlock()

	mp := httpc.NewMultipart()
	mp.Field("source", "gallery")
	if field != "" {
		mp.Field(field, text)
	}
	if name == "" {
		name = "demo.txt"
	}
	mp.Files = append(mp.Files, httpc.FilePart{
		Field:    "file",
		FileName: name,
		Reader:   strings.NewReader(text),
	})
	netpRun(netSpec{
		label:     "multipart 上传",
		method:    "POST",
		path:      "/api/upload",
		timeout:   30 * time.Second,
		multipart: mp,
		progress:  true,
	})
}

// netOpenDir 用系统文件管理器打开目录（失败时提示）。
// Windows 的 explorer.exe 不在 PATH 里，必须用绝对路径（SystemRoot 下），
// 找不到时退回 `cmd /c start ""`。
func netOpenDir(dir string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if exe := netWindowsExe("explorer.exe"); exe != "" {
			cmd = exec.Command(exe, dir)
		} else {
			cmd = exec.Command("cmd", "/c", "start", "", dir)
		}
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		Toast("打开目录失败：" + err.Error())
		return
	}
	go func() { _ = cmd.Wait() }()
}

// netWindowsExe 在 SystemRoot / WINDIR 下查找系统可执行文件（PATH 里没有 explorer.exe）。
func netWindowsExe(name string) string {
	for _, root := range []string{os.Getenv("SystemRoot"), os.Getenv("WINDIR"), `C:\Windows`} {
		if root == "" {
			continue
		}
		p := filepath.Join(root, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ---- 拦截器 / Cookie / 错误分类 ----

// netCookieOnce Cookie 演示用的持久客户端（保留 CookieJar）。
var (
	netCookieOnce sync.Once
	netCookieCli  *httpc.Client
)

// netCookieClient 返回带 CookieJar 的页面级客户端。
func netCookieClient() *httpc.Client {
	netCookieOnce.Do(func() {
		netCookieCli = httpc.New(httpc.Config{
			BaseURL: netDemoServer(),
			Cookies: true,
			Timeout: 8 * time.Second,
		})
	})
	return netCookieCli
}

// netAdvancedPanel 拦截器 / Cookie / 错误分类。
func netAdvancedPanel() *ui.Node {
	netMu.Lock()
	useInt, useCookie := netUseInterceptor, netUseCookie
	busy := netBusy
	netMu.Unlock()

	intSwitch := ui.Switch("netp-int", useInt, func(on bool) {
		netMu.Lock()
		netUseInterceptor = on
		netMu.Unlock()
		netpLog("拦截器 · %s", netpOnOff(on))
		ui.RequestRedraw()
	})

	cookieSwitch := ui.Switch("netp-cookie", useCookie, func(on bool) {
		netMu.Lock()
		netUseCookie = on
		netMu.Unlock()
		ui.RequestRedraw()
	})

	echoBtn := ui.Btn("发一次带拦截器的请求", ui.BtnSecondary, func() {
		netMu.Lock()
		on := netUseInterceptor
		netMu.Unlock()
		netpRun(netSpec{
			label:       "拦截器演示",
			method:      "GET",
			path:        "/api/echo",
			params:      "from=interceptor",
			timeout:     8 * time.Second,
			interceptor: on,
			bindCheck:   true,
		})
	})
	if busy {
		echoBtn = ui.Btn("请求中…", ui.BtnGhost, nil)
	}

	cookieBtn := ui.Btn("连续两次 Cookie 请求", ui.BtnSecondary, func() {
		netMu.Lock()
		on := netUseCookie
		netMu.Unlock()
		if !on {
			Toast("先打开「保持 Cookie」开关")
			return
		}
		netpCookieDemo()
	})

	intCard := ui.Card(ui.Column(
		ui.Text("拦截器（UseRequest / UseResponse / UseError）").St(ui.St().Text(13, 600, "#e8ecff")),
		ui.Row(
			ui.Text("注入 X-Trace 请求头").St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
			intSwitch,
			echoBtn,
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Text("演示服务 /api/echo 会把收到的请求头原样回显，可在下方响应里核对 X-Trace。").
			St(ui.St().Text(11, 400, ui.ColTextDim).WrapOn()),
	).St(ui.St().GapOf(10)))

	cookieCard := ui.Card(ui.Column(
		ui.Text("Cookie（Cookies: true → 内存 CookieJar）").St(ui.St().Text(13, 600, "#e8ecff")),
		ui.Row(
			ui.Text("保持 Cookie").St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
			cookieSwitch,
			cookieBtn,
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Text("第一次请求服务端下发 guis_sid，第二次请求由 CookieJar 自动带回。").
			St(ui.St().Text(11, 400, ui.ColTextDim).WrapOn()),
	).St(ui.St().GapOf(10)))

	errCard := ui.Card(ui.Column(
		ui.Text("错误分类（统一 *httpc.Error）").St(ui.St().Text(13, 600, "#e8ecff")),
		ui.Row(
			ui.Btn("404 业务错误", ui.BtnGhost, func() {
				netpRun(netSpec{label: "404 演示", method: "GET", path: "/api/status?code=404", timeout: 8 * time.Second})
			}),
			ui.Btn("500 业务错误", ui.BtnGhost, func() {
				netpRun(netSpec{label: "500 演示", method: "GET", path: "/api/status?code=500", timeout: 8 * time.Second})
			}),
			ui.Btn("超时", ui.BtnGhost, func() {
				netpRun(netSpec{label: "超时演示", method: "GET", path: "/api/slow?ms=1500", timeout: 300 * time.Millisecond})
			}),
			ui.Btn("取消", ui.BtnGhost, func() {
				netpRun(netSpec{label: "取消演示", method: "GET", path: "/api/slow?ms=1500", timeout: 5 * time.Second})
				go func() {
					time.Sleep(200 * time.Millisecond)
					netpCancel()
				}()
			}),
			ui.Btn("坏 JSON（Bind 失败）", ui.BtnGhost, func() {
				netpRun(netSpec{label: "坏 JSON 演示", method: "GET", path: "/api/badjson", timeout: 8 * time.Second, bindCheck: true})
			}),
			ui.Btn("网络不可达", ui.BtnGhost, func() {
				netpRun(netSpec{label: "网络错误演示", method: "GET", path: "http://127.0.0.1:1/nope", timeout: 2 * time.Second})
			}),
		).St(ui.St().GapOf(8)).AlignTo(ui.AlignCenter).St(ui.St().WrapOn()),
		ui.Text("状态行会显示 Kind 与 IsTimeout / IsCancel / IsStatus / IsNetwork 判定结果；状态码错误仍带 Response，可读业务错误体。").
			St(ui.St().Text(11, 400, ui.ColTextDim).WrapOn()),
		netStatusBlock(),
	).St(ui.St().GapOf(10)))

	cards := []*ui.Node{intCard, cookieCard, errCard}
	if resp := netRespBlock("响应 / 错误详情"); resp != nil {
		cards = append(cards, resp)
	}
	if log := netLogBlock("事件时间线", 10); log != nil {
		cards = append(cards, log)
	}
	return ui.Column(cards...).St(ui.St().GapOf(14))
}

// netpCookieDemo 用同一个客户端连打两次，演示 CookieJar 自动带回。
func netpCookieDemo() {
	client := netCookieClient()
	netMu.Lock()
	netBusy = true
	netGen++
	gen := netGen
	netPct = 0
	netResp = ""
	netMeta = ""
	netLine = "Cookie 演示 · 第一次请求…"
	netMu.Unlock()
	netpLog("→ GET /api/cookie/echo（第 1 次）")
	ui.RequestRedraw()

	go func() {
		var lines []string
		for i := 1; i <= 2; i++ {
			resp, err := client.Get("/api/cookie/echo")
			if err != nil {
				netpLog("✗ 第 %d 次失败：%v", i, err)
				lines = append(lines, fmt.Sprintf("第 %d 次：%v", i, err))
			} else {
				body := strings.TrimSpace(resp.Text())
				netpLog("← 第 %d 次 · HTTP %d · %s", i, resp.Status, netpCompact(body))
				lines = append(lines, fmt.Sprintf("第 %d 次响应：\n%s", i, body))
			}
			netMu.Lock()
			netLine = fmt.Sprintf("Cookie 演示 · 已完成第 %d/2 次请求", i)
			netMu.Unlock()
			ui.RequestRedraw()
			if i == 1 {
				time.Sleep(300 * time.Millisecond)
			}
		}
		netMu.Lock()
		if gen == netGen {
			netBusy = false
			netPct = 100
			netLine = "✓ Cookie 演示完成（第二次请求应带上 guis_sid）"
			netResp = strings.Join(lines, "\n\n")
		}
		netMu.Unlock()
		ui.RequestRedraw()
	}()
}

// ---- 示例代码 ----

// netCodeSnippets 各页签对应的等价代码。
var netCodeSnippets = map[string]string{
	"basic": `// 基础请求：实例配置 + 查询参数 + 请求头 + JSON 请求体
c := httpc.New(httpc.Config{
    BaseURL: "http://127.0.0.1:PORT",
    Timeout: 8 * time.Second,
})

resp, err := c.Do(&httpc.Request{
    Method: "POST",
    URL:    "/api/echo",
    Params: httpc.Params{"page": 1, "size": 2},
    Header: http.Header{"X-Demo": {"guis"}},
    JSON:   map[string]any{"hello": "guis", "n": 1},
})
if err != nil {
    // err 是 *httpc.Error：Kind / Status / Attempts / Response
    log.Println(httpc.KindOf(err), err)
    return
}
defer resp.Close()

var out map[string]any
if err := resp.Bind(&out); err != nil {   // 按 Content-Type 解码 JSON / XML
    log.Println("decode:", httpc.KindOf(err))
}
fmt.Println(resp.Status, resp.Size, resp.Duration, resp.Attempts)`,

	"retry": `// 重试与超时：指数退避 + 抖动，Retry-After 优先，退避可被取消
c := httpc.New(httpc.Config{
    BaseURL: "http://127.0.0.1:PORT",
    Timeout: 8 * time.Second,
    Retry: httpc.RetryConfig{
        Max:        2,                       // 重试 2 次（共 3 次尝试）
        Backoff:    150 * time.Millisecond,  // 首次退避，之后翻倍
        MaxBackoff: 2 * time.Second,
    },
    Debug: func(format string, args ...any) {   // 观察重试过程
        log.Printf(format, args...)
    },
})

resp, err := c.Get("/api/flaky?fail=2")   // 前两次 500，第三次 200
fmt.Println(resp.Status, resp.Attempts)   // 200 3

// 超时：整体超时 300ms，服务端睡 1500ms → KindTimeout
_, err = c.Get("/api/slow?ms=1500", httpc.WithTimeout(300*time.Millisecond))
if httpc.IsTimeout(err) { log.Println("超时") }

// 取消：CancelToken（也可用 httpc.WithContext(ctx)）
tok := httpc.NewCancelToken()
go func() { time.Sleep(200 * time.Millisecond); tok.Cancel() }()
_, err = c.Get("/api/slow?ms=1500", httpc.WithCancelToken(tok))
if httpc.IsCancel(err) { log.Println("已取消") }`,

	"transfer": `// 下载：ResponseStream + 临时文件 + rename，进度回调节流 60ms
resp, err := c.Download("/api/download?kb=512", "D:/tmp/demo.bin",
    httpc.WithProgress(func(p httpc.Progress) {
        fmt.Printf("%s %.1f%% %d B/s\n", p.Phase, p.Percent, p.Rate)
    }),
    httpc.WithProgressInterval(60*time.Millisecond),
)

// 上传：Multipart 管道流式（大文件不进内存）
mp := httpc.NewMultipart().Field("source", "gallery")
mp.Files = append(mp.Files, httpc.FilePart{
    Field:    "file",
    FileName: "demo.txt",
    Reader:   strings.NewReader("hello multipart from guis"),
})
resp, err = c.Post("/api/upload", httpc.Request{Multipart: mp},
    httpc.WithProgress(func(p httpc.Progress) {
        if p.Phase == "upload" {
            fmt.Printf("上传 %.1f%%\n", p.Percent)
        }
    }),
)

// 也可直接传本地路径（可重放，重试安全）
c.Upload("/api/upload", []httpc.FilePart{{Field: "file", Path: "D:/tmp/a.pdf"}})`,

	"advanced": `// 拦截器：请求改头、响应统一记录、错误统一上报
c := httpc.New(httpc.Config{BaseURL: "http://127.0.0.1:PORT"})
c.UseRequest(func(req *httpc.Request) error {
    if req.Header == nil { req.Header = http.Header{} }
    req.Header.Set("X-Trace", traceID())
    return nil                    // 返回错误 = 中止请求
})
c.UseResponse(func(resp *httpc.Response, req *httpc.Request) error {
    log.Printf("%s %s → %d in %s", req.Method, req.URL, resp.Status, resp.Duration)
    return nil                    // 逆序执行
})
c.UseError(func(err error, req *httpc.Request) error {
    report(err)                   // 可包装或吞掉错误
    return err
})

// Cookie：Cookies: true 时自动维护内存 CookieJar
cc := httpc.New(httpc.Config{BaseURL: "http://127.0.0.1:PORT", Cookies: true})
cc.Get("/api/cookie/echo")        // 服务端下发 guis_sid
cc.Get("/api/cookie/echo")        // 自动带回 guis_sid

// 错误分类
resp, err := c.Get("/api/status?code=404")
switch httpc.KindOf(err) {
case httpc.KindStatus:
    if httpc.IsStatus(err, 404) {
        var biz struct{ Error string ` + "`json:\"error\"`" + ` }
        _ = resp.Bind(&biz)       // 状态码错误仍带 Response
        log.Println(biz.Error)
    }
case httpc.KindTimeout:
case httpc.KindCanceled:
case httpc.KindNetwork:
case httpc.KindDecode:
}`,
}

// netCodePanel 展示各页签的等价代码。
func netCodePanel() *ui.Node {
	netMu.Lock()
	tab := netTab
	netMu.Unlock()
	code, ok := netCodeSnippets[tab]
	if !ok {
		code = netCodeSnippets["basic"]
	}
	lines := strings.Split(code, "\n")
	nodes := make([]*ui.Node, 0, len(lines))
	for _, l := range lines {
		if l == "" {
			l = " "
		}
		nodes = append(nodes, ui.Text(l).St(ui.St().Text(11.5, 400, "#cbd5e1").Font("Consolas")))
	}

	copyBtn := ui.Btn("复制代码", ui.BtnGhost, func() {
		if ui.ClipboardSetText(code) {
			Toast("代码已复制到剪贴板")
		} else {
			Toast("复制失败，请手动选择")
		}
	})

	head := ui.Row(
		ui.Column(
			ui.Text("与上方页签对应的等价代码").St(ui.St().Text(13, 600, "#e8ecff")),
			ui.Text("演示页做的就是这段代码：把内网地址换成你自己的服务即可直接使用。").
				St(ui.St().Text(11, 400, ui.ColTextMuted)),
		).Grow(1).St(ui.St().GapOf(4)),
		copyBtn,
	).St(ui.St().GapOf(12)).AlignTo(ui.AlignCenter)

	return ui.Column(head, ui.Card(ui.Column(nodes...).St(ui.St().GapOf(2)))).St(ui.St().GapOf(14))
}

// netpOr 空值回退。
func netpOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// netpOnOff 开关文案。
func netpOnOff(on bool) string {
	if on {
		return "已开启"
	}
	return "已关闭"
}

// netpCompact 单行摘要（日志用）。
func netpCompact(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 90 {
		return string(r[:90]) + "…"
	}
	return s
}
