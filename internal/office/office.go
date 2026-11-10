// guis — 办公文档能力（Excel / Word / PDF）
//
// 设计约定与 ui 包的「零依赖、零 cgo」一致：全部用标准库实现
// （archive/zip + encoding/xml + compress/flate），不引入任何第三方模块。
//
// 三层结构：
//
//	internal/office  文档模型 + 读写（本包，与 UI 无关，可在服务端 / 命令行复用）
//	ui/office.go     把模型渲染成组件树（表格 / 文档流 / PDF 页面）
//	cmd/gallery      示例页（打开 / 导出 / 打印预览）
//
// 统一入口：
//
//	f, err := office.Open("报表.xlsx")     // 按扩展名 + 文件头识别类型
//	switch f.Kind {
//	case office.KindExcel:
//	    for _, c := range f.Sheet.Sheets[0].Rows { … c[0].Text … }
//	case office.KindWord:
//	    for _, b := range f.Word.Blocks { … }
//	case office.KindPDF:
//	    for _, p := range f.PDF.Pages { … p.Items … }
//	}
//
// 生成：
//
//	office.WriteXLSX("报表.xlsx", &office.Spreadsheet{Sheets: []office.Sheet{sheet}})
//	office.WriteDOCX("说明.docx", &office.WordDoc{Blocks: blocks})
//	office.WritePDF("说明.pdf", &office.PDFDoc{Pages: pages})
package office

import (
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
)

// Kind 文档类型。
type Kind int

const (
	// KindUnknown 未知 / 不支持。
	KindUnknown Kind = iota
	// KindExcel Excel 工作簿（.xlsx）。
	KindExcel
	// KindWord Word 文档（.docx）。
	KindWord
	// KindPDF PDF 文档。
	KindPDF
)

// String 类型名（界面展示用）。
func (k Kind) String() string {
	switch k {
	case KindExcel:
		return "Excel"
	case KindWord:
		return "Word"
	case KindPDF:
		return "PDF"
	}
	return "未知"
}

// Ext 推荐扩展名（导出对话框用）。
func (k Kind) Ext() string {
	switch k {
	case KindExcel:
		return ".xlsx"
	case KindWord:
		return ".docx"
	case KindPDF:
		return ".pdf"
	}
	return ""
}

// ErrUnsupported 不支持的文档类型。
var ErrUnsupported = errors.New("office: 不支持的文档类型（仅支持 .xlsx / .docx / .pdf）")

// File 一份已打开的文档（Kind 决定哪个字段有效，其余为 nil）。
type File struct {
	Path  string
	Kind  Kind
	Sheet *Spreadsheet
	Word  *WordDoc
	PDF   *PDFDoc
}

// ---- Excel 模型 ----

// Cell 一个单元格。Text 是展示文本，Value/Num 保留原始数值（排序 / 合计用）。
type Cell struct {
	Text  string
	Value float64
	Num   bool
	Bold  bool
	// Fill 背景色（"#rrggbb"，空 = 无）。
	Fill string
}

// Sheet 一张工作表：Rows[r][c]，行列从 0 开始。
type Sheet struct {
	Name string
	Rows [][]Cell
	// Cols 最大列数（供渲染铺表头 A/B/C…）。
	Cols int
	// Header 首行是否为表头（读入时按「首行加粗」推断）。
	Header bool
}

// Cell 安全取单元格（越界返回零值）。
func (s *Sheet) Cell(r, c int) Cell {
	if r < 0 || r >= len(s.Rows) || c < 0 || c >= len(s.Rows[r]) {
		return Cell{}
	}
	return s.Rows[r][c]
}

// SetCell 写入单元格（自动扩展行 / 列）。
func (s *Sheet) SetCell(r, c int, v Cell) {
	for len(s.Rows) <= r {
		s.Rows = append(s.Rows, nil)
	}
	for len(s.Rows[r]) <= c {
		s.Rows[r] = append(s.Rows[r], Cell{})
	}
	s.Rows[r][c] = v
	if c+1 > s.Cols {
		s.Cols = c + 1
	}
}

// Set 写入文本单元格。
func (s *Sheet) Set(r, c int, text string) { s.SetCell(r, c, Cell{Text: text}) }

// SetNum 写入数值单元格。
func (s *Sheet) SetNum(r, c int, v float64) {
	s.SetCell(r, c, Cell{Text: FormatNumber(v), Value: v, Num: true})
}

// Spreadsheet 工作簿。
type Spreadsheet struct {
	Sheets []Sheet
}

// First 第一张工作表（没有则返回 nil）。
func (w *Spreadsheet) First() *Sheet {
	if w == nil || len(w.Sheets) == 0 {
		return nil
	}
	return &w.Sheets[0]
}

// SheetAt 按索引取工作表（越界返回 nil）。
func (w *Spreadsheet) SheetAt(i int) *Sheet {
	if w == nil || i < 0 || i >= len(w.Sheets) {
		return nil
	}
	return &w.Sheets[i]
}

// SheetByName 按名字取工作表（没有则返回第一张）。
func (w *Spreadsheet) SheetByName(name string) *Sheet {
	if w == nil {
		return nil
	}
	for i := range w.Sheets {
		if w.Sheets[i].Name == name {
			return &w.Sheets[i]
		}
	}
	return w.First()
}

// ---- Word 模型 ----

// BlockKind Word 内容块类型。
type BlockKind int

const (
	// BlockParagraph 普通段落。
	BlockParagraph BlockKind = iota
	// BlockHeading 标题（Level 为 1-6）。
	BlockHeading
	// BlockListItem 列表项。
	BlockListItem
	// BlockTable 表格（Rows 有效，HeaderRows 为表头行数）。
	BlockTable
	// BlockQuote 引用段落。
	BlockQuote
)

// Block 一个内容块。
type Block struct {
	Kind BlockKind
	// Level 标题级别（1-6）或列表缩进级别。
	Level int
	// Text 段落 / 标题 / 列表项文本。
	Text string
	// Bold 整段加粗。
	Bold bool
	// Rows 表格数据（BlockTable）。
	Rows [][]string
	// HeaderRows 表格表头行数（BlockTable）。
	HeaderRows int
	// Widths 表格各列相对宽度（空 = 等分）。
	Widths []float32
}

// Paragraph 构造段落块。
func Paragraph(text string) Block { return Block{Kind: BlockParagraph, Text: text} }

// Heading 构造标题块（level 1-6）。
func Heading(level int, text string) Block {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}
	return Block{Kind: BlockHeading, Level: level, Text: text}
}

// Bullet 构造列表项。
func Bullet(text string) Block { return Block{Kind: BlockListItem, Level: 1, Text: text} }

// Table 构造表格块（首行为表头）。
func Table(rows [][]string) Block {
	return Block{Kind: BlockTable, Rows: rows, HeaderRows: 1}
}

// WordDoc Word 文档。
type WordDoc struct {
	// Title 文档标题（取自 docProps/core.xml 的 dc:title，可空）。
	Title  string
	Blocks []Block
}

// PlainText 纯文本化（导出 / 预览摘要用）。
func (d *WordDoc) PlainText() string {
	if d == nil {
		return ""
	}
	var b strings.Builder
	for _, blk := range d.Blocks {
		switch blk.Kind {
		case BlockTable:
			for _, row := range blk.Rows {
				b.WriteString(strings.Join(row, "\t"))
				b.WriteByte('\n')
			}
		default:
			b.WriteString(blk.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// ---- PDF 模型 ----

// PDFItemKind PDF 绘制指令类型。
type PDFItemKind int

const (
	// PDFText 文本（X/Y 为左上角，Y 向下）。
	PDFText PDFItemKind = iota
	// PDFLine 直线。
	PDFLine
	// PDFRect 矩形（Fill 非空为填充，Stroke 为描边宽度）。
	PDFRect
	// PDFImage 图片（预留：读入时记录位置，渲染暂以占位框显示）。
	PDFImage
)

// PDFItem 一条绘制指令。坐标系与 ui.PrintCanvas 一致：原点左上、Y 向下、单位 pt。
type PDFItem struct {
	Kind PDFItemKind
	Text string
	X, Y float32
	// Size 字号（PDFText）。
	Size float32
	// W/H 宽高（PDFRect / PDFImage）。
	W, H float32
	// X2/Y2 终点（PDFLine）。
	X2, Y2 float32
	// Color 文字 / 线条颜色 "#rrggbb"（空 = 黑色）。
	Color string
	// Fill 填充色（PDFRect，空 = 不填充）。
	Fill string
	// Stroke 描边宽度（PDFRect / PDFLine），<=0 不描边。
	Stroke float32
	// Bold 文本是否加粗（渲染用）。
	Bold bool
}

// PDFPage 一页。
type PDFPage struct {
	// W/H 页面尺寸（pt）。
	W, H  float32
	Items []PDFItem
}

// PDFDoc PDF 文档。
type PDFDoc struct {
	Pages []PDFPage
	// Title 文档标题（取自 /Info /Title，可空）。
	Title string
}

// PDFPageSize 常用页面尺寸（pt）：A4 / Letter。
var (
	A4     = [2]float32{595.28, 841.89}
	Letter = [2]float32{612, 792}
)

// NewPDFPage 新建一页（w/h <= 0 时用 A4）。
func NewPDFPage(w, h float32) *PDFPage {
	if w <= 0 || h <= 0 {
		w, h = A4[0], A4[1]
	}
	return &PDFPage{W: w, H: h}
}

// Text 追加文本（x/y 为左上角，Y 向下）。
func (p *PDFPage) Text(x, y, size float32, s string) *PDFPage {
	if p == nil || s == "" {
		return p
	}
	if size <= 0 {
		size = 12
	}
	p.Items = append(p.Items, PDFItem{Kind: PDFText, Text: s, X: x, Y: y, Size: size})
	return p
}

// TextColor 追加带颜色的文本。
func (p *PDFPage) TextColor(x, y, size float32, s, color string) *PDFPage {
	p.Text(x, y, size, s)
	if n := len(p.Items); n > 0 {
		p.Items[n-1].Color = color
	}
	return p
}

// TextBold 追加加粗文本。
func (p *PDFPage) TextBold(x, y, size float32, s string) *PDFPage {
	p.Text(x, y, size, s)
	if n := len(p.Items); n > 0 {
		p.Items[n-1].Bold = true
	}
	return p
}

// Line 追加直线。
func (p *PDFPage) Line(x1, y1, x2, y2, width float32, color string) *PDFPage {
	if p == nil {
		return p
	}
	if width <= 0 {
		width = 1
	}
	p.Items = append(p.Items, PDFItem{Kind: PDFLine, X: x1, Y: y1, X2: x2, Y2: y2,
		Stroke: width, Color: color})
	return p
}

// Rect 追加矩形（fill 为空只描边，stroke<=0 只填充）。
func (p *PDFPage) Rect(x, y, w, h, stroke float32, strokeColor, fill string) *PDFPage {
	if p == nil {
		return p
	}
	p.Items = append(p.Items, PDFItem{Kind: PDFRect, X: x, Y: y, W: w, H: h,
		Stroke: stroke, Color: strokeColor, Fill: fill})
	return p
}

// ---- 类型探测 ----

// Detect 按扩展名 + 文件头判断类型（扩展名优先，文件头兜底）。
func Detect(path string) Kind {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".xlsx"), strings.HasSuffix(lower, ".xlsm"):
		return KindExcel
	case strings.HasSuffix(lower, ".docx"):
		return KindWord
	case strings.HasSuffix(lower, ".pdf"):
		return KindPDF
	}
	// 扩展名不认识时看文件头：%PDF- / PK（zip 包再按内部目录名细分）。
	f, err := os.Open(path)
	if err != nil {
		return KindUnknown
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	head = head[:n]
	if strings.HasPrefix(string(head), "%PDF-") {
		return KindPDF
	}
	return kindFromZip(path)
}

// Sniff 按内容判断类型（字节流，不落盘）。
func Sniff(data []byte) Kind {
	if len(data) == 0 {
		return KindUnknown
	}
	if strings.HasPrefix(string(data), "%PDF-") {
		return KindPDF
	}
	// zip 中央目录里的文件名：xl/workbook.xml → Excel，word/document.xml → Word
	s := string(data)
	switch {
	case strings.Contains(s, "xl/workbook.xml"):
		return KindExcel
	case strings.Contains(s, "word/document.xml"):
		return KindWord
	}
	return KindUnknown
}

// Open 打开并解析文档。类型不支持时返回 ErrUnsupported。
func Open(path string) (*File, error) {
	return OpenBytes(path, nil)
}

// OpenBytes 用给定字节解析（data 为 nil 时读文件）。path 只用于定位 / 判断类型。
func OpenBytes(path string, data []byte) (*File, error) {
	kind := Detect(path)
	if kind == KindUnknown && len(data) > 0 {
		kind = Sniff(data)
	}
	if kind == KindUnknown {
		return nil, ErrUnsupported
	}
	if data == nil {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		data = b
	}
	f := &File{Path: path, Kind: kind}
	switch kind {
	case KindExcel:
		wb, err := ReadXLSXBytes(data)
		if err != nil {
			return nil, err
		}
		f.Sheet = wb
	case KindWord:
		doc, err := ReadDOCXBytes(data)
		if err != nil {
			return nil, err
		}
		f.Word = doc
	case KindPDF:
		doc, err := ReadPDFBytes(data)
		if err != nil {
			return nil, err
		}
		f.PDF = doc
	}
	return f, nil
}

// OpenBytesKind 已知类型时直接按类型解析（避免重复探测）。
func OpenBytesKind(path string, kind Kind, data []byte) (*File, error) {
	if kind == KindUnknown {
		return OpenBytes(path, data)
	}
	if data == nil {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		data = b
	}
	f := &File{Path: path, Kind: kind}
	switch kind {
	case KindExcel:
		wb, err := ReadXLSXBytes(data)
		if err != nil {
			return nil, err
		}
		f.Sheet = wb
	case KindWord:
		doc, err := ReadDOCXBytes(data)
		if err != nil {
			return nil, err
		}
		f.Word = doc
	case KindPDF:
		doc, err := ReadPDFBytes(data)
		if err != nil {
			return nil, err
		}
		f.PDF = doc
	default:
		return nil, ErrUnsupported
	}
	return f, nil
}

// Summary 一句话摘要（列表 / 状态栏展示用）。
func (f *File) Summary() string {
	if f == nil {
		return ""
	}
	switch f.Kind {
	case KindExcel:
		if s := f.Sheet; s != nil {
			rows := 0
			for _, sh := range s.Sheets {
				rows += len(sh.Rows)
			}
			return plural(len(s.Sheets), "张工作表") + " · " + plural(rows, "行")
		}
	case KindWord:
		if d := f.Word; d != nil {
			return plural(len(d.Blocks), "个内容块")
		}
	case KindPDF:
		if d := f.PDF; d != nil {
			return plural(len(d.Pages), "页")
		}
	}
	return ""
}

// plural 生成「N 单位」文本。
func plural(n int, unit string) string {
	return itoa(n) + " " + unit
}

// itoa 无依赖整数转字符串（避免在多处 import strconv）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// kindFromZip 打开 zip 看内部结构判断类型。
func kindFromZip(path string) Kind {
	data, err := os.ReadFile(path)
	if err != nil {
		return KindUnknown
	}
	return Sniff(data)
}

// FormatNumber 数值 → 展示文本：整数不带小数点，小数最多保留 6 位且去掉尾随 0。
func FormatNumber(v float64) string {
	if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	s := strconv.FormatFloat(v, 'f', 6, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}
