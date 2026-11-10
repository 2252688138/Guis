package ui

import (
	"fmt"
	"testing"
)

// TestCodeEditorLSPLogBuffer 服务端日志只进缓冲，不灌进状态栏（gopls 日志是多行的）。
func TestCodeEditorLSPLogBuffer(t *testing.T) {
	e := NewCodeEditorState("lsp-log", "package main")
	status := ""
	e.OnStatus = func(m string) { status = m }

	e.appendLSPLog("Loading packages...\nview_id=\"1\"\troot_dir=\"/x\"")
	logs := e.LSPLog()
	if len(logs) != 1 {
		t.Fatalf("多行日志应压成一条，实际 %d 条：%v", len(logs), logs)
	}
	if logs[0] != `Loading packages... view_id="1" root_dir="/x"` {
		t.Fatalf("日志单行化结果不对：%q", logs[0])
	}

	// 超过上限只保留最新
	for i := 0; i < codeLSPLogMax+15; i++ {
		e.appendLSPLog(fmt.Sprintf("line-%d", i))
	}
	if n := len(e.LSPLog()); n != codeLSPLogMax {
		t.Fatalf("日志缓冲应截到 %d 条，实际 %d", codeLSPLogMax, n)
	}

	// 走 drainLSP 的日志投递不应改状态栏（只有 lspError 会提示）
	pushLSP(lspPending{kind: lspLogLine, msg: "go/packages.Load #1\nduration=1ms"})
	e.drainLSP()
	if status != "" {
		t.Fatalf("服务端日志不应写状态栏，实际写成：%q", status)
	}
	if got := e.LSPLog(); got[len(got)-1] != "go/packages.Load #1 duration=1ms" {
		t.Fatalf("日志未入缓冲：%q", got[len(got)-1])
	}

	// 连接层错误要提示
	pushLSP(lspPending{kind: lspError, msg: "gopls 读取结束：EOF"})
	e.drainLSP()
	if status != "gopls 读取结束：EOF" {
		t.Fatalf("连接层错误应写状态栏，实际：%q", status)
	}
}

// TestLSPConfigForItems workspace/configuration 必须按 items 数量逐项应答。
func TestLSPConfigForItems(t *testing.T) {
	c := NewLSPClient("gopls", "gopls")
	items := c.configForItems([]byte(`{"items":[{"section":"gopls"},{"section":"analyses"},{"section":"ui"}]}`))
	if len(items) != 3 {
		t.Fatalf("应回 3 项配置，实际 %d", len(items))
	}
	if v, ok := items[0]["semanticTokens"].(bool); !ok || !v {
		t.Fatalf("默认配置应打开 semanticTokens：%v", items[0])
	}
	// 缺省 / 非法 params 时至少回一项
	if n := len(c.configForItems(nil)); n != 1 {
		t.Fatalf("无参请求应回 1 项，实际 %d", n)
	}
}
