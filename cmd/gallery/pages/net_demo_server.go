// guis — gallery 页面拆分：内建演示 API（网络请求页的离线后端）
// 本文件属于 pages 包，与其它页面文件一起由 main.go 引入。

package pages

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

// netDemo 页内演示服务：用 httptest 在进程内起一个真实的 HTTP 服务，
// 这样所有演示（重试 / 超时 / 进度 / 上传 / Cookie / 压缩）都不依赖外网。
var (
	netDemoOnce sync.Once
	netDemoBase string
	netDemoErr  string
	netFlakyMu  sync.Mutex
	netFlakyHit = map[string]int{}
	netFlakyAt  = map[string]time.Time{}
)

// netDemoServer 返回演示服务基地址（首次调用时启动）。
func netDemoServer() string {
	netDemoOnce.Do(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/echo", netEcho)
		mux.HandleFunc("/api/users", netUsers)
		mux.HandleFunc("/api/status", netStatus)
		mux.HandleFunc("/api/slow", netSlow)
		mux.HandleFunc("/api/flaky", netFlaky)
		mux.HandleFunc("/api/download", netDownload)
		mux.HandleFunc("/api/upload", netUpload)
		mux.HandleFunc("/api/cookie/echo", netCookieEcho)
		mux.HandleFunc("/api/redirect", netRedirect)
		mux.HandleFunc("/api/badjson", netBadJSON)
		mux.HandleFunc("/api/gzip", netGzip)
		mux.HandleFunc("/api/xml", netXML)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			netWriteJSON(w, http.StatusOK, map[string]any{
				"service": "guis/httpc 演示服务",
				"routes": []string{
					"GET /api/echo", "GET /api/users", "GET /api/status?code=404",
					"GET /api/slow?ms=1500", "GET /api/flaky?fail=2",
					"GET /api/download?kb=512", "POST /api/upload",
					"GET /api/cookie/echo", "GET /api/redirect?n=3",
					"GET /api/badjson", "GET /api/gzip", "GET /api/xml",
				},
			})
		})
		srv := httptest.NewServer(mux)
		netDemoBase = srv.URL
	})
	return netDemoBase
}

// netWriteJSON 统一 JSON 输出。
func netWriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// netEcho 回显请求：方法 / 路径 / 查询 / 请求头 / 请求体。
func netEcho(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	hdr := make(map[string]string, len(r.Header))
	for k := range r.Header {
		hdr[k] = r.Header.Get(k)
	}
	netWriteJSON(w, http.StatusOK, map[string]any{
		"method":  r.Method,
		"path":    r.URL.Path,
		"query":   r.URL.RawQuery,
		"header":  hdr,
		"cookie":  r.Header.Get("Cookie"),
		"bodyLen": len(body),
		"body":    string(body),
		"time":    time.Now().Format("15:04:05.000"),
	})
}

// netUsers 固定 JSON 数据（演示 Bind）。
func netUsers(w http.ResponseWriter, r *http.Request) {
	netWriteJSON(w, http.StatusOK, []map[string]any{
		{"id": 1, "name": "林清越", "age": 27, "city": "杭州", "tags": []string{"Go", "UI"}},
		{"id": 2, "name": "周牧", "age": 31, "city": "成都", "tags": []string{"Rust", "DB"}},
		{"id": 3, "name": "沈星野", "age": 24, "city": "上海", "tags": []string{"TS", "Web"}},
	})
}

// netStatus 返回指定状态码，并带业务错误码。
func netStatus(w http.ResponseWriter, r *http.Request) {
	code, _ := strconv.Atoi(r.URL.Query().Get("code"))
	if code < 100 || code > 599 {
		code = 404
	}
	w.Header().Set("X-Error-Code", fmt.Sprintf("DEMO_%d", code))
	netWriteJSON(w, code, map[string]any{
		"error":  http.StatusText(code),
		"code":   code,
		"tip":    "这是演示服务返回的业务错误体，可通过 err.Response.Bind 读取",
		"detail": fmt.Sprintf("请求 %s 被判定为 %d", r.URL.Path, code),
	})
}

// netSlow 延迟响应（演示超时）。
func netSlow(w http.ResponseWriter, r *http.Request) {
	ms, _ := strconv.Atoi(r.URL.Query().Get("ms"))
	if ms <= 0 || ms > 10000 {
		ms = 1500
	}
	select {
	case <-time.After(time.Duration(ms) * time.Millisecond):
	case <-r.Context().Done():
		return
	}
	netWriteJSON(w, http.StatusOK, map[string]any{"sleptMs": ms, "time": time.Now().Format("15:04:05.000")})
}

// netFlaky 前 N 次返回 500，之后成功（演示重试退避）。
func netFlaky(w http.ResponseWriter, r *http.Request) {
	fail, _ := strconv.Atoi(r.URL.Query().Get("fail"))
	if fail <= 0 {
		fail = 2
	}
	key := fmt.Sprintf("%s?fail=%d", r.URL.Path, fail)
	netFlakyMu.Lock()
	if t, ok := netFlakyAt[key]; !ok || time.Since(t) > 10*time.Second {
		netFlakyHit[key] = 0
	}
	netFlakyHit[key]++
	hit := netFlakyHit[key]
	netFlakyAt[key] = time.Now()
	netFlakyMu.Unlock()

	if hit <= fail {
		w.Header().Set("Retry-After", "0")
		netWriteJSON(w, http.StatusInternalServerError, map[string]any{
			"attempt": hit,
			"tip":     fmt.Sprintf("第 %d 次请求故意失败（前 %d 次都会失败）", hit, fail),
		})
		return
	}
	netWriteJSON(w, http.StatusOK, map[string]any{
		"attempt": hit,
		"tip":     fmt.Sprintf("第 %d 次请求成功（第 %d 次重试后）", hit, hit-fail),
	})
}

// netDownload 生成指定大小的响应体（演示下载进度）。
func netDownload(w http.ResponseWriter, r *http.Request) {
	kb, _ := strconv.Atoi(r.URL.Query().Get("kb"))
	if kb <= 0 || kb > 64*1024 {
		kb = 512
	}
	size := kb * 1024
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(size))
	w.Header().Set("Content-Disposition", `attachment; filename="demo.bin"`)
	buf := bytes.Repeat([]byte("guis-httpc-demo-"), 4096)
	left := size
	for left > 0 {
		n := len(buf)
		if n > left {
			n = left
		}
		if _, err := w.Write(buf[:n]); err != nil {
			return
		}
		left -= n
	}
}

// netUpload 解析分段上传，回显字段与文件信息。
func netUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		netWriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	fields := map[string]string{}
	for k, v := range r.MultipartForm.Value {
		fields[k] = strings.Join(v, ",")
	}
	type fileInfo struct {
		Field string `json:"field"`
		Name  string `json:"name"`
		Size  int    `json:"size"`
	}
	var files []fileInfo
	for field, fhs := range r.MultipartForm.File {
		for _, fh := range fhs {
			files = append(files, fileInfo{Field: field, Name: fh.Filename, Size: int(fh.Size)})
		}
	}
	netWriteJSON(w, http.StatusOK, map[string]any{
		"fields": fields,
		"files":  files,
		"tip":    "文件以管道流式写给服务端，未整体读入内存",
	})
}

// netCookieEcho 首次下发 Cookie，之后回显收到的 Cookie。
func netCookieEcho(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("guis_sid")
	if err != nil {
		http.SetCookie(w, &http.Cookie{
			Name: "guis_sid", Value: fmt.Sprintf("sid-%d", time.Now().UnixNano()%100000),
			Path: "/", HttpOnly: true,
		})
		netWriteJSON(w, http.StatusOK, map[string]any{"cookie": "", "set": true, "tip": "本次下发 Cookie，下次请求会带上它"})
		return
	}
	netWriteJSON(w, http.StatusOK, map[string]any{"cookie": c.Value, "set": false, "tip": "CookieJar 已自动带回上次下发的 Cookie"})
}

// netRedirect 链式重定向。
func netRedirect(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n > 0 {
		http.Redirect(w, r, fmt.Sprintf("/api/redirect?n=%d", n-1), http.StatusFound)
		return
	}
	netWriteJSON(w, http.StatusOK, map[string]any{"redirected": true, "tip": "重定向链已走完"})
}

// netBadJSON 返回非法 JSON（演示 KindDecode）。
func netBadJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = io.WriteString(w, `{"broken": 这里不是合法 JSON`)
}

// netGzip 返回 gzip 压缩的 JSON（演示自动解压）。
func netGzip(w http.ResponseWriter, r *http.Request) {
	payload, _ := json.MarshalIndent(map[string]any{
		"encoding": "gzip",
		"tip":      "客户端自动解压，resp.Text() 拿到的是原文",
		"filler":   strings.Repeat("guis ", 200),
		"rand":     rand.Intn(1000),
	}, "", "  ")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Encoding", "gzip")
	gz := gzip.NewWriter(w)
	_, _ = gz.Write(payload)
	_ = gz.Close()
}

// netXML 返回 XML（演示 Bind 自动识别 XML）。
func netXML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = io.WriteString(w, `<user><id>7</id><name>林清越</name><city>杭州</city></user>`)
}
