// guis — PDF 写入（生成 PDF 1.7，含中文 TrueType 子集嵌入）
//
// 用法与 ui.PrintCanvas 完全一致（原点左上、Y 向下、单位 pt）：
//
//	doc := &office.PDFDoc{}
//	page := office.NewPDFPage(0, 0)
//	page.Text(56, 64, 20, "季度经营报表")
//	page.Line(56, 84, 539, 84, 1, "#e5e7eb")
//	page.Rect(56, 96, 483, 22, 0, "", "#eef2fa")
//	doc.Pages = append(doc.Pages, *page)
//	office.WritePDF("报表.pdf", doc)
//
// 中文处理：默认自动找系统 TrueType 中文字体（雅黑 / 黑体 / 苹方 / DejaVu），
// 只嵌入文档里真正用到的字形（子集化），几百字通常 20~60KB；
// 找不到字体时回退内置 Helvetica（仅西文，中文会被替换成 '?'）。

package office

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"sort"
	"strings"
)

// PDFOptions PDF 生成选项。
type PDFOptions struct {
	// Title / Author 文档属性（可选）。
	Title, Author string
	// FontPath 显式指定 TrueType 字体文件（.ttf / .ttc）；空 = 自动查找系统中文字体。
	FontPath string
	// FaceIndex TTC 集合里的第几个 face（默认 0）。
	FaceIndex int
	// NoCompress 关闭内容流压缩（调试 / 便于手看）。
	NoCompress bool
	// NoEmbed 不嵌入字体（只用内置 Helvetica，中文会变成 '?'）。
	NoEmbed bool
}

// WritePDF 把文档写成 .pdf 文件。
func WritePDF(path string, doc *PDFDoc) error {
	b, err := WritePDFBytes(doc, PDFOptions{})
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// WritePDFWith 带选项写文件。
func WritePDFWith(path string, doc *PDFDoc, opts PDFOptions) error {
	b, err := WritePDFBytes(doc, opts)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// pdfFont 生成端字体（子集 or 内置 Helvetica）。
type pdfFont struct {
	subset *subsetResult
	// embed 是否嵌入 TrueType 子集。
	embed bool
	// ascent 基线偏移比例（0~1，相对字号）。
	ascent float32
}

// pdfWriter PDF 对象拼装器。
type pdfWriter struct {
	objs []*bytes.Buffer
}

// addObj 追加一个对象，返回对象号。
func (w *pdfWriter) addObj(body string) int {
	w.objs = append(w.objs, bytes.NewBufferString(body))
	return len(w.objs)
}

// setObj 覆写对象内容。
func (w *pdfWriter) setObj(num int, body string) {
	if num <= 0 || num > len(w.objs) {
		return
	}
	w.objs[num-1].Reset()
	w.objs[num-1].WriteString(body)
}

// WritePDFBytes 生成 PDF 字节。
func WritePDFBytes(doc *PDFDoc, opts PDFOptions) ([]byte, error) {
	if doc == nil || len(doc.Pages) == 0 {
		doc = &PDFDoc{Pages: []PDFPage{*NewPDFPage(0, 0)}}
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = doc.Title
	}

	// ---- 1) 收集用字并加载字体 ----
	used := collectRunes(doc)
	font := &pdfFont{ascent: 0.718}
	if !opts.NoEmbed {
		if path, idx := pdfFontFile(opts.FontPath, opts.FaceIndex); path != "" {
			if data, err := os.ReadFile(path); err == nil {
				if tt, err := parseTrueType(data, idx); err == nil {
					sub := tt.subset(used)
					if len(sub.Font) > 0 {
						font = &pdfFont{subset: sub, embed: true}
						if sub.Ascent > 0 {
							font.ascent = float32(sub.Ascent) / 1000
						}
					}
				}
			}
		}
	}

	w := &pdfWriter{}
	// 1=Catalogue, 2=Pages
	catalogNum := w.addObj("")
	pagesNum := w.addObj("")

	// ---- 2) 字体对象 ----
	fontNum := 0
	descNum, fileNum, tounicodeNum := 0, 0, 0
	if font.embed {
		sub := font.subset
		psName := sanitizePSName(sub.PSName)
		// FontFile2：嵌入子集字体（Flate 压缩）
		compressed := deflate(sub.Font)
		fileNum = w.addObj(fmt.Sprintf("<< /Length %d /Length1 %d /Filter /FlateDecode >>\nstream\n", len(compressed), len(sub.Font)) + string(compressed) + "\nendstream")
		bbox := fmt.Sprintf("[%d %d %d %d]", sub.BBox[0], sub.BBox[1], sub.BBox[2], sub.BBox[3])
		descNum = w.addObj(fmt.Sprintf(
			"<< /Type /FontDescriptor /FontName /%s /Flags 4 /FontBBox %s /ItalicAngle 0 /Ascent %d /Descent %d /CapHeight %d /StemV 80 /FontFile2 %d 0 R >>",
			psName, bbox, sub.Ascent, sub.Descent, sub.CapHeight, fileNum))
		tounicodeNum = w.addObj(toUnicodeCMap(sub))
		// CIDFont：Identity-H 编码，CIDToGIDMap /Identity（子集里 CID == GID）
		cidFontNum := w.addObj(fmt.Sprintf(
			"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /%s /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor %d 0 R /DW 1000 /W %s /CIDToGIDMap /Identity >>",
			psName, descNum, type2Widths(sub.Widths)))
		fontNum = w.addObj(fmt.Sprintf(
			"<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H /DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>",
			psName, cidFontNum, tounicodeNum))
	} else {
		fontNum = w.addObj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	}

	// ---- 3) 各页面 ----
	pageNums := make([]int, 0, len(doc.Pages))
	for _, pg := range doc.Pages {
		wid, hei := pg.W, pg.H
		if wid <= 0 || hei <= 0 {
			wid, hei = A4[0], A4[1]
		}
		content := buildPageContent(&pg, wid, hei, font)
		var streamObj string
		if opts.NoCompress {
			streamObj = fmt.Sprintf("<< /Length %d >>\nstream\n", len(content)) + content + "\nendstream"
		} else {
			enc := deflate([]byte(content))
			streamObj = fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n", len(enc)) + string(enc) + "\nendstream"
		}
		contentNum := w.addObj(streamObj)
		pageNum := w.addObj(fmt.Sprintf(
			"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>",
			pagesNum, wid, hei, fontNum, contentNum))
		pageNums = append(pageNums, pageNum)
	}

	// ---- 4) 目录 / 页面树 / 文档信息 ----
	var kids strings.Builder
	for i, n := range pageNums {
		if i > 0 {
			kids.WriteByte(' ')
		}
		fmt.Fprintf(&kids, "%d 0 R", n)
	}
	w.setObj(pagesNum, fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", len(pageNums), kids.String()))
	w.setObj(catalogNum, fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesNum))
	infoNum := w.addObj(fmt.Sprintf("<< /Producer (GUIS office) /Title %s >>", pdfTextString(title)))

	// ---- 5) 序列化（带 xref 表）----
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(w.objs)+1)
	for i, b := range w.objs {
		offsets[i+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(b.Bytes())
		out.WriteString("\nendobj\n")
	}
	xrefPos := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n", len(w.objs)+1)
	out.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(w.objs); i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(w.objs)+1, catalogNum, infoNum, xrefPos)
	return out.Bytes(), nil
}

// buildPageContent 把一页的绘制指令编码成内容流。
func buildPageContent(pg *PDFPage, w, h float32, font *pdfFont) string {
	var b strings.Builder
	// 白色底（预览 / 打印都按纸面处理）
	fmt.Fprintf(&b, "1 1 1 rg 0 0 %.2f %.2f re f\n", w, h)
	for _, it := range pg.Items {
		switch it.Kind {
		case PDFText:
			txt := strings.ReplaceAll(strings.ReplaceAll(it.Text, "\r", ""), "\n", " ")
			if txt == "" {
				continue
			}
			size := it.Size
			if size <= 0 {
				size = 12
			}
			r, g, bl := parseHexColor(it.Color)
			baseline := h - it.Y - size*font.ascent
			fmt.Fprintf(&b, "BT %.3f %.3f %.3f rg /F1 %.2f Tf ", r, g, bl, size)
			if it.Bold {
				fmt.Fprintf(&b, "%.2f w %.3f %.3f %.3f RG 2 Tr ", size*0.035, r, g, bl)
			}
			fmt.Fprintf(&b, "%.2f %.2f Td %s Tj ET\n", it.X, baseline, pdfString(txt, font))
		case PDFLine:
			r, g, bl := parseHexColor(it.Color)
			lw := it.Stroke
			if lw <= 0 {
				lw = 1
			}
			fmt.Fprintf(&b, "%.3f %.3f %.3f RG %.2f w %.2f %.2f m %.2f %.2f l S\n",
				r, g, bl, lw, it.X, h-it.Y, it.X2, h-it.Y2)
		case PDFRect:
			r, g, bl := parseHexColor(it.Fill)
			x, y := it.X, h-it.Y-it.H
			if it.Fill != "" {
				fmt.Fprintf(&b, "%.3f %.3f %.3f rg %.2f %.2f %.2f %.2f re f\n", r, g, bl, x, y, it.W, it.H)
			}
			if it.Stroke > 0 && it.Color != "" {
				sr, sg, sb := parseHexColor(it.Color)
				fmt.Fprintf(&b, "%.3f %.3f %.3f RG %.2f w %.2f %.2f %.2f %.2f re S\n",
					sr, sg, sb, it.Stroke, x, y, it.W, it.H)
			}
		case PDFImage:
			// 图片暂画占位框（避免误以为内容丢失）
			fmt.Fprintf(&b, "0.92 0.94 0.98 rg %.2f %.2f %.2f %.2f re f 0.6 0.65 0.75 RG 0.5 w %.2f %.2f %.2f %.2f re S\n",
				it.X, h-it.Y-it.H, it.W, it.H, it.X, h-it.Y-it.H, it.W, it.H)
		}
	}
	return b.String()
}

// pdfString 文本 → PDF 字符串字面量。
//
// 嵌入子集时用 Identity-H：每字符写成 2 字节 CID（十六进制，避免转义）。
// 未嵌入时用 WinAnsi：非 ASCII 一律替换成 '?'（否则阅读器解码乱码）。
func pdfString(s string, font *pdfFont) string {
	if font.embed && font.subset != nil {
		var b strings.Builder
		b.WriteByte('<')
		for _, r := range s {
			cid, ok := font.subset.CIDs[r]
			if !ok {
				if cid, ok = font.subset.CIDs['?']; !ok {
					cid = 0
				}
			}
			fmt.Fprintf(&b, "%04X", cid)
		}
		b.WriteByte('>')
		return b.String()
	}
	var b strings.Builder
	b.WriteByte('(')
	for _, r := range s {
		switch {
		case r == '(' || r == ')' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r >= 32 && r < 127:
			b.WriteRune(r)
		default:
			b.WriteByte('?')
		}
	}
	b.WriteByte(')')
	return b.String()
}

// pdfTextString 文档信息里的文本（UTF-16BE + BOM 十六进制串）。
func pdfTextString(s string) string {
	if s == "" {
		return "()"
	}
	ascii := true
	for _, r := range s {
		if r < 32 || r > 126 {
			ascii = false
			break
		}
	}
	if ascii {
		return "(" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), ")", "\\)") + ")"
	}
	var b strings.Builder
	b.WriteString("<FEFF")
	for _, r := range s {
		// UTF-16BE（含代理对）
		if r > 0xFFFF {
			r -= 0x10000
			fmt.Fprintf(&b, "%04X%04X", 0xD800+(r>>10), 0xDC00+(r&0x3FF))
			continue
		}
		fmt.Fprintf(&b, "%04X", r)
	}
	b.WriteByte('>')
	return b.String()
}

// type2Widths 把宽度数组编码成 PDF 的 /W 数组（[start [w1 w2 …]] 形式）。
func type2Widths(widths []uint16) string {
	if len(widths) == 0 {
		return "[]"
	}
	// 默认宽度 1000 由 /DW 提供；这里只写有值的前缀段
	n := len(widths)
	for n > 0 && widths[n-1] == 1000 {
		n--
	}
	if n == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[1 [")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%d", widths[i])
	}
	b.WriteString("]]")
	return b.String()
}

// toUnicodeCMap 生成 ToUnicode CMap（保证 PDF 文本可复制 / 可检索）。
func toUnicodeCMap(sub *subsetResult) string {
	runes := make([]rune, 0, len(sub.CIDs))
	for r := range sub.CIDs {
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool {
		return sub.CIDs[runes[i]] < sub.CIDs[runes[j]]
	})
	var b strings.Builder
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n")
	b.WriteString("/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n")
	b.WriteString("/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n")
	fmt.Fprintf(&b, "1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	// bfchar 每段最多 100 条
	for start := 0; start < len(runes); start += 100 {
		end := start + 100
		if end > len(runes) {
			end = len(runes)
		}
		fmt.Fprintf(&b, "%d beginbfchar\n", end-start)
		for _, r := range runes[start:end] {
			fmt.Fprintf(&b, "<%04X> <%s>\n", sub.CIDs[r], utf16BEHex(r))
		}
		b.WriteString("endbfchar\n")
	}
	b.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend")
	body := b.String()
	var out strings.Builder
	fmt.Fprintf(&out, "<< /Length %d >>\nstream\n", len(body))
	out.WriteString(body)
	out.WriteString("\nendstream")
	return out.String()
}

// utf16BEHex rune → UTF-16BE 十六进制（代理对处理）。
func utf16BEHex(r rune) string {
	if r > 0xFFFF {
		r -= 0x10000
		return fmt.Sprintf("%04X%04X", 0xD800+(r>>10), 0xDC00+(r&0x3FF))
	}
	return fmt.Sprintf("%04X", r)
}

// collectRunes 收集文档里所有文本用到的 rune（去重）。
func collectRunes(doc *PDFDoc) []rune {
	seen := map[rune]bool{}
	var out []rune
	add := func(s string) {
		for _, r := range s {
			if r == '\n' || r == '\r' || r == '\t' {
				continue
			}
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	for _, pg := range doc.Pages {
		for _, it := range pg.Items {
			if it.Kind == PDFText {
				add(it.Text)
			}
		}
	}
	return out
}

// splitHexColor "#2563eb" → (0.145, 0.388, 0.902)；非法返回黑色。
func splitHexColor(s string) (float32, float32, float32) {
	return parseHexColor(s)
}

// deflate zlib 压缩（PDF FlateDecode）。
func deflate(data []byte) []byte {
	var buf bytes.Buffer
	zw, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		return data
	}
	if _, err := zw.Write(data); err != nil {
		return data
	}
	if err := zw.Close(); err != nil {
		return data
	}
	return buf.Bytes()
}

// sanitizePSName 归一化 PostScript 名（去掉空格 / 特殊字符）。
func sanitizePSName(s string) string {
	if s == "" {
		return "GUISEmbedded"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ':
			continue
		}
	}
	out := b.String()
	if out == "" {
		return "GUISEmbedded"
	}
	return out
}
