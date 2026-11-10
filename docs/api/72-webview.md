<!-- group: 系统能力 -->
# WebView 内嵌浏览器 · WebView2 / iframe

## 1. 平台实现

| 平台 | 实现 | 可用性 |
| --- | --- | --- |
| Windows | WebView2（Edge 内核，`internal/win32/webview2.go` 纯 syscall/purego 绑定） | 需安装 WebView2 Runtime |
| 浏览器 | `<iframe>` 叠加 | 始终可用 |
| Linux / macOS / Android | 占位桩（记录状态、不出画面） | 不可用 |

```go
ui.WebViewAvailable()   // 当前平台 + 运行时是否可用；false 时组件降级为占位卡片
```

## 2. 创建

```go
wv, err := ui.NewWebView(ui.WebViewOptions{
    URL:         "https://go.dev",
    HTML:        "",                       // 优先于 URL（离线页面 / 自绘模板）
    UserDataDir: "",                       // 空 = %LOCALAPPDATA%\GUIS\WebView2
    Zoom:        1.0,
    Transparent: false,
    OnChange:    func() { ui.RequestRedraw() },   // 加载开始/完成、地址、标题、历史变化
})
if err != nil { /* 没有 WebView2 运行时：降级为占位卡片 */ }
defer wv.Close()
```

必须在 **UI 线程**（按钮回调里）创建。多窗口场景用 `ui.NewWebViewIn(app, opts)`。

## 3. 组合成浏览器界面

```go
ui.Column(
    ui.WebViewBar(wv, ui.WebViewBarOptions{
        Address:   addr,
        Key:       "addr",
        OnAddress: func(v string) { addr = v },
        OnGo:      func() { wv.Navigate(addr) },
        ShowTitle: true,
    }),
    ui.WebViewStatus(wv),
    ui.WebViewNode(wv).St(ui.St().GrowTo(1)),   // 网页区域，尺寸由布局决定
).St(ui.St().Hp(100))
```

`ui.Browser(wv)` 是「工具条 + 页面区域」的一体化封装。

## 4. WebView 方法

```go
wv.Ready(); wv.Err()
wv.Navigate(url); wv.LoadHTML(html)
wv.Back(); wv.Forward(); wv.Reload(); wv.Stop()
wv.Focus()
wv.SetRadius(12)          // 网页区域圆角（逻辑像素，>0 用窗口区域真裁剪）
wv.SetZoom(1.25)
wv.ExecuteScript("document.title")
wv.URL(); wv.Title(); wv.Loading()
wv.CanGoBack(); wv.CanGoForward()
wv.Close()
```

## 5. 渲染与层级限制

* 组件按节点矩形**同步浏览器子窗口位置**（每帧收集矩形 → 按 DPI 换算物理像素），
  滚出视口 / 切页自动隐藏。
* 圆角用 `SetWindowRgn` 做真裁剪，可随 DPI / 尺寸重建。
* **HWND 托管的网页永远在 GL 内容之上**：无法被自绘浮层（菜单 / 弹窗）遮挡，也不能做 3D 变换。
  常见做法是菜单展开时临时隐藏网页（gallery 的 `case_webview.go` 就这么处理）。
* 升级路线（B 阶段 · DirectComposition 合成）：`AttachCompose` / `ComposeBegin` / `ComposeEnd`
  已在 `backend_windows.go` 提供，网页改由 CompositionController 托管后可获得同树 z-order
  与精确鼠标转发；当前仍以子窗口叠加为主。

## 6. 浏览器端 iframe 差异

* 跨域页面读不到 `Title`（返回地址）、`ExecuteScript` 会被拒绝。
* `Back / Forward` 依赖浏览器的 iframe 历史栈，行为与桌面略有差异。
* `Stop` 无法中断导航，实现为回到 `about:blank`。

## 7. 安全建议

* 只加载可信页面；`ExecuteScript` 不要在页面里注入密钥。
* `UserDataDir` 是 cookie / 缓存目录，多实例共享会互相影响，建议按窗口区分。
* 加载本地 HTML 用 `HTML` 字段（不落地文件），避免路径穿越问题。

## 8. 示例

`cmd/gallery/pages/case_webview.go`：地址栏 + 前进后退 + 加载指示 + 与菜单互斥的完整示例。
