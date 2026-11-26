package plugins

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateBasics(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad")
	_ = os.MkdirAll(bad, 0o755)
	_ = os.WriteFile(filepath.Join(bad, "plugin.json"), []byte(`{"id":"Bad ID","name":"x"}`), 0o644)
	if _, err := ReadManifest(bad); err == nil {
		t.Fatal("非法 id 应当被拒绝")
	}

	noCmd := filepath.Join(dir, "nocmd")
	_ = os.MkdirAll(noCmd, 0o755)
	_ = os.WriteFile(filepath.Join(noCmd, "plugin.json"),
		[]byte(`{"id":"ok","name":"x","tools":[{"name":"t","description":"d","type":"command"}]}`), 0o644)
	if _, err := ReadManifest(noCmd); err == nil {
		t.Fatal("命令工具缺 command 应当被拒绝")
	}

	dup := filepath.Join(dir, "dup")
	_ = os.MkdirAll(dup, 0o755)
	_ = os.WriteFile(filepath.Join(dup, "plugin.json"),
		[]byte(`{"id":"ok2","name":"x","tools":[{"name":"t","description":"d","type":"command","command":"a"},{"name":"t","description":"d","type":"command","command":"b"}]}`), 0o644)
	if _, err := ReadManifest(dup); err == nil {
		t.Fatal("工具重名应当被拒绝")
	}

	// 无 runtime 段的旧清单行为不变：type 默认成 command、参数补默认 schema
	legacy := filepath.Join(dir, "legacy")
	_ = os.MkdirAll(legacy, 0o755)
	_ = os.WriteFile(filepath.Join(legacy, "plugin.json"),
		[]byte(`{"id":"legacy","name":"x","tools":[{"name":"t","description":"d","command":"echo 1"}]}`), 0o644)
	mf, err := ReadManifest(legacy)
	if err != nil {
		t.Fatalf("旧清单应通过: %v", err)
	}
	if mf.Tools[0].Type != "command" || len(mf.Tools[0].Parameters) == 0 || mf.Runtime.IsProcess() {
		t.Fatalf("旧清单规范化异常: %+v", mf)
	}
}

func TestValidateRuntime(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "plugin"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	ok := &Manifest{ID: "p", Runtime: RuntimeSpec{Type: "process", Command: "bin/plugin"}, Dir: dir}
	if err := Validate(ok); err != nil {
		t.Fatalf("合法 runtime 应通过: %v", err)
	}
	if ok.Runtime.Protocol != ProtocolJSONRPCStdio || ok.Runtime.ProtocolVersion != ProtocolCurrent {
		t.Fatalf("协议应被补默认值: %+v", ok.Runtime)
	}

	badProto := &Manifest{ID: "p", Runtime: RuntimeSpec{Type: "process", Command: "bin/plugin", Protocol: "http"}, Dir: dir}
	if err := Validate(badProto); err == nil {
		t.Fatal("非法 protocol 应被拒绝")
	}

		badVer := &Manifest{ID: "p", Runtime: RuntimeSpec{Type: "process", Command: "bin/plugin", ProtocolVersion: 3}, Dir: dir}
	if err := Validate(badVer); err == nil {
		t.Fatal("未知 protocolVersion 应被拒绝")
	}
	v2 := &Manifest{ID: "p", Runtime: RuntimeSpec{Type: "process", Command: "bin/plugin", ProtocolVersion: 2}, Dir: dir}
	if err := Validate(v2); err != nil {
		t.Fatalf("protocolVersion=2 应被接受: %v", err)
	}

	badType := &Manifest{ID: "p", Runtime: RuntimeSpec{Type: "wasm", Command: "bin/plugin"}, Dir: dir}
	if err := Validate(badType); err == nil {
		t.Fatal("未知 runtime.type 应被拒绝")
	}

	missing := &Manifest{ID: "p", Runtime: RuntimeSpec{Type: "process", Command: "bin/nope"}, Dir: dir}
	if err := Validate(missing); err == nil {
		t.Fatal("command 不存在应被拒绝")
	}

	escape := &Manifest{ID: "p", Runtime: RuntimeSpec{Type: "process", Command: "../evil"}, Dir: dir}
	if err := Validate(escape); err == nil || !strings.Contains(err.Error(), "越出插件目录") {
		t.Fatalf("越界路径应被拒绝: %v", err)
	}
}

func TestInstallZipRejectsSlip(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	add("plugin.json", `{"id":"zip-plugin","name":"zip","description":"d"}`)
	add("../../evil.txt", "pwned")
	add("pkg/../../evil2.txt", "pwned")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	pluginsDir := filepath.Join(root, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mf, err := installZip(zipPath, pluginsDir)
	if err != nil {
		t.Fatalf("安装失败: %v", err)
	}
	if mf.ID != "zip-plugin" {
		t.Fatalf("清单异常: %+v", mf)
	}
	if _, err := os.Stat(filepath.Join(root, "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("zip-slip 文件被写到了插件目录之外")
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "zip-plugin", "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("穿越条目不应被解压到插件目录")
	}
}

func TestExpandTemplate(t *testing.T) {
	args := map[string]any{"q": "a b", "n": float64(3), "flag": true}
	if got := expandTemplate("x/{{q}}?n={{n}}&w={{workspace}}&f={{flag}}", args, "/ws", true); got != "x/a+b?n=3&w=%2Fws&f=true" {
		t.Fatalf("模板替换异常: %s", got)
	}
	if got := expandTemplate("{{missing}}", nil, "", false); got != "" {
		t.Fatalf("缺失参数应为空: %q", got)
	}
}
