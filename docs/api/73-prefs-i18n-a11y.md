<!-- group: 系统能力 -->
# 偏好 / 国际化 / 无障碍 / 崩溃

## 1. 偏好持久化

```go
ui.SetPref("theme", "dark")
v, ok := ui.GetPref("theme")
ui.DelPref("theme")

ui.SetPrefBool("tray", true); ui.GetPrefBool("tray", false)
ui.SetPrefInt("zoom", 120);   ui.GetPrefInt("zoom", 100)
ui.SetPrefFloat("pos", 0.28); ui.GetPrefFloat("pos", 0.28)

ui.SavePrefs()     // 立即原子写盘
ui.PrefsPath()     // 桌面：JSON 文件路径；Web：空串（用 localStorage）
ui.ResetPrefs()    // 清空内存中的偏好（恢复默认）
```

内置：`Split`（`PrefsKey`）、`TableV` 列宽、`Tour` 的 `Seen`、部分组件的展开态都会写偏好。
偏好按窗口共享（进程级一张表），适合放主题、布局、最近打开的文件、窗口位置等。

## 2. 国际化 i18n

一套 UI 在 Windows / Linux / macOS / Web / Android / iOS 上共用同一份语言包、同一套格式化
规则与同一套布局镜像逻辑。内置 **zh-CN / en-US** 两份语言包（`go:embed` 打进二进制，
浏览器与移动端不需要外部文件），另有 15 种语言的元数据（日期 / 月份 / 数字 / 方向）。

### 2.1 启动与切换

```go
ui.Run(ui.Options{Title: "我的应用"}, view)   // 自动：偏好 > 系统语言 > en-US
ui.Run(ui.Options{Locale: "en-US"}, view)     // 固定语言（忽略偏好）

ui.AutoLocale()          // 手动执行「偏好 > 系统语言 > 兜底」并返回结果
ui.UseLocale("en-US")    // 切换 + 记住（写偏好 ui.locale，重启后自动恢复）
ui.SetLocale("ja-JP")    // 只切换，不写偏好
ui.Locale()              // "ja-JP"
ui.LocaleGeneration()    // 语言代数：每次切换 +1（业务缓存失效用）

ui.SystemLanguages()     // ["zh-CN"] 系统语言（按优先级）
ui.SystemLocaleTag()     // 首选系统语言
ui.SetSystemLanguages("zh-TW")   // 移动端原生壳注入（无 JNI/ObjC 桥时）
ui.RefreshSystemLanguages()      // 系统语言已被用户改动时刷新缓存
```

系统语言怎么拿：

| 平台 | 来源 |
| --- | --- |
| Windows | `GetUserDefaultUILanguage` → `LCIDToLocaleName`（zh-CN） |
| Linux (X11) | `LC_ALL` / `LC_MESSAGES` / `LANGUAGE` / `LANG`（`zh_CN.UTF-8` → `zh-CN`） |
| macOS | `defaults read -g AppleLocale`，再退回环境变量 |
| Web | `navigator.languages` / `navigator.language` |
| Android / iOS | 环境变量；原生壳可调 `SetSystemLanguages` 注入 |

### 2.2 翻译

```go
ui.T("ui.dialog.ok")                                  // 确定 / OK
ui.Tf("greet", map[string]string{"name": "舒"})        // 占位 {name}
ui.Ts("ui.pager.pageNoPerm", "7")                     // 位置占位 {0} {1}
ui.TOr("src.文件", "文件")                                 // 缺失时显示原文（渐进接入）
ui.Tn("ui.time.minutesAgo", 5)                        // 5 分钟前 / 5 minutes ago
ui.Tnf("items", 3, map[string]string{"who": "你"})     // 复数 + 命名参数
```

查找顺序：**当前语言 → 语言基础链（zh-Hant-TW → zh-Hant → zh → zh-CN）→ 兜底语言
（默认 en-US）→ 可读 key**（`open_file` → `open file`，`ui.dialog.ok` → `ok`）。
不会出现空白或原始键名。

字符串表是扁平 JSON，可重复载入（后载入覆盖同 key）：

```go
ui.LoadCatalog("zh-CN", `{"save":"保存","cancel":"取消"}`)
ui.LoadCatalogFile("fr-FR", "locale/fr-FR.json")   // 桌面端；Web/移动端请用 go:embed
```

内置 key 命名空间 `ui.`：`ui.dialog.*`、`ui.common.*`、`ui.file.*`、`ui.video.*`、
`ui.music.*`、`ui.pager.*`、`ui.state.*`、`ui.form.*`、`ui.editor.*`、`ui.time.*`、
`ui.cap.*`。业务 key 建议用自己的前缀（`app.`）避免冲突。

### 2.3 复数（CLDR）

```json
{ "items.one": "{n} item", "items.other": "{n} items" }
{ "items.one": "{n} элемент", "items.few": "{n} элемента", "items.many": "{n} элементов" }
{ "items": "{n} 项" }
```

后缀 `.zero / .one / .two / .few / .many / .other`，按语言规则挑选（英 one/other、
俄 one/few/many、阿 zero/one/two/few/many/other、中/日/韩只有一条）。缺 `.other`
时回退到不带后缀的 key；`{n}` 自动替换成**本地化数字**。

### 2.4 数字 / 货币 / 日期 / 时长

```go
ui.LocaleNumber(1234.5, 1)         // 1,234.5 / 1.234,5 / ١٬٢٣٤٫٥
ui.LocaleInt(1234567)              // 1,234,567
ui.LocalePercent(12.5, 1)          // 12.5% / 12,5 %
ui.LocaleCurrency(9.9, "")         // ¥9.90 / $9.90 / 9,90 €（空 = 语言默认货币）
ui.LocaleCurrency(9.9, "EUR")      // 指定货币，符号位置仍按语言习惯

ui.LocaleDate(t, "short")          // 2026/11/6 · 11/6/2026 · 06.11.2026
ui.LocaleDate(t, "medium")         // 2026年11月6日 · Nov 6, 2026
ui.LocaleDate(t, "long")           // 2026年11月6日 星期五 · Friday, November 6, 2026
ui.LocaleTime(t, "short")          // 20:05 · 8:05 PM
ui.LocaleDateTime(t, "short")
ui.LocaleDatePattern(t, "yyyy-MM-dd EEE")   // 自定义图案
ui.LocaleWeekday(time.Friday, true)         // 星期五 / Friday
ui.LocaleMonth(time.November, false)        // 11月 / Nov
ui.LocaleWeekStart()                        // 0 = 周日 / 1 = 周一

ui.LocaleRelative(t, now)          // 3 分钟前 / in 2 hours / 2 天前
ui.LocaleDuration(90 * time.Second) // 1 分钟 和 30 秒 / 1 minute and 30 seconds
ui.LocaleList([]string{"a","b","c"}) // a、b 和 c / a, b and c
ui.LocaleBytes(1536)               // 1.5 KB
```

图案占位符：`yyyy yy MMMM MMM MM M dd d EEEE EEE e HH H hh h mm m ss s a`，
单引号内按字面输出（`d 'de' MMMM`）。

### 2.5 RTL（从右到左）

```go
ui.IsRTL()               // 当前是否 RTL
ui.SetRTLAuto(true)      // 跟随语言自动切换（默认开启）
ui.SetRTL(true)          // 手动强制（会关闭自动切换）
ui.DirAlign()            // RTL 时 AlignEnd
ui.MirrorRow(a, b)       // 手工节点序列镜像
ui.MirrorIcon("→")       // 方向图标镜像
ui.MirrorX(originX, boxW, x, w)  // 绘制期坐标镜像（自绘控件用）
```

切到 `ar` / `he` / `fa` / `ur` 等语言时，布局引擎在 LTR 摆放完成后**整树水平镜像**
（行 / 换行 / 网格 / 瀑布流 / 叠层 / 绝对定位一次到位），文本默认对齐翻到右侧。
方向本身有语义的子树（时间轴、图表坐标系、代码编辑器内部）设 `Node.NoMirror = true`。

### 2.6 语言切换组件

```go
ui.LocaleSwitcher("set-lang")        // 胶囊切换器（Wrap，2 种以上语言才显示）
ui.LocaleSegmented("set-lang2")      // 分段控件版本
ui.LocaleSettingsRow("set-lang3")    // 「语言 + 切换器」整行
codes, labels := ui.LocaleOptions()  // 自己排版的选项
ui.LocaleLabel("ar-SA")              // "العربية"
```

#### 顶部菜单 / 标题栏：手工拼一个语言菜单

菜单项自带 `Checked`，直接按可用语言生成即可（gallery 的顶部菜单就是这样做的，
标题栏再放一个点击轮换的胶囊，窄屏隐藏菜单栏时也能切）：

```go
// ① 语言文案：键有译文用译文，没有就显示中文原文（渐进接入不出现空白 / key）
ui.LoadCatalog("zh-CN", `{"nav.title":"界面案例"}`)
ui.LoadCatalog("en-US", `{"nav.title":"UI examples"}`)
label := ui.TOr("nav.title", "界面案例")

// ② 顶部菜单「语言」：每种语言一项 + 当前打勾 + 跟随系统
func languageMenu() ui.MenuItem {
    items := make([]ui.MenuItem, 0, 4)
    for _, in := range ui.AvailableLocales() {
        code := in.Code
        items = append(items, ui.MenuItem{
            Label:   in.NativeName,          // "中文（简体）" / "English (US)"
            Hint:    code,
            Checked: code == ui.Locale(),
            OnPick:  func() { ui.UseLocale(code) },
        })
    }
    items = append(items, ui.MenuItem{Separator: true})
    items = append(items, ui.MenuItem{
        Label: ui.TOr("lang.auto", "跟随系统"),
        OnPick: func() {
            ui.DelPref("ui.locale")   // 清掉选择后重新协商
            ui.AutoLocale()
        },
    })
    return ui.MenuItem{Label: ui.TOr("menu.language", "语言"), Items: items}
}

// ③ 标题栏胶囊：显示当前语言短名，点一下轮换到下一种
next := ui.AvailableLocales()[1].Code   // 实际按当前语言取「下一个」
chip := ui.Row(ui.IconColored("globe", 13, ui.ColTextMuted),
    ui.Text("中文")).St(ui.St().PadXY(9, 4).R(8)).
    Tap(func() { ui.UseLocale(next) })
```

`ui.UseLocale` 会写偏好 `ui.locale`，下次启动 `AutoLocale` 自动恢复；
界面每帧重建，切完立即生效，不需要额外刷新（缓存了文案的菜单 / 托盘 / 窗口标题
用 `ui.OnLocaleChange` 刷一次）。

### 2.7 自定义语言

```go
ui.LoadCatalog("fr-FR", frJSON)      // 只要译文：日期/数字按语言族自动派生
ui.RegisterLocale(ui.LocaleInfo{     // 只覆盖差异字段，其余自动补齐
    Code: "fr-CA", NativeName: "Français (Canada)",
    Currency: "CAD", CurrencySymbol: "C$",
})
ui.Locales()            // 全部已注册语言（含只有元数据的）
ui.AvailableLocales()   // 有译文的语言（切换器只列这些）
ui.LocaleInfoOf("de-DE")// 元数据（月份名 / 方向 / 货币 / 日期图案）
ui.MatchLocale("en-GB") // BCP-47 协商 → "en-US"
ui.OnLocaleChange(func(code string) { /* 刷新菜单 / 托盘 / 窗口标题 */ })
```

> 注：完整 bidi 排版（阿拉伯语整形、混排重排）未做；框架提供的是**布局镜像 + 对齐 +
> 字形后备**，足以支撑 RTL 界面。汉字 / 阿拉伯 / 西里尔 / 泰文等缺字形会自动落到系统
> 默认字体。

## 3. 无障碍 a11y

```go
ui.Btn("保存", ui.BtnPrimary, save).A11yOn(ui.A11yButton, "保存", "保存当前文件")
ui.Input("u", user, "用户名", setUser).A11yOn(ui.A11yTextField, "用户名", "必填")

ui.A11ySnapshot()          // []A11yEntry{Key, Role, Name, Desc, Focus}
ui.A11yRoleName(r)         // "button" / "textfield" / …
```

角色枚举：`A11yNone / Button / Text / Link / CheckBox / Radio / Switch / Slider / TextField /
List / ListItem / Tab / Dialog / Menu / MenuItem / Progress / Image / Table / Tree`。

配合焦点系统的两点：

* 键盘可达：`FocusOn()` + `ActivateEvt(fn)`（Enter / Space 激活）；
* 焦点可见：框架统一画焦点环，`FocusVisible()` 为真时绘制；自绘控件可用 `NoFocusRing()` 关掉。

## 4. 崩溃报告

```go
ui.SetCrashDir(`D:\logs\crash`)                      // 默认 <用户配置目录>/guis/crashes
ui.SetCrashHandler(func(r ui.CrashReport) {          // panic 恢复 + 写盘后回调
    log.Printf("crash: %s\n%s", r.Message, r.Stack)
})

// 结构体
type CrashReport struct {
    Time    time.Time
    Message string
    Stack   string
    File    string      // 落盘路径
}

ui.ReportCrash(v, debug.Stack())   // 手动记录
ui.CrashLast()                     // 最近一次（无则 ok=false）
ui.CrashPanel()                    // 现成面板：摘要 + 日志路径
defer ui.RecoverToReport("worker") // 业务 goroutine 兜底
```

## 5. 日志与诊断约定

| 手段 | 用法 |
| --- | --- |
| `GUIS_FPS=1` | 每帧构建 / 布局 / 绘制耗时 + 动画节点清单 |
| `app.FontCacheInfo()` | 字体实例与图集占用 |
| `ui.CountCacheRoots(root, w, h)` | 缓存根数量与覆盖面积 |
| `ui.DebugDump(root)` | 布局树全量输出 |
| `Options.DumpLayout` | 首帧布局树写文件 |
| `ui.CrashLast()` / `CrashPanel()` | 崩溃摘要与日志路径 |

## 6. 一次完整的「设置页」写法

```go
func settingsPage() *ui.Node {
    return ui.Column(
        ui.Title("设置", "偏好会立即保存"),
        ui.Card(
            ui.Row(ui.Text("深色模式"),
                ui.Spacer(),
                ui.Switch("dark", ui.IsDark(), func(v bool) {
                    if v { ui.UseTheme(ui.ThemeDark) } else { ui.UseTheme(ui.ThemeLight) }
                    ui.SetPrefBool("dark", v)
                }),
            ).St(ui.St().PadXY(16, 12)),
            ui.Divider(),
            ui.Row(ui.Text("开机自启"),
                ui.Spacer(),
                ui.Switch("autostart", ui.AutoStartEnabled(spec), func(v bool) {
                    if err := ui.SetAutoStart(v, spec); err != nil { ui.ToastShow(err.Error()) }
                }),
            ).St(ui.St().PadXY(16, 12)),
            ui.Divider(),
            ui.Row(ui.Text("界面缩放"),
                ui.Spacer(),
                ui.Slider("scale", (ui.UIScale()-0.6)/1.9, func(v float32) {
                    ui.SetUIScale(0.6 + v*1.9)
                }).St(ui.St().W(200)),
            ).St(ui.St().PadXY(16, 12)),
        ).St(ui.St().R(14).BgC(ui.ColBgSoft).Stroke(1, ui.ColStroke)),
    ).St(ui.St().GapOf(16).Padding(24))
}
```

`gallery` 的 `case_settings.go` 是这份代码的完整可运行版本。
