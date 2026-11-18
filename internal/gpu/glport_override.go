//go:build linux || darwin || freebsd || android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

// glFnsOverride 非 nil 时替换平台 GL 函数表。
//
// macOS Metal 后端通过它在运行期把同一组 glXxx 包装切到 Metal 实现
// （见 metal_gl_darwin.go 的 installMetalGL），渲染器本身不需要知道后端差异。
var glFnsOverride func() *glFns
