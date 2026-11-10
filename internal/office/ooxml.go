// guis — OOXML 公共工具（xlsx / docx 共用的 zip 与 XML 小工具）

package office

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

// readZip 读整个 zip 包为 文件名 → 内容 的映射（路径统一用 '/' 分隔）。
func readZip(data []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("office: 不是合法的 OOXML 包（zip 解析失败：%w）", err)
	}
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		out[normalizeZipPath(f.Name)] = b
	}
	return out, nil
}

// normalizeZipPath 去掉前导 '/' 与 "./"，统一分隔符。
func normalizeZipPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	return p
}

// zipPath 拼 OOXML 内部路径（相对 base 目录解析）。
func zipPath(base, target string) string {
	target = normalizeZipPath(target)
	if target == "" {
		return ""
	}
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(target, "/")
	}
	return normalizeZipPath(path.Join(path.Dir(base), target))
}

// xmlUnmarshal 宽松解析：忽略命名空间前缀差异与注释。
func xmlUnmarshal(data []byte, v any) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	return dec.Decode(v)
}

// esc 转义 XML 文本。
func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// zipBuilder 顺序写 zip（OOXML 包不需要 ZIP64，全内存拼装）。
type zipBuilder struct {
	buf  bytes.Buffer
	zw   *zip.Writer
	err  error
	seen map[string]bool
}

// newZipBuilder 新建 zip 写入器。
func newZipBuilder() *zipBuilder {
	zb := &zipBuilder{seen: map[string]bool{}}
	zb.zw = zip.NewWriter(&zb.buf)
	return zb
}

// add 写入一个文件（同名只写一次；内容为空则补一个空文件占位）。
func (z *zipBuilder) add(name, content string) {
	if z.err != nil || z.seen[name] {
		return
	}
	z.seen[name] = true
	w, err := z.zw.CreateHeader(&zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	})
	if err != nil {
		z.err = err
		return
	}
	if _, err := io.WriteString(w, content); err != nil {
		z.err = err
	}
}

// bytes 收尾并返回完整 zip 内容。
func (z *zipBuilder) bytes() ([]byte, error) {
	if z.err != nil {
		return nil, z.err
	}
	if err := z.zw.Close(); err != nil {
		return nil, err
	}
	return z.buf.Bytes(), nil
}

// ---- 列号转换（A1 ↔ (row, col)）----

// colName 0 → "A"，25 → "Z"，26 → "AA"。
func colName(c int) string {
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

// parseColName "A1" → (0, 0)；"AB12" → (11, 27)。非法返回 (-1, -1)。
func parseColName(ref string) (row, col int) {
	col = -1
	row = -1
	i := 0
	for ; i < len(ref); i++ {
		ch := ref[i]
		if ch >= 'a' && ch <= 'z' {
			ch -= 32
		}
		if ch < 'A' || ch > 'Z' {
			break
		}
		if col < 0 {
			col = 0
		}
		col = col*26 + int(ch-'A'+1)
	}
	if col < 0 || i == 0 || i >= len(ref) {
		return -1, -1
	}
	col--
	n := 0
	for ; i < len(ref); i++ {
		ch := ref[i]
		if ch < '0' || ch > '9' {
			return -1, -1
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return -1, -1
	}
	return n - 1, col
}

// a1 生成单元格引用（"B3"）。
func a1(row, col int) string { return colName(col) + itoa(row+1) }
