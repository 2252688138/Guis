// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package webcdp

// 用系统浏览器加载网页的 CDP 通道（零第三方依赖）。
//
// 思路：启动系统的 Chrome / Chromium / Edge（headless=new + --remote-debugging-port），
// 通过 DevTools 协议真正加载页面，再按需取帧（Page.captureScreenshot）与派发输入
// （Input.dispatch*）。相比自绘网页，这套方案：
//   - 用的就是系统浏览器内核（网页行为、字体、编解码都与本机浏览器一致）；
//   - 纯 Go、无 cgo、无插件，Linux / macOS / Windows 同一份代码；
//   - 截图是普通纹理，可以被自绘裁剪、圆角、被浮层覆盖。
// 代价是帧率受截图限制（建议 10~20fps）与更高的 CPU 占用。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNoBrowser 本机没有找到可用的 Chromium 系浏览器。
var ErrNoBrowser = errors.New("webcdp: 未找到可用的系统浏览器（Chrome / Chromium / Edge / Brave）")

// Config 启动配置。
type Config struct {
	// BrowserPath 浏览器可执行文件路径（空 = 自动探测）。
	BrowserPath string
	// UserDataDir 用户数据目录（空 = 每个实例独立临时目录；多实例不能共用同一个目录）。
	UserDataDir string
	// Width / Height 初始视口（CSS 像素）。
	Width, Height int
	// Zoom 初始缩放（会折算进 deviceScaleFactor）。
	Zoom float64
	// URL 初始地址（空 = about:blank）。
	URL string
	// Headless 无头模式（默认 true：离屏取帧时不应该弹出浏览器窗口）。
	Headless bool
	// NoHeadless 显式要求带窗口启动（调试用）。
	ShowWindow bool
	// Muted 启动时静音。
	Muted bool
	// UserAgent 自定义 UA。
	UserAgent string
	// ExtraArgs 追加命令行参数。
	ExtraArgs []string
}

// browserCandidates 各平台常见的 Chromium 系浏览器可执行文件。
var browserCandidates = map[string][]string{
	"windows": {
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
	},
	"darwin": {
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
	},
	"linux": {
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/usr/bin/microsoft-edge",
		"/usr/bin/brave-browser",
		"/snap/bin/chromium",
	},
}

// browserNames PATH 里依次尝试的可执行名。
var browserNames = []string{
	"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
	"chrome", "microsoft-edge", "microsoft-edge-stable", "msedge", "brave-browser",
}

// LocateBrowser 探测可用的 Chromium 系浏览器（空串 = 没找到）。
func LocateBrowser() string {
	if p := strings.TrimSpace(os.Getenv("GUIS_BROWSER_PATH")); p != "" {
		if fileExists(p) {
			return p
		}
	}
	for _, p := range browserCandidates[runtime.GOOS] {
		if fileExists(p) {
			return p
		}
	}
	for _, name := range browserNames {
		if p, err := exec.LookPath(name); err == nil && p != "" {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// Session 一个 CDP 会话（单标签页、单连接）。
type Session struct {
	ws      *wsConn
	writeMu sync.Mutex
	mu      sync.Mutex
	idSeq   int64
	pending map[int64]chan cdpReply
	events  map[string][]func(json.RawMessage)
	closed  atomic.Bool

	browser *Browser
}

type cdpReply struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Browser 受控的浏览器进程。
type Browser struct {
	cfg Config
	cmd *exec.Cmd
	dir string
}

// Canvas 尺寸变化回调（可选）。
func (c Config) viewport() (int, int) {
	w, h := c.Width, c.Height
	if w <= 0 {
		w = 1024
	}
	if h <= 0 {
		h = 768
	}
	return w, h
}

// Launch 启动系统浏览器并连接 CDP，返回会话。
func Launch(cfg Config) (*Session, error) {
	bin := strings.TrimSpace(cfg.BrowserPath)
	if bin == "" {
		bin = LocateBrowser()
	}
	if bin == "" {
		return nil, ErrNoBrowser
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	dir := strings.TrimSpace(cfg.UserDataDir)
	tmpDir := ""
	if dir == "" {
		base, err := os.MkdirTemp("", "guis-webcdp-*")
		if err != nil {
			return nil, err
		}
		dir, tmpDir = base, base
	}
	w, h := cfg.viewport()
	target := strings.TrimSpace(cfg.URL)
	if target == "" {
		target = "about:blank"
	}

	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--user-data-dir=" + dir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-features=Translate,MediaRouter",
		"--disable-background-networking",
		"--disable-sync",
		"--hide-scrollbars",
		"--mute-audio=" + boolStr(cfg.Muted),
		"--window-size=" + strconv.Itoa(w) + "," + strconv.Itoa(h),
		"--app=" + target,
	}
	if !cfg.ShowWindow {
		args = append(args, "--headless=new")
	}
	args = append(args, cfg.ExtraArgs...)

	cmd := exec.Command(bin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
		return nil, fmt.Errorf("webcdp: 启动浏览器失败：%w", err)
	}
	b := &Browser{cfg: cfg, cmd: cmd, dir: tmpDir}

	wsURL, err := waitTarget(context.Background(), port, 12*time.Second)
	if err != nil {
		b.Close()
		return nil, err
	}
	conn, err := wsDial(wsURL, 8*time.Second)
	if err != nil {
		b.Close()
		return nil, err
	}
	s := &Session{
		ws:      conn,
		pending: map[int64]chan cdpReply{},
		events:  map[string][]func(json.RawMessage){},
		browser: b,
	}
	go s.readLoop()

	// 打开基础域（事件 + 脚本 + 网络 UA 覆盖）
	for _, m := range []string{"Page.enable", "Runtime.enable", "Network.enable"} {
		_, _ = s.Call(m, nil)
	}
	if cfg.UserAgent != "" {
		_ = s.Page().SetUserAgent(cfg.UserAgent)
	}
	_ = s.Page().SetViewport(w, h, 1)
	// 先提交一次文档：headless 下若从未导航过，Page.setDocumentContent 会一直等不到主框架。
	if target == "about:blank" {
		_ = s.Page().Navigate("about:blank")
	}
	return s, nil
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// freePort 取一个空闲 TCP 端口。
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("webcdp: 取不到本地端口")
	}
	return addr.Port, nil
}

// waitTarget 等 DevTools HTTP 端点就绪，并返回页面的 WebSocket 地址。
func waitTarget(ctx context.Context, port int, timeout time.Duration) (string, error) {
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/json/list")
		if err == nil {
			buf, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			var list []struct {
				Type                   string `json:"type"`
				WebSocketDebuggerURL   string `json:"webSocketDebuggerUrl"`
				URL                    string `json:"url"`
			}
			if json.Unmarshal(buf, &list) == nil {
				for _, t := range list {
					if t.Type == "page" && t.WebSocketDebuggerURL != "" {
						return t.WebSocketDebuggerURL, nil
					}
				}
				if len(list) > 0 && list[0].WebSocketDebuggerURL != "" {
					return list[0].WebSocketDebuggerURL, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(120 * time.Millisecond):
		}
	}
	return "", errors.New("webcdp: 浏览器 DevTools 端点未就绪")
}

// readLoop 读线程：分发回复与事件。
func (s *Session) readLoop() {
	for {
		text, err := s.ws.ReadText()
		if err != nil {
			s.closed.Store(true)
			s.mu.Lock()
			for id, ch := range s.pending {
				close(ch)
				delete(s.pending, id)
			}
			s.mu.Unlock()
			return
		}
		var rep cdpReply
		if json.Unmarshal([]byte(text), &rep) != nil {
			continue
		}
		if rep.ID != 0 {
			s.mu.Lock()
			ch := s.pending[rep.ID]
			delete(s.pending, rep.ID)
			s.mu.Unlock()
			if ch != nil {
				ch <- rep
			}
			continue
		}
		if rep.Method == "" {
			continue
		}
		s.mu.Lock()
		fns := append([]func(json.RawMessage){}, s.events[rep.Method]...)
		s.mu.Unlock()
		for _, fn := range fns {
			fn(rep.Params)
		}
	}
}

// Call 发送一条 CDP 命令并等待结果。
func (s *Session) Call(method string, params any) (json.RawMessage, error) {
	return s.CallTimeout(method, params, 20*time.Second)
}

// CallTimeout 带超时的 CDP 命令。
func (s *Session) CallTimeout(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	if s == nil || s.closed.Load() {
		return nil, errors.New("webcdp: 会话已关闭")
	}
	id := atomic.AddInt64(&s.idSeq, 1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	buf, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	ch := make(chan cdpReply, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()

	s.writeMu.Lock()
	err = s.ws.WriteText(string(buf))
	s.writeMu.Unlock()
	if err != nil {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return nil, err
	}

	select {
	case rep, ok := <-ch:
		if !ok {
			return nil, errors.New("webcdp: 连接已断开")
		}
		if rep.Error != nil {
			return nil, fmt.Errorf("webcdp: %s 失败：%s", method, rep.Error.Message)
		}
		return rep.Result, nil
	case <-time.After(timeout):
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return nil, fmt.Errorf("webcdp: %s 超时", method)
	}
}

// On 订阅 CDP 事件。
func (s *Session) On(method string, fn func(params json.RawMessage)) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	s.events[method] = append(s.events[method], fn)
	s.mu.Unlock()
}

// Close 关闭会话并结束浏览器进程。
func (s *Session) Close() {
	if s == nil {
		return
	}
	if s.closed.CompareAndSwap(false, true) {
		_ = s.ws.Close()
	}
	if s.browser != nil {
		s.browser.Close()
	}
}

// Close 结束浏览器进程并清理临时目录。
func (b *Browser) Close() {
	if b == nil {
		return
	}
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
		_, _ = b.cmd.Process.Wait()
	}
	if b.dir != "" {
		_ = os.RemoveAll(b.dir)
	}
	b.cmd = nil
	b.dir = ""
}

// Pid 浏览器进程号（0 = 未启动）。真窗口嵌入靠它反查 X11 窗口。
func (b *Browser) Pid() int {
	if b == nil || b.cmd == nil || b.cmd.Process == nil {
		return 0
	}
	return b.cmd.Process.Pid
}

// Browser 取得会话所属的浏览器进程。
func (s *Session) Browser() *Browser {
	if s == nil {
		return nil
	}
	return s.browser
}

// Dir 实例数据目录（空 = 调用方指定的持久目录）。
func (b *Browser) Dir() string {
	if b == nil {
		return ""
	}
	return filepath.Clean(b.dir)
}
