package httpc

import (
	"crypto/tls"
	"net/http"
	"time"
)

// ResponseType 指定响应体的呈现形式。
type ResponseType int

const (
	// ResponseJSON 保留原始字节，Bind 时按 JSON 解码（默认）。
	ResponseJSON ResponseType = iota
	// ResponseText 以字符串形式提供（Text）。
	ResponseText
	// ResponseBytes 以字节切片形式提供（Bytes）。
	ResponseBytes
	// ResponseStream 不读取响应体，由调用方从 Body 自行流式读并负责 Close。
	ResponseStream
)

// String 返回名称。
func (t ResponseType) String() string {
	switch t {
	case ResponseText:
		return "text"
	case ResponseBytes:
		return "bytes"
	case ResponseStream:
		return "stream"
	default:
		return "json"
	}
}

// TLSConfig TLS 相关快捷配置；Config 与 TLS 同时提供时以后者为准。
type TLSConfig struct {
	// Config 直接使用的 tls.Config（非空时忽略下面各快捷字段）。
	Config *tls.Config
	// InsecureSkipVerify 跳过证书校验（仅调试用）。
	InsecureSkipVerify bool
	// RootCAs 追加的 CA 证书（PEM）。
	RootCAs []byte
	// ClientCertFile / ClientKeyFile 双向认证证书与私钥文件路径。
	ClientCertFile string
	ClientKeyFile  string
}

// Config 客户端配置。零值即默认配置。
//
// New 可以一次传多个 Config，后者覆盖前者（按非零字段）。
type Config struct {
	// BaseURL 基础地址，如 "https://api.example.com"；请求 URL 为绝对地址时忽略。
	BaseURL string
	// Header 所有请求的默认头（Authorization / Accept 等）。
	Header http.Header
	// Params 所有请求的默认查询参数（请求级参数覆盖同名项）。
	Params Params
	// UserAgent 默认 User-Agent，空则用 httpc/1.0。
	UserAgent string
	// Timeout 单个请求的总超时（含连接、传输、读完响应体）。0 取默认 30s，负数不限。
	Timeout time.Duration
	// ConnectTimeout 连接建立超时，0 取 min(Timeout, 15s)。
	ConnectTimeout time.Duration
	// Proxy 代理设置："http://host:port"、"https://..."、"socks5://user:pass@host:port"、
	// "none" 强制直连；空串则尊重环境变量（HTTP_PROXY / HTTPS_PROXY / NO_PROXY）。
	Proxy string
	// TLS TLS 快捷配置。
	TLS *TLSConfig
	// Transport 自定义传输层，非空时忽略 Proxy/TLS/连接池相关字段。
	Transport http.RoundTripper
	// CookieJar Cookie 存储；为空且 Cookies 为 true 时自动创建内存 jar。
	CookieJar http.CookieJar
	// Cookies 自动保持 Cookie（无 CookieJar 时创建内存 jar）。
	Cookies bool
	// MaxRedirects 最大重定向次数，0 取 10，负数禁止重定向。
	MaxRedirects int
	// CheckRedirect 自定义重定向决策（与 MaxRedirects 同时生效）。
	CheckRedirect func(req *http.Request, via []*http.Request) error
	// Retry 重试策略。
	Retry RetryConfig
	// MaxConcurrency 同时进行的最大请求数，<=0 不限。
	MaxConcurrency int
	// MaxIdleConns 连接池空闲连接上限，0 取 32。
	MaxIdleConns int
	// DisableCompression 关闭自动解压（默认开启 gzip/br）。
	DisableCompression bool
	// ValidateStatus 判定状态码是否成功，nil 时默认 200-299。
	ValidateStatus func(status int) bool
	// ResponseType 默认响应类型（ResponseJSON）。
	ResponseType ResponseType
	// MaxContentLength 响应体最大字节数，<=0 不限。
	MaxContentLength int64
	// Progress 全局进度回调。
	Progress ProgressFunc
	// ProgressInterval 进度回调最小间隔，0 取 50ms。
	ProgressInterval time.Duration
	// Interceptors 拦截器集合。
	Interceptors Interceptors
	// OnRequest 请求发出前的轻量钩子（日志用，不改变流程）。
	OnRequest func(*http.Request)
	// OnResponse 响应返回后的轻量钩子。
	OnResponse func(*Response)
	// Debug 调试输出，形如 fmt.Printf。
	Debug func(format string, args ...any)
}

// defaultUserAgent 默认 UA。
const defaultUserAgent = "guis-httpc/1.0"

// defaultMaxRedirects 默认重定向次数。
const defaultMaxRedirects = 10

// defaultTimeout 默认超时。
const defaultTimeout = 30 * time.Second

// defaultProgressInterval 进度回调节流间隔。
const defaultProgressInterval = 50 * time.Millisecond

// defaultValidateStatus 默认认为 2xx 成功。
func defaultValidateStatus(status int) bool { return status >= 200 && status < 300 }

// merge 用 other 的非零字段覆盖 c。
func mergeConfig(base, other Config) Config {
	if other.BaseURL != "" {
		base.BaseURL = other.BaseURL
	}
	if len(other.Header) > 0 {
		if base.Header == nil {
			base.Header = http.Header{}
		}
		for k, vs := range other.Header {
			cp := append([]string(nil), vs...)
			base.Header[k] = cp
		}
	}
	if len(other.Params) > 0 {
		if base.Params == nil {
			base.Params = Params{}
		}
		for k, v := range other.Params {
			base.Params[k] = v
		}
	}
	if other.UserAgent != "" {
		base.UserAgent = other.UserAgent
	}
	if other.Timeout != 0 {
		base.Timeout = other.Timeout
	}
	if other.ConnectTimeout != 0 {
		base.ConnectTimeout = other.ConnectTimeout
	}
	if other.Proxy != "" {
		base.Proxy = other.Proxy
	}
	if other.TLS != nil {
		base.TLS = other.TLS
	}
	if other.Transport != nil {
		base.Transport = other.Transport
	}
	if other.CookieJar != nil {
		base.CookieJar = other.CookieJar
	}
	if other.Cookies {
		base.Cookies = true
	}
	if other.MaxRedirects != 0 {
		base.MaxRedirects = other.MaxRedirects
	}
	if other.CheckRedirect != nil {
		base.CheckRedirect = other.CheckRedirect
	}
	base.Retry = base.Retry.merge(other.Retry)
	if other.MaxConcurrency != 0 {
		base.MaxConcurrency = other.MaxConcurrency
	}
	if other.MaxIdleConns != 0 {
		base.MaxIdleConns = other.MaxIdleConns
	}
	if other.DisableCompression {
		base.DisableCompression = true
	}
	if other.ValidateStatus != nil {
		base.ValidateStatus = other.ValidateStatus
	}
	if other.ResponseType != ResponseJSON {
		base.ResponseType = other.ResponseType
	}
	if other.MaxContentLength != 0 {
		base.MaxContentLength = other.MaxContentLength
	}
	if other.Progress != nil {
		base.Progress = other.Progress
	}
	if other.ProgressInterval != 0 {
		base.ProgressInterval = other.ProgressInterval
	}
	base.Interceptors = base.Interceptors.merge(other.Interceptors)
	if other.OnRequest != nil {
		base.OnRequest = other.OnRequest
	}
	if other.OnResponse != nil {
		base.OnResponse = other.OnResponse
	}
	if other.Debug != nil {
		base.Debug = other.Debug
	}
	return base
}

// normalized 把零值字段替换为默认值。
func (c Config) normalized() Config {
	if c.UserAgent == "" {
		c.UserAgent = defaultUserAgent
	}
	if c.Timeout == 0 {
		c.Timeout = defaultTimeout
	}
	if c.ConnectTimeout == 0 {
		ct := c.Timeout
		if ct <= 0 || ct > 15*time.Second {
			ct = 15 * time.Second
		}
		c.ConnectTimeout = ct
	}
	if c.MaxRedirects == 0 {
		c.MaxRedirects = defaultMaxRedirects
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 32
	}
	if c.ProgressInterval == 0 {
		c.ProgressInterval = defaultProgressInterval
	}
	if c.ValidateStatus == nil {
		c.ValidateStatus = defaultValidateStatus
	}
	c.Retry = c.Retry.normalized()
	return c
}

// debugf 输出调试信息，未配置 Debug 时静默。
func (c Config) debugf(format string, args ...any) {
	if c.Debug != nil {
		c.Debug(format, args...)
	}
}
