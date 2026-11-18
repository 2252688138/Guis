package httpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// buildTransport 根据配置构造传输层。
func buildTransport(cfg Config) (*http.Transport, error) {
	dialer := &net.Dialer{Timeout: cfg.ConnectTimeout, KeepAlive: 30 * time.Second}
	tr := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConns,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   cfg.ConnectTimeout,
		ExpectContinueTimeout: time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    true, // 压缩解压统一由本包处理
	}
	tlsCfg, err := buildTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}
	tr.TLSClientConfig = tlsCfg

	proxy := strings.TrimSpace(cfg.Proxy)
	switch {
	case proxy == "":
		tr.Proxy = http.ProxyFromEnvironment
	case strings.EqualFold(proxy, "none") || strings.EqualFold(proxy, "direct"):
		tr.Proxy = nil
	default:
		u, err := url.Parse(proxy)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("代理地址非法: %q", cfg.Proxy)
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https":
			tr.Proxy = http.ProxyURL(u)
		case "socks5", "socks5h":
			tr.Proxy = nil
			tr.DialContext = socks5DialContext(u, dialer)
		default:
			return nil, fmt.Errorf("不支持的代理协议: %q（支持 http/https/socks5）", u.Scheme)
		}
	}
	return tr, nil
}

// buildTLSConfig 构造 tls.Config。
func buildTLSConfig(c *TLSConfig) (*tls.Config, error) {
	if c == nil {
		return nil, nil
	}
	if c.Config != nil {
		return c.Config, nil
	}
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.InsecureSkipVerify,
	}
	if len(c.RootCAs) > 0 {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(c.RootCAs) {
			return nil, fmt.Errorf("RootCAs 不是合法的 PEM 证书")
		}
		cfg.RootCAs = pool
	}
	if c.ClientCertFile != "" || c.ClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载客户端证书失败: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}
