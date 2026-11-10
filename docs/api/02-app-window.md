<!-- group: 入门 -->
# 应用与窗口 · App / 多窗口 / 生命周期

## 1. App 是什么

`*ui.App` = 一个窗口实例：窗口后端 + GL 渲染器 + 字体缓存 + 事件分发 + 节点树 + 各种缓存。
UI 线程独占，所有方法都应在 UI 线程（`view()` 内或 `App.Post` 里）调用。

```go
func view() *ui.Node {
    app := ui.Current()      // 当前活跃窗口；无窗口时为 nil
    if app != nil && app.IsMaximized() { /* 响应式调整 */ }
    ...
}
```

## 2. 窗口状态

| 方法 | 说明 |
| --- | --- |
| `SetTitle(s)` | 改标题（自绘标题栏 + 任务栏） |
| `ShowWindow(v)` / `Show()` / `Restore()` | 显示 / 置前 / 还原 |
| `Minimize()` | 最小化（最小化时暂停渲染，恢复后强制重绘并重建媒体解码窗口） |
| `Center()` | 居中到当前显示器工作区 |
| `ToggleMaximize()` / `IsMaximized()` | 最大化切换 / 查询 |
| `BeginDragMove()` | 把按下事件交给系统拖动窗口（自绘标题栏用） |
| `SetFullscreen(v)` / `IsFullscreen()` | 全屏（覆盖当前显示器，ESC 可退出） |
| `SetCornerRadius(v)` / `CornerRadius()` | 窗口圆角（逻辑像素，`<=0` 取消） |
| `SetAlwaysOnTop(v)` / `IsAlwaysOnTop()` | 置顶 |
| `HWND()` | 原生句柄（Windows HWND / X11 Window / NSView* / ANativeWindow*） |
| `Close()` | 关闭（会经过 `SetOnClose` 拦截） |
| `Quit()` | 退出，**跳过**关闭拦截（托盘菜单「退出」用它） |
| `Dispose()` | 立刻释放资源（一般不用手调） |

### 窗口特效与任务栏（Windows 真实现，其它平台安全空操作）

| 方法 | 说明 |
| --- | --- |
| `SetOpacity(f)` / `Opacity()` | 整窗不透明度（0.05~1） |
| `SetClickThrough(v)` / `ClickThrough()` | 鼠标穿透（桌面宠物 / 悬浮挂件） |
| `SetKeepOnBottom(v)` | 压到 Z 序最底（壁纸式挂件） |
| `SetSkipTaskbar(v)` / `SkipTaskbar()` | 隐藏任务栏按钮 |
| `SetCaptureExcluded(v)` | 防截屏 / 录屏（隐私面板） |
| `SetDarkTitleBar(v)` | 系统标题栏跟随深色主题 |
| `SetBackdrop(kind)` | Win11 材质：`BackdropMica` / `BackdropAcrylic` / `BackdropTabbed` |
| `SetMinSize(w,h)` / `SetMaxSize(w,h)` | 客户区尺寸约束（`WM_GETMINMAXINFO`，逻辑像素） |
| `Activate()` | 拉到前台并抢焦点 |
| `SetTaskbarProgress(done,total)` / `ClearTaskbarProgress()` | 任务栏进度条（`total<=0` 为不确定进度） |
| `SetBadge(n)` | 任务栏未读数角标（0 清除） |
| `Flash(n)` | 闪烁任务栏图标（`n<=0` 一直闪到获得焦点） |

能力位：`HasCap(CapWindowFX)` / `HasCap(CapTaskbar)`。完整说明见《附录 · 桌面增强》第 9、10 节。

## 3. 关闭拦截（收进托盘）

```go
app := ui.Current()
app.SetOnClose(func() bool {
    app.ShowWindow(false)   // 只隐藏窗口
    return false            // false = 阻止关闭
})
// 托盘菜单「退出」时：
app.Quit()                  // true 语义，跳过拦截真正退出
```

## 4. 跨 goroutine 更新界面

```go
go func() {
    data := fetch()
    ui.RequestRedraw()          // 任意线程：请求重绘（状态已改）
    // 或只重绘某个窗口
    ui.RequestRedrawIn(other)
    // 需要在 UI 线程做的事（建窗口、开对话框）：
    app.Post(func() { ... })
}()
```

`ui.RunAsync(fn)` 是「可安全调用平台 API 的线程」上执行任务的入口（桌面为后台 goroutine，
浏览器为同线程排队）。

## 5. 多窗口

```go
child, err := ui.NewWindow(ui.Options{Title: "子窗口", Width: 520, Height: 360}, childView)
// 等价写法（当前 UI 线程内再开一个）：
child, err = ui.OpenWindow(opts, childView)
```

`NewWindow` 必须在 UI 线程 / 回调中调用。每个窗口有独立的 `view()`、滚动位置与浮层状态。

## 6. 每帧可用的查询 API

| API | 说明 |
| --- | --- |
| `ui.ViewportW()` / `ui.ViewportH()` | 最近一次布局的视口尺寸（逻辑像素），响应式分支用它 |
| `ui.PixelScale()` | 物理像素比（100% = 1） |
| `ui.MousePos()` | 鼠标当前位置 |
| `ui.MouseDown()` | 左键是否仍按下（拖动残留清理） |
| `ui.IsHovered(key)` / `ui.IsPressed(key)` / `ui.IsFocused(key)` | 按 Key 查询交互态 |
| `ui.LastFrameOf(key)` | 上一帧该节点的窗口坐标矩形（浮层按锚点定位） |
| `ui.ScrollOffset(key)` / `ui.ScrollTo` / `ui.ScrollBy` | 滚动容器偏移读写 |
| `ui.SetFocus(key)` / `FocusedKey()` | 键盘焦点控制 |
| `ui.AnimTime()` | 启动至今秒数（自绘动画用） |
| `ui.TextWidth` / `TextWidthOf` / `TextGlyphBounds` | 文本度量（UI 线程） |
| `ui.SetTooltip(text, x, y)` | 手动设置悬停提示 |
| `ui.OnKeyGlobal(fn)` | 全局按键回调（任何节点聚焦都能收到，F11 / ESC 常用） |
| `ui.App.Invalidate()` | 跨 goroutine 请求重绘 |

窗口自身尺寸变化不需要监听：`ViewportW/H` 每帧都是最新的，`view()` 里直接分支即可。

## 7. 帧循环与性能约定

* 只有「有交互 / 有动画 / 状态变化」时才重绘；静态页零 CPU。
* 标了 `AnimOn()` 的节点每帧重绘；`AnimSlowOn()` 按 ~30Hz 重绘（光标闪烁、骨架屏流光）。
* 组件栅格缓存（`ui.SetCacheLayers(true)` 默认开）：纯静态、不透明底色、无圆角/描边/阴影的最外层容器
  首次绘制后快照成纹理，后续帧一张图贴回。含交互控件 / 动画 / 图片 / 图表 / 视频 / 网页的子树会被自动排除。
* 最小化时停止渲染；恢复时 `ResumeMediaAll()` 自动重建媒体解码窗口。

## 8. 崩溃兜底

```go
ui.SetCrashDir(`D:\logs\crash`)          // 默认 <用户配置目录>/guis/crashes
ui.SetCrashHandler(func(r ui.CrashReport) { log.Println(r.Time, r.Message) })
defer ui.RecoverToReport("worker")        // 业务 goroutine 里自带 recover + 落盘
ui.CrashPanel()                           // 现成的崩溃摘要面板节点
ui.CrashLast()                            // 最近一次崩溃（含 Stack / File）
```
