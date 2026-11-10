<!-- group: 附录 -->
# 附录 E · 平台能力矩阵与降级表

## 1. 能力查询 API

```go
type Cap int   // CapVideo / CapAudio / CapWebView / CapTray / CapClipboard / CapDialogs
               // CapHotkey / CapDragDrop / CapCompose / CapSystemDark / CapKeepAwake
               // CapRecycleBin / CapTouch / CapPrint / CapScreenCapture / CapClipboardWatch
               // CapMultiMonitor / CapSingleInstance / CapModernNotify / CapAutoStart
               // CapFileWatch / CapKnownFolders / CapDragOut / CapFileIcon / CapNativeDialog
               // CapSystemInfo / CapTaskbar / CapWindowFX / CapClipboardRich
               // CapPowerStatus / CapSession / CapInputSim / CapShellLink

ui.HasCap(ui.CapPrint)          // 当前平台是否支持
ui.Capabilities()               // 全部支持的能力
ui.UnsupportedCaps()            // 缺失的能力（界面上统一提示）
ui.CapName(ui.CapPrint)         // 中文名："打印"
ui.CapLast                      // 最后一个能力值（遍历 / 测试用）
```

**正确写法**：`if ui.HasCap(ui.CapHotkey) { ... }`；
**不要用** `runtime.GOOS == "windows"` 判断能力（安卓 / 浏览器的差异会漏）。

## 2. 能力矩阵

| 能力 | Windows | Linux | macOS | Web(wasm) | Android / iOS |
| --- | --- | --- | --- | --- | --- |
| `CapVideo` 视频 | ✅ | ✅(ffmpeg) | ✅(ffmpeg) | ✅ `<video>` | ✗ |
| `CapAudio` 音频 | ✅ | ✅ | ✅ | ✅ `<video>` | ✗ |
| `CapWebView` 内嵌浏览器 | ✅ WebView2 | ✗ 占位 | ✗ 占位 | ✅ iframe | ✗ 占位 |
| `CapTray` 托盘 | ✅ | ✅ XEmbed※ | ✅ NSStatusItem | ✗ | ✗ |
| `CapClipboard` 剪贴板 | ✅ | ✅ | ✅ | 仅写 | ✗ |
| `CapClipboardWatch` 剪贴板监听 | ✅ | ✅ | ✅ | ✗ | ✗ |
| `CapDialogs` 文件对话框 | ✅ | ✅ | ✅ | 仅选文件 | ✗ |
| `CapHotkey` 全局热键 | ✅ Win32 | ✅ XGrabKey | ✅ CGEventTap※ | ✗ | ✗ |
| `CapDragDrop` 文件拖入 | ✅ WM_DROPFILES | ✅ XDND | ✅ NSDragging | ✗ | ✗ |
| `CapCompose` DirectComposition | ✅(可选) | ✗ | ✗ | ✗ | ✗ |
| `CapSystemDark` 系统深浅色 | ✅ | ✅ | ✅ | ✅ | ✗ |
| `CapKeepAwake` 阻止休眠 | ✅ | ✅ | ✅ | ✅ WakeLock | ✗ |
| `CapRecycleBin` 回收站 | ✅ | ✅ | ✅ | ✗ | ✗ |
| `CapTouch` 触摸 | ✗ | ✗ | ✗ | ✅ | ✅ |
| `CapPrint` 打印 | ✅ | ✗ | ✗ | ✗ | ✗ |
| `CapScreenCapture` 截屏 | ✅ | ✗ | ✗ | ✗ | ✗ |
| `CapMultiMonitor` 多显示器 | ✅ | ✗ | ✗ | ✗ | ✗ |
| `CapSingleInstance` 单实例 | ✅ | 视为首实例 | 视为首实例 | ✗ | ✗ |
| `CapModernNotify` 现代通知 | ✅ | ✅ | ✅ | ✅ | ✗ |
| `CapAutoStart` 开机自启 | ✅ | ✅ | ✅ | ✗ | ✗ |
| `CapFileWatch` 文件监听 | ✅ ReadDirectoryChangesW | ✅ 扫描兜底 | ✅ 扫描兜底 | 空实现 | ✅ 扫描兜底 |
| `CapKnownFolders` 标准目录 | ✅ SHGetKnownFolderPath | ✅ XDG | ✅ Apple 约定 | ✗ | ✅ 部分 |
| `CapDragOut` 文件拖出 | ✅ DoDragDrop | ✗ | ✗ | ✗ | ✗ |
| `CapFileIcon` 文件图标 | ✅ SHGetFileInfo | ✗ | ✗ | ✗ | ✗ |
| `CapNativeDialog` 原生消息框 | ✅ MessageBoxW | ✅ zenity/kdialog | ✅ osascript | ✗ | ✗ |
| `CapSystemInfo` 系统信息 | ✅ RtlGetVersion | ✅ | ✅ pmset | ✗ | 部分 |
| `CapTaskbar` 任务栏集成 | ✅ ITaskbarList3 | ✗ | ✗ | ✗ | ✗ |
| `CapWindowFX` 窗口特效 | ✅ | ✗ | ✗ | ✗ | ✗ |
| `CapClipboardRich` 富文本剪贴板 | ✅ CF_HTML/RTF | ✅ xclip | ✗ | ✗ | ✗ |
| `CapPowerStatus` 电源状态 | ✅ GetSystemPowerStatus | ✅ sysfs | ✅ pmset | ✗ | ✗ |
| `CapSession` 会话控制 | ✅ | ✅ loginctl/systemctl | ✅ osascript | ✗ | ✗ |
| `CapInputSim` 输入模拟 | ✅ SendInput | ✗ | ✗ | ✗ | ✗ |
| `CapShellLink` 快捷方式 | ✅ IShellLink | ✅ .desktop | ✅ .webloc | ✗ | ✗ |

> 「占位」= 接口存在且可调用，但不产生真实效果（WebView 返回不支持）。
> ※ macOS 的 `CGEventTap` 需要「输入监控」授权（未授权时注册返回 0）；
> ※ X11 的托盘需要桌面提供 XEmbed 宿主（`_NET_SYSTEM_TRAY_Sn` 有主），
>   Sway 这类纯 wlroots 桌面只有 StatusNotifierItem，`NewTray` 会返回错误。
> 「✗」= 调用是安全空操作或返回错误，UI 应当隐藏 / 置灰对应入口。

### 不在能力位上的组件

下面几个特性不走 `HasCap`（不是平台驱动能力），但平台覆盖范围同样需要知道：

| 特性 | Windows | Linux | macOS | Web(wasm) | Android / iOS | 说明 |
| --- | --- | --- | --- | --- | --- |
| `ui.Icon` 矢量图标 | ✅ | ✅ | ✅ | ✅ | ✅ | 纯自绘光栅化，不依赖系统字体 |
| `ui.CodeEditor` 代码编辑器 | ✅ | ✅ | ✅ | ✅ | ✅ | 纯自绘 + 软件排版，渲染与交互全平台一致 |
| LSP 客户端 | ✅ | ✅ | ✅ | ✗ `Start()` 报错 | ✗ 同左 | 需要子进程，界面应隐藏入口 |
| 热重载 | ✅ | ✅ | ✅ | ✗ 空实现 | ✗ 空实现 | 需要 `go build` + 重启子进程 |

## 3. 各平台后端一览

| 平台 | 窗口后端 | 渲染 | 输入 | 原生句柄 |
| --- | --- | --- | --- | --- |
| Windows | `internal/win32`（Win32 + WGL） | OpenGL + DirectComposition（可选合成） | WM_* 消息 | `HWND` |
| Linux | `internal/x11` | GLX | XI2 / Xlib | X11 `Window` |
| macOS | `internal/cocoa` + `objc` | NSOpenGL | NSEvent | `NSView*` |
| iOS | `internal/uikit` | GLES | UITouch | `UIWindow*` |
| Android | `internal/android`（NativeActivity） | EGL/GLES3 | AInputEvent | `ANativeWindow*` |
| Web | `backend_wasm.go` | canvas + WebGL2 | DOM 事件 | 0 |

统一接口 `ui.Backend`（`SetEventHandler / MakeCurrent / Swap / PixelSize / Scale / …`），
业务代码永远不直接依赖它。

## 4. 降级策略建议

| 场景 | 推荐做法 |
| --- | --- |
| 功能按钮 | `HasCap` 为假时 `DisabledOn(true)` + `TooltipOf` 说明「当前平台不支持」 |
| 菜单项 | 直接不渲染（`ui.If(cap, …)`） |
| 富交互（拖入 / 热键） | 额外提供按钮入口，保证触摸端可用 |
| 媒体 | `BackendName()` 展示实际后端，无后端时展示占位卡 |
| 打印 | 不支持时隐藏「打印」，保留「导出 PDF」（纯 Go，全平台可用） |
| 布局 | 窄屏 / 触摸端把 hover 才出现的操作改为常显 |

## 5. 屏幕尺寸与安全区

```go
w, h := ui.ViewportW(), ui.ViewportH()
scale := ui.PixelScale()          // 物理 / 逻辑
narrow := w < 900                 // 常见断点（gallery 用这个值）
```

安卓端状态栏 / 导航栏由 NativeActivity 处理，`ViewportH` 是可用绘制区，
不需要自己减安全区；浏览器端视口就是 canvas 尺寸，随窗口变化实时更新。
