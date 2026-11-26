// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// hotBoundsBackend 可选后端能力：读取 / 设置窗口在屏幕上的位置与尺寸。
//
// 目前 Windows 后端实现（见 backend_windows.go），其余平台缺失时只丢失几何信息。
// 接口本身放在无 build tag 的文件里：app.go 与 hotreload.go（后者带
// !js && !android && !ios）都要引用它，指向平台文件会让 wasm 编译失败。
type hotBoundsBackend interface {
	Bounds() (x, y, w, h int, maximized bool)
	SetBounds(x, y, w, h int)
}
