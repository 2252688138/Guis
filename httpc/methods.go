package httpc

import (
	"net/http"
	"strings"
)

// Send 以指定方法发送请求；body 可为 Request / *Request 或直接作为 Request.Data。
func (c *Client) Send(method, url string, body any, opts ...Option) (*Response, error) {
	return c.Do(buildRequest(method, url, body, opts))
}

// Get 发送 GET 请求。
func (c *Client) Get(url string, opts ...Option) (*Response, error) {
	return c.Do(buildRequest(http.MethodGet, url, nil, opts))
}

// Delete 发送 DELETE 请求。
func (c *Client) Delete(url string, opts ...Option) (*Response, error) {
	return c.Do(buildRequest(http.MethodDelete, url, nil, opts))
}

// Head 发送 HEAD 请求。
func (c *Client) Head(url string, opts ...Option) (*Response, error) {
	return c.Do(buildRequest(http.MethodHead, url, nil, opts))
}

// Options 发送 OPTIONS 请求。
func (c *Client) Options(url string, opts ...Option) (*Response, error) {
	return c.Do(buildRequest(http.MethodOptions, url, nil, opts))
}

// Post 发送 POST 请求。
func (c *Client) Post(url string, body any, opts ...Option) (*Response, error) {
	return c.Do(buildRequest(http.MethodPost, url, body, opts))
}

// Put 发送 PUT 请求。
func (c *Client) Put(url string, body any, opts ...Option) (*Response, error) {
	return c.Do(buildRequest(http.MethodPut, url, body, opts))
}

// Patch 发送 PATCH 请求。
func (c *Client) Patch(url string, body any, opts ...Option) (*Response, error) {
	return c.Do(buildRequest(http.MethodPatch, url, body, opts))
}

// buildRequest 组装请求描述：body 为 Request 时合并其字段。
func buildRequest(method, url string, body any, opts []Option) *Request {
	var r Request
	switch d := body.(type) {
	case Request:
		r = d
	case *Request:
		if d != nil {
			r = *d
		}
	case nil:
	default:
		r.Data = body
	}
	r.Method = strings.ToUpper(method)
	r.URL = url
	for _, o := range opts {
		if o != nil {
			o(&r)
		}
	}
	return &r
}

// 全局便捷方法（使用 Default()）。

// Get 使用默认客户端发送 GET 请求。
func Get(url string, opts ...Option) (*Response, error) { return Default().Get(url, opts...) }

// Post 使用默认客户端发送 POST 请求。
func Post(url string, body any, opts ...Option) (*Response, error) {
	return Default().Post(url, body, opts...)
}

// Put 使用默认客户端发送 PUT 请求。
func Put(url string, body any, opts ...Option) (*Response, error) {
	return Default().Put(url, body, opts...)
}

// Patch 使用默认客户端发送 PATCH 请求。
func Patch(url string, body any, opts ...Option) (*Response, error) {
	return Default().Patch(url, body, opts...)
}

// Delete 使用默认客户端发送 DELETE 请求。
func Delete(url string, opts ...Option) (*Response, error) { return Default().Delete(url, opts...) }

// Head 使用默认客户端发送 HEAD 请求。
func Head(url string, opts ...Option) (*Response, error) { return Default().Head(url, opts...) }

// Options 使用默认客户端发送 OPTIONS 请求。
func Options(url string, opts ...Option) (*Response, error) { return Default().Options(url, opts...) }

// Send 使用默认客户端发送请求。
func Send(method, url string, body any, opts ...Option) (*Response, error) {
	return Default().Send(method, url, body, opts...)
}

// Do 使用默认客户端发送请求。
func Do(r *Request) (*Response, error) { return Default().Do(r) }
