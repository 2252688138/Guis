// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"strings"
)

// 打印与打印预览。
//
//	type invoice struct{}
//	func (invoice) Pages() int { return 1 }
//	func (invoice) DrawPage(c *ui.PrintCanvas, page int) {
//	    c.Text(60, 60, 18, "发票", "#111111")
//	    c.Line(60, 96, 535, 96, 1, "#cccccc")
//	}
//	ui.PrintPreview("invoice", invoice{}, 0.8)   // 页面卡片预览
//	ui.Print("发票", invoice{})                  // 弹打印对话框
//
// 坐标单位：预览按 96 DPI 的 A4 页面（794×1123），打印时自动按打印机 DPI 缩放。

// PrintCanvas 页面绘制画布（打印与预览共用同一套坐标）。
type PrintCanvas struct {
	W, H float32
	cmds []printCmd
	// scale 打印时的设备像素 / 逻辑像素
	scale float32
	dc    uintptr
	live  bool
}

type printCmd struct {
	kind   int // 0 text, 1 rect, 2 line
	x, y   float32
	w, h   float32
	size   float32
	text   string
	color  string
	fill   string
	stroke float32
	sx, sy float32
	ex, ey float32
}

// A4 页面逻辑尺寸（96 DPI）。
const (
	PageW96 float32 = 794
	PageH96 float32 = 1123
)

// Text 绘制文本（x/y 为左上角）。
func (c *PrintCanvas) Text(x, y, size float32, s, color string) {
	if c == nil || s == "" {
		return
	}
	if c.live {
		printText(c.dc, x*c.scale, y*c.scale, size*c.scale, s, color)
		return
	}
	c.cmds = append(c.cmds, printCmd{kind: 0, x: x, y: y, size: size, text: s, color: color})
}

// Rect 绘制矩形（fill 为空只描边；stroke<=0 只填充）。
func (c *PrintCanvas) Rect(x, y, w, h, stroke float32, strokeColor, fill string) {
	if c == nil {
		return
	}
	if c.live {
		printRect(c.dc, x*c.scale, y*c.scale, w*c.scale, h*c.scale, stroke*c.scale, strokeColor, fill)
		return
	}
	c.cmds = append(c.cmds, printCmd{kind: 1, x: x, y: y, w: w, h: h, stroke: stroke,
		color: strokeColor, fill: fill})
}

// Line 绘制直线。
func (c *PrintCanvas) Line(x1, y1, x2, y2, width float32, color string) {
	if c == nil {
		return
	}
	if c.live {
		printLine(c.dc, x1*c.scale, y1*c.scale, x2*c.scale, y2*c.scale, width*c.scale, color)
		return
	}
	c.cmds = append(c.cmds, printCmd{kind: 2, sx: x1, sy: y1, ex: x2, ey: y2,
		stroke: width, color: color})
}

// Printable 可打印文档。
type Printable interface {
	Pages() int
	DrawPage(c *PrintCanvas, page int)
}

// Print 弹出系统打印对话框并打印文档；用户取消返回 (false, nil)。
func Print(title string, doc Printable) (bool, error) {
	if doc == nil {
		return false, nil
	}
	var hwnd uintptr
	if a := currentApp.Load(); a != nil {
		hwnd = a.HWND()
	}
	return printDocument(hwnd, title, doc)
}

// PrintPreview 打印预览（按 A4 比例渲染页面卡片，可缩放）。
func PrintPreview(key string, doc Printable, zoom float32) *Node {
	if doc == nil {
		return EmptyState("", "没有可打印内容", "", "", nil)
	}
	if zoom <= 0 {
		zoom = 0.62
	}
	pages := doc.Pages()
	if pages <= 0 {
		pages = 1
	}
	w, h := PageW96*zoom, PageH96*zoom
	list := make([]*Node, 0, pages)
	for p := 0; p < pages; p++ {
		canvas := &PrintCanvas{W: PageW96, H: PageH96}
		doc.DrawPage(canvas, p)
		items := make([]*Node, 0, len(canvas.cmds))
		for _, c := range canvas.cmds {
			switch c.kind {
			case 0:
				items = append(items, Text(c.text).St(St().
					Text(c.size*zoom, 400, colorOr(c.color, "#111111")).
					AbsTL(c.x*zoom, c.y*zoom).NoFocusRing()))
			case 1:
				box := Box().St(St().AbsTL(c.x*zoom, c.y*zoom).
					W(c.w * zoom).H(c.h * zoom).NoFocusRing())
				if c.fill != "" {
					box.St(St().BgC(c.fill))
				}
				if c.stroke > 0 {
					box.St(St().Stroke(c.stroke*zoom, c.color))
				}
				items = append(items, box)
			case 2:
				lw := c.stroke
				if lw <= 0 {
					lw = 1
				}
				dx, dy := c.ex-c.sx, c.ey-c.sy
				length := float32(1)
				if dx*dx+dy*dy > 0 {
					length = float32(sqrtf(dx*dx + dy*dy))
				}
				angle := atan2degf(dy, dx)
				items = append(items, Box().St(St().AbsTL(c.sx*zoom, c.sy*zoom-lw*zoom/2).
					W(length*zoom).H(lw*zoom).BgC(colorOr(c.color, "#999999")).
					NoFocusRing()))
				_ = angle
			}
		}
		page := Stack(items...).St(St().W(w).H(h).BgC("#ffffff").Clipped().
			Shadow(18, 6, 0, "rgba(0,0,0,0.35)").NoFocusRing())
		list = append(list, Column(page,
			Text(fmt.Sprintf("第 %d / %d 页", p+1, pages)).St(St().Text(10.5, 400, ColTextMuted).
				PadOnly(6, 0, 0, 0))).St(St().GapOf(4)).AlignTo(AlignCenter))
	}
	bar := Row(
		Text("打印预览").St(St().Text(13, 600, ColText)).Grow(1),
		Text(fmt.Sprintf("共 %d 页", pages)).St(St().Text(11, 400, ColTextMuted)),
		Btn("打印…", BtnPrimary, func() {
			title := printTitle(key, doc)
			// 失败必须回显：系统打印对话框打不开时改走内置对话框，
			// 不再「点了没反应」。
			if _, err := Print(title, doc); err != nil {
				OpenPrintDialog(title, doc, err.Error())
			}
		}),
	).St(St().GapOf(10)).AlignTo(AlignCenter)
	panel := Column(bar, Divider(),
		Scroll(Row(list...).St(St().GapOf(24).Padding(20))).K(key+"-preview-scroll").
			St(St().GrowTo(1))).St(St().GapOf(10).Padding(14))
	// 内置打印对话框叠在预览面板上（AbsFill 子节点不参与尺寸计算）。
	return Column(panel, PrintDialogLayer()).St(St().BgC(ColBgDeep))
}

// Printers 可用打印机名（默认打印机排第一位；非 Windows / Web 端返回 nil）。
func Printers() []string { return printList() }

// DefaultPrinter 默认打印机名（没有则空串）。
func DefaultPrinter() string { return printDefault() }

// PrintTo 不弹系统对话框，直接打印到指定打印机。
//
// 适用于系统打印对话框不可用的环境（PrintDlgW 返回 CDERR_DIALOGFAILURE）：
//
//	ui.PrintTo(ui.DefaultPrinter(), "发票", doc)
func PrintTo(printer, title string, doc Printable) (bool, error) {
	if doc == nil {
		return false, nil
	}
	return printTo(printer, title, doc)
}

// ---- 内置打印对话框（系统对话框不可用时的兜底）----

// printPick 内置打印对话框的跨帧状态。
var printPick struct {
	open     bool
	title    string
	doc      Printable
	reason   string
	printers []string
	sel      int
}

// PrintDialogOpen 当前是否有内置打印对话框在显示。
func PrintDialogOpen() bool { return printPick.open }

// OpenPrintDialog 打开内置打印对话框（打印机列表 + 打印 / 取消）。
//
// 一般不需要业务自己调：ui.PrintPreview 的「打印…」在系统打印对话框失败时
// 会自动降级到它；需要自定义入口时也可直接调。
func OpenPrintDialog(title string, doc Printable, reason string) {
	printPick.open = true
	printPick.title = title
	printPick.doc = doc
	printPick.reason = reason
	printPick.sel = 0
	printPick.printers = printList()
	RequestRedraw()
}

// ClosePrintDialog 关闭内置打印对话框。
func ClosePrintDialog() {
	if !printPick.open {
		return
	}
	printPick.open = false
	RequestRedraw()
}

// PrintDialogLayer 内置打印对话框浮层（挂在根 Stack 上；未打开时返回 nil）。
func PrintDialogLayer() *Node {
	if !printPick.open {
		return nil
	}
	body := make([]*Node, 0, 3)
	if len(printPick.printers) == 0 {
		body = append(body, Text("系统里没有检测到可用打印机").
			St(St().Text(12.5, 500, ColDanger).WrapOn()))
	} else {
		rows := make([]*Node, 0, len(printPick.printers))
		for i, name := range printPick.printers {
			i, name := i, name
			mark := Box().St(St().W(9).H(9).R(5).BgC(ColStroke))
			if i == printPick.sel {
				mark = Box().St(St().W(9).H(9).R(5).BgC(ColPrimary))
			}
			row := Row(
				mark,
				Text(name).St(St().Text(12.5, 500, ColText).GrowTo(1)),
			).St(St().GapOf(10).Padding(9).R(8)).Tap(func() {
				printPick.sel = i
				RequestRedraw()
			})
			rows = append(rows, row)
		}
		body = append(body, Column(
			Text(fmt.Sprintf("选择打印机（共 %d 台）", len(rows))).
				St(St().Text(11.5, 400, ColTextMuted)),
			Scroll(Column(rows...).St(St().GapOf(2))).St(St().H(168)),
		).St(St().GapOf(8)))
	}
	if printPick.reason != "" {
		body = append(body, Text("系统打印对话框不可用，已改用内置对话框："+printPick.reason).
			St(St().Text(10.5, 400, ColTextMuted).WrapOn()))
	}

	var footer *Node
	if len(printPick.printers) > 0 {
		footer = Row(
			Spacer(),
			Btn("取消", BtnSecondary, func() { ClosePrintDialog() }),
			Btn("打印", BtnPrimary, func() {
				printer := printPick.printers[printPick.sel]
				doc := printPick.doc
				title := printPick.title
				ClosePrintDialog()
				if ok, err := PrintTo(printer, title, doc); err != nil {
					ToastShow("打印失败：" + err.Error())
				} else if ok {
					ToastShow("已发送到打印机：" + printer)
				}
			}),
		).St(St().GapOf(10)).AlignTo(AlignCenter)
	} else {
		footer = Row(Spacer(), Btn("关闭", BtnSecondary, func() { ClosePrintDialog() })).
			St(St().GapOf(10)).AlignTo(AlignCenter)
	}

	return ModalPanel(DialogOpts{
		Key:      "print-dlg",
		Open:     true,
		Kind:     DlgInfo,
		Title:    "打印 · " + printPick.title,
		BodyNode: Column(body...).St(St().GapOf(12)),
		Footer:   footer,
		Closable: true,
		OnClose:  func() { ClosePrintDialog() },
	})
}

// printTitle 打印任务名：文档自己带 Title() 就用它，否则回退到预览 key。
func printTitle(key string, doc Printable) string {
	if t, ok := doc.(interface{ Title() string }); ok {
		if s := strings.TrimSpace(t.Title()); s != "" {
			return s
		}
	}
	if s := strings.TrimSpace(key); s != "" {
		return s
	}
	return "打印"
}

// colorOr 空颜色回退默认值。
func colorOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// sqrtf / atan2degf 轻量数学封装（避免在预览里引入额外依赖）。
func sqrtf(v float32) float64 {
	x := float64(v)
	z := x
	for i := 0; i < 12; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func atan2degf(y, x float32) float32 {
	if x == 0 && y == 0 {
		return 0
	}
	return 0
}
