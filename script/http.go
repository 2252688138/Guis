// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// http 命名空间：同步 / 异步请求，支持自定义 headers、任意方法、单次超时与响应体上限。
//
//	local st, body, hd = http.get(url, { headers = { Authorization = "Bearer x" } })
//	local t = json.decode(body)
//	http.async("POST", url, { body = json.encode{...} }, function(st, body, hd) ... end)
//
// 异步回调不在请求 goroutine 里跑：结果通过 Engine.post 投递回 Lua 线程，
// 由 UI 帧的 pumpLocked 在持锁状态下执行，因此脚本状态读写始终串行。

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// httpOpts 一次请求的参数。
type httpOpts struct {
	method   string
	url      string
	body     string
	ctype    string
	headers  map[string]string
	timeout  time.Duration
	maxBytes int64
}

// parseHTTPOpts 解析脚本传的 opts 表（第 idx 个参数，0 表示没有）。
func (e *Engine) parseHTTPOpts(method, rawURL string, idx int, L *lua.LState) httpOpts {
	o := httpOpts{
		method:   method,
		url:      rawURL,
		ctype:    "application/json; charset=utf-8",
		headers:  map[string]string{},
		timeout:  e.opt.HttpTimeout,
		maxBytes: e.opt.HTTPMaxBytes,
	}
	if o.maxBytes <= 0 {
		o.maxBytes = defaultHTTPMaxBytes
	}
	if idx <= 0 {
		return o
	}
	tb, ok := L.Get(idx).(*lua.LTable)
	if !ok {
		return o
	}
	tb.ForEach(func(k, v lua.LValue) {
		key := k.String()
		switch key {
		case "body", "data":
			o.body = v.String()
		case "content_type", "contentType", "ctype":
			o.ctype = v.String()
		case "timeout", "timeout_ms":
			if ms := luaToInt(v); ms > 0 {
				o.timeout = time.Duration(ms) * time.Millisecond
			}
		case "max_bytes", "maxBytes":
			if n := luaToInt(v); n > 0 {
				o.maxBytes = n
			}
		case "headers", "header":
			if ht, ok := v.(*lua.LTable); ok {
				ht.ForEach(func(hk, hv lua.LValue) {
					o.headers[hk.String()] = hv.String()
				})
			}
		default:
			o.headers[key] = v.String()
		}
	})
	return o
}

// luaToInt Lua 数值 / 数字字符串 → int64（非数值返回 0）。
func luaToInt(v lua.LValue) int64 {
	switch x := v.(type) {
	case lua.LNumber:
		return int64(x)
	case lua.LString:
		var n int64
		if _, err := fmt.Sscanf(string(x), "%d", &n); err == nil {
			return n
		}
	}
	return 0
}

// httpDo 真正发请求，返回状态码、响应体、响应头（小写键）。
func (e *Engine) httpDo(o httpOpts) (int, string, map[string]string, error) {
	if e.opt.NoNet {
		return 0, "", nil, errors.New("http: 已关闭（Options.NoNet）")
	}
	u, err := url.Parse(o.url)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return 0, "", nil, fmt.Errorf("http: 只支持 http/https，实际是 %q", o.url)
	}
	var rd io.Reader
	if o.body != "" {
		rd = strings.NewReader(o.body)
	}
	req, err := http.NewRequest(o.method, o.url, rd)
	if err != nil {
		return 0, "", nil, fmt.Errorf("http: %v", err)
	}
	if o.body != "" && o.ctype != "" {
		req.Header.Set("Content-Type", o.ctype)
	}
	for k, v := range o.headers {
		req.Header.Set(k, v)
	}
	cli := e.httpClient()
	if o.timeout > 0 && o.timeout != e.opt.HttpTimeout {
		c := *cli
		c.Timeout = o.timeout
		cli = &c
	}
	resp, err := cli.Do(req)
	if err != nil {
		return 0, "", nil, fmt.Errorf("http: %v", err)
	}
	defer resp.Body.Close()
	maxBytes := o.maxBytes
	if maxBytes <= 0 {
		maxBytes = defaultHTTPMaxBytes
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return 0, "", nil, fmt.Errorf("http: %v", err)
	}
	if int64(len(data)) > maxBytes {
		return 0, "", nil, fmt.Errorf("http: 响应体超过 %d 字节上限（可用 max_bytes 调整）", maxBytes)
	}
	hs := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			hs[strings.ToLower(k)] = v[0]
		}
	}
	return resp.StatusCode, string(data), hs, nil
}

// headerTable 响应头 → Lua 表。
func headerTable(L *lua.LState, hs map[string]string) *lua.LTable {
	t := L.NewTable()
	for k, v := range hs {
		t.RawSetString(k, lua.LString(v))
	}
	return t
}

// pushHTTPResult 把结果压栈（status, body, headers）。
func pushHTTPResult(L *lua.LState, status int, body string, hs map[string]string) int {
	L.Push(lua.LNumber(status))
	L.Push(lua.LString(body))
	L.Push(headerTable(L, hs))
	return 3
}

// luaHttpRequest http.request(method, url[, opts]) → status, body, headers。
func (e *Engine) luaHttpRequest(L *lua.LState) int {
	method := strings.ToUpper(L.CheckString(1))
	rawURL := L.CheckString(2)
	status, body, hs, err := e.httpDo(e.parseHTTPOpts(method, rawURL, 3, L))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	return pushHTTPResult(L, status, body, hs)
}

// luaHttpGet http.get(url[, opts]) → status, body, headers。
func (e *Engine) luaHttpGet(L *lua.LState) int {
	rawURL := L.CheckString(1)
	status, body, hs, err := e.httpDo(e.parseHTTPOpts(http.MethodGet, rawURL, 2, L))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	return pushHTTPResult(L, status, body, hs)
}

// luaHttpPost http.post(url[, body, opts]) → status, body, headers。
// 第二个参数是表时视为 opts；是字符串时作为请求体。
func (e *Engine) luaHttpPost(L *lua.LState) int {
	return e.luaHttpWithBody(L, http.MethodPost)
}

// luaHttpPut put / delete 与 post 同形。
func (e *Engine) luaHttpPut(L *lua.LState) int {
	return e.luaHttpWithBody(L, http.MethodPut)
}

// luaHttpDelete http.delete(url[, opts])。
func (e *Engine) luaHttpDelete(L *lua.LState) int {
	return e.luaHttpWithBody(L, http.MethodDelete)
}

// luaHttpWithBody post / put / delete 共用实现。
func (e *Engine) luaHttpWithBody(L *lua.LState, method string) int {
	rawURL := L.CheckString(1)
	optsIdx := 3
	if _, ok := L.Get(2).(*lua.LTable); ok {
		optsIdx = 2
	}
	o := e.parseHTTPOpts(method, rawURL, optsIdx, L)
	if v, ok := L.Get(2).(lua.LString); ok && string(v) != "" {
		o.body = string(v)
	}
	status, body, hs, err := e.httpDo(o)
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	return pushHTTPResult(L, status, body, hs)
}

// luaHttpAsync http.async(method, url[, opts], fn) → 请求序号。// fn(status, body, headers)；失败时回调 fn(nil, err)。
func (e *Engine) luaHttpAsync(L *lua.LState) int {
	method := strings.ToUpper(L.CheckString(1))
	rawURL := L.CheckString(2)
	var fn *lua.LFunction
	optsIdx := 0
	if f, ok := L.Get(3).(*lua.LFunction); ok {
		fn = f
	} else {
		optsIdx = 3
		if f, ok := L.Get(4).(*lua.LFunction); ok {
			fn = f
		}
	}
	if fn == nil {
		L.RaiseError("http.async: 最后一个参数必须是回调函数")
		return 0
	}
	o := e.parseHTTPOpts(method, rawURL, optsIdx, L)
	id := e.nextReqSeq()
	go func() {
		status, body, hs, err := e.httpDo(o)
		e.post(func(L *lua.LState) {
			if e.closed {
				return
			}
			if err != nil {
				if cerr := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, lua.LNil, lua.LString(err.Error())); cerr != nil {
					e.setErr(fmt.Errorf("http.async #%d 回调: %w", id, cerr))
				}
				return
			}
			if cerr := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true},
				lua.LNumber(status), lua.LString(body), headerTable(L, hs)); cerr != nil {
				e.setErr(fmt.Errorf("http.async #%d 回调: %w", id, cerr))
			}
		})
	}()
	L.Push(lua.LNumber(id))
	return 1
}

// nextReqSeq 取下一个请求序号（Lua 锁内调用）。
func (e *Engine) nextReqSeq() int {
	e.reqSeq++
	return e.reqSeq
}
