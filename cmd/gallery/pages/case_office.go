// guis — gallery 页面拆分：办公文档（Excel / Word / PDF）
//
// 演示框架新增的「办公能力」（internal/office + ui/office.go）：
//
//	Excel  · 读写 .xlsx（共享字符串 / 数值 / 日期 / 表头样式）+ 表格渲染 + 排序 / 累加
//	Word   · 读写 .docx（标题 / 段落 / 列表 / 表格）+ 文档流渲染
//	PDF    · 生成 PDF（嵌入系统中文字体子集）+ 解析回读 + 页面级渲染 + 缩放
//	打印   · 任意文档 → ui.PrintPreview / ui.Print，或 ui.ExportPDF 直接导出 PDF
//
// 全部示例数据都在内存里生成并「写 → 读」往返一次，所以页面上看到的表格 / 文档
// 就是解析器的真实输出（也能验证读写实现没跑偏），且三端（桌面 / 浏览器 / 安卓）
// 行为一致：不依赖临时文件，只有「打开 / 导出」按钮在支持文件对话框的平台才出现。

package pages

import (
	"fmt"
	"os"
	"strings"

	"guis/internal/office"
	"guis/ui"
)

// ---- 页面状态 ----

// officeTab 当前页签：excel / word / pdf。
var officeTab = "excel"

// officePane 一种文档类型的演示状态（各自独立，切页签不丢内容）。
type officePane struct {
	file  *office.File
	path  string
	size  int
	msg   string
	sheet string // Excel 当前工作表
	zoom  float32
	// preview 是否显示打印预览
	preview bool
}

var officePanes = map[string]*officePane{}

// officePaneFor 取（必要时懒创建）某页签的状态与示例文档。
func officePaneFor(tab string) *officePane {
	if p, ok := officePanes[tab]; ok && p != nil {
		return p
	}
	p := &officePane{zoom: 1}
	officePanes[tab] = p
	officeLoadSample(tab)
	return p
}

// officeLoadSample 生成内存示例文档（写 → 读往返）。
func officeLoadSample(tab string) {
	p := officePaneFor2(tab)
	wb := officeSampleWorkbook()
	switch tab {
	case "excel":
		if data, err := office.WriteXLSXBytes(wb); err == nil {
			if f, err := office.OpenBytesKind("示例.xlsx", office.KindExcel, data); err == nil {
				p.file = f
				p.size = len(data)
				p.path = "内存生成（WriteXLSXBytes → ReadXLSXBytes）"
				p.sheet = f.Sheet.First().Name
				p.msg = fmt.Sprintf("已载入示例工作簿：%d 张工作表", len(f.Sheet.Sheets))
			}
		}
	case "word":
		doc := officeSampleDoc(wb)
		if data, err := office.WriteDOCXBytes(doc); err == nil {
			if f, err := office.OpenBytesKind("示例.docx", office.KindWord, data); err == nil {
				p.file = f
				p.size = len(data)
				p.path = "内存生成（WriteDOCXBytes → ReadDOCXBytes）"
				p.msg = fmt.Sprintf("已载入示例文档：%d 个内容块", len(f.Word.Blocks))
			}
		}
	case "pdf":
		doc := officeBuildPDF(wb)
		data, err := office.WritePDFBytes(doc, office.PDFOptions{Title: "季度经营简报"})
		if err == nil {
			if f, err := office.OpenBytesKind("示例.pdf", office.KindPDF, data); err == nil {
				p.file = f
				p.size = len(data)
				p.path = "内存生成（PDF 写入器 → 解析器）"
				p.msg = fmt.Sprintf("已生成 PDF：%d 页 · 字体已子集嵌入", len(f.PDF.Pages))
			}
		}
	}
	if p.file == nil {
		p.msg = "示例生成失败"
	}
}

// officePaneFor2 取状态（不触发示例生成，避免递归）。
func officePaneFor2(tab string) *officePane {
	if p, ok := officePanes[tab]; ok && p != nil {
		return p
	}
	p := &officePane{zoom: 1}
	officePanes[tab] = p
	return p
}

// ---- 示例数据 ----

// officeSampleWorkbook 构造示例工作簿：销售明细 + 区域汇总。
func officeSampleWorkbook() *office.Spreadsheet {
	detail := office.Sheet{Name: "销售明细", Header: true}
	head := []string{"月份", "区域", "产品", "订单数", "金额", "同比"}
	for i, h := range head {
		detail.SetCell(0, i, office.Cell{Text: h, Bold: true})
	}
	type row struct {
		month, region, product string
		orders                 float64
		amount                 float64
		yoy                    float64
	}
	rows := []row{
		{"一月", "华东", "标准版", 128, 1890000, 0.182},
		{"一月", "华南", "标准版", 96, 1420000, 0.094},
		{"一月", "华北", "专业版", 54, 1080000, 0.236},
		{"二月", "华东", "专业版", 112, 2240000, 0.214},
		{"二月", "华南", "标准版", 88, 1210000, 0.052},
		{"二月", "华北", "标准版", 61, 980000, 0.118},
		{"三月", "华东", "企业版", 76, 3040000, 0.327},
		{"三月", "华南", "专业版", 102, 2040000, 0.148},
		{"三月", "华北", "标准版", 69, 1105000, 0.083},
		{"四月", "华东", "企业版", 81, 3240000, 0.291},
		{"四月", "华南", "专业版", 97, 1940000, 0.126},
		{"四月", "华北", "标准版", 72, 1160000, 0.097},
	}
	for i, r := range rows {
		line := i + 1
		detail.Set(line, 0, r.month)
		detail.Set(line, 1, r.region)
		detail.Set(line, 2, r.product)
		detail.SetNum(line, 3, r.orders)
		detail.SetNum(line, 4, r.amount)
		// 同比是百分比文案：写成文本（否则 xlsx 往返后会变回 0.182 这种纯数值）
		detail.Set(line, 5, fmt.Sprintf("%.1f%%", r.yoy*100))
	}

	// 汇总表：按区域累加（演示 Sheet 的读 / 算能力）
	sum := office.Sheet{Name: "区域汇总", Header: true}
	for i, h := range []string{"区域", "订单数", "金额", "客单价"} {
		sum.SetCell(0, i, office.Cell{Text: h, Bold: true})
	}
	regions := []string{"华东", "华南", "华北"}
	for i, rg := range regions {
		var orders, amount float64
		for _, r := range rows {
			if r.region == rg {
				orders += r.orders
				amount += r.amount
			}
		}
		line := i + 1
		sum.Set(line, 0, rg)
		sum.SetNum(line, 1, orders)
		sum.SetNum(line, 2, amount)
		sum.SetNum(line, 3, amount/orders)
	}
	return &office.Spreadsheet{Sheets: []office.Sheet{detail, sum}}
}

// officeSampleDoc 构造示例 Word 文档（含标题 / 段落 / 列表 / 引用 / 表格）。
func officeSampleDoc(wb *office.Spreadsheet) *office.WordDoc {
	doc := &office.WordDoc{Title: "季度经营简报"}
	doc.Blocks = append(doc.Blocks,
		office.Heading(1, "季度经营简报"),
		office.Paragraph("本季度整体营收同比增长 21%，其中华东区贡献最大；企业版占比从 12% 提升到 24%，客单价同步上行。"),
		office.Heading(2, "关键结论"),
		office.Bullet("新签客户 42 家，华东区占 21 家"),
		office.Bullet("复购率 68%，较上季度 +5 个百分点"),
		office.Bullet("企业版营收占比 24%，首次超过标准版"),
		office.Block{Kind: office.BlockQuote, Text: "数据来源：内部 BI 系统 · 统计口径为已确认收入"},
		office.Heading(2, "区域明细"),
	)
	// 把工作表的「区域汇总」搬进 Word（同一个模型两处复用）
	if wb != nil {
		for _, sh := range wb.Sheets {
			if sh.Name == "区域汇总" {
				doc.Blocks = append(doc.Blocks, office.TableFromSheet(&sh, 20))
			}
		}
	}
	doc.Blocks = append(doc.Blocks,
		office.Heading(2, "下季度计划"),
		office.Paragraph("扩招华东交付团队 6 人，把企业版交付周期从 21 天压缩到 14 天以内。"),
	)
	return doc
}

// officeBuildPDF 用 PDF 写入器直接绘制一份简报（含中文字体子集嵌入）。
func officeBuildPDF(wb *office.Spreadsheet) *office.PDFDoc {
	doc := &office.PDFDoc{Title: "季度经营简报"}
	pg := office.NewPDFPage(0, 0) // A4 595×842 pt

	// 页头
	pg.TextBold(56, 52, 22, "季度经营简报")
	pg.TextColor(56, 82, 10.5, "GUIS office · PDF 写入器演示（中文字体子集嵌入）", "#8b95a7")
	pg.Line(56, 100, 539, 100, 1, "#e2e8f0")

	// 摘要块
	pg.Rect(56, 116, 483, 54, 0, "", "#f1f5f9")
	pg.TextBold(68, 128, 12, "本季度营收同比增长 21%")
	pg.TextColor(68, 148, 10, "企业版占比 24%，华东区贡献最大；复购率 68%（+5pp）", "#64748b")

	// 明细表
	sh := wb.SheetByName("区域汇总")
	cols := []float32{56, 160, 300, 420}
	headY := float32(196)
	pg.Rect(56, headY, 483, 22, 0, "", "#eef2fa")
	for i, c := range cols {
		pg.TextBold(c+8, headY+6, 10.5, sh.Cell(0, i).Text)
	}
	pg.Line(56, headY+22, 539, headY+22, 0.8, "#c9d1de")

	y := headY + 22
	for r := 1; r < len(sh.Rows); r++ {
		if r%2 == 0 {
			pg.Rect(56, y, 483, 22, 0, "", "#f8fafc")
		}
		for i, c := range cols {
			pg.Text(c+8, y+6, 10.5, sh.Cell(r, i).Text)
		}
		y += 22
	}
	pg.Line(56, y, 539, y, 0.8, "#c9d1de")

	// 明细列表（用文本行表）
	detail := wb.SheetByName("销售明细")
	listY := y + 48
	pg.TextColor(56, listY, 9.5, "月份     区域     产品      订单数      金额", "#94a3b8")
	listY += 16
	for r := 1; r < len(detail.Rows) && r <= 8; r++ {
		line := fmt.Sprintf("%-8s %-8s %-9s %-11s %s",
			detail.Cell(r, 0).Text, detail.Cell(r, 1).Text, detail.Cell(r, 2).Text,
			detail.Cell(r, 3).Text, detail.Cell(r, 4).Text)
		pg.TextColor(56, listY, 9.5, line, "#334155")
		listY += 15
	}

	// 页脚
	pg.Line(56, 790, 539, 790, 0.8, "#e2e8f0")
	pg.TextColor(56, 798, 9, "GUIS office · 由 PDF 写入器生成（第 1 页）", "#94a3b8")
	doc.Pages = append(doc.Pages, *pg)
	return doc
}

// ---- 页面 ----

// officeTabs 办公页页签。
func officeTabs() *ui.Node {
	type tabDef struct{ key, label, hint string }
	tabs := []tabDef{
		{"excel", "Excel 表格", "读写 xlsx"},
		{"word", "Word 文档", "读写 docx"},
		{"pdf", "PDF 文档", "生成 / 解析"},
	}
	keys := make([]string, 0, len(tabs))
	labels := make([]string, 0, len(tabs))
	for _, t := range tabs {
		keys = append(keys, t.key)
		labels = append(labels, t.label)
	}
	return ui.Row(
		ui.TabsBar(keys, labels, officeTab, func(k string) {
			officeTab = k
			ui.RequestRedraw()
		}),
		ui.Spacer(),
		ui.Tag("新能力", "#a5b4fc"),
	).St(ui.St().GapOf(12).PadOnly(26, 26, 0, 26)).AlignTo(ui.AlignCenter)
}

// officesCase 办公文档页（由 Content 在 Current == "office" 时调用）。
func officeCase() *ui.Node {
	p := officePaneFor(officeTab)
	if p.preview {
		return ui.Column(officeTabs(),
			officePreviewNode(p), // 预览层（含「返回」按钮）
		).St(ui.St().GrowTo(1).Hp(100))
	}
	return ui.Column(
		officeTabs(),
		ui.Scroll(
			ui.Column(
				officeHeadCard(p),
				officeBody(p),
				officeHintCard(),
			).St(ui.St().Padding(26).GapOf(16)),
		).St(ui.St().GrowTo(1)).K("scroll-office-"+officeTab),
	).St(ui.St().GrowTo(1).Hp(100))
}

// officeHeadCard 顶部信息卡：文件信息 + 操作按钮。
func officeHeadCard(p *officePane) *ui.Node {
	kind := "未知"
	summary := ""
	if p.file != nil {
		kind = p.file.Kind.String()
		summary = p.file.Summary()
	}
	size := "—"
	if p.size > 0 {
		size = ui.FormatBytes(int64(p.size))
	}
	info := ui.Column(
		ui.Text("办公能力 · "+kind).St(ui.St().Text(19, 700, ui.ColText)),
		ui.Text(p.path).St(ui.St().Text(11.5, 400, ui.ColTextMuted).WrapOn()),
		ui.Row(
			ui.Tag(size, "#818cf8"),
			ui.Tag(summary, "#22d3ee"),
			ui.If(p.msg != "", func() *ui.Node {
				return ui.Tag(p.msg, "#34d399")
			}),
		).St(ui.St().GapOf(8)),
	).St(ui.St().GapOf(6)).Grow(1)

	actions := ui.Row(
		ui.If(ui.HasCap(ui.CapDialogs), func() *ui.Node {
			return ui.Btn("打开文档…", ui.BtnSecondary, officeOpenFile)
		}),
		ui.Btn("重新生成示例", ui.BtnGhost, func() {
			officeLoadSample(officeTab)
			toastShow("已重新生成示例：" + strings.ToUpper(officeTab))
			ui.RequestRedraw()
		}),
		ui.Btn("导出…", ui.BtnPrimary, func() { officeExport(officeTab) }),
	).St(ui.St().GapOf(8))

	return ui.Card(
		ui.Row(info, actions).St(ui.St().GapOf(16)).AlignTo(ui.AlignCenter),
	)
}

// officeBody 按页签渲染主体。
func officeBody(p *officePane) *ui.Node {
	switch officeTab {
	case "excel":
		return officeExcelBody(p)
	case "word":
		return officeWordBody(p)
	default:
		return officePDFBody(p)
	}
}

// officeExcelBody Excel：工作表页签 + 表格 + 数据操作。
func officeExcelBody(p *officePane) *ui.Node {
	if p.file == nil || p.file.Sheet == nil {
		return ui.Empty("没有数据", "点右上角「重新生成示例」或「打开文档…」")
	}
	wb := p.file.Sheet
	sh := wb.SheetByName(p.sheet)
	if sh == nil {
		sh = wb.First()
	}
	cur := sh.Name

	// 数据操作按钮：排序 / 追加一行 / 合计
	toolbar := ui.Row(
		ui.Text("数据操作").St(ui.St().Text(12.5, 500, ui.ColTextDim)),
		ui.Btn("按金额排序", ui.BtnGhost, func() {
			sh.SortRowsBy(4, false)
			p.msg = "已按「金额」降序重排（Sheet.SortRowsBy）"
			toastShow(p.msg)
			ui.RequestRedraw()
		}),
		ui.Btn("追加一行", ui.BtnGhost, func() {
			r := len(sh.Rows)
			sh.Set(r, 0, "五月")
			sh.Set(r, 1, "华东")
			sh.Set(r, 2, "标准版")
			sh.SetNum(r, 3, 64)
			sh.SetNum(r, 4, 960000)
			sh.Set(r, 5, "+8.2%")
			sh.Header = true
			p.msg = "已追加第 " + fmt.Sprint(r+1) + " 行（Sheet.Set / SetNum）"
			toastShow(p.msg)
			ui.RequestRedraw()
		}),
		ui.Spacer(),
		ui.Tag(fmt.Sprintf("订单合计 %s · 金额合计 %s",
			office.FormatNumber(sh.SumColumn(3)), office.FormatNumber(sh.SumColumn(4))), "#fbbf24"),
	).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter)

	view := ui.SpreadsheetView("office-sheet", wb, cur, func(name string) {
		p.sheet = name
	})
	return ui.Column(toolbar, ui.Card(view)).St(ui.St().GapOf(12))
}

// officeWordBody Word：文档流 + 结构信息。
func officeWordBody(p *officePane) *ui.Node {
	if p.file == nil || p.file.Word == nil {
		return ui.Empty("没有内容", "点右上角「重新生成示例」或「打开文档…」")
	}
	doc := p.file.Word
	var heads, paras, lists, tables int
	for _, b := range doc.Blocks {
		switch b.Kind {
		case office.BlockHeading:
			heads++
		case office.BlockListItem:
			lists++
		case office.BlockTable:
			tables++
		default:
			paras++
		}
	}
	stat := ui.Row(
		ui.Tag(fmt.Sprintf("标题 %d", heads), "#818cf8"),
		ui.Tag(fmt.Sprintf("段落 %d", paras), "#22d3ee"),
		ui.Tag(fmt.Sprintf("列表 %d", lists), "#f472b6"),
		ui.Tag(fmt.Sprintf("表格 %d", tables), "#34d399"),
		ui.Spacer(),
		ui.Btn("打印预览", ui.BtnGhost, func() {
			p.preview = true
			ui.RequestRedraw()
		}),
	).St(ui.St().GapOf(8)).AlignTo(ui.AlignCenter)

	// 纸张感：给文档流套一层浅色底 + 内边距
	paper := ui.Column(ui.WordView("office-word", doc)).
		St(ui.St().Padding(28).GapOf(4).R(12).BgC(ui.ColBgSoft).Stroke(1, ui.ColStroke))
	return ui.Column(stat, paper).St(ui.St().GapOf(12))
}

// officePDFBody PDF：缩放工具条 + 页面流。
func officePDFBody(p *officePane) *ui.Node {
	if p.file == nil || p.file.PDF == nil {
		return ui.Empty("没有页面", "点右上角「重新生成示例」或「打开文档…」")
	}
	doc := p.file.PDF
	if p.zoom <= 0 {
		p.zoom = 1
	}
	page := doc.Pages[0]
	zoomLabel := fmt.Sprintf("%d%%", int(p.zoom*100+0.5))
	toolbar := ui.Row(
		ui.Text("PDF 页面").St(ui.St().Text(12.5, 500, ui.ColTextDim)),
		ui.Btn("−", ui.BtnGhost, func() {
			if p.zoom > 0.5 {
				p.zoom -= 0.1
			}
			ui.RequestRedraw()
		}),
		ui.Text(zoomLabel).St(ui.St().Text(12, 600, ui.ColText).W(48).TAlign(ui.AlignCenter)),
		ui.Btn("+", ui.BtnGhost, func() {
			if p.zoom < 2 {
				p.zoom += 0.1
			}
			ui.RequestRedraw()
		}),
		ui.Btn("适应宽度", ui.BtnGhost, func() {
			w := ui.ViewportW() - 120
			if w > 300 && page.W > 0 {
				p.zoom = w / page.W
				if p.zoom > 2 {
					p.zoom = 2
				}
			}
			ui.RequestRedraw()
		}),
		ui.Spacer(),
		ui.Tag(fmt.Sprintf("%d 页 · %.0f×%.0f pt", len(doc.Pages), page.W, page.H), "#818cf8"),
		ui.Btn("打印预览", ui.BtnGhost, func() {
			p.preview = true
			ui.RequestRedraw()
		}),
	).St(ui.St().GapOf(8)).AlignTo(ui.AlignCenter)

	pages := ui.Row(ui.PDFView("office-pdf", doc, p.zoom), ui.Spacer()).
		St(ui.St().GapOf(20))
	return ui.Column(toolbar, ui.Card(pages)).St(ui.St().GapOf(12))
}

// officeHintCard 底部说明：能力边界与用法。
func officeHintCard() *ui.Node {
	lines := []string{
		"Excel：读写 .xlsx（共享字符串 / inlineStr / 数值 / 日期格式 / 表头加粗），不含图表与公式计算（只读缓存值）。",
		"Word：读写 .docx（标题 / 段落 / 列表 / 引用 / 表格），图片与域代码暂不处理。",
		"PDF：写入器按 pt 坐标绘制文本 / 线 / 矩形，并把系统 TrueType 中文字体子集嵌入（只含用到的字形）；解析器按内容流还原文本位置。",
		"打印：任何文档都能走 ui.OfficePrintable(file) → ui.PrintPreview / ui.Print；ui.ExportPDF 可把打印稿直接导出成 PDF。",
	}
	items := make([]*ui.Node, 0, len(lines))
	for _, l := range lines {
		items = append(items, ui.Row(
			ui.Box().St(ui.St().W(5).H(5).R(3).BgC(ui.ColPrimary)),
			ui.Text(l).St(ui.St().Text(11.5, 400, ui.ColTextMuted).WrapOn().GrowTo(1)),
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignStart))
	}
	return ui.Card(ui.Title("能力边界", "零依赖 · 纯标准库实现"), ui.Column(items...).St(ui.St().GapOf(8)))
}

// ---- 打开 / 导出 / 预览 ----

// officeOpenFile 打开本地文档（仅在支持文件对话框的平台出现入口）。
func officeOpenFile() {
	path, ok := ui.OpenFileDialog("打开办公文档",
		"办公文档|*.xlsx;*.docx;*.pdf|Excel 工作簿|*.xlsx|Word 文档|*.docx|PDF 文档|*.pdf|所有文件|*.*")
	if !ok || path == "" {
		return
	}
	f, err := office.Open(path)
	if err != nil {
		toastShow("打开失败：" + err.Error())
		return
	}
	tab := "excel"
	switch f.Kind {
	case office.KindWord:
		tab = "word"
	case office.KindPDF:
		tab = "pdf"
	}
	officeTab = tab
	p := officePaneFor2(tab)
	p.file = f
	p.path = path
	p.size = fileSize(path)
	p.preview = false
	p.zoom = 1
	if f.Kind == office.KindExcel && f.Sheet.First() != nil {
		p.sheet = f.Sheet.First().Name
	}
	p.msg = "已打开：" + f.Kind.String()
	toastShow("已打开 " + f.Kind.String() + "：" + f.Summary())
	ui.RequestRedraw()
}

// officeExport 导出当前文档：同格式另存，PDF 走 ui.ExportPDF（可导出打印稿）。
func officeExport(tab string) {
	p := officePaneFor(tab)
	if p.file == nil {
		toastShow("没有可导出的内容")
		return
	}
	if !ui.HasCap(ui.CapDialogs) {
		toastShow("当前平台没有文件对话框，导出示例请参考 ui.ExportPDF / office.WriteXLSX")
		return
	}
	switch tab {
	case "excel":
		path, ok := ui.SaveFileDialog("导出 Excel", "Excel 工作簿|*.xlsx", ".xlsx")
		if !ok || path == "" {
			return
		}
		if err := office.WriteXLSX(path, p.file.Sheet); err != nil {
			toastShow("导出失败：" + err.Error())
			return
		}
		toastShow("已导出：" + path)
	case "word":
		path, ok := ui.SaveFileDialog("导出 Word", "Word 文档|*.docx", ".docx")
		if !ok || path == "" {
			return
		}
		if err := office.WriteDOCX(path, p.file.Word); err != nil {
			toastShow("导出失败：" + err.Error())
			return
		}
		toastShow("已导出：" + path)
	default:
		path, ok := ui.SaveFileDialog("导出 PDF", "PDF 文档|*.pdf", ".pdf")
		if !ok || path == "" {
			return
		}
		// 打印稿 → PDF（与屏幕上看到的排版一致）
		if err := ui.ExportPDF(path, "季度经营简报", ui.OfficePrintable(p.file)); err != nil {
			toastShow("导出失败：" + err.Error())
			return
		}
		toastShow("已导出：" + path)
	}
	ui.RequestRedraw()
}

// fileSize 文件字节数（读不到返回 0）。
func fileSize(path string) int {
	if st, err := os.Stat(path); err == nil {
		return int(st.Size())
	}
	return 0
}

// officePreviewNode 打印预览（替换页面主体，带返回按钮）。
func officePreviewNode(p *officePane) *ui.Node {
	doc := ui.OfficePrintable(p.file)
	back := ui.Row(
		ui.Btn("← 返回文档", ui.BtnGhost, func() {
			p.preview = false
			ui.RequestRedraw()
		}),
		ui.Spacer(),
		ui.Text("打印预览 · 与「打印…」「导出 PDF」使用同一份页面绘制代码").
			St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
	).St(ui.St().GapOf(12).PadOnly(26, 26, 0, 26)).AlignTo(ui.AlignCenter)
	return ui.Column(back,
		ui.PrintPreview("office-preview", doc, 0.62).St(ui.St().GrowTo(1)),
	).St(ui.St().GrowTo(1).Hp(100))
}
