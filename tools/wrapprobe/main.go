// probe: 验证 WrapOn 折行后的测量高度
package main

import (
	"fmt"

	"guis/ui"
)

const s = "GUIS 是一套纯 Go、零 cgo 的跨平台自绘 GUI 框架：一份界面代码，编译到 Windows、Linux、macOS、WebAssembly 与 Android。" +
	"OpenGL 直出，SDF 圆角、任意角度渐变、阴影与毛玻璃全部由片元着色器实时绘制。"

func frame(n *ui.Node) string {
	return fmt.Sprintf("%.0fx%.0f", n.Frame.W, n.Frame.H)
}

func main() {
	// 1) 固定宽列的折行文本
	t1 := ui.Text(s).St(ui.St().Text(15, 400, "#ffffff").WrapOn().W(560).MaxWidth(560))
	col := ui.Column(t1).St(ui.St().W(560))
	ui.LayoutForTest(col, 1200, 900, "", 1)
	fmt.Println("1 fixed W560:", frame(t1))

	// 2) 只 WrapOn，父列宽 560
	t2 := ui.Text(s).St(ui.St().Text(15, 400, "#ffffff").WrapOn())
	col2 := ui.Column(t2).St(ui.St().W(560))
	ui.LayoutForTest(col2, 1200, 900, "", 1)
	fmt.Println("2 wrap in col560:", frame(t2))

	// 3) 不折行
	t3 := ui.Text(s).St(ui.St().Text(15, 400, "#ffffff"))
	col3 := ui.Column(t3).St(ui.St().W(560))
	ui.LayoutForTest(col3, 1200, 900, "", 1)
	fmt.Println("3 nowrap:", frame(t3))
}
