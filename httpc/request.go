package httpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Request 描述一个 HTTP 请求。字段可按需组合，优先级：
// Multipart > Form > JSON > Data > Body。
type Request struct {
	// Method 请求方法，空则 GET。
	Method string
	// URL 请求地址，可以是相对地址（配合 BaseURL）。
	URL string
	// BaseURL 覆盖实例的 BaseURL。
	BaseURL string
	// Params 查询参数。
	Params Params
	// ParamsEncoder 自定义参数编码器。
	ParamsEncoder ParamsEncoder
	// Header 请求头（与实例默认头合并）。
	Header http.Header
	// Data 请求体：[]byte / string / io.Reader / url.Values / 其他（JSON 序列化）。
	Data any
	// JSON 以 JSON 序列化的请求体（优先于 Data）。
	JSON any
	// Form 表单请求体（application/x-www-form-urlencoded）。
	Form url.Values
	// Multipart 分段上传请求体。
	Multipart *Multipart
	// Body 原始请求体，最多只能被发送一次（不实现 io.Seeker 时不可重试）。
	Body io.Reader
	// ContentType 覆盖自动推导的 Content-Type。
	ContentType string
	// Timeout 本请求超时（0 = 用实例默认，负数不限）。
	Timeout time.Duration
	// Context 请求上下文，可用于取消与携带截止时间。
	Context context.Context
	// Token 取消令牌（与 Context 可同时使用）。
	Token *CancelToken
	// ResponseType 响应体形式（默认 JSON）。
	ResponseType ResponseType
	// ValidateStatus 本请求的状态码判定函数。
	ValidateStatus func(status int) bool
	// Progress 本请求的进度回调。
	Progress ProgressFunc
	// ProgressInterval 进度回调最小间隔。
	ProgressInterval time.Duration
	// MaxContentLength 响应体最大字节数（0 = 用实例默认）。
	MaxContentLength int64
	// MaxRedirects 重定向次数（0 = 用实例默认，负数禁止）。
	MaxRedirects int
	// Retry 本请求的重试策略（非空时覆盖实例默认）。
	Retry *RetryConfig
	// NoRetry 禁用本请求的重试。
	NoRetry bool
	// Cookies 本请求额外携带的 Cookie。
	Cookies []*http.Cookie

	// Attempts 已尝试次数，失败时回填（只读）。
	Attempts int
}

// bodySource 请求体来源，供重试时重建。
type bodySource struct {
	open        func() (io.ReadCloser, int64, error)
	replayable  bool
	contentType string
	size        int64
}

// open 打开一份全新的请求体（不可重放时返回错误）。
func (b *bodySource) openOnce(attempt int) (io.ReadCloser, int64, error) {
	if b == nil {
		return nil, 0, nil
	}
	if attempt > 1 && !b.replayable {
		return nil, 0, fmt.Errorf("请求体不可重放，已跳过重试")
	}
	return b.open()
}

// method 返回规范化方法名。
func (r *Request) method() string {
	m := strings.ToUpper(strings.TrimSpace(r.Method))
	if m == "" {
		return http.MethodGet
	}
	return m
}

// buildBody 按字段优先级生成请求体。
func (r *Request) buildBody() (*bodySource, error) {
	switch {
	case r.Multipart != nil:
		return r.Multipart.body()
	case len(r.Form) > 0:
		return bytesBody([]byte(r.Form.Encode()), "application/x-www-form-urlencoded"), nil
	case r.JSON != nil:
		data, err := json.Marshal(r.JSON)
		if err != nil {
			return nil, fmt.Errorf("JSON 序列化失败: %w", err)
		}
		return bytesBody(data, "application/json; charset=utf-8"), nil
	case r.Data != nil:
		return dataBody(r.Data)
	case r.Body != nil:
		return readerBody(r.Body)
	}
	return nil, nil
}

// bytesBody 由字节切片生成可重放体。
func bytesBody(b []byte, ct string) *bodySource {
	return &bodySource{
		replayable:  true,
		contentType: ct,
		size:        int64(len(b)),
		open: func() (io.ReadCloser, int64, error) {
			return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
		},
	}
}

// readerBody 包装 io.Reader；实现 io.ReadSeeker 时可重放。
func readerBody(rd io.Reader) (*bodySource, error) {
	if rs, ok := rd.(io.ReadSeeker); ok {
		start, err := rs.Seek(0, io.SeekCurrent)
		if err == nil {
			end, err2 := rs.Seek(0, io.SeekEnd)
			if err2 == nil {
				if _, err3 := rs.Seek(start, io.SeekStart); err3 == nil {
					size := end - start
					return &bodySource{
						replayable:  true,
						contentType: "application/octet-stream",
						size:        size,
						open: func() (io.ReadCloser, int64, error) {
							if _, err := rs.Seek(start, io.SeekStart); err != nil {
								return nil, 0, err
							}
							return io.NopCloser(rs), size, nil
						},
					}, nil
				}
			}
		}
	}
	return &bodySource{
		replayable:  false,
		contentType: "application/octet-stream",
		size:        -1,
		open: func() (io.ReadCloser, int64, error) {
			return io.NopCloser(rd), -1, nil
		},
	}, nil
}

// dataBody 把 Data 按类型序列化。
func dataBody(v any) (*bodySource, error) {
	switch d := v.(type) {
	case []byte:
		return bytesBody(d, "application/octet-stream"), nil
	case json.RawMessage:
		return bytesBody([]byte(d), "application/json; charset=utf-8"), nil
	case string:
		return bytesBody([]byte(d), "text/plain; charset=utf-8"), nil
	case *string:
		return bytesBody([]byte(*d), "text/plain; charset=utf-8"), nil
	case url.Values:
		return bytesBody([]byte(d.Encode()), "application/x-www-form-urlencoded"), nil
	case io.Reader:
		return readerBody(d)
	case nil:
		return nil, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("请求体 JSON 序列化失败: %w", err)
		}
		return bytesBody(data, "application/json; charset=utf-8"), nil
	}
}

// resolved 是完全确定下来的请求参数。
type resolved struct {
	method           string
	url              string
	header           http.Header
	body             *bodySource
	timeout          time.Duration
	maxContentLength int64
	responseType     ResponseType
	validate         func(int) bool
	progress         ProgressFunc
	progressInterval time.Duration
	retry            RetryConfig
	maxRedirects     int
	cookies          []*http.Cookie
	ctx              context.Context
}

// resolve 合并实例配置与请求字段。
func (c *Client) resolve(r *Request) (*resolved, error) {
	cfg := c.config()
	if r == nil {
		return nil, fmt.Errorf("非空请求描述缺失")
	}
	base := cfg.BaseURL
	if r.BaseURL != "" {
		base = r.BaseURL
	}
	params := mergeParams(cfg.Params, r.Params)
	addr, err := resolveURL(base, r.URL, params, r.ParamsEncoder)
	if err != nil {
		return nil, err
	}
	header := make(http.Header, len(cfg.Header)+len(r.Header)+2)
	for k, vs := range cfg.Header {
		header[k] = append([]string(nil), vs...)
	}
	for k, vs := range r.Header {
		header[k] = append([]string(nil), vs...)
	}
	if header.Get("User-Agent") == "" && cfg.UserAgent != "" {
		header.Set("User-Agent", cfg.UserAgent)
	}
	body, err := r.buildBody()
	if err != nil {
		return nil, err
	}
	ct := r.ContentType
	if ct == "" && body != nil {
		ct = body.contentType
	}
	if ct != "" {
		header.Set("Content-Type", ct)
	}
	timeout := cfg.Timeout
	if r.Timeout != 0 {
		timeout = r.Timeout
	}
	maxLen := cfg.MaxContentLength
	if r.MaxContentLength != 0 {
		maxLen = r.MaxContentLength
	}
	progress := cfg.Progress
	if r.Progress != nil {
		progress = r.Progress
	}
	interval := cfg.ProgressInterval
	if r.ProgressInterval != 0 {
		interval = r.ProgressInterval
	}
	validate := cfg.ValidateStatus
	if r.ValidateStatus != nil {
		validate = r.ValidateStatus
	}
	retry := cfg.Retry
	if r.Retry != nil {
		retry = retry.merge(*r.Retry)
	}
	if r.NoRetry {
		retry.Max = 0
	}
	maxRedirects := cfg.MaxRedirects
	if r.MaxRedirects != 0 {
		maxRedirects = r.MaxRedirects
	}
	ctx := r.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return &resolved{
		method:           r.method(),
		url:              addr,
		header:           header,
		body:             body,
		timeout:          timeout,
		maxContentLength: maxLen,
		responseType:     r.ResponseType,
		validate:         validate,
		progress:         progress,
		progressInterval: interval,
		retry:            retry,
		maxRedirects:     maxRedirects,
		cookies:          r.Cookies,
		ctx:              ctx,
	}, nil
}
