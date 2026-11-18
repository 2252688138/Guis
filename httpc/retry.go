package httpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"
)

// RetryConfig 重试策略。零值 = 不重试。
type RetryConfig struct {
	// Max 最大重试次数（不含首次请求）。
	Max int
	// Backoff 首次退避时长，默认 200ms。
	Backoff time.Duration
	// MaxBackoff 退避上限，默认 5s。
	MaxBackoff time.Duration
	// Jitter 退避抖动系数（0-1），默认 0.2；负数禁用抖动。
	Jitter float64
	// DisableRetryAfter 忽略服务端的 Retry-After 头。
	DisableRetryAfter bool
	// RetryOn 自定义是否需要重试；返回 false 则不重试（优先级最高）。
	RetryOn func(resp *Response, err error) bool
	// RetryStatus 额外需要重试的状态码（默认 429 / 5xx）。
	RetryStatus []int
}

// normalized 补默认值。
func (r RetryConfig) normalized() RetryConfig {
	if r.Backoff <= 0 {
		r.Backoff = 200 * time.Millisecond
	}
	if r.MaxBackoff <= 0 {
		r.MaxBackoff = 5 * time.Second
	}
	if r.Jitter == 0 {
		r.Jitter = 0.2
	}
	return r
}

// merge 用 other 的非零字段覆盖 r。
func (r RetryConfig) merge(other RetryConfig) RetryConfig {
	if other.Max != 0 {
		r.Max = other.Max
	}
	if other.Backoff != 0 {
		r.Backoff = other.Backoff
	}
	if other.MaxBackoff != 0 {
		r.MaxBackoff = other.MaxBackoff
	}
	if other.Jitter != 0 {
		r.Jitter = other.Jitter
	}
	if other.DisableRetryAfter {
		r.DisableRetryAfter = true
	}
	if other.RetryOn != nil {
		r.RetryOn = other.RetryOn
	}
	if len(other.RetryStatus) > 0 {
		r.RetryStatus = other.RetryStatus
	}
	return r
}

// retryableStatus 判断状态码是否可重试。
func (r RetryConfig) retryableStatus(status int) bool {
	for _, s := range r.RetryStatus {
		if s == status {
			return true
		}
	}
	return status == 429 || (status >= 500 && status <= 599)
}

// shouldRetry 判断是否需要再次尝试。
func shouldRetry(cfg RetryConfig, resp *Response, err error, attempt, maxAttempts int) bool {
	if attempt >= maxAttempts || cfg.Max <= 0 {
		return false
	}
	if cfg.RetryOn != nil {
		return cfg.RetryOn(resp, err)
	}
	if err != nil {
		switch KindOf(err) {
		case KindCanceled, KindConfig, KindTooLarge, KindUnsupported, KindDecode:
			return false
		case KindStatus:
			// 由响应状态决定
		default:
			return true
		}
	}
	if resp != nil {
		return cfg.retryableStatus(resp.Status)
	}
	return false
}

// retryDelay 计算第 attempt 次重试前的等待时长。
func retryDelay(cfg RetryConfig, attempt int, resp *Response) time.Duration {
	d := cfg.Backoff
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= cfg.MaxBackoff {
			break
		}
	}
	if d > cfg.MaxBackoff {
		d = cfg.MaxBackoff
	}
	if cfg.Jitter > 0 {
		d += time.Duration(float64(d) * cfg.Jitter * jitterFactor())
	}
	if resp != nil && !cfg.DisableRetryAfter {
		if ra := parseRetryAfter(resp.Header); ra > d {
			d = ra
		}
	}
	if d < 0 {
		d = 0
	}
	return d
}

// sleepCtx 可被 context 中断的睡眠。
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// wrapTransportError 把传输层错误映射成本包错误。
func wrapTransportError(err error, res *resolved, attempt int) error {
	if err == nil {
		return nil
	}
	e := &Error{Op: res.method, Method: res.method, URL: res.url, Err: err, Attempts: attempt}
	ctx := res.ctx
	if ctx != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			e.Kind = KindTimeout
			return e
		}
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			e.Kind = KindCanceled
			return e
		}
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		e.Kind = KindTimeout
		return e
	}
	// url.Error 包裹的超时
	var ue *url.Error
	if errors.As(err, &ue) && ue.Timeout() {
		e.Kind = KindTimeout
		return e
	}
	e.Kind = KindNetwork
	return e
}

// sameOrigin 判断两个 URL 是否同源。
func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return true
	}
	return a.Scheme == b.Scheme && a.Host == b.Host
}

// jitterFactor 返回 [0,1) 随机数（取时间纳秒低位，避免引入 math/rand 锁）。
func jitterFactor() float64 {
	n := time.Now().UnixNano()
	return float64(n%1000) / 1000.0
}

// errRetryBody 请求体不可重放时的提示。
var errRetryBody = fmt.Errorf("请求体不可重放")
