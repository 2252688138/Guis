<!-- group: 入门 -->
# 总览 · GUIS 框架与文档地图

GUIS（`guis`）是一套**纯 Go、零 cgo、自绘渲染**的跨平台 GUI 框架：一个窗口、一套声明式节点树，
同时跑在 Windows / Linux / macOS / 浏览器（wasm）/ Android（NativeActivity）。

```go
import "guis/ui"

func init() {
    ui.SetEntry(func() (ui.Options, func() *ui.Node) {
        return ui.Options{Title: "Hello", Width: 900, Height: 620}, view
    })
}

func main() { ui.Main() }

func view() *ui.Node {
    return ui.Center(ui.Text("Hello GUIS").St(ui.St().Text(24, 700, ui.ColText)))
}
```

一句话概括渲染模型：**每帧重建节点树**（`view()` 返回整页），框架负责布局、绘制、命中、焦点与浮层；
交互状态（滚动位置、输入光标、开关过渡……）按节点 `Key` 跨帧保存。

---

## 1. 能力总览

| 领域 | 提供的能力 |
| --- | --- |
| 布局 | Flex（Row/Column）、Stack、Grid、Wrap、Masonry、EqualRow、绝对定位、Constraints 约束模型、断点 `BP` |
| 样式 | 链式 `St()`：尺寸 / 内外边距 / 圆角 / 渐变 / 描边 / 阴影 / 透明度 / 裁剪 / 毛玻璃 / 绝对定位 |
| 主题 | `Theme` 色板 + 语义 `Tokens` + 深浅色 / 跟随系统 + 界面缩放 / 文字锐度 / 超采样 |
| 基础组件 | 文本、按钮、图标按钮、矢量图标库（Iconfont，80+ 内置 + 运行时注册）、标签、徽章、头像、卡片、分隔线、进度、环形、骨架、空状态、横幅 |
| 输入组件 | Input / PasswordInput / Textarea / NumberInput / SearchInput / TagInput / KeyCapture / 上传 / 取色 |
| 选择组件 | Switch（6 种预设 + 任意节点自定义）、Checkbox、Radio、Slider、RangeSlider、Rate、Select、Date、Time、Cascader、AutoComplete |
| 数据展示 | Table / TableV（虚拟 / 分组 / 合计 / 列宽拖拽）、VList、ReorderList、FileTree、图文卡片、社区帖子、分页器、图表（8 种）、Markdown、富文本、代码编辑器（十万行 / 小地图 / LSP） |
| 容器与浮层 | 菜单栏 / 多级菜单、Modal / Alert / Confirm / Prompt / Drawer、Toast、通知中心、Tooltip、命令面板、引导、图片灯箱、Split、Dock |
| 媒体 | 视频播放器（ffmpeg / 系统解码器 / wasm `<video>`）、音频播放器 + FFT 频谱、GIF / 图片序列 |
| 文档 | Excel（xlsx）读写渲染、Word（docx）读写渲染、PDF 读 + 写（中文子集嵌入）、打印与打印预览、导出 PDF |
| 系统 | 文件对话框、剪贴板（文本 / 文件 / 图片 / 监听）、全局热键、托盘、文件拖入、开机自启、多显示器、截屏、单实例、更新器、崩溃报告 |
| 平台 | 窗口后端（Win32 / X11 / Cocoa / UIKit / Android / wasm）、内嵌 WebView2、浏览器 iframe |
| 开发体验 | 热重载（`go run` 下改 .go 自动重编译，窗口几何与业务状态自动恢复）、布局树导出 `--dump`、组件示例 gallery |

平台差异统一用 `ui.HasCap(ui.CapXxx)` 判断（见《附录 E · 平台能力》）。
所有平台都**必须能编译**：不支持的接口有安全降级桩（空操作 / 返回不支持），不会 panic。

---

## 2. 源码结构

```text
guis/
├── ui/                 ← 框架本体（唯一需要 import 的包）
│   ├── app.go          窗口 + 消息泵 + 帧循环 + 焦点/命中/滚动
│   ├── node.go         节点与容器构造器（Box/Row/Column/Stack/Grid/Wrap…）
│   ├── layout.go       布局算法（measure / arrange）与视口、断点
│   ├── style.go        Style 链式样式
│   ├── theme.go        色板与 Col* 颜色别名
│   ├── widgets.go      基础控件（按钮/输入/表格/页签/模态…）
│   ├── form.go         Select / Textarea / NumberInput / Date
│   ├── inputs_extra.go AutoComplete / TagInput / Rate / Cascader / Upload / ColorPicker…
│   ├── cards.go        MediaCard / ListRows / PostCard
│   ├── tree.go         FileTree
│   ├── chart*.go       图表（数据 / 绘制 / 光栅 / 命中）
│   ├── markdown*.go    Markdown 解析 / 渲染 / 高亮 / 状态
│   ├── richtext*.go    行内富文本 + 富文本编辑器
│   ├── codeeditor*.go  代码编辑器（渲染 / 语法 / 查找 / 折叠 / 多光标 / 悬停 / LSP）
│   ├── lsp.go          LSP 客户端（JSON-RPC over stdio）
│   ├── iconfont*.go    矢量图标库（SVG path 光栅化 + 纹理缓存）
│   ├── video*.go       帧序列 / 播放器 / 填充方式
│   ├── media*.go       ffmpeg 解码源（按平台分文件）
│   ├── music*.go       音频源 + 播放器 + 频谱
│   ├── office.go       办公文档渲染节点
│   ├── print.go        打印 / 预览 / 内置打印对话框
│   ├── system*.go      系统能力（剪贴板/热键/托盘/拖入/多屏/截图/单实例）
│   ├── hotreload*.go   热重载（守护进程 + 子进程现场恢复）
│   ├── autostart*.go   开机自启
│   ├── webview*.go     内嵌浏览器
│   └── backend_*.go    平台窗口后端
├── internal/
│   ├── geom/ gpu/ text/   几何、OpenGL 绘制、字体与字形
│   ├── win32/           Win32 / Media Foundation / WebView2 / 打印 / 托盘绑定
│   ├── x11/ cocoa/ objc/ uikit/ android/ display/   其他平台
│   ├── office/          xlsx / docx / pdf 读写（纯标准库）
│   ├── audio/           音频输出设备（waveOut / ALSA / AudioQueue / wasm）
│   └── version/         构建版本信息
├── docs/api/            ← 本手册（Markdown，go:embed 进二进制）
└── cmd/gallery/         全部组件的可视化示例（本手册的活演示）
```

---

## 3. 文档地图

| 分组 | 章节 |
| --- | --- |
| 入门 | `00` 总览 · `01` 快速开始 · `02` 应用与窗口 · `03` 热重载 |
| 布局与样式 | `10` 节点与布局 · `11` 样式 St() · `12` 主题 / 缩放 / 字体 |
| 事件与交互 | `20` 事件与焦点 · `21` 文本编辑与输入法 |
| 基础组件 | `30` 基础组件 · `31` 输入与表单 · `32` 选择器 · `34` 图标库 |
| 数据展示 | `40` 列表 / 表格 / 树 · `41` 图表 · `42` Markdown · `43` 富文本 · `44` 代码编辑器 |
| 容器与浮层 | `50` 菜单与标题栏 · `51` 浮层 / 弹窗 · `52` 提示与动画 · `53` 分栏与停靠 |
| 媒体与文档 | `60` 视频 · `61` 音频 · `62` 办公文档与打印 |
| 系统能力 | `70` 系统能力 · `71` 桌面增强 · `72` WebView · `73` 偏好 / 多语言 i18n / 无障碍 |
| 附录 | `A` 核心 API · `B` 组件 API · `C` 文本与媒体 API · `D` 系统/平台 API · `E` 平台能力矩阵 |

> [!TIP]
> 附录 A~D 是从源码**自动导出**的完整符号表（含每个导出符号的签名与注释首行）。
> 正文按「怎么用」讲解，附录保证「一个也没漏」——查某个函数是否存在，直接看附录。

---

## 4. 运行示例

```bash
go run ./cmd/gallery                    # 全部组件示例（桌面）
go run ./cmd/gallery --case office      # 直接打开某分类页
go run ./cmd/gallery --dump d.txt --case table   # 导出首帧布局树
GOOS=js GOARCH=wasm go build -o dist/app.wasm ./cmd/gallery   # Web 版
./build-android-apk.ps1                # 安卓 APK
```

`gallery` 的每一页（`cmd/gallery/pages/case_*.go`）都是可以整段拷走改写的示例代码。

---

## 5. 阅读建议

1. 先读 `01` 快速开始，跑起 `go run ./cmd/gallery`；
2. 手上有界面要搭，按 `10 → 11 → 30` 的顺序（布局 → 样式 → 控件）即可开工；
3. 要弹窗 / 通知 / 动画，看 `51`、`52`；
4. 要多端一致，务必读 `附录 E`，用 `HasCap` 做分支而不是 `runtime.GOOS`；
5. 开发期想要「保存即刷新」，读 `03` 热重载；要做 IDE 类界面，读 `44` 代码编辑器，
   图标统一用 `34` 的 `ui.Icon`，不要再用文字符号拼图标。
