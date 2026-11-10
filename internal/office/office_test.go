// guis — office 包单元测试：读写往返（xlsx / docx / pdf）

package office

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testSheet 构造一张带表头与数值的示例表。
func testSheet() *Spreadsheet {
	sh := Sheet{Name: "销售", Header: true}
	head := []string{"月份", "订单数", "金额", "备注"}
	for i, h := range head {
		sh.SetCell(0, i, Cell{Text: h, Bold: true})
	}
	rows := [][3]any{
		{"一月", 128, 1890000.0},
		{"二月", 96, 1420000.0},
		{"三月", 154, 2310000.0},
	}
	for r, row := range rows {
		sh.Set(r+1, 0, row[0].(string))
		sh.SetNum(r+1, 1, float64(row[1].(int)))
		sh.SetNum(r+1, 2, row[2].(float64))
		sh.Set(r+1, 3, "含运费")
	}
	return &Spreadsheet{Sheets: []Sheet{sh}}
}

// TestXLSXRoundTrip xlsx 写入后读回内容一致。
func TestXLSXRoundTrip(t *testing.T) {
	wb := testSheet()
	data, err := WriteXLSXBytes(wb)
	if err != nil {
		t.Fatalf("WriteXLSXBytes: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("xlsx 内容为空")
	}
	got, err := ReadXLSXBytes(data)
	if err != nil {
		t.Fatalf("ReadXLSXBytes: %v", err)
	}
	if len(got.Sheets) != 1 {
		t.Fatalf("工作表数量 = %d，期望 1", len(got.Sheets))
	}
	sh := got.First()
	if sh.Name != "销售" {
		t.Fatalf("表名 = %q", sh.Name)
	}
	if !sh.Header {
		t.Fatal("首行应被识别为表头（加粗）")
	}
	if len(sh.Rows) != 4 {
		t.Fatalf("行数 = %d，期望 4", len(sh.Rows))
	}
	if v := sh.Cell(0, 1).Text; v != "订单数" {
		t.Fatalf("表头 B1 = %q", v)
	}
	if c := sh.Cell(1, 1); !c.Num || c.Value != 128 {
		t.Fatalf("B2 = %+v，期望数值 128", c)
	}
	if v := sh.Cell(2, 2).Text; v != "1420000" {
		t.Fatalf("C3 = %q，期望 1420000", v)
	}
	if sum := sh.SumColumn(1); sum != 378 {
		t.Fatalf("订单数合计 = %v，期望 378", sum)
	}
}

// TestXLSXMultiSheet 多表往返 + 表名去重。
func TestXLSXMultiSheet(t *testing.T) {
	wb := &Spreadsheet{Sheets: []Sheet{
		{Name: "A", Rows: [][]Cell{{{Text: "x"}}}},
		{Name: "A", Rows: [][]Cell{{{Text: "y"}}}},
		{Name: "", Rows: [][]Cell{{{Text: "z"}}}},
	}}
	data, err := WriteXLSXBytes(wb)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadXLSXBytes(data)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got.Sheets) != 3 {
		t.Fatalf("工作表数 = %d", len(got.Sheets))
	}
	if got.Sheets[0].Name != "A" || got.Sheets[1].Name != "A-2" {
		t.Fatalf("表名去重失败：%q / %q", got.Sheets[0].Name, got.Sheets[1].Name)
	}
	if got.Sheets[2].Name != "Sheet3" {
		t.Fatalf("空表名兜底失败：%q", got.Sheets[2].Name)
	}
}

// TestDOCXRoundTrip docx 写入后读回块结构一致。
func TestDOCXRoundTrip(t *testing.T) {
	doc := &WordDoc{Title: "季度报告", Blocks: []Block{
		Heading(1, "季度经营报告"),
		Paragraph("本季度整体营收同比增长 21%，其中华东区贡献最大。"),
		Bullet("新签客户 42 家"),
		Bullet("复购率 68%"),
		Block{Kind: BlockQuote, Text: "数据来源：内部 BI 系统"},
		Table([][]string{
			{"区域", "营收", "同比"},
			{"华东", "820 万", "+26%"},
			{"华南", "540 万", "+14%"},
		}),
	}}
	data, err := WriteDOCXBytes(doc)
	if err != nil {
		t.Fatalf("WriteDOCXBytes: %v", err)
	}
	got, err := ReadDOCXBytes(data)
	if err != nil {
		t.Fatalf("ReadDOCXBytes: %v", err)
	}
	if got.Title != "季度报告" {
		t.Fatalf("标题 = %q", got.Title)
	}
	if len(got.Blocks) != 6 {
		t.Fatalf("块数 = %d，期望 6：%+v", len(got.Blocks), got.Blocks)
	}
	if b := got.Blocks[0]; b.Kind != BlockHeading || b.Level != 1 || b.Text != "季度经营报告" {
		t.Fatalf("标题块 = %+v", b)
	}
	if b := got.Blocks[1]; b.Kind != BlockParagraph || !strings.Contains(b.Text, "同比增长") {
		t.Fatalf("段落块 = %+v", b)
	}
	if b := got.Blocks[2]; b.Kind != BlockListItem || !strings.Contains(b.Text, "42 家") {
		t.Fatalf("列表块 = %+v", b)
	}
	tbl := got.Blocks[5]
	if tbl.Kind != BlockTable || len(tbl.Rows) != 3 || tbl.Rows[1][1] != "820 万" {
		t.Fatalf("表格块 = %+v", tbl)
	}
}

// TestPDFDeviceFont PDF 用内置 Helvetica（不嵌字体）时结构正确。
func TestPDFDeviceFont(t *testing.T) {
	doc := &PDFDoc{Title: "Test"}
	pg := NewPDFPage(0, 0)
	pg.Text(56, 64, 20, "Hello PDF")
	pg.Line(56, 84, 539, 84, 1, "#e5e7eb")
	pg.Rect(56, 96, 120, 24, 1, "#cccccc", "#eef2fa")
	doc.Pages = append(doc.Pages, *pg)

	data, err := WritePDFBytes(doc, PDFOptions{NoEmbed: true})
	if err != nil {
		t.Fatalf("WritePDFBytes: %v", err)
	}
	if !strings.HasPrefix(string(data), "%PDF-1.7") {
		t.Fatalf("PDF 头错误：%q", string(data[:16]))
	}
	got, err := ReadPDFBytes(data)
	if err != nil {
		t.Fatalf("ReadPDFBytes: %v", err)
	}
	if len(got.Pages) != 1 {
		t.Fatalf("页数 = %d", len(got.Pages))
	}
	if got.Title != "Test" {
		t.Fatalf("标题 = %q", got.Title)
	}
	if txt := pdfTextOf(got.Pages[0]); !strings.Contains(txt, "Hello PDF") {
		t.Fatalf("文本抽取失败：%q", txt)
	}
}

// TestPDFChinese 中文 PDF：嵌入子集字体并保证文本可还原。
func TestPDFChinese(t *testing.T) {
	if p, _ := pdfFontFile("", 0); p == "" {
		t.Skip("本机没有可嵌入的 TrueType 字体，跳过中文 PDF 测试")
	}
	doc := &PDFDoc{}
	pg := NewPDFPage(0, 0)
	pg.Text(56, 80, 22, "季度经营报表")
	pg.Text(56, 116, 12, "华东区营收同比增长 26%")
	doc.Pages = append(doc.Pages, *pg)

	data, err := WritePDFBytes(doc, PDFOptions{})
	if err != nil {
		t.Fatalf("WritePDFBytes: %v", err)
	}
	// 嵌入的字体应该被裁到很小（远小于原始字体）
	if len(data) > 4<<20 {
		t.Fatalf("PDF 体积异常（%d 字节），子集化可能失败", len(data))
	}
	got, err := ReadPDFBytes(data)
	if err != nil {
		t.Fatalf("ReadPDFBytes: %v", err)
	}
	txt := pdfTextOf(got.Pages[0])
	if !strings.Contains(txt, "季度经营报表") {
		t.Fatalf("中文文本未还原：%q", txt)
	}
	if !strings.Contains(txt, "华东区营收同比增长") {
		t.Fatalf("中文段落未还原：%q", txt)
	}
}

// TestDetect 类型探测。
func TestDetect(t *testing.T) {
	dir := t.TempDir()
	xlsxPath := filepath.Join(dir, "a.xlsx")
	if err := WriteXLSX(xlsxPath, testSheet()); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	if k := Detect(xlsxPath); k != KindExcel {
		t.Fatalf("Detect(xlsx) = %v", k)
	}
	docxPath := filepath.Join(dir, "a.docx")
	if err := WriteDOCX(docxPath, &WordDoc{Blocks: []Block{Paragraph("hi")}}); err != nil {
		t.Fatalf("write docx: %v", err)
	}
	if k := Detect(docxPath); k != KindWord {
		t.Fatalf("Detect(docx) = %v", k)
	}
	pdfPath := filepath.Join(dir, "a.pdf")
	pg := NewPDFPage(0, 0)
	pg.Text(20, 20, 12, "hi")
	if err := WritePDFWith(pdfPath, &PDFDoc{Pages: []PDFPage{*pg}}, PDFOptions{NoEmbed: true}); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	if k := Detect(pdfPath); k != KindPDF {
		t.Fatalf("Detect(pdf) = %v", k)
	}
	// 无扩展名时按文件头判断
	anon := filepath.Join(dir, "blob")
	raw, _ := os.ReadFile(pdfPath)
	if err := os.WriteFile(anon, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if k := Detect(anon); k != KindPDF {
		t.Fatalf("Detect(blob) = %v，期望 PDF", k)
	}
	// Open 统一入口
	f, err := Open(xlsxPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if f.Kind != KindExcel || f.Sheet == nil {
		t.Fatalf("Open 结果异常：%+v", f)
	}
	if s := f.Summary(); !strings.Contains(s, "1 张工作表") {
		t.Fatalf("Summary = %q", s)
	}
}

// TestNumberFormat 数值格式化。
func TestNumberFormat(t *testing.T) {
	cases := map[float64]string{
		0:        "0",
		128:      "128",
		-7:       "-7",
		1.5:      "1.5",
		0.25:     "0.25",
		1234.567: "1234.567",
	}
	for v, want := range cases {
		if got := FormatNumber(v); got != want {
			t.Fatalf("FormatNumber(%v) = %q，期望 %q", v, got, want)
		}
	}
}

// pdfTextOf 拼一页里的所有文本（测试辅助）。
func pdfTextOf(p PDFPage) string {
	var b strings.Builder
	for _, it := range p.Items {
		if it.Kind == PDFText {
			b.WriteString(it.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
