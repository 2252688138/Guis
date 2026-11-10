// guis — Word（.docx）读写
//
// 读：word/document.xml 按出现顺序取 段落 / 表格（标题靠 pStyle 判断），
//     支持 run 级加粗、w:tab、w:br、超链接文字；
// 写：最小 OOXML 包（Content-Types / rels / document / styles / docProps）。
//
// 刻意不做的：图片、脚注、域代码、修订、复杂编号（列表按「• 文本」渲染）、
// 分栏 / 浮动版式。文档预览只需要「标题 / 段落 / 列表 / 表格」四种块。

package office

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ---- 读 ----

// ReadDOCX 读取 .docx 文件。
func ReadDOCX(path string) (*WordDoc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadDOCXBytes(b)
}

type docxRunXML struct {
	B   *struct{}  `xml:"rPr>b"`
	I   *struct{}  `xml:"rPr>i"`
	T   []string   `xml:"t"`
	Tab []struct{} `xml:"tab"`
	Br  []struct{} `xml:"br"`
}

type docxParaXML struct {
	PPr struct {
		Style struct {
			Val string `xml:"val,attr"`
		} `xml:"pStyle"`
		NumPr *struct{} `xml:"numPr"`
		Ind   struct {
			Left string `xml:"left,attr"`
		} `xml:"ind"`
	} `xml:"pPr"`
	Runs  []docxRunXML `xml:"r"`
	Links []struct {
		Runs []docxRunXML `xml:"r"`
	} `xml:"hyperlink"`
}

type docxCellXML struct {
	Paras []docxParaXML `xml:"p"`
}

type docxRowXML struct {
	Cells []docxCellXML `xml:"tc"`
}

type docxTblXML struct {
	Rows []docxRowXML `xml:"tr"`
}

// ReadDOCXBytes 从内存解析 .docx。
func ReadDOCXBytes(data []byte) (*WordDoc, error) {
	files, err := readZip(data)
	if err != nil {
		return nil, err
	}
	raw, ok := files["word/document.xml"]
	if !ok {
		return nil, errors.New("office: 不是有效的 docx（缺少 word/document.xml）")
	}
	doc := &WordDoc{}
	if core, ok := files["docProps/core.xml"]; ok {
		doc.Title = docxCoreTitle(core)
	}

	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	inBody := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			if ee, isEnd := tok.(xml.EndElement); isEnd && ee.Name.Local == "body" {
				inBody = false
			}
			continue
		}
		switch se.Name.Local {
		case "body":
			inBody = true
		case "p":
			if !inBody {
				continue
			}
			var p docxParaXML
			if err := dec.DecodeElement(&p, &se); err != nil {
				return nil, fmt.Errorf("office: 段落解析失败：%w", err)
			}
			if blk, ok := docxBlockFromPara(p); ok {
				doc.Blocks = append(doc.Blocks, blk)
			}
			if len(doc.Blocks) > 5000 {
				return doc, nil
			}
		case "tbl":
			if !inBody {
				continue
			}
			var t docxTblXML
			if err := dec.DecodeElement(&t, &se); err != nil {
				return nil, fmt.Errorf("office: 表格解析失败：%w", err)
			}
			if blk, ok := docxBlockFromTable(t); ok {
				doc.Blocks = append(doc.Blocks, blk)
			}
		}
	}
	if len(doc.Blocks) == 0 && doc.Title == "" {
		return nil, errors.New("office: docx 内容为空")
	}
	return doc, nil
}

// docxCoreTitle 读 docProps/core.xml 的 dc:title。
func docxCoreTitle(data []byte) string {
	var core struct {
		Title string `xml:"title"`
	}
	if err := xmlUnmarshal(data, &core); err != nil {
		return ""
	}
	return strings.TrimSpace(core.Title)
}

// docxBlockFromPara 段落 → 块（标题 / 列表 / 引用 / 普通段落）。空段落丢弃。
func docxBlockFromPara(p docxParaXML) (Block, bool) {
	var (
		b    strings.Builder
		bold = false
		any  = false
	)
	addRuns := func(runs []docxRunXML) {
		for _, r := range runs {
			for _, t := range r.T {
				b.WriteString(t)
				any = true
			}
			// w:tab → 制表符；w:br → 换行
			for range r.Tab {
				b.WriteByte('\t')
			}
			for range r.Br {
				b.WriteByte(' ')
			}
			if r.B != nil && strings.TrimSpace(strings.Join(r.T, "")) != "" {
				bold = true
			}
		}
	}
	addRuns(p.Runs)
	for _, l := range p.Links {
		addRuns(l.Runs)
	}
	text := strings.TrimRight(b.String(), " \t")
	if !any || strings.TrimSpace(text) == "" {
		return Block{}, false
	}

	style := strings.ToLower(strings.TrimSpace(p.PPr.Style.Val))
	switch {
	case strings.HasPrefix(style, "heading"):
		lv := atoiSafe(strings.TrimPrefix(style, "heading"))
		if lv < 1 {
			lv = 1
		}
		return Heading(lv, strings.TrimSpace(text)), true
	case style == "title", style == "subtitle":
		return Heading(1, strings.TrimSpace(text)), true
	case style == "quote", style == "intensequote":
		return Block{Kind: BlockQuote, Text: strings.TrimSpace(text)}, true
	}
	if p.PPr.NumPr != nil {
		level := 1
		if lv := atoiSafe(p.PPr.Ind.Left); lv > 0 {
			level = lv/360 + 1
			if level > 4 {
				level = 4
			}
		}
		return Block{Kind: BlockListItem, Level: level, Text: strings.TrimSpace(text)}, true
	}
	if strings.HasPrefix(style, "listparagraph") {
		return Block{Kind: BlockListItem, Level: 1, Text: strings.TrimSpace(text)}, true
	}
	return Block{Kind: BlockParagraph, Text: text, Bold: bold && len([]rune(text)) < 40}, true
}

// docxBlockFromTable 表格 → 块（首个非空行作表头）。
func docxBlockFromTable(t docxTblXML) (Block, bool) {
	var rows [][]string
	cols := 0
	for _, r := range t.Rows {
		if len(r.Cells) == 0 {
			continue
		}
		row := make([]string, 0, len(r.Cells))
		empty := true
		for _, c := range r.Cells {
			var parts []string
			for _, p := range c.Paras {
				var b strings.Builder
				for _, run := range p.Runs {
					for _, s := range run.T {
						b.WriteString(s)
					}
				}
				if s := strings.TrimSpace(b.String()); s != "" {
					parts = append(parts, s)
				}
			}
			txt := strings.Join(parts, "\n")
			if txt != "" {
				empty = false
			}
			row = append(row, txt)
		}
		if empty {
			continue
		}
		if len(row) > cols {
			cols = len(row)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return Block{}, false
	}
	// 补齐列数，避免下游按列下标取值越界
	for i, row := range rows {
		for len(row) < cols {
			row = append(row, "")
		}
		rows[i] = row
	}
	return Block{Kind: BlockTable, Rows: rows, HeaderRows: 1}, true
}

// ---- 写 ----

// WriteDOCX 把文档写成 .docx 文件。
func WriteDOCX(path string, doc *WordDoc) error {
	b, err := WriteDOCXBytes(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// WriteDOCXBytes 生成 .docx 字节。
func WriteDOCXBytes(doc *WordDoc) ([]byte, error) {
	if doc == nil {
		doc = &WordDoc{}
	}
	z := newZipBuilder()

	z.add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`+
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`+
		`<Default Extension="xml" ContentType="application/xml"/>`+
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>`+
		`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>`+
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`+
		`</Types>`)

	z.add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>`+
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>`+
		`</Relationships>`)

	z.add("word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`+
		`</Relationships>`)

	title := strings.TrimSpace(doc.Title)
	z.add("docProps/core.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" `+
		`xmlns:dc="http://purl.org/dc/elements/1.1/">`+
		`<dc:title>`+esc(title)+`</dc:title><dc:creator>GUIS office</dc:creator>`+
		`</cp:coreProperties>`)

	z.add("word/styles.xml", docxStylesXML)

	var body strings.Builder
	for _, blk := range doc.Blocks {
		body.WriteString(docxBlockXML(blk))
	}
	z.add("word/document.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`+
		`<w:body>`+body.String()+
		`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/>`+
		`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>`+
		`</w:body></w:document>`)
	return z.bytes()
}

// docxStylesXML 样式表（Normal / Title / Heading1-6 / Quote / ListParagraph / TableGrid）。
const docxStylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
	`<w:docDefaults><w:rPrDefault><w:rPr>` +
	`<w:rFonts w:ascii="Microsoft YaHei" w:eastAsia="Microsoft YaHei" w:hAnsi="Microsoft YaHei"/>` +
	`<w:sz w:val="22"/><w:szCs w:val="22"/>` +
	`</w:rPr></w:rPrDefault></w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/>` +
	`<w:pPr><w:spacing w:after="120" w:line="300" w:lineRule="auto"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/>` +
	`<w:pPr><w:spacing w:before="240" w:after="200"/></w:pPr>` +
	`<w:rPr><w:b/><w:sz w:val="48"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/>` +
	`<w:pPr><w:keepNext/><w:spacing w:before="280" w:after="140"/><w:outlineLvl w:val="0"/></w:pPr>` +
	`<w:rPr><w:b/><w:sz w:val="36"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/>` +
	`<w:pPr><w:keepNext/><w:spacing w:before="240" w:after="120"/><w:outlineLvl w:val="1"/></w:pPr>` +
	`<w:rPr><w:b/><w:sz w:val="30"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/>` +
	`<w:pPr><w:keepNext/><w:spacing w:before="200" w:after="100"/><w:outlineLvl w:val="2"/></w:pPr>` +
	`<w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading4"><w:name w:val="heading 4"/><w:basedOn w:val="Normal"/>` +
	`<w:rPr><w:b/><w:sz w:val="24"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading5"><w:name w:val="heading 5"/><w:basedOn w:val="Normal"/>` +
	`<w:rPr><w:b/><w:sz w:val="22"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading6"><w:name w:val="heading 6"/><w:basedOn w:val="Normal"/>` +
	`<w:rPr><w:b/><w:sz w:val="21"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/>` +
	`<w:pPr><w:ind w:left="480"/></w:pPr><w:rPr><w:i/><w:color w:val="555555"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/>` +
	`<w:pPr><w:ind w:left="600"/><w:spacing w:after="60"/></w:pPr></w:style>` +
	`<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/>` +
	`<w:tblPr><w:tblBorders>` +
	`<w:top w:val="single" w:sz="4" w:color="D0D7E2"/><w:left w:val="single" w:sz="4" w:color="D0D7E2"/>` +
	`<w:bottom w:val="single" w:sz="4" w:color="D0D7E2"/><w:right w:val="single" w:sz="4" w:color="D0D7E2"/>` +
	`<w:insideH w:val="single" w:sz="4" w:color="D0D7E2"/><w:insideV w:val="single" w:sz="4" w:color="D0D7E2"/>` +
	`</w:tblBorders></w:tblPr></w:style>` +
	`</w:styles>`

// docxPara 生成一个段落（style 为空 = 普通段落，bold 控制整段加粗）。
func docxPara(text, style string, bold bool) string {
	// 段落内换行拆成多个 w:br
	lines := strings.Split(text, "\n")
	var run strings.Builder
	run.WriteString(`<w:r>`)
	if bold {
		run.WriteString(`<w:rPr><w:b/></w:rPr>`)
	}
	for i, ln := range lines {
		if i > 0 {
			run.WriteString(`<w:br/>`)
		}
		fmt.Fprintf(&run, `<w:t xml:space="preserve">%s</w:t>`, esc(ln))
	}
	run.WriteString(`</w:r>`)
	ppr := ""
	if style != "" {
		ppr = `<w:pPr><w:pStyle w:val="` + style + `"/></w:pPr>`
	}
	return `<w:p>` + ppr + run.String() + `</w:p>`
}

// docxBlockXML 单块 → WordprocessingML。
func docxBlockXML(blk Block) string {
	switch blk.Kind {
	case BlockHeading:
		lv := blk.Level
		if lv < 1 || lv > 6 {
			lv = 1
		}
		return docxPara(blk.Text, "Heading"+itoa(lv), false)
	case BlockListItem:
		return docxPara("• "+blk.Text, "ListParagraph", false)
	case BlockQuote:
		return docxPara(blk.Text, "Quote", false)
	case BlockTable:
		return docxTableXML(blk)
	default:
		return docxPara(blk.Text, "", blk.Bold)
	}
}

// docxTableXML 表格 → w:tbl（总宽 9026 twips ≈ A4 去边距）。
func docxTableXML(blk Block) string {
	rows := blk.Rows
	if len(rows) == 0 {
		return ""
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	if cols == 0 {
		return ""
	}
	const total = 9026
	colW := total / cols
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="` + itoa(total) + `" w:type="dxa"/>` +
		`<w:tblLayout w:type="fixed"/></w:tblPr><w:tblGrid>`)
	for c := 0; c < cols; c++ {
		b.WriteString(`<w:gridCol w:w="` + itoa(colW) + `"/>`)
	}
	b.WriteString(`</w:tblGrid>`)
	for ri, row := range rows {
		b.WriteString(`<w:tr>`)
		for c := 0; c < cols; c++ {
			txt := ""
			if c < len(row) {
				txt = row[c]
			}
			b.WriteString(`<w:tc><w:tcPr><w:tcW w:w="` + itoa(colW) + `" w:type="dxa"/>`)
			if ri < blk.HeaderRows {
				b.WriteString(`<w:shd w:val="clear" w:color="auto" w:fill="EFF3FA"/>`)
			}
			b.WriteString(`</w:tcPr>`)
			b.WriteString(docxPara(txt, "", ri < blk.HeaderRows))
			b.WriteString(`</w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	// 表格后补一个空段落，避免两张表粘连
	b.WriteString(`<w:p/>`)
	return b.String()
}

// TableFromSheet 把工作表转成表格块（导出 Word 报表用）。
func TableFromSheet(sh *Sheet, maxRows int) Block {
	if sh == nil {
		return Block{Kind: BlockTable}
	}
	limit := len(sh.Rows)
	if maxRows > 0 && limit > maxRows {
		limit = maxRows
	}
	cols := sheetCols(sh)
	rows := make([][]string, 0, limit)
	for r := 0; r < limit; r++ {
		row := make([]string, cols)
		for c := 0; c < cols; c++ {
			row[c] = sh.Cell(r, c).Text
		}
		rows = append(rows, row)
	}
	header := 0
	if sh.Header && len(rows) > 0 {
		header = 1
	}
	return Block{Kind: BlockTable, Rows: rows, HeaderRows: header}
}

// SheetFromBlock 把表格块转成工作表（Word 表格 → Excel 用）。
func SheetFromBlock(blk Block, name string) Sheet {
	sh := Sheet{Name: name, Header: blk.HeaderRows > 0}
	if sh.Name == "" {
		sh.Name = "Sheet1"
	}
	for r, row := range blk.Rows {
		for c, txt := range row {
			sh.SetCell(r, c, Cell{Text: txt, Bold: r < blk.HeaderRows})
		}
	}
	return sh
}

// DocxFromSpreadsheet 工作簿 → Word 文档（每张表一节，标题为表名）。
func DocxFromSpreadsheet(wb *Spreadsheet, title string) *WordDoc {
	doc := &WordDoc{Title: title}
	if title != "" {
		doc.Blocks = append(doc.Blocks, Heading(1, title))
	}
	if wb == nil {
		return doc
	}
	for i := range wb.Sheets {
		sh := &wb.Sheets[i]
		doc.Blocks = append(doc.Blocks, Heading(2, sh.Name))
		doc.Blocks = append(doc.Blocks, TableFromSheet(sh, 200))
	}
	return doc
}
