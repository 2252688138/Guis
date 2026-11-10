<!-- group: 布局与样式 -->
# 主题 / 缩放 / 字体

## 1. 主题模式

```go
ui.UseTheme(ui.ThemeDark)     // "dark"
ui.UseTheme(ui.ThemeLight)    // "light"
ui.UseTheme(ui.ThemeSystem)   // "system"，跟随系统深浅色

ui.ThemeMode()                // 当前模式
ui.IsDark()                   // 当前是否深色
ui.CurrentTheme()             // 当前生效的 Theme 色板
ui.SystemPrefersDark()        // 系统偏好（读注册表 / matchMedia）
```

非 Windows 端 `SystemDarkMode()` 也会尽力探测（Linux 桌面环境 / macOS 外观 / 浏览器 `prefers-color-scheme`）。

## 2. Theme 色板字段

```go
type Theme struct {
    Name, Text, TextDim, TextMuted          string
    Bg, BgSoft, BgDeep                      string
    Stroke, Line                            string
    Primary, Primary2                       string
    Success, Warn, Danger, Info             string
    Input, Panel, PanelDeep                 string
    Row, RowAlt, Header, Placeholder        string
    Hover, Scrim, Track, Glass, FocusRing   string
}
```

内置 `ui.Dark`、`ui.Light`。常用颜色别名（随主题切换而变）：

`ColText / ColTextDim / ColTextMuted / ColBg / ColBgSoft / ColBgDeep / ColStroke / ColLine /
ColPrimary / ColPrimary2 / ColSuccess / ColWarn / ColDanger / ColInfo / ColInput / ColPanel /
ColPanelDeep / ColRow / ColRowAlt / ColHeader / ColPlaceholder / ColHover / ColScrim / ColTrack /
ColGlass / ColFocus / ColSurface* / ColOnAccent`

> 注意：`ColXxx` 是**包级变量**，切主题时框架会重写它们；不要在 `init()` 里把它们拷进常量。

## 3. 自定义 / 注册主题

```go
// ① 注册一整套具名主题（同名会覆盖内置 dark / light）
ui.RegisterTheme("ocean", ui.Theme{
    ...ui.Dark,                     // 以深色为底改几项
    Name: "ocean", Bg: "#061626", BgSoft: "#0b2135", Primary: "#22d3ee",
})
ui.UseThemeName("ocean")
ui.ThemeNames()      // 已注册主题名
ui.ThemeTitle("ocean")  // 界面显示名

// ② 只覆盖语义 token（更轻）
t := ui.TokensOf()
t.Accent = "#22d3ee"
ui.SetTokens(t)      // 空串字段保持原值，立即生效并记住覆盖
ui.ClearTokenOverrides()
```

`Tokens` 语义字段：`Surface / SurfaceMuted / SurfaceRaised / SurfaceDeep / OnSurface /
OnSurfaceMuted / OnSurfaceFaint / Border / BorderSubtle / Accent / AccentAlt / OnAccent /
Success / Warning / Danger / Info / Overlay / Track / FocusRing / Glass / Header /
Row / RowAlt / Hover / Placeholder`。

## 4. 界面缩放与超采样

```go
ui.UIScale()                 // 当前缩放（1 = 100%）
ui.SetUIScale(1.15)          // 60%~250% 自动夹取；立即重建渲染尺寸与字形

ui.SetSuperSample(ui.SS200)  // SSAuto / SSOff / SS125 / SS150 / SS200
ui.SuperSample()             // 实际生效倍数（含 DPI 自动判定与像素预算降档）
ui.SuperSampleMode()         // 档位设置值

ui.SetTextSharp(2)           // 0 标准 / 1 清晰 / 2 极致（立即重建字形）
ui.TextSharp()
```

在线性缩放下，`SetUIScale` 会同时缩放布局与字形，等于系统级 DPI 缩放的应用内版本。

## 5. 字体内置规则

* 桌面端从系统字体目录加载：默认微软雅黑（`msyh.ttc`，粗体 `msyhbd.ttc`）。
* `Options.FontFamily` 按**子串匹配**（区分大小写）：`SimSun` / `宋体`、`SimHei` / `黑体`、
  `Consolas` / `Mono`、`Segoe UI` / `Segoe`、`Symbol`、`Emoji`；未命中的任意名称回落雅黑。
* 浏览器端字体必须自举：

```go
ui.RegisterFont("Microsoft YaHei", ttfBytes)
ui.RegisterFontBold("Microsoft YaHei", boldBytes)   // 以 "<name>/Bold" 命名，按字重优先取
```

* 字体缓存诊断：`app.FontCacheInfo()` 返回实例数与图集字节数。
* 文本度量：`ui.TextWidth(s, size, weight)`、`ui.TextWidthOf(s, size, weight, family)`、
  `ui.TextGlyphBounds(s, size, weight)`（视觉包围盒，垂直居中排版用）。

## 6. 颜色工具

```go
ui.C("#6366f1")             // 支持 #rgb / #rrggbb / #rrggbbaa / rgb(1,2,3) / uint32 0xRRGGBB
ui.Blend(ui.C("#ff0000"), ui.C("#0000ff"), 0.5)   // 两色插值
```

`gpu.Paint` 就是渲染层使用的颜色类型；`BgC` / `Stroke` / `Shadow` / `Text` 等接受 `any`，
传字符串或 `ui.C(...)` 都行。

## 7. 数字 / 语言 / 方向

```go
ui.FormatNumber(12345.678, 2)   // "12,345.68"（固定千分位 / 英文习惯）
ui.LocaleNumber(12345.678, 2)   // 按当前语言："12,345.68" / "12.345,68"
ui.AnimatedNumber("kpi", 1280, 0.6, 0)  // 数值缓动到目标（KPI 卡用）
ui.SetRTL(true)                 // 从右到左布局（手动；切语言时会自动开关）
ui.DirAlign()                   // RTL 时返回 AlignEnd
ui.MirrorRow(children...)       // RTL 时反转子节点顺序（手工序列用）
ui.MirrorIcon("→")              // RTL 时镜像方向图标
ui.MirrorX(x0, w, x, w1)        // 绘制期水平镜像（自绘控件用）
```

切到 RTL 语言（ar / he / fa / ur）时，布局在 LTR 摆放完成后**整树水平镜像**，
行 / 换行 / 网格 / 瀑布流 / 叠层 / 绝对定位一次到位，文本默认对齐翻到右侧；
方向本身有语义的子树（时间轴、图表坐标系）设 `Node.NoMirror = true`。

数字 / 日期 / 货币的本地化格式见 `73` 章的「国际化 i18n」。
