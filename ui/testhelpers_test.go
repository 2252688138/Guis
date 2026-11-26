// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

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
