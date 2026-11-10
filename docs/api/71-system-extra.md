<!-- group: 系统能力 -->
# 桌面增强 · 多显示器 / 截屏 / 更新器 / 窗口置顶

## 1. 显示器

```go
type Monitor struct {
    Index              int
    Name               string
    X, Y, W, H         int      // 物理像素
    WorkX, WorkY       int      // 工作区（去掉任务栏）
    WorkW, WorkH       int
    Scale              float32
    Primary            bool
}

ms := ui.Monitors()                 // 主屏在前（Windows 完整；其它平台可能为空）
ui.MonitorAt(mouseX, mouseY)        // 包含该屏幕坐标的显示器
x, y, w, h := ui.VirtualScreen()    // 所有显示器并集
```

典型用途：把窗口摆到光标所在屏幕、按 DPI 选图、判断「是否主屏」。

## 2. 屏幕截图

```go
img, err := ui.CaptureScreen(x, y, w, h, false)   // 物理像素，支持负坐标 / 多屏
full, err := ui.CaptureFullScreen()               // 主屏
if err == nil {
    // img 是 *image.NRGBA，可直接存 PNG 或喂给 ui.Image 数据源
}
```

非 Windows 端返回 `ErrNotSupported`（`CapScreenCapture` 为假）。

## 3. 窗口置顶 / 焦点

```go
ui.SetAlwaysOnTop(true)
ui.IsAlwaysOnTop()
ui.Current().SetAlwaysOnTop(true)   // 窗口方法等价

ui.Current().Show()      // 显示并置前
ui.Current().Restore()   // 从最小化恢复
ui.Current().Minimize()
```

## 4. 自更新

```go
cfg := ui.UpdateConfig{
    Current:  version.Number,                       // "1.2.3"
    CheckURL: "https://example.com/latest.json",    // {"version","notes","url","sha256","size"}
    DownloadURL: "",                                // 纯文本检查模式下的下载地址
}

info, err := ui.CheckUpdate(cfg)      // 无更新返回 (nil, nil)
if info != nil {
    err = ui.DownloadUpdate(info, "update.exe", func(done, total int64) {
        ui.NotifyProgress("up", float32(done)/float32(total), "下载更新")
    })
    err = ui.VerifySHA256("update.exe", info.SHA256)   // want 为空则跳过
    err = ui.ApplyUpdate("update.exe")                 // Windows：重命名 + 重启脚本
}

ui.CompareVersion("1.3.0", "1.2.9")   // 1
ui.UpdateCheck(cfg)                   // 后台检查，结果下一帧生效
ui.UpdatePanelView(cfg)               // 现成的更新面板节点
```

`UpdateInfo` 字段：`Version / Notes / URL / SHA256 / Size`（带 json tag，可直接对应服务端 JSON）。

## 5. 版本信息

```go
import "guis/internal/version"

version.Number      // "0.1.0"（构建期 -ldflags 覆盖）
version.String()    // "v0.1.0"
version.Title()     // "GUIS v0.1.0"
version.Full()      // 完整描述（关于 / 日志）
version.Current()   // Info{Product, Number, Commit, BuildTime, Channel}
version.IsRelease() // 是否 release 通道
version.Compare(a, b)
version.Parse("1.2.3-rc1")   // (1, 2, 3, "rc1", nil)
```

构建期注入：

```bash
go build -ldflags "-X guis/internal/version.Number=1.3.0 \
  -X guis/internal/version.Channel=release \
  -X guis/internal/version.Commit=$(git rev-parse --short HEAD) \
  -X guis/internal/version.BuildTime=$(date -u +%FT%TZ)"
```

## 6. 窗口图标

```go
ui.SetWindowIcon(icoBytes)    // .ico 字节 → 大小图标 + 窗口类图标
ui.DefaultIcon()             // 内置图标（只读，勿修改）
```

窗口 / 任务栏 / 托盘使用的图标是同一个；`Options.Icon` 在创建窗口时设置，
需要「exe 文件图标」时用 `tools/winres` 把同一 .ico 嵌进可执行文件。

## 7. 调试辅助

```go
app.FontCacheInfo()                     // (字体实例数, 图集字节数)
ui.CountCacheRoots(root, w, h)          // 统计可缓存根节点数与面积
ui.DropCachedLayers()                   // 清空所有窗口的栅格缓存（改主题后想彻底重绘时）
ui.DebugDump(root)                      // 打印布局树
app.HWND()                              // 原生句柄（对接自有 win32 调用）
```

## 8. 配套的构建脚手架

| 脚本 | 用途 |
| --- | --- |
| `build.ps1` | 单目标构建（含版本注入） |
| `build-multi.ps1` | 多平台（windows/linux/darwin + wasm） |
| `verify-platforms.ps1` | 校验脚本：三端编译（含 darwin-arm64）+ wasm + vet + 单测 + 调试代码扫描 |
| `build-android.ps1` / `build-android-apk.ps1` | 安卓 so + APK |
| `installer/guis.nsi` | NSIS 安装包（支持 `!ifdef ASSETS_DIR` 可选资源块） |
| `tools/winres` | 把 .ico / 版本资源写进 exe（生成 .syso） |

---

## 9. 窗口特效（透明度 / 穿透 / 置底 / 材质）

```go
app := ui.Current()
app.SetOpacity(0.92)              // 整窗半透明（WS_EX_LAYERED）
app.SetClickThrough(true)         // 鼠标穿透：点击落到下面的窗口（桌面宠物 / 悬浮挂件）
app.SetSkipTaskbar(true)          // 不在任务栏显示（WS_EX_TOOLWINDOW）
app.SetKeepOnBottom(true)         // 压在所有普通窗口下面（壁纸式）
app.SetCaptureExcluded(true)      // 防截屏 / 录屏（WDA_EXCLUDEFROMCAPTURE）
app.SetDarkTitleBar(true)         // 系统标题栏跟随深色
app.SetBackdrop(ui.BackdropMica)  // Win11 材质：Mica / Acrylic / Tabbed
app.SetMinSize(720, 480)          // 最小客户区尺寸（WM_GETMINMAXINFO）
app.SetMaxSize(0, 0)              // 0 = 不限制
app.Activate()                    // 拉到前台并抢焦点
```

* 全部是**安全空操作**：非 Windows 平台不实现这些后端方法，`App` 侧类型断言失败即跳过；
* 能力位：`ui.HasCap(ui.CapWindowFX)`；
* `Options` 里也支持 `MinWidth / MinHeight`（`display.Config` 透传，创建时即生效）；
* **材质可见的前提**：窗口必须留出透明区域 —— 自绘窗口默认整窗铺底色，
  要让 Mica 透出来，把 `Options.Background` 设成半透明色（如 `rgba(0,0,0,0.35)`）;
* 鼠标穿透开启后本窗口收不到任何鼠标事件，请务必留一个全局热键或托盘入口恢复。

## 10. 任务栏集成（进度 / 角标 / 闪烁）

```go
app.SetTaskbarProgress(3, 10)    // 任务栏图标上的进度条（total<=0 = 不确定进度）
app.SetTaskbarProgress(0, 0)     // 不确定进度（跑马灯）
app.ClearTaskbarProgress()
app.SetBadge(5)                  // 未读数角标（红底白字，>99 显示 99+）
app.SetBadge(0)                  // 清除
app.Flash(3)                     // 闪烁任务栏图标 3 次
app.Flash(0)                     // 一直闪到窗口获得焦点
```

底层是 `ITaskbarList3`（`HrInit` 懒初始化，失败后不再重试）。角标是现场绘制的
16×16 `HICON`，`SetOverlayIcon` 后由系统复制，图标随即释放。
能力位：`ui.HasCap(ui.CapTaskbar)`。

## 11. 文件图标与缩略图

```go
icon, err := ui.FileIcon(path, 32)          // 16 / 24 / 32 / 48 / 256
thumb, err := ui.FileThumbnail(path, 128, 128)
name := ui.FileTypeName(path)               // "文本文档" / "文件夹"
```

* Windows 走 `SHGetFileInfoW` + `SHGetImageList`（大尺寸从系统图像列表取）；
  文件不存在时按扩展名推断（`SHGFI_USEFILEATTRIBUTES`），适合列表里画「虚拟文件」；
* 返回 `*image.NRGBA`，直接给 `ui.Image()` 需要的是路径或 `data:` URL：
  先用 `png.Encode` 写到临时文件/缓存目录再用 `ui.Image(path)`，或者编码成 `data:image/png;base64,...` 传进去；
* 能力位：`ui.HasCap(ui.CapFileIcon)`（目前仅 Windows）。

## 12. 把文件拖出去（Drag Out）

```go
// 在处理鼠标按下的回调里调用（DoDragDrop 会进入自己的消息循环）
err := ui.DragFiles([]string{"D:/a.png", "D:/b.png"}, x, y)
```

* x/y 是窗口内逻辑像素；传 `-1, -1` 表示用当前鼠标位置；
* 内部实现 `IDataObject`（只提供 `CF_HDROP`）+ `IDropSource`，可直接拖进资源管理器、
  微信、Slack 等任何接受文件拖放的目标；
* 能力位：`ui.HasCap(ui.CapDragOut)`（目前仅 Windows）。

## 13. 富文本剪贴板（HTML / RTF）

```go
ui.ClipboardSetHTML(`<b>GUIS</b> 富文本`)   // 同时写纯文本回退
html, err := ui.ClipboardHTML()
ui.HasClipboardHTML()
ui.ClipboardSetRTF(rtf)                     // RTF 文档
plain := ui.HTMLToPlainText(html)            // 「粘成纯文本」
```

* Windows 写的是标准 `CF_HTML`（含 `Version / StartHTML / EndHTML / StartFragment /
  EndFragment` 定宽头，UTF-8 字节偏移）；
* Linux 需要 `xclip`（按 `text/html` target 读写）；
* 能力位：`ui.HasCap(ui.CapClipboardRich)`。

## 14. 键鼠模拟输入

```go
ui.SendText("张三")                  // 逐字符 Unicode 输入（中文不经过输入法）
ui.SendKey(ui.VK_RETURN)             // 单键
ui.SendKeyCombo(ui.HotCtrl, 'S')     // Ctrl+S
ui.MouseMoveTo(800, 600)             // 屏幕绝对坐标（物理像素）
ui.MouseClickAt(800, 600, ui.MouseLeft)
ui.MouseWheel(120)
```

⚠️ 模拟输入等于替用户操作，安全软件会标记；只在自有应用里、由用户明确触发的场景使用。
能力位：`ui.HasCap(ui.CapInputSim)`（目前仅 Windows，`SendInput`）。

## 15. 快捷方式 / 文件关联 / 前台窗口 / 终端

```go
ui.CreateShortcut(`C:\Users\me\Desktop\MyApp.lnk`, ui.ShellLink{
    Target: `D:\App\app.exe`, Args: []string{"--tray"},
    WorkDir: `D:\App`, Description: "MyApp", Hotkey: "Ctrl+Alt+K",
})
// Linux: 传以 .desktop 结尾的路径会写 freedesktop 桌面入口
// macOS: .webloc 写目标 URL

ui.SetFileAssociation(".myapp", "MyApp.Document", `"D:\App\app.exe" "%1"`)  // HKCU，无需管理员

w := ui.ForegroundWindowInfo()   // {Handle, Title, Process, PID}
ui.OpenTerminal(dir)             // Windows Terminal / cmd / gnome-terminal …
```

对应能力位：`CapShellLink`（快捷方式与关联）、`CapSystemInfo`（前台窗口）。
`SetFileAssociation` 会顺手 `SHChangeNotify(SHCNE_ASSOCCHANGED)` 刷新资源管理器。
