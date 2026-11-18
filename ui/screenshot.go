// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"errors"
	"image"
	"image/png"
	"os"
)

// 窗口内容截图（像素级）。
//
//	img, err := ui.Screenshot()
//	_ = ui.SaveScreenshot("dist/shot.png")
//
// 用途：应用内「导出当前界面」、自动化冒烟取色、以及渲染回归（golden）比对。
// 读回的是**物理像素**（含超采样），逻辑尺寸与它的比值见 Style 缩放与
// `ui.Options.SuperSample`。

// New 创建窗口与渲染资源，但**不进入事件循环**。
//
// 常规应用用 ui.Run；本函数供工具链使用（截图工具、渲染回归、自动化冒烟）：
// 拿到 *App 后可直接 CaptureFrame 取一帧像素，再 Close 退出。
func New(opts Options, view func() *Node) (*App, error) {
	return newApp(opts, view)
}

// errNoFramebuffer 当前窗口没有可读回的帧缓冲。
var errNoFramebuffer = errors.New("ui: 当前没有可读回的帧缓冲（窗口未创建或平台不支持）")

// Screenshot 把当前窗口内容读回成图像。
//
// 须在 UI 线程调用（会自动 MakeCurrent）；无窗口或平台不支持时返回错误。
func Screenshot() (*image.RGBA, error) {
	a := currentApp.Load()
	if a == nil {
		return nil, errNoFramebuffer
	}
	return a.Screenshot()
}

// Screenshot 读回本窗口当前内容（物理像素，左上原点）。
func (a *App) Screenshot() (*image.RGBA, error) {
	if a == nil || a.r == nil || a.win == nil {
		return nil, errNoFramebuffer
	}
	a.win.MakeCurrent()
	pix, w, h := a.r.ReadPixels()
	if w <= 0 || h <= 0 || len(pix) < w*h*4 {
		return nil, errNoFramebuffer
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, pix)
	return img, nil
}

// CaptureFrame 主动渲染一帧，并在 Swap 之前把像素读回。
//
// 与 Screenshot 的区别：Screenshot 读的是当前帧缓冲里已有的内容，双缓冲交换
// 之后其内容未定义；CaptureFrame 自己跑一帧并抢在 Swap 之前读，因此渲染回归
// （golden）与自动化冒烟都用它。
func (a *App) CaptureFrame() (*image.RGBA, error) {
	if a == nil || a.r == nil || a.win == nil {
		return nil, errNoFramebuffer
	}
	a.capturePix, a.captureErr, a.captureReq = nil, nil, true
	a.frame(true)
	if a.captureErr != nil {
		return nil, a.captureErr
	}
	if a.captureW <= 0 || a.captureH <= 0 || len(a.capturePix) < a.captureW*a.captureH*4 {
		return nil, errNoFramebuffer
	}
	img := image.NewRGBA(image.Rect(0, 0, a.captureW, a.captureH))
	copy(img.Pix, a.capturePix)
	a.capturePix = nil
	return img, nil
}

// SaveScreenshot 把当前窗口内容写成 PNG 文件。
func SaveScreenshot(path string) error {
	img, err := Screenshot()
	if err != nil {
		return err
	}
	return writePNG(path, img)
}

// writePNG 落盘（测试与截图共用）。
func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
