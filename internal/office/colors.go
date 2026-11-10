// guis — 颜色 / 字体定位等小工具

package office

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// parseHexColor "#2563eb" / "#fff" / "2563eb" → (0~1 的 r,g,b)；非法返回黑色。
func parseHexColor(s string) (float32, float32, float32) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 8: // AARRGGBB → 取后 6 位
		s = s[2:]
	}
	if len(s) != 6 {
		return 0, 0, 0
	}
	var v [3]int
	for i := 0; i < 3; i++ {
		hi, ok1 := hexVal(s[i*2])
		lo, ok2 := hexVal(s[i*2+1])
		if !ok1 || !ok2 {
			return 0, 0, 0
		}
		v[i] = hi*16 + lo
	}
	return float32(v[0]) / 255, float32(v[1]) / 255, float32(v[2]) / 255
}

// hexVal 十六进制字符 → 数值。
func hexVal(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

// HexColor 把 0~1 的 rgb 转回 "#rrggbb"（渲染端统一色值格式）。
func HexColor(r, g, b float32) string {
	clamp := func(v float32) int {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 255
		}
		return int(v*255 + 0.5)
	}
	const hexDigits = "0123456789abcdef"
	rr, gg, bb := clamp(r), clamp(g), clamp(b)
	return string([]byte{'#', hexDigits[rr>>4], hexDigits[rr&15],
		hexDigits[gg>>4], hexDigits[gg&15], hexDigits[bb>>4], hexDigits[bb&15]})
}

// pdfFontCandidates 各平台可能的 TrueType（glyf 轮廓）中文字体。
// 注意：必须排除纯 CFF/OTTO 的字体（如 NotoSansCJK-Regular.ttc），
// 它们无法用本包的子集化器处理。
func pdfFontCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		root := os.Getenv("WINDIR")
		if root == "" {
			root = `C:\Windows`
		}
		f := filepath.Join(root, "Fonts")
		return []string{
			filepath.Join(f, "msyh.ttc"),   // 微软雅黑
			filepath.Join(f, "msyhbd.ttc"), // 微软雅黑 Bold
			filepath.Join(f, "Deng.ttf"),   // 等线
			filepath.Join(f, "simhei.ttf"), // 黑体
			filepath.Join(f, "simsun.ttc"), // 宋体
			filepath.Join(f, "simkai.ttf"), // 楷体
			filepath.Join(f, "msjh.ttc"),   // 微软正黑（繁中）
			filepath.Join(f, "arial.ttf"),
		}
	case "darwin":
		return []string{
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/STHeiti Light.ttc",
			"/System/Library/Fonts/Supplemental/Songti.ttc",
			"/Library/Fonts/Arial Unicode.ttf",
			"/System/Library/Fonts/Supplemental/Arial.ttf",
		}
	default:
		return []string{
			"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
			"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
			"/usr/share/fonts/truetype/arphic/uming.ttc",
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/System/Library/Fonts/PingFang.ttc",
		}
	}
}

// pdfFontFile 选字体文件：显式指定优先，其次系统候选表里第一个存在的。
// faceIndex 对 .ttc 有效（默认 0）；.ttf 恒为 0。
func pdfFontFile(explicit string, faceIndex int) (string, int) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, faceIndex
		}
	}
	if v := strings.TrimSpace(os.Getenv("GUIS_PDF_FONT")); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v, faceIndex
		}
	}
	for _, p := range pdfFontCandidates() {
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return p, faceIndex
		}
	}
	return "", 0
}
