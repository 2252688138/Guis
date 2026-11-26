package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeDeclarativePlugin(t *testing.T, parent, id string) string {
	t.Helper()
	dir := filepath.Join(parent, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "id": "` + id + `",
  "name": "声明式插件",
  "version": "1.0.0",
  "description": "测试用",
  "prompt": "插件提示词",
  "skills": ["skills"],
  "tools": [
    {"name":"echo","description":"回显","parameters":{"type":"object","properties":{"text":{"type":"string"}}},"type":"command","command":"echo {{text}}","safe":true},
    {"name":"demo_http","description":"示例 HTTP","parameters":{"type":"object","properties":{}},"type":"http","request":{"method":"GET","url":"https://example.invalid/{{workspace}}"},"safe":true}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestManagerInstallLifecycle(t *testing.T) {
	root := t.TempDir()
	src := writeDeclarativePlugin(t, filepath.Join(root, "src"), "decl-plugin")
	dataDir := filepath.Join(root, "data")
	m := New(Options{DataDir: dataDir})
	defer m.Close()

	mf, err := m.Install(context.Background(), src)
	if err != nil {
		t.Fatalf("安装失败: %v", err)
	}
	if mf.ID != "decl-plugin" {
		t.Fatalf("清单 id 异常: %s", mf.ID)
	}
	infos := m.List()
	if len(infos) != 1 || !infos[0].Enabled {
		t.Fatalf("声明式插件安装后应启用: %+v", infos)
	}
	if len(m.DeclarativeTools()) != 2 {
		t.Fatalf("工具数异常: %d", len(m.DeclarativeTools()))
	}

	if err := m.SetEnabled("decl-plugin", false); err != nil {
		t.Fatal(err)
	}
	if len(m.DeclarativeTools()) != 0 || len(m.EnabledManifests()) != 0 {
		t.Fatal("停用后仍暴露工具或清单")
	}

	// 状态应持久化
	m2 := New(Options{DataDir: dataDir})
	infos2 := m2.List()
	if len(infos2) != 1 || infos2[0].Enabled {
		t.Fatalf("状态未持久化: %+v", infos2)
	}
	if err := m2.Uninstall("decl-plugin"); err != nil {
		t.Fatalf("卸载失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m2.PluginsDir(), "decl-plugin")); !os.IsNotExist(err) {
		t.Fatal("卸载后目录仍存在")
	}
	if len(m2.List()) != 0 {
		t.Fatal("卸载后仍出现在列表")
	}
}

func TestManagerDeclarativeCallUsesRunner(t *testing.T) {
	root := t.TempDir()
	src := writeDeclarativePlugin(t, filepath.Join(root, "src"), "decl-plugin")
	var gotCmdline string
	m := New(Options{
		DataDir: filepath.Join(root, "data"),
		Runner: CommandFunc(func(ctx context.Context, cmdline, dir string, timeout time.Duration) (string, error) {
			gotCmdline = cmdline
			return "ok:" + cmdline, nil
		}),
	})
	defer m.Close()
	if _, err := m.Install(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	res, err := m.Call(context.Background(), "decl-plugin", "echo", map[string]any{"text": "hi"}, Env{Workspace: root, AllowCommands: true})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res.Text != "ok:echo hi" || gotCmdline != "echo hi" {
		t.Fatalf("模板替换异常: %q / %q", res.Text, gotCmdline)
	}

	// 权限不足时拒绝执行命令
	if _, err := m.Call(context.Background(), "decl-plugin", "echo", map[string]any{"text": "hi"}, Env{}); err == nil {
		t.Fatal("AllowCommands=false 时应拒绝")
	}
}

func TestManagerProcessPluginNeedsExplicitEnable(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src", "fake-proc")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	mf := writeFakePlugin(t, src, "ok", func(rt *RuntimeSpec) {
		rt.StartTimeoutMs, rt.CallTimeoutMs = 5000, 3000
	})
	raw, err := json.Marshal(mf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "plugin.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(Options{DataDir: filepath.Join(root, "data")})
	defer m.Close()
	if _, err := m.Install(context.Background(), src); err != nil {
		t.Fatalf("安装失败: %v", err)
	}
	infos := m.List()
	if len(infos) != 1 || infos[0].Enabled {
		t.Fatalf("进程插件安装后应默认停用: %+v", infos)
	}
	if infos[0].Runtime != RuntimeProcess {
		t.Fatalf("应标记 runtime=process: %+v", infos[0])
	}
	// 未启用不应拉起子进程
	m.hostMu.Lock()
	started := m.host
	m.hostMu.Unlock()
	if started != nil && started.Running("fake-proc") {
		t.Fatal("未启用时不应启动插件进程")
	}

	if err := m.SetEnabled("fake-proc", true); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if m.hostIfStarted() != nil && m.hostIfStarted().Running("fake-proc") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	res, err := m.Call(context.Background(), "fake-proc", "echo", map[string]any{"text": "from-manager"}, Env{})
	if err != nil {
		t.Fatalf("进程插件调用失败: %v", err)
	}
	if res.Text != "from-manager" {
		t.Fatalf("结果异常: %+v", res)
	}
	if err := m.SetEnabled("fake-proc", false); err != nil {
		t.Fatal(err)
	}
	if m.hostIfStarted() != nil && m.hostIfStarted().Running("fake-proc") {
		t.Fatal("停用后应结束子进程")
	}
}

func TestManagerRejectsEscapingCommand(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "p")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	mf := &Manifest{ID: "x", Runtime: RuntimeSpec{Type: RuntimeProcess, Command: "../outside"}, Dir: inner}
	err := Validate(mf)
	if err == nil || !strings.Contains(err.Error(), "越出插件目录") {
		t.Fatalf("应拒绝越界路径，实际: %v", err)
	}
	abs := &Manifest{ID: "x", Runtime: RuntimeSpec{Type: RuntimeProcess, Command: filepath.Join(inner, "a")}, Dir: inner}
	if err := Validate(abs); err == nil || !strings.Contains(err.Error(), "相对路径") {
		t.Fatalf("应拒绝绝对路径，实际: %v", err)
	}
}
