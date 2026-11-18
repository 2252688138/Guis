//go:build windows || linux || darwin

// guis — 像素级渲染回归（golden image）
//
// 目的：绘制层（圆角 / 阴影 / 渐变 / 文字混排）的改动很难用断言表达，但一旦
// 改坏就是肉眼可见的视觉回归。这里把核心组件渲染成 PNG，与
// testdata/golden/*.png 做像素比对（容忍抗锯齿产生的少量差异）。
//
// 生成 / 更新基线：
//
//	$env:UPDATE_GOLDEN=1; go test ./ui/ -run TestGolden
//
// 无法创建 GL 上下文的环境（无显卡 CI、无头 Linux、部分沙箱）自动 skip ——
// 这类门禁应放在有显卡的 runner，或改用独立进程的 cmd/goldencap。

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestGolden 渲染核心组件并与基线图比对（差异像素比例 > 0.5% 判失败）。
func TestGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("golden 需要 GL 上下文，-short 下跳过")
	}
	if os.Getenv("GUIS_GOLDEN") == "" && os.Getenv("UPDATE_GOLDEN") == "" {
		t.Skip("像素回归需显式开启（GUIS_GOLDEN=1），或改用 cmd/goldencap")
	}
	update := os.Getenv("UPDATE_GOLDEN") != ""
	dir := filepath.Join("testdata", "golden")
	if update {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("创建基线目录失败：%v", err)
		}
	}
	for _, c := range GoldenCases() {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			base := filepath.Join(dir, c.Name+".png")
			if update {
				if err := renderGoldenSafe(base, c); err != nil {
					t.Skipf("%v", err)
				}
				return
			}
			if _, err := loadPNG(base); err != nil {
				t.Skipf("缺少基线 %s（UPDATE_GOLDEN=1 生成）", base)
			}
			cur := filepath.Join(t.TempDir(), c.Name+".png")
			if err := renderGoldenSafe(cur, c); err != nil {
				t.Skipf("%v", err)
			}
			diff, err := ComparePNG(base, cur)
			if err != nil {
				t.Fatalf("比对失败：%v", err)
			}
			if diff > 0.005 {
				t.Fatalf("渲染回归：%s 与基线差异 %.2f%%（阈值 0.5%%）", c.Name, diff*100)
			}
			t.Logf("%s 差异 %.3f%%", c.Name, diff*100)
		})
	}
}

// renderGoldenSafe 渲染一个用例，并把「环境不可用」的 panic 转成普通错误
//（测试进程里 GL 初始化失败会直接 panic，不能让它把整个包打挂）。
func renderGoldenSafe(path string, c GoldenCase) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("渲染环境不可用，已跳过：%v", r)
		}
	}()
	return RenderToPNG(path, c.Width, c.Height, c.View)
}
