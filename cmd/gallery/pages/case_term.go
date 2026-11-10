// guis — gallery 页面：伪终端（xterm 风格 VT/ANSI 终端组件）
//
// 演示内容：
//   - 启动本地 shell（Windows ConPTY / Linux ptmx），真终端行为
//   - 键盘输入、颜色、宽字符（中文）、光标、Ctrl+Shift+C/V 复制粘贴
//   - 回看滚动（滚轮 / Shift+PageUp / Shift+PageDown）、选区拖选
//   - 字号 / 配色切换、清屏与清回看
package pages

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"guis/ui"
)

var (
	termMu         sync.Mutex
	termState      *ui.TermState
	termProc       *ui.TermProc
	termStatus     = "未启动：点「启动 shell」运行本地命令"
	termFontPx     float32 = 13
	termAutoThm            = true
	termNeedFocus          = true
	// termEchoMode 没有可用 PTY（或 PTY 不产出）时把键盘输入本地回显，
	// 至少让用户看得到「能输入」，而不是一片死黑。
	termEchoMode = true
	termEchoLine []rune
	termEchoEsc  bool
	// termStartTried 页面首次进入自动拉起 shell（避免用户以为“没反应”，也让输入能立刻用）。
	termStartTried bool
)

// termWelcome 首次进入页面时的示例输出（未启动 shell 也能看到渲染效果）。
const termWelcome = "\x1b[38;2;120;170;255m╭──────────────────────────────────────────────╮\r\n" +
	"\x1b[0m│  guis 伪终端 · VT/ANSI 终端组件                \x1b[38;2;120;170;255m│\r\n" +
	"\x1b[0m╰──────────────────────────────────────────────╯\x1b[0m\r\n" +
	"\r\n" +
	"  支持：16/256/真彩色 · 粗体/斜体/下划线/反显 · 备用屏 · 滚动区\r\n" +
	"        DECAWM 自动换行 · CJK 宽字符 · 回看缓冲 · 选区复制\r\n" +
	"\r\n" +
	"  \x1b[32m●\x1b[0m 点上方「启动 shell」连接本地 PowerShell / bash\r\n" +
	"  \x1b[33m●\x1b[0m 快捷键：Ctrl+Shift+C 复制　Ctrl+Shift+V 粘贴\r\n" +
	"  \x1b[33m●\x1b[0m 翻历史：滚轮　Shift+PageUp / Shift+PageDown\r\n" +
	"\r\n" +
	"  \x1b[90m# 也可以只当渲染组件用：term.Feed(bytes) 喂任意数据流\x1b[0m\r\n"

// ensureTerm 懒建终端状态（每帧构建时复用同一实例）。
func ensureTerm() *ui.TermState {
	termMu.Lock()
	defer termMu.Unlock()
	if termState == nil {
		t := ui.NewTermState("gallery-term")
		t.FontSize = termFontPx
		t.ScrollMax = 5000
		t.Theme = ui.DefaultTermTheme()
		t.OnContextMenu = func(x, y float32) {
			ui.Toast("终端右键：Ctrl+Shift+C 复制 / Ctrl+Shift+V 粘贴")
		}
		// 默认就把键盘输入接入本地回显：没启 PTY 时也能看到键入内容
		t.OnData = func(b []byte) { termEchoLocal(t, b) }
		t.Feed([]byte(termWelcome))
		t.Feed(termColorDemo())
		termPrompt(t)
		termState = t
	}
	return termState
}

// termEchoLocal 本地回显：把键盘字节渲染成可见的行编辑（Enter/退格/Ctrl+C/方向键）。
func termEchoLocal(t *ui.TermState, b []byte) {
	for _, r := range string(b) {
		if termEchoEsc {
			if r >= 0x40 && r <= 0x7e {
				termEchoEsc = false
			}
			continue
		}
		switch r {
		case 0x1b:
			termEchoEsc = true
		case '\r', '\n':
			t.Feed([]byte("\r\n\x1b[38;5;40m$\x1b[0m "))
			termEchoLine = termEchoLine[:0]
		case 0x7f, 0x08:
			if len(termEchoLine) > 0 {
				termEchoLine = termEchoLine[:len(termEchoLine)-1]
				t.Feed([]byte("\b \b"))
			}
		case 0x03:
			t.Feed([]byte("^C\r\n\x1b[38;5;40m$\x1b[0m "))
			termEchoLine = termEchoLine[:0]
		case 0x04:
			t.Feed([]byte("^D\r\n"))
		default:
			if r >= 0x20 {
				termEchoLine = append(termEchoLine, r)
				t.Feed([]byte(string(r)))
			}
		}
	}
}

// termColorDemo 生成一段 SGR 颜色样例：16 色块、256 色、真彩、文本样式。
// 用来直观验证「终端输出的颜色码」是否真的渲染成了颜色。
func termColorDemo() []byte {
	var sb strings.Builder
	sb.WriteString("\r\n\x1b[1m颜色渲染样例\x1b[0m\r\n")

	sb.WriteString("  16 色  ")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&sb, "\x1b[4%dm   \x1b[0m", i)
	}
	sb.WriteString("  ")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&sb, "\x1b[10%dm   \x1b[0m", i)
	}
	sb.WriteString("\r\n")

	sb.WriteString("  256 色 ")
	for i := 16; i < 52; i += 3 {
		fmt.Fprintf(&sb, "\x1b[48;5;%dm  \x1b[0m", i)
	}
	sb.WriteString("\r\n")

	sb.WriteString("  真彩   ")
	for _, c := range [][3]int{{255, 92, 92}, {92, 255, 140}, {92, 160, 255}, {255, 200, 80}, {200, 120, 255}} {
		fmt.Fprintf(&sb, "\x1b[48;2;%d;%d;%dm   \x1b[0m", c[0], c[1], c[2])
	}
	sb.WriteString("\r\n")

	sb.WriteString("  样式   ")
	sb.WriteString("\x1b[1m粗体\x1b[0m \x1b[2m暗淡\x1b[0m \x1b[3m斜体\x1b[0m ") 
	sb.WriteString("\x1b[4m下划线\x1b[0m \x1b[7m反显\x1b[0m \x1b[9m删除线\x1b[0m")
	sb.WriteString("\r\n")

	sb.WriteString("  前景   ")
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&sb, "\x1b[1;%dm#\x1b[0m", 30+i%8)
	}
	sb.WriteString("\r\n")
	return []byte(sb.String())
}

// promptOf 本地回显模式下的提示符（首次进入时写一行）。
func termPrompt(t *ui.TermState) {
	t.Feed([]byte("\x1b[38;5;40m$\x1b[0m "))
}

// termFocus 把键盘焦点交给终端（点按钮后焦点会被清空，必须显式抢回来）。
func termFocus(t *ui.TermState) { ui.SetFocus(t.Key) }

// termStartPTY 启动本地 shell。
func termStartPTY() {
	t := ensureTerm()
	termFocus(t)
	termMu.Lock()
	busy := termProc != nil
	termMu.Unlock()
	if busy {
		ui.Toast("shell 已在运行")
		return
	}
	proc, err := ui.StartShell(t, ui.ShellOptions{
		OnExit: func(code int) {
			termMu.Lock()
			termStatus = fmt.Sprintf("shell 已退出（退出码 %d）", code)
			termProc = nil
			termMu.Unlock()
			ui.RequestRedraw()
		},
	})
	termMu.Lock()
	baseOut := t.OutputCount()
	if err != nil {
		termStatus = "启动失败：" + err.Error()
	} else {
		termProc = proc
		termStatus = fmt.Sprintf("已连接本地 shell（PID %d）", proc.PID())
		// 真实 PTY：输入写管道，终端自己回显；1.5s 内没有任何输出则退回本地回显
		termEchoMode = false
		t.OnData = func(b []byte) {
			_, _ = proc.Write(b)
			termMu.Lock()
			echo := termEchoMode
			termMu.Unlock()
			if echo {
				termEchoLocal(t, b)
			}
		}
	}
	termMu.Unlock()
	ui.RequestRedraw()
	// 启动完立即把焦点还给终端，否则用户直接敲字会被按钮的“清空焦点”吃掉。
	termFocus(t)
	if err == nil {
		goSafeEcho(t, baseOut)
	}
}

// goSafeEcho PTY 启动后检查是否真的有输出；本机 ConPTY 不产出时退回本地回显。
// base 是启动前的输出字节数：终端里已经有欢迎文本，不能拿「屏幕非空」判断。
func goSafeEcho(t *ui.TermState, base uint64) {
	go func() {
		time.Sleep(1500 * time.Millisecond)
		if t.OutputCount() != base {
			return
		}
		termMu.Lock()
		if termProc == nil {
			termMu.Unlock()
			return
		}
		termEchoMode = true
		termStatus = "ConPTY 无输出，已切本地回显（按键可见，命令不执行）"
		termMu.Unlock()
		t.Feed([]byte("\r\n\x1b[33m[提示] 当前环境 ConPTY 未产生输出，已启用本地回显演示（按键能看到，命令不会真正执行）。\x1b[0m\r\n"))
		termPrompt(t)
		ui.RequestRedraw()
	}()
}

// termStopPTY 结束 shell。
func termStopPTY() {
	termMu.Lock()
	proc := termProc
	termProc = nil
	termMu.Unlock()
	defer termFocus(ensureTerm())
	if proc != nil {
		_ = proc.Close()
		termMu.Lock()
		termStatus = "shell 已停止"
		termMu.Unlock()
	}
	ui.RequestRedraw()
}

// termCase 伪终端演示页。
func termCase() *ui.Node {
	t := ensureTerm()
	// 焦点保持：从菜单/按钮回来时 focusKey 会被清空，这里在「无人持有焦点」时接管。
	// 不每帧强制覆盖，避免抢走输入框的焦点。
	if fk := ui.FocusedKey(); fk == "" || fk == t.Key {
		if fk == "" {
			ui.SetFocus(t.Key)
		}
	} else if fk == "term-start-btn" {
		ui.SetFocus(t.Key)
	}
	if termNeedFocus {
		termNeedFocus = false
		termFocus(t)
	}
	// 首次进入自动启动一次本地 shell（后台进行，不阻塞首帧绘制）。
	if !termStartTried {
		termStartTried = true
		go termStartPTY()
	}
	if termAutoThm {
		t.Theme = ui.DefaultTermTheme()
	}
	t.FontSize = termFontPx

	head := pageHead("伪终端",
		"xterm 风格 VT/ANSI 终端 · 内置 ConPTY(pmx) 本地 shell · 回看 / 选区 / 复制粘贴")

	// 工具行
	toolbar := ui.Row(
		ui.Btn("启动 shell", ui.BtnPrimary, termStartPTY),
		ui.Btn("停止", ui.BtnSecondary, termStopPTY),
		ui.Btn("清屏", ui.BtnGhost, func() {
			t.Clear()
			t.ScrollBottom()
		}),
		ui.Btn("清回看", ui.BtnGhost, func() { t.ClearScrollback() }),
		ui.Btn("A-", ui.BtnGhost, func() {
			if termFontPx > 9 {
				termFontPx--
			}
			ui.RequestRedraw()
		}),
		ui.Btn("A+", ui.BtnGhost, func() {
			if termFontPx < 24 {
				termFontPx++
			}
			ui.RequestRedraw()
		}),
		ui.Spacer(),
		ui.Text(termThemeLabel()).St(ui.St().Text(11, 500, ui.ColTextMuted)),
	).St(ui.St().GapOf(8)).AlignTo(ui.AlignCenter)

	termNode := ui.Terminal(t).St(ui.St().Wp(100).GrowTo(1).MinHeight(240))
	termBox := ui.Box(termNode).St(ui.St().Wp(100).GrowTo(1).R(10).
		Stroke(1, ui.ColStroke).Clipped())

	cols, rows := t.Size()
	status := ui.Row(
		ui.Text(fmt.Sprintf("%d×%d", cols, rows)).St(ui.St().Text(11, 500, ui.ColTextMuted)),
		ui.Text("·").St(ui.St().Text(11, 400, ui.ColTextMuted)),
		ui.Text(termStatusLine()).St(ui.St().Text(11, 500, ui.ColPrimary)).Grow(1),
		ui.Text("Ctrl+Shift+C/V 复制粘贴 · Shift+PageUp/PageDown 翻历史").
			St(ui.St().Text(10.5, 400, ui.ColTextMuted)),
	).St(ui.St().GapOf(8)).AlignTo(ui.AlignCenter)

	return ui.Column(head, toolbar, termBox, status).
		St(ui.St().GapOf(10).Padding(18).Hp(100))
}

// termThemeLabel 主题标签（点一下切深/浅终端配色）。
func termThemeLabel() string {
	if termAutoThm {
		return "配色：跟随界面"
	}
	return "配色：自定义"
}

// termStatusLine 状态栏文案。
func termStatusLine() string {
	termMu.Lock()
	msg := termStatus
	running := termProc != nil
	termMu.Unlock()
	if running {
		return msg
	}
	return msg
}
