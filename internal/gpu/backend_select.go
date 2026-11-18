// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

import (
	"os"
	"strings"
	"sync"
)

// 渲染后端选择（目前仅 macOS 有 OpenGL / Metal 两条通路）。
//
// 优先级：SetRenderBackend（ui.Options.RenderBackend）> 环境变量 GUIS_RENDER。
// 取值 "metal" / "mtl" 启用 Metal；"gl" / "opengl" 强制 OpenGL；留空默认 OpenGL。
// 其它平台忽略该设置。
var (
	backendMu   sync.RWMutex
	backendName string
)

// SetRenderBackend 设置首选渲染后端（在创建窗口前调用）。
func SetRenderBackend(name string) {
	backendMu.Lock()
	backendName = strings.ToLower(strings.TrimSpace(name))
	backendMu.Unlock()
}

// RenderBackend 当前首选后端（小写，可能为空）。
func RenderBackend() string {
	backendMu.RLock()
	n := backendName
	backendMu.RUnlock()
	if n != "" {
		return n
	}
	return strings.ToLower(strings.TrimSpace(os.Getenv("GUIS_RENDER")))
}

// MetalRequested 是否请求使用 Metal 后端。
func MetalRequested() bool {
	n := RenderBackend()
	return n == "metal" || n == "mtl"
}

// ForceGLRequested 是否显式要求 OpenGL。
func ForceGLRequested() bool {
	n := RenderBackend()
	return n == "gl" || n == "opengl"
}
