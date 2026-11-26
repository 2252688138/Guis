// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 整数小工具：放在无 build tag 的文件里。
//
// 这两个函数原本分别住在 media.go（windows / linux / freebsd）与 term.go，
// 而 viewport3d_demo.go、media.go 之外的代码在每个平台都要用它们；
// 一旦被平台的 build tag 挡住，macOS 与 wasm 上就会 undefined。

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// maxInt 两整数较大值。
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
