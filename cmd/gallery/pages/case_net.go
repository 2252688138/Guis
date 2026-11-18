// guis — gallery 页面拆分：网络请求（guis/httpc 完整能力演示）
// 本文件属于 pages 包，与其它页面文件一起由 main.go 引入。

package pages

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"guis/httpc"
	"guis/ui"
)

// ---- 页面状态（全部由 netMu 保护，请求在后台 goroutine 里跑）----
var (
	netMu sync.Mutex

	netTabKeys   = []string{"basic", "retry", "transfer", "advanced", "code"}
	netTabLabels = []string{"基础请求", "重试与超时", "上传与下载", "拦截器与错误", "示例代码"}
	netTab       = "basic"

	// 基础请求
	netMethod = "GET"
	netPath   = "/api/echo"
	netParams = "page=1&size=2"
	netHdrKey = "X-Demo"
	netHdrVal = "guis"
	netBody   = "{\n  \"hello\": \"guis\",\n  \"n\": 1\n}"

	// 重试 / 超时
	netRetryMax     = "2"
	netRetryBackoff = "150"
	netTimeoutMs    = "3000"
	netRetryPath    = "/api/flaky?fail=2"
	netSlowPath     = "/api/slow?ms=1500"

	// 上传下载
	netDownKB   = 512
	netDownPath = ""
	netUpField  = "note"
	netUpName   = "demo.txt"
	netUpText   = "hello multipart from guis"

	// 高级
	netUseInterceptor = true
	netUseCookie      = false
	netErrCode        = "404"

	// 运行状态
	netBusy  bool
	netLine  = "尚未发送请求"
	netMeta  string
	netResp  string
	netPct   float32
	netRate  string
	netLogs  []string
	netToken *httpc.CancelToken
	netGen   int
)

// netSpec 一次演示请求的描述。
type netSpec struct {
	label       string
	method      string
	path        string
	params      string
	header      map[string]string
	body        string
	timeout     time.Duration
	retry       httpc.RetryConfig
	cookies     bool
	interceptor bool
	progress    bool
	bindCheck   bool
	downloadTo  string
	multipart   *httpc.Multipart
}

// netCase 网络请求演示页：5 个页签覆盖 guis/httpc 的完整能力。
func netCase() *ui.Node {
	netMu.Lock()
	tab := netTab
	netMu.Unlock()

	head := ui.Row(
		ui.Column(
			ui.Text("网络请求 · guis/httpc").St(ui.St().Text(19, 700, "#e8ecff")),
			ui.Text("内建演示服务（进程内 httptest）：基础请求 / 重试与超时 / 上传下载 / 拦截器与 Cookie / 错误分类，全部离线可跑。").
				St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
		).Grow(1).St(ui.St().GapOf(4)),
		ui.Tag(netpBaseLabel(), "#7dd3fc"),
	).St(ui.St().GapOf(12)).AlignTo(ui.AlignCenter)

	tabs := ui.TabsBar(netTabKeys, netTabLabels, tab, func(k string) {
		netMu.Lock()
		netTab = k
		netMu.Unlock()
		ui.RequestRedraw()
	})

	var body *ui.Node
	switch tab {
	case "retry":
		body = netRetryPanel()
	case "transfer":
		body = netTransferPanel()
	case "advanced":
		body = netAdvancedPanel()
	case "code":
		body = netCodePanel()
	default:
		body = netBasicPanel()
	}
	return ui.Column(
		head,
		tabs,
		ui.Scroll(body).St(ui.St().GrowTo(1)).K("scroll-net-"+tab),
	).St(ui.St().GapOf(12).Padding(18))
}

// ---- 基础请求 ----

// netBasicPanel 基础请求面板：方法 / 路径 / 参数 / 头 / 请求体 / 响应。
func netBasicPanel() *ui.Node {
	netMu.Lock()
	method, path, params, hk, hv, body := netMethod, netPath, netParams, netHdrKey, netHdrVal, netBody
	busy := netBusy
	netMu.Unlock()

	methodSel := ui.Segmented(
		[]string{"GET", "POST", "PUT", "DELETE"},
		[]string{"GET", "POST", "PUT", "DELETE"},
		method,
		func(k string) {
			netMu.Lock()
			netMethod = k
			netMu.Unlock()
			ui.RequestRedraw()
		},
	)

	pathIn := netpFieldInput("netp-path", path, "/api/echo", func(v string) {
		netMu.Lock()
		netPath = v
		netMu.Unlock()
	})
	paramIn := netpFieldInput("netp-params", params, "page=1&size=2（用 & 分隔）", func(v string) {
		netMu.Lock()
		netParams = v
		netMu.Unlock()
	})
	hdrKeyIn := netpFieldInput("netp-hdr-key", hk, "请求头名", func(v string) {
		netMu.Lock()
		netHdrKey = v
		netMu.Unlock()
	})
	hdrValIn := netpFieldInput("netp-hdr-val", hv, "请求头值", func(v string) {
		netMu.Lock()
		netHdrVal = v
		netMu.Unlock()
	})

	var bodyIn *ui.Node
	if method != "GET" {
		bodyIn = ui.Textarea("netp-body", body, "请求体（按 JSON 发送）", 96, func(v string) {
			netMu.Lock()
			netBody = v
			netMu.Unlock()
		})
	}

	sendBtn := ui.Btn("发送请求", ui.BtnPrimary, func() {
		netMu.Lock()
		sp := netSpec{
			label:       "基础请求",
			method:      netMethod,
			path:        netPath,
			params:      netParams,
			body:        netBody,
			timeout:     8 * time.Second,
			header:      map[string]string{},
			interceptor: netUseInterceptor,
		}
		if netHdrKey != "" {
			sp.header[netHdrKey] = netHdrVal
		}
		netMu.Unlock()
		netpRun(sp)
	})
	if busy {
		sendBtn = ui.Btn("请求中…", ui.BtnGhost, nil)
	}

	actions := ui.Row(
		sendBtn,
		ui.Btn("取消", ui.BtnSecondary, netpCancel),
		ui.Btn("清空结果", ui.BtnGhost, func() {
			netMu.Lock()
			netResp, netMeta, netLine = "", "", "尚未发送请求"
			netPct = 0
			netMu.Unlock()
			ui.RequestRedraw()
		}),
	).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter)

	quick := ui.Row(
		ui.Text("快捷路径").St(ui.St().Text(11, 500, ui.ColTextDim)),
		netQuickBtn("/api/echo", "GET", "/api/echo"),
		netQuickBtn("/api/users", "GET", "/api/users"),
		netQuickBtn("/api/status?code=404", "GET", "/api/status?code=404"),
		netQuickBtn("/api/gzip", "GET", "/api/gzip（自动解压）"),
		netQuickBtn("/api/xml", "GET", "/api/xml（XML 绑定）"),
		netQuickBtn("/api/echo", "POST", "POST /api/echo"),
	).St(ui.St().GapOf(8)).AlignTo(ui.AlignCenter)

	reqCard := ui.Card(ui.Column(
		ui.Row(
			ui.Text("方法").St(ui.St().Text(11, 500, ui.ColTextDim).W(52)),
			methodSel,
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Row(
			ui.Text("路径").St(ui.St().Text(11, 500, ui.ColTextDim).W(52)),
			pathIn,
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Row(
			ui.Text("参数").St(ui.St().Text(11, 500, ui.ColTextDim).W(52)),
			paramIn,
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Row(
			ui.Text("请求头").St(ui.St().Text(11, 500, ui.ColTextDim).W(52)),
			hdrKeyIn, hdrValIn,
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		bodyIn,
		quick,
		actions,
		netStatusBlock(),
	).St(ui.St().GapOf(10)))

	cards := []*ui.Node{reqCard}
	if resp := netRespBlock("响应预览"); resp != nil {
		cards = append(cards, resp)
	}
	if log := netLogBlock("请求时间线", 6); log != nil {
		cards = append(cards, log)
	}
	return ui.Column(cards...).St(ui.St().GapOf(14))
}

// netQuickBtn 快捷路径按钮：同时设置方法与路径。
func netQuickBtn(path, method, label string) *ui.Node {
	return ui.Btn(label, ui.BtnGhost, func() {
		netMu.Lock()
		netPath = path
		netMethod = method
		if i := strings.IndexByte(path, '?'); i > 0 {
			netPath = path[:i]
			netParams = path[i+1:]
		}
		netMu.Unlock()
		ui.RequestRedraw()
	})
}

// ---- 重试与超时 ----

// netRetryPanel 重试 / 超时演示：flaky 接口 + 退避日志 + 超时分类。
func netRetryPanel() *ui.Node {
	netMu.Lock()
	rmax, backoff, tms, rpath, spath := netRetryMax, netRetryBackoff, netTimeoutMs, netRetryPath, netSlowPath
	busy := netBusy
	netMu.Unlock()

	retryIn := netpFieldInput("netp-retry-max", rmax, "重试次数", func(v string) {
		netMu.Lock()
		netRetryMax = v
		netMu.Unlock()
	})
	backoffIn := netpFieldInput("netp-retry-backoff", backoff, "首次退避（毫秒）", func(v string) {
		netMu.Lock()
		netRetryBackoff = v
		netMu.Unlock()
	})
	timeoutIn := netpFieldInput("netp-timeout", tms, "超时（毫秒）", func(v string) {
		netMu.Lock()
		netTimeoutMs = v
		netMu.Unlock()
	})
	pathIn := netpFieldInput("netp-retry-path", rpath, "/api/flaky?fail=2", func(v string) {
		netMu.Lock()
		netRetryPath = v
		netMu.Unlock()
	})

	flakyBtn := ui.Btn("发起不稳定请求（前 2 次 500）", ui.BtnPrimary, func() {
		netMu.Lock()
		sp := netSpec{
			label:   "重试演示",
			method:  "GET",
			path:    netRetryPath,
			timeout: netpMillis(netTimeoutMs, 8000),
			retry: httpc.RetryConfig{
				Max:        netpAtoi(netRetryMax, 2),
				Backoff:    time.Duration(netpAtoi(netRetryBackoff, 150)) * time.Millisecond,
				MaxBackoff: 2 * time.Second,
			},
			interceptor: netUseInterceptor,
		}
		netMu.Unlock()
		netpRun(sp)
	})
	if busy {
		flakyBtn = ui.Btn("请求中…", ui.BtnGhost, nil)
	}

	timeoutBtn := ui.Btn("超时演示（服务端慢 1.5s / 超时 300ms）", ui.BtnSecondary, func() {
		netMu.Lock()
		sp := netSpec{
			label:   "超时演示",
			method:  "GET",
			path:    spath,
			timeout: 300 * time.Millisecond,
		}
		netMu.Unlock()
		netpRun(sp)
	})

	cancelBtn := ui.Btn("取消演示（慢请求 + 立即取消）", ui.BtnSecondary, func() {
		netMu.Lock()
		sp := netSpec{label: "取消演示", method: "GET", path: spath, timeout: 5 * time.Second}
		netMu.Unlock()
		netpRun(sp)
		go func() {
			time.Sleep(250 * time.Millisecond)
			netpCancel()
		}()
	})

	note := ui.Text("重试策略：网络错误 / 超时 / 429 / 5xx 自动重试，指数退避 + 抖动；Retry-After 优先；退避期间可被取消。").
		St(ui.St().Text(11, 400, ui.ColTextDim).WrapOn())

	card := ui.Card(ui.Column(
		ui.Row(
			ui.Text("重试").St(ui.St().Text(11, 500, ui.ColTextDim).W(52)),
			retryIn, backoffIn, timeoutIn,
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Row(
			ui.Text("路径").St(ui.St().Text(11, 500, ui.ColTextDim).W(52)),
			pathIn,
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		note,
		ui.Row(flakyBtn, ui.Btn("重置计数", ui.BtnGhost, func() {
			netFlakyMu.Lock()
			netFlakyHit = map[string]int{}
			netFlakyAt = map[string]time.Time{}
			netFlakyMu.Unlock()
			Toast("不稳定接口计数已重置")
		})).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Row(timeoutBtn, cancelBtn).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		netStatusBlock(),
	).St(ui.St().GapOf(10)))

	cards := []*ui.Node{card}
	if log := netLogBlock("重试 / 超时时间线", 12); log != nil {
		cards = append(cards, log)
	}
	if resp := netRespBlock("最后一次响应"); resp != nil {
		cards = append(cards, resp)
	}
	return ui.Column(cards...).St(ui.St().GapOf(14))
}

// ---- 通用块 ----

// netStatusBlock 状态行 + 元信息 + 进度。
func netStatusBlock() *ui.Node {
	netMu.Lock()
	line, meta, pct, rate, busy := netLine, netMeta, netPct, netRate, netBusy
	netMu.Unlock()

	color := ui.ColTextMuted
	switch {
	case busy:
		color = "#fbbf24"
	case strings.HasPrefix(line, "✓"):
		color = "#34d399"
	case strings.HasPrefix(line, "✗"):
		color = "#f87171"
	}
	rows := []*ui.Node{
		ui.Text(line).St(ui.St().Text(12, 600, color).WrapOn()),
	}
	if meta != "" {
		rows = append(rows, ui.Text(meta).St(ui.St().Text(11, 400, ui.ColTextDim).WrapOn()))
	}
	if busy || (pct > 0 && pct < 100) {
		rows = append(rows, ui.Progress(pct/100, "#6366f1").St(ui.St().H(6)))
	}
	if rate != "" && busy {
		rows = append(rows, ui.Text(rate).St(ui.St().Text(11, 400, ui.ColTextDim)))
	}
	return ui.Column(rows...).St(ui.St().GapOf(6))
}

// netRespBlock 响应预览（等宽逐行渲染，超长截断）。
func netRespBlock(title string) *ui.Node {
	netMu.Lock()
	text := netResp
	netMu.Unlock()
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 90 {
		lines = append(lines[:90], "…（已截断）")
	}
	nodes := make([]*ui.Node, 0, len(lines)+1)
	nodes = append(nodes, ui.Text(title).St(ui.St().Text(12, 600, ui.ColTextDim)))
	for _, l := range lines {
		if l == "" {
			l = " "
		}
		nodes = append(nodes, ui.Text(l).St(ui.St().Text(11.5, 400, "#cbd5e1").Font("Consolas")))
	}
	return ui.Card(ui.Column(nodes...).St(ui.St().GapOf(2)))
}

// netLogBlock 事件日志（最新在上）。
func netLogBlock(title string, limit int) *ui.Node {
	netMu.Lock()
	logs := append([]string(nil), netLogs...)
	netMu.Unlock()
	if len(logs) == 0 {
		return nil
	}
	if limit > 0 && len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}
	nodes := make([]*ui.Node, 0, len(logs)+1)
	nodes = append(nodes, ui.Text(title).St(ui.St().Text(12, 600, ui.ColTextDim)))
	for i := len(logs) - 1; i >= 0; i-- {
		nodes = append(nodes, ui.Text(logs[i]).St(ui.St().Text(11, 400, "#94a3b8").Font("Consolas").WrapOn()))
	}
	return ui.Card(ui.Column(nodes...).St(ui.St().GapOf(2)))
}

// ---- 动作 ----

// netpRun 在后台 goroutine 执行一次演示请求，结束后回到主线程重绘。
func netpRun(sp netSpec) {
	netMu.Lock()
	if netBusy {
		netMu.Unlock()
		return
	}
	netBusy = true
	netGen++
	gen := netGen
	token := httpc.NewCancelToken()
	netToken = token
	netPct = 0
	netRate = ""
	netMeta = ""
	netResp = ""
	netLine = sp.label + " · 请求中…"
	netMu.Unlock()
	netpLog("→ %s %s%s", sp.method, sp.path, netpParamsSuffix(sp.params))
	ui.RequestRedraw()

	go func() {
		base := netDemoServer()
		client := httpc.New(httpc.Config{
			BaseURL: base,
			Timeout: sp.timeout,
			Cookies: sp.cookies,
			Retry:   sp.retry,
			Debug: func(format string, args ...any) {
				netpLog("⋯ %s", fmt.Sprintf(format, args...))
			},
		})
		if sp.interceptor {
			client.UseRequest(func(req *httpc.Request) error {
				trace := netpTrace()
				if req.Header == nil {
					req.Header = http.Header{}
				}
				req.Header.Set("X-Trace", trace)
				netpLog("拦截器 · 注入 X-Trace: %s", trace)
				return nil
			})
		}

		req := httpc.Request{Method: sp.method, URL: sp.path}
		if sp.multipart != nil {
			req.Multipart = sp.multipart
		}
		for _, o := range netpOptions(sp, token) {
			o(&req)
		}

		start := time.Now()
		var (
			resp *httpc.Response
			err  error
		)
		if sp.downloadTo != "" {
			resp, err = client.Download(req.URL, sp.downloadTo, netpOptions(sp, token)...)
		} else {
			resp, err = client.Do(&req)
		}
		cost := time.Since(start)

		netMu.Lock()
		if gen != netGen {
			netMu.Unlock()
			return
		}
		netBusy = false
		netPct = 100
		netRate = ""
		var body string
		var meta string
		switch {
		case err != nil && httpc.IsCancel(err):
			netLine = "✗ 已取消 · 用时 " + cost.Round(time.Millisecond).String()
			netpLogMsgNoLock("✗ 已取消（KindCanceled）")
		case err != nil:
			kind := httpc.KindOf(err)
			flags := netpFlags(err)
			netLine = fmt.Sprintf("✗ 失败 · %s%s · 用时 %s", kind, flags, cost.Round(time.Millisecond))
			if resp != nil {
				meta = fmt.Sprintf("HTTP %d · %s", resp.Status, netpBytes(resp.Size))
				body = resp.Text()
			}
			body = netpJoin(body, err.Error())
			netpLogMsgNoLock(fmt.Sprintf("✗ %s%s", kind, flags))
			if e := httpc.ErrorOf(err); e != nil && e.Response != nil {
				var biz struct {
					Error  string `json:"error"`
					Code   int    `json:"code"`
					Detail string `json:"detail"`
				}
				if e.Response.Bind(&biz) == nil && biz.Error != "" {
					netpLogMsgNoLock(fmt.Sprintf("业务错误体 · %s (code=%d) %s", biz.Error, biz.Code, biz.Detail))
				}
			}
		default:
			netLine = fmt.Sprintf("✓ HTTP %d · 用时 %s · 尝试 %d 次", resp.Status, cost.Round(time.Millisecond), resp.Attempts)
			meta = fmt.Sprintf("%s · 响应体 %s · Content-Type %s", resp.URL, netpBytes(resp.Size), resp.ContentType())
			if sp.downloadTo != "" {
				if fi, statErr := os.Stat(sp.downloadTo); statErr == nil {
					meta = fmt.Sprintf("已保存 %s · %s", sp.downloadTo, netpBytes(fi.Size()))
				}
			}
			body = resp.Text()
			netpLogMsgNoLock(fmt.Sprintf("✓ %d · %s", resp.Status, netpBytes(resp.Size)))
			if sp.bindCheck {
				var v map[string]any
				if berr := resp.Bind(&v); berr != nil {
					netpLogMsgNoLock("Bind 失败 · " + string(httpc.KindOf(berr)))
				} else {
					netpLogMsgNoLock(fmt.Sprintf("Bind 成功 · JSON 字段 %d 个", len(v)))
				}
			}
		}
		netLine = strings.TrimSpace(netLine)
		netMeta = meta
		netResp = netpClip(body, 6000)
		netMu.Unlock()
		ui.RequestRedraw()
	}()
}

// netpOptions 生成请求选项（参数 / 请求头 / 进度 / 取消）。
func netpOptions(sp netSpec, token *httpc.CancelToken) []httpc.Option {
	opts := []httpc.Option{httpc.WithCancelToken(token)}
	for _, kv := range strings.Split(sp.params, "&") {
		if k, v, ok := strings.Cut(kv, "="); ok && k != "" {
			opts = append(opts, httpc.WithParam(k, v))
		}
	}
	for k, v := range sp.header {
		if k != "" {
			opts = append(opts, httpc.WithHeader(k, v))
		}
	}
	if sp.body != "" && sp.method != "GET" {
		opts = append(opts, httpc.WithBody(sp.body), httpc.WithContentType("application/json; charset=utf-8"))
	}
	if sp.progress {
		opts = append(opts, httpc.WithProgress(func(p httpc.Progress) {
			netMu.Lock()
			netPct = float32(p.Percent)
			netRate = fmt.Sprintf("%s / %s · %s/s", netpBytes(p.Loaded), netpBytes(p.Total), netpBytes(p.Rate))
			netMu.Unlock()
			ui.RequestRedraw()
		}), httpc.WithProgressInterval(60*time.Millisecond))
	}
	return opts
}

// netpCancel 取消当前请求。
func netpCancel() {
	netMu.Lock()
	tok := netToken
	netMu.Unlock()
	if tok != nil {
		tok.Cancel()
	}
}

// netpLog 追加一条带时间戳的事件日志。
func netpLog(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	netMu.Lock()
	netpLogMsgNoLock(msg)
	netMu.Unlock()
}

// netpLogMsgNoLock 追加日志（调用方已持有 netMu）。
func netpLogMsgNoLock(msg string) {
	netLogs = append(netLogs, time.Now().Format("15:04:05.000")+"  "+msg)
	if len(netLogs) > 60 {
		netLogs = netLogs[len(netLogs)-60:]
	}
}

// netpFlags 错误判定展示串。
func netpFlags(err error) string {
	var flags []string
	if httpc.IsTimeout(err) {
		flags = append(flags, "IsTimeout")
	}
	if httpc.IsCancel(err) {
		flags = append(flags, "IsCancel")
	}
	if httpc.IsStatus(err) {
		flags = append(flags, "IsStatus")
	}
	if httpc.IsNetwork(err) {
		flags = append(flags, "IsNetwork")
	}
	if len(flags) == 0 {
		return ""
	}
	return " · " + strings.Join(flags, "+")
}

// netpBaseLabel 演示服务标识。
func netpBaseLabel() string {
	base := netDemoServer()
	if base == "" {
		return "演示服务未启动"
	}
	return strings.TrimPrefix(strings.TrimPrefix(base, "http://"), "https://")
}

// netpParamsSuffix 查询串后缀（日志用）。
func netpParamsSuffix(params string) string {
	if strings.TrimSpace(params) == "" {
		return ""
	}
	return "?" + params
}

// netpTrace 生成短 trace id。
func netpTrace() string {
	return strconv.FormatInt(time.Now().UnixNano()%1e12, 36)
}

// netpAtoi 安全整数解析。
func netpAtoi(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return def
	}
	return n
}

// netpMillis 毫秒字符串转 Duration。
func netpMillis(s string, def int) time.Duration {
	return time.Duration(netpAtoi(s, def)) * time.Millisecond
}

// netpJoin 拼接非空文本。
func netpJoin(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "\n\n" + b
	}
}

// netpBytes 字节数友好格式。
func netpBytes(n int64) string {
	switch {
	case n < 0:
		return "未知大小"
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
	}
}

// netpClip 按字符数截断（不切断 UTF-8）。
func netpClip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "\n…（已截断）"
}

// netpFieldInput 表单行里的输入框：默认撑满剩余宽度（Row 内不定宽会塌成 0）。
func netpFieldInput(key, value, placeholder string, onChange func(string)) *ui.Node {
	return ui.InputH(key, value, placeholder, 34, onChange).St(ui.St().GrowTo(1))
}
