// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"reflect"
	"testing"
)

// TestParseURIList XDND / NSDraggingDestination 共用的 URL 列表解析。
func TestParseURIList(t *testing.T) {
	type row struct {
		name string
		in   string
		want []string
	}
	rows := []row{
		{"空串", "", nil},
		{"注释与空行", "# comment\r\n\r\n", nil},
		{"单个本地文件", "file:///home/u/a.txt", []string{"/home/u/a.txt"}},
		{"百分号编码", "file:///home/u/%E4%B8%AD%E6%96%87.txt", []string{"/home/u/中文.txt"}},
		{"多行 CRLF", "file:///a/b.txt\r\nfile:///c/d.txt\r\n", []string{"/a/b.txt", "/c/d.txt"}},
		{"只含 CR", "file:///a.txt\rfile:///b.txt", []string{"/a.txt", "/b.txt"}},
		{"Windows 盘符 URI", "file:///C:/Users/x/a.txt", []string{"C:/Users/x/a.txt"}},
		{"直接给 Windows 路径", `C:\Users\x\a.txt`, []string{`C:\Users\x\a.txt`}},
		{"localhost 视作本地", "file://localhost/tmp/a.txt", []string{"/tmp/a.txt"}},
		{"远程主机跳过", "file://server/share/a.txt", nil},
		{"http 跳过", "https://example.com/a.txt", nil},
		{"混合", "# x\nfile:///a.txt\nhttp://e.com/b\nfile:///c.txt", []string{"/a.txt", "/c.txt"}},
	}
	for _, c := range rows {
		got := ParseURIList(c.in)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s: ParseURIList(%q) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}
