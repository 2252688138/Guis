package httpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Kind 错误分类。
type Kind string

const (
	// KindNetwork 建立连接/传输层错误（DNS、拒绝连接、连接重置等）。
	KindNetwork Kind = "network"
	// KindTimeout 超时（连接、传输或整体超时）。
	KindTimeout Kind = "timeout"
	// KindCanceled 被调用方取消（context 或 CancelToken）。
	KindCanceled Kind = "canceled"
	// KindStatus 状态码未通过 ValidateStatus。
	KindStatus Kind = "status"
	// KindDecode 响应体解析失败（Bind / 解压等）。
	KindDecode Kind = "decode"
	// KindTooLarge 响应体超过 MaxContentLength。
	KindTooLarge Kind = "too_large"
	// KindConfig 请求配置非法（URL 为空、方法非法等）。
	KindConfig Kind = "config"
	// KindUnsupported 当前平台不支持该能力（如 wasm 下的代理/自定义 TLS）。
	KindUnsupported Kind = "unsupported"
)

// sensitiveHeaders 错误信息里必须脱敏的头。
var sensitiveHeaders = []string{"Authorization", "Cookie", "Set-Cookie", "Proxy-Authorization"}

// Error 是本包统一错误类型。
type Error struct {
	// Kind 错误分类。
	Kind Kind
	// Op 请求方法（GET/POST…）。
	Op string
	// URL 请求地址（不含 query 中的敏感值）。
	URL string
	// Status HTTP 状态码，非状态类错误为 0。
	Status int
	// Code 业务错误码（来自响应头 X-Error-Code）。
	Code string
	// RetryAfter 服务端建议的重试等待时间。
	RetryAfter time.Duration
	// Response 失败时的响应（含业务错误体），可能为 nil。
	Response *Response
	// Method 请求方法别名（同 Op）。
	Method string
	// Attempts 已尝试次数（含首次）。
	Attempts int
	// Err 底层错误。
	Err error
}

// Error 实现 error 接口，文本包含方法、地址、状态码与原因，自动脱敏。
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	var b strings.Builder
	b.WriteString("httpc: ")
	if e.Op != "" {
		b.WriteString(e.Op)
		b.WriteByte(' ')
	}
	b.WriteString(e.URL)
	if e.Status != 0 {
		fmt.Fprintf(&b, " → HTTP %d", e.Status)
	}
	if e.Code != "" {
		fmt.Fprintf(&b, " [%s]", e.Code)
	}
	fmt.Fprintf(&b, " (%s)", e.Kind)
	if e.Attempts > 1 {
		fmt.Fprintf(&b, " after %d attempts", e.Attempts)
	}
	if e.Err != nil {
		msg := e.Err.Error()
		for _, h := range sensitiveHeaders {
			msg = redactHeader(msg, h)
		}
		b.WriteString(": ")
		b.WriteString(msg)
	}
	return b.String()
}

// Unwrap 支持 errors.Is / errors.As 穿透到底层错误。
func (e *Error) Unwrap() error { return e.Err }

// Timeout 供 net.Error 风格的调用方判别超时。
func (e *Error) Timeout() bool { return e != nil && e.Kind == KindTimeout }

// Temporary 供 net.Error 风格的调用方判别临时错误（网络/超时）。
func (e *Error) Temporary() bool {
	return e != nil && (e.Kind == KindNetwork || e.Kind == KindTimeout)
}

// redactHeader 把 "Header: value" 形式的敏感值替换为 ***。
func redactHeader(s, header string) string {
	idx := 0
	lower := strings.ToLower(s)
	h := strings.ToLower(header)
	for {
		i := strings.Index(lower[idx:], h+":")
		if i < 0 {
			return s
		}
		start := idx + i + len(h) + 1
		end := start
		for end < len(s) && s[end] != '\n' && s[end] != ';' {
			end++
		}
		s = s[:start] + " ***" + s[end:]
		lower = strings.ToLower(s)
		idx = start
	}
}

// newError 构造错误。
func newError(kind Kind, req *Request, method, url string, err error) *Error {
	e := &Error{Kind: kind, Op: method, Method: method, URL: url, Err: err}
	if req != nil && req.Attempts > 0 {
		e.Attempts = req.Attempts
	}
	return e
}

// KindOf 返回错误分类，非本包错误返回空串。
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return ""
}

// IsTimeout 判断是否超时。
func IsTimeout(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind == KindTimeout
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// IsCancel 判断是否被取消。
func IsCancel(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind == KindCanceled
	}
	return errors.Is(err, context.Canceled)
}

// IsStatus 判断是否为状态码错误；不给 code 时任意状态码错误均为 true。
func IsStatus(err error, code ...int) bool {
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindStatus {
		return false
	}
	if len(code) == 0 {
		return true
	}
	for _, c := range code {
		if e.Status == c {
			return true
		}
	}
	return false
}

// IsNetwork 判断是否网络层错误。
func IsNetwork(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Kind == KindNetwork
}

// ErrorOf 取出 *Error，非本包错误返回 nil。
func ErrorOf(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}

// statusError 由响应构造状态码错误。
func statusError(resp *Response, attempts int) *Error {
	e := &Error{
		Kind:     KindStatus,
		Status:   resp.Status,
		Response: resp,
		Attempts: attempts,
	}
	if resp.Request != nil {
		e.Op = resp.Request.Method
		e.Method = resp.Request.Method
		e.URL = resp.Request.URL.String()
	}
	if resp.Header != nil {
		e.Code = resp.Header.Get("X-Error-Code")
	}
	e.RetryAfter = parseRetryAfter(resp.Header)
	if resp.Body != nil {
		// 让调用方还能读一次错误体
	}
	return e
}

// httpStatusText 中文友好提示。
func httpStatusText(status int) string {
	if s := http.StatusText(status); s != "" {
		return fmt.Sprintf("%d %s", status, s)
	}
	return fmt.Sprintf("%d", status)
}
