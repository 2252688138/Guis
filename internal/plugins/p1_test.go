package plugins

import (
	"context"
	"os"
		"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSanitizeArgsMasksSecrets(t *testing.T) {
	in := map[string]any{
		"text":          "hello",
		"api_key":       "abc123",
		"Authorization": "Bearer xyz",
		"long":          strings.Repeat("a", 500),
	}
	out := sanitizeArgs(in)
	if out["api_key"] != "***" || out["Authorization"] != "***" {
		t.Fatalf("敏感键未脱敏: %+v", out)
	}
	if s, _ := out["long"].(string); len(s) > auditValueMax+4 {
		t.Fatalf("长值未截断（%d 字符）", len(s))
	}
	if out["text"] != "hello" {
		t.Fatalf("普通值不应改变: %+v", out)
	}
}

func TestAuditRotates(t *testing.T) {
	a := newAuditor(t.TempDir())
	big := strings.Repeat("x", 3000)
	for i := 0; i < 2200; i++ {
		a.write(auditEntry{Plugin: "p", Tool: "t", ArgsDigest: map[string]any{"k": big}})
	}
	if _, err := os.Stat(a.Path()); err != nil {
		t.Fatalf("审计文件不存在: %v", err)
	}
	rotated := strings.TrimSuffix(a.Path(), ".jsonl") + ".1.jsonl"
	if _, err := os.Stat(rotated); err != nil {
		t.Fatalf("超过 5MB 应轮转: %v", err)
	}
}

func TestAuditRecordsCalls(t *testing.T) {
	dir := t.TempDir()
	m := New(Options{DataDir: dir, Runner: CommandFunc(func(context.Context, string, string, time.Duration) (string, error) {
		return "ok", nil
	})})
	t.Cleanup(m.Close)
	src := writeDeclarativePlugin(t, t.TempDir(), "audit-plugin")
	if _, err := m.Install(context.Background(), src); err != nil {
		t.Fatalf("安装失败: %v", err)
	}
		if _, err := m.Call(context.Background(), "audit-plugin", "echo", map[string]any{"text": "hi", "token": "secret-value"}, Env{AllowCommands: true}); err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	// 失败调用也要记录
	_, _ = m.Call(context.Background(), "audit-plugin", "nope", nil, Env{})
	raw, err := os.ReadFile(m.AuditPath())
	if err != nil {
		t.Fatalf("读审计失败: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, `"plugin":"audit-plugin"`) || !strings.Contains(text, `"ok":false`) {
		t.Fatalf("审计内容不完整: %s", text)
	}
	if strings.Contains(text, "secret-value") {
		t.Fatalf("审计里出现了明文密钥: %s", text)
	}
}

func TestLimiterConcurrencyAndRate(t *testing.T) {
	l := newLimiter(2, 100)
	release1, err := l.acquire("p")
	if err != nil {
		t.Fatal(err)
	}
	release2, err := l.acquire("p")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.acquire("p"); err == nil {
		t.Fatal("第三个并发调用应被拒绝")
	} else if !strings.Contains(err.Error(), "并发") {
		t.Fatalf("错误文案应说明并发上限: %v", err)
	}
	release1()
	if _, err := l.acquire("p"); err != nil {
		t.Fatalf("释放后应可再次获取: %v", err)
	}
	release2()

	l2 := newLimiter(10, 2)
	r1, _ := l2.acquire("p")
	r1()
	r2, _ := l2.acquire("p")
	r2()
	if _, err := l2.acquire("p"); err == nil {
		t.Fatal("超过每分钟上限应被拒绝")
	} else if !strings.Contains(err.Error(), "每分钟") {
		t.Fatalf("错误文案应说明每分钟上限: %v", err)
	}
}

func TestHTTPPermissionWhitelist(t *testing.T) {
	mf := &Manifest{ID: "p", Permissions: Permissions{Net: []string{"example.com", "*.trusted.io"}}}
	ok := []string{"https://example.com/a", "https://a.trusted.io/x"}
	bad := []string{"https://api.example.com/a", "https://trusted.io/x", "http://evil.com/"}
	for _, u := range ok {
		if err := checkHTTPPermission(mf, u); err != nil {
			t.Fatalf("%s 应被放行: %v", u, err)
		}
	}
	for _, u := range bad {
		if err := checkHTTPPermission(mf, u); err == nil {
			t.Fatalf("%s 应被拒绝", u)
		}
	}
	// 未声明 net 白名单：不限制
	if err := checkHTTPPermission(&Manifest{ID: "q"}, "https://anywhere.com/"); err != nil {
		t.Fatalf("空白名单不应限制: %v", err)
	}
}

func TestCommandPermissionDeclaredOnly(t *testing.T) {
	legacy := &Manifest{ID: "legacy"}
	if err := checkCommandPermission(legacy); err != nil {
		t.Fatalf("未声明 permissions 的旧插件应保持宽松: %v", err)
	}
	strict := &Manifest{ID: "strict", Permissions: Permissions{declared: true}}
	if err := checkCommandPermission(strict); err == nil {
		t.Fatal("声明了 permissions 但 exec=false 应被拒绝")
	}
	allowed := &Manifest{ID: "allowed", Permissions: Permissions{declared: true, Exec: true}}
	if err := checkCommandPermission(allowed); err != nil {
		t.Fatalf("exec=true 应放行: %v", err)
	}
	// 经 ReadManifest 解析时也应识别 declared
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "plugin.json"), `{"id":"m","name":"m","version":"1.0.0","permissions":{"exec":true}}`)
	mf, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !mf.Permissions.Declared() || !mf.Permissions.Exec {
		t.Fatalf("permissions 解析异常: %+v", mf.Permissions)
	}
}

func TestToolsCacheSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	m := New(Options{DataDir: dir, Logf: t.Logf, DisableAudit: true})
	src := t.TempDir()
	writeFakePlugin(t, src, "v2", nil)
	if _, err := m.Install(ctx, src); err != nil {
		m.Close()
		t.Fatalf("安装失败: %v", err)
	}
	if err := m.SetEnabled("fake-proc", true); err != nil {
		m.Close()
		t.Fatalf("启用失败: %v", err)
	}
	if tools, err := m.ProcessTools(ctx, "fake-proc"); err != nil || len(tools) == 0 {
		m.Close()
		t.Fatalf("取工具失败: %v", err)
	}
			m.Close()

	cacheFile := filepath.Join(dir, "plugins", "fake-proc", toolsCacheName)
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("未写入工具缓存: %v", err)
	}
	m2 := New(Options{DataDir: dir, DisableAudit: true, Logf: t.Logf})
	t.Cleanup(m2.Close)
	// 不调用 StartAll：仍应能从缓存列出工具
		tools := m2.RunningProcessTools("fake-proc")
	if len(tools) == 0 {
		for _, info := range m2.List() {
			t.Logf("plugin=%s enabled=%v cached=%d err=%q last=%q", info.ID, info.Enabled, info.CachedTools, info.Error, info.LastError)
		}
		if mf, ok := m2.Get("fake-proc"); ok {
			t.Logf("manifest dir=%s version=%s protocol=%d", mf.Dir, mf.Version, mf.Runtime.ProtocolVersion)
		} else {
			t.Log("manifest 缺失")
		}
		t.Fatal("重启后应能从缓存列出工具")
	}
	for _, td := range tools {
		if td.PluginID != "fake-proc" {
			t.Fatalf("缓存工具缺少 pluginId: %+v", td)
		}
	}
}

func TestHealthFailoverStopsPlugin(t *testing.T) {
	noRestart := false
	h, mf := newTestHost(t, "v2-noping", func(rt *RuntimeSpec) {
		rt.AutoRestart = &noRestart
		rt.PingTimeoutMs = 100
	})
	stopped := make(chan string, 1)
	h.OnUnhealthy = func(id, msg string) { stopped <- msg }

	ctx := context.Background()
	if _, err := h.Ensure(ctx, mf); err != nil {
		t.Fatalf("握手失败: %v", err)
	}
	h.PingAll(ctx, []*Manifest{mf})
	if h.Healthy(mf.ID) {
		t.Fatal("第一次 ping 失败后不应是健康状态")
	}
	h.PingAll(ctx, []*Manifest{mf})

	select {
	case msg := <-stopped:
		if !strings.Contains(msg, "健康检查") {
			t.Fatalf("回调文案异常: %s", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("连续失败应触发 OnUnhealthy")
	}
	if h.Running(mf.ID) {
		t.Fatal("不健康插件应被停止")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHealthUnhealthyDisablesPlugin(t *testing.T) {
	ctx := context.Background()
	noRestart := false
	dir := t.TempDir()
	m := New(Options{DataDir: dir, Logf: t.Logf, DisableAudit: true})
	t.Cleanup(m.Close)
	src := t.TempDir()
	mf := writeFakePlugin(t, src, "v2-noping", func(rt *RuntimeSpec) {
		rt.AutoRestart = &noRestart
		rt.PingTimeoutMs = 100
		rt.HealthIntervalMs = 1
	})
	if _, err := m.Install(ctx, src); err != nil {
		t.Fatalf("安装失败: %v", err)
	}
	if err := m.SetEnabled(mf.ID, true); err != nil {
		t.Fatalf("启用失败: %v", err)
	}
	m.StartAll(ctx)
	if _, err := m.ProcessTools(ctx, mf.ID); err != nil {
		t.Fatalf("握手失败: %v", err)
	}
	for i := 0; i < 3; i++ {
		m.sweepHealth()
	}
	time.Sleep(150 * time.Millisecond)
	var info PluginInfo
	for _, it := range m.List() {
		if it.ID == mf.ID {
			info = it
		}
	}
	if info.Enabled {
		t.Fatal("连续健康检查失败且 autoRestart=false 时应自动停用")
	}
	if !strings.Contains(info.LastError, "健康检查") {
		t.Fatalf("lastError 应含「健康检查」，实际: %q", info.LastError)
	}
}

func TestMemoryLimitApplied(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("内存上限只在 Windows（Job Object）上实现")
	}
	mf := writeFakePlugin(t, t.TempDir(), "ok", func(rt *RuntimeSpec) { rt.MemoryLimitMB = 64 })
	h := NewHost(t.Logf)
	t.Cleanup(h.Close)
	p, err := h.Ensure(context.Background(), mf)
	if err != nil {
		t.Fatalf("带内存上限的插件应能启动: %v", err)
	}
	if p.group == nil {
		t.Fatal("Windows 上应拿到 Job Object 句柄")
	}
	if err := p.group.SetMemoryLimit(64 << 20); err != nil {
		t.Fatalf("SetMemoryLimit 失败: %v", err)
	}
}

func TestCompareVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.2", "1.2.0", 0},
		{"1.10.0", "1.9.9", 1},
		{"v1.0.0", "1.0.1", -1},
		{"2", "1.9.9", 1},
	}
	for _, c := range cases {
		if got := CompareVersion(c.a, c.b); got != c.want {
			t.Fatalf("CompareVersion(%q,%q)=%d，期望 %d", c.a, c.b, got, c.want)
		}
	}
}
