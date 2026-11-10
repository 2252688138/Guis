// guis — 伪终端组件（四）：本地 shell 进程封装（平台无关部分）
//
//	term := ui.NewTermState("shell")
//	proc, err := ui.StartShell(term, ui.ShellOptions{})
//	defer proc.Close()
//	...
//	node := ui.Terminal(term)
//
// 各平台实现：
//   - Windows：ConPTY（CreatePseudoConsole，Win10 1809+），见 term_pty_windows.go
//   - Linux/macOS：/dev/ptmx + fork，见 term_pty_unix.go
//   - 其它（wasm / android / ios）：不支持，见 term_pty_stub.go
//
// 只要渲染不要本地进程时不要调用 StartShell，自己把数据喂给 term.Feed 即可
// （远程 SSH、容器日志、串口、WebSocket 都用同一条路）。
package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// ptyDebug 临时调试（GUIS_PTY_DEBUG=1）。
var ptyDebug = os.Getenv("GUIS_PTY_DEBUG") != ""

// ShellOptions 启动本地 shell 的参数。
type ShellOptions struct {
	// Command 可执行文件；空 = 平台默认（Windows: %COMSPEC%，Unix: $SHELL）。
	Command string
	// Args 参数（Windows 会按命令行规则拼接）。
	Args []string
	// Dir 工作目录；空 = 当前目录。
	Dir string
	// Env 追加的环境变量（"K=V" 形式），会叠加在系统环境之上。
	Env []string
	// Cols / Rows 初始尺寸；0 表示取终端状态当前尺寸。
	Cols, Rows int
	// OnExit 进程退出回调（在后台 goroutine 中执行）。
	OnExit func(code int)
}

// ErrPTYUnsupported 当前平台不支持本地伪终端。
var ErrPTYUnsupported = errors.New("ui: 当前平台不支持本地伪终端")

// ErrPTYClosed 终端进程已关闭。
var ErrPTYClosed = errors.New("ui: 终端进程已关闭")

// TermProc 已启动的终端进程（PTY 主端）。
type TermProc struct {
	term *TermState

	mu     sync.Mutex
	in     io.WriteCloser
	out    io.ReadCloser
	closed bool
	code   int

	// sessionID 后端会话标识（pty_server.dll 用）。
	sessionID string
	// writeFn 自定义写入通道（DLL 后端：pty_exec write）。
	writeFn func([]byte) (int, error)
	// setFeed 注册输出接收器（DLL 后端：把输出事件推给终端）。
	setFeed func(func([]byte))

	resizeFn func(cols, rows int) error
	closeFn  func() error
	waitFn   func() (int, error)
	pidFn    func() int

	waitOnce sync.Once
	waitErr  error
}

// Write 把键盘输入写进 PTY（实现 io.Writer）。
func (p *TermProc) Write(b []byte) (int, error) {
	if p == nil {
		return 0, ErrPTYClosed
	}
	p.mu.Lock()
	in, closed := p.in, p.closed
	writeFn := p.writeFn
	p.mu.Unlock()
	if closed {
		return 0, ErrPTYClosed
	}
	if writeFn != nil {
		return writeFn(b)
	}
	if in == nil {
		return 0, ErrPTYClosed
	}
	return in.Write(b)
}

// Resize 调整 PTY 窗口尺寸（列 × 行）。
func (p *TermProc) Resize(cols, rows int) error {
	if p == nil {
		return ErrPTYClosed
	}
	p.mu.Lock()
	closed := p.closed
	fn := p.resizeFn
	p.mu.Unlock()
	if closed {
		return ErrPTYClosed
	}
	if fn == nil {
		return nil
	}
	return fn(cols, rows)
}

// PID 子进程 ID（未知返回 0）。
func (p *TermProc) PID() int {
	if p == nil || p.pidFn == nil {
		return 0
	}
	return p.pidFn()
}

// Close 结束终端进程并释放资源（可重复调用）。
func (p *TermProc) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	in, out := p.in, p.out
	closeFn := p.closeFn
	p.mu.Unlock()

	if closeFn != nil {
		closeFn()
	}
	if in != nil {
		in.Close()
	}
	if out != nil {
		out.Close()
	}
	return nil
}

// Wait 等待进程退出，返回退出码。
func (p *TermProc) Wait() (int, error) {
	if p == nil {
		return -1, ErrPTYClosed
	}
	p.waitOnce.Do(func() {
		if p.waitFn != nil {
			p.code, p.waitErr = p.waitFn()
		}
	})
	return p.code, p.waitErr
}

// StartShell 启动本地 shell 并接到终端上：
//   - 终端键盘输出 → PTY 输入
//   - PTY 输出 → 终端网格
//   - 终端尺寸变化 → PTY Resize
//
// 返回的进程需要由调用方 Close（或在退出回调里处理）。
func StartShell(term *TermState, opts ShellOptions) (*TermProc, error) {
	if term == nil {
		return nil, errors.New("ui: StartShell 需要非空终端状态")
	}
	term.ensure()
	cols, rows := term.Size()
	if opts.Cols > 0 {
		cols = opts.Cols
	}
	if opts.Rows > 0 {
		rows = opts.Rows
	}
	proc, err := startPTY(cols, rows, opts)
	if err != nil {
		return nil, err
	}
	proc.term = term

	term.OnData = func(b []byte) {
		_, _ = proc.Write(b)
	}
	term.OnResize = func(c, r int) {
		_ = proc.Resize(c, r)
	}

	// DLL 后端：输出由 pty_poll_event 轮询分发，不需要读协程。
	if proc.setFeed != nil {
		proc.setFeed(term.Feed)
		return proc, nil
	}
	if proc.out == nil {
		return proc, nil
	}

	// 读协程：PTY 输出 → VT 解析。goSafe 兜住 panic，避免一个坏序列杀掉进程。
	goSafe("term-pty-read", func() {
		defer func() {
			code, _ := proc.Wait()
			if opts.OnExit != nil {
				opts.OnExit(code)
			}
			RequestRedraw()
		}()
		buf := make([]byte, 32*1024)
		for {
			n, rerr := proc.out.Read(buf)
			if ptyDebug {
				fmt.Fprintf(os.Stderr, "[pty] read n=%d err=%v\n", n, rerr)
			}
			if n > 0 {
				term.Feed(buf[:n])
			}
			if rerr != nil {
				return
			}
		}
	})
	return proc, nil
}
