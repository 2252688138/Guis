package plugins

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// CommandRunner 声明式 command 工具的执行器。宿主可注入自己的实现
// （例如复用带进程树管控、隐藏控制台的实现）。
type CommandRunner interface {
	Run(ctx context.Context, cmdline, dir string, timeout time.Duration) (string, error)
}

// CommandFunc 把函数适配成 CommandRunner。
type CommandFunc func(ctx context.Context, cmdline, dir string, timeout time.Duration) (string, error)

// Run 实现 CommandRunner。
func (f CommandFunc) Run(ctx context.Context, cmdline, dir string, timeout time.Duration) (string, error) {
	return f(ctx, cmdline, dir, timeout)
}

// defaultRunner 内核自带的简单执行器（无进程树管控）。
func defaultRunner(ctx context.Context, cmdline, dir string, timeout time.Duration) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cctx, "cmd.exe", "/C", cmdline)
	} else {
		cmd = exec.CommandContext(cctx, "/bin/sh", "-c", cmdline)
	}
	if dir != "" {
		cmd.Dir = dir
	}
	out := &limitedBuffer{limit: MaxResultBytes}
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.WaitDelay = 3 * time.Second
	runErr := cmd.Run()
	if cctx.Err() == context.DeadlineExceeded {
		return out.String(), fmt.Errorf("插件命令超时（%s）", timeout)
	}
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			return out.String(), fmt.Errorf("命令退出码 %d", ee.ExitCode())
		}
		return out.String(), runErr
	}
	return out.String(), nil
}

// limitedBuffer 限制累计写入大小的缓冲。
type limitedBuffer struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	limit int
	cut   bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if b.limit > 0 && b.buf.Len()+len(p) > b.limit {
		room := b.limit - b.buf.Len()
		if room > 0 {
			b.buf.Write(p[:room])
		}
		b.cut = true
		return n, nil
	}
	b.buf.Write(p)
	return n, nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.buf.String()
	if b.cut {
		s += "\n…（输出过长，已截断）"
	}
	return s
}

// HTTPClient 声明式 HTTP 工具的客户端（宿主可替换）。
var HTTPClient = &http.Client{Timeout: 60 * time.Second}

func (m *Manager) runHTTP(ctx context.Context, mf *Manifest, t PluginTool, args map[string]any, env Env) (CallResult, error) {
	req := t.Request
	if req == nil {
		return CallResult{}, fmt.Errorf("工具 %s 缺少 request", t.Name)
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	rawURL := expandTemplate(req.URL, args, env.Workspace, true)
	if _, err := url.Parse(rawURL); err != nil {
		return CallResult{}, fmt.Errorf("URL 无效: %w", err)
	}
	if err := checkHTTPPermission(mf, rawURL); err != nil {
		return CallResult{}, err
	}
	body := ""
	if req.Body != "" {
		body = expandTemplate(req.Body, args, env.Workspace, false)
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	timeout := time.Duration(t.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	hreq, err := http.NewRequestWithContext(cctx, method, rawURL, reader)
	if err != nil {
		return CallResult{}, err
	}
	if body != "" && hreq.Header.Get("Content-Type") == "" {
		hreq.Header.Set("Content-Type", "application/json")
	}
	for k, v := range req.Headers {
		hreq.Header.Set(k, expandTemplate(v, args, env.Workspace, false))
	}
	resp, err := HTTPClient.Do(hreq)
	if err != nil {
		return CallResult{}, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	text := string(raw)
	if len(text) > MaxResultBytes {
		text = text[:MaxResultBytes] + "\n…（响应过长，已截断）"
	}
	return CallResult{Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, text)}, nil
}

func (m *Manager) runCommand(ctx context.Context, mf *Manifest, t PluginTool, args map[string]any, env Env) (CallResult, error) {
	if !env.AllowCommands {
		return CallResult{}, fmt.Errorf("当前权限不允许执行插件命令")
	}
	if err := checkCommandPermission(mf); err != nil {
		return CallResult{}, err
	}
	cmdline := expandTemplate(t.Command, args, env.Workspace, false)
	timeout := time.Duration(t.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	runner := m.Runner
	if runner == nil {
		runner = CommandFunc(defaultRunner)
	}
	text, err := runner.Run(ctx, cmdline, env.Workspace, timeout)
	if err != nil {
		if strings.TrimSpace(text) != "" {
			return CallResult{Text: text, IsError: true}, nil
		}
		return CallResult{}, err
	}
	return CallResult{Text: text}, nil
}
