package httpc

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Response 是一次成功（或未通过 ValidateStatus 但仍收到）的响应。
//
// 除 ResponseType 为 ResponseStream 外，响应体已读完并缓存在 Data 中，无需调用方 Close。
type Response struct {
	// Status HTTP 状态码。
	Status int
	// StatusText 状态文本（如 "200 OK"）。
	StatusText string
	// Proto 协议版本（如 "HTTP/1.1"）。
	Proto string
	// Header 响应头。
	Header http.Header
	// Request 底层请求（由本包构造）。
	Request *http.Request
	// URL 最终请求地址（含重定向后的地址）。
	URL string
	// Duration 本次请求耗时。
	Duration time.Duration
	// Size 响应体字节数（解压后）；stream 模式为 -1。
	Size int64
	// Data 响应体字节（除 stream 模式）。
	Data []byte
	// Body stream 模式下的响应体，调用方负责 Close。
	Body io.ReadCloser
	// Type 响应类型。
	Type ResponseType
	// Attempts 实际尝试次数。
	Attempts int
	// Cookies 响应下发的 Cookie。
	Cookies []*http.Cookie
	// cancel 流式响应的 context 释放函数（由 Close 调用）。
	cancel context.CancelFunc
}

// OK 状态码是否在 200-299。
func (r *Response) OK() bool { return r != nil && r.Status >= 200 && r.Status < 300 }

// ContentType 返回响应 Content-Type。
func (r *Response) ContentType() string {
	if r == nil || r.Header == nil {
		return ""
	}
	return r.Header.Get("Content-Type")
}

// Bytes 返回响应体字节。
func (r *Response) Bytes() []byte {
	if r == nil {
		return nil
	}
	return r.Data
}

// Text 返回响应体字符串。
func (r *Response) Text() string {
	if r == nil {
		return ""
	}
	return string(r.Data)
}

// Bind 把响应体解码到 v：按 Content-Type 选 JSON / XML，不明时先试 JSON 再试 XML。
func (r *Response) Bind(v any) error {
	if r == nil {
		return &Error{Kind: KindDecode, Err: fmt.Errorf("响应为 nil")}
	}
	if v == nil {
		return nil
	}
	ct := strings.ToLower(r.ContentType())
	wantXML := strings.Contains(ct, "xml")
	if wantXML {
		return r.bindXML(v)
	}
	if len(r.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(r.Data, v); err == nil {
		return nil
	}
	if xerr := r.bindXML(v); xerr == nil {
		return nil
	}
	return &Error{Kind: KindDecode, Status: r.Status, URL: r.URL, Err: fmt.Errorf("响应体不是合法的 JSON/XML")}
}

// bindXML 以 XML 解码。
func (r *Response) bindXML(v any) error {
	if len(r.Data) == 0 {
		return nil
	}
	if err := xml.Unmarshal(r.Data, v); err != nil {
		return &Error{Kind: KindDecode, Status: r.Status, URL: r.URL, Err: err}
	}
	return nil
}

// SaveTo 把响应体写入文件（先写临时文件再改名，失败不留半成品）。
func (r *Response) SaveTo(path string) error {
	if r == nil {
		return fmt.Errorf("响应为 nil")
	}
	if r.Type != ResponseStream {
		return writeFileAtomic(path, r.Data)
	}
	if r.Body == nil {
		return fmt.Errorf("响应体为空")
	}
	return copyToFileAtomic(path, r.Body)
}

// Close 释放 stream 模式占用的连接与超时 context，其余模式无操作。
func (r *Response) Close() error {
	if r == nil {
		return nil
	}
	var err error
	if r.Body != nil {
		err = r.Body.Close()
		r.Body = nil
	}
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	return err
}

// setCancel 记录流式响应的 context 释放函数。
func (r *Response) setCancel(fn context.CancelFunc) {
	if r != nil {
		r.cancel = fn
	}
}

// writeFileAtomic 原子写文件。
func writeFileAtomic(path string, data []byte) error {
	dir := dirOf(path)
	f, err := os.CreateTemp(dir, ".httpc-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// copyToFileAtomic 流式原子落盘。
func copyToFileAtomic(path string, src io.Reader) error {
	dir := dirOf(path)
	f, err := os.CreateTemp(dir, ".httpc-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// dirOf 返回路径所在目录（不存在则当前目录）。
func dirOf(path string) string {
	i := strings.LastIndexAny(path, "/\\")
	if i <= 0 {
		return "."
	}
	return path[:i]
}

// parseRetryAfter 解析 Retry-After 头（秒数或 HTTP 日期）。
func parseRetryAfter(h http.Header) time.Duration {
	if h == nil {
		return 0
	}
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil {
		if n < 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// newResponse 依据状态与头构造响应。
func newResponse(req *http.Request, hr *http.Response, data []byte, size int64, dur time.Duration, body io.ReadCloser, typ ResponseType) *Response {
	resp := &Response{
		Status:     hr.StatusCode,
		StatusText: httpStatusText(hr.StatusCode),
		Proto:      hr.Proto,
		Header:     hr.Header,
		Request:    req,
		Duration:   dur,
		Size:       size,
		Data:       data,
		Body:       body,
		Type:       typ,
		Cookies:    hr.Cookies(),
	}
	if req != nil && req.URL != nil {
		resp.URL = req.URL.String()
	}
	return resp
}

// ensureContextErr 把 context 错误转成本包错误。
func ensureContextErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return newError(KindTimeout, nil, "", "", err)
		}
		if ctx.Err() == context.Canceled {
			return newError(KindCanceled, nil, "", "", err)
		}
	}
	return err
}
