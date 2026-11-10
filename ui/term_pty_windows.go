//go:build windows

// guis — 伪终端组件（五）：Windows 原生 ConPTY 后端
//
// 写法与 H:\JIMaX\internal\terminal\conpty_windows.go 对齐（那边线上已验证），
// 关键点全部踩过坑：
//
//  1. CreatePseudoConsole 的 flags 传 PSEUDOCONSOLE_INHERIT_CURSOR(1)：
//     PowerShell / python / node 这类交互式程序的首屏提示符依赖它，否则
//     「进程起来了、键盘也写进去了，但屏幕一个字都不出」。
//  2. 属性表里 lpValue 传 &hPC（HPCON 的地址），bInheritHandles 传 true。
//  3. 输出管道只能用同步 ReadFile（winPipe），不能包成 os.File —— Go 会把它
//     挂到 IOCP 上做非阻塞预取，ConPTY 的管道读不到数据。
//  4. pipeInR / pipeOutW 在会话期间不要关（ConPTY 复用这两个句柄），
//     统一在 Close 里释放。
//
// 支持交互式命令：python / node / bash / ssh 等只要是控制台程序，都能拿到
// 真 tty 语义（isatty=真、颜色、行编辑、Ctrl+C 按 0x03 字节送入、窗口尺寸变化）。
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// pseudoConsoleInheritCursor PSEUDOCONSOLE_INHERIT_CURSOR（不要用：本机实测会导致无输出）。
const pseudoConsoleInheritCursor uint32 = 0x1

// procUpdateThreadAttr kernel32!UpdateProcThreadAttribute（原生调用，便于直接传句柄值）。
var procUpdateThreadAttr = windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")

// startfUseStdHandles STARTF_USESTDHANDLES。
const startfUseStdHandles uint32 = 0x00000100

// winPipe 直接包装 Win32 句柄的阻塞读写（绕过 Go runtime poller）。
type winPipe struct {
	r, w   windows.Handle
	mu     sync.Mutex
	closed bool
}

func (p *winPipe) Read(b []byte) (int, error) {
	p.mu.Lock()
	h, closed := p.r, p.closed
	p.mu.Unlock()
	if closed || h == 0 || len(b) == 0 {
		return 0, io.EOF
	}
	var n uint32
	if err := windows.ReadFile(h, b, &n, nil); err != nil {
		return int(n), err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return int(n), nil
}

func (p *winPipe) Write(b []byte) (int, error) {
	p.mu.Lock()
	h, closed := p.w, p.closed
	p.mu.Unlock()
	if closed || h == 0 {
		return 0, ErrPTYClosed
	}
	var n uint32
	if err := windows.WriteFile(h, b, &n, nil); err != nil {
		return int(n), err
	}
	return int(n), nil
}

func (p *winPipe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	if p.w != 0 {
		windows.CloseHandle(p.w)
		p.w = 0
	}
	if p.r != 0 {
		windows.CloseHandle(p.r)
	}
	p.r = 0
	return nil
}

// defaultShell Windows 默认 shell：PowerShell 优先，回退 cmd.exe。
func defaultShell() (string, []string) {
	if p := findPowerShell(); p != "" {
		return p, []string{"-NoLogo"}
	}
	if cs := os.Getenv("COMSPEC"); cs != "" {
		return cs, nil
	}
	return "cmd.exe", nil
}

// termEnvVars 下发给子进程的终端能力环境变量。
//
// 没有 TERM 时 git / ls / node / npm 等会认为「不是终端」而不输出颜色码，
// ConPTY 里就会看到一片白字。这些变量与 xterm.js / VS Code 集成终端一致。
func termEnvVars() []string {
	return []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"TERM_PROGRAM=guis",
		"TERM_PROGRAM_VERSION=1.0",
		"CLICOLOR=1",
		"CLICOLOR_FORCE=1",
		"FORCE_COLOR=1",
		"ConEmuANSI=ON",
	}
}

// buildEnvBlock 合并系统环境 + 额外变量，生成 UTF-16 环境块（双 NUL 结尾）。
// 大小写不敏感覆盖（Windows 环境块语义）。
func buildEnvBlock(extra []string) []uint16 {
	type kv struct{ k, v string }
	order := make([]string, 0, 64)
	val := map[string]string{}
	put := func(s string) {
		i := strings.IndexByte(s, '=')
		if i <= 0 {
			return
		}
		key := strings.ToUpper(s[:i])
		if _, ok := val[key]; !ok {
			order = append(order, key)
		}
		val[key] = s
	}
	for _, s := range os.Environ() {
		put(s)
	}
	for _, s := range extra {
		put(s)
	}
	// NO_COLOR 会让很多程序禁用颜色，强制移除
	delete(val, "NO_COLOR")
	seen := map[string]bool{}
	var out []uint16
	for _, k := range order {
		if k == "NO_COLOR" || seen[k] {
			continue
		}
		seen[k] = true
		u, err := windows.UTF16FromString(val[k])
		if err != nil {
			continue
		}
		out = append(out, u...) // UTF16FromString 已带结尾 NUL
	}
	out = append(out, 0)
	return out
}

// findPowerShell 在 PATH 中查找 pwsh.exe / powershell.exe。
func findPowerShell() string {
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		if p := lookPathWin(name); p != "" {
			return p
		}
	}
	return ""
}

// lookPathWin 在 PATH 中查找可执行文件。
func lookPathWin(name string) string {
	path := os.Getenv("PATH")
	for _, dir := range strings.Split(path, ";") {
		if dir == "" {
			continue
		}
		p := strings.TrimRight(dir, `\`) + `\` + name
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// startPTY 启动原生 ConPTY 会话。
func startPTY(cols, rows int, opts ShellOptions) (*TermProc, error) {
	if cols < 2 {
		cols = 80
	}
	if rows < 2 {
		rows = 24
	}
	// 遵循 MS 官方样例 / pty_server.dll：管道用默认安全属性（不可继承），
	// CreatePseudoConsole 会内部 dup 这些句柄，之后我们关掉 PTY 端。
	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		return nil, fmt.Errorf("ui: 创建输入管道失败：%w", err)
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		windows.CloseHandle(inR)
		windows.CloseHandle(inW)
		return nil, fmt.Errorf("ui: 创建输出管道失败：%w", err)
	}

	// flags 必须为 0：INHERIT_CURSOR(1) 会让首屏不刷新（部分环境下完全无输出）。
	var hpc windows.Handle
	coord := windows.Coord{X: int16(cols), Y: int16(rows)}
	if err := windows.CreatePseudoConsole(coord, inR, outW, 0, &hpc); err != nil {
		for _, h := range []windows.Handle{inR, inW, outR, outW} {
			windows.CloseHandle(h)
		}
		return nil, fmt.Errorf("ui: CreatePseudoConsole 失败：%w", err)
	}

	attr, err := windows.NewProcThreadAttributeList(2)
	if err != nil {
		windows.ClosePseudoConsole(hpc)
		for _, h := range []windows.Handle{inR, inW, outR, outW} {
			windows.CloseHandle(h)
		}
		return nil, fmt.Errorf("ui: 创建属性列表失败：%w", err)
	}
	// lpValue 传 HPCON 值本身（不是 &HPCON）：pty_server.dll / node-pty 都是这么传的，
	// 传地址在本机实测属性不生效（子进程挂到父控制台，管道永远读不到输出）。
	// 这里直接调原生 UpdateProcThreadAttribute（传句柄值），避免 uintptr→unsafe.Pointer
	// 的 vet 告警。
	ok, _, uerr := procUpdateThreadAttr.Call(
		uintptr(unsafe.Pointer(attr.List())), 0, uintptr(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE),
		uintptr(hpc), unsafe.Sizeof(hpc), 0, 0)
	if ok == 0 {
		attr.Delete()
		windows.ClosePseudoConsole(hpc)
		for _, h := range []windows.Handle{inR, inW, outR, outW} {
			windows.CloseHandle(h)
		}
		return nil, fmt.Errorf("ui: 设置伪控制台属性失败：%v", uerr)
	}

	app, args := opts.Command, opts.Args
	if app == "" {
		app, args = defaultShell()
	}
	appName, err := windows.UTF16PtrFromString(app)
	if err != nil {
		attr.Delete()
		windows.ClosePseudoConsole(hpc)
		for _, h := range []windows.Handle{inR, inW, outR, outW} {
			windows.CloseHandle(h)
		}
		return nil, err
	}
	cmdLine := windows.StringToUTF16Ptr(buildCmdLine(app, args))
	var cwdPtr *uint16
	if opts.Dir != "" {
		cwdPtr, _ = windows.UTF16PtrFromString(opts.Dir)
	}
	siEx := &windows.StartupInfoEx{}
	siEx.StartupInfo.Cb = uint32(unsafe.Sizeof(*siEx))
	// 关键（对齐 node-pty / pty_server.dll）：显式声明使用 std 句柄且全置 NULL，
	// 让子进程的 std 落到伪控制台，而不是继承本进程的句柄。
	siEx.StartupInfo.Flags |= startfUseStdHandles
	siEx.ProcThreadAttributeList = attr.List()

	var pi windows.ProcessInformation
	// 环境块：TERM/COLORTERM 等终端能力变量（否则 git/ls/node 不输出颜色码）。
	envBlock := buildEnvBlock(termEnvVars())
	var envPtr *uint16
	if len(envBlock) > 0 {
		envPtr = &envBlock[0]
	}
	cerr := windows.CreateProcess(appName, cmdLine, nil, nil, false,
		windows.EXTENDED_STARTUPINFO_PRESENT|0x00000400, // + CREATE_UNICODE_ENVIRONMENT
		envPtr, cwdPtr, &siEx.StartupInfo, &pi)
	attr.Delete()
	if cerr != nil {
		windows.ClosePseudoConsole(hpc)
		for _, h := range []windows.Handle{inR, inW, outR, outW} {
			windows.CloseHandle(h)
		}
		return nil, fmt.Errorf("ui: 创建终端进程失败：%w", cerr)
	}

	proc := &TermProc{}
	proc.in = &winPipe{w: inW}
	proc.out = &winPipe{r: outR}
	proc.resizeFn = func(c, r int) error {
		if c < 2 {
			c = 2
		}
		if r < 2 {
			r = 2
		}
		return windows.ResizePseudoConsole(hpc, windows.Coord{X: int16(c), Y: int16(r)})
	}
	proc.closeFn = func() error {
		// 先结束进程，再关伪控制台（顺序反了 ClosePseudoConsole 会等 conhost 退出）
		windows.TerminateProcess(pi.Process, 1)
		windows.ClosePseudoConsole(hpc)
		windows.CloseHandle(pi.Thread)
		windows.CloseHandle(pi.Process)
		// ConPTY 会话期间一直保留的两端在这里释放
		windows.CloseHandle(inR)
		windows.CloseHandle(outW)
		return nil
	}
	proc.waitFn = func() (int, error) {
		_, _ = windows.WaitForSingleObject(pi.Process, windows.INFINITE)
		var code uint32
		if err := windows.GetExitCodeProcess(pi.Process, &code); err != nil {
			return -1, err
		}
		return int(code), nil
	}
	proc.pidFn = func() int { return int(pi.ProcessId) }
	return proc, nil
}

// buildCmdLine 拼命令行（引号规则与 CommandLineToArgvW 兼容）。
func buildCmdLine(app string, args []string) string {
	var sb strings.Builder
	sb.WriteString(escapeWinArg(app))
	for _, a := range args {
		sb.WriteByte(' ')
		sb.WriteString(escapeWinArg(a))
	}
	return sb.String()
}

// escapeWinArg 给含空白 / 引号的参数加引号。
func escapeWinArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\n\v\"") {
		return s
	}
	var sb strings.Builder
	sb.WriteByte('"')
	backslashes := 0
	for _, r := range s {
		switch r {
		case '\\':
			backslashes++
			sb.WriteRune(r)
		case '"':
			for i := 0; i < backslashes; i++ {
				sb.WriteByte('\\')
			}
			backslashes = 0
			sb.WriteString(`\"`)
		default:
			backslashes = 0
			sb.WriteRune(r)
		}
	}
	for i := 0; i < backslashes; i++ {
		sb.WriteByte('\\')
	}
	sb.WriteByte('"')
	return sb.String()
}
