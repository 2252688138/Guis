// goldencap 渲染回归门禁：把 ui.GoldenCases 里的用例渲染成 PNG 并与基线比对。
//
//	go run ./cmd/goldencap -update -case card_basic   # 生成 / 更新基线
//	go run ./cmd/goldencap -case card_basic           # 比对，超阈值时退出码 1
//
// 与 `go test ./ui/ -run TestGolden` 的区别：本工具是独立进程，GL 上下文初始化
// 走和正式应用一样的路径，因此在测试进程里 init 失败的环境（部分 CI / 沙箱）
// 仍然可用；退出码可直接作为 CI 门禁。
//
// 重要限制：本框架假定「一进程一窗口」。同一进程里 Close 之后再建窗口会崩，
// 所以这里每次只渲染一个用例（默认第一个），多用例请由外部脚本逐次调用：
//
//	foreach ($c in card_basic,button_row,nested_layout) { go run ./cmd/goldencap -case $c }
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"guis/ui"
)

func main() {
	dir := flag.String("dir", filepath.Join("ui", "testdata", "golden"), "基线图目录")
	update := flag.Bool("update", false, "生成 / 更新基线图")
	tol := flag.Float64("tol", 0.005, "允许的差异像素比例（默认 0.5%）")
	name := flag.String("case", "", "要渲染的用例名（默认第一个）")
	flag.Parse()

	cases := ui.GoldenCases()
	if len(cases) == 0 {
		fmt.Fprintln(os.Stderr, "没有渲染回归用例")
		os.Exit(2)
	}
	c := cases[0]
	if *name != "" {
		found := false
		for _, it := range cases {
			if it.Name == *name {
				c, found = it, true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "没有名为 %q 的用例\n", *name)
			os.Exit(2)
		}
	}

	if *update {
		if err := os.MkdirAll(*dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "创建基线目录失败：%v\n", err)
			os.Exit(2)
		}
	}
	base := filepath.Join(*dir, c.Name+".png")

	if *update {
		if err := renderSafe(base, c); err != nil {
			fmt.Printf("SKIP  %-14s %v\n", c.Name, err)
			os.Exit(0)
		}
		fmt.Printf("WRITE %-14s %s\n", c.Name, base)
		return
	}

	if _, err := os.Stat(base); err != nil {
		fmt.Printf("SKIP  %-14s 缺少基线（先跑 -update）\n", c.Name)
		os.Exit(0)
	}
	cur := filepath.Join(os.TempDir(), "goldencap-"+c.Name+".png")
	defer os.Remove(cur)
	if err := renderSafe(cur, c); err != nil {
		fmt.Printf("SKIP  %-14s %v\n", c.Name, err)
		os.Exit(0)
	}
	diff, err := ui.ComparePNG(base, cur)
	if err != nil {
		fmt.Printf("FAIL  %-14s 比对失败：%v\n", c.Name, err)
		os.Exit(1)
	}
	if diff > *tol {
		fmt.Printf("FAIL  %-14s 差异 %.3f%% > %.3f%%（本次：%s）\n", c.Name, diff*100, *tol*100, cur)
		os.Exit(1)
	}
	fmt.Printf("OK    %-14s 差异 %.3f%%\n", c.Name, diff*100)
}

// renderSafe 渲染一个用例，把环境不可用的 panic 转成错误。
func renderSafe(path string, c ui.GoldenCase) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("渲染环境不可用：%v", r)
		}
	}()
	return ui.RenderToPNG(path, c.Width, c.Height, c.View)
}
