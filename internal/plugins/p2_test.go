package plugins

import (
	"archive/zip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildPluginZip 把声明式插件目录打成 zip（根目录直接是 plugin.json）。
func buildPluginZip(t *testing.T, id string) string {
	t.Helper()
	src := writeDeclarativePlugin(t, t.TempDir(), id)
	out := filepath.Join(t.TempDir(), id+".zip")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	err = filepath.Walk(src, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	sum, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func TestV2CommandsConfigProgress(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(Options{DataDir: dir, Logf: t.Logf})
	t.Cleanup(m.Close)
	src := t.TempDir()
	mf := writeFakePlugin(t, src, "v2", nil)
	if _, err := m.Install(ctx, src); err != nil {
		t.Fatalf("安装失败: %v", err)
	}
	if err := m.SetEnabled(mf.ID, true); err != nil {
		t.Fatalf("启用失败: %v", err)
	}

	// 命令列表（空 ID 与重名应被过滤）
	cmds, err := m.Commands(ctx, mf.ID)
	if err != nil {
		t.Fatalf("取命令失败: %v", err)
	}
	if len(cmds) != 1 || cmds[0].ID != "greet" || cmds[0].PluginID != mf.ID {
		t.Fatalf("命令列表异常: %+v", cmds)
	}

	// 命令执行
	res, err := m.RunCommand(ctx, mf.ID, "greet", map[string]any{"name": "小明"})
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}
	if !strings.Contains(res.Text, "默认问候") || !strings.Contains(res.Text, "小明") {
		t.Fatalf("命令输出异常: %q", res.Text)
	}

	// 配置读写
		if err := m.SetConfig(mf.ID, map[string]any{"greeting": "嗨"}); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	raw, err := os.ReadFile(m.ConfigPath(mf.ID))
	if err != nil {
		t.Fatalf("配置文件不存在: %v", err)
	}
	if !strings.Contains(string(raw), "嗨") {
		t.Fatalf("配置文件内容异常: %s", raw)
	}
	cfg, err := m.GetConfig(mf.ID)
	if err != nil {
		t.Fatalf("读配置失败: %v", err)
	}
	if cfg["greeting"] != "嗨" {
		t.Fatalf("配置回读异常: %+v", cfg)
	}

	// 工具调用 + progress / log 通知
	if _, err := m.Call(ctx, mf.ID, "echo", map[string]any{"text": "hi"}, Env{}); err != nil {
		t.Fatalf("调用工具失败: %v", err)
	}
	events := m.Progress(mf.ID)
	if len(events) == 0 {
		t.Fatal("应收到 progress 通知")
	}
	if events[len(events)-1].Tool != "echo" {
		t.Fatalf("progress 事件异常: %+v", events)
	}
	logs := strings.Join(m.ProcessLogs(mf.ID), "\n")
	if !strings.Contains(logs, "echo 被调用") {
		t.Fatalf("应合并 log 通知，实际: %s", logs)
	}

	// 停用后命令不可用
	if err := m.SetEnabled(mf.ID, false); err != nil {
		t.Fatalf("停用失败: %v", err)
	}
	if _, err := m.Commands(ctx, mf.ID); err == nil {
		t.Fatal("停用后应拒绝取命令")
	}
}

func TestV1PluginHasNoCommands(t *testing.T) {
	ctx := context.Background()
	m := New(Options{DataDir: t.TempDir(), Logf: t.Logf, DisableAudit: true})
	t.Cleanup(m.Close)
	src := t.TempDir()
	mf := writeFakePlugin(t, src, "ok", nil)
	if _, err := m.Install(ctx, src); err != nil {
		t.Fatal(err)
	}
	if err := m.SetEnabled(mf.ID, true); err != nil {
		t.Fatal(err)
	}
	cmds, err := m.Commands(ctx, mf.ID)
	if err != nil {
		t.Fatalf("v1 插件不应报错: %v", err)
	}
	if len(cmds) != 0 {
		t.Fatalf("v1 插件不应有命令: %+v", cmds)
	}
	if _, err := m.RunCommand(ctx, mf.ID, "greet", nil); err == nil {
		t.Fatal("v1 插件执行命令应被拒绝")
	}
}

func TestMinHostVersionRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "plugin.json"), `{"id":"future","name":"future","version":"1.0.0","minHostVersion":"9.0.0"}`)
	if _, err := ReadManifest(dir); err == nil {
		t.Fatal("minHostVersion 高于宿主版本应被拒绝")
	} else if !strings.Contains(err.Error(), "9.0.0") {
		t.Fatalf("错误文案应包含要求版本: %v", err)
	}
	writeFile(t, filepath.Join(dir, "plugin.json"), `{"id":"fine","name":"fine","version":"1.0.0","minHostVersion":"1.0.0"}`)
	if _, err := ReadManifest(dir); err != nil {
		t.Fatalf("符合版本要求应可安装: %v", err)
	}
}

func TestRegistryInstallWithSHA256(t *testing.T) {
	ctx := context.Background()
	zipPath := buildPluginZip(t, "reg-plugin")
	sum := fileDigest(t, zipPath)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, zipPath)
	}))
	defer srv.Close()

	dir := t.TempDir()
	registry := map[string]any{"plugins": []map[string]any{
		{"id": "reg-plugin", "name": "Registry 插件", "version": "1.0.0", "url": srv.URL + "/reg-plugin.zip", "sha256": sum},
		{"id": "bad-plugin", "version": "1.0.0", "url": srv.URL + "/reg-plugin.zip", "sha256": strings.Repeat("0", 64)},
	}}
	raw, _ := json.Marshal(registry)
	writeFile(t, filepath.Join(dir, "plugins_registry.json"), string(raw))

	m := New(Options{DataDir: dir, Runner: CommandFunc(func(context.Context, string, string, time.Duration) (string, error) {
		return "ok", nil
	})})
	t.Cleanup(m.Close)

	entries, err := m.Registry()
	if err != nil {
		t.Fatalf("读 registry 失败: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("registry 条目数异常: %+v", entries)
	}

	mf, err := m.InstallFromRegistry(ctx, "reg-plugin")
	if err != nil {
		t.Fatalf("从 registry 安装失败: %v", err)
	}
	if mf.ID != "reg-plugin" {
		t.Fatalf("安装的插件 ID 异常: %s", mf.ID)
	}
	var trust string
	for _, info := range m.List() {
		if info.ID == "reg-plugin" {
			trust = info.Trust
		}
	}
	if trust != TrustVerified {
		t.Fatalf("摘要校验通过后信任等级应为 verified，实际: %q", trust)
	}

	if _, err := m.InstallFromRegistry(ctx, "bad-plugin"); err == nil {
		t.Fatal("摘要不匹配应拒绝安装")
	} else if !strings.Contains(err.Error(), "校验失败") {
		t.Fatalf("错误文案异常: %v", err)
	}
}

func TestInstallOptionsSHA256Mismatch(t *testing.T) {
	ctx := context.Background()
	m := New(Options{DataDir: t.TempDir(), DisableAudit: true})
	t.Cleanup(m.Close)
	src := t.TempDir()
	writeFakePlugin(t, src, "ok", nil)
	_, err := m.InstallWithOptions(ctx, src, InstallOptions{ExpectedSHA256: strings.Repeat("a", 64)})
	if err == nil {
		t.Fatal("目录安装源带摘要应报错")
	}
	zipPath := buildPluginZip(t, "zip-plugin")
	if _, err := m.InstallWithOptions(ctx, zipPath, InstallOptions{ExpectedSHA256: strings.Repeat("a", 64)}); err == nil {
		t.Fatal("摘要不匹配应拒绝安装")
	}
	target := filepath.Join(m.PluginsDir(), "zip-plugin")
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatal("摘要不匹配时不应落盘")
	}
	if _, err := m.InstallWithOptions(ctx, zipPath, InstallOptions{ExpectedSHA256: fileDigest(t, zipPath)}); err != nil {
		t.Fatalf("摘要匹配应安装成功: %v", err)
	}
	if _, ok := m.Get("zip-plugin"); !ok {
		t.Fatal("安装后应能取到清单")
	}
}

// sha256 摘要一致性由 TestRegistryInstallWithSHA256 覆盖。
