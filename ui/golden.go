// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"image"
	"image/png"
	"os"
)

// 渲染回归（golden image）用例与渲染入口。
//
// 这份用例同时被两处使用：
//   - ui/golden_test.go：`go test ./ui/ -run TestGolden`（需要能创建 GL 上下文，
//     无显卡环境自动 skip）；
//   - cmd/goldencap：独立进程渲染并写/比 PNG，退出码表达结果，可在 CI 当门禁：
//
//		go run ./cmd/goldencap -update   # 生成基线
//		go run ./cmd/goldencap           # 比对基线
//
// 用例刻意只用基础盒模型 + 文字 + 主题色：跨驱动稳定，改动绘制层时能第一时间
// 发现「圆角/阴影/渐变/文字位置」这类肉眼可见的回归。

// GoldenCase 一个渲染回归用例。
type GoldenCase struct {
	Name   string
	Width  int
	Height int
	View   func() *Node
}

// GoldenCases 渲染回归用例集合（顺序稳定，便于基线比对与 diff 定位）。
func GoldenCases() []GoldenCase {
	card := func(fill string, radius float32, txt string) *Node {
		return Box(
			Text(txt).St(St().Text(14, 600, ColText)),
		).St(St().Padding(16).R(radius).BgC(fill).Stroke(1, ColStroke))
	}
	return []GoldenCase{
		{
			Name: "card_basic", Width: 240, Height: 140,
			View: func() *Node {
				return Column(card("#1f2430", 12, "Hello GUIS")).
					St(St().Padding(20).BgC("#0b0f19").Hp(100).GapOf(12))
			},
		},
		{
			Name: "button_row", Width: 320, Height: 120,
			View: func() *Node {
				return Row(
					Button("确定", func() {}),
					Button("取消", func() {}).St(St().BgC("#2a3040")),
				).St(St().GapOf(12).Padding(20).BgC("#0b0f19").Hp(100))
			},
		},
		{
			Name: "nested_layout", Width: 300, Height: 200,
			View: func() *Node {
				return Column(
					card("#1f2430", 10, "顶部"),
					Row(
						card("#232a38", 8, "左"),
						card("#232a38", 8, "右"),
					).St(St().GapOf(10)).Grow(1),
				).St(St().Padding(16).BgC("#0b0f19").Hp(100).GapOf(10))
			},
		},
	}
}

// loadPNG 读入 PNG 图像。
func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// ComparePNG 比对两张 PNG，返回差异像素占比（单通道差 > 8 才算不同，吸收抗锯齿细差）。
//
// 供渲染回归（golden）与 cmd/goldencap 共用；读图失败时返回错误。
func ComparePNG(basePath, curPath string) (float64, error) {
	a, err := loadPNG(basePath)
	if err != nil {
		return 0, err
	}
	b, err := loadPNG(curPath)
	if err != nil {
		return 0, err
	}
	return pixelDiffRatio(a, b), nil
}

// pixelDiffRatio 返回「差异像素占比」。
func pixelDiffRatio(a, b image.Image) float64 {
	ab, bb := a.Bounds(), b.Bounds()
	w, h := ab.Dx(), ab.Dy()
	if bb.Dx() != w || bb.Dy() != h {
		return 1
	}
	if w == 0 || h == 0 {
		return 0
	}
	const tol = 8
	diff := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r1, g1, b1, a1 := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			r2, g2, b2, a2 := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			if absDiff(r1, r2) > tol || absDiff(g1, g2) > tol ||
				absDiff(b1, b2) > tol || absDiff(a1, a2) > tol {
				diff++
			}
		}
	}
	return float64(diff) / float64(w*h)
}

// absDiff 两个 16 位通道值的差（换算到 8 位量级）。
func absDiff(x, y uint32) uint32 {
	if x > y {
		return (x - y) >> 8
	}
	return (y - x) >> 8
}

// RenderToPNG 用隐藏窗口把视图渲染一帧并写成 PNG。
//
// 供截图工具与渲染回归使用；返回错误表示无法创建窗口（无显卡 / 无 GL 上下文）
// 或读回像素失败。
func RenderToPNG(path string, w, h int, view func() *Node) error {
	app, err := newApp(Options{
		Title:      "capture",
		Width:      w,
		Height:     h,
		Hidden:     true,
		NoTitleBar: true,
	}, view)
	if err != nil {
		return err
	}
	defer app.Close()
	img, err := app.CaptureFrame()
	if err != nil {
		return err
	}
	return writePNG(path, img)
}
