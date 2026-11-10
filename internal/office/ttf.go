// guis — TrueType 字体子集化（PDF 嵌入用）
//
// 目的：把系统里的大字体（微软雅黑 msyh.ttc 约 19MB、思源黑体更大）裁成
// 「只含本文档用到的字符」的小字体，几百字通常 20~60KB，直接塞进 PDF。
//
// 做法（足够 PDF 的 CIDFontType2 使用，不追求通用字体工具的能力）：
//
//	① 解析 sfnt 目录（含 TTC 集合头 ttcf），取出 head/hhea/maxp/hmtx/loca/glyf 等表；
//	② 按 cmap（format 4 / 12）把 rune 映射成原字体 glyph id；
//	③ 复合字形递归展开引用，得到最小 glyph 集合；
//	④ 重排 glyph 顺序：0=.notdef，其后按 CID 顺序排列（这样 CIDToGIDMap 可用 /Identity）；
//	⑤ 重写 glyf（复合字形内的组件 id 同步改写）、loca（long 格式）、hmtx、head/maxp 计数；
//	⑥ 重算表校验和与 head.checkSumAdjustment。
//
// CFF（OTTO）轮廓字体不支持子集化，调用方需回退到内置 Helvetica。

package office

import (
	"encoding/binary"
	"errors"
	"sort"
)

// errNotTrueType 非 TrueType 轮廓（如 CFF/OTTO），无法子集化。
var errNotTrueType = errors.New("office: 该字体不是 TrueType 轮廓（CFF/OTTO 不支持子集化）")

// trueTypeFont 已解析的字体（只保留子集化需要的表）。
type trueTypeFont struct {
	data       []byte
	tables     map[string][]byte
	unitsPerEm int
	numGlyphs  int
	numHMetric int
	locaLong   bool
	loca       []uint32 // 长度 numGlyphs+1
	glyf       []byte
	hmtx       []byte
	advances   []uint16
	ascent     int16
	descent    int16
	bbox       [4]int16
	cmap       map[rune]uint16
	psName     string
}

// be16 / be32 / put16 / put32 大端读写。
func be16(b []byte) uint16     { return binary.BigEndian.Uint16(b) }
func be32(b []byte) uint32     { return binary.BigEndian.Uint32(b) }
func put16(b []byte, v uint16) { binary.BigEndian.PutUint16(b, v) }
func put32(b []byte, v uint32) { binary.BigEndian.PutUint32(b, v) }

// parseTrueType 解析字体文件（支持 .ttf / .ttc，取第 faceIndex 个 face）。
func parseTrueType(data []byte, faceIndex int) (*trueTypeFont, error) {
	if len(data) < 12 {
		return nil, errors.New("office: 字体文件太小")
	}
	base := 0
	if string(data[:4]) == "ttcf" {
		n := int(be32(data[8:]))
		if n == 0 {
			return nil, errors.New("office: 字体集合为空")
		}
		off := 12 + faceIndex*4
		if faceIndex < 0 || off+4 > len(data) {
			faceIndex = 0
			off = 12
		}
		base = int(be32(data[off:]))
		if base+12 > len(data) {
			return nil, errors.New("office: 字体集合偏移非法")
		}
	}
	tag := be32(data[base:])
	switch tag {
	case 0x00010000, 0x74727565: // TrueType
	case 0x4F54544F: // 'OTTO'
		return nil, errNotTrueType
	default:
		return nil, errors.New("office: 无法识别的字体格式")
	}
	numTables := int(be16(data[base+4:]))
	if numTables <= 0 || numTables > 512 {
		return nil, errors.New("office: 字体表数量异常")
	}
	f := &trueTypeFont{data: data, tables: make(map[string][]byte, numTables), bbox: [4]int16{}}
	for i := 0; i < numTables; i++ {
		p := base + 12 + i*16
		if p+16 > len(data) {
			return nil, errors.New("office: 字体表目录越界")
		}
		name := string(data[p : p+4])
		off := int(be32(data[p+8:]))
		length := int(be32(data[p+12:]))
		if off < 0 || length < 0 || off+length > len(data) {
			continue
		}
		f.tables[name] = data[off : off+length]
	}
	if err := f.parseHead(); err != nil {
		return nil, err
	}
	if err := f.parseMetrics(); err != nil {
		return nil, err
	}
	f.parseName()
	f.parseCMap()
	return f, nil
}

// parseHead 读 head（unitsPerEm / indexToLocFormat / 包围盒）。
func (f *trueTypeFont) parseHead() error {
	head, ok := f.tables["head"]
	if !ok || len(head) < 54 {
		return errors.New("office: 字体缺少 head 表")
	}
	f.unitsPerEm = int(be16(head[18:]))
	if f.unitsPerEm <= 0 {
		f.unitsPerEm = 1000
	}
	f.locaLong = int16(be16(head[50:])) != 0
	f.bbox = [4]int16{int16(be16(head[36:])), int16(be16(head[38:])),
		int16(be16(head[40:])), int16(be16(head[42:]))}
	maxp, ok := f.tables["maxp"]
	if !ok || len(maxp) < 6 {
		return errors.New("office: 字体缺少 maxp 表")
	}
	f.numGlyphs = int(be16(maxp[4:]))
	if f.numGlyphs <= 0 {
		return errors.New("office: 字体 glyph 数为 0")
	}
	return nil
}

// parseMetrics 读 hhea / hmtx / loca / glyf。
func (f *trueTypeFont) parseMetrics() error {
	hhea, ok := f.tables["hhea"]
	if !ok || len(hhea) < 36 {
		return errors.New("office: 字体缺少 hhea 表")
	}
	f.ascent = int16(be16(hhea[4:]))
	f.descent = int16(be16(hhea[6:]))
	f.numHMetric = int(be16(hhea[34:]))
	if f.numHMetric <= 0 || f.numHMetric > f.numGlyphs {
		f.numHMetric = f.numGlyphs
	}
	f.glyf = f.tables["glyf"]
	loca, ok := f.tables["loca"]
	if !ok {
		return errors.New("office: 字体缺少 loca 表")
	}
	f.loca = make([]uint32, f.numGlyphs+1)
	if f.locaLong {
		if len(loca) < (f.numGlyphs+1)*4 {
			return errors.New("office: loca 表长度不足")
		}
		for i := 0; i <= f.numGlyphs; i++ {
			f.loca[i] = be32(loca[i*4:])
		}
	} else {
		if len(loca) < (f.numGlyphs+1)*2 {
			return errors.New("office: loca 表长度不足")
		}
		for i := 0; i <= f.numGlyphs; i++ {
			f.loca[i] = uint32(be16(loca[i*2:])) * 2
		}
	}
	f.hmtx = f.tables["hmtx"]
	f.advances = make([]uint16, f.numGlyphs)
	for i := 0; i < f.numGlyphs; i++ {
		if i < f.numHMetric {
			p := i * 4
			if p+2 > len(f.hmtx) {
				break
			}
			f.advances[i] = be16(f.hmtx[p:])
		} else if f.numHMetric > 0 {
			f.advances[i] = f.advances[f.numHMetric-1]
		}
	}
	return nil
}

// parseName 取 PostScript 名（nameID=6），失败留空。
func (f *trueTypeFont) parseName() {
	tbl, ok := f.tables["name"]
	if !ok || len(tbl) < 6 {
		return
	}
	count := int(be16(tbl[2:]))
	strOff := int(be16(tbl[4:]))
	for i := 0; i < count; i++ {
		p := 6 + i*12
		if p+12 > len(tbl) {
			return
		}
		platform := be16(tbl[p:])
		nameID := be16(tbl[p+6:])
		length := int(be16(tbl[p+8:]))
		off := int(be16(tbl[p+10:]))
		if nameID != 6 {
			continue
		}
		start := strOff + off
		if start < 0 || start+length > len(tbl) {
			continue
		}
		raw := tbl[start : start+length]
		if platform == 3 || platform == 0 { // UTF-16BE
			var out []rune
			for j := 0; j+1 < len(raw); j += 2 {
				out = append(out, rune(be16(raw[j:])))
			}
			f.psName = string(out)
		} else {
			f.psName = string(raw)
		}
		if f.psName != "" {
			return
		}
	}
}

// parseCMap 解析 cmap（优先 format 12 → 补充平面，回退 format 4 → BMP）。
func (f *trueTypeFont) parseCMap() {
	f.cmap = make(map[rune]uint16, 4096)
	tbl, ok := f.tables["cmap"]
	if !ok || len(tbl) < 4 {
		return
	}
	n := int(be16(tbl[2:]))
	var best4, best12 int = -1, -1
	for i := 0; i < n; i++ {
		p := 4 + i*8
		if p+8 > len(tbl) {
			break
		}
		platform := be16(tbl[p:])
		encoding := be16(tbl[p+2:])
		off := int(be32(tbl[p+4:]))
		if off < 0 || off+2 > len(tbl) {
			continue
		}
		format := be16(tbl[off:])
		unicode := platform == 0 || (platform == 3 && (encoding == 1 || encoding == 10))
		if !unicode {
			continue
		}
		switch format {
		case 12:
			if best12 < 0 {
				best12 = off
			}
		case 4:
			if best4 < 0 {
				best4 = off
			}
		}
	}
	if best12 >= 0 {
		f.parseCMap12(tbl[best12:])
	}
	if best4 >= 0 {
		f.parseCMap4(tbl[best4:])
	}
}

// parseCMap4 解析 format 4 子表。
func (f *trueTypeFont) parseCMap4(t []byte) {
	if len(t) < 14 {
		return
	}
	segCount := int(be16(t[6:])) / 2
	if segCount <= 0 || segCount > 16384 {
		return
	}
	endBase := 14
	startBase := endBase + segCount*2 + 2
	deltaBase := startBase + segCount*2
	rangeBase := deltaBase + segCount*2
	if rangeBase+segCount*2 > len(t) {
		return
	}
	for i := 0; i < segCount; i++ {
		end := be16(t[endBase+i*2:])
		start := be16(t[startBase+i*2:])
		delta := int16(be16(t[deltaBase+i*2:]))
		ro := int(be16(t[rangeBase+i*2:]))
		if start > end {
			continue
		}
		for c := int(start); c <= int(end) && c <= 0xFFFF; c++ {
			if c == 0xFFFF {
				break
			}
			var gid uint16
			if ro == 0 {
				gid = uint16(int(c) + int(delta))
			} else {
				p := rangeBase + i*2 + ro + (c-int(start))*2
				if p+2 > len(t) {
					continue
				}
				g := be16(t[p:])
				if g == 0 {
					continue
				}
				gid = uint16(int(g) + int(delta))
			}
			if gid != 0 {
				if _, exists := f.cmap[rune(c)]; !exists {
					f.cmap[rune(c)] = gid
				}
			}
		}
	}
}

// parseCMap12 解析 format 12 子表。
func (f *trueTypeFont) parseCMap12(t []byte) {
	if len(t) < 16 {
		return
	}
	groups := int(be32(t[12:]))
	if groups <= 0 || groups > 1<<20 {
		return
	}
	for i := 0; i < groups; i++ {
		p := 16 + i*12
		if p+12 > len(t) {
			return
		}
		start := rune(be32(t[p:]))
		end := rune(be32(t[p+4:]))
		gid := uint32(be32(t[p+8:]))
		if end < start || end-start > 0x40000 {
			continue
		}
		for c := start; c <= end && c <= 0x10FFFF; c++ {
			if gid+uint32(c-start) > 0xFFFF {
				break
			}
			if _, exists := f.cmap[c]; !exists {
				f.cmap[c] = uint16(gid + uint32(c-start))
			}
		}
	}
}

// glyphData 取 glyph 原始字节（非法返回 nil）。
func (f *trueTypeFont) glyphData(gid int) []byte {
	if gid < 0 || gid+1 >= len(f.loca) {
		return nil
	}
	start, end := f.loca[gid], f.loca[gid+1]
	if start >= end || int(end) > len(f.glyf) {
		return nil
	}
	return f.glyf[start:end]
}

// expandComposite 递归展开复合字形的组件。
func (f *trueTypeFont) expandComposite(gid int, seen map[int]bool) {
	if seen[gid] {
		return
	}
	seen[gid] = true
	data := f.glyphData(gid)
	if len(data) < 10 || int16(be16(data)) >= 0 {
		return // 简单字形
	}
	p := 10
	for p+4 <= len(data) {
		flags := be16(data[p:])
		comp := int(be16(data[p+2:]))
		f.expandComposite(comp, seen)
		p += 4
		if flags&0x0001 != 0 { // ARG_1_AND_2_ARE_WORDS
			p += 4
		} else {
			p += 2
		}
		switch {
		case flags&0x0008 != 0: // WE_HAVE_A_SCALE
			p += 2
		case flags&0x0040 != 0: // X_AND_Y_SCALE
			p += 4
		case flags&0x0080 != 0: // TWO_BY_TWO
			p += 8
		}
		if flags&0x0020 == 0 { // MORE_COMPONENTS
			break
		}
	}
}

// subsetResult 子集化结果。
type subsetResult struct {
	// Font 子集字体字节（无可用字形时为 nil）。
	Font []byte
	// CIDs rune → CID（CID 即子集字体内的 glyph id，缺失为 0）。
	CIDs map[rune]uint16
	// Widths 按 CID 索引的字宽（1/1000 em）。
	Widths []uint16
	// NumGlyphs 子集字形数。
	NumGlyphs int
	// Ascent/Descent/CapHeight 以 1/1000 em 表示（PDF FontDescriptor 用）。
	Ascent, Descent, CapHeight int16
	// BBox 0..1000 em 包围盒。
	BBox [4]int16
	// PSName PostScript 名。
	PSName string
}

// subset 按 rune 集合子集化字体。CID = 子集 glyph id（0 为 .notdef），
// 因此 PDF 里 CIDToGIDMap 用 /Identity 即可。
func (f *trueTypeFont) subset(runes []rune) *subsetResult {
	res := &subsetResult{CIDs: map[rune]uint16{}, PSName: f.psName}
	scale := func(v int) int16 {
		if f.unitsPerEm == 0 {
			return 0
		}
		return int16(int(v) * 1000 / f.unitsPerEm)
	}
	res.Ascent = scale(int(f.ascent))
	res.Descent = scale(int(f.descent))
	res.CapHeight = int16(int(res.Ascent) * 7 / 10)
	res.BBox = [4]int16{scale(int(f.bbox[0])), scale(int(f.bbox[1])),
		scale(int(f.bbox[2])), scale(int(f.bbox[3]))}

	// 1) rune → 原字体 gid（去重、按 rune 排序保证输出稳定）
	uniq := make([]rune, 0, len(runes))
	seenRune := map[rune]bool{}
	for _, r := range runes {
		if r == 0 || seenRune[r] {
			continue
		}
		seenRune[r] = true
		if _, ok := f.cmap[r]; ok {
			uniq = append(uniq, r)
		}
	}
	sort.Slice(uniq, func(i, j int) bool { return uniq[i] < uniq[j] })
	if len(uniq) == 0 {
		return res
	}

	// 2) 展开复合字形依赖
	need := map[int]bool{0: true}
	order := []int{0}
	for _, r := range uniq {
		gid := int(f.cmap[r])
		if need[gid] {
			continue
		}
		before := len(need)
		f.expandComposite(gid, need)
		// 新增的依赖字形也要有稳定的顺序（按 gid 升序补进 order）
		added := make([]int, 0, len(need)-before)
		for g := range need {
			if !containsInt(order, g) {
				added = append(added, g)
			}
		}
		sort.Ints(added)
		order = append(order, added...)
	}
	// 让主字形仍按 rune 顺序排在前面：重建顺序（0 + 每个 rune 的 gid + 其依赖）
	order = order[:0]
	placed := map[int]bool{0: true}
	order = append(order, 0)
	for _, r := range uniq {
		gid := int(f.cmap[r])
		if !placed[gid] {
			placed[gid] = true
			order = append(order, gid)
		}
	}
	// 依赖字形追加到末尾（不参与 CID 编址）
	deps := make([]int, 0, len(need))
	for g := range need {
		if !placed[g] {
			deps = append(deps, g)
		}
	}
	sort.Ints(deps)
	order = append(order, deps...)

	// 3) 重排索引：新 gid = 下标
	newGID := make(map[int]int, len(order))
	for i, g := range order {
		newGID[g] = i
	}

	// 4) 生成 glyf / loca
	glyf := make([]byte, 0, len(f.glyf)/8+1024)
	loca := make([]uint32, len(order)+1)
	for i, g := range order {
		loca[i] = uint32(len(glyf))
		raw := f.glyphData(g)
		if len(raw) == 0 {
			continue
		}
		buf := make([]byte, len(raw))
		copy(buf, raw)
		if len(buf) >= 10 && int16(be16(buf)) < 0 {
			patchComposite(buf, newGID)
		}
		glyf = append(glyf, buf...)
		// 每个 glyph 4 字节对齐（部分校验器要求）
		for len(glyf)%4 != 0 {
			glyf = append(glyf, 0)
		}
	}
	loca[len(order)] = uint32(len(glyf))
	if len(glyf) == 0 {
		return res
	}

	// 5) hmtx
	hmtx := make([]byte, 0, len(order)*4)
	res.Widths = make([]uint16, len(order))
	for i, g := range order {
		adv := uint16(0)
		if g < len(f.advances) {
			adv = f.advances[g]
		}
		w := uint16(0)
		if f.unitsPerEm > 0 {
			w = uint16(int(adv) * 1000 / f.unitsPerEm)
		}
		res.Widths[i] = w
		lsb := int16(0)
		if g < f.numHMetric {
			p := g*4 + 2
			if p+2 <= len(f.hmtx) {
				lsb = int16(be16(f.hmtx[p:]))
			}
		}
		buf := make([]byte, 4)
		put16(buf, adv)
		put16(buf[2:], uint16(lsb))
		hmtx = append(hmtx, buf...)
	}

	// 6) head / hhea / maxp 改写
	head := append([]byte(nil), f.tables["head"]...)
	if len(head) < 54 {
		return res
	}
	put32(head[8:], 0)  // checkSumAdjustment（最后重算）
	put16(head[50:], 1) // indexToLocFormat = long

	hhea := append([]byte(nil), f.tables["hhea"]...)
	if len(hhea) >= 36 {
		put16(hhea[34:], uint16(len(order))) // numberOfHMetrics
	}

	maxp := append([]byte(nil), f.tables["maxp"]...)
	if len(maxp) >= 6 {
		put16(maxp[4:], uint16(len(order)))
	}

	// 7) 组装 sfnt
	tables := map[string][]byte{
		"head": head,
		"hhea": hhea,
		"maxp": maxp,
		"hmtx": hmtx,
		"loca": locaTable(loca),
		"glyf": glyf,
	}
	// 可选表：保留影响栅格化的提示表；name/OS/2 体积小也留着
	for _, k := range []string{"cvt ", "fpgm", "prep", "name", "OS/2", "gasp"} {
		if t, ok := f.tables[k]; ok && len(t) > 0 {
			tables[k] = t
		}
	}
	// post 表若含字形名（version 2.0）会与子集字形数冲突 → 改写为 3.0（无名字）
	if post, ok := f.tables["post"]; ok && len(post) >= 32 {
		p3 := make([]byte, 32)
		copy(p3, post[:32])
		put32(p3, 0x00030000)
		tables["post"] = p3
	}
	res.Font = buildSFNT(tables)
	res.NumGlyphs = len(order)

	for i, r := range uniq {
		gid := int(f.cmap[r])
		if ng, ok := newGID[gid]; ok && ng < 0xFFFF {
			res.CIDs[r] = uint16(ng)
			_ = i
		}
	}
	return res
}

// patchComposite 把复合字形里的组件 id 改写成新 gid。
func patchComposite(data []byte, newGID map[int]int) {
	p := 10
	for p+4 <= len(data) {
		flags := be16(data[p:])
		comp := int(be16(data[p+2:]))
		if ng, ok := newGID[comp]; ok && ng <= 0xFFFF {
			put16(data[p+2:], uint16(ng))
		}
		p += 4
		if flags&0x0001 != 0 {
			p += 4
		} else {
			p += 2
		}
		switch {
		case flags&0x0008 != 0:
			p += 2
		case flags&0x0040 != 0:
			p += 4
		case flags&0x0080 != 0:
			p += 8
		}
		if flags&0x0020 == 0 {
			break
		}
	}
}

// locaTable long 格式 loca 表。
func locaTable(loca []uint32) []byte {
	b := make([]byte, len(loca)*4)
	for i, v := range loca {
		put32(b[i*4:], v)
	}
	return b
}

// buildSFNT 按 sfnt 规范拼装字体文件（含表校验和与 head.checkSumAdjustment）。
func buildSFNT(tables map[string][]byte) []byte {
	type entry struct {
		tag   string
		data  []byte
		check uint32
		off   int
	}
	// 按 tag 升序排列（规范要求）
	names := make([]string, 0, len(tables))
	for k := range tables {
		names = append(names, k)
	}
	sort.Strings(names)
	entries := make([]entry, 0, len(names))
	for _, n := range names {
		d := tables[n]
		entries = append(entries, entry{tag: n, data: d, check: tableChecksum(d)})
	}

	numTables := len(entries)
	headerSize := 12 + numTables*16
	// 计算 searchRange / entrySelector / rangeShift
	entrySelector := 0
	for 1<<(entrySelector+1) <= numTables {
		entrySelector++
	}
	searchRange := 1 << entrySelector * 16
	rangeShift := numTables*16 - searchRange

	out := make([]byte, headerSize)
	put32(out[0:], 0x00010000)
	put16(out[4:], uint16(numTables))
	put16(out[6:], uint16(searchRange))
	put16(out[8:], uint16(entrySelector))
	put16(out[10:], uint16(rangeShift))

	offset := headerSize
	// 表数据按 4 字节对齐
	body := make([]byte, 0, 4096)
	for i := range entries {
		entries[i].off = offset
		put32(out[12+i*16:], tagToUint32(entries[i].tag))
		put32(out[12+i*16+4:], entries[i].check)
		put32(out[12+i*16+8:], uint32(offset))
		put32(out[12+i*16+12:], uint32(len(entries[i].data)))
		body = append(body, entries[i].data...)
		for (offset+len(body))%4 != 0 {
			body = append(body, 0)
		}
		offset = headerSize + len(body)
	}
	font := append(out, body...)
	// checkSumAdjustment：整个文件求和后取反
	findHead := -1
	for i := range entries {
		if entries[i].tag == "head" {
			findHead = entries[i].off
			break
		}
	}
	if findHead >= 0 && findHead+12 <= len(font) {
		put32(font[findHead+8:], 0)
		sum := tableChecksum(font)
		put32(font[findHead+8:], 0xB1B0AFBA-sum)
	}
	return font
}

// tableChecksum 表校验和（按 4 字节大端累加）。
func tableChecksum(data []byte) uint32 {
	var sum uint32
	n := len(data)
	i := 0
	for ; i+4 <= n; i += 4 {
		sum += be32(data[i:])
	}
	if i < n {
		var tail [4]byte
		copy(tail[:], data[i:])
		sum += be32(tail[:])
	}
	return sum
}

// tagToUint32 4 字符 tag → uint32。
func tagToUint32(tag string) uint32 {
	var v uint32
	for i := 0; i < 4; i++ {
		var c byte = ' '
		if i < len(tag) {
			c = tag[i]
		}
		v = v<<8 | uint32(c)
	}
	return v
}

// containsInt 判断切片是否含某值。
func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
