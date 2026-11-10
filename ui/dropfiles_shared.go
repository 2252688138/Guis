// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// 跨平台的拖入数据处理（无 build tag，可在任何系统上单测）。
//
// X11 的 XDND 与 macOS 的 NSDraggingDestination 拿到的都是「URL 列表」，
// 只有 Windows 直接给本地路径；因此把 URL 列表解析成路径的逻辑抽到这里。
package ui

import (
	"net/url"
	"strings"
)

// ParseURIList 把 text/uri-list 或剪贴板里的 URL 列表解析成本地路径。
//
// 规则（RFC 2483 + 桌面惯例）：
//   - 每行一个条目，`#` 开头是注释，空行忽略；
//   - 支持 `file:///path`、`%E4%B8%AD` 这类百分号编码，以及 `\\?\` 前缀；
//   - `file://host/path` 形式的远程主机（host 非空且不是 localhost）跳过——
//     它们是网络共享，不是本地文件；
//   - 非 `file:` 协议（http/mailto…）跳过，本框架的拖入只处理本地文件；
//   - 兼容 CRLF 与只有 `\r` 的老式换行。
func ParseURIList(data string) []string {
	var out []string
	for _, raw := range strings.FieldsFunc(data, func(r rune) bool {
		return r == '\n' || r == '\r'
	}) {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if p, ok := uriToPath(line); ok {
			out = append(out, p)
		}
	}
	return out
}

// uriToPath 单个 URI → 本地路径（返回 ok=false 表示不是本地文件）。
func uriToPath(s string) (string, bool) {
	if len(s) == 0 {
		return "", false
	}
	// 已经是 Windows 路径（某些应用直接塞 C:\...）：原样接受
	if len(s) >= 2 && s[1] == ':' {
		return s, true
	}
	if !strings.HasPrefix(strings.ToLower(s), "file:") {
		return "", false
	}
	u, err := url.Parse(s)
	if err != nil {
		// 解析失败时退化为手工去前缀 + 反转义
		raw := strings.TrimPrefix(s, "file://")
		raw = strings.TrimPrefix(raw, "file:")
		dec, derr := url.PathUnescape(raw)
		if derr != nil || dec == "" {
			return "", false
		}
		return dec, true
	}
	// file://host/path：只有 localhost / 空 host 才是本地文件，
	// 其它 host 是网络共享（//server/share），框架不当作本地路径收。
	if h := u.Host; h != "" && !strings.EqualFold(h, "localhost") {
		return "", false
	}
	p := u.Path
	if p == "" {
		p = u.Opaque
	}
	if p == "" {
		return "", false
	}
	// file:///C:/x → /C:/x，去掉盘符前的斜杠
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return p, true
}
