<!-- group: 系统能力 -->
# 系统能力 · 文件对话框 / 剪贴板 / 热键 / 托盘 / 拖入 / 自启

所有系统能力都应先判断 `ui.HasCap(...)`（见《附录 E》），不支持的平台调用是安全空操作。

## 1. 文件与文件夹对话框

```go
path, ok := ui.OpenFileDialog("选择图片", "图片|*.png;*.jpg|所有文件|*.*")
path, ok = ui.SaveFileDialog("另存为", "文本|*.txt", "txt")
dir, ok := ui.PickFolderDialog("选择输出目录")
paths, ok := ui.PickFiles("选择多个文件", "图片|*.png")   // 多选（桌面）

// 回调式（浏览器 / 移动端必须用这个）
ui.PickFile("选择图片", "图片|*.png", func(path string) { open(path) })
ui.PickFilesAsync("选择文件", "", func(paths []string) { openAll(paths) })

ui.ShellOpen("https://go.dev")     // 用系统默认程序打开路径 / 网址
ui.RevealInExplorer("D:/a/b.txt")  // 资源管理器里定位并选中
ui.MoveToRecycleBin("D:/a/b.txt")  // 移入回收站（可还原）
```

过滤器格式：`"显示名|*.ext;*.ext|显示名2|*.*"`（竖线分段）。

## 2. 剪贴板

```go
ui.ClipboardSetText("内容")
txt := ui.ClipboardText()

ui.ClipboardSetFiles([]string{"D:/a.png", "D:/b.png"})   // 可直接粘贴到资源管理器
ui.ClipboardSetFiles(nil)                                 // 清空

// 摘要 + 图片 + 监听（Windows 完整支持）
info := ui.Clipboard()          // {Seq, Text, HasText, HasImage, HasFiles}
img, err := ui.ClipboardImage() // 剪贴板位图 → *image.NRGBA
stop := ui.WatchClipboard(func(i ui.ClipboardInfo) { syncClip(i) })  // 500ms 轮询序号
defer stop()
```

## 3. 全局热键（窗口失焦也触发）

```go
id := ui.RegisterHotkey(ui.HotCtrl|ui.HotAlt, 'K', func() { ui.OpenCommandPalette() })
if id == 0 { ui.ToastShow("热键被其它程序占用") }
defer ui.UnregisterHotkey(id)

// 窗口方法（等价，作用于该窗口）
id = ui.Current().RegisterHotkey(ui.HotCtrl|ui.HotShift, 0x51, showWindow)
```

修饰键：`HotAlt / HotCtrl / HotShift / HotWin / HotNoRepeat`（`MOD_NOREPEAT` 默认已带）。
回调在 UI 线程执行，可直接改状态 + `RequestRedraw()`。

## 4. 托盘

```go
tray, err := ui.NewTray("GUIS",
    func() { ui.Current().Show() },         // 左键点击
    []ui.TrayItem{
        {ID: 1, Label: "显示主窗口"},
        {Separator: true},
        {ID: 2, Label: "开机自启", Checked: true},
        {ID: 3, Label: "退出"},
    },
    func(id int) {                          // 右键菜单点击
        switch id {
        case 1: ui.Current().Show()
        case 2: toggleAutoStart()
        case 3: ui.Current().Quit()
        }
    },
)
tray.SetTooltip("GUIS 运行中")
tray.SetItems(newItems)
tray.Notify("提示", "已最小化到托盘")     // 气泡通知
tray.Remove()
```

常驻托盘模式：`Options.Hidden = true` + `app.SetOnClose(func() bool { app.ShowWindow(false); return false })`。

## 5. 文件拖入

```go
ui.DropZone("dz", "把文件拖到这里", 160, func(paths []string) { importFiles(paths) })
ui.Box(...).DropEvt(func(paths []string) { ... })          // 任意节点
ui.SetOnDropFiles(func(paths []string, x, y float32) { })   // 应用级兜底
ui.SetAcceptDrops(false)                                    // 关闭接收
```

## 6. 开机自启

```go
spec := ui.AutoStartSpec{
    Name:    "GUIS",       // 空 = 可执行文件名
    Args:    []string{"--tray"},
    Comment: "GUIS 桌面助手",
}
err := ui.SetAutoStart(true, spec)
on  := ui.AutoStartEnabled(spec)
if !ui.AutoStartSupported() { /* 浏览器 / 移动端不支持 */ }
```

* Windows：写 `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
* Linux：`~/.config/autostart/<name>.desktop`
* macOS：`~/Library/LaunchAgents/<name>.plist`
* Web / 移动端：不支持（返回错误）
* `GUIS_AUTOSTART_CMD` 可覆盖注册的命令行

## 7. 通知 / 阻止休眠 / 系统深浅色

```go
tray.Notify("导出完成", "report.xlsx")     // 系统级气泡
ui.Notify(ui.Notice{...})                  // 应用内通知中心（跨平台，见 52 章）

ui.SetKeepAwake(true)                      // 阻止休眠 / 关屏（演示、下载、长任务）
ui.SetKeepAwake(false)
ui.SystemDarkMode()                        // 系统是否深色（读注册表 / 环境 / matchMedia）
```

## 8. 单实例（Windows 完整）

```go
if !ui.SingleInstance("com.example.guis") {
    ui.NotifyFirstInstance("com.example.guis", os.Args[1:])  // 把参数交给已运行实例
    return                                                     // 本进程退出
}
go ui.WatchSecondInstance("com.example.guis", func(args []string) {
    ui.Current().Show()      // 收到二次启动：把窗口拉到前台
})
```

## 9. 能力对照速查

| 能力 | Cap | Windows | Linux | macOS | Web | Android |
| --- | --- | --- | --- | --- | --- | --- |
| 文件对话框 | `CapDialogs` | ✅ | ✅ | ✅ | 部分（`PickFile`） | ✗ |
| 剪贴板 | `CapClipboard` | ✅ | ✅ | ✅ | 写 ✅ / 读 ✗ | ✗ |
| 剪贴板监听 | `CapClipboardWatch` | ✅ | ✅ | ✅ | ✗ | ✗ |
| 全局热键 | `CapHotkey` | ✅ | ✅ XGrabKey | ✅ CGEventTap | ✗ | ✗ |
| 托盘 | `CapTray` | ✅ | ✅ XEmbed | ✅ NSStatusItem | ✗ | ✗ |
| 文件拖入 | `CapDragDrop` | ✅ | ✅ XDND | ✅ NSDragging | ✗ | ✗ |
| 回收站 | `CapRecycleBin` | ✅ | ✅ | ✅ | ✗ | ✗ |
| 阻止休眠 | `CapKeepAwake` | ✅ | ✅ | ✅ | ✅（WakeLock） | ✗ |
| 开机自启 | `CapAutoStart` | ✅ | ✅ | ✅ | ✗ | ✗ |

> 表里 ✗ 的项调用是**安全空操作**（返回零值 / 错误），不会 panic，但界面上应当隐藏或置灰。

---

## 10. Linux / macOS 三个能力的实现与限制

`CapHotkey` / `CapTray` / `CapDragDrop` 在 Linux(X11) 与 macOS 上都已真正实现，能力位如实上报
（`caps_integration_x11.go` / `caps_integration_darwin.go`）。

| 能力 | Linux(X11) | macOS |
| --- | --- | --- |
| 全局热键 | `XGrabKey`（含 CapsLock/NumLock 四种锁组合），根窗口 `KeyPress` 匹配 | `CGEventTap`（ListenOnly，不吞按键） |
| 托盘 | XEmbed：`_NET_SYSTEM_TRAY_Sn` 选择 + `SYSTEM_TRAY_REQUEST_DOCK` | `NSStatusItem` + `NSMenu`（支持二级菜单） |
| 文件拖入 | XDND：`XdndEnter/Position/Drop` → `ConvertSelection(text/uri-list)` → `XdndFinished` | `NSDraggingDestination`（`registerForDraggedTypes:`） |

使用上的差异（业务侧需要知道的三件事）：

1. **热键授权**：macOS 10.15+ 需要「输入监控」授权，未授权时 `RegisterHotKey` 返回 0；
   X11 侧热键被其它程序占用时 `XGrabKey` 会异步报 `BadAccess`，当前按「返回 id 视为成功」处理。
2. **托盘宿主**：X11 托盘是 XEmbed 协议，Sway 等纯 wlroots 桌面只有 StatusNotifierItem，
   此时 `NewTray` 返回错误（`HasCap(CapTray)` 仍为真，因为能力是编译期确定的）；
   托盘图标**没有悬停提示通道**，`SetTooltip` 只保存文本；右键回调 `onMenu(-1)`，
   由业务自己弹 `ui.MenuPanel`。
3. **拖入落点**：X11 给的是根窗口坐标，框架已用 `XTranslateCoordinates` 换算成窗口内物理像素
   再按 DPI 折成逻辑像素；macOS 的 `NSDraggingInfo` 只有屏幕坐标，这里用当前指针位置近似。

实现位置：`ui/hotkey_x11.go`、`ui/tray_x11.go`、`ui/drop_x11.go`、
`ui/hotkey_darwin.go`、`ui/tray_darwin.go`、`internal/cocoa/drop.go`；
共用解析逻辑在 `ui/dropfiles_shared.go`（`ParseURIList`，跨平台单测覆盖）。

---

## 11. 系统标准目录

```go
ui.KnownFolder(ui.DirDownloads)      // 下载（Windows 走 SHGetKnownFolderPath，支持被重定向到 D 盘）
ui.KnownFolder(ui.DirConfig)         // %APPDATA% / ~/.config / ~/Library/Application Support
ui.SubFolder(ui.DirData, "MyApp")    // 拼子目录（不创建）
ui.EnsureFolder(ui.DirCache)         // 不存在就创建，返回 (路径, error)
ui.KnownFolderExists(ui.DirPictures) // 目录是否存在
```

目录类别：`DirHome / DirDesktop / DirDocuments / DirDownloads / DirPictures / DirMusic /
DirVideos / DirConfig / DirCache / DirData / DirTemp / DirPrograms / DirExecutable`。

* Windows 用已知文件夹 API（用户把「下载」改到 D 盘也能拿到真实路径）；
* Linux 读 `~/.config/user-dirs.dirs`（中文系统是 `~/下载`，不能写死 `~/Downloads`）；
* 拿不到时返回空串，`KnownDirName(d)` 给出中文名用于界面展示。

## 12. 文件 / 目录变更监听

```go
stop := ui.WatchDir(dir, ui.WatchOptions{Recursive: true}, func(evs []ui.FsEvent) {
    for _, e := range evs {
        log.Println(e.String())   // "新建 D:\a\b.txt"
    }
})
defer stop()                       // 可重复调用（幂等）

stop = ui.WatchFile(cfgPath, func(e ui.FsEvent) { applyConfig(e.Path) })
```

`FsEvent{Path, OldPath, Change, IsDir}`，`Change` 为 `FsCreated / FsModified /
FsRemoved / FsRenamed`。

* Windows：`ReadDirectoryChangesW`，事件驱动、零轮询；
* 其它平台：定时扫描 + 差异比对（`PollInterval` 默认 400ms，只比较大小/修改时间/类型）；
* 回调整体切到 UI 线程，可以直接改状态并 `RequestRedraw()`；
* `WatchOptions.Filter` 可按路径过滤（`WatchFile` 内部就是这么实现的）。

## 13. 原生消息框（崩溃兜底）

```go
ui.MessageBox("无法启动", "初始化 OpenGL 失败，请更新显卡驱动。")
switch ui.MessageBoxEx("删除文件", "该操作不可撤销，继续？", ui.MsgWarn, ui.MsgBoxYesNo) {
case ui.MsgResYes: del()
case ui.MsgResNo, ui.MsgResCancel: return
}
```

* Windows `MessageBoxW`，Linux `zenity` / `kdialog`，macOS `osascript`；
* 都不支持时写 stderr 并返回 `MsgResOK`（不阻塞、不 panic）；
* **框架自身已接入**：`Run` 捕获到的 panic 会先写崩溃日志再弹一次原生错误框
  （GUI 子系统下 stderr 不可见），自动化可用 `GUIS_NO_MSGBOX=1` 关闭；
* `ui.MsgButtonTexts(kind)` 可复用同一套按钮文案做自绘对话框。

## 14. 系统信息与电源

```go
info := ui.SystemInfo()
// {Locale "zh-CN", LocaleName "中文（简体）", UserName, HostName,
//  OS "Windows 11 (build 22631)", OSVersion "10.0.22631", Arch, Elevated}

ui.SystemLocale()          // 系统 UI 语言（"zh-CN"）
ui.SystemLanguages()       // 按优先级排列的系统语言（Win32 UI 语言 / LANG / AppleLocale / navigator.languages）
ui.SetSystemLanguages("zh-TW")   // 移动端原生壳注入（无 JNI / ObjC 语言桥时）
ui.RefreshSystemLanguages()      // 用户改了系统语言后刷新缓存
ui.IsElevated()            // 是否管理员 / root

pw := ui.PowerState()   // {OnBattery, Percent (-1=未知), Charging, ACOnline, SaverOn}
ui.HasBattery()
```

用途：关于页、崩溃报告、按系统语言初始化界面（`ui.AutoLocale()` 或
`ui.SetLocale(ui.SystemLocale())`，详见 `73` 章）、笔记本省电降帧。

## 15. 会话控制

```go
ui.LockScreen()                      // 锁屏（Win+L 等价）
ui.Suspend(false)                    // 睡眠；true = 休眠
ui.Shutdown(30, false)               // 30 秒后关机（可用 CancelShutdown 取消）
ui.Reboot(0, false)                  // 立即重启
ui.SignOut(false)                    // 注销当前用户
ui.CancelShutdown()
if ui.IsUnsupported(err) { /* 隐藏入口 */ }
```

Windows 走 `LockWorkStation` / `InitiateShutdownW` / `ExitWindowsEx`（自动尝试
提升 `SeShutdownPrivilege`），Linux 走 `loginctl` / `systemctl`，macOS 走
`osascript` / `pmset`。**关机 / 重启不可逆，框架不替你弹确认框**，请业务自己确认。

## 16. 系统音量与联网状态

```go
ui.SystemVolume()          // 0~1，读不到返回 -1
ui.SetSystemVolume(0.4)
muted, err := ui.ToggleSystemMute()

if !ui.Online() { /* 离线：隐藏下载按钮 */ }
ui.OnlineAsync(func(ok bool) { /* 后台检测，回 UI 线程 */ })
stop := ui.SetOnNetworkChange(func(ok bool) { /* 5 秒轮询，状态变化才回调 */ })
```

Windows 走 Core Audio（`IAudioEndpointVolume`），Linux 走 `wpctl` / `pactl` /
`amixer`，macOS 走 `osascript`。`Online()` 是粗粒度判断（ICMP + DNS 回退），
要精确到「某个服务是否可达」请自己发 HEAD 请求。
