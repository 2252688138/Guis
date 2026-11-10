// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"os"
)

// 原生消息框。
//
// 自绘对话框依赖 GL 上下文与窗口，一旦初始化失败（显卡驱动 / 远程桌面 / GL 版本
// 不足）就什么都弹不出来，用户看到的是「双击后没反应」。这里提供系统原生消息框
// 作为兜底通道：
//
//	ui.MessageBox("无法启动", "初始化 OpenGL 失败，请更新显卡驱动。")
//	if ui.MessageBoxEx("删除", "确定删除该文件？", ui.MsgWarn, ui.MsgBoxYesNo) == ui.MsgResYes { ... }
//
// Windows 走 MessageBoxW，Linux 走 zenity / kdialog，macOS 走 osascript；
// 都不支持时写 stderr 并返回 MsgResOK（不阻塞、不 panic）。

// MsgKind 消息框图标类型。
type MsgKind int

const (
	// MsgInfo 信息（i 图标）。
	MsgInfo MsgKind = iota
	// MsgWarn 警告（! 图标）。
	MsgWarn
	// MsgError 错误（× 图标）。
	MsgError
	// MsgQuestion 询问（? 图标）。
	MsgQuestion
)

// MsgButtons 消息框按钮组合。
type MsgButtons int

const (
	// MsgBoxOK 只有一个「确定」。
	MsgBoxOK MsgButtons = iota
	// MsgBoxOKCancel 确定 / 取消。
	MsgBoxOKCancel
	// MsgBoxYesNo 是 / 否。
	MsgBoxYesNo
	// MsgBoxRetryCancel 重试 / 取消。
	MsgBoxRetryCancel
)

// MsgResult 消息框返回值。
type MsgResult int

const (
	// MsgResOK 「确定」或「是」。
	MsgResOK MsgResult = iota
	// MsgResCancel 「取消」或关闭窗口。
	MsgResCancel
	// MsgResYes 「是」。
	MsgResYes
	// MsgResNo 「否」。
	MsgResNo
)

// messageBoxNative 平台原生实现；ok=false 表示本平台不可用，走 stderr 兜底。
var messageBoxNative func(title, body string, kind MsgKind, buttons MsgButtons) (MsgResult, bool)

// MessageBox 弹出只有一个「确定」的原生消息框（信息级别）。
func MessageBox(title, body string) MsgResult {
	return MessageBoxEx(title, body, MsgInfo, MsgBoxOK)
}

// MessageBoxEx 弹出原生消息框并等待用户选择（模态，返回前阻塞 UI 线程）。
// 非 Windows 端若找不到 zenity / kdialog / osascript，则退化为写 stderr。
func MessageBoxEx(title, body string, kind MsgKind, buttons MsgButtons) MsgResult {
	if messageBoxNative != nil {
		if r, ok := messageBoxNative(title, body, kind, buttons); ok {
			return r
		}
	}
	// 兜底：不阻塞，保证日志里一定能看到内容
	fmt.Fprintf(os.Stderr, "[guis] %s: %s\n", title, body)
	return MsgResOK
}

// MsgButtonTexts 返回某种按钮组合的标签（自绘对话框复用同一套文案）。
func MsgButtonTexts(b MsgButtons) []string {
	switch b {
	case MsgBoxOKCancel:
		return []string{"确定", "取消"}
	case MsgBoxYesNo:
		return []string{"是", "否"}
	case MsgBoxRetryCancel:
		return []string{"重试", "取消"}
	}
	return []string{"确定"}
}
