<!-- group: 入门 -->
# 快速开始 · 入口 / 选项 / 三端构建

## 1. 最小可运行程序

```go
package main

import "guis/ui"

func init() {
    // 必须在 init 里注册：Android 的 NativeActivity 在 dlopen 阶段就回调，
    // 早于 main()，只在 main 里准备参数会黑屏。
    ui.SetEntry(func() (ui.Options, func() *ui.Node) {
        return ui.Options{
            Title:        "Hello",
            Width:        900,
            Height:       620,
            NoTitleBar:   true,      // 自绘标题栏
            CornerRadius: 5,
            Background:   "#070a12",
        }, view
    })
}

func view() *ui.Node {
    return ui.Center(
        ui.Column(
            ui.Text("Hello GUIS").St(ui.St().Text(24, 700, ui.ColText)),
            ui.Text("纯 Go 自绘 GUI").St(ui.St().Text(13, 400, ui.ColTextMuted)),
        ).St(ui.St().GapOf(8)),
    ).St(ui.St().Hp(100).BgC(ui.ColBg))
}

func main() { ui.Main() }
```

## 2. 三端同一个入口

| 平台 | 入口实现 | 构建 |
| --- | --- | --- |
| Windows / Linux / macOS | `main_entry_desktop.go` → Win32 / X11 / Cocoa | `go build ./...` |
| 浏览器 | `boot_wasm.go`（先自举字体再起 canvas + WebGL2） | `GOOS=js GOARCH=wasm go build` |
| Android | `main_entry_android.go`（NativeActivity + EGL/GLES3） | `./build-android-apk.ps1` |

`ui.SetEntry` 返回的第二个值就是整页构建函数 `func() *Node`，**每帧调用一次**。
`ui.Main()` 阻塞运行消息泵 + 渲染循环。

## 3. Options 全部字段

| 字段 | 说明 |
| --- | --- |
| `Title` | 窗口标题（自绘标题栏时也要设，任务栏显示它） |
| `Width` / `Height` | 逻辑像素初始尺寸 |
| `FontFamily` | 字体家族预设：空 / `Microsoft YaHei`（默认）、`SimSun`、`SimHei`、`Consolas`、`Segoe UI`、`Symbol`、`Emoji`；未命中回落微软雅黑 |
| `Background` | 窗口背景色（`any`：`#rrggbb` / `#rrggbbaa` / `gpu.Paint`） |
| `Resizable` | 是否可缩放窗口 |
| `DumpLayout` | 非空时首帧后把布局树写入该文件（排错用） |
| `Hidden` | 创建后不显示（配合托盘常驻） |
| `NoTitleBar` | 去掉系统标题栏（自绘标题栏场景） |
| `CornerRadius` | 窗口圆角（Win11 走 DWM 原生，低版本回退 `SetWindowRgn`） |
| `Icon` | 窗口 / 任务栏图标（.ico 字节），空则用内置图标 |
| `Locale` | 启动语言（BCP-47，如 `"en-US"`）。留空 = 自动：偏好 `ui.locale` > 系统语言 > `en-US`（见 `73` 章） |

窗口选项只在**创建时**生效，之后改标题用 `ui.Current().SetTitle(...)`。

## 4. 两种运行方式

```go
// ① 单窗口（最常见）
ui.Main()

// ② 自己控制（需要先建窗口再做事，或多窗口）
err := ui.Run(opts, view)          // 阻塞
app, err := ui.NewWindow(opts, view) // 在当前 UI 线程再开一个窗口
```

`ui.Current()` 返回当前活跃窗口的 `*App`；`ui.WindowCount()` 返回存活窗口数。

> `go run` 启动时会**自动开启热重载**：改完 `.go` 源码自动重编译并重新出图（详见 `03` 热重载）。

## 5. 命令行 / URL 参数

```go
// 桌面读 os.Args，Web 读 ?key=value，移动端恒空串
if v := ui.StartupQuery("case"); v != "" { /* 初始页面 */ }
```

桌面上常用 `os.Args` 自行解析（`gallery` 的 `--case` / `--dump` / `--fullscreen` 就是这么做）。

## 6. 构建与产物

```bash
# 桌面
go build -o dist/guis.exe ./cmd/gallery

# 带版本信息
go build -ldflags "-X guis/internal/version.Number=1.2.3 \
  -X guis/internal/version.Channel=release" -o dist/guis.exe ./cmd/gallery

# Web：产物 = app.wasm + wasm_exec.js + web/ 下的壳页面
GOOS=js GOARCH=wasm go build -o web/app.wasm ./cmd/gallery

# 安卓
./build-android-apk.ps1
```

仓库脚本：`build.ps1`（通用）、`build-multi.ps1`（多平台）、`build-android.ps1` / `build-android-apk.ps1`、
`installer/guis.nsi`（NSIS 安装包，`makensis` 编译）。

## 7. 调试开关（环境变量）

| 变量 | 作用 |
| --- | --- |
| `GUIS_FPS=1` | 每帧输出构建 / 布局 / 绘制耗时与动画节点清单 |
| `GUIS_CACHE=0` | 关闭组件栅格缓存（`ui.SetCacheLayers(false)` 等价） |
| `GUIS_DECODER=system` | 强制走系统解码器（Windows Media Foundation），验证回退链路 |
| `GUIS_PDF_FONT` | 指定导出 PDF 时嵌入的 TrueType 中文字体 |
| `GUIS_AUTOSTART_CMD` | 覆盖开机自启注册的命令行 |
| `GUIS_HOT=0` | 关闭热重载（`=1` 则强制开启；留空按「是否 go run」自动判断） |
| `GUIS_LSP_CMD` | 代码编辑器示例页使用的语言服务器命令（默认 `gopls`） |

## 8. 热重载（改代码即刷新）

`go run ./cmd/gallery` 起的窗口，保存 `.go` 文件后会自动重编译并重启界面，不用来回 Ctrl+C：

```go
// 可选：把业务状态带到下一进程（窗口几何默认已保留）
func init() {
    ui.SetHotState(
        func() []byte { return []byte(currentPage) },
        func(b []byte) { currentPage = string(b) },
    )
}
```

* 判断是否处于热重载：`ui.HotReloadActive()`；
* 关闭：`GUIS_HOT=0 go run ./cmd/gallery`；
* 仅桌面端生效（wasm / Android / iOS 为空操作），每次重载窗口会重建一次。

## 9. 三个必踩的坑

1. **别在 `main()` 里调 `SetEntry`** —— 安卓端会黑屏（回调早于 main）。
2. **每帧重建节点树** —— 别把 `*ui.Node` 存到全局复用；要保留状态请用 `Key` + 全局变量/结构体。
3. **浮层必须放进根 Stack** —— `MenuLayer` / `ToastLayer` / `NotifyLayer` / `CommandPaletteLayer`
   等返回的是浮层节点，要作为根 `Stack` 的**靠后**子节点，否则会被内容区压住。
