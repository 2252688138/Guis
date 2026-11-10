package ui

import "testing"

// 注册名归一化：非 ASCII 安全字符折叠成 '-'，避免文件系统 / 注册表拼接出问题。
func TestAutoStartFileName(t *testing.T) {
	cases := map[string]string{
		"DeepSeek Harness": "DeepSeek-Harness",
		"app v1.2":         "app-v1.2",
		"  --x--  ":        "x",
		"a/b\\c":           "a-b-c",
		"my_app-1":         "my_app-1",
		"中文名":              "",
	}
	for in, want := range cases {
		if got := autoStartFileName(in); got != want {
			t.Errorf("autoStartFileName(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// GUIS_AUTOSTART_CMD 解析：带引号路径 + 参数。
func TestSplitCommandLine(t *testing.T) {
	exe, args := splitCommandLine(`"C:\Program Files\My App\app.exe" --tray --name="a b"`)
	if exe != `C:\Program Files\My App\app.exe` {
		t.Errorf("exe = %q", exe)
	}
	want := []string{"--tray", "--name=a b"}
	if len(args) != len(want) {
		t.Fatalf("args = %#v，期望 %#v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %q，期望 %q", i, args[i], want[i])
		}
	}

	exe, args = splitCommandLine("app.exe")
	if exe != "app.exe" || len(args) != 0 {
		t.Errorf("无参数时解析错误: %q %#v", exe, args)
	}
}

// 规格补默认值：Name 缺省取可执行名（不带扩展名），空参数被丢弃。
func TestResolveAutoStartSpec(t *testing.T) {
	got, err := resolveAutoStartSpec(AutoStartSpec{Args: []string{" --tray ", "", "  "}})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got.Name == "" {
		t.Error("Name 不应为空")
	}
	if got.Exe == "" {
		t.Error("Exe 不应为空")
	}
	if len(got.Args) != 1 || got.Args[0] != "--tray" {
		t.Errorf("参数清洗错误: %#v", got.Args)
	}
	if got.Comment == "" {
		t.Error("Comment 缺省时应回退为注册名")
	}
}
