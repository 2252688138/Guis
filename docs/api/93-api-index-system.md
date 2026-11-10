<!-- group: 附录 -->
# 附录 D · 系统能力与平台差异 API 速查

系统级能力（剪贴板 / 热键 / 托盘 / 拖入 / 自启 / 多屏 / 截图 / 单实例 / 更新）、
WebView 平台桩、媒体平台桩以及各平台窗口后端的导出符号。

> 说明：本附录为**自动生成**的完整符号清单，按源文件分组；
> 想按「用途」查找请先读前面的正文章节，本附录保证**没有任何导出 API 被漏掉**。

| 源文件 | 符号数 |
| --- | --- |
| `ui/system.go` | 14 |
| `ui/system_win32.go` | 9 |
| `ui/system_extra_windows.go` | 13 |
| `ui/system_extra_other.go` | 13 |
| `ui/autostart.go` | 4 |
| `ui/prefs.go` | 12 |
| `ui/update.go` | 9 |
| `ui/print.go` | 16 |
| `ui/webview.go` | 28 |
| `ui/webview_stub.go` | 28 |
| `ui/webview_wasm.go` | 28 |
| `ui/media_wasm.go` | 25 |
| `ui/music_stub.go` | 31 |
| `ui/media_stub.go` | 23 |
| `ui/backend_windows.go` | 10 |
| `ui/backend_x11.go` | 32 |
| `ui/backend_darwin.go` | 34 |
| `ui/backend_ios.go` | 34 |
| `ui/backend_android.go` | 34 |
| `ui/backend_wasm.go` | 35 |
| `ui/backend_null.go` | 34 |
| `ui/hotreload.go` | 2 |
| `ui/hotreload_stub.go` | 2 |

合计 **470** 个导出符号。

---

## `ui/system.go`

- **函数** `func SetOnDropFiles(fn func(paths []string, x, y float32))` — SetOnDropFiles 给当前所有窗口设置应用级文件拖入回调。
- **函数** `func SetAcceptDrops(on bool)` — SetAcceptDrops 开启/关闭当前窗口接收文件拖入（默认开启）。
- **函数** `func DropZone(key, hint string, height float32, onDrop func(paths []string)) *Node` — DropZone 文件拖入区域：把文件/文件夹拖进框内松手即触发 onDrop。
- **函数** `func FormatBytes(n int64) string` — FormatBytes 把字节数格式化成 1.2 MB 这样的可读文本。
- **函数** `func FileRow(path string, onClick func()) *Node` — FileRow 拖入结果的一行：文件名 + 大小 + 所在目录。
- **函数** `func SetAlwaysOnTop(on bool)` — SetAlwaysOnTop 窗口置顶 / 取消置顶。
- **函数** `func IsAlwaysOnTop() bool` — IsAlwaysOnTop 当前窗口是否置顶。
- **函数** `func RegisterHotkey(mods uint32, vk uint32, fn func()) int32` — RegisterHotkey 注册系统级热键（窗口最小化/失焦时同样触发）。
- **函数** `func UnregisterHotkey(id int32)` — UnregisterHotkey 注销热键。
- **常量** `const HotAlt = 0x0001` — 全局热键修饰键（RegisterHotkey 的 mods 参数）。
- **常量** `const HotCtrl = 0x0002` — 全局热键修饰键（RegisterHotkey 的 mods 参数）。
- **常量** `const HotShift = 0x0004` — 全局热键修饰键（RegisterHotkey 的 mods 参数）。
- **常量** `const HotWin = 0x0008` — 全局热键修饰键（RegisterHotkey 的 mods 参数）。
- **常量** `const HotNoRepeat = 0x4000` — 全局热键修饰键（RegisterHotkey 的 mods 参数）。

## `ui/system_win32.go`

- **函数** `func PickFiles(title, filter string) ([]string, bool)` — PickFiles 打开多选文件对话框（filter 形如 "图片|*.png;*.jpg|所有文件|*.*"）。
- **函数** `func ClipboardText() string` — ClipboardText 读取剪贴板文本（无文本返回空串）。
- **函数** `func ClipboardSetText(s string) bool` — ClipboardSetText 写入剪贴板文本。
- **函数** `func ClipboardSetFiles(paths []string) bool` — ClipboardSetFiles 把文件路径列表写入剪贴板（CF_HDROP，可直接粘贴到资源管理器）。
- **函数** `func SetKeepAwake(on bool) uint32` — SetKeepAwake 阻止系统休眠与关屏（false = 恢复默认）。返回调用前的标志位。
- **函数** `func RevealInExplorer(path string) bool` — RevealInExplorer 在资源管理器中定位并选中给定路径。
- **函数** `func MoveToRecycleBin(path string) bool` — MoveToRecycleBin 把文件/文件夹移入回收站（不弹确认框，可从回收站还原）。
- **函数** `func SystemDarkMode() bool` — SystemDarkMode 系统是否处于深色模式（读注册表 AppsUseLightTheme）。
- **方法** `func Tray.Notify(title, msg string)` — Notify 在托盘图标上弹出气泡提示（title 可空）。

## `ui/system_extra_windows.go`

- **类型** `type Monitor win32.MonitorInfo` — Monitor 显示器信息。
- **类型** `type ClipboardInfo win32.ClipboardInfo` — ClipboardInfo 剪贴板快照。
- **函数** `func Monitors() []Monitor` — Monitors 枚举显示器（主屏在前）。
- **函数** `func MonitorAt(x, y int) (Monitor, bool)` — MonitorAt 返回包含指定屏幕坐标的显示器。
- **函数** `func VirtualScreen() (int, int, int, int)` — VirtualScreen 所有显示器并集（x, y, w, h）。
- **函数** `func CaptureScreen(x, y, w, h int, cursor bool) (*image.NRGBA, error)` — CaptureScreen 截取屏幕区域（物理像素，支持负坐标 / 多屏）。cursor 预留，
- **函数** `func CaptureFullScreen() (*image.NRGBA, error)` — CaptureFullScreen 截取主屏。
- **函数** `func Clipboard() ClipboardInfo` — Clipboard 读取剪贴板摘要。
- **函数** `func ClipboardImage() (*image.NRGBA, error)` — ClipboardImage 读取剪贴板位图。
- **函数** `func WatchClipboard(fn func(ClipboardInfo)) (stop func())` — WatchClipboard 监听剪贴板变化（500ms 轮询序号，变化时回调）。
- **函数** `func SingleInstance(key string) bool` — SingleInstance 申请单实例锁。返回 false 表示已有实例在运行，此时调用
- **函数** `func NotifyFirstInstance(key string, args []string)` — NotifyFirstInstance 把本次启动参数发给已运行实例。
- **函数** `func WatchSecondInstance(key string, fn func(args []string)) (stop func())` — WatchSecondInstance 轮询二次启动参数（首个实例调用），args 非空时回调。

## `ui/system_extra_other.go`

- **类型** `type Monitor struct { Index int Name string X, Y int W, H int WorkX int WorkY int WorkW int WorkH int Scale float32 Primary bool }` — Monitor 显示器信息（非 Windows 端字段保留，值由各端自行填充或为空）。
- **类型** `type ClipboardInfo struct { Seq uint32 Text string HasText bool HasImage bool HasFiles bool }` — ClipboardInfo 剪贴板快照。
- **函数** `func Monitors() []Monitor` — Monitors 非 Windows 端返回空（X11 可后续接 XRandR）。
- **函数** `func MonitorAt(x, y int) (Monitor, bool)` — MonitorAt 非 Windows 端恒不支持。
- **函数** `func VirtualScreen() (int, int, int, int)` — VirtualScreen 非 Windows 端返回零值。
- **函数** `func CaptureScreen(x, y, w, h int, cursor bool) (*image.NRGBA, error)` — CaptureScreen 非 Windows 端暂不支持。
- **函数** `func CaptureFullScreen() (*image.NRGBA, error)` — CaptureFullScreen 非 Windows 端暂不支持。
- **函数** `func Clipboard() ClipboardInfo` — Clipboard 读取剪贴板摘要（非 Windows 仅文本）。
- **函数** `func ClipboardImage() (*image.NRGBA, error)` — ClipboardImage 非 Windows 端不支持。
- **函数** `func WatchClipboard(fn func(ClipboardInfo)) (stop func())` — WatchClipboard 非 Windows 端用轮询文本变化近似实现。
- **函数** `func SingleInstance(key string) bool` — SingleInstance 非 Windows 端不做锁，恒视为首个实例。
- **函数** `func NotifyFirstInstance(key string, args []string)` — NotifyFirstInstance 非 Windows 端空实现。
- **函数** `func WatchSecondInstance(key string, fn func(args []string)) (stop func())` — WatchSecondInstance 非 Windows 端空实现。

## `ui/autostart.go`

- **类型** `type AutoStartSpec struct { // Name 注册项名称：Windows 注册表值名 / Linux .desktop 文件名 / macOS plist 文件名。 // 为空时取可执行文件名（不含扩展名）；非安全字符会归一化成 '-'。 Name string // Args 启动参数（例如 "--tray"：开机后直接驻留托盘，不弹主窗口）。 Args []string // Comment 描述：Linux .desktop 的 Name 列 / macOS plist 的用途说明，可空。 Comment string }` — AutoStartSpec 开机自启的注册信息。
- **函数** `func AutoStartSupported() bool` — AutoStartSupported 当前平台是否支持开机自启（等价 HasCap(CapAutoStart)）。
- **函数** `func SetAutoStart(on bool, spec AutoStartSpec) error` — SetAutoStart 开启 / 关闭开机自启（当前用户级，无需管理员权限）。
- **函数** `func AutoStartEnabled(spec AutoStartSpec) bool` — AutoStartEnabled 是否已注册开机自启（直接读注册表 / .desktop / plist）。

## `ui/prefs.go`

- **函数** `func SetPref(key, val string)` — SetPref 写入一项偏好（值统一按字符串保存）。
- **函数** `func GetPref(key string) (string, bool)` — GetPref 读取一项偏好。
- **函数** `func DelPref(key string)` — DelPref 删除一项偏好。
- **函数** `func GetPrefFloat(key string, def float32) float32` — GetPrefFloat 读取浮点偏好（缺失或解析失败时返回 def）。
- **函数** `func GetPrefInt(key string, def int) int` — GetPrefInt 读取整数偏好。
- **函数** `func GetPrefBool(key string, def bool) bool` — GetPrefBool 读取布尔偏好（"1"/"true" 为真）。
- **函数** `func SetPrefFloat(key string, v float32)` — SetPrefFloat 写入浮点偏好。
- **函数** `func SetPrefInt(key string, v int)` — SetPrefInt 写入整数偏好。
- **函数** `func SetPrefBool(key string, v bool)` — SetPrefBool 写入布尔偏好。
- **函数** `func PrefsPath() string` — PrefsPath 偏好文件路径（Web 端为空，表示用 localStorage）。
- **函数** `func SavePrefs() error` — SavePrefs 立即把偏好写入磁盘（原子替换）。
- **函数** `func ResetPrefs()` — ResetPrefs 清空内存中的偏好（测试 / 恢复默认设置）。

## `ui/update.go`

- **类型** `type UpdateConfig struct { // Current 当前版本（"1.2.3"）。 Current string // CheckURL 返回 JSON：{"version":"1.3.0","notes":"...","url":"https://...","sha256":"...","size":123} // 也支持纯文本版本号（此时下载地址用 DownloadURL）。 CheckURL string // DownloadURL 纯文本检查模式下的下载地址。 DownloadURL string }` — UpdateConfig 更新配置。
- **类型** `type UpdateInfo struct { Version string `json:"version"` Notes string `json:"notes"` URL string `json:"url"` SHA256 string `json:"sha256"` Size int64 `json:"size"` }` — UpdateInfo 可用更新。
- **函数** `func CheckUpdate(cfg UpdateConfig) (*UpdateInfo, error)` — CheckUpdate 检查更新；无更新返回 (nil, nil)。
- **函数** `func CompareVersion(a, b string) int` — CompareVersion 语义化版本比较：a>b 返回 1，a<b 返回 -1，相等返回 0。
- **函数** `func DownloadUpdate(info *UpdateInfo, dst string, onProgress func(done, total int64)) error` — DownloadUpdate 下载更新到 dst，onProgress 可为 nil。
- **函数** `func VerifySHA256(path, want string) error` — VerifySHA256 校验文件摘要（空 want 直接通过）。
- **函数** `func ApplyUpdate(newExe string) error` — ApplyUpdate 用新文件替换当前可执行文件；Windows 上通过重命名 + 重启脚本完成。
- **函数** `func UpdateCheck(cfg UpdateConfig)` — UpdateCheck 后台检查更新（结果下一帧生效）。
- **函数** `func UpdatePanelView(cfg UpdateConfig) *Node` — UpdatePanelView 渲染更新面板。

## `ui/print.go`

- **类型** `type PrintCanvas struct { W, H float32 cmds []printCmd // scale 打印时的设备像素 / 逻辑像素 scale float32 dc uintptr live bool }` — PrintCanvas 页面绘制画布（打印与预览共用同一套坐标）。
- **类型** `type Printable interface { Pages() int DrawPage(c *PrintCanvas, page int) }` — Printable 可打印文档。
- **函数** `func Print(title string, doc Printable) (bool, error)` — Print 弹出系统打印对话框并打印文档；用户取消返回 (false, nil)。
- **函数** `func PrintPreview(key string, doc Printable, zoom float32) *Node` — PrintPreview 打印预览（按 A4 比例渲染页面卡片，可缩放）。
- **函数** `func Printers() []string` — Printers 可用打印机名（默认打印机排第一位；非 Windows / Web 端返回 nil）。
- **函数** `func DefaultPrinter() string` — DefaultPrinter 默认打印机名（没有则空串）。
- **函数** `func PrintTo(printer, title string, doc Printable) (bool, error)` — PrintTo 不弹系统对话框，直接打印到指定打印机。
- **函数** `func PrintDialogOpen() bool` — PrintDialogOpen 当前是否有内置打印对话框在显示。
- **函数** `func OpenPrintDialog(title string, doc Printable, reason string)` — OpenPrintDialog 打开内置打印对话框（打印机列表 + 打印 / 取消）。
- **函数** `func ClosePrintDialog()` — ClosePrintDialog 关闭内置打印对话框。
- **函数** `func PrintDialogLayer() *Node` — PrintDialogLayer 内置打印对话框浮层（挂在根 Stack 上；未打开时返回 nil）。
- **方法** `func PrintCanvas.Text(x, y, size float32, s, color string)` — Text 绘制文本（x/y 为左上角）。
- **方法** `func PrintCanvas.Rect(x, y, w, h, stroke float32, strokeColor, fill string)` — Rect 绘制矩形（fill 为空只描边；stroke<=0 只填充）。
- **方法** `func PrintCanvas.Line(x1, y1, x2, y2, width float32, color string)` — Line 绘制直线。
- **常量** `const PageW96 = 794` — A4 页面逻辑尺寸（96 DPI）。
- **常量** `const PageH96 = 1123` — A4 页面逻辑尺寸（96 DPI）。

## `ui/webview.go`

- **类型** `type WebView struct { core *win32.WebView2 app *App // 上一帧的逻辑矩形与可见状态（调试 / 命中测试用） rect geom.Rect visible bool // radius 网页区域圆角（逻辑像素；>0 时用窗口区域真裁剪，而非画圆角） radius float32 // 合成模式（B 阶段）：网页是 DComp 图层，位置/裁剪走视觉属性 compose bool compVisual uintptr }` — WebView 内嵌浏览器节点（系统 WebView2 / Edge 内核）。
- **类型** `type WebViewOptions struct { // URL 初始地址（http/https/file 均可）。 URL string // HTML 直接加载 HTML（优先于 URL，适合离线页面 / 自绘模板）。 HTML string // UserDataDir 用户数据目录（cookie / 缓存），空 = %LOCALAPPDATA%\GUIS\WebView2。 UserDataDir string // Zoom 初始缩放（1 = 100%）。 Zoom float64 // Transparent 背景透明（网页自身透明处透出下层自绘内容）。 Transparent bool // OnChange 页面状态变化（加载开始/完成、地址、标题、历史）时回调。 OnChange func() }` — WebViewOptions 创建内嵌浏览器参数。
- **类型** `type WebViewBarOptions struct { // Address 地址栏当前文本（由调用方持有）。 Address string // Key 地址输入框的 key（同一界面内唯一）。 Key string // OnAddress 地址文本变化。 OnAddress func(string) // OnGo 回车或点击「打开」时触发（通常调 wv.Navigate(addr)）。 OnGo func() // ShowTitle 在地址栏右侧显示页面标题。 ShowTitle bool }` — WebViewBarOptions 工具条参数。
- **函数** `func NewWebView(opts WebViewOptions) (*WebView, error)` — NewWebView 在当前窗口创建内嵌浏览器（须在 UI 线程调用，如按钮回调里）。
- **函数** `func NewWebViewIn(a *App, opts WebViewOptions) (*WebView, error)` — NewWebViewIn 在指定窗口创建内嵌浏览器。
- **函数** `func WebViewAvailable() bool` — Available WebView2 运行时是否可用（未安装时组件降级为占位卡片）。
- **函数** `func Browser(wv *WebView) *Node` — Browser WebView 节点（网页区域的占位节点；真实像素由浏览器子窗口绘制）。
- **函数** `func WebViewNode(wv *WebView) *Node` — WebViewNode WebView 节点，用法同 ui.Video(p)：尺寸由样式决定，
- **函数** `func WebViewBar(wv *WebView, o WebViewBarOptions) *Node` — WebViewBar 自绘浏览器工具条：← → ↻ + 地址栏 + 打开 + 加载指示。
- **函数** `func WebViewStatus(wv *WebView) *Node` — WebViewStatus 页面状态行：地址 / 标题 / 加载中提示。
- **方法** `func WebView.Ready() bool` — Ready 浏览器内核是否已就绪。
- **方法** `func WebView.Err() error` — Err 创建错误（无则 nil）。
- **方法** `func WebView.Navigate(url string)` — Navigate 打开地址。
- **方法** `func WebView.LoadHTML(html string)` — LoadHTML 加载一段 HTML。
- **方法** `func WebView.Back()` — Back / Forward / Reload / Stop 导航控制。
- **方法** `func WebView.Forward()`
- **方法** `func WebView.Reload()`
- **方法** `func WebView.Stop()`
- **方法** `func WebView.Focus()` — Focus 把焦点交回网页。
- **方法** `func WebView.SetRadius(r float32)` — SetRadius 网页区域圆角（逻辑像素，>0 生效）。
- **方法** `func WebView.SetZoom(z float64)` — SetZoom 缩放（1 = 100%）。
- **方法** `func WebView.ExecuteScript(js string)` — ExecuteScript 在页面执行 JS。
- **方法** `func WebView.URL() string` — URL 当前地址。
- **方法** `func WebView.Title() string` — Title 当前页面标题。
- **方法** `func WebView.Loading() bool` — Loading 是否正在加载。
- **方法** `func WebView.CanGoBack() bool` — CanGoBack / CanGoForward 历史状态。
- **方法** `func WebView.CanGoForward() bool`
- **方法** `func WebView.Close()` — Close 关闭浏览器（窗口销毁时会自动调用，也可手动提前释放）。

## `ui/webview_stub.go`

- **类型** `type WebViewOptions struct { URL string HTML string UserDataDir string Zoom float64 Transparent bool OnChange func() }` — WebViewOptions 内嵌浏览器选项（字段与 Windows 端一致）。
- **类型** `type WebView struct { url string html string title string loading bool visible bool zoom float64 radius float32 }` — WebView 内嵌浏览器（本平台占位：记录状态但不产生画面）。
- **类型** `type WebViewBarOptions struct { Address string Key string OnAddress func(string) OnGo func() ShowTitle bool }` — WebViewBarOptions 地址栏选项（字段与 Windows 端一致）。
- **函数** `func NewWebView(opts WebViewOptions) (*WebView, error)` — NewWebView 在当前窗口创建内嵌浏览器（本平台返回不支持）。
- **函数** `func NewWebViewIn(a *App, opts WebViewOptions) (*WebView, error)` — NewWebViewIn 在指定窗口创建内嵌浏览器（本平台返回不支持）。
- **函数** `func WebViewAvailable() bool` — WebViewAvailable 当前平台是否可用内嵌浏览器。
- **函数** `func Browser(wv *WebView) *Node` — Browser 浏览器组合节点（地址栏 + 页面区域）。
- **函数** `func WebViewNode(wv *WebView) *Node` — WebViewNode 页面区域节点（本平台渲染占位卡片）。
- **函数** `func WebViewBar(wv *WebView, o WebViewBarOptions) *Node` — WebViewBar 自绘浏览器工具条（本平台给出禁用态，保持布局一致）。
- **函数** `func WebViewStatus(wv *WebView) *Node` — WebViewStatus 状态标签（本平台提示不可用）。
- **方法** `func WebView.Ready() bool` — Ready 是否已就绪（占位实现：恒 false）。
- **方法** `func WebView.Err() error` — Err 当前错误。
- **方法** `func WebView.Navigate(url string)` — Navigate 打开地址。
- **方法** `func WebView.LoadHTML(html string)` — LoadHTML 加载 HTML。
- **方法** `func WebView.Back()` — Back 后退（空操作）。
- **方法** `func WebView.Forward()` — Forward 前进（空操作）。
- **方法** `func WebView.Reload()` — Reload 刷新（空操作）。
- **方法** `func WebView.Stop()` — Stop 停止加载（空操作）。
- **方法** `func WebView.Focus()` — Focus 聚焦（空操作）。
- **方法** `func WebView.SetRadius(r float32)` — SetRadius 圆角半径。
- **方法** `func WebView.SetZoom(z float64)` — SetZoom 缩放。
- **方法** `func WebView.ExecuteScript(js string)` — ExecuteScript 注入脚本（空操作）。
- **方法** `func WebView.URL() string` — URL 当前地址。
- **方法** `func WebView.Title() string` — Title 页面标题。
- **方法** `func WebView.Loading() bool` — Loading 是否加载中。
- **方法** `func WebView.CanGoBack() bool` — CanGoBack 能否后退。
- **方法** `func WebView.CanGoForward() bool` — CanGoForward 能否前进。
- **方法** `func WebView.Close()` — Close 关闭浏览器。

## `ui/webview_wasm.go`

- **类型** `type WebViewOptions struct { URL string HTML string UserDataDir string Zoom float64 Transparent bool OnChange func() }` — WebViewOptions 内嵌浏览器选项。
- **类型** `type WebView struct { url string html string title string loading bool visible bool zoom float64 radius float32 id string el js.Value funcs []js.Func stack []string idx int }` — WebView 浏览器端内嵌浏览器（<iframe>）。
- **类型** `type WebViewBarOptions struct { Address string Key string OnAddress func(string) OnGo func() ShowTitle bool }` — WebViewBarOptions 地址栏选项。
- **函数** `func NewWebView(opts WebViewOptions) (*WebView, error)` — NewWebView 在当前窗口创建内嵌浏览器。
- **函数** `func NewWebViewIn(a *App, opts WebViewOptions) (*WebView, error)` — NewWebViewIn 在指定窗口创建内嵌浏览器。
- **函数** `func WebViewAvailable() bool` — WebViewAvailable 浏览器端始终可用。
- **函数** `func Browser(wv *WebView) *Node` — Browser 浏览器组合节点（地址栏 + 页面区域）。
- **函数** `func WebViewNode(wv *WebView) *Node` — WebViewNode 页面区域节点：真实像素由叠加的 <iframe> 绘制。
- **函数** `func WebViewBar(wv *WebView, o WebViewBarOptions) *Node` — WebViewBar 自绘浏览器工具条：← → ↻ + 地址栏 + 打开 + 新标签。
- **函数** `func WebViewStatus(wv *WebView) *Node` — WebViewStatus 状态标签。
- **方法** `func WebView.Ready() bool` — Ready 是否已就绪。
- **方法** `func WebView.Err() error` — Err 当前错误。
- **方法** `func WebView.Navigate(url string)` — Navigate 打开地址。
- **方法** `func WebView.LoadHTML(html string)` — LoadHTML 加载 HTML 片段（iframe srcdoc）。
- **方法** `func WebView.Back()` — Back 后退。
- **方法** `func WebView.Forward()` — Forward 前进。
- **方法** `func WebView.Reload()` — Reload 刷新。
- **方法** `func WebView.Stop()` — Stop 停止加载（iframe 无法中断导航，移除 srcdoc/src 之外的等效做法是置 about:blank）。
- **方法** `func WebView.Focus()` — Focus 聚焦到网页。
- **方法** `func WebView.SetRadius(r float32)` — SetRadius 圆角半径。
- **方法** `func WebView.SetZoom(z float64)` — SetZoom 缩放（CSS zoom）。
- **方法** `func WebView.ExecuteScript(js string)` — ExecuteScript 注入脚本（跨域会被浏览器拒绝，调用方需自行保证同源）。
- **方法** `func WebView.URL() string` — URL 当前地址。
- **方法** `func WebView.Title() string` — Title 页面标题（跨域读不到，返回地址）。
- **方法** `func WebView.Loading() bool` — Loading 是否加载中。
- **方法** `func WebView.CanGoBack() bool` — CanGoBack 能否后退。
- **方法** `func WebView.CanGoForward() bool` — CanGoForward 能否前进。
- **方法** `func WebView.Close()` — Close 关闭浏览器（移除 DOM 元素）。

## `ui/media_wasm.go`

- **类型** `type MediaInfo struct { Path string Format string Duration time.Duration Width int Height int FPS float64 Bitrate int64 VideoCodec string AudioCodec string SampleRate int Channels int HasVideo bool HasAudio bool // Backend 实际使用的解码后端；浏览器端恒为 "browser"。 Backend string }` — MediaInfo 媒体探测结果（字段与桌面端一致）。
- **类型** `type MediaConfig struct { FFmpeg string FFprobe string MaxWidth int MaxHeight int FPS float64 NoAudio bool Volume float32 HWAccel bool }` — MediaConfig 解码参数（浏览器端只用到 NoAudio / Volume）。
- **类型** `type MediaSource struct { path string cfg MediaConfig el js.Value id string info MediaInfo err error ready bool funcs []js.Func // 最近一次 seek 的时间与目标，拖动进度条时做节流 lastSeekAt float64 lastSeekTo time.Duration // wantVol / wantMuted 业务期望的音量；浏览器自动播放策略要求「静音起播」， // 所以 play() 成功后再按期望值恢复（见 MediaPlay / applyWanted）。 wantVol float32 wantMuted bool // hls hls.js 实例（播放 .m3u8 时创建；Safari 等原生支持 HLS 的浏览器为空） hls js.Value }` — MediaSource 媒体源：一个 <video> 元素 + 元数据缓存。
- **函数** `func IsMediaPath(path string) bool` — IsMediaPath 按扩展名判断是否媒体文件（blob:/data: 地址按视频处理）。
- **函数** `func MediaFormats() []string` — MediaFormats 浏览器端支持的容器列表（界面展示用）。
- **函数** `func DecoderPreference() string` — DecoderPreference 解码器偏好（浏览器端固定走原生解码）。
- **函数** `func ProbeMedia(path string) (*MediaInfo, error)` — ProbeMedia 探测媒体信息（浏览器端只填路径与容器名，尺寸/时长在元数据到达后回填）。
- **函数** `func ProbeMediaWith(path, ffprobe string) (*MediaInfo, error)` — ProbeMediaWith 指定 ffprobe 路径探测（浏览器端忽略该参数）。
- **函数** `func OpenMedia(path string) (*MediaSource, error)` — OpenMedia 打开媒体（mp4 / webm / m3u8 / blob: / data:）。
- **函数** `func OpenMediaWith(path string, cfg MediaConfig) (*MediaSource, error)` — OpenMediaWith 按配置打开媒体。
- **方法** `func MediaSource.Info() MediaInfo` — Info 探测信息（元数据到达后包含尺寸与时长）。
- **方法** `func MediaSource.Path() string` — Path 媒体路径。
- **方法** `func MediaSource.Size() (int, int)` — Size 画面尺寸（元数据未知时 0,0）。
- **方法** `func MediaSource.Duration() time.Duration` — Duration 时长（未知返回 0）。
- **方法** `func MediaSource.HasAudio() bool` — HasAudio 是否有音轨。
- **方法** `func MediaSource.Err() error` — Err 打开/解码错误。
- **方法** `func MediaSource.Frame(pos time.Duration) (*VideoFrame, error)` — Frame 取指定时间点画面：浏览器端画面由 DOM 元素显示，这里只负责把
- **方法** `func MediaSource.MediaResume()` — MediaResume 最小化恢复后重建解码器（浏览器端无需处理）。
- **方法** `func MediaSource.MediaClock() (time.Duration, bool)` — MediaClock 解码器时钟：以 <video> 的真实播放位置为准（A/V 同步）。
- **方法** `func MediaSource.MediaScrub(v bool)` — MediaScrub 拖动定位状态（浏览器端即时 seek，无需额外处理）。
- **方法** `func MediaSource.MediaPlay(playing bool)` — MediaPlay 播放/暂停。
- **方法** `func MediaSource.MediaVolume(v float32, muted bool)` — MediaVolume 音量与静音（0~1）。播放中立即生效，未播放时先记录。
- **方法** `func MediaSource.MediaSpeed(v float32)` — MediaSpeed 播放速率。
- **方法** `func MediaSource.Close()` — Close 关闭媒体源：停止播放并移除 DOM 元素。
- **常量** `const MediaFilters = "视频|*.mp4;*.m4v;*.mkv;*.mov;*.avi;*.webm;*.flv;*.ts;*.m2ts;*.mts;*.m3u8|" + "音频|*.mp3;*.flac;*.wav;*.m4a;*.aac;*.ogg;*.opus|所有文件|*.*"` — MediaFilters 文件对话框用的媒体过滤器（与桌面端一致）。

## `ui/music_stub.go`

- **类型** `type MusicSource struct { path string info MediaInfo cfg MediaConfig an *Spectrum pos time.Duration // 当前播放位置 last time.Time rate float64 vol float32 muted bool loop bool play bool ended bool }` — MusicSource 纯音频解码源（本平台为占位实现：无声音、按墙钟推进）。
- **函数** `func IsMusicPath(path string) bool` — IsMusicPath 按扩展名判断是否音频文件。
- **函数** `func OpenMusic(path string) (*MusicSource, error)` — OpenMusic 打开音频文件（本平台无解码后端，返回占位源）。
- **函数** `func OpenMusicWith(path string, cfg MediaConfig) (*MusicSource, error)` — OpenMusicWith 按配置打开音频文件（本平台无解码后端，返回占位源）。
- **函数** `func OpenMusicPlayer(path string) (*MusicSource, error)` — OpenMusicPlayer 打开音频并立即播放（本平台无音频输出，仅推进时间轴）。
- **方法** `func MusicSource.Path() string` — Path 音频路径。
- **方法** `func MusicSource.Info() MediaInfo` — Info 探测信息。
- **方法** `func MusicSource.Duration() time.Duration` — Duration 时长（无解码后端 → 0，界面显示 --:--）。
- **方法** `func MusicSource.HasAudio() bool` — HasAudio 是否有音轨。
- **方法** `func MusicSource.Err() error` — Err 错误（占位源无错误）。
- **方法** `func MusicSource.Spectrum() *Spectrum` — Spectrum 频谱分析器（无声源，能量恒为 0）。
- **方法** `func MusicSource.Pos() time.Duration` — Pos 当前播放位置。
- **方法** `func MusicSource.Progress() float32` — Progress 播放进度 0~1。
- **方法** `func MusicSource.Playing() bool` — Playing 是否正在播放。
- **方法** `func MusicSource.Play()` — Play 开始播放。
- **方法** `func MusicSource.Pause()` — Pause 暂停。
- **方法** `func MusicSource.TogglePlay()` — TogglePlay 切换播放/暂停。
- **方法** `func MusicSource.Seek(pos time.Duration)` — Seek 跳转到指定位置。
- **方法** `func MusicSource.SeekRatio(v float32)` — SeekRatio 按比例跳转 0~1。
- **方法** `func MusicSource.Speed() float32` — Speed 播放速率。
- **方法** `func MusicSource.SetSpeed(v float32)` — SetSpeed 设置播放速率（<=0 视为 1）。
- **方法** `func MusicSource.Volume() float32` — Volume 音量 0~1。
- **方法** `func MusicSource.Muted() bool` — Muted 是否静音。
- **方法** `func MusicSource.SetVolume(v float32)` — SetVolume 设置音量。
- **方法** `func MusicSource.ToggleMute()` — ToggleMute 切换静音。
- **方法** `func MusicSource.Loop() bool` — Loop 是否循环。
- **方法** `func MusicSource.SetLoop(v bool)` — SetLoop 设置循环。
- **方法** `func MusicSource.Tick() bool` — Tick 每帧推进；返回 true 表示当前曲目已播完（组件据此自动下一首）。
- **方法** `func MusicSource.Close()` — Close 释放音源。
- **方法** `func MusicSource.NowPlaying() MusicTrack` — NowPlaying 当前曲目信息（标题回退文件名）。
- **常量** `const MusicFilters = "音频|*.mp3;*.flac;*.wav;*.m4a;*.aac;*.ogg;*.opus;*.wma|所有文件|*.*"` — MusicFilters 文件对话框用的音频过滤器。

## `ui/media_stub.go`

- **类型** `type MediaInfo struct { Path string Format string Duration time.Duration Width int Height int FPS float64 Bitrate int64 VideoCodec string AudioCodec string SampleRate int Channels int HasVideo bool HasAudio bool // Backend 实际使用的解码后端；本平台无后端，恒为 ""。 Backend string }` — MediaInfo 媒体探测结果（字段与 Windows 端一致）。
- **类型** `type MediaConfig struct { FFmpeg string FFprobe string MaxWidth int MaxHeight int FPS float64 NoAudio bool Volume float32 HWAccel bool }` — MediaConfig 解码参数（与 Windows 端一致，本平台忽略）。
- **类型** `type MediaSource struct { path string info MediaInfo cfg MediaConfig err error }` — MediaSource 媒体源：占位实现，实现 VideoSource 接口但不产生画面。
- **函数** `func IsMediaPath(path string) bool` — IsMediaPath 按扩展名判断是否媒体文件。
- **函数** `func MediaFormats() []string` — MediaFormats 支持的媒体容器列表（界面展示用）。
- **函数** `func DecoderPreference() string` — DecoderPreference 解码器偏好（本平台无后端）。
- **函数** `func ProbeMedia(path string) (*MediaInfo, error)` — ProbeMedia 探测媒体信息（本平台无后端，返回不支持）。
- **函数** `func ProbeMediaWith(path, ffprobe string) (*MediaInfo, error)` — ProbeMediaWith 指定 ffprobe 路径探测（本平台无后端，返回不支持）。
- **函数** `func OpenMedia(path string) (*MediaSource, error)` — OpenMedia 打开媒体（本平台无后端）。
- **函数** `func OpenMediaWith(path string, cfg MediaConfig) (*MediaSource, error)` — OpenMediaWith 按配置打开媒体（本平台无后端）。
- **方法** `func MediaSource.Info() MediaInfo` — Info 探测信息（零值）。
- **方法** `func MediaSource.Path() string` — Path 媒体路径。
- **方法** `func MediaSource.Size() (int, int)` — Size 画面尺寸（无画面 → 0,0）。
- **方法** `func MediaSource.Duration() time.Duration` — Duration 时长（未知 → 0）。
- **方法** `func MediaSource.HasAudio() bool` — HasAudio 是否有音轨。
- **方法** `func MediaSource.Err() error` — Err 打开/解码错误。
- **方法** `func MediaSource.Frame(pos time.Duration) (*VideoFrame, error)` — Frame 取指定时间点画面（无解码器 → nil, nil，保留上一帧）。
- **方法** `func MediaSource.MediaResume()` — MediaResume 最小化恢复后重建解码器（空操作）。
- **方法** `func MediaSource.MediaClock() (time.Duration, bool)` — MediaClock 解码器时钟（无效）。
- **方法** `func MediaSource.MediaScrub(v bool)` — MediaScrub 拖动定位状态（空操作）。
- **方法** `func MediaSource.MediaPlay(playing bool)` — MediaPlay 播放/暂停（空操作）。
- **方法** `func MediaSource.Close()` — Close 关闭媒体源。
- **常量** `const MediaFilters = "视频|*.mp4;*.m4v;*.mkv;*.mov;*.avi;*.webm;*.flv;*.ts;*.m2ts;*.mts;*.m3u8|" + "音频|*.mp3;*.flac;*.wav;*.m4a;*.aac;*.ogg;*.opus|所有文件|*.*"` — MediaFilters 文件对话框用的媒体过滤器（与 Windows 端一致）。

## `ui/backend_windows.go`

- **方法** `func platformWin.SetEventHandler(fn func(Event))` — SetEventHandler 注册事件回调（App.handleEvent）。
- **方法** `func platformWin.Visible() bool` — Visible 窗口当前是否可见（关闭时收进托盘后为 false，渲染循环据此跳过绘制）。
- **方法** `func platformWin.BlockClose()` — BlockClose 放弃本次 WM_CLOSE：窗口只隐藏、不销毁（关闭时收进托盘）。
- **方法** `func platformWin.AttachCompose(pw, ph int32)` — AttachCompose 尝试启用 DirectComposition 合成（失败则回退子窗口叠加）。
- **方法** `func platformWin.ComposeBegin() uint32` — ComposeBegin 开始一帧合成，返回离屏目标 FBO（0 = 不用合成）。
- **方法** `func platformWin.HasCompose() bool` — HasCompose 是否启用了 DirectComposition 合成。
- **方法** `func platformWin.Compositor() any` — Compositor 返回 DirectComposition 合成器（未启用时为 nil）。
- **方法** `func platformWin.ResizeCompose(pw, ph int32)` — ResizeCompose 尺寸变化时重建合成表面（无合成器时为空操作）。
- **方法** `func platformWin.ComposeEnd()` — ComposeEnd 提交本帧合成。
- **方法** `func platformWin.Destroy()` — Destroy 释放窗口（先关合成器）。

## `ui/backend_x11.go`

- **方法** `func platformWin.SetEventHandler(fn func(Event))`
- **方法** `func platformWin.MakeCurrent()`
- **方法** `func platformWin.Swap()` — Swap 提交一帧。交换失败（表面失效 / 驱动重置）时主动发一次 EvResize 让 App 重建
- **方法** `func platformWin.PixelSize() (int32, int32)`
- **方法** `func platformWin.Scale() float32`
- **方法** `func platformWin.IsMinimized() bool`
- **方法** `func platformWin.SetTitle(s string)`
- **方法** `func platformWin.SetIcon(ico []byte) bool` — SetIcon 设置窗口图标（EWMH _NET_WM_ICON 需要逐像素 ARGB，一期不支持）。
- **方法** `func platformWin.SetVisible(v bool)`
- **方法** `func platformWin.Minimize()`
- **方法** `func platformWin.Restore()`
- **方法** `func platformWin.Center()`
- **方法** `func platformWin.BeginDragMove()` — BeginDragMove 自绘标题栏拖动：X11 下交给窗口管理器（_NET_WM_MOVERESIZE），
- **方法** `func platformWin.ToggleMaximize()`
- **方法** `func platformWin.IsMaximized() bool`
- **方法** `func platformWin.SetFullscreen(v bool)`
- **方法** `func platformWin.IsFullscreen() bool`
- **方法** `func platformWin.SetAlwaysOnTop(on bool)`
- **方法** `func platformWin.AlwaysOnTop() bool`
- **方法** `func platformWin.SetCornerRadius(v float32)`
- **方法** `func platformWin.CornerRadius() float32`
- **方法** `func platformWin.SetAcceptDrops(on bool)`
- **方法** `func platformWin.AcceptDrops() bool`
- **方法** `func platformWin.SetImePosition(x, y int32)` — SetImePosition 输入法候选框位置（需 XIM，一期空实现）。
- **方法** `func platformWin.AttachCompose(pw, ph int32)`
- **方法** `func platformWin.HasCompose() bool`
- **方法** `func platformWin.ResizeCompose(pw, ph int32)`
- **方法** `func platformWin.ComposeBegin() uint32`
- **方法** `func platformWin.ComposeEnd()`
- **方法** `func platformWin.NativeHandle() uintptr` — NativeHandle 原生句柄（X11 窗口 id）。
- **方法** `func platformWin.Close()`
- **方法** `func platformWin.Destroy()`

## `ui/backend_darwin.go`

- **方法** `func platformWin.SetEventHandler(fn func(Event))`
- **方法** `func platformWin.MakeCurrent()`
- **方法** `func platformWin.Swap()`
- **方法** `func platformWin.PixelSize() (int32, int32)`
- **方法** `func platformWin.Scale() float32`
- **方法** `func platformWin.IsMinimized() bool`
- **方法** `func platformWin.SetTitle(s string)`
- **方法** `func platformWin.SetIcon(ico []byte) bool`
- **方法** `func platformWin.SetVisible(v bool)`
- **方法** `func platformWin.Minimize()`
- **方法** `func platformWin.Restore()`
- **方法** `func platformWin.Center()`
- **方法** `func platformWin.BeginDragMove()` — BeginDragMove 自绘标题栏拖动：macOS 由系统标题栏负责，这里空实现。
- **方法** `func platformWin.ToggleMaximize()`
- **方法** `func platformWin.IsMaximized() bool`
- **方法** `func platformWin.SetFullscreen(v bool)`
- **方法** `func platformWin.IsFullscreen() bool`
- **方法** `func platformWin.SetAlwaysOnTop(on bool)`
- **方法** `func platformWin.AlwaysOnTop() bool`
- **方法** `func platformWin.SetCornerRadius(v float32)`
- **方法** `func platformWin.CornerRadius() float32`
- **方法** `func platformWin.SetAcceptDrops(on bool)`
- **方法** `func platformWin.AcceptDrops() bool`
- **方法** `func platformWin.SetImePosition(x, y int32)` — SetImePosition 输入法候选框（AppKit 自动跟随插入点，空实现）。
- **方法** `func platformWin.RegisterHotKey(mods, vk uint32, fn func()) int32` — RegisterHotKey 全局热键需要 CGEventTap，一期返回 0（不支持）。
- **方法** `func platformWin.UnregisterHotkey(id int32)`
- **方法** `func platformWin.AttachCompose(pw, ph int32)`
- **方法** `func platformWin.HasCompose() bool`
- **方法** `func platformWin.ResizeCompose(pw, ph int32)`
- **方法** `func platformWin.ComposeBegin() uint32`
- **方法** `func platformWin.ComposeEnd()`
- **方法** `func platformWin.NativeHandle() uintptr` — NativeHandle 原生句柄（NSView*）。
- **方法** `func platformWin.Close()`
- **方法** `func platformWin.Destroy()`

## `ui/backend_ios.go`

- **方法** `func platformWin.SetEventHandler(fn func(Event))`
- **方法** `func platformWin.MakeCurrent()`
- **方法** `func platformWin.Swap()`
- **方法** `func platformWin.PixelSize() (int32, int32)`
- **方法** `func platformWin.Scale() float32`
- **方法** `func platformWin.IsMinimized() bool`
- **方法** `func platformWin.SetTitle(s string)`
- **方法** `func platformWin.SetIcon(ico []byte) bool`
- **方法** `func platformWin.SetVisible(v bool)`
- **方法** `func platformWin.Minimize()`
- **方法** `func platformWin.Restore()`
- **方法** `func platformWin.Center()`
- **方法** `func platformWin.BeginDragMove()`
- **方法** `func platformWin.ToggleMaximize()`
- **方法** `func platformWin.IsMaximized() bool`
- **方法** `func platformWin.SetFullscreen(v bool)`
- **方法** `func platformWin.IsFullscreen() bool`
- **方法** `func platformWin.SetAlwaysOnTop(on bool)`
- **方法** `func platformWin.AlwaysOnTop() bool`
- **方法** `func platformWin.SetCornerRadius(v float32)`
- **方法** `func platformWin.CornerRadius() float32`
- **方法** `func platformWin.SetAcceptDrops(on bool)`
- **方法** `func platformWin.AcceptDrops() bool`
- **方法** `func platformWin.SetImePosition(x, y int32)` — SetImePosition 软键盘由系统摆放，空实现。
- **方法** `func platformWin.RegisterHotKey(mods, vk uint32, fn func()) int32`
- **方法** `func platformWin.UnregisterHotkey(id int32)`
- **方法** `func platformWin.AttachCompose(pw, ph int32)`
- **方法** `func platformWin.HasCompose() bool`
- **方法** `func platformWin.ResizeCompose(pw, ph int32)`
- **方法** `func platformWin.ComposeBegin() uint32`
- **方法** `func platformWin.ComposeEnd()`
- **方法** `func platformWin.NativeHandle() uintptr` — NativeHandle 原生窗口句柄（UIWindow*）。
- **方法** `func platformWin.Close()`
- **方法** `func platformWin.Destroy()`

## `ui/backend_android.go`

- **方法** `func platformWin.SetEventHandler(fn func(Event))`
- **方法** `func platformWin.MakeCurrent()`
- **方法** `func platformWin.Swap()`
- **方法** `func platformWin.PixelSize() (int32, int32)`
- **方法** `func platformWin.Scale() float32`
- **方法** `func platformWin.IsMinimized() bool`
- **方法** `func platformWin.SetTitle(s string)`
- **方法** `func platformWin.SetIcon(ico []byte) bool`
- **方法** `func platformWin.SetVisible(v bool)`
- **方法** `func platformWin.Minimize()`
- **方法** `func platformWin.Restore()`
- **方法** `func platformWin.Center()`
- **方法** `func platformWin.BeginDragMove()`
- **方法** `func platformWin.ToggleMaximize()`
- **方法** `func platformWin.IsMaximized() bool` — IsMaximized 移动端没有窗口最大化概念（恒 true）。
- **方法** `func platformWin.SetFullscreen(v bool)`
- **方法** `func platformWin.IsFullscreen() bool`
- **方法** `func platformWin.SetAlwaysOnTop(on bool)`
- **方法** `func platformWin.AlwaysOnTop() bool`
- **方法** `func platformWin.SetCornerRadius(v float32)`
- **方法** `func platformWin.CornerRadius() float32`
- **方法** `func platformWin.SetAcceptDrops(on bool)`
- **方法** `func platformWin.AcceptDrops() bool`
- **方法** `func platformWin.SetImePosition(x, y int32)` — SetImePosition 软键盘候选框位置（由系统输入法决定）。
- **方法** `func platformWin.RegisterHotKey(mods, vk uint32, fn func()) int32`
- **方法** `func platformWin.UnregisterHotkey(id int32)`
- **方法** `func platformWin.AttachCompose(pw, ph int32)`
- **方法** `func platformWin.HasCompose() bool`
- **方法** `func platformWin.ResizeCompose(pw, ph int32)`
- **方法** `func platformWin.ComposeBegin() uint32`
- **方法** `func platformWin.ComposeEnd()`
- **方法** `func platformWin.NativeHandle() uintptr` — NativeHandle 原生窗口指针（ANativeWindow*）。
- **方法** `func platformWin.Close()`
- **方法** `func platformWin.Destroy()`

## `ui/backend_wasm.go`

- **方法** `func platformWin.NativeHandle() uintptr` — NativeHandle 浏览器端无窗口句柄，返回 0（保持 Backend 接口一致）。
- **方法** `func platformWin.Compositor() any` — Compositor 浏览器端无 DirectComposition 合成器。
- **方法** `func platformWin.SetEventHandler(fn func(Event))`
- **方法** `func platformWin.MakeCurrent()`
- **方法** `func platformWin.Swap()`
- **方法** `func platformWin.PixelSize() (int32, int32)`
- **方法** `func platformWin.Scale() float32`
- **方法** `func platformWin.IsMinimized() bool`
- **方法** `func platformWin.SetTitle(s string)`
- **方法** `func platformWin.SetIcon([]byte) bool`
- **方法** `func platformWin.SetVisible(v bool)`
- **方法** `func platformWin.Minimize()`
- **方法** `func platformWin.Restore()`
- **方法** `func platformWin.Center()`
- **方法** `func platformWin.BeginDragMove()`
- **方法** `func platformWin.SetAlwaysOnTop(bool)`
- **方法** `func platformWin.AlwaysOnTop() bool`
- **方法** `func platformWin.SetAcceptDrops(bool)`
- **方法** `func platformWin.AcceptDrops() bool`
- **方法** `func platformWin.RegisterHotKey(mods, vk uint32, fn func()) int32`
- **方法** `func platformWin.UnregisterHotkey(id int32)`
- **方法** `func platformWin.SetCornerRadius(float32)`
- **方法** `func platformWin.CornerRadius() float32`
- **方法** `func platformWin.ToggleMaximize()`
- **方法** `func platformWin.IsMaximized() bool`
- **方法** `func platformWin.SetFullscreen(v bool)`
- **方法** `func platformWin.IsFullscreen() bool`
- **方法** `func platformWin.SetImePosition(x, y int32)`
- **方法** `func platformWin.Close()`
- **方法** `func platformWin.Destroy()`
- **方法** `func platformWin.AttachCompose(pw, ph int32)` — 合成与硬件句柄：浏览器端不存在，统一空实现。
- **方法** `func platformWin.HasCompose() bool`
- **方法** `func platformWin.ResizeCompose(pw, ph int32)`
- **方法** `func platformWin.ComposeBegin() uint32`
- **方法** `func platformWin.ComposeEnd()`

## `ui/backend_null.go`

- **方法** `func platformWin.SetEventHandler(fn func(Event))`
- **方法** `func platformWin.MakeCurrent()`
- **方法** `func platformWin.Swap()`
- **方法** `func platformWin.PixelSize() (int32, int32)`
- **方法** `func platformWin.Scale() float32`
- **方法** `func platformWin.IsMinimized() bool`
- **方法** `func platformWin.SetTitle(s string)`
- **方法** `func platformWin.SetIcon(ico []byte) bool`
- **方法** `func platformWin.SetVisible(v bool)`
- **方法** `func platformWin.Minimize()`
- **方法** `func platformWin.Restore()`
- **方法** `func platformWin.Center()`
- **方法** `func platformWin.BeginDragMove()`
- **方法** `func platformWin.ToggleMaximize()`
- **方法** `func platformWin.IsMaximized() bool`
- **方法** `func platformWin.SetFullscreen(v bool)`
- **方法** `func platformWin.IsFullscreen() bool`
- **方法** `func platformWin.SetAlwaysOnTop(on bool)`
- **方法** `func platformWin.AlwaysOnTop() bool`
- **方法** `func platformWin.SetCornerRadius(v float32)`
- **方法** `func platformWin.CornerRadius() float32`
- **方法** `func platformWin.SetAcceptDrops(on bool)`
- **方法** `func platformWin.AcceptDrops() bool`
- **方法** `func platformWin.SetImePosition(x, y int32)`
- **方法** `func platformWin.RegisterHotKey(mods, vk uint32, fn func()) int32`
- **方法** `func platformWin.UnregisterHotkey(id int32)`
- **方法** `func platformWin.AttachCompose(pw, ph int32)`
- **方法** `func platformWin.HasCompose() bool`
- **方法** `func platformWin.ResizeCompose(pw, ph int32)`
- **方法** `func platformWin.ComposeBegin() uint32`
- **方法** `func platformWin.ComposeEnd()`
- **方法** `func platformWin.NativeHandle() uintptr`
- **方法** `func platformWin.Close()`
- **方法** `func platformWin.Destroy()`

## `ui/hotreload.go`

- **函数** `func SetHotState(save func() []byte, restore func([]byte))` — SetHotState 注册热重载现场保存 / 恢复回调（可选）。
- **函数** `func HotReloadActive() bool` — HotReloadActive 当前进程是否运行在热重载模式下（由守护进程启动的子进程）。


## `ui/hotreload_stub.go`

- **函数** `func SetHotState(save func() []byte, restore func([]byte))` — SetHotState 注册热重载现场保存 / 恢复回调（移动端与浏览器端为空操作）。
- **函数** `func HotReloadActive() bool` — HotReloadActive 当前进程是否运行在热重载模式下（移动端与浏览器端恒为 false）。

---

## 系统级操作（2026-11 新增）

以下文件补齐了「与操作系统深度交互」的能力面，全部遵循同一约定：
符号在所有平台都存在，平台差异用 `HasCap` 判断，调用失败返回零值或错误、绝不 panic。

## `ui/system_dirs.go`

- **类型** `type KnownDir int` — 系统标准目录类别。
- **函数** `func KnownDirName(d KnownDir) string` — 目录类别的中文名（界面展示 / 日志用）。
- **函数** `func KnownFolder(d KnownDir) string` — 返回标准目录绝对路径；无法确定时空串。
- **函数** `func KnownFolderExists(d KnownDir) bool` — 标准目录是否存在。
- **函数** `func EnsureFolder(d KnownDir) (string, error)` — 不存在时创建并返回路径。
- **函数** `func SubFolder(d KnownDir, sub string) string` — 拼子目录（不创建）。
- **常量** `DirHome / DirDesktop / DirDocuments / DirDownloads / DirPictures / DirMusic / DirVideos / DirConfig / DirCache / DirData / DirTemp / DirPrograms / DirExecutable`

## `ui/system_watch.go`

- **类型** `type FsChange int` — 变更类型。
- **类型** `type FsEvent struct{ Path, OldPath string; Change FsChange; IsDir bool }` — 一条文件变更。
- **类型** `type WatchOptions struct{ Recursive bool; Debounce, PollInterval time.Duration; Filter func(string) bool }` — 监听参数。
- **函数** `func FsChangeName(c FsChange) string` — 变更类型中文名。
- **函数** `func (e FsEvent) String() string` — 简短描述。
- **函数** `func WatchDir(dir string, opts WatchOptions, fn func([]FsEvent)) func()` — 监听目录，返回停止函数（幂等）。
- **函数** `func WatchFile(path string, fn func(FsEvent)) func()` — 监听单个文件。
- **常量** `FsCreated / FsModified / FsRemoved / FsRenamed`

## `ui/system_native.go`

- **类型** `type MsgKind int` / `type MsgButtons int` / `type MsgResult int` — 原生消息框参数与返回值。
- **函数** `func MessageBox(title, body string) MsgResult` — 只有一个「确定」的原生消息框。
- **函数** `func MessageBoxEx(title, body string, kind MsgKind, buttons MsgButtons) MsgResult` — 完整参数版本。
- **函数** `func MsgButtonTexts(b MsgButtons) []string` — 按钮文案（自绘对话框复用）。
- **常量** `MsgInfo / MsgWarn / MsgError / MsgQuestion`、`MsgBoxOK / MsgBoxOKCancel / MsgBoxYesNo / MsgBoxRetryCancel`、`MsgResOK / MsgResCancel / MsgResYes / MsgResNo`

## `ui/system_info.go`

- **类型** `type SysInfo struct` — 系统信息快照（Locale / LocaleName / UserName / HostName / OS / OSVersion / Arch / Elevated）。
- **类型** `type PowerStatus struct` — 电源状态（OnBattery / Percent / Charging / ACOnline / SaverOn）。
- **函数** `func SystemInfo() SysInfo` — 读取系统信息。
- **函数** `func (s SysInfo) String() string` — 单行描述。
- **函数** `func SystemLocale() string` — 系统 UI 语言（读不到返回 "en-US"）。
- **函数** `func IsElevated() bool` — 是否管理员 / root。
- **函数** `func PowerState() PowerStatus` — 读取电源状态。
- **函数** `func HasBattery() bool` — 本机是否有电池。

## `ui/system_session.go`

- **函数** `func LockScreen() error` — 锁屏。
- **函数** `func Suspend(hibernate bool) error` — 睡眠 / 休眠。
- **函数** `func Shutdown(delay int, force bool) error` — 关机。
- **函数** `func Reboot(delay int, force bool) error` — 重启。
- **函数** `func SignOut(force bool) error` — 注销。
- **函数** `func CancelShutdown() error` — 取消排队的关机。
- **函数** `func IsUnsupported(err error) bool` — 错误是否来自「平台不支持」。

## `ui/system_shell.go`

- **类型** `type ShellLink struct` — 快捷方式参数（Target / Args / WorkDir / IconPath / IconIndex / Description / Hotkey / Minimized）。
- **类型** `type WindowInfo struct{ Handle uintptr; Title, Process string; PID uint32 }` — 前台窗口信息。
- **函数** `func FileIcon(path string, size int) (*image.NRGBA, error)` — 文件类型图标。
- **函数** `func FileThumbnail(path string, w, h int) (*image.NRGBA, error)` — 文件缩略图。
- **函数** `func FileTypeName(path string) string` — 文件类型友好名。
- **函数** `func DragFiles(paths []string, x, y float32) error` — OLE 拖出文件。
- **函数** `func CreateShortcut(lnkPath string, sc ShellLink) error` — 创建快捷方式（.lnk / .desktop / .webloc）。
- **函数** `func SetFileAssociation(ext, progID, command string) error` — 注册文件关联（HKCU）。
- **函数** `func ForegroundWindowInfo() WindowInfo` — 前台窗口信息。
- **函数** `func (a *App) Activate()` — 拉到前台并抢焦点。
- **函数** `func OpenTerminal(dir string) error` — 在指定目录打开终端。

## `ui/system_windowfx.go`

- **类型** `type BackdropKind int` — 窗口材质（BackdropNone / BackdropMica / BackdropAcrylic / BackdropTabbed）。
- **函数** `func (a *App) SetOpacity(f float32)` / `func (a *App) Opacity() float32` — 整窗不透明度。
- **函数** `func (a *App) SetClickThrough(on bool)` / `func (a *App) ClickThrough() bool` — 鼠标穿透。
- **函数** `func (a *App) SetKeepOnBottom(on bool)` — 窗口置底。
- **函数** `func (a *App) SetSkipTaskbar(on bool)` / `func (a *App) SkipTaskbar() bool` — 隐藏任务栏按钮。
- **函数** `func (a *App) SetCaptureExcluded(on bool)` — 防截屏 / 录屏。
- **函数** `func (a *App) SetDarkTitleBar(on bool)` — 系统标题栏深色。
- **函数** `func (a *App) SetBackdrop(kind BackdropKind)` — Win11 背景材质。
- **函数** `func (a *App) SetMinSize(w, h int)` / `func (a *App) SetMaxSize(w, h int)` — 尺寸约束。
- **函数** `func (a *App) SetTaskbarProgress(done, total int64)` / `func (a *App) ClearTaskbarProgress()` — 任务栏进度。
- **函数** `func (a *App) SetBadge(n int)` — 任务栏角标。
- **函数** `func (a *App) Flash(count int)` — 闪烁任务栏图标。

## `ui/system_clipboard_rich.go`

- **函数** `func ClipboardHTML() (string, error)` / `func ClipboardSetHTML(html string) error` — HTML 剪贴板。
- **函数** `func ClipboardRTF() (string, error)` / `func ClipboardSetRTF(rtf string) error` — RTF 剪贴板。
- **函数** `func HasClipboardHTML() bool` — 剪贴板是否含 HTML。
- **函数** `func HTMLToPlainText(html string) string` — 极简去标签。
- **常量** `ClipHTML / ClipRTF`

## `ui/system_input.go`

- **函数** `func SendKey(vk uint32) error` — 模拟单键。
- **函数** `func SendKeyCombo(mods uint32, vk uint32) error` — 模拟组合键。
- **函数** `func SendText(s string) error` — 逐字符 Unicode 输入。
- **函数** `func MouseMoveTo(x, y int) error` — 移动鼠标（屏幕绝对坐标）。
- **函数** `func MouseClickAt(x, y int, button int) error` — 单击。
- **函数** `func MouseWheel(delta int) error` — 滚轮。
- **函数** `func WaitIdle()` — 模拟输入后的便捷重绘。
- **常量** `MouseLeft / MouseRight / MouseMiddle`

## `ui/system_audio_net.go`

- **函数** `func SystemVolume() float32` — 系统主音量 0~1（未知 -1）。
- **函数** `func SetSystemVolume(v float32) error` — 设置主音量。
- **函数** `func ToggleSystemMute() (bool, error)` — 切换静音。
- **函数** `func Online() bool` — 粗粒度联网判断。
- **函数** `func OnlineAsync(fn func(online bool))` — 后台检测。
- **函数** `func SetOnNetworkChange(fn func(online bool)) func()` — 联网状态变化回调（轮询）。
