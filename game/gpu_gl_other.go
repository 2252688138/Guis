// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build !windows

package game

import "errors"

// 非 Windows 平台暂不提供 OpenGL 后端（Linux 需要 GLX、移动端需要 EGL，
// 都涉及额外窗口系统绑定）。注册占位实现，NewGPU 会自动回退软件后端。

func init() {
	RegisterGPUBackend("gl", func(g *GPU) (gpuBackend, error) {
		return nil, errors.New("game: 当前平台未实现 OpenGL 后端（仅 Windows），请使用 soft 后端")
	})
}
