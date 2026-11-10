<!-- group: 附录 -->
# 附录 A · 核心 API 速查（节点 / 布局 / 样式 / 主题 / 事件 / 窗口）

本附录由工具从源码导出符号自动生成（含每个导出符号的签名与文档首行），
覆盖 `ui` 包中与「搭界面」直接相关的全部导出 API，作为正文各章的完整索引使用。

> 说明：本附录为**自动生成**的完整符号清单，按源文件分组；
> 想按「用途」查找请先读前面的正文章节，本附录保证**没有任何导出 API 被漏掉**。

| 源文件 | 符号数 |
| --- | --- |
| `ui/main_entry_desktop.go` | 2 |
| `ui/main_entry_android.go` | 2 |
| `ui/boot_wasm.go` | 2 |
| `ui/app.go` | 55 |
| `ui/platform.go` | 16 |
| `ui/platform_unix.go` | 19 |
| `ui/platform_wasm.go` | 30 |
| `ui/platform_misc_unix.go` | 3 |
| `ui/backend.go` | 1 |
| `ui/node.go` | 67 |
| `ui/layout.go` | 7 |
| `ui/style.go` | 63 |
| `ui/color.go` | 2 |
| `ui/theme.go` | 42 |
| `ui/theme_tokens.go` | 8 |
| `ui/scale.go` | 14 |
| `ui/event.go` | 41 |
| `ui/focus.go` | 12 |
| `ui/a11y.go` | 24 |
| `ui/shortcuts.go` | 16 |
| `ui/selection.go` | 5 |
| `ui/textedit.go` | 28 |
| `ui/textseg.go` | 1 |
| `ui/layers.go` | 5 |
| `ui/debug.go` | 2 |
| `ui/caps.go` | 26 |

合计 **493** 个导出符号。

---

## `ui/main_entry_desktop.go`

- **函数** `func SetEntry(fn func() (Options, func() *Node))` — SetEntry 注册应用入口：返回窗口选项与整页构建函数。
- **函数** `func Main()` — Main 三端统一的应用入口（桌面版：Windows / Linux / macOS）。

## `ui/main_entry_android.go`

- **函数** `func SetEntry(fn func() (Options, func() *Node))` — SetEntry 注册应用入口（应用 init() 里调用，先于 NativeActivity 回调）。
- **函数** `func Main()` — Main 三端统一入口的 Android 版：UI 在 OnCreate 派生的 goroutine 里跑，

## `ui/boot_wasm.go`

- **函数** `func SetEntry(fn func() (Options, func() *Node))` — SetEntry 注册应用入口：返回窗口选项与整页构建函数。
- **函数** `func Main()` — Main 三端统一的应用入口（浏览器版）。

## `ui/app.go`

- **类型** `type Options struct { Title string Width int Height int FontFamily string Background any Resizable bool // DumpLayout 非空时首帧后将布局树写入该文件并退出（排查布局问题）。 DumpLayout string // Hidden 创建后不显示窗口（配合 Tray 常驻托盘）。 Hidden bool // NoTitleBar 去掉系统标题栏（自绘标题栏场景）。 NoTitleBar bool // CornerRadius 窗口圆角半径（逻辑像素；0 = 系统默认，>0 = 圆角）。 // Win11 走 DWM 原生圆角，低版本/无 DWM 时回退 SetWindowRgn 区域圆角。 CornerRadius float32 // Icon 窗口/任务栏图标（.ico 字节）；为空时用内置默认图标。 // 需要 exe 文件图标时，用 rsrc 类工具把同一 .ico 嵌入可执行文件。 Icon []byte }` — Options 启动参数。
- **类型** `type App struct { win Backend r *gpu.Renderer fonts *fontCache opts Options view func() *Node root *Node dirty bool running bool closed bool // revealed 首帧已呈现、窗口已显示（启动时先隐藏窗口，避免白屏/闪一下） revealed bool mouseX, mouseY float32 mouseIn bool pressX, pressY float32 pressed bool clickPending bool clickX, clickY float32 // 右键按下：下一帧（树指针有效时）分发给 OnRightClickAt。 rightPending bool rightX, rightY float32 // redrawRequest 允许业务回调在任意线程请求重绘 redrawRequest atomic.Bool // hoverProbe 仅指针位置变化（未确定是否需要重绘外观）时置位 hoverProbe bool hoverNode *Node pressNode *Node // pressSent 本次按下是否已分发过 OnPress。 // // 不能用「节点指针」判断：节点树每帧重建，指针帧帧不同，会导致 // OnPress 在按住期间每帧重放（拖选的锚点被不断改写成鼠标当前位置， // 选区永远拉不出来）。 pressSent bool scrolls map[string]float32 bg gpu.Paint onKey func(uint32) onChar func(rune) frameCount int // 键盘焦点（输入框）与逐帧动画 focusKey string dragKey string hoverKey string startTime time.Time animating bool // animFast 本帧存在需要高帧率的动画节点（未标记 AnimSlow）； // 只有低帧率动画时不降帧会持续吃满一个核。 animFast bool // slowOnlyFrame 上一帧的重绘只由装饰动画（光标闪烁 / shimmer）驱动。 // runLoop 据此把整树重建+绘制节流到 8–12Hz，而消息泵仍跑 120Hz。 slowOnlyFrame bool // animDirty 动画续帧标记（可被装饰动画节流；dirty 则必须立即绘制）。 animDirty bool // dbgAnim 本帧参与持续重绘的动画节点（GUIS_FPS=1 时输出，定位 CPU 用）。 dbgAnim []string // dbgBusy 上一帧结束时 animBusy 的取值（诊断用）。 dbgBusy bool // dbgViewMs / dbgLayMs / dbgPaintMs 上一帧各阶段耗时（ms，诊断用）。 dbgViewMs, dbgLayMs, dbgPaintMs float64 // 图片纹理缓存 textures *imageTextures // 子菜单指示三角纹理（懒创建，随窗口销毁） arrowTex *gpu.Texture // 图表矢量层纹理（每张图表一张，总预算 chartTexBudget，按 LRU 淘汰） chartList []*chartCache chartBytes int64 chartStamp int64 // 视频帧纹理缓存（每个播放器一份，随窗口销毁；按帧序号做 LRU 回收） videos map[*Player]*videoTexture videoSeq int // 内嵌浏览器（WebView2）：本帧绘制到的矩形与已显示集合，用于同步子窗口 webviews map[*WebView]bool wvRects map[*WebView]geom.Rect wvShown map[*WebView]bool // IME 组合串（当前窗口内的输入法预览） imeText string imeCaret int imeX, imeY int32 // 文件拖入：WM_DROPFILES 的路径与落点，延迟到下一帧（节点树有效时）分发 dropPending bool dropPaths []string dropX, dropY float32 // onDrop 应用级拖入回调（没有节点设置 OnDrop 时兑底） onDrop func(paths []string, x, y float32) // 滚动：当前偏移（scrolls）与平滑滚动目标偏移（scrollTargets） scrollTargets map[string]float32 // 滚动条：悬停 / 拖动 / 淡出 scrollHoverKey string scrollDragKey string scrollDragY float32 scrollDragStart float32 // 拖动开始时缓存的滑块几何：指针拖出 12px 热区后仍能继续跟手 scrollDragTrack geom.Rect scrollDragThumb float32 scrollDragMax float32 // scrollIdle 距上次滚动/悬停的秒数（滚动条淡出用） scrollIdle float32 // 上一帧时间戳与帧间隔（滚动缓动/淡出推进） lastFrame time.Time animDt float32 // lastPaint 最近一次真正绘制的时刻（空闲回收缓存用） lastPaint time.Time // trimmed 本段空闲期是否已释放过可重建的 GPU 缓存（恢复绘制时清零） trimmed bool // trimmedDeep 本段空闲期是否已做过「工作集级」回收（更激进，代价是重新调页） trimmedDeep bool // 本帧裁剪栈（用于去重：子项裁剪区与父级完全相同时不重复入栈） clipStack []geom.Rect clipN int clipSkip int // 组件栅格缓存：结构路径键 → 一张屏幕快照纹理（见 ui/layers.go） layers map[uint64]*layerEntry layerPending []layerShot layerStamp int64 // cacheLayers 本帧是否开启「能缓则缓」（帧开头从全局开关同步） cacheLayers bool // layerOpen 本帧已在一层栅格缓存子树内：嵌套层不再重复抓取 layerOpen bool // layerRoots 本帧被标记为缓存根的节点数（诊断用） layerRoots int // layerRej 本帧成为候选但被绘制期条件拒绝的次数（诊断用） layerRej int // 本帧窗口逻辑尺寸（栅格缓存可用性判断用） winW, winH float32 // 组件缓存命中 / 未命中次数（诊断用：确认缓存真的生效） layerHit, layerMiss int // 窗口关闭回调（返回 false 可阻止关闭，例如收进托盘） onClose func() bool // quitting 跳过关闭拦截（App.Quit 真正退出时置位） quitting bool pending []func() // disposed 保证 GL/字体资源只释放一次 disposed bool // minimized 上一帧是否处于最小化（恢复时强制重绘，并重建媒体解码窗口） minimized bool // sizeMoving 系统正在拖动/缩放窗口（WM_ENTERSIZEMOVE..WM_EXITSIZEMOVE）。 // 模态循环期间鼠标消息不再更新，滚动条/悬停高亮暂停绘制， // 否则窗边会残留一条静止的「假滚动条」。 sizeMoving bool // shiftDown Shift 键当前是否按下（Shift+Tab 反向遍历用）。 shiftDown bool // ctrlDown / altDown 修饰键状态（命令面板 Ctrl+K、快捷键匹配用）。 ctrlDown bool altDown bool }` — App 一个窗口的应用实例（UI 线程独占）。
- **函数** `func Run(opts Options, view func() *Node) error` — Run 阻塞运行应用（必须在 main goroutine）。
- **函数** `func OpenWindow(opts Options, view func() *Node) (*App, error)` — OpenWindow 在当前 UI 线程再开一个窗口（可选）。
- **函数** `func SetFocus(key string)` — SetFocus 设置键盘焦点到指定 Key（传空串取消焦点）。
- **函数** `func FocusedKey() string` — FocusedKey 返回当前焦点 Key。
- **函数** `func IsFocused(key string) bool` — IsFocused 判断节点是否持有焦点。
- **函数** `func IsHovered(key string) bool` — IsHovered 判断鼠标当前是否悬停在 Key 对应的节点上（悬停外观用）。
- **函数** `func IsPressed(key string) bool` — IsPressed 判断鼠标是否正按在 Key 对应的节点上（按下外观用）。
- **函数** `func LastFrameOf(key string) (geom.Rect, bool)` — LastFrameOf 返回上一帧中 Key 对应节点在窗口坐标系里的矩形（浮层按锚点定位用）。
- **函数** `func ComposingText() string` — ComposingText 返回当前窗口的 IME 组合串（未组合时为空）。
- **函数** `func ComposingCaret() int` — ComposingCaret 返回组合串内的光标位置（rune 数）。
- **函数** `func AnimTime() float32` — AnimTime 返回启动至今的秒数（含 Animated 节点时每帧递增重绘）。
- **函数** `func MousePos() (float32, float32)` — MousePos 返回鼠标当前位置（逻辑像素）。
- **函数** `func ScrollOffset(key string) float32` — ScrollOffset 返回指定滚动容器当前的纵向偏移（虚拟列表据此计算可见区间）。
- **函数** `func ScrollTo(key string, y float32)` — ScrollTo 立即跳转到指定偏移（自动夹取到 0..max）。
- **函数** `func ScrollBy(key string, dy float32)` — ScrollBy 相对滚动（正数向下）。
- **函数** `func MouseDown() bool` — MouseDown 报告鼠标左键当前是否仍按下（拖动残留清理用）。
- **函数** `func SetTooltip(text string, x, y float32)` — SetTooltip 设置悬停提示文本与位置（空串清除），一般传 MousePos()。
- **函数** `func TextWidth(s string, size float32, weight int32) float32` — TextWidth 测量单行文本宽度（逻辑像素，UI 线程调用）。
- **函数** `func TextWidthOf(s string, size float32, weight int32, family string) float32` — TextWidthOf 按字体家族测量单行文本宽度。
- **函数** `func TextGlyphBounds(s string, size float32, weight int32) (top, h float32)` — TextGlyphBounds 返回单行文本相对「行顶」的视觉包围盒：top 为字形顶部偏移，
- **函数** `func WindowCount() int` — WindowCount 返回当前存活窗口数。
- **函数** `func OnKeyGlobal(fn func(uint32))` — OnKeyGlobal 给当前活跃窗口设置全局按键回调（无窗口时忽略）。
- **函数** `func RequestRedraw()` — RequestRedraw 从任意 goroutine 请求重绘（业务回调修改状态后调用）。
- **函数** `func RequestRedrawIn(a *App)` — RequestRedrawIn 只重绘指定窗口。
- **方法** `func App.Invalidate()` — Invalidate 请求重绘（可跨 goroutine 调用）。
- **方法** `func App.SetOnKey(fn func(uint32))` — SetOnKey 设置全局按键回调（任意节点聚焦时都会收到，用于 ESC 等全局快捷键）。
- **方法** `func App.SetOnDropFiles(fn func(paths []string, x, y float32))` — SetOnDropFiles 设置本窗口的文件拖入回调（拖到没有 OnDrop 的区域时触发）。
- **方法** `func App.SetAcceptDrops(on bool)` — SetAcceptDrops 开启/关闭窗口接收文件拖入（默认开启）。
- **方法** `func App.AcceptDrops() bool` — AcceptDrops 当前窗口是否接收文件拖入。
- **方法** `func App.SetAlwaysOnTop(on bool)` — SetAlwaysOnTop 窗口置顶 / 取消置顶。
- **方法** `func App.IsAlwaysOnTop() bool` — IsAlwaysOnTop 当前窗口是否置顶。
- **方法** `func App.RegisterHotkey(mods uint32, vk uint32, fn func()) int32` — RegisterHotkey 注册系统级热键（窗口失焦/最小化同样触发），返回 id（0 = 失败）。
- **方法** `func App.UnregisterHotkey(id int32)` — UnregisterHotkey 注销热键。
- **方法** `func App.SetOnClose(fn func() bool)` — SetOnClose 设置窗口关闭拦截：返回 false 阻止关闭（例如最小化到托盘）。
- **方法** `func App.HWND() uintptr` — HWND 返回窗口句柄（文件对话框、托盘使用）。
- **方法** `func App.ShowWindow(v bool)` — ShowWindow 显示/隐藏窗口。
- **方法** `func App.Show()` — Show 显示窗口并置前（隐藏到托盘后从托盘恢复）。
- **方法** `func App.Minimize()` — Minimize 最小化窗口。
- **方法** `func App.Center()` — Center 把窗口居中到当前显示器工作区（水平 + 垂直）。
- **方法** `func App.BeginDragMove()` — BeginDragMove 把按下事件交给系统做窗口拖动（自绘标题栏用）。
- **方法** `func App.IsMaximized() bool` — IsMaximized 窗口是否已最大化。
- **方法** `func App.ToggleMaximize()` — ToggleMaximize 最大化与还原之间切换。
- **方法** `func App.SetFullscreen(v bool)` — SetFullscreen 切换窗口全屏（去掉标题栏并覆盖当前显示器；ESC 可退出）。
- **方法** `func App.IsFullscreen() bool` — IsFullscreen 窗口是否处于全屏。
- **方法** `func App.SetCornerRadius(v float32)` — SetCornerRadius 设置窗口圆角半径（逻辑像素；<=0 取消圆角）。
- **方法** `func App.CornerRadius() float32` — CornerRadius 返回当前窗口圆角半径（逻辑像素）。
- **方法** `func App.Restore()` — Restore 还原窗口并置前。
- **方法** `func App.SetTitle(s string)` — SetTitle 修改窗口标题。
- **方法** `func App.Close()` — Close 关闭窗口（会经过 SetOnClose 拦截，例如收进托盘）。
- **方法** `func App.Quit()` — Quit 退出应用：跳过 SetOnClose 的关闭拦截（托盘菜单「退出」用）。
- **方法** `func App.Post(fn func())` — Post 在 UI 线程执行回调（可从任意 goroutine 调用）。
- **方法** `func App.Dispose()` — Dispose 主动释放窗口资源。
- **方法** `func App.FontCacheInfo() (fonts int, atlasBytes int)` — FontCacheInfo 字体缓存的实例数与图集字节数（诊断 / 测试用）。

## `ui/platform.go`

- **类型** `type TrayItem win32.TrayItem` — TrayItem 托盘菜单项（Separator 为分隔线）。
- **类型** `type Tray struct { t *win32.Tray }` — Tray 任务栏托盘图标（依附当前窗口的消息循环）。
- **函数** `func RunAsync(fn func())` — RunAsync 在后台 goroutine 执行 fn（Windows 端解码/探测这类阻塞工作放后台，
- **函数** `func OpenFileDialog(title, filter string) (string, bool)` — OpenFileDialog 打开文件对话框。filter 形如 "图片|*.png;*.jpg|所有文件|*.*"。
- **函数** `func SaveFileDialog(title, filter, defExt string) (string, bool)` — SaveFileDialog 保存文件对话框。
- **函数** `func PickFolderDialog(title string) (string, bool)` — PickFolderDialog 选择文件夹。
- **函数** `func ShellOpen(path string)` — ShellOpen 用系统默认程序打开路径/网址。
- **函数** `func PickFile(title, filter string, onPick func(path string))` — PickFile 选单个文件（桌面端同步实现，选完立即回调）。
- **函数** `func PickFilesAsync(title, filter string, onPick func(paths []string))` — PickFilesAsync 选多个文件（桌面端同步实现，选完立即回调）。
- **函数** `func StartupQuery(key string) string` — StartupQuery 读取启动参数：桌面端没有 URL，恒返回空串（Web 端实现见 platform_wasm.go）。
- **函数** `func NewTray(tooltip string, onClick func(), items []TrayItem, onMenu func(id int)) (*Tray, error)` — NewTray 创建托盘图标：onClick 为左键点击，items/onMenu 为右键菜单。
- **函数** `func Current() *App` — Current 返回当前活跃窗口（无则 nil）。
- **函数** `func NewWindow(opts Options, view func() *Node) (*App, error)` — NewWindow 打开新窗口（须在 UI 线程/回调中调用）。
- **方法** `func Tray.SetTooltip(s string)` — SetTooltip 更新托盘提示。
- **方法** `func Tray.SetItems(items []TrayItem)` — SetItems 更新右键菜单。
- **方法** `func Tray.Remove()` — Remove 移除托盘图标。

## `ui/platform_unix.go`

- **类型** `type TrayItem struct { ID int Label string Separator bool Checked bool Disabled bool // Submenu 非空时生成二级菜单。 Submenu []TrayItem }`
- **函数** `func Current() *App` — Current 返回当前活跃窗口（无则 nil）。
- **函数** `func NewWindow(opts Options, view func() *Node) (*App, error)` — NewWindow 打开新窗口（须在 UI 线程/回调中调用）。
- **函数** `func ShellOpen(path string)` — ShellOpen 用系统默认程序打开路径或网址。
- **函数** `func PickFile(title, filter string, onPick func(path string))` — PickFile 选单个文件（类 Unix 端同步实现，选完立即回调）。
- **函数** `func PickFilesAsync(title, filter string, onPick func(paths []string))` — PickFilesAsync 选多个文件（类 Unix 端同步实现，选完立即回调）。
- **函数** `func StartupQuery(key string) string` — StartupQuery 读取启动参数：类 Unix 端没有 URL，恒返回空串（Web 端实现见 platform_wasm.go）。
- **函数** `func RunAsync(fn func())` — RunAsync 在后台 goroutine 执行 fn（类 Unix 端把阻塞工作放后台，
- **函数** `func OpenFileDialog(title, filter string) (string, bool)` — OpenFileDialog 打开文件对话框。filter 形如 "图片|*.png;*.jpg|所有文件|*.*"。
- **函数** `func SaveFileDialog(title, filter, defExt string) (string, bool)` — SaveFileDialog 保存文件对话框。
- **函数** `func PickFolderDialog(title string) (string, bool)` — PickFolderDialog 选择文件夹。
- **函数** `func PickFiles(title, filter string) ([]string, bool)` — PickFiles 打开多选文件对话框（本实现一次返回一个文件，保持 API 一致）。
- **函数** `func ClipboardText() string` — ClipboardText 读取剪贴板文本（无文本返回空串）。
- **函数** `func ClipboardSetText(s string) bool` — ClipboardSetText 写入剪贴板文本。
- **函数** `func ClipboardSetFiles(paths []string) bool` — ClipboardSetFiles 把文件路径写入剪贴板（本实现退化为文本形式，保持 API 一致）。
- **函数** `func SetKeepAwake(on bool) uint32` — SetKeepAwake 阻止系统休眠（Linux 用 systemd-inhibit，macOS 用 caffeinate）。
- **函数** `func RevealInExplorer(path string) bool` — RevealInExplorer 在文件管理器中定位给定路径（打开父目录即视为成功）。
- **函数** `func MoveToRecycleBin(path string) bool` — MoveToRecycleBin 把文件移入回收站（使用 gio trash 命令，失败返回 false）。
- **函数** `func SystemDarkMode() bool` — SystemDarkMode 系统是否处于深色模式（读取桌面环境的主题偏好）。

## `ui/platform_wasm.go`

- **类型** `type TrayItem struct { ID int Label string Separator bool Checked bool Disabled bool Submenu []TrayItem }` — TrayItem 托盘菜单项（字段与桌面端一致，保证同一份菜单代码可编译）。
- **类型** `type Tray struct { tooltip string }` — Tray 托盘图标（浏览器端占位）。
- **函数** `func RegisterFont(name string, data []byte)` — RegisterFont 注册一份字体数据（浏览器端唯一字体来源，如 fetch 回来的 TTF）。
- **函数** `func RegisterFontBold(name string, data []byte)` — RegisterFontBold 注册粗体字重（以 "<name>/Bold" 命名，字体缓存按字重优先取它）。
- **函数** `func Current() *App` — Current 返回当前活跃窗口。
- **函数** `func NewWindow(opts Options, view func() *Node) (*App, error)` — NewWindow 浏览器端只支持单窗口，直接复用当前窗口。
- **函数** `func ShellOpen(path string)` — ShellOpen 用新标签页打开链接。
- **函数** `func ClipboardSetText(s string) bool` — ClipboardSetText 写入剪贴板（异步 API，成功与否以调用是否抛错为准）。
- **函数** `func ClipboardText() string` — ClipboardText 浏览器端需要用户授权，这里返回空串（组件会走「无文本」分支）。
- **函数** `func ClipboardSetFiles(paths []string) bool` — ClipboardSetFiles 浏览器端不能写文件到剪贴板，退化为文本（路径列表）。
- **函数** `func OpenFileDialog(title, filter string) (string, bool)` — OpenFileDialog 打开文件对话框（浏览器端不支持同步返回，用 PickFile）。
- **函数** `func PickFile(title, filter string, onPick func(path string))` — PickFile 浏览器端选文件：用隐藏的 <input type=file> 异步实现。
- **函数** `func PickFilesAsync(title, filter string, onPick func(paths []string))` — PickFilesAsync 浏览器端多选文件（回调顺序与用户选择顺序一致）。
- **函数** `func SaveFileDialog(title, filter, defExt string) (string, bool)` — SaveFileDialog 保存文件对话框（浏览器端不支持：用 a[download] 代替）。
- **函数** `func PickFolderDialog(title string) (string, bool)` — PickFolderDialog 选择文件夹（浏览器端不支持）。
- **函数** `func PickFiles(title, filter string) ([]string, bool)` — PickFiles 多选文件（浏览器端不支持：用 <input type=file multiple>）。
- **函数** `func RunAsync(fn func())` — RunAsync 在「可安全调用平台 API 的线程」上执行 fn：浏览器端是单线程，
- **函数** `func StartupQuery(key string) string` — StartupQuery 读取 Web 端启动参数（?key=value，例如 ?case=video）；
- **函数** `func NewTray(tooltip string, onClick func(), items []TrayItem, onMenu func(id int)) (*Tray, error)` — NewTray 创建托盘（浏览器端永远返回 "不支持"）。
- **函数** `func RevealInExplorer(path string) bool` — RevealInExplorer 在文件管理器中定位（浏览器端不支持）。
- **函数** `func MoveToRecycleBin(path string) bool` — MoveToRecycleBin 移入回收站（浏览器端不支持）。
- **函数** `func SetKeepAwake(on bool) uint32` — SetKeepAwake 阻止休眠：浏览器端用 Screen Wake Lock API（存在才启用）。
- **函数** `func SystemDarkMode() bool` — SystemDarkMode 系统是否深色模式（matchMedia prefers-color-scheme）。
- **函数** `func BackendName() string` — BackendName 当前可用的解码后端名称（浏览器端走 <video> 原生解码）。
- **函数** `func BackendLabel(name string) string` — BackendLabel 解码后端的中文标签。
- **函数** `func FormatBackend(name string) string` — FormatBackend 后端名短标签。
- **方法** `func Tray.SetTooltip(s string)` — SetTooltip 更新提示。
- **方法** `func Tray.SetItems(items []TrayItem)` — SetItems 更新菜单。
- **方法** `func Tray.Notify(title, msg string)` — Notify 气泡通知（不支持）。
- **方法** `func Tray.Remove()` — Remove 移除托盘。

## `ui/platform_misc_unix.go`

- **函数** `func BackendName() string` — BackendName 当前可用的解码后端名称（界面展示用）。
- **函数** `func BackendLabel(name string) string` — BackendLabel 解码后端的中文标签。
- **函数** `func FormatBackend(name string) string` — FormatBackend 后端名短标签（"ffmpeg" / "系统"）。

## `ui/backend.go`

- **类型** `type Backend interface { // ---- 生命周期与呈现 ---- SetEventHandler(fn func(Event)) // 注册事件回调（App.handleEvent） MakeCurrent() // 绑定本窗口的 GL 上下文 Swap() // 交换缓冲，呈现一帧 PixelSize() (int32, int32) // 绘制目标的物理像素尺寸 Scale() float32 // 物理像素 / 逻辑像素 // ---- 窗口状态 ---- IsMinimized() bool SetTitle(s string) SetIcon(ico []byte) bool SetVisible(v bool) Minimize() Restore() Center() BeginDragMove() ToggleMaximize() IsMaximized() bool SetFullscreen(v bool) IsFullscreen() bool SetAlwaysOnTop(on bool) AlwaysOnTop() bool SetCornerRadius(v float32) CornerRadius() float32 // ---- 输入 ---- SetAcceptDrops(on bool) AcceptDrops() bool SetImePosition(x, y int32) // ---- 可选能力（不支持时空实现）---- RegisterHotKey(mods, vk uint32, fn func()) int32 UnregisterHotkey(id int32) // ---- 合成（仅 Windows 的 DirectComposition 有实际实现）---- AttachCompose(pw, ph int32) HasCompose() bool ResizeCompose(pw, ph int32) ComposeBegin() uint32 ComposeEnd() // ---- 原生句柄 ---- // Windows: HWND；X11: Window(xid)；Cocoa: NSView*；Android: ANativeWindow*；Web: 0。 NativeHandle() uintptr // ---- 销毁 ---- Close() Destroy() }` — Backend 平台窗口后端：把「窗口 + 消息泵 + GL 上下文」的平台差异收敛成一组方法，

## `ui/node.go`

Node 中与图标相关的字段：`IconName`（图标名称，`Kind == KIcon` 时生效）、`IconStroke`（线宽，设计网格单位，0 = 图标自带线宽），
详见 `ui/iconfont.go` 与正文《图标 · Iconfont 矢量图标库》。

- **类型** `type Kind uint8` — Kind 节点类型。
- **类型** `type Node struct { Kind Kind Key string Style Style Dir Direction Align Align Justify Align Text string Children []*Node // Src 为图片路径（Kind == KImage 时生效）。 Src string // Video 视频播放器状态（Kind == KVideo 时生效）。 Video *Player // WebView 内嵌浏览器状态（Kind == KWebView 时生效）。 WebView *WebView // Chart 图表数据与缓存（Kind == KChart 时生效）。 Chart *Chart // Code 代码编辑器状态（Kind == KCodeEditor 时生效）。 Code *CodeEditorState // Spans 富文本片段（Kind == KRichText 时生效）。 Spans []TextSpan RichOpts RichTextOptions // rich 布局阶段写入的折行/片段数据（绘制与命中共用）。 rich richLayoutData // OnSpanClick 富文本片段点击回调（链接命中）。 OnSpanClick func(TextSpan) // OnActivate 键盘 Enter/Space 激活回调（无则回退 OnClick）。 OnActivate func() // A11y 无障碍元数据（角色/名称/描述），当前仅用于导出与调试。 A11y A11yInfo OnClick func() // OnClickAt 带点击坐标（相对节点左上角）的点击回调，滑块等需要。 OnClickAt func(x, y float32) // OnHover 悬停变化回调。 OnHover func(bool) // OnHoverAt 悬停移动回调：鼠标停留期间每帧一次（含进入帧），坐标相对节点左上角。 // 需要「鼠标停在某个词上就弹提示」的自绘控件（代码编辑器）用它。 OnHoverAt func(x, y float32) // OnPress 鼠标按下即触发（在按下后的第一帧分发一次，早于 OnDrag/OnClick）。 // 自绘标题栏拖动、菜单按下等需要“按下就响应”的场景用它。 OnPress func() // OnRightClickAt 鼠标右键点击（坐标相对节点左上角）：右键菜单用。 // 右键不会触发 OnPress / OnClick，也不参与左键拖选。 OnRightClickAt func(x, y float32) OnScroll func(dy float32) // OnChar / OnKey 仅在节点获得键盘焦点时收到事件。 OnChar func(rune) OnKey func(uint32) // OnDrag 按下并拖动时回调（坐标相对节点左上角）；Disabled 禁用点击与拖动。 OnDrag func(x, y float32) // OnDragEnd 拖动结束（松手）时回调，对应 gio Clickable.Pressed() 由 true→false； // 拖动类组件据此提交最终值，不必靠超时猜测松手。 OnDragEnd func() // OnDrop 文件拖入回调：拖入落点命中本节点区域时触发，paths 为拖入的文件/文件夹路径。 // 与 OnClick 一样沿祖先冒泡，未设置 OnDrop 的子节点由最近的祖先接管。 OnDrop func(paths []string) Disabled bool // 运行时状态 Hover, Pressed, Focus bool ScrollY float32 // staticOK / cacheRoot 每帧由布局后的缓存标记阶段写入（见 ui/layers.go）： // staticOK: 0 未计算、1 子树纯静态、2 含动态/交互内容 // cacheRoot: 本帧可作为缓存根（最外层合格容器） // layerKey: 结构路径哈希（父哈希 + 子序号），跨帧稳定，用作缓存索引 staticOK int8 cacheRoot bool layerKey uint64 // wrapW 测量阶段的折行宽度：WrapOn 文本由布局写入，绘制阶段复用同一宽度， // 保证「测量行数」与「实际绘制行数」一致。 wrapW float32 ContentH float32 Frame geom.Rect Parent *Node tag string }` — Node 声明式节点（每帧重建，交互状态按 Key 继承）。
- **类型** `type Build func() *Node` — Build 用于封装局部 view。
- **函数** `func Box(children ...*Node) *Node` — Box 全能容器。
- **函数** `func Column(children ...*Node) *Node` — Column 纵向弹性容器。
- **函数** `func Row(children ...*Node) *Node` — Row 横向弹性容器（交叉轴默认拉伸，与 CSS flex align-items:stretch 一致）。
- **函数** `func Center(child *Node) *Node` — Center 居中容器。
- **函数** `func Spacer() *Node` — Spacer 可伸缩空白。
- **函数** `func Text(s string) *Node` — Text 文本节点。
- **函数** `func Rich(s string) *Node` — Rich 与 Text 相同，语义别名。
- **函数** `func Image(path string) *Node` — Image 图片节点（PNG/JPEG/GIF/BMP，尺寸默认取原图像素）。
- **函数** `func ImageSized(path string, w, h float32) *Node` — ImageSized 指定显示尺寸的图片节点。
- **函数** `func Video(p *Player) *Node` — Video 视频画面节点（尺寸默认取画面像素，可用 Style 覆盖；填充方式由 Player.Fit 决定）。
- **函数** `func VideoSized(p *Player, w, h float32) *Node` — VideoSized 指定显示尺寸的视频画面节点。
- **函数** `func VideoSurface(key string, p *Player) *Node` — VideoSurface 纯画面视频节点：每帧自行推进播放时钟（等价于 VideoPlayer 的画面部分），
- **函数** `func VideoSurfaceTick(key string, p *Player) *Node` — VideoSurfaceTick 纯画面 + 推进播放时钟，但**不**点亮全局续帧（animBusy）。
- **函数** `func Button(label string, onClick func()) *Node` — Button 按钮（可点击）。
- **函数** `func Scroll(children ...*Node) *Node` — Scroll 纵向滚动容器。
- **函数** `func Stack(children ...*Node) *Node` — Stack 叠层容器：子节点互相重叠，后声明的绘制在上层（用于开关旋钮、进度条、浮层）。
- **函数** `func Grid(children ...*Node) *Node` — Grid 网格布局：等宽列，行高取该行最高子项。
- **函数** `func Wrap(children ...*Node) *Node` — Wrap 流式换行布局（flex-wrap: wrap）：子项按声明顺序排布，一行放不下自动折行。
- **函数** `func Masonry(children ...*Node) *Node` — Masonry 瀑布流布局：每项落入当前最矮的一列（列数同 Grid）。
- **函数** `func EqualRow(children ...*Node) *Node` — EqualRow 横向等分栏：每个子项占等宽（Grow=1）。
- **函数** `func EqualCol(children ...*Node) *Node` — EqualCol 纵向等分栏：每个子项占等高（Grow=1）。
- **函数** `func Pick(xs, sm, md, lg func() *Node) *Node` — Pick 响应式选择：按视口宽度断点（<640 / <960 / <1280 / 其余）执行对应构造函数。
- **函数** `func Abs(child *Node) *Node` — Abs 绝对定位容器（脱离 flex 流，尺寸由 Style 决定）。
- **函数** `func If(cond bool, fn func() *Node) *Node` — If 条件构造。
- **函数** `func Each[T any](items []T, fn func(T, int) *Node) []*Node` — Each 列表构造。
- **方法** `func Node.ActivateEvt(fn func()) *Node` — ActivateEvt 键盘激活回调（Enter / Space 落到焦点节点时触发）。
- **方法** `func Node.A11yOn(role A11yRole, name, desc string) *Node` — A11y 设置无障碍元数据（角色 + 名称 + 描述）。
- **方法** `func Node.St(s *Style) *Node`
- **方法** `func Node.Add(c ...*Node) *Node`
- **方法** `func Node.K(key string) *Node`
- **方法** `func Node.Grow(v float32) *Node`
- **方法** `func Node.GapElastic(v float32) *Node`
- **方法** `func Node.AlignTo(a Align) *Node`
- **方法** `func Node.JustifyTo(a Align) *Node`
- **方法** `func Node.Horizontal() *Node`
- **方法** `func Node.Tap(fn func()) *Node` — Tap 点击回调。nil 安全：可选节点（如透明度为 0 时返回 nil 的 Scrim）
- **方法** `func Node.TapAt(fn func(x, y float32)) *Node` — TapAt 带相对坐标的点击回调（滑块、进度跳转等）。nil 安全。
- **方法** `func Node.OnCharEvt(fn func(rune)) *Node` — OnCharEvt 键盘字符输入回调（需配合 .St(St().FocusOn())）。
- **方法** `func Node.OnKeyEvt(fn func(uint32)) *Node`
- **方法** `func Node.DragEvt(fn func(x, y float32)) *Node` — DragEvt 按下拖动回调（需设置 Key）。
- **方法** `func Node.DragEndEvt(fn func()) *Node` — DragEndEvt 拖动结束（松手）回调（需设置 Key 并配合 DragEvt）。
- **方法** `func Node.DropEvt(fn func(paths []string)) *Node` — DropEvt 文件拖入回调（paths 为拖入的文件/文件夹路径，落点在本节点内时触发）。
- **方法** `func Node.DisabledOn(v bool) *Node` — DisabledOn 禁用控件（不响应点击/拖动，外观由样式决定）。
- **方法** `func Node.OnHoverEvt(fn func(bool)) *Node`
- **方法** `func Node.HoverAtEvt(fn func(x, y float32)) *Node` — HoverAtEvt 悬停移动回调（x/y 相对节点左上角，鼠标停留期间每帧一次）。
- **方法** `func Node.OnPressEvt(fn func()) *Node` — OnPressEvt 鼠标按下回调（需设置 Key，可用于自绘标题栏拖动）。
- **方法** `func Node.RightClickEvt(fn func(x, y float32)) *Node` — RightClickEvt 鼠标右键回调（x/y 相对节点左上角），右键菜单用。
- **方法** `func Node.ScrollEvt(fn func(float32)) *Node`
- **常量** `const KBox = iota`
- **常量** `const KText`
- **常量** `const KButton`
- **常量** `const KScroll`
- **常量** `const KImage`
- **常量** `const KStack`
- **常量** `const KGrid`
- **常量** `const KWrap`
- **常量** `const KMasonry`
- **常量** `const KVideo`
- **常量** `const KWebView`
- **常量** `const KArrow`
- **常量** `const KChart`
- **常量** `const KRichText`
- **常量** `const KCodeEditor`
- **常量** `const KIcon`

## `ui/layout.go`

- **类型** `type Constraints struct { MinW, MaxW, MinH, MaxH float32 }` — Constraints 约束模型（Flutter 风格）。
- **函数** `func SetViewport(w, h float32)` — SetViewport 用窗口逻辑尺寸初始化视口（供 newApp 在首帧构建 view 之前调用）。
- **函数** `func ViewportW() float32` — ViewportW 返回最近一次布局的视口宽度（逻辑像素），用于响应式分支。
- **函数** `func PixelScale() float32` — PixelScale 当前窗口的物理像素比（100% = 1）。位图资源按它解码后 1:1 贴图才不发虚。
- **函数** `func ViewportH() float32` — ViewportH 返回最近一次布局的视口高度（逻辑像素）。
- **函数** `func BP[T any](xs, sm, md, lg T) T` — BP 按视口宽度选择断点值：<640 取 xs，<960 取 sm，<1280 取 md，其余 lg。
- **函数** `func Layout(root *Node, w, h float32, fonts *fontCache)` — Layout 对整棵树做布局：先 measure（算尺寸），再 arrange（定位）。

## `ui/style.go`

- **类型** `type Align uint8` — Align 主轴/交叉轴对齐。
- **类型** `type Direction uint8` — Direction 弹性方向。
- **类型** `type Style struct { Width, Height float32 // 0 = auto WidthPct, HeightPct float32 // >0 = 占父容器百分比 MinW, MaxW float32 MinH, MaxH float32 Pad geom.Insets Margin geom.Insets Gap float32 Grow float32 Basis float32 // 主轴基准尺寸（配合 Grow 实现等宽/等高分栏，类似 flex-basis） AlignSelf Align // Cols / ColW 网格与瀑布流列参数：Cols>0 固定列数；ColW>0 固定列宽（自动算列数）。 Cols int ColW float32 Bg gpu.Paint BgTo gpu.Paint // BgToSet 为 true 时启用线性渐变 BgAngle float32 // BgSet / BgToSet 记录 Bg / BgTo 是否被显式设置过。 // 渐变端点经常是全透明（rgba(...,0)，alpha=0），零值 Paint 与「未设置」无法区分， // merge 只按 alpha>0 判断会把透明端丢成纯黑透明，渐变就画错了。 BgSet, BgToSet bool Radius [4]float32 Border float32 BorderColor gpu.Paint ShadowBlur float32 ShadowY float32 ShadowSpread float32 ShadowColor gpu.Paint FontSize float32 FontWeight int32 // FontFamily 字体家族名（空 = 默认 Microsoft YaHei UI）。 FontFamily string Color gpu.Paint LineHeight float32 TextAlign Align // CenterY 为 true 时文字在自身高度内垂直居中（按钮默认开启）。 CenterY bool Wrap bool Opacity float32 Clip bool Scrollable bool // 绝对定位：相对父容器内边距区域定位（Stack 叠层与浮层用） PosAbs bool AbsL, AbsT, AbsR, AbsB float32 HasL, HasT, HasR, HasB bool AbsCenterX, AbsCenterY bool // Focusable 可获取键盘焦点（输入框等）；Animated 每帧重绘（加载动画/骨架屏） Focusable bool Animated bool // AnimSlow 低帧率动画（光标闪烁 / 骨架屏流光等周期型动画）： // 仍然持续重绘，但按 ~30Hz 而不是 120Hz，避免装饰动画把 CPU 拉满。 AnimSlow bool // BackdropBlur > 0 时对该节点区域应用毛玻璃背景模糊（离屏 FBO + Kawase） BackdropBlur float32 // BackdropTint 毛玻璃上的叠加色 BackdropTint gpu.Paint // CacheLayer 把该子树首次绘制的结果快照成纹理，内容未变时后续帧直接贴回： // 整棵子树不再产生顶点、不再查字形、不再走 SDF 着色，一次 DrawImage 搞定。 // 要求：节点自带**不透明底色**且无圆角 / 描边 / 阴影，子树内无动画、毛玻璃、 // 视频、网页、图表、图片（否则不会启用，按普通绘制走）。 // 适合：大卡片、长文档块、自绘的复杂控件、滚动后的历史内容。 CacheLayer bool // Scrollbar 滚动条可见（默认可见） Scrollbar bool // NoRing 关闭统一焦点环（默认开启，键盘聚焦时绘制）。 NoRing bool // DrawX / DrawY 绘制期位移（FLIP 布局动画、抖动等，不影响布局与命中基准）。 DrawX, DrawY float32 }` — Style 声明式样式（类似 CSS 的子集）。
- **函数** `func St() *Style` — St 新建样式。
- **方法** `func Style.W(v float32) *Style`
- **方法** `func Style.H(v float32) *Style`
- **方法** `func Style.Wp(v float32) *Style`
- **方法** `func Style.Hp(v float32) *Style`
- **方法** `func Style.MinWidth(v float32) *Style`
- **方法** `func Style.MaxWidth(v float32) *Style`
- **方法** `func Style.MinHeight(v float32) *Style`
- **方法** `func Style.MaxHeight(v float32) *Style`
- **方法** `func Style.Padding(v float32) *Style`
- **方法** `func Style.PadXY(h, v float32) *Style`
- **方法** `func Style.PadOnly(t, r, b, l float32) *Style`
- **方法** `func Style.Mar(v float32) *Style`
- **方法** `func Style.MarXY(h, v float32) *Style`
- **方法** `func Style.MarOnly(t, r, b, l float32) *Style`
- **方法** `func Style.GapOf(v float32) *Style`
- **方法** `func Style.GrowTo(v float32) *Style`
- **方法** `func Style.FlexBasis(v float32) *Style`
- **方法** `func Style.Self(a Align) *Style`
- **方法** `func Style.ColsN(n int) *Style` — ColsN 设置网格/瀑布流列数（0 = 自动）。
- **方法** `func Style.ColWidth(v float32) *Style` — ColWidth 设置固定列宽（>0 且未指定列数时按可用宽度自动算列数）。
- **方法** `func Style.BgC(v any) *Style`
- **方法** `func Style.BgGrad(from, to any, angle float32) *Style`
- **方法** `func Style.R(v float32) *Style`
- **方法** `func Style.Rx(tl, tr, br, bl float32) *Style`
- **方法** `func Style.Stroke(w float32, c any) *Style`
- **方法** `func Style.Shadow(blur, y, spread float32, c any) *Style`
- **方法** `func Style.Text(size float32, weight int32, c any) *Style`
- **方法** `func Style.Lh(v float32) *Style`
- **方法** `func Style.Font(family string) *Style` — Font 指定字体家族（需要在系统字体目录可解析）。
- **方法** `func Style.Backdrop(blur float32, tint any) *Style` — Backdrop 毛玻璃背景模糊（blur 为逻辑像素半径，tint 为叠加色）。
- **方法** `func Style.TAlign(a Align) *Style`
- **方法** `func Style.WrapOn() *Style`
- **方法** `func Style.CenterYOn() *Style` — CenterYOn 文字在自身高度内垂直居中。
- **方法** `func Style.Fade(v float32) *Style`
- **方法** `func Style.Clipped() *Style`
- **方法** `func Style.ScrollY() *Style`
- **方法** `func Style.ScrollbarOn() *Style` — ScrollbarOn 滚动条常显（不随闲置淡出）。
- **方法** `func Style.AbsFill() *Style` — AbsFill 铺满父容器内边距区域。
- **方法** `func Style.AbsTL(l, t float32) *Style` — AbsTL 左上角绝对定位。
- **方法** `func Style.AbsTR(r, t float32) *Style` — AbsTR 右上角绝对定位。
- **方法** `func Style.AbsBL(l, b float32) *Style` — AbsBL 左下角绝对定位。
- **方法** `func Style.AbsBR(r, b float32) *Style` — AbsBR 右下角绝对定位。
- **方法** `func Style.AbsCenter() *Style` — AbsCenter 在父容器内水平+垂直居中。
- **方法** `func Style.AbsBottom(b float32) *Style` — AbsBottom 水平居中、贴底定位（Toast 等）。
- **方法** `func Style.FocusOn() *Style` — FocusOn 允许获取键盘焦点。
- **方法** `func Style.AnimOn() *Style` — AnimOn 标记为需要逐帧重绘。
- **方法** `func Style.AnimSlowOn() *Style` — AnimSlowOn 标记为「低帧率持续重绘」（光标闪烁 / 骨架屏流光）。
- **方法** `func Style.CacheOn() *Style` — CacheOn 开启组件栅格缓存：子树首次绘制后快照成纹理，内容未变时后续帧直接贴回。
- **方法** `func Style.NoFocusRing() *Style` — NoFocusRing 关闭统一焦点环（自绘焦点样式的控件用）。
- **方法** `func Style.Translate(x, y float32) *Style` — Translate 绘制期位移（不参与布局，FLIP / 拖拽跟手用）。
- **常量** `const AlignStart = iota`
- **常量** `const AlignCenter`
- **常量** `const AlignEnd`
- **常量** `const AlignStretch`
- **常量** `const AlignSpaceBetween`
- **常量** `const AlignSpaceAround`
- **常量** `const AlignSpaceEvenly`
- **常量** `const Vertical = iota`
- **常量** `const Horizontal`

## `ui/color.go`

- **函数** `func C(v any) gpu.Paint` — C 颜色构造：支持 "#rgb" "#rrggbb" "#rrggbbaa" "rgb(1,2,3)" 以及 uint32 0xRRGGBB。
- **函数** `func Blend(a, b gpu.Paint, t float32) gpu.Paint` — Blend 两色插值。

## `ui/theme.go`

- **类型** `type Theme struct { Name string Text string // 主文字 TextDim string // 次要文字 TextMuted string // 弱化文字 / 占位说明 Bg string // 页面背景 BgSoft string // 次级背景（卡片内分区、按钮底） BgDeep string // 更深背景（表格行/代码块） Stroke string // 描边 Line string // 细分隔线 / 半透明描边 Primary string Primary2 string Success string Warn string Danger string Info string Input string // 输入框底色 Panel string // 浮层（菜单/下拉/提示）底色 PanelDeep string // 弹窗/抽屉底色 Row string // 表格偶数行 RowAlt string // 表格奇数行 Header string // 表头底色 Placeholder string Hover string // 半透明悬停底色 Scrim string // 弹窗遮罩 Track string // 滑块/进度轨道 Glass string // 毛玻璃叠色（Backdrop tint） FocusRing string // 键盘焦点环 }` — Theme 一套色板。
- **函数** `func ThemeMode() string` — ThemeMode 当前主题模式（dark / light / system）。
- **函数** `func IsDark() bool` — IsDark 当前是否深色。
- **函数** `func CurrentTheme() Theme` — CurrentTheme 当前生效的色板。
- **函数** `func UseTheme(mode string)` — UseTheme 切换主题模式并立即应用（dark / light / system）。
- **函数** `func SystemPrefersDark() bool` — SystemPrefersDark 读取系统是否使用深色模式（读取失败时按深色处理）。
- **变量** `var Dark = Theme{ Name: "dark", Text: "#eef2ff", TextDim: "#98a3c0", TextMuted: "#6b7694", Bg: "#101827", BgSoft: "#151f33", BgDeep: "#0a1120", Stroke: "#243049", Line: "rgba(255,255,255,0.08)", Primary: "#6366f1", Primary2: "#8b5cf6", Success: "#34d399", Warn: "#fbbf24", Danger: "#f87171", Info: "#38bdf8", Input: "#0c1322", Panel: "#131c2e", PanelDeep: "#0f1728", Row: "#0e1626", RowAlt: "#0b1220", Header: "#0c1424", Placeholder: "#4d5670", Hover: "rgba(255,255,255,0.05)", Scrim: "rgba(4,7,14,0.62)", Track: "#1e2740", Glass: "rgba(11,16,32,0.40)", FocusRing: "#8b93ff", }` — Dark 深色主题（默认）。
- **变量** `var Light = Theme{ Name: "light", Text: "#0f172a", TextDim: "#475569", TextMuted: "#7c8aa0", Bg: "#ffffff", BgSoft: "#f2f5fb", BgDeep: "#f8fafc", Stroke: "#dbe2ee", Line: "rgba(15,23,42,0.10)", Primary: "#4f46e5", Primary2: "#7c3aed", Success: "#059669", Warn: "#d97706", Danger: "#dc2626", Info: "#0284c7", Input: "#f7f9fd", Panel: "#ffffff", PanelDeep: "#ffffff", Row: "#ffffff", RowAlt: "#f7f9fd", Header: "#eef2f9", Placeholder: "#94a3b8", Hover: "rgba(15,23,42,0.05)", Scrim: "rgba(15,23,42,0.38)", Track: "#e2e8f0", Glass: "rgba(255,255,255,0.55)", FocusRing: "#4f46e5", }` — Light 浅色主题。
- **变量** `var ColText = Dark.Text`
- **变量** `var ColTextDim = Dark.TextDim`
- **变量** `var ColTextMuted = Dark.TextMuted`
- **变量** `var ColBg = Dark.Bg`
- **变量** `var ColBgSoft = Dark.BgSoft`
- **变量** `var ColBgDeep = Dark.BgDeep`
- **变量** `var ColStroke = Dark.Stroke`
- **变量** `var ColLine = Dark.Line`
- **变量** `var ColPrimary = Dark.Primary`
- **变量** `var ColPrimary2 = Dark.Primary2`
- **变量** `var ColSuccess = Dark.Success`
- **变量** `var ColWarn = Dark.Warn`
- **变量** `var ColDanger = Dark.Danger`
- **变量** `var ColInfo = Dark.Info`
- **变量** `var ColInput = Dark.Input`
- **变量** `var ColPanel = Dark.Panel`
- **变量** `var ColPanelDeep = Dark.PanelDeep`
- **变量** `var ColRow = Dark.Row`
- **变量** `var ColRowAlt = Dark.RowAlt`
- **变量** `var ColHeader = Dark.Header`
- **变量** `var ColPlaceholder = Dark.Placeholder`
- **变量** `var ColHover = Dark.Hover`
- **变量** `var ColScrim = Dark.Scrim`
- **变量** `var ColTrack = Dark.Track`
- **变量** `var ColGlass = Dark.Glass`
- **变量** `var ColFocus = Dark.FocusRing`
- **变量** `var ColSurface = Dark.Bg`
- **变量** `var ColSurfaceMuted = Dark.BgSoft`
- **变量** `var ColSurfaceRaised = Dark.Panel`
- **变量** `var ColSurfaceDeep = Dark.BgDeep`
- **变量** `var ColOnAccent = "#ffffff"`
- **常量** `const ThemeDark = "dark"` — 主题模式。
- **常量** `const ThemeLight = "light"` — 主题模式。
- **常量** `const ThemeSystem = "system"` — 主题模式。

## `ui/theme_tokens.go`

- **类型** `type Tokens struct { Surface string // 页面主背景 SurfaceMuted string // 次级背景（卡片内分区、按钮底） SurfaceRaised string // 浮层/弹窗背景 SurfaceDeep string // 更深背景（代码块、输入框） OnSurface string // 主文字 OnSurfaceMuted string // 次要文字 OnSurfaceFaint string // 弱化文字 / 占位 Border string // 描边 BorderSubtle string // 分隔线 Accent string // 主色 AccentAlt string // 主色渐变终点 OnAccent string // 主色上的文字 Success string Warning string Danger string Info string Overlay string // 遮罩 Track string // 轨道 FocusRing string // 焦点环 Glass string // 毛玻璃叠色 Header string Row string RowAlt string Hover string Placeholder string }` — Tokens 语义色板。
- **函数** `func TokensOf() Tokens` — TokensOf 当前主题的语义 token。
- **函数** `func SetTokens(t Tokens)` — SetTokens 覆盖当前主题的语义色（空串字段保持原值），立即生效并记住覆盖。
- **函数** `func ClearTokenOverrides()` — ClearTokenOverrides 清除用户覆盖，回到主题自带配色。
- **函数** `func RegisterTheme(name string, t Theme)` — RegisterTheme 注册一套具名主题（name 为 "dark"/"light" 时会覆盖内置主题）。
- **函数** `func ThemeNames() []string` — ThemeNames 已注册的主题名（含内置 dark / light）。
- **函数** `func UseThemeName(name string)` — UseThemeName 切换到已注册/内置的具名主题；未知名称保持当前主题。
- **函数** `func ThemeTitle(name string) string` — ThemeTitle 主题显示名（界面切换器用）。

## `ui/scale.go`

- **函数** `func UIScale() float32` — UIScale 当前界面缩放（1 = 100%）。
- **函数** `func SetUIScale(k float32)` — SetUIScale 设置界面缩放（自动夹取到 60%~150%），立即重建渲染尺寸与字形。
- **函数** `func SetSuperSample(k float32)` — SetSuperSample 设置超高清档位（SSAuto / SSOff / SS125 / SS150 / SS200）。
- **函数** `func SuperSampleMode() float32` — SuperSampleMode 返回当前档位设置（SSAuto 表示跟随 DPI 自动）。
- **函数** `func SetTextSharp(level int)` — SetTextSharp 设置文字锐度档位（0 标准 / 1 清晰 / 2 极致），立即重建字形并重绘。
- **函数** `func TextSharp() int` — TextSharp 当前文字锐度档位。
- **函数** `func SuperSample() float32` — SuperSample 返回当前窗口实际生效的超采样倍数（含 DPI 自动判定，不含像素预算降档）。
- **常量** `const UIScaleMin = 0.6`
- **常量** `const UIScaleMax = 2.5`
- **常量** `const SSAuto = 0` — 超高清（整帧超采样）档位：SSAuto = 自动，SSOff = 关闭，其余为显式倍数。
- **常量** `const SSOff = 1` — 超高清（整帧超采样）档位：SSAuto = 自动，SSOff = 关闭，其余为显式倍数。
- **常量** `const SS125 = 1.25` — 超高清（整帧超采样）档位：SSAuto = 自动，SSOff = 关闭，其余为显式倍数。
- **常量** `const SS150 = 1.5` — 超高清（整帧超采样）档位：SSAuto = 自动，SSOff = 关闭，其余为显式倍数。
- **常量** `const SS200 = 2` — 超高清（整帧超采样）档位：SSAuto = 自动，SSOff = 关闭，其余为显式倍数。

## `ui/event.go`

- **类型** `type EventType int` — Event 平台无关的输入/窗口事件。桌面端由 Win32 消息翻译而来，浏览器端由 DOM 事件翻译，
- **类型** `type Event struct { Type EventType X, Y float32 DX, DY float32 Button int Key uint32 Char rune Runes []rune Width float32 Height float32 Scale float32 Text string Caret int // Pointer 指针类型：0 = 鼠标，1 = 触摸/手写笔（移动端）。 Pointer int Paths []string HotkeyID int }` — Event 一个输入事件（坐标均为逻辑像素）。
- **常量** `const EvNone = iota`
- **常量** `const EvResize`
- **常量** `const EvClose`
- **常量** `const EvFocusGained`
- **常量** `const EvFocusLost`
- **常量** `const EvMouseMove`
- **常量** `const EvMouseDown`
- **常量** `const EvMouseUp`
- **常量** `const EvMouseWheel`
- **常量** `const EvMouseLeave`
- **常量** `const EvKeyDown`
- **常量** `const EvKeyUp`
- **常量** `const EvChar`
- **常量** `const EvCompositionStart`
- **常量** `const EvComposition`
- **常量** `const EvCompositionEnd`
- **常量** `const EvSizeMoveStart`
- **常量** `const EvSizeMoveEnd`
- **常量** `const EvDropFiles`
- **常量** `const EvHotkey`
- **常量** `const EvTouchDown`
- **常量** `const EvTouchMove`
- **常量** `const EvTouchUp`
- **常量** `const EvTextInput`
- **常量** `const EvBack`
- **常量** `const VK_BACK = 0x08` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_TAB = 0x09` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_RETURN = 0x0D` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_SHIFT = 0x10` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_CONTROL = 0x11` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_MENU = 0x12` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_ESCAPE = 0x1B` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_SPACE = 0x20` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_LEFT = 0x25` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_UP = 0x26` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_RIGHT = 0x27` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_DOWN = 0x28` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_DELETE = 0x2E` — 常用虚拟键码（与 Windows VK_* 一致）。
- **常量** `const VK_F11 = 0x7A` — 常用虚拟键码（与 Windows VK_* 一致）。

## `ui/focus.go`

- **函数** `func FocusVisible() bool` — FocusVisible 当前是否应绘制焦点环（键盘导航中为 true）。
- **函数** `func SetFocusVisible(v bool)` — SetFocusVisible 手动控制焦点环可见性（触摸端可关闭）。
- **函数** `func PushFocusScope(key string, trap bool)` — PushFocusScope 进入一个焦点作用域（trap=true 时 Tab 只在作用域内循环）。
- **函数** `func PopFocusScope(key string)` — PopFocusScope 退出作用域并恢复进入前的焦点（若该节点仍存在）。
- **函数** `func FocusScopeDepth() int` — FocusScopeDepth 当前作用域层数（调试 / 测试用）。
- **函数** `func ResetFocusScopes()` — ResetFocusScopes 清空作用域（窗口销毁 / 切页时调用）。
- **函数** `func FocusNext()` — FocusNext 焦点移到下一个可聚焦节点（dir=1）或上一个（dir=-1）。
- **函数** `func FocusPrev()`
- **函数** `func OpenOverlay(key string, close func() bool)` — OpenOverlay 登记一个打开的浮层。close 返回 true 表示已消费 ESC。
- **函数** `func CloseOverlay(key string)` — CloseOverlay 移除登记（浮层已经自行关闭时调用）。
- **函数** `func OverlayDepth() int` — OverlayDepth 当前登记的浮层数量。
- **函数** `func ResetOverlays()` — ResetOverlays 清空浮层登记（窗口销毁时调用）。

## `ui/a11y.go`

- **类型** `type A11yRole int` — A11yRole 无障碍角色（对应 UIA ControlType 的常用子集）。
- **类型** `type A11yInfo struct { Role A11yRole Name string Desc string }` — A11yInfo 节点无障碍信息。
- **类型** `type A11yEntry struct { Key string Role A11yRole Name string Desc string Focus bool }` — A11yEntry 导出的一项（Key 为节点 Key，Path 为树中位置）。
- **函数** `func A11yRoleName(r A11yRole) string` — A11yRoleName 角色名称（调试与导出用）。
- **函数** `func A11ySnapshot() []A11yEntry` — A11ySnapshot 按先序导出界面中的无障碍节点（无名无角色的节点跳过）。
- **常量** `const A11yNone = iota`
- **常量** `const A11yButton`
- **常量** `const A11yText`
- **常量** `const A11yLink`
- **常量** `const A11yCheckBox`
- **常量** `const A11yRadio`
- **常量** `const A11ySwitch`
- **常量** `const A11ySlider`
- **常量** `const A11yTextField`
- **常量** `const A11yList`
- **常量** `const A11yListItem`
- **常量** `const A11yTab`
- **常量** `const A11yDialog`
- **常量** `const A11yMenu`
- **常量** `const A11yMenuItem`
- **常量** `const A11yProgress`
- **常量** `const A11yImage`
- **常量** `const A11yTable`
- **常量** `const A11yTree`

## `ui/shortcuts.go`

- **类型** `type ShortcutScope string` — ShortcutScope 快捷键作用域。
- **类型** `type Shortcut struct { ID string Keys string // "Ctrl+S" / "F11" / "Ctrl+Shift+P" Label string Group string Run func() }` — Shortcut 一条快捷键。
- **函数** `func RegisterShortcut(scope ShortcutScope, sc Shortcut) bool` — RegisterShortcut 登记快捷键；同一作用域内组合冲突时返回 false 并保留原绑定。
- **函数** `func UnregisterShortcut(scope ShortcutScope, id string)` — UnregisterShortcut 移除快捷键。
- **函数** `func ShortcutConflict(scope ShortcutScope, keys string) string` — ShortcutConflict 返回占用该组合的快捷键 ID（无冲突返回空串）。
- **函数** `func SetShortcutScope(scope ShortcutScope)` — SetShortcutScope 压入/设置当前面板作用域（空串回到 global）。
- **函数** `func PushShortcutScope(scope ShortcutScope)` — PushShortcutScope 进入模态作用域（退出时 PopShortcutScope）。
- **函数** `func PopShortcutScope()` — PopShortcutScope 退出模态作用域。
- **函数** `func DispatchShortcuts(vk uint32) bool` — DispatchShortcuts 在当前按键下查找并执行快捷键（App 的事件循环调用）。
- **函数** `func ShortcutList(scope ShortcutScope) []Shortcut` — ShortcutList 按作用域列出快捷键（帮助面板用）。
- **函数** `func ShortcutsHelp(scope ShortcutScope) *Node` — ShortcutsHelp 快捷键帮助列表（分组展示）。
- **函数** `func ShortcutSerialize(scope ShortcutScope) string` — ShortcutSerialize 把当前全局快捷键序列化为 "id=keys" 行（保存到偏好）。
- **函数** `func ShortcutRebind(scope ShortcutScope, id, keys string) bool` — ShortcutRebind 修改绑定（冲突时返回 false）。
- **常量** `const ScopeGlobal = "global"`
- **常量** `const ScopePanel = "panel"`
- **常量** `const ScopeModal = "modal"`

## `ui/selection.go`

- **类型** `type SelectionState struct { Active bool X0, Y0 float32 X1, Y1 float32 Text string }` — SelectionState 选择状态。
- **函数** `func SelectedText() string` — SelectedText 当前选中的文字。
- **函数** `func HasSelection() bool` — HasSelection 是否存在选中文字。
- **函数** `func ClearSelection()` — ClearSelection 清除选区。
- **函数** `func SelectionArea(key string, children ...*Node) *Node` — SelectionArea 包裹内容并允许拖选文本。

## `ui/textedit.go`

- **类型** `type TextEditAction int` — TextEditAction 文本编辑动作（剪贴板 / 全选）。
- **类型** `type TextSel struct { Runes []rune Caret int Anchor int }` — TextSel 文本编辑状态：文本 + 光标 + 选区锚点（下标均按 rune 计）。
- **类型** `type TextSeg struct { Text string Sel bool }` — TextSeg 一行按选区切分后的片段（渲染高亮用）。
- **函数** `func RegisterTextEdit(key string, fn func(TextEditAction) bool)` — RegisterTextEdit 登记一个控件的文本编辑动作（输入框内部调用）。
- **函数** `func TextEditBound(key string) bool` — TextEditBound 判断 Key 对应控件是否支持文本编辑。
- **函数** `func DispatchTextEdit(vk uint32) bool` — DispatchTextEdit 把 Ctrl+A/C/X/V 交给获得焦点的文本控件。
- **函数** `func NewTextSel(value string, caret, anchor int) *TextSel` — NewTextSel 用当前文本与记忆的光标 / 锚点构造状态（越界自动夹取）。
- **方法** `func TextSel.Clamp()` — Clamp 把光标与锚点夹取到 [0, 文本长度]。
- **方法** `func TextSel.Text() string` — Text 当前文本。
- **方法** `func TextSel.Selection() (lo, hi int, ok bool)` — Selection 返回选区区间 [lo, hi) 与是否存在选区。
- **方法** `func TextSel.Selected() string` — Selected 选中的文本（无选区返回空串）。
- **方法** `func TextSel.Move(delta int, keep bool)` — Move 移动光标（delta 按 rune）；keep=false 时折叠选区（普通方向键）。
- **方法** `func TextSel.SetCaret(pos int, keep bool)` — SetCaret 定位光标；keep=false 时折叠选区。
- **方法** `func TextSel.SelectAll()` — SelectAll 全选。
- **方法** `func TextSel.DeleteSelection() bool` — DeleteSelection 删除选中区间（无选区返回 false）。
- **方法** `func TextSel.Insert(s string) bool` — Insert 在光标处插入文本（有选区先删除选区）。
- **方法** `func TextSel.Backspace() bool` — Backspace 删除光标前一个 rune（有选区时先删选区）。
- **方法** `func TextSel.DeleteForward() bool` — DeleteForward 删除光标后一个 rune（有选区时先删选区）。
- **方法** `func TextSel.Copy() bool` — Copy 把选中文本写入系统剪贴板。
- **方法** `func TextSel.Cut() bool` — Cut 剪切选中文本到剪贴板。
- **方法** `func TextSel.Paste(multiline bool) bool` — Paste 粘贴剪贴板文本（multiline=false 时把换行替换为空格）。
- **方法** `func TextSel.IndexAtX(x, size float32, weight int32) int` — IndexAtX 单行文本按横向偏移定位光标（x 相对文本起点）。
- **方法** `func TextSel.IndexAtPoint(x, y, lineH, size float32, weight int32) int` — IndexAtPoint 多行文本按坐标定位光标（x/y 相对文本左上角，行高 lineH）。
- **方法** `func TextSel.SelLineSegments(start, end int) []TextSeg` — SelLineSegments 把 [start,end) 这段 rune（一行）按当前选区切成若干片段。
- **常量** `const TextEditCopy = iota`
- **常量** `const TextEditCut`
- **常量** `const TextEditPaste`
- **常量** `const TextEditSelectAll`

## `ui/textseg.go`

- **函数** `func TextSegNodes(t *TextSel, start, end int, size float32, weight int32, color string, centerY bool) []*Node` — TextSegNodes 把 [start,end) 这段文本按当前选区切段渲染成节点。

## `ui/layers.go`

- **函数** `func DropCachedLayers()` — DropCachedLayers 丢弃所有窗口的组件栅格缓存。
- **函数** `func CachedLayer(key string, bg any, child *Node) *Node` — CachedLayer 把静态内容包一层「栅格缓存容器」。
- **函数** `func SetCacheLayers(on bool)` — SetCacheLayers 开启/关闭自动组件缓存（进程级默认值，新建窗口生效）。
- **函数** `func CacheLayersEnabled() bool` — CacheLayersEnabled 当前是否开启了自动组件缓存。
- **函数** `func CountCacheRoots(root *Node, w, h float32) (roots int, area float32)` — CountCacheRoots 诊断用：对一棵已布局的节点树重新标记，返回「缓存根」的数量

## `ui/debug.go`

- **函数** `func LayoutForTest(root *Node, w, h float32, face string, scale float32)` — LayoutForTest 在无窗口环境下布局一棵树（供 tools/layouttest 使用）。
- **函数** `func DebugDump(n *Node)` — DebugDump 输出整棵布局树到 stdout（用于 tools 调试）。

## `ui/caps.go`

- **类型** `type Cap int` — 平台能力查询：同一套 UI 代码要在 Windows / Linux / macOS / Web / 移动端编译运行，
- **函数** `func CapName(c Cap) string` — CapName 返回能力的中文名称。
- **函数** `func Capabilities() []Cap` — Capabilities 当前平台支持的能力列表（由各平台文件提供）。
- **函数** `func HasCap(c Cap) bool` — HasCap 当前平台是否支持某能力。
- **函数** `func UnsupportedCaps() []Cap` — UnsupportedCaps 当前平台缺失的能力（便于界面上统一提示）。
- **常量** `const CapVideo = iota`
- **常量** `const CapAudio`
- **常量** `const CapWebView`
- **常量** `const CapTray`
- **常量** `const CapClipboard`
- **常量** `const CapDialogs`
- **常量** `const CapHotkey`
- **常量** `const CapDragDrop`
- **常量** `const CapCompose`
- **常量** `const CapSystemDark`
- **常量** `const CapKeepAwake`
- **常量** `const CapRecycleBin`
- **常量** `const CapTouch`
- **常量** `const CapPrint`
- **常量** `const CapScreenCapture`
- **常量** `const CapClipboardWatch`
- **常量** `const CapMultiMonitor`
- **常量** `const CapSingleInstance`
- **常量** `const CapModernNotify`
- **常量** `const CapAutoStart`
- **常量** `const CapLast = CapAutoStart` — CapLast 最后一个能力值（遍历 / 测试用；新增能力时同步更新）。
