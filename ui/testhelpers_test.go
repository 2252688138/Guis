// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"runtime"
	"testing"
)

// 测试公用小工具（无 build tag）。
//
// findNodeKey 原本住在 music_test.go（//go:build windows），而 autowrap_test.go
// 没有平台限制，Linux 上 go vet 直接报 undefined。

// findNodeKey 在节点树里按键查找节点（测试辅助）。
func findNodeKey(n *Node, key string) *Node {
	if n == nil {
		return nil
	}
	if n.Key == key {
		return n
	}
	for _, c := range n.Children {
		if r := findNodeKey(c, key); r != nil {
			return r
		}
	}
	return nil
}

// skipOnNonWindows 跳过以 Windows 字体度量为基准的测试。
//
// 布局 / 渲染快照与图标对齐都依赖 newFontCache("Microsoft YaHei UI") 的字宽度量，
// Linux / macOS runner 上没有该字体（会 fallback 到别的字体），基线必然不匹配。
// 这里明确跳过而不是放宽断言：跳过是诚实的“未验证”，放宽会掩盖真回归。
func skipOnNonWindows(t *testing.T, why string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 验证：" + why)
	}
}
