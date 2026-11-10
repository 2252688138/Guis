//go:build windows

package win32

import (
	"os"
	"testing"
)

// TestEnvPersistentRoundTrip 用户环境变量写入 / 读取 / 删除的往返验证。
//
// 只用一个一次性变量名，且不广播 WM_SETTINGCHANGE（避免打扰桌面上正在运行的程序），
// 测试结束必定删除，不会在开发机留下残留。
func TestEnvPersistentRoundTrip(t *testing.T) {
	const name = "GUIS_ENVTEST_KEY"
	t.Cleanup(func() {
		_ = DeleteEnvVar(name, EnvScopeUser)
		_ = os.Unsetenv(name)
	})

	if err := SetEnvVar(name, "sk-roundtrip", EnvScopeUser); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if got, ok := GetEnvPersistent(name, EnvScopeUser); !ok || got != "sk-roundtrip" {
		t.Fatalf("回读不一致: %q ok=%v", got, ok)
	}
	if got := os.Getenv(name); got != "sk-roundtrip" {
		t.Fatalf("当前进程应立即生效，实际 %q", got)
	}

	if err := DeleteEnvVar(name, EnvScopeUser); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, ok := GetEnvPersistent(name, EnvScopeUser); ok {
		t.Fatal("删除后不应还能读到")
	}

	// 删除不存在的变量应视为成功（幂等）
	if err := DeleteEnvVar(name, EnvScopeUser); err != nil {
		t.Fatalf("重复删除应成功: %v", err)
	}
}
