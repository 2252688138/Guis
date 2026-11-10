//go:build windows

package ui

import (
	"strings"
	"testing"

	"guis/internal/win32"
)

// 注册表往返：写 HKCU\...\Run → 读回 → 删除（测试项名独立，跑完即清理）。
func TestAutoStartRegistryRoundTrip(t *testing.T) {
	spec := AutoStartSpec{Name: "guis-autostart-selftest", Args: []string{"--tray"}}
	if err := SetAutoStart(true, spec); err != nil {
		t.Skipf("当前环境不允许写自启项：%v", err)
	}
	defer func() {
		if err := SetAutoStart(false, spec); err != nil {
			t.Errorf("清理自启项失败：%v", err)
		}
	}()

	cmd, ok := win32.RunAtLogin("guis-autostart-selftest")
	if !ok {
		t.Fatal("注册表里没有写入自启项")
	}
	if !strings.Contains(cmd, "--tray") {
		t.Errorf("自启命令应带 --tray 参数：%s", cmd)
	}
	if !AutoStartEnabled(spec) {
		t.Error("写入后 AutoStartEnabled 应为 true")
	}

	if err := SetAutoStart(false, spec); err != nil {
		t.Fatalf("删除失败：%v", err)
	}
	if _, ok := win32.RunAtLogin("guis-autostart-selftest"); ok {
		t.Error("删除后注册表应无该值")
	}
	if AutoStartEnabled(spec) {
		t.Error("删除后 AutoStartEnabled 应为 false")
	}
}

// 命令行引号：只对含空白 / 引号的参数加引号。
func TestAutoStartCommandQuoting(t *testing.T) {
	target := autoStartTarget{Exe: `C:\Program Files\App\app.exe`, Args: []string{"--tray", "a b"}}
	got := autoStartCommand(target)
	want := `"C:\Program Files\App\app.exe" --tray "a b"`
	if got != want {
		t.Errorf("autoStartCommand = %q，期望 %q", got, want)
	}
}
