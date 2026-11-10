package pages

import (
	"strings"
	"testing"
)

// TestCodeStressTextExactLines 压力测试文本的生成行数必须与宣称一致。
func TestCodeStressTextExactLines(t *testing.T) {
	const want = 100000
	txt := CodeStressText(want)
	got := len(strings.Split(txt, "\n"))
	if got != want {
		t.Fatalf("生成 %d 行，期望 %d 行", got, want)
	}
	if !strings.HasPrefix(txt, "// 十万行压力测试") {
		t.Fatalf("首行不是说明注释：%q", strings.SplitN(txt, "\n", 2)[0])
	}
	if !strings.Contains(txt, "package main") {
		t.Fatal("缺少 package 声明")
	}
}

// TestCodeSampleTextAndPath 语言切换的示例文本与路径映射。
func TestCodeSampleTextAndPath(t *testing.T) {
	if !strings.Contains(codeSampleText("json"), "\"name\"") {
		t.Fatal("json 示例缺失")
	}
	if !strings.Contains(codeSampleText("python"), "def tokenize") {
		t.Fatal("python 示例缺失")
	}
	if !strings.HasSuffix(codeSamplePath("go"), "main.go") {
		t.Fatal("go 路径应为 main.go")
	}
	if !strings.HasSuffix(codeSamplePath("typescript"), "virtual-list.ts") {
		t.Fatal("typescript 路径错误")
	}
}
