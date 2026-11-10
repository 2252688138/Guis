// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 富文本剪贴板（HTML / RTF）。
//
//	ui.ClipboardSetHTML("<b>加粗</b>")   // 粘到 Word / 浏览器里保留格式
//	html, err := ui.ClipboardHTML()      // 从浏览器复制的内容能拿到标签
//	ui.ClipboardSetRTF(rtfText)
//
// 与 ClipboardSetText 的区别：纯文本会丢掉所有格式；这里写的是 CF_HTML / RTF
// 格式，Word、Outlook、Notion 都能正确识别。同时仍会写入纯文本回退（老应用
// 只认 CF_UNICODETEXT）。
//
// Windows 走 RegisterClipboardFormat；Linux 需要 xclip（-t text/html）。

// clipboardRichGetNative 读取富文本（Windows: CF_HTML / Rich Text Format）。
var clipboardRichGetNative func(format string) (string, bool)

// clipboardRichSetNative 写入富文本。
var clipboardRichSetNative func(format, data string) bool

// 富文本格式常量（传给 ClipboardHTML / ClipboardSetHTML 之外的通用接口）。
const (
	// ClipHTML HTML 片段。
	ClipHTML = "text/html"
	// ClipRTF RTF 文档。
	ClipRTF = "text/rtf"
)

// ClipboardHTML 读取剪贴板里的 HTML 片段（无则返回空串 + 错误）。
func ClipboardHTML() (string, error) {
	if clipboardRichGetNative == nil {
		return "", errUnsupported("读取 HTML 剪贴板")
	}
	if s, ok := clipboardRichGetNative(ClipHTML); ok {
		return s, nil
	}
	return "", errUnsupported("读取 HTML 剪贴板")
}

// ClipboardSetHTML 把 HTML 片段写入剪贴板（同时写纯文本回退）。
func ClipboardSetHTML(html string) error {
	if html == "" {
		return errUnsupported("写入 HTML 剪贴板")
	}
	if clipboardRichSetNative == nil || !clipboardRichSetNative(ClipHTML, html) {
		return errUnsupported("写入 HTML 剪贴板")
	}
	return nil
}

// ClipboardRTF 读取剪贴板里的 RTF。
func ClipboardRTF() (string, error) {
	if clipboardRichGetNative == nil {
		return "", errUnsupported("读取 RTF 剪贴板")
	}
	if s, ok := clipboardRichGetNative(ClipRTF); ok {
		return s, nil
	}
	return "", errUnsupported("读取 RTF 剪贴板")
}

// ClipboardSetRTF 把 RTF 写入剪贴板。
func ClipboardSetRTF(rtf string) error {
	if rtf == "" {
		return errUnsupported("写入 RTF 剪贴板")
	}
	if clipboardRichSetNative == nil || !clipboardRichSetNative(ClipRTF, rtf) {
		return errUnsupported("写入 RTF 剪贴板")
	}
	return nil
}

// HasClipboardHTML 剪贴板当前是否含 HTML。
func HasClipboardHTML() bool {
	if clipboardRichGetNative == nil {
		return false
	}
	_, ok := clipboardRichGetNative(ClipHTML)
	return ok
}

// HTMLToPlainText 极简 HTML 去标签（用于「粘成纯文本」）。
// 只处理标签剥离与常见实体，不追求完整解析。
func HTMLToPlainText(html string) string {
	out := make([]rune, 0, len(html))
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out = append(out, r)
		}
	}
	s := string(out)
	s = replaceAllSimple(s, "&nbsp;", " ")
	s = replaceAllSimple(s, "&lt;", "<")
	s = replaceAllSimple(s, "&gt;", ">")
	s = replaceAllSimple(s, "&quot;", "\"")
	s = replaceAllSimple(s, "&amp;", "&")
	return s
}

// replaceAllSimple 不依赖 strings.ReplaceAll 之外逻辑的最小封装（保持实体替换集中）。
func replaceAllSimple(s, old, new string) string {
	if old == "" {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out = append(out, new...)
			i += len(old)
			continue
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}
