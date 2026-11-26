package plugins

import (
	"context"
	"strings"
	"testing"
	"time"
)

func newTestHost(t *testing.T, mode string, tweak func(*RuntimeSpec)) (*Host, *Manifest) {
	t.Helper()
	dir := t.TempDir()
	mf := writeFakePlugin(t, dir, mode, tweak)
	h := NewHost(t.Logf)
	h.Workspace = dir
	h.DataDir = dir
	t.Cleanup(h.Close)
	return h, mf
}

func TestHostEnsureAndCall(t *testing.T) {
	h, mf := newTestHost(t, "ok", nil)
	ctx := context.Background()
	if !h.Running(mf.ID) { // 尚未启动
		if _, err := h.Ensure(ctx, mf); err != nil {
			t.Fatalf("启动失败: %v", err)
		}
	}
	if !h.Running(mf.ID) {
		t.Fatal("启动后 Running 应为 true")
	}
	res, err := h.Call(ctx, mf, Env{}, "echo", map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res.Text != "hello" || res.IsError {
		t.Fatalf("结果异常: %+v", res)
	}
}

func TestHostToolFiltering(t *testing.T) {
	h, mf := newTestHost(t, "ok", nil)
	tools, err := h.Tools(context.Background(), mf)
	if err != nil {
		t.Fatalf("取工具失败: %v", err)
	}
	var names []string
	for _, td := range tools {
		names = append(names, td.Name)
	}
	if len(tools) != 2 {
		t.Fatalf("应只剩 echo/fail 两个合法工具，实际: %v", names)
	}
	for _, td := range tools {
		if td.Name != "echo" && td.Name != "fail" {
			t.Fatalf("出现了不该有的工具: %v", names)
		}
	}
}

func TestHostToolErrorFlag(t *testing.T) {
	h, mf := newTestHost(t, "ok", nil)
	res, err := h.Call(context.Background(), mf, Env{}, "fail", nil)
	if err != nil {
		t.Fatalf("isError 属于正常响应，不应返回错误: %v", err)
	}
	if !res.IsError || res.Text != "boom" {
		t.Fatalf("结果异常: %+v", res)
	}
}

func TestHostCallTimeout(t *testing.T) {
	h, mf := newTestHost(t, "slow", func(rt *RuntimeSpec) { rt.CallTimeoutMs = 200 })
	_, err := h.Call(context.Background(), mf, Env{}, "echo", map[string]any{"text": "x"})
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("应超时，实际: %v", err)
	}
	if h.Running(mf.ID) {
		t.Fatal("超时后应强杀插件进程")
	}
}

func TestHostCrashRestartBudget(t *testing.T) {
	h, mf := newTestHost(t, "crash", func(rt *RuntimeSpec) { rt.StartTimeoutMs = 2000 })
	var lastErr error
	limitHit := false
	for i := 0; i < maxRestarts+2; i++ {
		_, lastErr = h.Call(context.Background(), mf, Env{}, "echo", map[string]any{"text": "x"})
		if lastErr != nil && strings.Contains(lastErr.Error(), "重启上限") {
			limitHit = true
			break
		}
	}
	if !limitHit {
		t.Fatalf("应触发重启上限，最后错误: %v", lastErr)
	}
}

func TestHostBadProtocolOutput(t *testing.T) {
	h, mf := newTestHost(t, "garbage", func(rt *RuntimeSpec) { rt.StartTimeoutMs = 2000 })
	if _, err := h.Ensure(context.Background(), mf); err == nil {
		t.Fatal("非法 JSON 输出应握手失败")
	}
	if h.Running(mf.ID) {
		t.Fatal("握手失败后不应残留进程")
	}
}

func TestHostStderrRing(t *testing.T) {
	h, mf := newTestHost(t, "noisy", nil)
	if _, err := h.Tools(context.Background(), mf); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(h.Logs(mf.ID)) >= stderrLines {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	logs := h.Logs(mf.ID)
	if len(logs) != stderrLines {
		t.Fatalf("stderr 环形缓冲应保留 %d 行，实际 %d 行", stderrLines, len(logs))
	}
	if !strings.Contains(logs[len(logs)-1], "stderr line 299") {
		t.Fatalf("应保留最后一行，实际: %s", logs[len(logs)-1])
	}
}

func TestHostCloseKillsProcess(t *testing.T) {
	h, mf := newTestHost(t, "ok", nil)
	if _, err := h.Ensure(context.Background(), mf); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	h.Close()
	if h.Running(mf.ID) {
		t.Fatal("Close 后不应再有存活进程")
	}
}
