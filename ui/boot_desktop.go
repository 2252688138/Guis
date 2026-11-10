//go:build !js

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"os"
)

// prepareRun 桌面 / 移动端启动前没有额外准备：字体直接用系统字体目录（见 internal/text/fontplat_*）。
func prepareRun(opts *Options) {
	// 未捕获 panic / fatal error 写盘：否则后台 goroutine 崩了只剩“exit status 2”。
	enableCrashLog()
	startCPUProfileIfNeeded()
}

// reportPanic 输出 Run 内部捕获到的 panic。
//
// 旧实现是空实现：runLoop 里的 panic 被 recover 后既不打印也不上报，
// 进程「静默退出（exit 0）」，使用者只看到窗口消失 / 黑屏，无法定位。
// 现在统一写到 stderr（桌面端带控制台或重定向日志时可见），
// 消息里已包含 panic 值与 goroutine 堆栈（见 Run 里的 debug.Stack）。
func reportPanic(msg string) {
	fmt.Fprintf(os.Stderr, "[guis] 运行期 panic（界面已停止）：\n%s\n", msg)
	// GUI 子系统（windowsgui / .app）下 stderr 没有人看得见：只写日志等于
	// 「双击后窗口一闪就没了」。这里补一次系统原生消息框（不依赖 GL / 窗口，
	// 因此在初始化失败时同样有效）。CI / 自动化可用 GUIS_NO_MSGBOX=1 关掉。
	if os.Getenv("GUIS_NO_MSGBOX") == "" {
		body := msg
		if len(body) > 1200 {
			body = body[:1200] + "\n…（完整堆栈见崩溃日志）"
		}
		MessageBoxEx("程序发生错误", body, MsgError, MsgBoxOK)
	}
}
