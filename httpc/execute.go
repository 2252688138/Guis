package httpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Do 发送请求并按配置完成重试、限流、拦截器与响应体读取。
func (c *Client) Do(r *Request) (*Response, error) {
	cfg := c.config()
	if r == nil {
		r = &Request{}
	}
	if r.Header == nil {
		r.Header = http.Header{}
	}
	if err := runRequestInterceptors(cfg.Interceptors.Request, r); err != nil {
		return nil, runErrorInterceptors(cfg.Interceptors.Error, err, r)
	}
	res, err := c.resolve(r)
	if err != nil {
		e := &Error{Kind: KindConfig, Op: r.method(), Method: r.method(), URL: r.URL, Err: err}
		return nil, runErrorInterceptors(cfg.Interceptors.Error, e, r)
	}
	ctx := res.ctx
	var cancels []context.CancelFunc
	releaseCtx := func() {
		for _, cancel := range cancels {
			cancel()
		}
		cancels = nil
	}
	if r.Token != nil {
		tctx, cancel := r.Token.Context(ctx)
		cancels = append(cancels, cancel)
		ctx = tctx
	}
	if res.timeout > 0 {
		tctx, cancel := context.WithTimeout(ctx, res.timeout)
		cancels = append(cancels, cancel)
		ctx = tctx
	}
	// 流式响应的 context 必须存活到调用方 Close，否则响应体读到一半会被取消。
	streamCtx := false
	defer func() {
		if !streamCtx {
			releaseCtx()
		}
	}()
	res.ctx = ctx

	release, err := c.acquire(ctx, cfg)
	if err != nil {
		e := &Error{Kind: KindCanceled, Op: res.method, Method: res.method, URL: res.url, Err: err}
		return nil, runErrorInterceptors(cfg.Interceptors.Error, e, r)
	}
	defer release()

	maxAttempts := res.retry.Max + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var (
		lastResp *Response
		lastErr  error
	)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		r.Attempts = attempt
		resp, exErr := c.execute(cfg, res, attempt)
		lastResp, lastErr = resp, exErr
		if exErr == nil {
			if err := runResponseInterceptors(cfg.Interceptors.Response, resp, r); err != nil {
				lastErr = err
				break
			}
			if cfg.OnResponse != nil {
				cfg.OnResponse(resp)
			}
			if resp.Type == ResponseStream {
				streamCtx = true
				resp.setCancel(releaseCtx)
			}
			return resp, nil
		}
		if !shouldRetry(res.retry, lastResp, exErr, attempt, maxAttempts) {
			break
		}
		delay := retryDelay(res.retry, attempt, lastResp)
		cfg.debugf("httpc: %s %s 第 %d 次失败(%v)，%s 后重试", res.method, res.url, attempt, exErr, delay)
		if werr := sleepCtx(ctx, delay); werr != nil {
			lastErr = &Error{Kind: KindCanceled, Op: res.method, Method: res.method, URL: res.url, Err: werr, Attempts: attempt}
			break
		}
	}
	if lastErr == nil {
		lastErr = &Error{Kind: KindNetwork, Op: res.method, Method: res.method, URL: res.url, Err: errors.New("请求失败")}
	}
	return lastResp, runErrorInterceptors(cfg.Interceptors.Error, lastErr, r)
}

// Request 同 Do（语义命名，便于阅读）。
func (c *Client) Request(r *Request) (*Response, error) { return c.Do(r) }

// execute 执行一次传输（含响应体读取与状态校验）。
func (c *Client) execute(cfg Config, res *resolved, attempt int) (*Response, error) {
	body, size, err := res.body.openOnce(attempt)
	if err != nil {
		return nil, &Error{Kind: KindConfig, Op: res.method, Method: res.method, URL: res.url, Err: err, Attempts: attempt}
	}
	var payload io.Reader
	if body != nil {
		payload = body
		if res.progress != nil {
			payload = newProgressReader(body, size, res.progress, "upload", res.progressInterval)
		}
	}
	req, err := http.NewRequestWithContext(res.ctx, res.method, res.url, payload)
	if err != nil {
		if body != nil {
			body.Close()
		}
		return nil, &Error{Kind: KindConfig, Op: res.method, Method: res.method, URL: res.url, Err: err, Attempts: attempt}
	}
	if len(res.header) > 0 {
		req.Header = res.header.Clone()
	}
	if size > 0 {
		req.ContentLength = size
	}
	for _, ck := range res.cookies {
		if ck != nil {
			req.AddCookie(ck)
		}
	}
	if !cfg.DisableCompression && req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "gzip, br")
	}
	if cfg.OnRequest != nil {
		cfg.OnRequest(req)
	}
	hc, err := c.httpClient(cfg, res.maxRedirects)
	if err != nil {
		return nil, &Error{Kind: KindConfig, Op: res.method, Method: res.method, URL: res.url, Err: err, Attempts: attempt}
	}
	start := time.Now()
	hr, err := hc.Do(req)
	if err != nil {
		return nil, wrapTransportError(err, res, attempt)
	}
	dur := time.Since(start)

	if res.responseType == ResponseStream {
		var rd io.Reader = hr.Body
		if res.progress != nil {
			rd = newProgressReader(hr.Body, hr.ContentLength, res.progress, "download", res.progressInterval)
		}
		resp := newResponse(req, hr, nil, -1, dur, wrapReadCloser(rd, hr.Body), ResponseStream)
		resp.Attempts = attempt
		if res.validate != nil && !res.validate(hr.StatusCode) && !noRedirectStatus(res.maxRedirects, hr.StatusCode) {
			return resp, statusError(resp, attempt)
		}
		return resp, nil
	}

	defer hr.Body.Close()
	reader, err := decompress(hr.Body, hr.Header.Get("Content-Encoding"))
	if err != nil {
		return nil, &Error{Kind: KindDecode, Op: res.method, Method: res.method, URL: res.url, Err: err, Attempts: attempt}
	}
	if res.progress != nil {
		reader = newProgressReader(reader, hr.ContentLength, res.progress, "download", res.progressInterval)
	}
	data, tooLarge, err := readAllLimited(reader, res.maxContentLength)
	if err != nil {
		mapped := wrapReadError(err, res, attempt)
		return nil, mapped
	}
	if tooLarge {
		return nil, &Error{
			Kind:     KindTooLarge,
			Op:       res.method,
			Method:   res.method,
			URL:      res.url,
			Attempts: attempt,
			Err:      fmt.Errorf("响应体超过 %d 字节", res.maxContentLength),
		}
	}
	resp := newResponse(req, hr, data, int64(len(data)), dur, nil, res.responseType)
	resp.Attempts = attempt
	if res.validate != nil && !res.validate(hr.StatusCode) && !noRedirectStatus(res.maxRedirects, hr.StatusCode) {
		return resp, statusError(resp, attempt)
	}
	return resp, nil
}

// readAllLimited 读取响应体，超限返回 tooLarge。
func readAllLimited(r io.Reader, limit int64) (data []byte, tooLarge bool, err error) {
	if limit <= 0 {
		b, err := io.ReadAll(r)
		return b, false, err
	}
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(b)) > limit {
		return nil, true, nil
	}
	return b, false, nil
}

// wrapReadError 把读响应体时的错误分类。
func wrapReadError(err error, res *resolved, attempt int) error {
	if err == nil {
		return nil
	}
	if res.ctx != nil {
		if errors.Is(res.ctx.Err(), context.DeadlineExceeded) {
			return &Error{Kind: KindTimeout, Op: res.method, Method: res.method, URL: res.url, Err: err, Attempts: attempt}
		}
		if errors.Is(res.ctx.Err(), context.Canceled) {
			return &Error{Kind: KindCanceled, Op: res.method, Method: res.method, URL: res.url, Err: err, Attempts: attempt}
		}
	}
	return &Error{Kind: KindNetwork, Op: res.method, Method: res.method, URL: res.url, Err: err, Attempts: attempt}
}

// readCloser 把 reader + closer 合成 ReadCloser。
type readCloser struct {
	r io.Reader
	c io.Closer
}

// Read 实现 io.Reader。
func (rc *readCloser) Read(b []byte) (int, error) { return rc.r.Read(b) }

// Close 实现 io.Closer。
func (rc *readCloser) Close() error {
	if rc.c != nil {
		return rc.c.Close()
	}
	return nil
}

// wrapReadCloser 保证返回 ReadCloser。
func wrapReadCloser(r io.Reader, c io.Closer) io.ReadCloser {
	if rc, ok := r.(io.ReadCloser); ok {
		return rc
	}
	return &readCloser{r: r, c: c}
}
