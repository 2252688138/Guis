<!-- group: 基础组件 -->
# 图标 · Iconfont 矢量图标库

`ui.Icon` 系列是**矢量图标**：内置 80+ 常见图标（Lucide / Feather 观感），
由 24×24 设计网格上的 SVG path 光栅化成纹理，**不依赖系统字体或 .ttf 文件**——
任意尺寸都清晰、可随时换色，也不会因为缺字形画出「方块」。

## 1. 直接使用

```go
ui.Icon("search")                        // 18×18，次要文字色
ui.IconSized("star", 28)                 // 指定尺寸
ui.IconColored("heart", 22, "#f472b6")   // 指定颜色（任意颜色表达：Col* / "#rrggbb" / "rgba(...)"）
ui.IconStroked("check", 48, 2.5)         // 指定线宽（设计网格单位，0 = 图标自带线宽）
ui.IconLabel("folder", "项目文件", 16, ui.ColTextDim)   // 图标 + 文字一行（自动 8px 间距 + 垂直居中）
ui.IconAction("trash", 18, func() { ... })              // 方形图标按钮（悬停提亮）
```

| 构造函数 | 说明 |
| --- | --- |
| `Icon(name)` | 默认 18×18，颜色 `ColTextDim` |
| `IconSized(name, size)` | 指定边长（正方形） |
| `IconColored(name, size, color)` | 尺寸 + 颜色 |
| `IconStroked(name, size, stroke)` | 尺寸 + 线宽（12px 用细线、48px 以上用粗线更协调） |
| `IconLabel(name, label, size, color)` | `Row(图标, 文字)`，`GapOf(8)` + 垂直居中 |
| `IconAction(name, size, onClick)` | `size+18` 的方形按钮，`ColBgSoft` 底 + 描边，悬停文字色提亮 |

需要自己排布时，直接用节点字段：

```go
ui.Box().St(ui.St().W(24).H(24)).Add(
    &ui.Node{Kind: ui.KIcon, IconName: "settings", IconStroke: 1.6},
)
```

`Kind == KIcon` 的节点由框架自绘（`paintIcon`）：按**短边等比居中**，颜色取 `Style.Color`
（未设置时用次要文字色），所以宽高不等也不会拉伸变形。

## 2. 注册自定义图标

```go
ui.RegisterIcon(ui.IconDef{
    Name:  "logo",            // 唯一名称（Icon 的参数），大小写敏感
    Label: "品牌",            // 展示名（图标库示例页 / 无障碍朗读用）
    Paths: []string{          // 可以多条，同一份 24×24 网格内叠加
        "M4 4h16v16H4z",
        "M8 12h8",
    },
    Fill:   false,            // true = 按奇偶规则填充（实心图标），false = 描边
    Stroke: 2,                // 描边宽度（设计网格单位，0 = 默认 2）
})
```

* 同名注册会**覆盖**旧定义并返回 `true`（业务可以用自有品牌图标替换内置图标）；
* `UnregisterIcon(name)` 移除图标（示例页演示注册后还原）；
* 支持的 path 指令：`M / L / H / V / C / S / Q / T / A / Z`（大小写相对/绝对均可）；
* 描边做解析式抗锯齿，填充做 4×4 超采样，小尺寸也不糊。

查询注册表：

```go
ui.IconExists("logo")     // 是否已注册
ui.IconDefOf("logo")      // (IconDef, bool)
ui.IconNames()            // []string，按注册顺序（示例页按此铺网格）
ui.IconCount()            // 已注册数量
```

## 3. 渲染与缓存

* 图标纹理是「白色 + 覆盖率 alpha」，绘制时用 `Tint` 染色 → **同尺寸不同颜色共用一张纹理**；
* 缓存键是 `(名称, 像素边长, 线宽)`：只有尺寸真的变化（拖窗口 / DPI 切换 / 缩放）才重新光栅化，
  正常帧只有一次 `DrawImage`；
* 像素边长按 `effScale() * textureSS()` 计算并夹在 4~512 之间，不随窗口无限增长；
* 纹理挂在 `App.icons` 上，空闲回收 / 关窗时统一释放，下一帧按需重建；
* 找不到名称或 path 为空时**不绘制任何内容**（返回 nil 纹理），不会画出缺字形方块。

## 4. 与旧图标的区别

| | `ui.Icon`（Iconfont） | `ui.DefaultIcon()` / `SetWindowIcon` |
| --- | --- | --- |
| 用途 | 界面内的矢量图标 | 窗口 / 任务栏图标（ICO 字节） |
| 尺寸 | 任意，按需光栅化 | 系统决定（16/32/48…） |
| 换色 | 任意颜色 | 不可 |

文字符号（`"→"`、`"☀"`）仍然可以用 `ui.MirrorIcon` 做 RTL 镜像；
需要「真正的图形图标」时优先用 `ui.Icon`。

## 5. 示例

`go run ./cmd/gallery --case icons`（别名 `icon` / `iconfont`）：
关键字过滤、尺寸档位、线宽档位、调色板循环、点击复制图标名、运行时注册自定义图标。
