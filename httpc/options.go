package httpc

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Option 修改请求描述的选项，仅在本次请求生效。
type Option func(*Request)

// WithHeader 设置请求头。
func WithHeader(key, value string) Option {
	return func(r *Request) {
		if r.Header == nil {
			r.Header = http.Header{}
		}
		r.Header.Set(key, value)
	}
}

// WithHeaders 合并多个请求头。
func WithHeaders(h http.Header) Option {
	return func(r *Request) {
		if len(h) == 0 {
			return
		}
		if r.Header == nil {
			r.Header = http.Header{}
		}
		for k, vs := range h {
			r.Header[k] = append([]string(nil), vs...)
		}
	}
}

// WithParam 设置查询参数。
func WithParam(key string, value any) Option {
	return func(r *Request) {
		if r.Params == nil {
			r.Params = Params{}
		}
		r.Params[key] = value
	}
}

// WithParams 合并查询参数。
func WithParams(p Params) Option {
	return func(r *Request) {
		if len(p) == 0 {
			return
		}
		if r.Params == nil {
			r.Params = Params{}
		}
		for k, v := range p {
			r.Params[k] = v
		}
	}
}

// WithTimeout 设置本请求超时（负数不限）。
func WithTimeout(d time.Duration) Option {
	return func(r *Request) { r.Timeout = d }
}

// WithContext 绑定上下文。
func WithContext(ctx context.Context) Option {
	return func(r *Request) { r.Context = ctx }
}

// WithCancelToken 绑定取消令牌。
func WithCancelToken(t *CancelToken) Option {
	return func(r *Request) { r.Token = t }
}

// WithProgress 设置进度回调。
func WithProgress(fn ProgressFunc) Option {
	return func(r *Request) { r.Progress = fn }
}

// WithProgressInterval 设置进度回调最小间隔。
func WithProgressInterval(d time.Duration) Option {
	return func(r *Request) { r.ProgressInterval = d }
}

// WithResponseType 设置响应类型。
func WithResponseType(t ResponseType) Option {
	return func(r *Request) { r.ResponseType = t }
}

// WithRetry 覆盖重试策略。
func WithRetry(cfg RetryConfig) Option {
	return func(r *Request) { r.Retry = &cfg }
}

// WithNoRetry 禁用本请求的重试。
func WithNoRetry() Option {
	return func(r *Request) { r.NoRetry = true }
}

// WithValidateStatus 自定义状态码成功判定。
func WithValidateStatus(fn func(status int) bool) Option {
	return func(r *Request) { r.ValidateStatus = fn }
}

// WithBasicAuth 设置 Basic 认证（避免手工拼接 base64）。
func WithBasicAuth(user, pass string) Option {
	return func(r *Request) {
		if r.Header == nil {
			r.Header = http.Header{}
		}
		r.Header.Set("Authorization", basicAuth(user, pass))
	}
}

// WithBearer 设置 Bearer 认证。
func WithBearer(token string) Option {
	return func(r *Request) {
		if r.Header == nil {
			r.Header = http.Header{}
		}
		r.Header.Set("Authorization", authHeader("Bearer", token))
	}
}

// WithUserAgent 设置 User-Agent。
func WithUserAgent(ua string) Option {
	return func(r *Request) {
		if r.Header == nil {
			r.Header = http.Header{}
		}
		r.Header.Set("User-Agent", ua)
	}
}

// WithContentType 覆盖 Content-Type。
func WithContentType(ct string) Option {
	return func(r *Request) { r.ContentType = ct }
}

// WithBody 设置原始请求体（[]byte / string / io.Reader 等，见 Request.Data）。
func WithBody(data any) Option {
	return func(r *Request) { r.Data = data }
}

// WithJSON 设置 JSON 请求体。
func WithJSON(v any) Option {
	return func(r *Request) { r.JSON = v }
}

// WithForm 设置表单请求体。
func WithForm(v url.Values) Option {
	return func(r *Request) { r.Form = v }
}

// WithMultipart 设置分段上传请求体。
func WithMultipart(mp *Multipart) Option {
	return func(r *Request) { r.Multipart = mp }
}

// WithCookies 追加请求 Cookie。
func WithCookies(cookies ...*http.Cookie) Option {
	return func(r *Request) { r.Cookies = append(r.Cookies, cookies...) }
}

// WithMaxContentLength 限制响应体大小（<=0 不限）。
func WithMaxContentLength(n int64) Option {
	return func(r *Request) { r.MaxContentLength = n }
}

// WithMaxRedirects 设置重定向次数（负数禁止）。
func WithMaxRedirects(n int) Option {
	return func(r *Request) { r.MaxRedirects = n }
}

// WithBaseURL 覆盖本次请求的 BaseURL。
func WithBaseURL(url string) Option {
	return func(r *Request) { r.BaseURL = url }
}

// WithParamsEncoder 设置参数编码器。
func WithParamsEncoder(fn ParamsEncoder) Option {
	return func(r *Request) { r.ParamsEncoder = fn }
}

// basicAuth 生成 Basic 认证头值。
func basicAuth(user, pass string) string {
	const b64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	src := []byte(user + ":" + pass)
	out := make([]byte, 0, (len(src)+2)/3*4)
	for i := 0; i < len(src); i += 3 {
		var buf [3]byte
		n := copy(buf[:], src[i:])
		out = append(out,
			b64[buf[0]>>2],
			b64[(buf[0]&0x03)<<4|buf[1]>>4],
		)
		if n > 1 {
			out = append(out, b64[(buf[1]&0x0F)<<2|buf[2]>>6])
		} else {
			out = append(out, '=')
		}
		if n > 2 {
			out = append(out, b64[buf[2]&0x3F])
		} else {
			out = append(out, '=')
		}
	}
	return "Basic " + string(out)
}
