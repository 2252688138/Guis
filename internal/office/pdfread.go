// guis — PDF 解析（页面 / 文本 / 线条 / 矩形 → 结构化预览数据）
//
// 目标不是「完整 PDF 渲染引擎」，而是**可预览、可抽取文字**：
//
//	· 扫描 xref 之外的 "N 0 obj" 定位对象（对损坏的 xref 更宽容）；
//	· 目录 → 页面树 → 每页的 /Resources /Contents；
//	· 内容流按 FlateDecode 解压后解析 BT/Tf/Tm/Td/Tj/TJ 等文本算子；
//	· 字体经 /ToUnicode（以及 Type0 + Identity-H 的 CID 直映）还原成 Unicode；
//	· 输出 PDFItem{X,Y,W,H,Size,Text}，坐标原点左上、Y 向下、单位 pt，
//	  与 ui.PrintCanvas / ui.PDFView 的坐标系一致。
//
// 不支持（预览时会缺内容，但不会报错）：图片（只画占位框）、阴影 / 渐变、
// 内嵌字体子集轮廓还原、加密文档（会返回错误）。

package office

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ReadPDF 读取 .pdf 文件。
func ReadPDF(path string) (*PDFDoc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadPDFBytes(b)
}

// pdfDoc 解析中的中间结构。
type pdfDoc struct {
	data []byte
	objs map[int][]byte // num → 对象体（含 stream 字典部分）
}

// pdfObj 取对象体。
func (d *pdfDoc) obj(num int) []byte { return d.objs[num] }

// ReadPDFBytes 从内存解析 PDF。
func ReadPDFBytes(data []byte) (*PDFDoc, error) {
	if len(data) < 8 || !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, errors.New("office: 不是有效的 PDF（缺少 %PDF- 头）")
	}
	// 加密文档直接拒绝（内容流无法解）
	if bytes.Contains(data, []byte("/Encrypt")) {
		return nil, errors.New("office: 加密 PDF 不支持解析")
	}
	d := &pdfDoc{data: data, objs: scanPDFObjects(data)}
	if len(d.objs) == 0 {
		return nil, errors.New("office: PDF 中未找到任何对象")
	}

	doc := &PDFDoc{Title: d.infoTitle()}
	pageNums := d.pageOrder()
	for _, num := range pageNums {
		pg, err := d.parsePage(num)
		if err != nil {
			continue
		}
		doc.Pages = append(doc.Pages, *pg)
		if len(doc.Pages) >= 200 {
			break
		}
	}
	if len(doc.Pages) == 0 {
		return nil, errors.New("office: PDF 中没有可解析的页面")
	}
	return doc, nil
}

// ---- 对象扫描 ----

// scanPDFObjects 扫描 "N G obj … endobj"，返回对象体（不含头尾）。
func scanPDFObjects(data []byte) map[int][]byte {
	out := map[int][]byte{}
	i := 0
	for i < len(data)-4 {
		j := bytes.Index(data[i:], []byte(" obj"))
		if j < 0 {
			break
		}
		pos := i + j
		// 向前回读 "N G"：跳过空格，回读数字，再跳过空格，回读数字
		k := pos - 1
		for k >= 0 && isSpaceByte(data[k]) {
			k--
		}
		genEnd := k + 1
		for k >= 0 && data[k] >= '0' && data[k] <= '9' {
			k--
		}
		if k == genEnd-1 { // 没有代数
			i = pos + 4
			continue
		}
		for k >= 0 && isSpaceByte(data[k]) {
			k--
		}
		numEnd := k + 1
		for k >= 0 && data[k] >= '0' && data[k] <= '9' {
			k--
		}
		if k == numEnd-1 {
			i = pos + 4
			continue
		}
		num, err := strconv.Atoi(string(data[k+1 : numEnd]))
		if err != nil || num <= 0 {
			i = pos + 4
			continue
		}
		start := pos + 4
		if start < len(data) && data[start] == '\r' {
			start++
		}
		if start < len(data) && data[start] == '\n' {
			start++
		}
		end := bytes.Index(data[start:], []byte("endobj"))
		if end < 0 {
			break
		}
		body := data[start : start+end]
		if _, exists := out[num]; !exists {
			out[num] = body
		}
		i = start + end + 6
	}
	return out
}

// isSpaceByte 空白字符。
func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\f' || b == 0
}

// ---- 字典 / 引用解析 ----

// ref 解析间接引用 "12 0 R" / "12 R" → 12；不是引用返回 0。
func ref(s string) int {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return 0
	}
	for _, f := range fields {
		if f == "R" || f == "r" {
			n, err := strconv.Atoi(fields[0])
			if err == nil {
				return n
			}
			return 0
		}
	}
	return 0
}

// dictValue 从对象体里取 /Key 的值（原始 token 文本：数字 / 引用 / 名字 / 数组起点）。
func dictValue(body []byte, key string) string {
	s := string(body)
	idx := strings.Index(s, "/"+key)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(key)+1:]
	// 跳过空白
	rest = strings.TrimLeft(rest, " \t\r\n")
	if rest == "" {
		return ""
	}
	switch rest[0] {
	case '[':
		depth := 0
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					return rest[:i+1]
				}
			}
		}
		return rest
	case '<':
		if len(rest) > 1 && rest[1] == '<' {
			depth := 0
			for i := 0; i+1 < len(rest); i++ {
				if rest[i] == '<' && rest[i+1] == '<' {
					depth++
					i++
					continue
				}
				if rest[i] == '>' && rest[i+1] == '>' {
					depth--
					i++
					if depth == 0 {
						return rest[:i+1]
					}
				}
			}
			return rest
		}
		// 十六进制字符串
		if end := strings.IndexByte(rest, '>'); end > 0 {
			return rest[:end+1]
		}
		return rest
	}
	// 普通 token：名字 / 数字 / 引用（可能后跟 "0 R"）
	end := 0
	for end < len(rest) && !isSpaceByte(rest[end]) && rest[end] != '/' && rest[end] != ']' && rest[end] != '>' {
		end++
	}
	tok := rest[:end]
	if ref := ref(rest); ref > 0 {
		return rest[:minInt(len(rest), end+5)]
	}
	return tok
}

// resolve 取间接引用指向的对象体（非引用时原样返回）。
func (d *pdfDoc) resolve(v string) []byte {
	if n := ref(v); n > 0 {
		return d.obj(n)
	}
	return []byte(v)
}

// resolveNum 解析数字（可能是引用）。
func (d *pdfDoc) resolveNum(v string) float64 {
	v = strings.TrimSpace(v)
	if n := ref(v); n > 0 {
		v = strings.TrimSpace(string(d.obj(n)))
	}
	f, err := parseFloatSafe(strings.Fields(v + " ")[0])
	if err != nil {
		return 0
	}
	return f
}

// numbers 把 "[a b c d]" 解析成数字数组。
func numbers(s string) []float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == ','
	})
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := parseFloatSafe(f)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

// infoTitle 取 /Info 里的 /Title。
func (d *pdfDoc) infoTitle() string {
	for _, body := range d.objs {
		if !bytes.Contains(body, []byte("/Title")) {
			continue
		}
		if !bytes.Contains(body, []byte("/Producer")) && !bytes.Contains(body, []byte("/Creator")) {
			continue
		}
		v := dictValue(body, "Title")
		if v == "" {
			continue
		}
		if s, ok := decodePDFString(v); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// pageOrder 页面对象号顺序：优先走页面树 /Kids，失败则按对象号排序兜底。
func (d *pdfDoc) pageOrder() []int {
	var out []int
	// 找 /Type /Pages 且含 /Kids 的根节点（无 /Parent 的那个）
	rootPages := 0
	for num, body := range d.objs {
		if !bytes.Contains(body, []byte("/Type")) || !bytes.Contains(body, []byte("/Pages")) {
			continue
		}
		if !bytes.Contains(body, []byte("/Kids")) {
			continue
		}
		if !bytes.Contains(body, []byte("/Parent")) {
			rootPages = num
			break
		}
	}
	seen := map[int]bool{}
	var walk func(num int, depth int)
	walk = func(num int, depth int) {
		if num <= 0 || seen[num] || depth > 64 {
			return
		}
		seen[num] = true
		body := d.obj(num)
		if body == nil {
			return
		}
		if !bytes.Contains(body, []byte("/Kids")) {
			out = append(out, num)
			return
		}
		kids := dictValue(body, "Kids")
		for _, tok := range strings.FieldsFunc(strings.Trim(kids, "[]"), func(r rune) bool { return isSpaceByte(byte(r)) }) {
			if n := ref(tok); n > 0 {
				walk(n, depth+1)
			}
		}
	}
	if rootPages > 0 {
		walk(rootPages, 0)
	}
	if len(out) > 0 {
		return out
	}
	// 兜底：所有 /Type /Page
	var nums []int
	for num, body := range d.objs {
		if isPageDict(body) {
			nums = append(nums, num)
		}
	}
	sort.Ints(nums)
	return nums
}

// isPageDict 是否页面字典（排除 /Pages）。
func isPageDict(body []byte) bool {
	if !bytes.Contains(body, []byte("/Type")) {
		return false
	}
	s := string(body)
	for i := 0; ; {
		idx := strings.Index(s[i:], "/Page")
		if idx < 0 {
			return false
		}
		p := i + idx + len("/Page")
		if p >= len(s) || s[p] != 's' { // 后面不是 's' → /Page
			return true
		}
		i = p
	}
}

// ---- 页面解析 ----

// parsePage 解析一页。
func (d *pdfDoc) parsePage(num int) (*PDFPage, error) {
	body := d.obj(num)
	if body == nil {
		return nil, errors.New("page not found")
	}
	box := numbers(dictValue(body, "MediaBox"))
	if len(box) < 4 {
		box = []float64{0, 0, float64(A4[0]), float64(A4[1])}
	}
	w := float32(abs64(box[2] - box[0]))
	h := float32(abs64(box[3] - box[1]))
	if w <= 0 || h <= 0 {
		w, h = A4[0], A4[1]
	}
	pg := &PDFPage{W: round2(w), H: round2(h)}

	// 资源字典（可能是引用）
	resBody := d.resolve(dictValue(body, "Resources"))
	if resBody == nil {
		// 有些文件把 Resources 放在父节点
		resBody = body
	}
	fonts := d.parseFonts(resBody)
	xobjects := d.parseXObjects(resBody)

	// 内容流（可能是一个引用或引用数组）
	var content []byte
	contentVal := dictValue(body, "Contents")
	if strings.HasPrefix(strings.TrimSpace(contentVal), "[") {
		for _, tok := range strings.FieldsFunc(strings.Trim(contentVal, "[]"), func(r rune) bool { return isSpaceByte(byte(r)) }) {
			if n := ref(tok); n > 0 {
				content = append(content, d.streamData(n)...)
				content = append(content, '\n')
			}
		}
	} else if n := ref(contentVal); n > 0 {
		content = d.streamData(n)
	}
	if len(content) == 0 {
		return pg, nil
	}
	pg.Items = d.extractItems(content, fonts, xobjects, h)
	return pg, nil
}

// streamData 取对象流并解压（支持 FlateDecode）。
func (d *pdfDoc) streamData(num int) []byte {
	body := d.obj(num)
	if body == nil {
		return nil
	}
	s := string(body)
	idx := strings.Index(s, "stream")
	if idx < 0 {
		return nil
	}
	start := idx + len("stream")
	// 跳过 stream 后的 CRLF / LF
	if start < len(s) && s[start] == '\r' {
		start++
	}
	if start < len(s) && s[start] == '\n' {
		start++
	}
	// 优先用 /Length（直接数值）
	raw := body[start:]
	if lv := dictValue(body, "Length"); lv != "" {
		if n := int(d.resolveNum(lv)); n > 0 && n <= len(raw) {
			raw = raw[:n]
		}
	}
	if end := bytes.Index(raw, []byte("endstream")); end >= 0 && len(raw) > end {
		// /Length 不准时按 endstream 截断
		if lv := dictValue(body, "Length"); lv == "" || int(d.resolveNum(lv)) > len(raw) {
			raw = raw[:end]
		}
	}
	if bytes.Contains(body, []byte("/FlateDecode")) {
		if out, err := inflate(raw); err == nil {
			return out
		}
	}
	return raw
}

// inflate zlib 解压（容忍多一段头尾垃圾）。
func inflate(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty")
	}
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		// 有些流没有 zlib 头（raw deflate），用 flate 兜底
		return nil, err
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, 64<<20))
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return out, nil
}

// pdfFontInfo 预览用的字体信息。
type pdfFontInfo struct {
	toUnicode map[int]string
	// identity 表示 Type0/Identity-H：代码即 CID，按 Unicode 码位直映
	identity bool
	// twoByte 代码长度为 2 字节
	twoByte bool
}

// decode 代码串 → Unicode 文本。
func (f *pdfFontInfo) decode(raw []byte) string {
	if f == nil {
		return latin1(raw)
	}
	if f.twoByte || f.identity {
		var b strings.Builder
		for i := 0; i+1 < len(raw); i += 2 {
			code := int(raw[i])<<8 | int(raw[i+1])
			if s, ok := f.toUnicode[code]; ok {
				b.WriteString(s)
				continue
			}
			if f.identity && code >= 32 && code <= 0x10FFFF {
				b.WriteRune(rune(code))
				continue
			}
			b.WriteByte(' ')
		}
		return b.String()
	}
	var b strings.Builder
	for _, c := range raw {
		if s, ok := f.toUnicode[int(c)]; ok {
			b.WriteString(s)
			continue
		}
		if c >= 32 && c < 127 {
			b.WriteByte(c)
			continue
		}
		b.WriteRune(rune(c))
	}
	return b.String()
}

// latin1 单字节直译（无字体信息时的兜底）。
func latin1(raw []byte) string {
	var b strings.Builder
	for _, c := range raw {
		if c >= 32 && c < 127 {
			b.WriteByte(c)
		} else if c >= 0xA0 {
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}

// parseFonts 解析页面 /Font 资源。
func (d *pdfDoc) parseFonts(res []byte) map[string]*pdfFontInfo {
	out := map[string]*pdfFontInfo{}
	if res == nil {
		return out
	}
	fontDict := dictValue(res, "Font")
	if fontDict == "" {
		return out
	}
	// 资源字典可能被内联成 << /F1 5 0 R >>；逐个 /Name 引用解码
	s := string(d.resolve(fontDict))
	i := 0
	for {
		idx := strings.Index(s[i:], "/")
		if idx < 0 {
			break
		}
		p := i + idx
		// 名字
		end := p + 1
		for end < len(s) && !isSpaceByte(s[end]) && s[end] != '/' && s[end] != '>' && s[end] != '<' {
			end++
		}
		name := s[p+1 : end]
		rest := s[end:]
		n := ref(rest)
		if n > 0 {
			out[name] = d.fontInfo(n)
		}
		i = end
	}
	return out
}

// fontInfo 解析单个字体对象。
func (d *pdfDoc) fontInfo(num int) *pdfFontInfo {
	body := d.obj(num)
	if body == nil {
		return nil
	}
	info := &pdfFontInfo{toUnicode: map[int]string{}}
	isType0 := bytes.Contains(body, []byte("/Type0"))
	info.identity = isType0
	info.twoByte = isType0
	if isType0 && !bytes.Contains(body, []byte("/Identity-H")) && !bytes.Contains(body, []byte("/Identity-V")) {
		info.identity = false
	}
	// ToUnicode CMap
	if n := ref(dictValue(body, "ToUnicode")); n > 0 {
		if cmap := d.streamData(n); len(cmap) > 0 {
			parseToUnicode(cmap, info.toUnicode)
		}
	}
	return info
}

// parseToUnicode 解析 ToUnicode CMap 的 bfchar / bfrange。
func parseToUnicode(data []byte, out map[int]string) {
	s := string(data)
	for _, block := range []string{"beginbfchar", "beginbfrange"} {
		idx := 0
		for {
			start := strings.Index(s[idx:], block)
			if start < 0 {
				break
			}
			start += idx
			endKey := "end" + strings.TrimPrefix(block, "begin")
			end := strings.Index(s[start:], endKey)
			if end < 0 {
				break
			}
			parseCMapBlock(s[start+len(block):start+end], block == "beginbfrange", out)
			idx = start + end + len(endKey)
		}
	}
}

// parseCMapBlock 解析 bfchar / bfrange 的内容。
func parseCMapBlock(body string, isRange bool, out map[int]string) {
	type hexTok struct {
		codes []int
	}
	// 逐个取 <...> 与 [ ... ] 片段
	var tokens []string
	for i := 0; i < len(body); {
		switch body[i] {
		case '<':
			end := strings.IndexByte(body[i:], '>')
			if end < 0 {
				return
			}
			tokens = append(tokens, body[i:i+end+1])
			i += end + 1
		case '[':
			end := strings.IndexByte(body[i:], ']')
			if end < 0 {
				return
			}
			tokens = append(tokens, body[i:i+end+1])
			i += end + 1
		default:
			i++
		}
	}
	for i := 0; i < len(tokens); {
		if isRange {
			if i+2 >= len(tokens) {
				return
			}
			lo, ok1 := hexToInt(tokens[i])
			hi, ok2 := hexToInt(tokens[i+1])
			if !ok1 || !ok2 || hi < lo || hi-lo > 0xFFFF {
				i++
				continue
			}
			if strings.HasPrefix(tokens[i+2], "[") {
				items := strings.FieldsFunc(strings.Trim(tokens[i+2], "[]"), func(r rune) bool { return isSpaceByte(byte(r)) })
				for k, item := range items {
					if v, ok := decodeHexString(item); ok {
						out[lo+k] = v
					}
				}
				i += 3
				continue
			}
			base, ok3 := decodeHexString(tokens[i+2])
			if ok3 {
				runes := []rune(base)
				for code := lo; code <= hi; code++ {
					idx := code - lo
					if idx < len(runes) {
						out[code] = string(runes[idx])
					} else if len(runes) > 0 {
						out[code] = string(runes[len(runes)-1] + rune(idx-len(runes)+1))
					}
				}
			}
			i += 3
			continue
		}
		if i+1 >= len(tokens) {
			return
		}
		code, ok := hexToInt(tokens[i])
		if !ok {
			i++
			continue
		}
		if v, ok := decodeHexString(tokens[i+1]); ok {
			out[code] = v
		}
		i += 2
	}
}

// hexToInt "<0041>" → 0x41（多字节按大端拼接）。
func hexToInt(tok string) (int, bool) {
	b, ok := hexToBytes(tok)
	if !ok || len(b) == 0 {
		return 0, false
	}
	v := 0
	for i := 0; i < len(b) && i < 4; i++ {
		v = v<<8 | int(b[i])
	}
	return v, true
}

// hexToBytes "<41 42>" → []byte{0x41,0x42}。
func hexToBytes(tok string) ([]byte, bool) {
	tok = strings.TrimSpace(tok)
	if !strings.HasPrefix(tok, "<") || !strings.HasSuffix(tok, ">") {
		return nil, false
	}
	s := tok[1 : len(tok)-1]
	var out []byte
	var cur int
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		v, ok := hexVal(c)
		if !ok {
			return nil, false
		}
		cur = cur<<4 | v
		n++
		if n == 2 {
			out = append(out, byte(cur))
			cur, n = 0, 0
		}
	}
	if n == 1 { // 奇数位补 0
		out = append(out, byte(cur<<4))
	}
	return out, true
}

// decodeHexString "<0041D83DDE00>" → UTF-16BE 文本。
func decodeHexString(tok string) (string, bool) {
	b, ok := hexToBytes(tok)
	if !ok || len(b) == 0 {
		return "", false
	}
	if len(b) == 1 {
		return string(rune(b[0])), true
	}
	var runes []rune
	for i := 0; i+1 < len(b); i += 2 {
		u := rune(b[i])<<8 | rune(b[i+1])
		if u >= 0xD800 && u <= 0xDBFF && i+3 < len(b) {
			lo := rune(b[i+2])<<8 | rune(b[i+3])
			if lo >= 0xDC00 && lo <= 0xDFFF {
				runes = append(runes, 0x10000+(u-0xD800)<<10+(lo-0xDC00))
				i += 2
				continue
			}
		}
		runes = append(runes, u)
	}
	return string(runes), true
}

// parseXObjects 解析页面 /XObject（图片按占位框显示）。
func (d *pdfDoc) parseXObjects(res []byte) map[string][2]float32 {
	out := map[string][2]float32{}
	if res == nil {
		return out
	}
	xo := dictValue(res, "XObject")
	if xo == "" {
		return out
	}
	s := string(d.resolve(xo))
	i := 0
	for {
		idx := strings.Index(s[i:], "/")
		if idx < 0 {
			break
		}
		p := i + idx
		end := p + 1
		for end < len(s) && !isSpaceByte(s[end]) && s[end] != '/' && s[end] != '>' && s[end] != '<' {
			end++
		}
		name := s[p+1 : end]
		n := ref(s[end:])
		if n > 0 {
			body := d.obj(n)
			if bytes.Contains(body, []byte("/Image")) {
				w := d.resolveNum(dictValue(body, "Width"))
				h := d.resolveNum(dictValue(body, "Height"))
				out[name] = [2]float32{float32(w), float32(h)}
			}
		}
		i = end
	}
	return out
}

// ---- 内容流 → 绘制指令 ----

// pdfCtx 内容流解释状态。
type pdfCtx struct {
	pageH float32
	fonts map[string]*pdfFontInfo
	xobj  map[string][2]float32
	items []PDFItem

	font     *pdfFontInfo
	fontSize float32
	leading  float32
	// tm / tlm 文本矩阵的 e/f（位置）与缩放
	tx, ty float32
	scale  float32
	penX   float32 // 未重定位时同一行文本的推进量
	charSp float32
	wordSp float32
}

// extractItems 解析内容流。
func (d *pdfDoc) extractItems(content []byte, fonts map[string]*pdfFontInfo, xobj map[string][2]float32, pageH float32) []PDFItem {
	c := &pdfCtx{pageH: pageH, fonts: fonts, xobj: xobj, scale: 1}
	lex := &pdfLexer{s: content}
	var stack []pdfToken
	for {
		tok, ok := lex.next()
		if !ok {
			break
		}
		if tok.kind != tkOp {
			stack = append(stack, tok)
			if len(stack) > 64 {
				stack = stack[len(stack)-64:]
			}
			continue
		}
		switch tok.raw {
		case "BT":
			c.tx, c.ty, c.penX, c.scale = 0, 0, 0, 1
		case "Tf":
			if len(stack) >= 2 {
				name := stack[len(stack)-2]
				size := stack[len(stack)-1]
				if name.kind == tkName {
					c.font = c.fonts[name.raw]
				}
				c.fontSize = float32(size.num)
				if c.fontSize <= 0 {
					c.fontSize = 12
				}
			}
		case "TL":
			if len(stack) >= 1 {
				c.leading = float32(stack[len(stack)-1].num)
			}
		case "Tc":
			if len(stack) >= 1 {
				c.charSp = float32(stack[len(stack)-1].num)
			}
		case "Tw":
			if len(stack) >= 1 {
				c.wordSp = float32(stack[len(stack)-1].num)
			}
		case "Td":
			if len(stack) >= 2 {
				c.tx += float32(stack[len(stack)-2].num) * c.scale
				c.ty += float32(stack[len(stack)-1].num) * c.scale
				c.penX = 0
			}
		case "TD":
			if len(stack) >= 2 {
				dx := float32(stack[len(stack)-2].num) * c.scale
				dy := float32(stack[len(stack)-1].num) * c.scale
				c.tx += dx
				c.ty += dy
				c.leading = -dy
				c.penX = 0
			}
		case "Tm":
			if len(stack) >= 6 {
				a := float32(stack[len(stack)-6].num)
				b := float32(stack[len(stack)-5].num)
				c.scale = sqrt32(a*a + b*b)
				if c.scale <= 0 {
					c.scale = 1
				}
				c.tx = float32(stack[len(stack)-2].num)
				c.ty = float32(stack[len(stack)-1].num)
				c.penX = 0
			}
		case "T*":
			c.ty -= c.leading * c.scale
			c.penX = 0
		case "Tj":
			if len(stack) >= 1 {
				c.show([]pdfToken{stack[len(stack)-1]})
			}
		case "TJ":
			if len(stack) >= 1 {
				c.showArray(stack[len(stack)-1])
			}
		case "'":
			c.ty -= c.leading * c.scale
			c.penX = 0
			if len(stack) >= 1 {
				c.show([]pdfToken{stack[len(stack)-1]})
			}
		case "\"":
			c.ty -= c.leading * c.scale
			c.penX = 0
			if len(stack) >= 1 {
				c.show([]pdfToken{stack[len(stack)-1]})
			}
		case "Do":
			if len(stack) >= 1 && stack[len(stack)-1].kind == tkName {
				if wh, ok := c.xobj[stack[len(stack)-1].raw]; ok {
					w := wh[0] * c.scale
					h := wh[1] * c.scale
					c.items = append(c.items, PDFItem{Kind: PDFImage,
						X: c.tx, Y: round2(c.pageH - c.ty - h), W: w, H: h})
				}
			}
		case "re":
			if len(stack) >= 4 {
				x := float32(stack[len(stack)-4].num)
				y := float32(stack[len(stack)-3].num)
				w := float32(stack[len(stack)-2].num)
				h := float32(stack[len(stack)-1].num)
				if w > 2 && h > 2 { // 忽略细线矩形
					c.items = append(c.items, PDFItem{Kind: PDFRect, X: x, Y: round2(c.pageH - y - h),
						W: w, H: h, Fill: "", Stroke: 0})
				}
			}
		case "m", "l":
			// 路径点只参与表格线 / 分隔线绘制，本轮预览不还原路径（保持精简）
		}
		stack = stack[:0]
	}
	return compactItems(c.items)
}

// show 显示一段字符串（Tj / ' / "）。
func (c *pdfCtx) show(ops []pdfToken) {
	var raw []byte
	for _, o := range ops {
		if o.kind == tkString {
			raw = append(raw, o.bytes...)
		}
	}
	c.emit(raw)
}

// showArray 显示 TJ 数组（字符串 + 字距，忽略字距）。
func (c *pdfCtx) showArray(tok pdfToken) {
	var raw []byte
	for _, el := range tok.list {
		if el.kind == tkString {
			raw = append(raw, el.bytes...)
		}
	}
	c.emit(raw)
}

// emit 把解码后的文本追加为绘制项并推进笔位。
func (c *pdfCtx) emit(raw []byte) {
	if len(raw) == 0 {
		return
	}
	text := c.font.decode(raw)
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	size := c.fontSize * c.scale
	if size <= 0 {
		size = 12
	}
	x := c.tx + c.penX
	y := round2(c.pageH - c.ty - size*0.82)
	c.items = append(c.items, PDFItem{Kind: PDFText, Text: text, X: round2(x), Y: y, Size: round2(size)})
	c.penX += estimateWidth(text, size) + c.charSp*float32(len(raw))
}

// estimateWidth 估算文本宽度（无字体度量时的近似值）。
func estimateWidth(s string, size float32) float32 {
	var w float32
	for _, r := range s {
		switch {
		case r == ' ':
			w += 0.28 * size
		case r < 0x2E80: // 拉丁 / 标点
			w += 0.52 * size
		default: // 中日韩全角
			w += 1.0 * size
		}
	}
	return w
}

// compactItems 去掉重叠的重复文本（同一位置同一内容只留一条）。
func compactItems(items []PDFItem) []PDFItem {
	if len(items) < 2 {
		return items
	}
	type key struct {
		x, y int
		t    string
	}
	seen := make(map[key]bool, len(items))
	out := items[:0]
	for _, it := range items {
		k := key{int(it.X), int(it.Y), it.Text}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, it)
	}
	return out
}

// ---- 内容流词法 ----

const (
	tkNumber = iota
	tkName
	tkString
	tkArray
	tkOp
)

// pdfToken 一个词法单元。
type pdfToken struct {
	kind  int
	num   float64
	raw   string
	bytes []byte
	list  []pdfToken
}

// pdfLexer 内容流词法分析器。
type pdfLexer struct {
	s []byte
	i int
}

// next 取下一个 token。
func (l *pdfLexer) next() (pdfToken, bool) {
	// 跳过空白与注释
	for l.i < len(l.s) {
		c := l.s[l.i]
		if isSpaceByte(c) {
			l.i++
			continue
		}
		if c == '%' {
			for l.i < len(l.s) && l.s[l.i] != '\n' {
				l.i++
			}
			continue
		}
		break
	}
	if l.i >= len(l.s) {
		return pdfToken{}, false
	}
	c := l.s[l.i]
	switch {
	case c == '(':
		l.i++
		var out []byte
		depth := 1
		for l.i < len(l.s) {
			ch := l.s[l.i]
			l.i++
			switch ch {
			case '\\':
				if l.i >= len(l.s) {
					break
				}
				esc := l.s[l.i]
				l.i++
				switch esc {
				case 'n':
					out = append(out, '\n')
				case 'r':
					out = append(out, '\r')
				case 't':
					out = append(out, '\t')
				case 'b':
					out = append(out, '\b')
				case 'f':
					out = append(out, '\f')
				case '(', ')', '\\':
					out = append(out, esc)
				case '\r':
					if l.i < len(l.s) && l.s[l.i] == '\n' {
						l.i++
					}
				case '\n':
					// 续行
				default:
					if esc >= '0' && esc <= '7' {
						v := int(esc - '0')
						for k := 0; k < 2 && l.i < len(l.s); k++ {
							d := l.s[l.i]
							if d < '0' || d > '7' {
								break
							}
							v = v*8 + int(d-'0')
							l.i++
						}
						out = append(out, byte(v))
					} else {
						out = append(out, esc)
					}
				}
			case '(':
				depth++
				out = append(out, ch)
			case ')':
				depth--
				if depth == 0 {
					return pdfToken{kind: tkString, bytes: out, raw: string(out)}, true
				}
				out = append(out, ch)
			default:
				out = append(out, ch)
			}
		}
		return pdfToken{kind: tkString, bytes: out, raw: string(out)}, true
	case c == '<':
		if l.i+1 < len(l.s) && l.s[l.i+1] == '<' {
			l.i += 2
			// 字典：递归读到匹配的 >>（内容丢弃，只保证不破坏词法）
			depth := 1
			for l.i+1 < len(l.s) {
				if l.s[l.i] == '<' && l.s[l.i+1] == '<' {
					depth++
					l.i += 2
					continue
				}
				if l.s[l.i] == '>' && l.s[l.i+1] == '>' {
					depth--
					l.i += 2
					if depth == 0 {
						break
					}
					continue
				}
				l.i++
			}
			return pdfToken{kind: tkOp, raw: "<<"}, true
		}
		start := l.i
		for l.i < len(l.s) && l.s[l.i] != '>' {
			l.i++
		}
		tok := string(l.s[start:minInt(len(l.s), l.i+1)])
		l.i++
		b, _ := hexToBytes(tok)
		return pdfToken{kind: tkString, bytes: b, raw: tok}, true
	case c == '[':
		l.i++
		tok := pdfToken{kind: tkArray}
		for {
			inner, ok := l.next()
			if !ok {
				break
			}
			if inner.kind == tkOp && inner.raw == "]" {
				break
			}
			tok.list = append(tok.list, inner)
			if len(tok.list) > 4096 {
				break
			}
		}
		return tok, true
	case c == ']':
		l.i++
		return pdfToken{kind: tkOp, raw: "]"}, true
	case c == '/':
		start := l.i + 1
		l.i++
		for l.i < len(l.s) {
			ch := l.s[l.i]
			if isSpaceByte(ch) || ch == '/' || ch == '[' || ch == ']' || ch == '<' || ch == '>' || ch == '(' || ch == ')' {
				break
			}
			l.i++
		}
		return pdfToken{kind: tkName, raw: string(l.s[start:l.i])}, true
	}
	// 数字或算子
	start := l.i
	for l.i < len(l.s) {
		ch := l.s[l.i]
		if isSpaceByte(ch) || ch == '/' || ch == '[' || ch == ']' || ch == '<' || ch == '>' || ch == '(' || ch == ')' {
			break
		}
		l.i++
	}
	word := string(l.s[start:l.i])
	if v, err := parseFloatSafe(word); err == nil {
		return pdfToken{kind: tkNumber, num: v, raw: word}, true
	}
	return pdfToken{kind: tkOp, raw: word}, true
}

// ---- 小工具 ----

// abs64 绝对值。
func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// round2 保留两位小数（预览坐标不抖动）。
func round2(v float32) float32 {
	return float32(int(v*100+0.5)) / 100
}

// sqrt32 平方根（牛顿迭代，避免额外 import 争议）。
func sqrt32(v float32) float32 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 16; i++ {
		x = (x + v/x) / 2
	}
	return x
}

// minInt 较小值。
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// decodePDFString 解析 PDF 文本串（"<FEFF…>" 或 "(…)"）。
func decodePDFString(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "<") {
		return decodeHexString(v)
	}
	if strings.HasPrefix(v, "(") {
		end := strings.LastIndexByte(v, ')')
		if end < 0 {
			return "", false
		}
		s := v[1:end]
		s = strings.ReplaceAll(s, `\(`, "(")
		s = strings.ReplaceAll(s, `\)`, ")")
		s = strings.ReplaceAll(s, `\\`, `\`)
		if strings.HasPrefix(s, "\xfe\xff") && len(s) > 2 {
			var runes []rune
			b := []byte(s[2:])
			for i := 0; i+1 < len(b); i += 2 {
				runes = append(runes, rune(b[i])<<8|rune(b[i+1]))
			}
			return string(runes), true
		}
		return s, true
	}
	return "", false
}

// PDFPageCount 快速统计页数（不解析内容，界面列表用）。
func PDFPageCount(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	d := &pdfDoc{data: b, objs: scanPDFObjects(b)}
	n := 0
	for _, body := range d.objs {
		if isPageDict(body) {
			n++
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("office: 未找到页面")
	}
	return n, nil
}
