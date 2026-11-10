// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 窗口特效与任务栏集成。
//
//	app.SetOpacity(0.92)            // 整体半透明
//	app.SetClickThrough(true)       // 鼠标穿透（桌面宠物 / 悬浮挂件）
//	app.SetSkipTaskbar(true)        // 不在任务栏显示
//	app.SetKeepOnBottom(true)       // 压在所有普通窗口下面（壁纸式）
//	app.SetDarkTitleBar(true)       // 系统标题栏跟随深色主题
//	app.SetBackdrop(ui.BackdropMica) // Win11 材质（Mica / Acrylic）
//	app.SetCaptureExcluded(true)     // 防截屏 / 录屏（隐私面板）
//	app.SetMinSize(720, 480)
//	app.SetTaskbarProgress(3, 10)    // 任务栏进度条
//	app.SetBadge(5)                  // 任务栏未读数角标
//	app.Flash(3)                     // 闪烁任务栏提醒
//
// 这些能力在非 Windows 平台上是**安全空操作**（接口断言失败直接返回），
// 业务侧可用 HasCap(CapWindowFX) / HasCap(CapTaskbar) 决定是否展示入口。

// BackdropKind 窗口背景材质（Windows 11 起支持）。
type BackdropKind int

const (
	// BackdropNone 关闭材质（普通不透明窗口）。
	BackdropNone BackdropKind = iota
	// BackdropMica Mica（Win11 22H2+，随壁纸取色）。
	BackdropMica
	// BackdropAcrylic 亚克力（模糊壁纸 / 下层窗口）。
	BackdropAcrylic
	// BackdropTabbed Mica Alt（标签页式窗口，着色更深）。
	BackdropTabbed
)

// windowFXBackend 可选的窗口特效后端：只有提供的平台需要实现，
// App 侧用类型断言，其它平台自然落空（不 panic、不改 Backend 接口）。
type windowFXBackend interface {
	SetWindowOpacity(f float32)
	WindowOpacity() float32
	SetClickThrough(on bool)
	ClickThrough() bool
	SetKeepOnBottom(on bool)
	SetSkipTaskbar(on bool)
	SkipTaskbar() bool
	SetCaptureExcluded(on bool)
	SetDarkTitleBar(on bool)
	SetBackdrop(kind BackdropKind)
	SetMinSize(w, h int)
	SetMaxSize(w, h int)
	Activate()
}

// taskbarBackend 可选的任务栏后端（Windows: ITaskbarList3）。
type taskbarBackend interface {
	SetTaskbarProgress(done, total int64)
	ClearTaskbarProgress()
	SetTaskbarBadge(n int)
	FlashTaskbar(count int, untilFocused bool)
}

func (a *App) fx() windowFXBackend {
	if a == nil || a.win == nil {
		return nil
	}
	b, _ := a.win.(windowFXBackend)
	return b
}

func (a *App) tb() taskbarBackend {
	if a == nil || a.win == nil {
		return nil
	}
	b, _ := a.win.(taskbarBackend)
	return b
}

// SetOpacity 设置窗口整体不透明度（0.1~1.0，1 = 完全不透明）。
// 除 Windows 外的平台为空操作。
func (a *App) SetOpacity(f float32) {
	if f < 0.05 {
		f = 0.05
	}
	if f > 1 {
		f = 1
	}
	if b := a.fx(); b != nil {
		b.SetWindowOpacity(f)
	}
	a.dirty = true
}

// Opacity 当前窗口不透明度（不支持时返回 1）。
func (a *App) Opacity() float32 {
	if b := a.fx(); b != nil {
		return b.WindowOpacity()
	}
	return 1
}

// SetClickThrough 开启鼠标穿透：点击直接落到下面的窗口（本窗口只负责绘制）。
// 桌面宠物 / 悬浮挂件 / 屏幕涂层常驻工具用；开启后本窗口收不到任何鼠标事件。
func (a *App) SetClickThrough(on bool) {
	if b := a.fx(); b != nil {
		b.SetClickThrough(on)
	}
}

// ClickThrough 当前是否处于鼠标穿透状态。
func (a *App) ClickThrough() bool {
	if b := a.fx(); b != nil {
		return b.ClickThrough()
	}
	return false
}

// SetKeepOnBottom 把窗口压到所有普通窗口下面（壁纸 / 桌面挂件）。
func (a *App) SetKeepOnBottom(on bool) {
	if b := a.fx(); b != nil {
		b.SetKeepOnBottom(on)
	}
	a.dirty = true
}

// SetSkipTaskbar 是否在任务栏隐藏本窗口（托盘常驻工具的副窗口常用）。
func (a *App) SetSkipTaskbar(on bool) {
	if b := a.fx(); b != nil {
		b.SetSkipTaskbar(on)
	}
}

// SkipTaskbar 当前是否隐藏了任务栏按钮。
func (a *App) SkipTaskbar() bool {
	if b := a.fx(); b != nil {
		return b.SkipTaskbar()
	}
	return false
}

// SetCaptureExcluded 是否禁止被截屏 / 录屏捕获（SetWindowDisplayAffinity）。
// 密码框、隐私面板、付费内容预览常用。
func (a *App) SetCaptureExcluded(on bool) {
	if b := a.fx(); b != nil {
		b.SetCaptureExcluded(on)
	}
}

// SetDarkTitleBar 系统标题栏是否使用深色（自绘标题栏窗口无影响）。
func (a *App) SetDarkTitleBar(on bool) {
	if b := a.fx(); b != nil {
		b.SetDarkTitleBar(on)
	}
}

// SetBackdrop 设置窗口背景材质（Win11 Mica / Acrylic）。
// Windows 10 及更早、其它平台上是空操作。
func (a *App) SetBackdrop(kind BackdropKind) {
	if b := a.fx(); b != nil {
		b.SetBackdrop(kind)
	}
}

// SetMinSize 设置窗口最小尺寸（逻辑像素，0 = 不限制）。
func (a *App) SetMinSize(w, h int) {
	if b := a.fx(); b != nil {
		b.SetMinSize(w, h)
	}
}

// SetMaxSize 设置窗口最大尺寸（逻辑像素，0 = 不限制）。
func (a *App) SetMaxSize(w, h int) {
	if b := a.fx(); b != nil {
		b.SetMaxSize(w, h)
	}
}

// SetTaskbarProgress 在任务栏图标上显示进度条（done/total；total<=0 表示不确定进度）。
// 下载 / 导出 / 批量处理这类耗时任务用它，用户最小化后也能看到进展。
func (a *App) SetTaskbarProgress(done, total int64) {
	if b := a.tb(); b != nil {
		b.SetTaskbarProgress(done, total)
	}
}

// ClearTaskbarProgress 隐藏任务栏进度条。
func (a *App) ClearTaskbarProgress() {
	if b := a.tb(); b != nil {
		b.ClearTaskbarProgress()
	}
}

// SetBadge 在任务栏图标上显示未读数角标（0 = 清除）。非 Windows 空操作。
func (a *App) SetBadge(n int) {
	if b := a.tb(); b != nil {
		b.SetTaskbarBadge(n)
	}
}

// Flash 闪烁任务栏图标提醒用户（count<=0 时一直闪到窗口获得焦点）。
func (a *App) Flash(count int) {
	if b := a.tb(); b != nil {
		b.FlashTaskbar(count, count <= 0)
	}
}
