// guis — 办公文档渲染（Excel / Word / PDF → 组件树 + 打印 / 导出）
//
// 与 internal/office（纯数据 + 读写）配合，本文件负责「展示」这一层：
//
//	ui.SheetTable("k", sheet, opts)   工作表 → 表格组件（表头 / 斑马纹 / 数值右对齐）
//	ui.SpreadsheetView("k", wb)       工作簿 → 页签 + 表格
//	ui.WordView("k", doc)             Word → 标题 / 段落 / 列表 / 引用 / 表格流
//	ui.PDFView("k", doc, 1.0)         PDF → 每页白纸 + 绝对定位文本（可缩放）
//	ui.OfficePrintable(file)          任意办公文档 → Printable（交给 Print / PrintPreview）
//	ui.ExportPDF("out.pdf", "标题", doc)  任意 Printable → PDF（自动嵌入中文字体）
//
// 为什么不用位图渲染：本框架的文本光栅化是逐字进图集的，一个 20 页 PDF 转成图片
// 会吃掉大量显存且缩放糊；按「文本 + 矩形」重排成组件树，缩放始终清晰，
// 而且能直接复用主题色（深色模式下也能看清）。
package ui

import (
	"fmt"
	"strings"

	"guis/internal/office"
)

// ---- 工作表 ----

// SheetViewOptions 工作表渲染选项（零值即默认）。
type SheetViewOptions struct {
	// MaxRows / MaxCols 最多渲染多少行 / 列（默认 300 / 24，防止超大表拖慢布局）。
	MaxRows, MaxCols int
	// RowH 行高（默认 30）。
	RowH float32
	// FontSize 字号（默认 12）。
	FontSize float32
	// ShowColName 没有表头时是否用 A/B/C 作为列名（默认 true）。
	ShowColName bool
	// OnCell 单元格点击回调（行、列从 0 开始，-1 表示表头行）。
	OnCell func(row, col int)
}

// SheetTable 把一张工作表渲染成表格组件。
func SheetTable(key string, sh *office.Sheet, opt SheetViewOptions) *Node {
	if sh == nil {
		return Empty("没有工作表", "打开一个 .xlsx 文件即可预览")
	}
	if opt.MaxRows <= 0 {
		opt.MaxRows = 300
	}
	if opt.MaxCols <= 0 {
		opt.MaxCols = 24
	}
	if opt.RowH <= 0 {
		opt.RowH = 30
	}
	if opt.FontSize <= 0 {
		opt.FontSize = 12
	}

	totalRows := len(sh.Rows)
	if totalRows > opt.MaxRows {
		totalRows = opt.MaxRows
	}
	cols := sheetColCount(sh)
	if cols > opt.MaxCols {
		cols = opt.MaxCols
	}
	if cols == 0 {
		return Empty("空工作表", "这张表没有内容")
	}

	// 列宽：按内容长度估算，限制在 72~260 之间
	widths := make([]float32, cols)
	for c := 0; c < cols; c++ {
		maxLen := 0
		for r := 0; r < totalRows; r++ {
			if n := len([]rune(sh.Cell(r, c).Text)); n > maxLen {
				maxLen = n
			}
		}
		w := float32(maxLen)*opt.FontSize*0.62 + 28
		if w < 78 {
			w = 78
		}
		if w > 260 {
			w = 260
		}
		widths[c] = w
	}

	headerRow := -1
	if sh.Header && totalRows > 0 {
		headerRow = 0
	}

	// 表头
	headCells := make([]*Node, 0, cols)
	for c := 0; c < cols; c++ {
		txt := ""
		if headerRow >= 0 {
			txt = sh.Cell(headerRow, c).Text
		} else if opt.ShowColName {
			txt = colLetter(c)
		}
		cell := Text(txt).St(St().Text(opt.FontSize-0.5, 600, ColTextMuted).
			W(widths[c]).PadXY(12, 8).WrapOn())
		col := c
		cell.Tap(func() {
			if opt.OnCell != nil {
				opt.OnCell(headerRow, col)
			}
		})
		headCells = append(headCells, cell)
	}
	head := Row(headCells...).St(St().GapOf(0).BgC(ColHeader)).AlignTo(AlignCenter)

	start := 0
	if headerRow == 0 {
		start = 1
	}
	rows := make([]*Node, 0, totalRows-start)
	for r := start; r < totalRows; r++ {
		bg := ColRow
		if (r-start)%2 == 1 {
			bg = ColRowAlt
		}
		cells := make([]*Node, 0, cols)
		for c := 0; c < cols; c++ {
			cell := sh.Cell(r, c)
			txt := cell.Text
			color := ColTextDim
			weight := int32(400)
			if cell.Bold {
				color, weight = ColText, 600
			}
			t := Text(txt).St(St().Text(opt.FontSize, weight, color).
				W(widths[c]).PadXY(12, 6).WrapOn())
			if cell.Num {
				t.St(St().TAlign(AlignEnd))
			}
			if cell.Fill != "" && cell.Num { // 样式底色只在有底色的数值单元格上体现
				t.St(St().BgC(cell.Fill))
			}
			row, colN := r, c
			t.Tap(func() {
				if opt.OnCell != nil {
					opt.OnCell(row, colN)
				}
			})
			cells = append(cells, t)
		}
		minH := opt.RowH
		if sh.Rows[r] != nil && rowNeedsWrap(sh.Rows[r], widths, opt.FontSize) {
			minH = 0 // 交给内容撑高
		}
		rw := Row(cells...).St(St().GapOf(0).BgC(bg).MinHeight(opt.RowH))
		if minH > 0 {
			rw.St(St().H(minH))
		}
		rows = append(rows, rw)
	}

	body := Column(append([]*Node{Divider(), head, Divider()}, rows...)...).
		St(St().R(12).Stroke(1, ColStroke).Clipped())
	foot := Row(
		Text(fmt.Sprintf("%d 行 × %d 列", totalRows, cols)).
			St(St().Text(11, 400, ColPlaceholder)),
		Spacer(),
		Text(truncatedHint(totalRows, len(sh.Rows), cols, sheetColCount(sh))).
			St(St().Text(11, 400, ColPlaceholder)),
	).St(St().GapOf(8))
	return Column(body, foot).St(St().GapOf(8)).K(key)
}

// rowNeedsWrap 该行是否有单元格文本会超出列宽（需要多行高度）。
func rowNeedsWrap(row []office.Cell, widths []float32, fontSize float32) bool {
	for i, c := range row {
		if i >= len(widths) {
			break
		}
		if float32(len([]rune(c.Text)))*fontSize*0.62 > widths[i]-24 {
			return true
		}
	}
	return false
}

// sheetColCount 工作表最大列数。
func sheetColCount(sh *office.Sheet) int {
	n := sh.Cols
	for _, row := range sh.Rows {
		if len(row) > n {
			n = len(row)
		}
	}
	return n
}

// truncatedHint 超出行 / 列上限时的提示文案（不超出则为空）。
func truncatedHint(shownRows, allRows, shownCols, allCols int) string {
	var parts []string
	if shownRows < allRows {
		parts = append(parts, fmt.Sprintf("仅渲染前 %d 行（共 %d 行）", shownRows, allRows))
	}
	if shownCols < allCols {
		parts = append(parts, fmt.Sprintf("前 %d 列（共 %d 列）", shownCols, allCols))
	}
	return strings.Join(parts, " · ")
}

// colLetter 0 → A，25 → Z，26 → AA。
func colLetter(c int) string {
	if c < 0 {
		c = 0
	}
	var buf [8]byte
	i := len(buf)
	for {
		i--
		buf[i] = byte('A' + c%26)
		c = c/26 - 1
		if c < 0 {
			break
		}
	}
	return string(buf[i:])
}

// SpreadsheetView 工作簿：顶部工作表页签 + 当前表。
// active 由调用方持有（页签点击后改值并 RequestRedraw）。
func SpreadsheetView(key string, wb *office.Spreadsheet, active string, onPick func(name string)) *Node {
	if wb == nil || len(wb.Sheets) == 0 {
		return Empty("没有工作簿", "打开一个 .xlsx 文件即可预览")
	}
	names := make([]string, 0, len(wb.Sheets))
	for _, sh := range wb.Sheets {
		names = append(names, sh.Name)
	}
	cur := wb.SheetByName(active)
	if cur == nil {
		cur = wb.First()
	}
	activeName := cur.Name

	head := Row(
		TabsBar(names, names, activeName, func(name string) {
			if onPick != nil {
				onPick(name)
			}
			RequestRedraw()
		}),
		Spacer(),
		Tag(fmt.Sprintf("%d 张表", len(names)), ColTextMuted),
	).St(St().GapOf(12).PadOnly(0, 0, 10, 0)).AlignTo(AlignCenter)

	return Column(head, SheetTable(key+"-"+activeName, cur, SheetViewOptions{})).
		St(St().GapOf(10)).K(key)
}

// ---- Word ----

// headingSize 标题级别 → 字号。
func headingSize(level int) float32 {
	switch level {
	case 1:
		return 25
	case 2:
		return 21
	case 3:
		return 18
	case 4:
		return 16
	case 5:
		return 15
	}
	return 14
}

// WordView 把 Word 文档渲染成文档流。
func WordView(key string, doc *office.WordDoc) *Node {
	if doc == nil || len(doc.Blocks) == 0 {
		return Empty("没有内容", "打开一个 .docx 文件即可预览")
	}
	items := make([]*Node, 0, len(doc.Blocks)+1)
	if doc.Title != "" {
		items = append(items, Text(doc.Title).
			St(St().Text(13, 500, ColTextMuted).PadOnly(0, 0, 2, 0)))
	}
	for _, blk := range doc.Blocks {
		switch blk.Kind {
		case office.BlockHeading:
			n := Text(blk.Text).St(St().Text(headingSize(blk.Level), 700, ColText).
				WrapOn().PadOnly(map[bool]float32{true: 14, false: 12}[blk.Level <= 2], 0, 6, 0))
			if blk.Level == 1 {
				n.St(St().PadOnly(6, 0, 10, 0))
			}
			items = append(items, n)
		case office.BlockListItem:
			indent := float32(18 * (blk.Level - 1))
			items = append(items, Row(
				Box().St(St().W(5).H(5).R(3).BgC(ColPrimary)),
				Text(blk.Text).St(St().Text(13.5, 400, ColTextDim).WrapOn().GrowTo(1)),
			).St(St().GapOf(10).PadOnly(4, 0, 4, 8+indent)).AlignTo(AlignStart))
		case office.BlockQuote:
			items = append(items, Row(
				Box().St(St().W(3).BgC(ColPrimary).MinHeight(18)),
				Text(blk.Text).St(St().Text(13, 400, ColTextMuted).WrapOn().GrowTo(1)),
			).St(St().GapOf(10).PadOnly(6, 8, 6, 10)))
		case office.BlockTable:
			items = append(items, wordTableView(key, blk))
		default:
			w := int32(400)
			if blk.Bold {
				w = 600
			}
			items = append(items, Text(blk.Text).St(St().Text(13.5, w, ColTextDim).
				WrapOn().Lh(1.75).PadOnly(2, 0, 2, 0)))
		}
	}
	return Column(items...).St(St().GapOf(2).MaxWidth(860)).K(key)
}

// wordTableView 表格块 → 表格组件。
func wordTableView(key string, blk office.Block) *Node {
	sh := office.SheetFromBlock(blk, "表格")
	opt := SheetViewOptions{RowH: 32, MaxRows: 200, MaxCols: 12}
	return SheetTable(key+"-tbl", &sh, opt)
}

// ---- PDF ----

// PDFView 把 PDF 渲染成页面卡片流（zoom 为 pt → 逻辑像素的缩放，默认 1）。
func PDFView(key string, doc *office.PDFDoc, zoom float32) *Node {
	if doc == nil || len(doc.Pages) == 0 {
		return Empty("没有页面", "打开一个 .pdf 文件即可预览")
	}
	if zoom <= 0 {
		zoom = 1
	}
	pages := make([]*Node, 0, len(doc.Pages))
	for _, pg := range doc.Pages {
		pages = append(pages, pdfPageNode(pg, zoom))
	}
	return Row(pages...).St(St().GapOf(20).Padding(2)).K(key)
}

// pdfPageNode 单页：白纸 + 绝对定位的文本 / 线条 / 矩形。
func pdfPageNode(pg office.PDFPage, zoom float32) *Node {
	w, h := pg.W*zoom, pg.H*zoom
	if w <= 0 || h <= 0 {
		w, h = PageW96, PageH96
	}
	items := make([]*Node, 0, len(pg.Items))
	for _, it := range pg.Items {
		switch it.Kind {
		case office.PDFText:
			color := it.Color
			if color == "" {
				color = "#1f2733"
			}
			weight := int32(400)
			if it.Bold {
				weight = 600
			}
			items = append(items, Text(it.Text).St(St().
				Text(it.Size*zoom, weight, color).
				AbsTL(it.X*zoom, it.Y*zoom).NoFocusRing()))
		case office.PDFLine:
			lw := it.Stroke * zoom
			if lw < 0.6 {
				lw = 0.6
			}
			dx := it.X2 - it.X
			dy := it.Y2 - it.Y
			if absF(dy) < 0.5 { // 水平线
				items = append(items, Box().St(St().
					AbsTL(minF32(it.X, it.X2)*zoom, it.Y*zoom-lw/2).
					W(absF(dx)*zoom+0.5).H(lw).BgC(colorOrStr(it.Color, "#9aa3b2")).NoFocusRing()))
			} else if absF(dx) < 0.5 { // 垂直线
				items = append(items, Box().St(St().
					AbsTL(it.X*zoom-lw/2, minF32(it.Y, it.Y2)*zoom).
					W(lw).H(absF(dy)*zoom+0.5).BgC(colorOrStr(it.Color, "#9aa3b2")).NoFocusRing()))
			}
		case office.PDFRect:
			box := Box().St(St().AbsTL(it.X*zoom, it.Y*zoom).
				W(it.W * zoom).H(it.H * zoom).NoFocusRing())
			if it.Fill != "" {
				box.St(St().BgC(it.Fill))
			}
			if it.Stroke > 0 {
				box.St(St().Stroke(it.Stroke*zoom, colorOrStr(it.Color, "#c9d1de")))
			}
			items = append(items, box)
		case office.PDFImage:
			items = append(items, Stack(
				Box().St(St().AbsFill().BgC("#eef1f6")),
				Text("图片").St(St().AbsFill().Text(11, 400, "#9aa3b2").
					TAlign(AlignCenter).CenterYOn()),
			).St(St().AbsTL(it.X*zoom, it.Y*zoom).W(it.W*zoom).H(it.H*zoom).
				Stroke(1, "#d6dce6").NoFocusRing()))
		}
	}
	return Stack(items...).St(St().W(w).H(h).BgC("#ffffff").Clipped().
		Shadow(18, 6, 0, "rgba(0,0,0,0.35)").NoFocusRing())
}

// colorOrStr 空颜色回退默认值。
func colorOrStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// ---- 统一入口 ----

// OfficeView 按文件类型渲染（Excel / Word / PDF 自动分派）。
func OfficeView(key string, f *office.File) *Node {
	if f == nil {
		return Empty("未打开文档", "支持 .xlsx / .docx / .pdf")
	}
	switch f.Kind {
	case office.KindExcel:
		return SpreadsheetView(key, f.Sheet, "", func(string) {})
	case office.KindWord:
		return WordView(key, f.Word)
	case office.KindPDF:
		return PDFView(key, f.PDF, 1)
	}
	return Empty("不支持的格式", f.Path)
}

// ---- 打印 / 导出 ----

// PDFPrintable 把 PDF 文档适配成 Printable（打印 / 打印预览用）。
func PDFPrintable(doc *office.PDFDoc) Printable {
	return &pdfPrintable{doc: doc}
}

type pdfPrintable struct{ doc *office.PDFDoc }

// office.File 无标题时兜底的任务名。
func (p *pdfPrintable) Title() string {
	if p == nil || p.doc == nil {
		return ""
	}
	return strings.TrimSpace(p.doc.Title)
}

func (p *pdfPrintable) Pages() int {
	if p == nil || p.doc == nil {
		return 0
	}
	return len(p.doc.Pages)
}

func (p *pdfPrintable) DrawPage(c *PrintCanvas, page int) {
	if p == nil || p.doc == nil || page < 0 || page >= len(p.doc.Pages) {
		return
	}
	pg := p.doc.Pages[page]
	k := PageW96 / pg.W // PDF 页宽 → 96dpi 页面宽
	for _, it := range pg.Items {
		switch it.Kind {
		case office.PDFText:
			c.Text(it.X*k, it.Y*k, it.Size*k, it.Text, colorOrStr(it.Color, "#111111"))
		case office.PDFLine:
			c.Line(it.X*k, it.Y*k, it.X2*k, it.Y2*k, it.Stroke*k, colorOrStr(it.Color, "#999999"))
		case office.PDFRect:
			c.Rect(it.X*k, it.Y*k, it.W*k, it.H*k, it.Stroke*k, it.Color, it.Fill)
		}
	}
}

// SheetPrintable 把工作表适配成 Printable（自动分页）。
func SheetPrintable(sh *office.Sheet, title string) Printable {
	return &sheetPrintable{sh: sh, title: title}
}

type sheetPrintable struct {
	sh    *office.Sheet
	title string
}

const (
	sheetPgRowH    float32 = 22
	sheetPgMarginX float32 = 56
	sheetPgMarginY float32 = 56
	sheetPgFoot    float32 = 44
)

// Title 打印任务名（工作表名）。
func (p *sheetPrintable) Title() string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.title)
}

// rowsPerPage 每页可排多少行。
func (p *sheetPrintable) rowsPerPage() int {
	if p == nil || p.sh == nil {
		return 1
	}
	usable := PageH96 - sheetPgMarginY - sheetPgFoot
	n := int(usable / sheetPgRowH)
	if n < 1 {
		n = 1
	}
	return n
}

func (p *sheetPrintable) Pages() int {
	if p == nil || p.sh == nil || len(p.sh.Rows) == 0 {
		return 1
	}
	per := p.rowsPerPage()
	// 表头每页重复，所以按「数据行」分页
	data := len(p.sh.Rows)
	if p.sh.Header && data > 0 {
		data--
	}
	pages := (data + per - 1) / per
	if pages < 1 {
		pages = 1
	}
	return pages
}

func (p *sheetPrintable) DrawPage(c *PrintCanvas, page int) {
	if p == nil || p.sh == nil || c == nil {
		return
	}
	sh := p.sh
	cols := sheetColCount(sh)
	if cols > 14 { // 打印最多 14 列，超出部分提示在页脚
		cols = 14
	}
	if cols == 0 {
		return
	}
	usableW := PageW96 - sheetPgMarginX*2
	widths := make([]float32, cols)
	var total float32
	for cc := 0; cc < cols; cc++ {
		maxLen := 6
		for r := range sh.Rows {
			if n := len([]rune(sh.Cell(r, cc).Text)); n > maxLen {
				maxLen = n
			}
		}
		w := float32(maxLen) * 6.2
		if w > 160 {
			w = 160
		}
		widths[cc] = w
		total += w
	}
	if total > 0 {
		for cc := range widths {
			widths[cc] = widths[cc] / total * usableW
		}
	}

	// 标题
	y := sheetPgMarginY
	if p.title != "" {
		c.Text(sheetPgMarginX, y-22, 15, p.title, "#111111")
	}

	// 表头（每页重复）
	headY := y
	if sh.Header && len(sh.Rows) > 0 {
		c.Rect(sheetPgMarginX, headY, usableW, sheetPgRowH, 0, "", "#eef2fa")
		x := sheetPgMarginX
		for cc := 0; cc < cols; cc++ {
			c.Text(x+8, headY+6, 11, sh.Cell(0, cc).Text, "#334155")
			x += widths[cc]
		}
		c.Line(sheetPgMarginX, headY+sheetPgRowH, sheetPgMarginX+usableW, headY+sheetPgRowH, 0.8, "#c9d1de")
		headY += sheetPgRowH
	}

	// 数据
	per := p.rowsPerPage()
	start := 0
	if sh.Header {
		start = 1
	}
	start += page * per
	end := start + per
	if end > len(sh.Rows) {
		end = len(sh.Rows)
	}
	rowY := headY
	for r := start; r < end; r++ {
		if (r-start)%2 == 1 {
			c.Rect(sheetPgMarginX, rowY, usableW, sheetPgRowH, 0, "", "#f8fafc")
		}
		x := sheetPgMarginX
		for cc := 0; cc < cols; cc++ {
			cell := sh.Cell(r, cc)
			color := "#334155"
			if cell.Bold {
				color = "#0f172a"
			}
			c.Text(x+8, rowY+6, 11, cell.Text, color)
			x += widths[cc]
		}
		rowY += sheetPgRowH
	}

	// 页脚
	c.Line(sheetPgMarginX, PageH96-sheetPgFoot, sheetPgMarginX+usableW, PageH96-sheetPgFoot, 0.8, "#e2e8f0")
	c.Text(sheetPgMarginX, PageH96-sheetPgFoot+10, 10,
		fmt.Sprintf("%s · 第 %d / %d 页", sh.Name, page+1, p.Pages()), "#94a3b8")
}

// WordPrintable 把 Word 文档适配成 Printable（按行流式排版 + 自动分页）。
func WordPrintable(doc *office.WordDoc) Printable {
	return &wordPrintable{doc: doc}
}

type wordPrintable struct {
	doc    *office.WordDoc
	layout []officePrintLine // 懒排版：首次调用时算好所有行
}

// officePrintLine 排版后的一行。
type officePrintLine struct {
	kind    int // 0 普通，1 标题，2 列表，3 表格，4 引用
	size    float32
	weight  int32
	text    string
	color   string
	indent  float32
	rowBG   bool
	tableLn bool
}

const (
	printMarginX float32 = 64
	printMarginY float32 = 64
)

func (p *wordPrintable) build() {
	if p.layout != nil || p.doc == nil {
		return
	}
	usable := PageW96 - printMarginX*2
	add := func(pl officePrintLine) { p.layout = append(p.layout, pl) }
	for _, blk := range p.doc.Blocks {
		switch blk.Kind {
		case office.BlockHeading:
			size := headingSize(blk.Level)
			p.layout = append(p.layout, officePrintLine{kind: 1, size: 8, text: "", color: "#ffffff"})
			for _, line := range wrapPrint(blk.Text, size, 700, usable) {
				add(officePrintLine{kind: 1, size: size, weight: 700, text: line, color: "#111111"})
			}
		case office.BlockListItem:
			for _, line := range wrapPrint("• "+blk.Text, 11, 400, usable-float32(blk.Level)*12) {
				add(officePrintLine{kind: 2, size: 11, weight: 400, text: line,
					color: "#334155", indent: float32(blk.Level-1) * 18})
			}
		case office.BlockQuote:
			for _, line := range wrapPrint(blk.Text, 11, 400, usable-16) {
				add(officePrintLine{kind: 4, size: 11, weight: 400, text: line, color: "#64748b", indent: 16})
			}
		case office.BlockTable:
			for ri, row := range blk.Rows {
				line := strings.Join(row, "   |   ")
				for _, seg := range wrapPrint(line, 10.5, 400, usable) {
					add(officePrintLine{kind: 3, size: 10.5, weight: 400, text: seg,
						color: "#334155", rowBG: ri == 0})
				}
			}
			add(officePrintLine{kind: 0, size: 6, text: "", color: "#ffffff"})
		default:
			for _, line := range wrapPrint(blk.Text, 11.5, 400, usable) {
				add(officePrintLine{kind: 0, size: 11.5, weight: 400, text: line, color: "#334155"})
			}
			add(officePrintLine{kind: 0, size: 6, text: "", color: "#ffffff"})
		}
	}
}

// lineH 行高。
func (pl officePrintLine) lineH() float32 {
	return pl.size*1.65 + 2
}

// Title 打印任务名（文档标题）。
func (p *wordPrintable) Title() string {
	if p == nil || p.doc == nil {
		return ""
	}
	return strings.TrimSpace(p.doc.Title)
}

func (p *wordPrintable) Pages() int {
	p.build()
	total := float32(0)
	usable := PageH96 - printMarginY*2
	pages := 1
	for _, ln := range p.layout {
		h := ln.lineH()
		if total+h > usable {
			pages++
			total = 0
		}
		total += h
	}
	return pages
}

func (p *wordPrintable) DrawPage(c *PrintCanvas, page int) {
	p.build()
	if c == nil {
		return
	}
	usable := PageH96 - printMarginY*2
	y := printMarginY
	cur := 0
	for _, ln := range p.layout {
		h := ln.lineH()
		if y+h > printMarginY+usable {
			cur++
			y = printMarginY
		}
		if cur < page {
			y += h
			continue
		}
		if cur > page {
			break
		}
		if ln.rowBG {
			c.Rect(printMarginX, y, PageW96-printMarginX*2, h, 0, "", "#f1f5f9")
		}
		if ln.text != "" && ln.color != "#ffffff" {
			c.Text(printMarginX+ln.indent, y+2, ln.size, ln.text, ln.color)
		}
		y += h
	}
	c.Text(printMarginX, PageH96-40, 9,
		fmt.Sprintf("第 %d / %d 页", page+1, p.Pages()), "#94a3b8")
}

// wrapPrint 按宽度折行（用组件库的字宽测量，中英文混排都准）。
func wrapPrint(s string, size float32, weight int32, maxW float32) []string {
	s = strings.ReplaceAll(s, "\r", "")
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, para := range raw {
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		var line strings.Builder
		lineW := float32(0)
		for _, r := range para {
			w := TextWidth(string(r), size, weight)
			if lineW+w > maxW && line.Len() > 0 {
				out = append(out, line.String())
				line.Reset()
				lineW = 0
			}
			line.WriteRune(r)
			lineW += w
		}
		if line.Len() > 0 {
			out = append(out, line.String())
		}
	}
	return out
}

// OfficePrintable 任意办公文档 → Printable。
func OfficePrintable(f *office.File) Printable {
	if f == nil {
		return nil
	}
	switch f.Kind {
	case office.KindExcel:
		name := "工作表"
		if sh := f.Sheet.First(); sh != nil && strings.TrimSpace(sh.Name) != "" {
			name = sh.Name
		}
		return SheetPrintable(f.Sheet.First(), name)
	case office.KindWord:
		return WordPrintable(f.Word)
	case office.KindPDF:
		return PDFPrintable(f.PDF)
	}
	return nil
}

// ExportPDF 把任意 Printable 导出成 PDF 文件（自动嵌入中文字体子集）。
//
// 96 DPI 的打印页面（794×1123）按比例映射到 A4 的 pt 尺寸（595×842）。
func ExportPDF(path, title string, doc Printable) error {
	if doc == nil {
		return fmt.Errorf("ui: 没有可导出的内容")
	}
	pd := &office.PDFDoc{Title: title}
	pages := doc.Pages()
	if pages <= 0 {
		pages = 1
	}
	k := office.A4[0] / PageW96
	for i := 0; i < pages; i++ {
		canvas := &PrintCanvas{W: PageW96, H: PageH96}
		doc.DrawPage(canvas, i)
		pg := office.NewPDFPage(0, 0)
		for _, cmd := range canvas.cmds {
			switch cmd.kind {
			case 0:
				pg.TextColor(cmd.x*k, cmd.y*k, cmd.size*k, cmd.text, colorOrStr(cmd.color, "#111111"))
			case 1:
				pg.Rect(cmd.x*k, cmd.y*k, cmd.w*k, cmd.h*k, cmd.stroke*k, cmd.color, cmd.fill)
			case 2:
				pg.Line(cmd.sx*k, cmd.sy*k, cmd.ex*k, cmd.ey*k, cmd.stroke*k,
					colorOrStr(cmd.color, "#999999"))
			}
		}
		pd.Pages = append(pd.Pages, *pg)
	}
	return office.WritePDFWith(path, pd, office.PDFOptions{Title: title})
}
