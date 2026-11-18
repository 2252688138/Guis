<!-- group: 系统能力 -->
# WebView 内嵌浏览器 · 一套 API + 各端后端

## 1. 平台实现

| 平台 | 实现 | 可用性 |
| --- | --- | --- |
| Windows | ① WebView2（Edge 内核，`internal/win32/webview2.go` 纯 syscall/purego 绑定）；② 系统浏览器 CDP（纹理后端） | ① 需 WebView2 Runtime；② 需本机有 Chromium 系浏览器 |
| Linux / X11 | ① 真窗口嵌入（`XReparentWindow` 把浏览器窗口挂进 GUIS 窗口）；② 系统浏览器 CDP（纹理后端） | 需本机有 Chromium 系浏览器；Wayland 下自动降级到 ② |
| macOS | 系统浏览器 CDP（纹理后端，`internal/webcdp`） | 需 Chrome / Chromium / Edge / Brave |
| 浏览器 | `<iframe>` 叠加（`browser-iframe`） | 始终可用 |
| Android / iOS、或以上都没有 | 降级后端（`stub`）：不出画面，维护状态 + 外部浏览器打开 | `WebViewAvailable()` 为 false |

三条桌面路线的取舍（`newWebViewBackend` 按能力自动选择，可用环境变量
`GUIS_WEBVIEW_BACKEND=webview2|embed|cdp|stub` 强制指定）：

| 路线 | 渲染 | 优点 | 代价 |
| --- | --- | --- | --- |
| WebView2 | 子窗口 / 合成 | 最流畅、支持圆角真裁剪 | 仅 Windows，依赖运行时 |
| 真窗口嵌入（X11） | 浏览器自己的窗口 | 帧率与浏览器一致，输入法 / 视频原生 | 永远盖在 GL 画面上，不能压浮层、不能圆角裁剪 |
| CDP 纹理（`system-browser-cdp`） | 浏览器推帧 → GL 纹理 | 可被裁剪 / 圆角 / 被浮层覆盖；跨平台 | 按 CSS 像素取帧（HiDPI 略软），输入有几十毫秒延迟 |

```go
ui.WebViewAvailable()   // 当前平台 + 运行时是否可用；false 时组件降级为占位卡片
wv.Platform()           // "windows-webview2" / "system-browser-x11" / "system-browser-cdp" / "browser-iframe" / "stub"
```

CDP 后端（`internal/webcdp` 纯 Go DevTools 协议客户端 + `ui/webview_cdp.go`）的工作方式：

- 启动系统浏览器：`--headless=new --remote-debugging-port=<随机端口> --user-data-dir=<临时目录>`，
  用 DevTools `/json/list` 拿页面 WebSocket 地址，自写 WS 客户端（零第三方依赖）。
- 取帧：`Page.startScreencast`（浏览器只在页面变化时推帧，比反复 `captureScreenshot` 省 CPU，
  也不会把命令通道堵死），帧解码成 RGBA 后上传 GL 纹理、按节点矩形绘制。
- 输入：自绘节点把鼠标 / 滚轮 / 按键 / 字符（`Input.insertText`，中文可用）转发给网页。
- 缩放：页面 CSS `zoom`（取帧尺寸不变）；静音 / UA / 执行脚本走 CDP 命令。
- CDP 事件回调里**不能**同步发 CDP 命令（会自己等自己），统一丢后台，见 `refreshMeta`。

代码结构（平台差异只允许出现在后端里）：

| 文件 | 职责 |
| --- | --- |
| `ui/webview_core.go` | 唯一一份 `WebView` / `WebViewOptions` / 公开方法 / 事件 / 历史 / 查找 / 注入 / 消息桥 / 节点构建器 |
| `ui/webview_windows.go` | Windows 后端（WebView2 子窗口 + DPI 换算 + 圆角真裁剪） |
| `ui/webview_cdp.go` | 系统浏览器 CDP 后端（推帧 → 纹理，跨平台，可裁剪 / 圆角 / 被浮层覆盖） |
| `ui/webview_embed_linux.go` | Linux/X11 真窗口嵌入后端（`XReparentWindow` + CDP 控制） |
| `ui/webview_pick_*.go` | 平台取后端顺序（webview2 / embed / cdp / stub）+ `openExternalURL` |
| `ui/webview_wasm.go` | 浏览器后端（iframe 定位 / 跨域降级） |
| `ui/webview_stub.go` | 降级后端（外部浏览器打开） |
| `internal/webcdp/` | 纯 Go DevTools 协议客户端：启动浏览器、WS、页面命令、截图 |
| `internal/x11/embed.go` | X11 嵌入用到的少量绑定（Reparent / QueryTree / 32 位属性） |
| `ui/webview_panel.go` | 多端 UI 组合件：自适应工具条 / 查找栏 / 标签页 / `BrowserPanel` |

> 同一套 UI 代码在四端都能编译；能力差异用 `WebViewAvailable()` / `Platform()` 判断，
> 用 `wv.OpenExternal()` 兜底。

## 2. 创建

```go
wv, err := ui.NewWebView(ui.WebViewOptions{
    URL:         "https://go.dev",
    HTML:        "",                       // 优先于 URL（离线页面 / 自绘模板）
    UserDataDir: "",                       // 空 = 平台默认目录
    Zoom:        1.0,
    Transparent: false,
    OnChange:    func() { ui.RequestRedraw() },   // 加载开始/完成、地址、标题、历史变化

    // ---- 以下全部可选，零值 = 原有行为，后端不支持时自动忽略 ----
    UserAgent:  "",                        // 自定义 UA（Windows 暂只记录，浏览器端受限）
    Muted:      false,                     // 初始静音
    DarkMode:   false,                     // 初始深色（CSS 滤镜注入）
    Radius:     12,                        // 初始圆角（逻辑像素）
    BGColor:    "",                        // 网页底色 "#rrggbb" / "transparent"
    HomeURL:    "https://go.dev",          // GoHome() 目标
    InjectCSS:  "body{padding:0}",         // 文档注入 CSS（导航后自动重放）
    AllowNavigation: func(url string) bool { return !strings.Contains(url, "ads.") },
    OnNavStart: func(url string) {},       // 导航开始
    OnNavDone:  func(url string, ok bool) {}, // 导航完成
    OnTitle:    func(title string) {},
    OnNewWindow: func(url string) bool { return false }, // 预留：返回 true = 宿主自己处理
    OnMessage:  func(data string) { fmt.Println("来自页面:", data) },
    OnError:    func(err error) {},
})
if err != nil { /* Windows 上没有 WebView2 运行时：降级为占位卡片 */ }
defer wv.Close()
```

必须在 **UI 线程**（按钮回调里）创建。多窗口场景用 `ui.NewWebViewIn(app, opts)`。

降级后端（Linux / macOS / 移动端）**不返回错误**：拿到实例后用 `wv.Err()` 查原因，
用 `wv.OpenExternal()` 把链接交给系统浏览器，UI 才会「处处能跑」。

## 3. 组合成浏览器界面

一体化（推荐）：

```go
ui.BrowserPanel(wv, ui.BrowserPanelOptions{
    WebViewBarOptions: ui.WebViewBarOptions{
        Address:     addr,
        Key:         "addr",
        OnAddress:   func(v string) { addr = v },
        OnGo:        func() { wv.Navigate(addr) },
        ShowTitle:   true,
        ShowFind:    true,   // 出现 ⌕ 查找入口
        ShowZoom:    true,   // 出现 − 100% +
        AutoCompact: true,   // 窄屏自动紧凑（隐藏文字按钮、按钮收到 30px）
    },
    ShowStatus: true,
}).St(ui.St().Hp(100))
```

手动拼（与旧代码完全兼容）：

```go
ui.Column(
    ui.WebViewBar(wv, ui.WebViewBarOptions{
        Address:   addr,
        Key:       "addr",
        OnAddress: func(v string) { addr = v },
        OnGo:      func() { wv.Navigate(addr) },
        ShowTitle: true,
    }),
    ui.WebViewFindBar(wv, ui.WebViewFindBarOptions{Key: "addr-find"}),
    ui.WebViewStatus(wv),
    ui.WebViewNode(wv).St(ui.St().GrowTo(1)),   // 网页区域，尺寸由布局决定
).St(ui.St().Hp(100))
```

多标签页（标题由调用方给，切换 / 关闭 / 新建回调自管）：

```go
ui.WebViewTabs(ui.WebViewTabsOptions{
    Labels:   []string{wv.Title(), "新标签页"},
    Active:   active,
    OnSelect: func(i int) { active = i },
    OnClose:  func(i int) { tabs = append(tabs[:i], tabs[i+1:]...) },
    OnAdd:    func() { /* 新建 wv */ },
    Compact:  true,
})
```

## 4. 方法

```go
// 状态
wv.Ready(); wv.Err(); wv.Platform()
wv.URL(); wv.Title(); wv.Loading(); wv.CanGoBack(); wv.CanGoForward()

// 导航
wv.Navigate(url); wv.LoadHTML(html); wv.GoHome()
wv.Back(); wv.Forward(); wv.Reload(); wv.Stop(); wv.Focus()
wv.History(); wv.HistoryIndex(); wv.GoHistory(i)
wv.SetAllowNavigation(func(url string) bool)

// 视图
wv.SetZoom(1.25); wv.ZoomFactor(); wv.ZoomIn(); wv.ZoomOut(); wv.ZoomReset()
wv.SetRadius(12)          // 网页区域圆角（逻辑像素，Windows 用窗口区域真裁剪）
wv.SetMuted(true); wv.Muted()
wv.SetDarkMode(true); wv.DarkMode()
wv.SetUserAgent(ua)

// 脚本与注入
wv.ExecuteScript("document.title")
wv.ExecuteScriptAsync("1+1", func(r string) { log(r) })   // r 是 JS 结果文本
wv.InjectCSS("body{padding:0}"); wv.ClearInjectedCSS()

// 页面内查找
wv.Find("关键词"); wv.FindNext(); wv.FindPrev(); wv.ClearFind(); wv.FindText()

// JS ↔ Go 消息桥
wv.Post("hello"); wv.PostJSON(map[string]any{"a": 1})
wv.OnMessage(func(data string) { fmt.Println(data) })

// 其它
wv.Print(); wv.OpenExternal(); wv.Close()
```

## 5. 消息桥（页面 ⇄ Go）

宿主 → 页面：`wv.Post(data)`（或 `PostJSON`），页面侧：

```html
<script>
  // 可选：页面自己初始化桥（宿主在导航后会自动注入同一份脚本）
  guisOnMessage(function (data) { console.log("from Go:", data); });
  document.addEventListener("guis:message", e => console.log(e.detail));

  // 页面 → Go
  guisSend({ type: "ready", url: location.href });
</script>
```

`ui.WebViewBridgeJS` 是完整桥脚本（`guisSend` / `guisOnMessage` / `__guisRecv` /
`guis:message` 事件），宿主在设置了 `OnMessage` 时自动注入并在导航后重放。
页面发来的消息由 core 每 12 帧拉取一次（不依赖任何平台特有的 Com 消息通道）。

### 预留与限制

* `OnNewWindow` 目前是预留钩子：Windows 后端尚未接 `add_NewWindowRequested`，`<iframe>` 也无法
  拦截 `window.open`。需要「新窗口一律在本页打开」时，用 `SetAllowNavigation` 做白名单，
  或在 `OnNavStart` 里自行改写目标。
* `OnError` 由 core 每帧对比后端错误触发（只在上报状态变化时回调一次）。
* `Generate / 打印` 走页面自身的 `window.print()`：Windows 会弹页面的打印对话框，浏览器端交给浏览器。
* `UserAgent`：Windows 记录待接（WebView2 settings 槽位尚未绑定），浏览器端受同源策略限制，
  stub 端只记录 —— 三端都不会因此报错。

## 6. 渲染与层级限制（Windows）

* 组件按节点矩形**同步浏览器子窗口位置**（每帧收集矩形 → 按 DPI 换算物理像素），
  滚出视口 / 切页自动隐藏。
* 圆角用 `SetWindowRgn` 做真裁剪，可随 DPI / 尺寸重建。
* **HWND 托管的网页永远在 GL 内容之上**：无法被自绘浮层（菜单 / 弹窗）遮挡，也不能做 3D 变换。
  常见做法是菜单展开时临时隐藏网页（gallery 的 `case_webview.go` 就这么处理）。
* 升级路线（B 阶段 · DirectComposition 合成）：`AttachCompose` / `ComposeBegin` / `ComposeEnd`
  已在 `backend_windows.go` 提供，本轮未改变该路线。

## 7. 浏览器端 iframe 差异

* 跨域页面读不到 `Title`（回退为地址）、`ExecuteScript` / `Find` / `InjectCSS` /
  `SetMuted` 会失败（返回 false / 空串，不 panic）。
* `Back / Forward` 依赖自建地址栈，行为与桌面略有差异。
* `UserAgent` 无法修改（浏览器安全策略），只记录在 `data-guis-ua` 属性上。

## 8. 能力矩阵

| 能力 | Windows(WebView2) | CDP 纹理（三端） | X11 真窗口嵌入 | 浏览器(iframe) | Android/iOS(stub) |
| --- | --- | --- | --- | --- | --- |
| 网页渲染 | ✅ | ✅ | ✅ | ✅ | ❌ 占位卡片 |
| 导航 / 前进后退 / 刷新 | ✅ | ✅ | ✅ | ✅ | ✅ 记录状态 |
| 查找 / 注入 CSS / 深色 / 缩放 / 静音 | ✅ | ✅ | ✅ | ✅（同源） | ❌ 记录状态 |
| 鼠标 / 键盘 / 中文输入 | ✅ 原生 | ✅ 转发 | ✅ 原生 | ✅ 原生 | ❌ |
| 被自绘浮层遮挡 / 圆角裁剪 | ✅ 合成后 | ✅ | ❌ | ✅ | — |
| JS → Go 消息桥 | ✅ | ✅ | ✅ | ✅（同源） | ❌ |
| Go → 页面 Post | ✅ | ✅ | ✅ | ✅（同源） | ❌ |
| 外部浏览器打开 | ✅ | ✅ | ✅ | ✅（新标签页） | ✅ `xdg-open` / `open` |
| `OnNewWindow` 新窗口钩子 | ⏳ 预留 | ✅ 同窗口导航 | ✅ 同窗口导航 | ⏳ 预留 | — |
| `OnError` 错误上报 | ✅ | ✅ | ✅ | ✅ | ✅ |
