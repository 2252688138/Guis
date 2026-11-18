package httpc

import (
	"fmt"
	"net/http"
)

// RequestInterceptor 请求拦截器：在请求发出前调用，可修改 req；返回错误则中止请求。
type RequestInterceptor func(req *Request) error

// ResponseInterceptor 响应拦截器：收到响应后调用（逆序），返回错误则中断并作为最终错误。
type ResponseInterceptor func(resp *Response, req *Request) error

// ErrorInterceptor 错误拦截器：任意阶段出错时调用，可包装或吞掉错误。
type ErrorInterceptor func(err error, req *Request) error

// Interceptors 拦截器集合。
type Interceptors struct {
	// Request 请求拦截器，按注册顺序执行。
	Request []RequestInterceptor
	// Response 响应拦截器，按注册逆序执行（与 axios 一致）。
	Response []ResponseInterceptor
	// Error 错误拦截器。
	Error []ErrorInterceptor
}

// merge 追加另一组拦截器（不影响原对象）。
func (i Interceptors) merge(other Interceptors) Interceptors {
	out := Interceptors{
		Request:  append(append([]RequestInterceptor(nil), i.Request...), other.Request...),
		Response: append(append([]ResponseInterceptor(nil), i.Response...), other.Response...),
		Error:    append(append([]ErrorInterceptor(nil), i.Error...), other.Error...),
	}
	return out
}

// clone 深拷贝。
func (i Interceptors) clone() Interceptors { return Interceptors{}.merge(i) }

// UseRequest 注册请求拦截器，返回自身便于链式调用。
func (c *Client) UseRequest(fn RequestInterceptor) *Client {
	if fn == nil {
		return c
	}
	c.mu.Lock()
	c.cfg.Interceptors.Request = append(c.cfg.Interceptors.Request, fn)
	c.mu.Unlock()
	return c
}

// UseResponse 注册响应拦截器（逆序执行），返回自身便于链式调用。
func (c *Client) UseResponse(fn ResponseInterceptor) *Client {
	if fn == nil {
		return c
	}
	c.mu.Lock()
	c.cfg.Interceptors.Response = append(c.cfg.Interceptors.Response, fn)
	c.mu.Unlock()
	return c
}

// UseError 注册错误拦截器，返回自身便于链式调用。
func (c *Client) UseError(fn ErrorInterceptor) *Client {
	if fn == nil {
		return c
	}
	c.mu.Lock()
	c.cfg.Interceptors.Error = append(c.cfg.Interceptors.Error, fn)
	c.mu.Unlock()
	return c
}

// Use 一次性注册（nil 项忽略）。
func (c *Client) Use(req RequestInterceptor, resp ResponseInterceptor) *Client {
	c.UseRequest(req)
	c.UseResponse(resp)
	return c
}

// runRequestInterceptors 顺序执行请求拦截器（panic 会被捕获并转为错误）。
func runRequestInterceptors(list []RequestInterceptor, req *Request) error {
	for _, fn := range list {
		if err := safeCallRequest(fn, req); err != nil {
			return err
		}
	}
	return nil
}

// safeCallRequest 调用单个请求拦截器并捕获 panic。
func safeCallRequest(fn RequestInterceptor, req *Request) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("请求拦截器 panic: %v", r)
		}
	}()
	return fn(req)
}

// runResponseInterceptors 逆序执行响应拦截器。
func runResponseInterceptors(list []ResponseInterceptor, resp *Response, req *Request) error {
	for i := len(list) - 1; i >= 0; i-- {
		if err := list[i](resp, req); err != nil {
			return err
		}
	}
	return nil
}

// runErrorInterceptors 执行错误拦截器。
func runErrorInterceptors(list []ErrorInterceptor, err error, req *Request) error {
	for _, fn := range list {
		if out := fn(err, req); out != nil {
			err = out
		}
	}
	return err
}

// authHeader 构造 Authorization 头值。
func authHeader(kind, token string) string {
	if kind == "" {
		kind = "Bearer"
	}
	return kind + " " + token
}

// cookieHeader 把 Cookie 拼成请求头。
func cookieHeader(cookies []*http.Cookie) string {
	if len(cookies) == 0 {
		return ""
	}
	req := &http.Request{Header: http.Header{}}
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}
	return req.Header.Get("Cookie")
}
