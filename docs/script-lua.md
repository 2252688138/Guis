# Lua 脚本（guis/script）

让 **Lua 脚本直接驱动 guis 界面**：脚本里写 `function view()` 返回节点表，框架每帧把它转成 `ui.Node`；事件、定时器、文件、HTTP、JSON 都在同一个 Lua VM 里跑，改脚本保存即热重载，不用重新编译。

- 引擎：[gopher-lua](https://github.com/yuin/gopher-lua)（纯 Go，无 CGO，全平台可用）
- 包路径：`guis/script`（不改动 `ui` 代码）
- 示例：`examples/lua-demo`、`cmd/guislua`

## 快速开始

```go
package main

import (
	"guis/script"
	"guis/ui"
)

func main() {
	err := script.RunFile("ui.lua",
		ui.Options{Title: "脚本应用", Width: 900, Height: 620},
		script.Options{})
	if err != nil {
		panic(err)
	}
}
```

```lua
-- ui.lua
count = 0
function inc() count = count + 1 end

function view()
  return ui.col{ pad = 16, gap = 12,
    ui.text{ key = "cnt", text = "计数：" .. count, size = 22 },
    ui.button{ text = "+1", onClick = "inc" },
  }
end
```

跑起来以后改 `ui.lua` 保存，界面自动刷新（默认 200ms 轮询）。

## Go 侧 API

```go
type Options struct {
	WorkDir        string        // fs.* / require 的根目录（默认脚本所在目录）
	NoFS           bool          // 关闭 fs.*
	NoNet          bool          // 关闭 http.*
	HttpTimeout    time.Duration // 默认 10s
	HTTPMaxBytes   int64         // 响应体上限，默认 8MB
	FSMaxReadBytes int64         // fs.read 单次读取上限，默认 8MB
	Interval       time.Duration // 热重载轮询（默认 200ms，负数关闭）
	ExecTimeout    time.Duration // 单次脚本执行上限（默认 5s）
	OnError        func(error)   // 脚本错误出口
	Logger         func(level, msg string)
	OnQuit         func()        // ui.quit() 的处理
	OnTitle        func(string)  // ui.title(s) 的处理
	OnExit         func()        // 关闭前的宿主钩子
	Args           []string      // 脚本 arg 表
	ModulePath     []string      // require 的额外搜索目录
	OnViewport3D   func(key string) *ui.Viewport3DOpts // 3D 视口渲染源（脚本 ui.viewport）
}
```

| Go 侧函数 | 说明 |
|---|---|
| `script.RunFile(path, uiOpts, scriptOpts)` | 用脚本启动应用（阻塞到窗口关闭） |
| `script.Attach(path, scriptOpts)` | 只装载脚本，返回可传给 `ui.SetEntry` 的引擎 |
| `e.DoString(src)` / `e.DoFile(path)` | 执行脚本片段 / 文件（`DoFile` 会记为热重载目标） |
| `e.Root()` | 当前界面树（传给 `ui.Run(opts, e.Root)`） |
| `e.LastError()` | 最近一次错误（成功执行后清空） |
| `e.SetArgs(args)` | 填 `arg` 表 |

## ui.* —— 搭界面

构造器与 `ui/uidsl` 的类型名 1:1，属性名与 `uidsl.Node` 的 JSON 标签一致。

### 容器

| 调用 | 说明 |
|---|---|
| `ui.box{...}` `ui.col{...}` `ui.row{...}` | 盒子 / 纵向 / 横向 |
| `ui.center{...}` `ui.stack{...}` `ui.grid{...}` `ui.wrap{...}` | 居中 / 叠放 / 栅格 / 自动换行 |
| `ui.scroll{...}` `ui.spacer{}` | 滚动容器 / 弹性空隙 |
| `ui.card{...}` `ui.masonry{...}` | 卡片 / 瀑布流 |

### 文本与图形

| 调用 | 说明 |
|---|---|
| `ui.text{ text=..., size=, weight=, color=/fg= }` | 文本（`text` 写数字/布尔会自动 tostring） |
| `ui.button{ text=..., onClick= }` | 按钮 |
| `ui.icon{ icon="...", size= }` `ui.image{ text="路径或URL" }` | 图标 / 图片 |
| `ui.divider{}` `ui.arrow{ color= }` | 分隔线 / 箭头 |
| `ui.badge{ count= }` `ui.avatar{ text=, color= }` `ui.chip{ label=, checked=, onClick= }` | 角标 / 头像 / 标签 |
| `ui.spinner{ size=, color= }` `ui.skeleton{ w=, h= }` | 加载动画 / 骨架屏 |
| `ui.markdown{ src="..." }` `ui.richtext{ spans={ {text=,color=,code=} } }` | Markdown / 富文本 |

### 表单与值控件（`onChange` 回调带参数）

| 调用 | 回调参数 |
|---|---|
| `ui.input{ key=, value=, placeholder=, onChange= }` | string |
| `ui.textarea{ key=, value=, placeholder=, h=, onChange= }` | string |
| `ui.checkbox{ key=, text=, checked=, onChange= }` | bool |
| `ui.switch{ key=, checked=, onChange= }` | bool |
| `ui.slider{ key=, pct=, onChange= }` | number |
| `ui.progress{ pct=, color= }` | — |
| `ui.select{ key=, items={ {value=,label=} }, value=, placeholder=, onChange= }` | string |
| `ui.segmented{ items={ {value=,label=} }, active=, onSubmit= }` | string |

### 数据与大件

| 调用 | 说明 |
|---|---|
| `ui.chart{ ctype="line|area|bar|pie|donut|radar", labels={...}, series={ {name=,color=,values={...}} } }` | 图表 |
| `ui.codeeditor{ key=, src="...", lang=, readonly= }` | 代码编辑器（只读展示 / 轻编辑） |
| `ui.term{ key=, lines={"a","b"}, size= }` | 终端视图（同 key 复用 `TermState`，内容增量写入） |
| `ui.canvas{ key=, w=, h=, bg=, shapes={...} }` | 自绘画布（形状见下文） |
| `ui.video{ key=, src="a.mp4" }` | 视频播放器（需 ffmpeg / ffprobe 在 PATH；打不开降级为提示文本） |
| `ui.webview{ key=, src="https://...", html=, zoom=, w=, h= }` | 内嵌浏览器（平台无后端时降级为提示文本）；**建议显式写 w/h**，纵向容器里没尺寸可能塌成 0 高 |
| `ui.viewport{ key="main" }` | 3D 视口（渲染源由**宿主**注入，见下文） |
| `ui.menubar{ key=, menus={ {label="文件", items={ {label="新建", onClick="new"} }} } }` | 菜单栏 |

### Canvas 自绘

画布在 CPU 上把一组声明式形状光栅成纹理，**形状没变时不重绘**（静态图形零开销）：

```lua
ui.canvas{ key = "board", w = 320, h = 180, bg = "#101830", shapes = {
  { kind = "rect",      x = 12, y = 12, w = 120, h = 48, r = 10, fill = "#4c8dff" },
  { kind = "roundrect", x = 150, y = 12, w = 120, h = 48, r = 12, stroke = "#7cf", strokeW = 2 },
  { kind = "circle",    cx = 60, cy = 120, radius = 34, fill = "#ff6b6b" },
  { kind = "ring",      cx = 150, cy = 120, radius = 34, stroke = "#4cd964", strokeW = 6 },
  { kind = "polygon",   points = { {200,90}, {290,90}, {245,160} }, fill = "#ffd166" },
  { kind = "arc",       cx = 245, cy = 90, radius = 40, a0 = 200, a1 = 340, stroke = "#fff", strokeW = 2 },
  { kind = "line",      x = 12, y = 172, w = 300, h = 172, stroke = "#ffffff66", strokeW = 2 },
  { kind = "polyline",  points = { {10,10}, {40,30}, {70,10} }, stroke = "#fff", strokeW = 2 },
} }
```

| 字段 | 说明 |
|---|---|
| `kind` | `rect` `roundrect` `line` `polyline` `polygon` `circle` `ring` `arc`（未知值忽略） |
| `x,y,w,h` | 矩形 / 圆角矩形的位置尺寸；`line` 用 `x,y` 起点与 `w,h` 终点 |
| `r` | 圆角半径（`roundrect`） |
| `points` | `{ {x,y}, ... }`（`polyline` / `polygon`） |
| `cx,cy,radius` | 圆心与半径（`circle` / `ring` / `arc`） |
| `a0,a1` | 圆弧起止角度（度，0 = 正右，顺时针） |
| `fill` / `stroke` / `strokeW` | 填充色 / 描边色 / 线宽 |
| `opacity` | 该形状整体透明度（乘进 alpha） |
| `bg` | 画布底色（画在形状下面） |

颜色写法：`#rgb`、`#rrggbb`、`#rrggbbaa` 或具名色。

### 动画

节点都可以带 `anim`（可以多条叠加，进场 + 持续各一条）：

```lua
ui.card{ anim = { { kind = "pop", dur = 0.25 }, { kind = "breathe", loop = true, amp = 0.03 } },
  ui.text{ text = "卡片" },
}
```

| 字段 | 说明 |
|---|---|
| `kind` | `fade` `slide` `scale/pop/zoom` `shake` `pulse/breathe` `float` `bounce` `flipx` `flipy` `rotate` `spin` `swing` `jello` `heartbeat` `tada` `flash` `drop` `rubber`（未知值按 `fade`） |
| `side` | `left` `right` `top` `bottom`（`slide` 用） |
| `dur` / `delay` | 秒；`dur` <= 0 按 0.3 |
| `dist` / `amp` / `scale` / `rot` | 位移距离 / 幅度 / 缩放起点 / 起始弧度 |
| `loop` / `out` | 循环播放 / 出场方向 |
| `ease` | `linear` `in_out_quad` `in_out_cubic` `out_back`（空 = 框架默认） |

### 3D 视口（宿主注入渲染源）

脚本只负责声明与布局，渲染源由 Go 侧提供：

```go
opts := script.Options{
	OnViewport3D: func(key string) *ui.Viewport3DOpts {
		if key == "main" {
			return &ui.Viewport3DOpts{Src: gameEngine, Key: key, MouseLock: true}
		}
		return nil // 返回 nil 表示不支持，界面降级为提示文本
	},
}
```

```lua
ui.viewport{ key = "main" }
```

### 弹层

| 调用 | 说明 |
|---|---|
| `ui.alert{ key=, open=, dlgKind="info|warn|error", title=, body=, onClick= }` | 提示框 |
| `ui.confirm{ key=, open=, title=, body=, okLabel=, onClick=, onCancel= }` | 确认框 |
| `ui.drawer{ key=, open=, title=, body=, w=, onClick= }` | 右侧抽屉 |

### 运行期控制

| 调用 | 说明 |
|---|---|
| `ui.set(key, prop, value)` | 改某个节点的属性（`value=nil` 表示撤销覆盖） |
| `ui.reset(key)` | 清掉某节点的全部覆盖 |
| `ui.redraw()` | 请求重绘（改脚本状态后一般不需要手动调） |
| `ui.toast(msg)` / `ui.title(s)` / `ui.quit()` | 轻提示 / 改窗口标题 / 退出 |
| `ui.error()` | 取最近一次错误信息字符串 |

### 系统能力

| 调用 | 说明 |
|---|---|
| `ui.clipboard.get()` / `ui.clipboard.set(s)` | 读写剪贴板（set 返回 bool） |
| `ui.focus(key)` / `ui.scrollTo(key, y)` | 聚焦 / 滚动定位 |
| `ui.notify(msg[, title, kind])` | 系统通知 |
| `ui.notifyProgress(id, pct, body)` / `ui.notifyClose(id)` / `ui.notifyClear()` | 进度通知 |
| `ui.openFile([title, filter])` | 文件选择框，取消返回 nil |
| `ui.key.enter` `ui.key.esc` `ui.key.f1` … | 键码常量（配合 `onKey`） |

## 事件

事件属性写**函数名**（或直接写函数，会自动登记成匿名名）：

```lua
function view()
  return ui.col{
    ui.input{ key = "i", onKey = "keydown", onChar = "typed" },
    ui.box{ key = "b", onHover = "hover", onScroll = "wheel", onDrag = "drag" },
    ui.button{ text = "点我", onClick = function() count = count + 1 end },
  }
end
```

| 属性 | 回调参数 |
|---|---|
| `onClick` `onPress` `onActivate` | 无 |
| `onChange` / `onSubmit` | 新值 / 提交文本 |
| `onKey` / `onKeyUp` | 键码（number，配合 `ui.key.*`） |
| `onChar` | 字符（string） |
| `onHover` | bool（进入 / 离开） |
| `onScroll` | dy（number） |
| `onDrag` | x, y |
| `onDrop` | 路径数组（Lua 表） |
| `onClickAt` | x, y |

事件名如果脚本里没有对应函数，会记一条错误（`ui.error()` 可读），但不阻断其它节点。

## timer.* —— 定时器

```lua
timer.every(1000, function() clock = os.date("%H:%M:%S") end)
local id = timer.after(2000, function() ui.toast("2 秒到了") end)
timer.cancel(id)
```

回调在独立协程里跑，但进 Lua 前先拿锁；拿不到锁就跳过这一拍，绝不卡 UI。

## json.*

```lua
local t = json.decode('{"ok":true,"list":[1,2,3]}')
local s = json.encode{ ok = true, list = { 1, 2, 3 } }
```

数字优先解成整数，非法 JSON 抛错。

## http.*

```lua
-- 同步：status, body, headers
local st, body, hd = http.get("https://api.example.com/x", {
  headers = { ["X-Token"] = "t" },
  timeout = 3000,     -- 毫秒，覆盖默认
  max_bytes = 1048576 -- 响应体上限
})
local data = json.decode(body)

-- 其他方法
http.post(url, json.encode{ a = 1 })            -- POST
http.put(url, body) / http.delete(url)          -- PUT / DELETE
http.request("PATCH", url, { body = body })     -- 任意方法

-- 异步：回调在 Lua 线程执行，不阻塞 UI
http.async("GET", url, { headers = { ["X-Token"] = "t" } }, function(st, body, hd)
  if st == nil then ui.toast("请求失败：" .. body) else result = json.decode(body) end
end)
```

只允许 `http/https`，`NoNet` 时直接报错；响应体超过上限报错而不是撑爆内存。

## fs.* / log.*

| 调用 | 说明 |
|---|---|
| `fs.read(path[, maxBytes])` | 读文本（超过上限报错） |
| `fs.write(path, data)` / `fs.append(path, data)` | 写 / 追加 |
| `fs.exists(path)` / `fs.list(dir)` / `fs.remove(path)` | 存在 / 列表 / 删除 |
| `fs.mkdir(path[, recursive])` | 建目录（默认递归） |
| `fs.rename(old, new)` / `fs.copy(src, dst)` | 重命名 / 复制 |
| `fs.stat(path)` → `{ size, mtime, dir, name }` | 文件信息 |
| `log.info(s)` / `log.warn(s)` / `log.error(s)` | 走 `Options.Logger` |

`fs.*` 的路径必须在 `Options.WorkDir` 之下（绝对路径也一样），`../` 越界直接报错；`NoFS` 时调用报错。

## require —— 拆分脚本

```lua
local util = require("util")        -- WorkDir 下 util.lua
local sib  = require("./sibling")   -- 相对**当前模块**所在目录
local page = require("lib/ui")      -- 子目录 lib/ui.lua
local pkg  = require("pkg")         -- 目录包 pkg/init.lua
print(package.path, package.loaded["util"])
```

- 搜索顺序：`WorkDir` → `Options.ModulePath`（宿主显式开放的额外目录），每个目录试 `name` / `name.lua` / `name/init.lua`；
- `./` `../` 相对当前模块解析（不能跑出 WorkDir/ModulePath）；
- 同一模块只执行一次，结果同时写进 `package.loaded`；循环 require 拿到占位 `true`，不会无限递归；
- 热重载脚本时缓存清空，改了子模块也会生效。

## 沙箱说明

已加载：`base`（去掉 `dofile` `loadfile` `load` `loadstring`）、`string`、`table`、`math`、`coroutine`、`os`（只留 `time` `date` `clock` `difftime` 等，去掉 `execute` `exit` `remove` `rename` `getenv` `setenv` `tmpname`），以及框架注入的 `ui` `timer` `json` `fs` `http` `log` 和受限 `require`。

**未加载**：`io`、`package`、`debug`、`utf8`。需要文件能力用 `fs.*`（有路径边界），网络用 `http.*`（限协议 + 超时 + 体积上限）。

## 热重载与错误处理

- 轮询脚本文件 mtime / size，变了就重新执行：**先清掉上一版脚本的全局、匿名回调、全部定时器、`ui.set` 覆盖与模块缓存（保留标准库与 `ui`/`timer`/`json`/`fs`/`http`/`log`/`require`），再执行新版**，因此**全局状态会被重置**（计数会归零），需要跨重载保留的状态请写到文件或用 `fs.*`。
- 语法错误 / 运行时错误 / 构树错误都走 `Options.OnError`，并保留上一版界面；`ui.error()` 可在界面里把错误显示出来。
- 未知节点类型、未知 `ui.set` 属性都会**明确报错**（不再静默失效）。
- 单次脚本执行默认 5s 超时（`ExecTimeout`），死循环会被中断并报「脚本执行超时」。

## 并发模型

- 单 VM + 单锁：装载 / 事件 / 定时器 / 构树全部串行执行。
- 树缓存 + dirty：脚本状态没变时空闲帧**不进** Lua VM，直接复用上一棵树。
- UI 线程用 `TryLock`：构树时如果定时器/事件正持锁，就用上一棵树顶上并请求下一帧重绘，不会卡帧。
- `http.async` 的结果通过投递队列回到 Lua 线程执行，脚本状态读写仍然串行。
- 不要在脚本里写重循环：所有脚本执行都在同一把锁上，会把 UI 一起拖慢。

## 已知限制

- **Canvas 内不画文字**：`ui.canvas` 只做图形光栅，文字请用相邻的 `ui.text` / `ui.richtext` 叠加（光栅器不带字体渲染）。
- 脚本拿不到 `*ui.Node` 句柄，动画只能写在节点的 `anim` 属性上，**没有运行期逐节点重播/暂停 API**（`ui.Animate` 只能在 Go 侧用）。
- `video` 依赖外部 `ffmpeg` / `ffprobe`；`webview` 依赖平台后端（Windows WebView2 等），不支持时降级为提示文本。
- `term` 是展示型终端：脚本可以通过 `lines` 写内容，但没有 pty 交互（键盘输入回传需宿主接 `ui.Terminal` 自己的会话）。
- 没有 Lua 包管理器 / 版本语义，`require` 只做文件级模块复用。
