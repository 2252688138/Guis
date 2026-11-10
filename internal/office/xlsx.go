// guis — Excel（.xlsx）读写
//
// 只实现「表格数据 + 少量样式」这一层，足够做报表导出 / 预览 / 简单编辑：
//
//	读：workbook.xml + rels 定位每张表 → sharedStrings + inlineStr + 数值 + 日期格式
//	写：一个最小合法 OOXML 包（Content-Types / workbook / rels / sheets / styles）
//
// 不支持的（刻意略过，保持零依赖与体积）：图表、图片、数据透视、公式计算
// （公式只保留缓存值）、合并单元格（预览按左上角显示）。

package office

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// ---- 读 ----

// ReadXLSX 读取 .xlsx 文件。
func ReadXLSX(path string) (*Spreadsheet, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadXLSXBytes(b)
}

type xlsxWorkbookXML struct {
	Sheets struct {
		Sheet []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"id,attr"`
		} `xml:"sheet"`
	} `xml:"sheets"`
}

type xlsxRelsXML struct {
	Rel []struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
		Type   string `xml:"Type,attr"`
	} `xml:"Relationship"`
}

type xlsxSSTXML struct {
	SI []struct {
		T string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

type xlsxStylesXML struct {
	NumFmts struct {
		NumFmt []struct {
			NumFmtID   int    `xml:"numFmtId,attr"`
			FormatCode string `xml:"formatCode,attr"`
		} `xml:"numFmt"`
	} `xml:"numFmts"`
	Fonts struct {
		Font []struct {
			B *struct{} `xml:"b"`
		} `xml:"font"`
	} `xml:"fonts"`
	Fills struct {
		Fill []struct {
			PatternFill struct {
				FgColor struct {
					RGB   string `xml:"rgb,attr"`
					Theme string `xml:"theme,attr"`
					Index string `xml:"indexed,attr"`
				} `xml:"fgColor"`
			} `xml:"patternFill"`
		} `xml:"fill"`
	} `xml:"fills"`
	CellXfs struct {
		Xf []struct {
			NumFmtID int    `xml:"numFmtId,attr"`
			FontID   int    `xml:"fontId,attr"`
			FillID   int    `xml:"fillId,attr"`
			ApplyNum string `xml:"applyNumberFormat,attr"`
		} `xml:"xf"`
	} `xml:"cellXfs"`
}

type xlsxSheetXML struct {
	Cols struct {
		Col []struct {
			Min   int     `xml:"min,attr"`
			Max   int     `xml:"max,attr"`
			Width float64 `xml:"width,attr"`
		} `xml:"col"`
	} `xml:"cols"`
	SheetData struct {
		Row []struct {
			R int `xml:"r,attr"`
			C []struct {
				R  string `xml:"r,attr"`
				T  string `xml:"t,attr"`
				S  int    `xml:"s,attr"`
				V  string `xml:"v"`
				IS struct {
					T string `xml:"t"`
					R []struct {
						T string `xml:"t"`
					} `xml:"r"`
				} `xml:"is"`
			} `xml:"c"`
		} `xml:"row"`
	} `xml:"sheetData"`
}

// ReadXLSXBytes 从内存解析 .xlsx。
func ReadXLSXBytes(data []byte) (*Spreadsheet, error) {
	files, err := readZip(data)
	if err != nil {
		return nil, err
	}
	wbXML, ok := files["xl/workbook.xml"]
	if !ok {
		return nil, errors.New("office: 不是有效的 xlsx（缺少 xl/workbook.xml）")
	}
	var wb xlsxWorkbookXML
	if err := xmlUnmarshal(wbXML, &wb); err != nil {
		return nil, fmt.Errorf("office: workbook.xml 解析失败：%w", err)
	}

	// rels：rId → 工作表实际路径
	targets := map[string]string{}
	if relsXML, ok := files["xl/_rels/workbook.xml.rels"]; ok {
		var rels xlsxRelsXML
		if err := xmlUnmarshal(relsXML, &rels); err == nil {
			for _, r := range rels.Rel {
				targets[r.ID] = zipPath("xl/workbook.xml", r.Target)
			}
		}
	}

	sst := readSharedStrings(files["xl/sharedStrings.xml"])
	st := readStyles(files["xl/styles.xml"])

	out := &Spreadsheet{}
	sheets := wb.Sheets.Sheet
	for i, sh := range sheets {
		p := targets[sh.RID]
		if p == "" {
			// 没有 rels 时按约定路径兜底
			p = fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
		}
		raw, ok := files[p]
		if !ok {
			continue
		}
		sheet, err := parseSheetXML(raw, sst, st)
		if err != nil {
			return nil, err
		}
		sheet.Name = sh.Name
		if sheet.Name == "" {
			sheet.Name = "Sheet" + itoa(i+1)
		}
		out.Sheets = append(out.Sheets, *sheet)
	}
	if len(out.Sheets) == 0 {
		return nil, errors.New("office: xlsx 中没有可读的工作表")
	}
	return out, nil
}

// readSharedStrings 读共享字符串表。
func readSharedStrings(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var sst xlsxSSTXML
	if err := xmlUnmarshal(data, &sst); err != nil {
		return nil
	}
	out := make([]string, 0, len(sst.SI))
	for _, si := range sst.SI {
		var b strings.Builder
		b.WriteString(si.T)
		for _, r := range si.R {
			b.WriteString(r.T)
		}
		out = append(out, b.String())
	}
	return out
}

// xlsxStyle 解析后的样式（按 cellXfs 下标索引）。
type xlsxStyle struct {
	bold     bool
	numFmtID int
	format   string
	fill     string
}

// readStyles 解析 styles.xml。
func readStyles(data []byte) []xlsxStyle {
	if len(data) == 0 {
		return nil
	}
	var st xlsxStylesXML
	if err := xmlUnmarshal(data, &st); err != nil {
		return nil
	}
	customFmt := map[int]string{}
	for _, nf := range st.NumFmts.NumFmt {
		customFmt[nf.NumFmtID] = nf.FormatCode
	}
	bold := make([]bool, len(st.Fonts.Font))
	for i, f := range st.Fonts.Font {
		bold[i] = f.B != nil
	}
	fills := make([]string, len(st.Fills.Fill))
	for i, f := range st.Fills.Fill {
		rgb := f.PatternFill.FgColor.RGB
		if len(rgb) == 8 { // ARGB → #rrggbb
			rgb = rgb[2:]
		}
		if len(rgb) == 6 {
			fills[i] = "#" + strings.ToLower(rgb)
		}
	}
	out := make([]xlsxStyle, len(st.CellXfs.Xf))
	for i, xf := range st.CellXfs.Xf {
		s := xlsxStyle{numFmtID: xf.NumFmtID}
		if xf.FontID >= 0 && xf.FontID < len(bold) {
			s.bold = bold[xf.FontID]
		}
		if xf.FillID >= 0 && xf.FillID < len(fills) {
			s.fill = fills[xf.FillID]
		}
		if code, ok := customFmt[xf.NumFmtID]; ok {
			s.format = code
		}
		out[i] = s
	}
	return out
}

// parseSheetXML 解析单张工作表。
func parseSheetXML(data []byte, sst []string, styles []xlsxStyle) (*Sheet, error) {
	var sx xlsxSheetXML
	if err := xmlUnmarshal(data, &sx); err != nil {
		return nil, fmt.Errorf("office: worksheet 解析失败：%w", err)
	}
	sh := &Sheet{}
	for ri, row := range sx.SheetData.Row {
		r := row.R
		if r <= 0 {
			r = ri + 1
		}
		r--
		// 保证行号连续（空行也要占位，否则表格错位）
		for len(sh.Rows) < r+1 {
			sh.Rows = append(sh.Rows, nil)
		}
		if len(sh.Rows) > 20000 {
			break
		}
		for ci, c := range row.C {
			rr, cc := parseColName(c.R)
			if rr < 0 {
				rr, cc = r, ci
			}
			var st xlsxStyle
			if c.S >= 0 && c.S < len(styles) {
				st = styles[c.S]
			}
			cell := Cell{Bold: st.bold, Fill: st.fill}
			switch c.T {
			case "s": // 共享字符串
				if n := atoiSafe(c.V); n >= 0 && n < len(sst) {
					cell.Text = sst[n]
				}
			case "inlineStr":
				var b strings.Builder
				b.WriteString(c.IS.T)
				for _, run := range c.IS.R {
					b.WriteString(run.T)
				}
				cell.Text = b.String()
			case "str":
				cell.Text = c.V
			case "b":
				if c.V == "1" {
					cell.Text = "TRUE"
				} else {
					cell.Text = "FALSE"
				}
			default: // 数值 / 日期
				if c.V == "" {
					continue
				}
				f, err := parseFloatSafe(c.V)
				if err != nil {
					cell.Text = c.V
				} else if isDateFormat(st.numFmtID, st.format) {
					cell.Text = excelDate(f, st.format)
					cell.Value, cell.Num = f, true
				} else {
					cell.Text = FormatNumber(f)
					cell.Value, cell.Num = f, true
					if st.format != "" && strings.Contains(st.format, "%") {
						cell.Text = FormatNumber(f*100) + "%"
					}
				}
			}
			sh.SetCell(rr, cc, cell)
		}
	}
	// 首行整行加粗 → 认定是表头
	if len(sh.Rows) > 0 {
		allBold, any := true, false
		for _, c := range sh.Rows[0] {
			if strings.TrimSpace(c.Text) == "" {
				continue
			}
			any = true
			if !c.Bold {
				allBold = false
			}
		}
		sh.Header = any && allBold
	}
	return sh, nil
}

// isDateFormat 判断数字格式是否为日期（内置 id + 自定义格式串）。
func isDateFormat(numFmtID int, code string) bool {
	switch numFmtID {
	case 14, 15, 16, 17, 18, 19, 20, 21, 22, 45, 46, 47:
		return true
	}
	if numFmtID >= 27 && numFmtID <= 36 { // 部分语言的内置日期格式
		return true
	}
	if numFmtID >= 50 && numFmtID <= 58 {
		return true
	}
	if code == "" {
		return false
	}
	lower := strings.ToLower(code)
	if strings.ContainsAny(lower, "ymd") && !strings.Contains(lower, "#") {
		return true
	}
	if strings.Contains(lower, "h:mm") || strings.Contains(lower, "yyyy") {
		return true
	}
	return false
}

// excelDate Excel 序列号 → 文本（含 1900 闰年 bug 兼容）。
func excelDate(serial float64, code string) string {
	if serial < 0 {
		return FormatNumber(serial)
	}
	days := int(math.Floor(serial))
	frac := serial - float64(days)
	var base time.Time
	if days >= 60 {
		base = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	} else {
		base = time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	}
	t := base.AddDate(0, 0, days)
	timeOnly := strings.Contains(strings.ToLower(code), "h") &&
		!strings.Contains(strings.ToLower(code), "y") &&
		!strings.Contains(strings.ToLower(code), "d")
	if timeOnly {
		secs := int(math.Round(frac * 86400))
		return fmt.Sprintf("%02d:%02d", secs/3600, (secs%3600)/60)
	}
	if frac == 0 {
		return t.Format("2006-01-02")
	}
	secs := int(math.Round(frac * 86400))
	hh, mm := secs/3600, (secs%3600)/60
	return fmt.Sprintf("%s %02d:%02d", t.Format("2006-01-02"), hh, mm)
}

// atoiSafe 宽松解析非负整数。
func atoiSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return -1
		}
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// parseFloatSafe 解析浮点（支持科学计数与指数符号）。
func parseFloatSafe(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	var (
		neg     bool
		i       int
		intPart float64
		frac    float64
		div     = 1.0
		exp     int
		expNeg  bool
	)
	if s[i] == '+' || s[i] == '-' {
		neg = s[i] == '-'
		i++
	}
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		intPart = intPart*10 + float64(s[i]-'0')
	}
	if i < len(s) && s[i] == '.' {
		i++
		for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
			frac = frac*10 + float64(s[i]-'0')
			div *= 10
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			expNeg = s[i] == '-'
			i++
		}
		for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
			exp = exp*10 + int(s[i]-'0')
		}
	}
	if i != len(s) {
		return 0, fmt.Errorf("office: 非法数值 %q", s)
	}
	v := intPart + frac/div
	if exp != 0 {
		mul := math.Pow(10, float64(exp))
		if expNeg {
			v /= mul
		} else {
			v *= mul
		}
	}
	if neg {
		v = -v
	}
	return v, nil
}

// ---- 写 ----

// WriteXLSX 把工作簿写成 .xlsx 文件。
func WriteXLSX(path string, wb *Spreadsheet) error {
	b, err := WriteXLSXBytes(wb)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// WriteXLSXBytes 生成 .xlsx 字节。多张表 ≥1；没有表时补一张空表（Excel 不接受 0 表）。
func WriteXLSXBytes(wb *Spreadsheet) ([]byte, error) {
	if wb == nil || len(wb.Sheets) == 0 {
		wb = &Spreadsheet{Sheets: []Sheet{{Name: "Sheet1"}}}
	}
	names := sheetNames(wb)

	z := newZipBuilder()
	// [Content_Types].xml
	var ct strings.Builder
	ct.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	ct.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	ct.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	ct.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	ct.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	for i := range wb.Sheets {
		ct.WriteString(fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1))
	}
	ct.WriteString(`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	ct.WriteString(`</Types>`)
	z.add("[Content_Types].xml", ct.String())

	z.add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>`+
		`</Relationships>`)

	// workbook.xml
	var wbx strings.Builder
	wbx.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	wbx.WriteString(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`)
	for i, n := range names {
		wbx.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, esc(n), i+1, i+1))
	}
	wbx.WriteString(`</sheets></workbook>`)
	z.add("xl/workbook.xml", wbx.String())

	// workbook.xml.rels
	var rels strings.Builder
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	rels.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := range wb.Sheets {
		rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1))
	}
	rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, len(wb.Sheets)+1))
	rels.WriteString(`</Relationships>`)
	z.add("xl/_rels/workbook.xml.rels", rels.String())

	z.add("xl/styles.xml", xlsxStylesXMLText())

	for i, sh := range wb.Sheets {
		z.add(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), xlsxSheetXMLText(&sh))
	}
	return z.bytes()
}

// sheetNames 归一化工作表名：去非法字符、去重、空名补 SheetN。
func sheetNames(wb *Spreadsheet) []string {
	out := make([]string, len(wb.Sheets))
	used := map[string]bool{}
	for i, sh := range wb.Sheets {
		n := strings.TrimSpace(sh.Name)
		n = strings.Map(func(r rune) rune {
			switch r {
			case '\\', '/', '?', '*', '[', ']', ':':
				return ' '
			}
			return r
		}, n)
		if n == "" {
			n = "Sheet" + itoa(i+1)
		}
		if len([]rune(n)) > 31 {
			n = string([]rune(n)[:31])
		}
		base := n
		for k := 2; used[strings.ToLower(n)]; k++ {
			n = base + "-" + itoa(k)
		}
		used[strings.ToLower(n)] = true
		out[i] = n
	}
	return out
}

// xlsxStylesXMLText 固定样式表：0=默认，1=加粗，2=表头（加粗+底色+居中+下边框）。
func xlsxStylesXMLText() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<fonts count="3">` +
		`<font><sz val="11"/><name val="Calibri"/></font>` +
		`<font><b/><sz val="11"/><name val="Calibri"/></font>` +
		`<font><sz val="12"/><name val="Calibri"/></font>` +
		`</fonts>` +
		`<fills count="3">` +
		`<fill><patternFill patternType="none"/></fill>` +
		`<fill><patternFill patternType="gray125"/></fill>` +
		`<fill><patternFill patternType="solid"><fgColor rgb="FFEFF3FA"/><bgColor indexed="64"/></patternFill></fill>` +
		`</fills>` +
		`<borders count="2">` +
		`<border><left/><right/><top/><bottom/><diagonal/></border>` +
		`<border><left/><right/><top/><bottom style="thin"><color rgb="FFD0D7E2"/></bottom><diagonal/></border>` +
		`</borders>` +
		`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
		`<cellXfs count="3">` +
		`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
		`<xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/>` +
		`<xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>` +
		`</cellXfs>` +
		`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
		`</styleSheet>`
}

// xlsxSheetXMLText 生成单张工作表 XML。
func xlsxSheetXMLText(sh *Sheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)

	// 列宽：按内容最长字符估算（6 ~ 48）
	if cols := sheetCols(sh); cols > 0 {
		b.WriteString(`<cols>`)
		for c := 0; c < cols; c++ {
			w := 10.0
			for r := range sh.Rows {
				if n := float64(len([]rune(sh.Cell(r, c).Text))); n > 0 {
					if n+2 > w {
						w = n + 2
					}
				}
			}
			if w > 48 {
				w = 48
			}
			b.WriteString(fmt.Sprintf(`<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, c+1, c+1, w))
		}
		b.WriteString(`</cols>`)
	}

	b.WriteString(`<sheetData>`)
	for r := range sh.Rows {
		b.WriteString(fmt.Sprintf(`<row r="%d">`, r+1))
		for c := range sh.Rows[r] {
			cell := sh.Cell(r, c)
			if strings.TrimSpace(cell.Text) == "" && !cell.Num {
				continue
			}
			style := 0
			if sh.Header && r == 0 {
				style = 2
			} else if cell.Bold {
				style = 1
			}
			ref := a1(r, c)
			// 表头行一律按文本写（表头写数值会让 Excel 当数字统计）
			if cell.Num && !(sh.Header && r == 0) {
				fmt.Fprintf(&b, `<c r="%s" s="%d"><v>%s</v></c>`, ref, style, numLiteral(cell.Value))
				continue
			}
			fmt.Fprintf(&b, `<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`,
				ref, style, esc(cell.Text))
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

// numLiteral 数值 → 最简字面量（避免 1.0000001 这种噪声）。
func numLiteral(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return itoa(int(v))
	}
	return trimFloatFull(v)
}

// trimFloatFull 输出最多 10 位有效小数（Excel 内部按 double 存）。
func trimFloatFull(v float64) string {
	s := fmt.Sprintf("%.10f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// sheetCols 工作表最大列数。
func sheetCols(sh *Sheet) int {
	n := sh.Cols
	for _, row := range sh.Rows {
		if len(row) > n {
			n = len(row)
		}
	}
	return n
}

// SortRowsBy 按某列排序（数值优先，其次文本；跳过表头行）。
func (s *Sheet) SortRowsBy(col int, asc bool) {
	if s == nil || len(s.Rows) < 2 {
		return
	}
	start := 0
	if s.Header {
		start = 1
	}
	body := s.Rows[start:]
	sort.SliceStable(body, func(i, j int) bool {
		a, b := body[i], body[j]
		var ca, cb Cell
		if col >= 0 && col < len(a) {
			ca = a[col]
		}
		if col >= 0 && col < len(b) {
			cb = b[col]
		}
		if ca.Num && cb.Num {
			if asc {
				return ca.Value < cb.Value
			}
			return ca.Value > cb.Value
		}
		if asc {
			return ca.Text < cb.Text
		}
		return ca.Text > cb.Text
	})
	s.Rows = append(s.Rows[:start], body...)
}

// SumColumn 统计某列数值合计（跳过表头）。
func (s *Sheet) SumColumn(col int) float64 {
	if s == nil {
		return 0
	}
	start := 0
	if s.Header {
		start = 1
	}
	var sum float64
	for r := start; r < len(s.Rows); r++ {
		if c := s.Cell(r, col); c.Num {
			sum += c.Value
		}
	}
	return sum
}
