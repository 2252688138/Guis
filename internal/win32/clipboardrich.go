//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"errors"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// 富文本剪贴板：CF_HTML / RTF 以及任意自定义格式。
//
// CF_HTML 的数据不是裸 HTML：开头要带一段 "Version:0.9 / StartHTML / EndHTML /
// StartFragment / EndFragment" 头，偏移量是**字节**偏移（UTF-8），目标应用靠它
// 截取片段。这里负责拼头与解析，业务只看到纯 HTML。

var (
	procRegisterClipboardFormatW = user32L.NewProc("RegisterClipboardFormatW")
	procGetClipboardSequence     = user32L.NewProc("GetClipboardSequenceNumber")
)

// RegisterClipboardFormat 注册 / 查询剪贴板格式号（同名格式进程间共享 ID）。
func RegisterClipboardFormat(name string) uint16 {
	if procRegisterClipboardFormatW.Find() != nil || name == "" {
		return 0
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0
	}
	r, _, _ := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(p)))
	return uint16(r)
}

// CF_HTMLFragment 把一段 HTML 包成标准 CF_HTML 片段（带偏移头）。
func CF_HTMLFragment(html string) []byte {
	// 头长度固定为 8 位十进制偏移，因此先按最大宽度拼再回填。
	const tmpl = "Version:0.9\r\nStartHTML:0000000105\r\nEndHTML:0000000000\r\nStartFragment:0000000000\r\nEndFragment:0000000000\r\n"
	prefix := "<html><body><!--StartFragment-->"
	suffix := "<!--EndFragment--></body></html>"
	headerLen := len(tmpl)
	startHTML := headerLen
	startFrag := startHTML + len(prefix)
	endFrag := startFrag + len(html)
	endHTML := endFrag + len(suffix)

	var b strings.Builder
	b.Grow(endHTML)
	b.WriteString("Version:0.9\r\n")
	b.WriteString("StartHTML:" + pad10(startHTML) + "\r\n")
	b.WriteString("EndHTML:" + pad10(endHTML) + "\r\n")
	b.WriteString("StartFragment:" + pad10(startFrag) + "\r\n")
	b.WriteString("EndFragment:" + pad10(endFrag) + "\r\n")
	b.WriteString(prefix)
	b.WriteString(html)
	b.WriteString(suffix)
	return []byte(b.String())
}

// pad10 把偏移补成 10 位（CF_HTML 规范要求定宽，否则头长度会漂移）。
func pad10(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 10 {
		s = "0" + s
	}
	return s
}

// ParseCF_HTML 从 CF_HTML 数据里取出片段（无头时原样返回）。
func ParseCF_HTML(data []byte) string {
	s := string(data)
	start := findHeaderInt(s, "StartFragment:")
	end := findHeaderInt(s, "EndFragment:")
	if start <= 0 || end <= start || end > len(data) {
		return s
	}
	return string(data[start:end])
}

// findHeaderInt 解析 "Key:0000000123" 形式的头字段。
func findHeaderInt(s, key string) int {
	i := strings.Index(s, key)
	if i < 0 {
		return -1
	}
	rest := s[i+len(key):]
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j == 0 {
		return -1
	}
	n, err := strconv.Atoi(rest[:j])
	if err != nil {
		return -1
	}
	return n
}

// ClipboardFormats 返回剪贴板当前可用的自定义格式名（CF_HTML / RTF 等）。
// 只做「是否可用」判断：为 0 表示没有数据。
func ClipboardFormats() []string {
	var out []string
	for _, name := range []string{"HTML Format", "Rich Text Format"} {
		if RegisterClipboardFormat(name) == 0 {
			continue
		}
		if IsClipboardFormatAvailable != nil &&
			IsClipboardFormatAvailable(uint32(RegisterClipboardFormat(name))) != 0 {
			out = append(out, name)
		}
	}
	return out
}

// ClipboardGetFormat 读取指定自定义格式的原始字节。
func ClipboardGetFormat(name string) ([]byte, error) {
	cf := RegisterClipboardFormat(name)
	if cf == 0 || OpenClipboard == nil {
		return nil, errors.New("win32: 剪贴板格式不可用")
	}
	if OpenClipboard(0) == 0 {
		return nil, errors.New("win32: 打开剪贴板失败")
	}
	defer CloseClipboard()
	h, _, _ := procGetClipboardData.Call(uintptr(cf))
	if h == 0 {
		return nil, errors.New("win32: 剪贴板无该格式数据")
	}
	size, _, _ := procGlobalSize.Call(h)
	if size == 0 {
		return nil, errors.New("win32: 数据为空")
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return nil, errors.New("win32: 锁定剪贴板内存失败")
	}
	defer procGlobalUnlock.Call(h)
	buf := make([]byte, size)
	copy(buf, unsafe.Slice((*byte)(unsafe.Pointer(p)), size))
	// 去掉尾部 NUL
	for len(buf) > 0 && buf[len(buf)-1] == 0 {
		buf = buf[:len(buf)-1]
	}
	return buf, nil
}

// ClipboardSetFormat 写入指定自定义格式（同时写纯文本回退）。
func ClipboardSetFormat(name string, data []byte, plainText string) error {
	cf := RegisterClipboardFormat(name)
	if cf == 0 || OpenClipboard == nil || SetClipboardData == nil {
		return errors.New("win32: 剪贴板格式不可用")
	}
	if OpenClipboard(0) == 0 {
		return errors.New("win32: 打开剪贴板失败")
	}
	defer CloseClipboard()
	EmptyClipboard()
	if err := globalSet(cf, data); err != nil {
		return err
	}
	if plainText != "" {
		_ = globalSet(cfUnicodeText, encodeUTF16(plainText))
	}
	return nil
}

const cfUnicodeText = 13

// globalSet 把字节写入 HGLOBAL 并交给剪贴板。
func globalSet(cf uint16, data []byte) error {
	h := GlobalAlloc(GMEMoveable, uintptr(len(data)))
	if h == 0 {
		return errors.New("win32: 分配剪贴板内存失败")
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		GlobalFree(h)
		return errors.New("win32: 锁定内存失败")
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(p)), len(data)), data)
	procGlobalUnlock.Call(h)
	if r, _, _ := procSetClipboardData.Call(uintptr(cf), h); r == 0 {
		GlobalFree(h)
		return errors.New("win32: SetClipboardData 失败")
	}
	// 成功后所有权归系统，不能再 GlobalFree
	return nil
}

// encodeUTF16 把字符串编码成带结尾 NUL 的 UTF-16LE 字节。
func encodeUTF16(s string) []byte {
	u := syscall.StringToUTF16(s)
	buf := make([]byte, len(u)*2)
	for i, c := range u {
		buf[i*2] = byte(c)
		buf[i*2+1] = byte(c >> 8)
	}
	return buf
}

// procSetClipboardData 单独声明（其它剪贴板过程在 desktop_extra.go 里已有）。
var procSetClipboardData = user32L.NewProc("SetClipboardData")
