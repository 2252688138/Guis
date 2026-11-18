package httpc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"sync"
)

// Client 是 HTTP 客户端实例，并发安全，零值即可用。
type Client struct {
	mu  sync.Mutex
	cfg Config
	rt  http.RoundTripper
	sem chan struct{}
	jar http.CookieJar
}

// New 创建客户端；传入多个 Config 时按顺序合并（后者非零字段覆盖前者）。
func New(cfgs ...Config) *Client {
	c := &Client{}
	for _, cfg := range cfgs {
		c.cfg = mergeConfig(c.cfg, cfg)
	}
	return c
}

var (
	defaultOnce sync.Once
	defaultCli  *Client
)

// Default 返回全局默认客户端（懒初始化）。
func Default() *Client {
	defaultOnce.Do(func() { defaultCli = New() })
	return defaultCli
}

// Config 返回当前配置快照（已规范化）。
func (c *Client) Config() Config {
	if c == nil {
		return Config{}.normalized()
	}
	c.mu.Lock()
	cfg := c.cfg
	c.mu.Unlock()
	return cfg.normalized()
}

// Set 合并配置并重置内部传输层（便于运行期改代理/超时）。
func (c *Client) Set(cfg Config) *Client {
	if c == nil {
		return c
	}
	c.mu.Lock()
	c.cfg = mergeConfig(c.cfg, cfg)
	c.rt = nil
	c.sem = nil
	c.mu.Unlock()
	return c
}

// Clone 复制一份客户端（含配置与拦截器，不共享连接池）。
func (c *Client) Clone() *Client {
	if c == nil {
		return New()
	}
	c.mu.Lock()
	cfg := c.cfg
	c.mu.Unlock()
	cfg.Interceptors = cfg.Interceptors.clone()
	return &Client{cfg: cfg}
}

// config 返回规范化后的配置快照。
func (c *Client) config() Config {
	return c.Config()
}

// semaphore 返回并发限流信号量，nil 表示不限。
func (c *Client) semaphore(cfg Config) chan struct{} {
	if cfg.MaxConcurrency <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sem == nil || cap(c.sem) != cfg.MaxConcurrency {
		c.sem = make(chan struct{}, cfg.MaxConcurrency)
	}
	return c.sem
}

// acquire 获取并发名额；返回的 func 用于释放。
func (c *Client) acquire(ctx context.Context, cfg Config) (func(), error) {
	sem := c.semaphore(cfg)
	if sem == nil {
		return func() {}, nil
	}
	select {
	case sem <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-sem }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// roundTripper 返回（或懒构建）传输层。
func (c *Client) roundTripper(cfg Config) (http.RoundTripper, error) {
	if cfg.Transport != nil {
		return cfg.Transport, nil
	}
	c.mu.Lock()
	if c.rt != nil {
		rt := c.rt
		c.mu.Unlock()
		return rt, nil
	}
	c.mu.Unlock()
	rt, err := buildTransport(cfg)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.rt == nil {
		c.rt = rt
	}
	out := c.rt
	c.mu.Unlock()
	return out, nil
}

// httpClient 组装单次请求用的 *http.Client（共享连接池，单独的重定向策略）。
func (c *Client) httpClient(cfg Config, maxRedirects int) (*http.Client, error) {
	rt, err := c.roundTripper(cfg)
	if err != nil {
		return nil, err
	}
	hc := &http.Client{
		Transport:     rt,
		Timeout:       0,
		CheckRedirect: redirectPolicy(maxRedirects, cfg.CheckRedirect),
	}
	if cfg.CookieJar != nil {
		hc.Jar = cfg.CookieJar
	} else if jar := c.cookieJar(cfg); jar != nil {
		hc.Jar = jar
	}
	return hc, nil
}

// cookieJar 返回 Cookie 存储，开启 Cookies 时懒建内存 jar。
func (c *Client) cookieJar(cfg Config) http.CookieJar {
	if cfg.CookieJar != nil {
		return cfg.CookieJar
	}
	if !cfg.Cookies {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil
		}
		c.jar = jar
	}
	return c.jar
}

// noRedirectStatus 禁止跟随重定向时，3xx 视为正常响应。
func noRedirectStatus(maxRedirects, status int) bool {
	return maxRedirects < 0 && status >= 300 && status < 400
}

// redirectPolicy 生成重定向策略。
func redirectPolicy(max int, custom func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if max < 0 {
			return http.ErrUseLastResponse
		}
		if len(via) >= max {
			return fmt.Errorf("重定向次数超过 %d", max)
		}
		if len(via) > 0 && !sameOrigin(via[0].URL, req.URL) {
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			req.Header.Del("Proxy-Authorization")
		}
		if custom != nil {
			return custom(req, via)
		}
		return nil
	}
}
