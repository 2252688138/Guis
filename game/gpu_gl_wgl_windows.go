// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build windows

package game

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// Windows WGL 绑定：用 syscall（无 cgo、无第三方依赖）创建隐藏窗口 +
// OpenGL 上下文，并按名解析 GL 函数入口。
//
// 只创建隐藏的 1×1 窗口用于取 DC，渲染全部发生在离线 FBO 上，
// 因此不需要与 UI 主循环合成，失败时上层可无痛回退软件后端。

var (
	glUser32 = syscall.NewLazyDLL("user32.dll")
	glGdi32  = syscall.NewLazyDLL("gdi32.dll")
	glLib    = syscall.NewLazyDLL("opengl32.dll")

	procGLRegisterClassExW = glUser32.NewProc("RegisterClassExW")
	procGLCreateWindowExW  = glUser32.NewProc("CreateWindowExW")
	procGLGetDC            = glUser32.NewProc("GetDC")
	procGLReleaseDC        = glUser32.NewProc("ReleaseDC")
	procGLDestroyWindow    = glUser32.NewProc("DestroyWindow")
	procGLDefWindowProcW   = glUser32.NewProc("DefWindowProcW")
	procGLGetModuleHandle  = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")

	procGLChoosePixelFormat = glGdi32.NewProc("ChoosePixelFormat")
	procGLSetPixelFormat    = glGdi32.NewProc("SetPixelFormat")

	procGLWglCreateContext       = glLib.NewProc("wglCreateContext")
	procGLWglMakeCurrent         = glLib.NewProc("wglMakeCurrent")
	procGLWglDeleteContext       = glLib.NewProc("wglDeleteContext")
	procGLWglGetProcAddress      = glLib.NewProc("wglGetProcAddress")
	procGLWglGetCurrentContext   = glLib.NewProc("wglGetCurrentContext")
	procGLWglGetCurrentDC        = glLib.NewProc("wglGetCurrentDC")
	procGLWglGetExtensionsString = glLib.NewProc("wglGetExtensionsStringARB")
)

// glWndClassExW Win32 WNDCLASSEXW。
type glWndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

// glPixelFormatDescriptor Win32 PIXELFORMATDESCRIPTOR。
type glPixelFormatDescriptor struct {
	nSize           uint16
	nVersion        uint16
	dwFlags         uint32
	iPixelType      uint8
	cColorBits      uint8
	cRedBits        uint8
	cRedShift       uint8
	cGreenBits      uint8
	cGreenShift     uint8
	cBlueBits       uint8
	cBlueShift      uint8
	cAlphaBits      uint8
	cAlphaShift     uint8
	cAccumBits      uint8
	cAccumRedBits   uint8
	cAccumGreenBits uint8
	cAccumBlueBits  uint8
	cAccumAlphaBits uint8
	cDepthBits      uint8
	cStencilBits    uint8
	cAuxBuffers     uint8
	iLayerType      uint8
	bReserved       uint8
	dwLayerMask     uint32
	dwVisibleMask   uint32
	dwDamageMask    uint32
}

// glContext 隐藏窗口 + GL 上下文。
type glContext struct {
	hwnd  uintptr
	hdc   uintptr
	hrc   uintptr
	procs map[string]uintptr
	// external 复用宿主（GUI）已绑定的上下文：不拥有窗口，不删上下文。
	external bool
}

// glWndProc 空消息处理（隐藏窗口不需要任何消息响应）。
var glWndProc = syscall.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
	r, _, _ := procGLDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
})

// sharedGLContext 当前线程已有 GL 上下文时，包一层复用它（不新建窗口）。
func sharedGLContext() *glContext {
	cur, _, _ := procGLWglGetCurrentContext.Call()
	if cur == 0 {
		return nil
	}
	dc, _, _ := procGLWglGetCurrentDC.Call()
	if dc == 0 {
		return nil
	}
	return &glContext{hdc: dc, hrc: cur, procs: map[string]uintptr{}, external: true}
}

// glLoader 名称 → 入口地址解析器。
type glLoader func(name string) (uintptr, bool)

// newGLContext 取得 OpenGL 上下文。
//
// 优先复用当前线程已绑定的上下文：GUI 主循环里 guis 已把自己的窗口上下文设为
// current，复用之后 game 的纹理/FBO 与 GUI 处在同一命名空间，才能把 FBO 颜色
// 纹理零拷贝交给 GUI 合成（否则跨上下文纹理不可见，只能回读像素）。
// 当前线程没有上下文时才创建隐藏窗口 + 独立上下文（离线渲染 / 单测场景）。
func newGLContext() (*glContext, error) {
	if c := sharedGLContext(); c != nil {
		return c, nil
	}
	hInst, _, _ := procGLGetModuleHandle.Call(0)
	cls, err := syscall.UTF16PtrFromString("GuisGPUOffscreen")
	if err != nil {
		return nil, err
	}
	wc := glWndClassExW{
		cbSize:        uint32(unsafe.Sizeof(glWndClassExW{})),
		style:         0x0020, // CS_OWNDC
		lpfnWndProc:   glWndProc,
		hInstance:     hInst,
		lpszClassName: cls,
	}
	// 忽略重复注册（多设备场景）
	procGLRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	title, _ := syscall.UTF16PtrFromString("guis-gpu")
	hwnd, _, e := procGLCreateWindowExW.Call(0,
		uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)),
		0x80000000, // WS_POPUP
		0, 0, 1, 1, 0, 0, hInst, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("game: 创建 GL 隐藏窗口失败: %v", e)
	}
	dc, _, _ := procGLGetDC.Call(hwnd)
	if dc == 0 {
		procGLDestroyWindow.Call(hwnd)
		return nil, errors.New("game: 取窗口 DC 失败")
	}
	pfd := glPixelFormatDescriptor{
		nSize:      uint16(unsafe.Sizeof(glPixelFormatDescriptor{})),
		nVersion:   1,
		dwFlags:    0x4 | 0x20 | 0x1, // PFD_DRAW_TO_WINDOW | PFD_SUPPORT_OPENGL | PFD_DOUBLEBUFFER
		iPixelType: 0,
		cColorBits: 32,
		cAlphaBits: 8,
		cDepthBits: 24,
	}
	pf, _, e := procGLChoosePixelFormat.Call(dc, uintptr(unsafe.Pointer(&pfd)))
	if pf == 0 {
		procGLReleaseDC.Call(hwnd, dc)
		procGLDestroyWindow.Call(hwnd)
		return nil, fmt.Errorf("game: 无可用像素格式: %v", e)
	}
	if r, _, _ := procGLSetPixelFormat.Call(dc, pf, uintptr(unsafe.Pointer(&pfd))); r == 0 {
		procGLReleaseDC.Call(hwnd, dc)
		procGLDestroyWindow.Call(hwnd)
		return nil, errors.New("game: SetPixelFormat 失败")
	}
	rc, _, e := procGLWglCreateContext.Call(dc)
	if rc == 0 {
		procGLReleaseDC.Call(hwnd, dc)
		procGLDestroyWindow.Call(hwnd)
		return nil, fmt.Errorf("game: wglCreateContext 失败: %v", e)
	}
	if r, _, _ := procGLWglMakeCurrent.Call(dc, rc); r == 0 {
		procGLWglDeleteContext.Call(rc)
		procGLReleaseDC.Call(hwnd, dc)
		procGLDestroyWindow.Call(hwnd)
		return nil, errors.New("game: wglMakeCurrent 失败")
	}
	c := &glContext{hwnd: hwnd, hdc: dc, hrc: rc, procs: map[string]uintptr{}}
	return c, nil
}

// resolve 解析 GL 函数入口：先试 wglGetProcAddress，再试 opengl32.dll 导出。
func (c *glContext) resolve(name string) (uintptr, bool) {
	if a, ok := c.procs[name]; ok {
		return a, ok
	}
	cs, err := syscall.BytePtrFromString(name)
	if err != nil {
		return 0, false
	}
	if a, _, _ := procGLWglGetProcAddress.Call(uintptr(unsafe.Pointer(cs))); a != 0 {
		c.procs[name] = a
		return a, true
	}
	if p := glLib.NewProc(name); p.Find() == nil {
		a := p.Addr()
		c.procs[name] = a
		return a, true
	}
	return 0, false
}

// load 批量解析必需函数（缺失即失败）。
func (c *glContext) load(required []string) error {
	missing := ""
	for _, n := range required {
		if _, ok := c.resolve(n); !ok {
			missing += n + " "
		}
	}
	if missing != "" {
		return fmt.Errorf("game: GL 入口缺失: %s", missing)
	}
	return nil
}

// call 调用 GL 函数（无返回值）。
func (c *glContext) call(name string, args ...uintptr) uintptr {
	a, ok := c.resolve(name)
	if !ok {
		return 0
	}
	r, _, _ := syscall.SyscallN(a, args...)
	return r
}

// str 读取 glGetString。
func (c *glContext) str(pname uintptr) string {
	p := c.call("glGetString", pname)
	if p == 0 {
		return ""
	}
	var out []byte
	for i := 0; i < 512; i++ {
		b := *(*byte)(unsafe.Pointer(p + uintptr(i)))
		if b == 0 {
			break
		}
		out = append(out, b)
	}
	return string(out)
}

// glContextMu 串行化 GL 调用：OpenGL 上下文同一时刻只能在一个线程生效，
// Go 的 goroutine 会在线程间迁移，因此每次进入 GL 前都要重绑上下文。
var glContextMu sync.Mutex

// makeCurrentNow 尝试把上下文绑到当前 OS 线程，返回是否成功。
func (c *glContext) makeCurrentNow() bool {
	if c == nil || c.hrc == 0 {
		return false
	}
	cur, _, _ := procGLWglGetCurrentContext.Call()
	if cur == c.hrc {
		return true
	}
	r, _, _ := procGLWglMakeCurrent.Call(c.hdc, c.hrc)
	return r != 0
}

// ensureCurrent 把上下文绑定到当前 OS 线程。
func (c *glContext) ensureCurrent() {
	c.makeCurrentNow()
}

// destroy 释放上下文与窗口。
func (c *glContext) destroy() {
	if c == nil {
		return
	}
	if c.external {
		// 宿主上下文归宿主所有，这里只丢弃函数表缓存。
		c.procs = nil
		c.hdc, c.hrc, c.hwnd = 0, 0, 0
		return
	}
	if c.hrc != 0 {
		procGLWglMakeCurrent.Call(0, 0)
		procGLWglDeleteContext.Call(c.hrc)
		c.hrc = 0
	}
	if c.hwnd != 0 && c.hdc != 0 {
		procGLReleaseDC.Call(c.hwnd, c.hdc)
		c.hdc = 0
	}
	if c.hwnd != 0 {
		procGLDestroyWindow.Call(c.hwnd)
		c.hwnd = 0
	}
}
