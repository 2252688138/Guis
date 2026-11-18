<!-- group: 低代码与扩展 -->
# uidsl · 外部界面描述（JSON / DSL）与改文件即时刷新

界面写在文件里，`view()` 返回 `l.Root()`：**改文件 → 界面立刻变，不重新编译、不重启进程**。
Go 只负责写好回调（按钮点下去做什么），「长什么样」交给描述文件。

```go
l, err := uidsl.Open("ui.json", uidsl.Actions{
    "hello": func() { ui.ToastShow("点了按钮") },
})
if err != nil {
    log.Fatal(err)
}
defer l.Close()

ui.SetEntry(func() (ui.Options, func() *ui.Node) {
    return ui.Options{Title: "低代码界面", Width: 900, Height: 620}, l.Root
})
ui.Main()
```

`Open` 之后内部按 200ms 轮询文件修改时间，变更即重新解析 → `ui.RequestRedraw()`。
**解析失败不会白屏**：保留上一版界面，错误记在 `l.LastError()`，适合显示在状态条上。

## 1. 节点类型

| type | 构造器 | 说明 |
| --- | --- | --- |
| `box` / `vbox` | `ui.Box` | 纵向全能容器（交叉轴拉伸） |
| `col` / `column` | `ui.Column` | 纵向弹性容器 |
| `row` | `ui.Row` | 横向弹性容器 |
| `center` | `ui.Center` | 居中容器（只取第一个子节点） |
| `stack` | `ui.Stack` | 叠层容器（后声明的画在上层） |
| `grid` | `ui.Grid` | 网格，配 `cols` / `colW` |
| `wrap` | `ui.Wrap` | 流式换行 |
| `scroll` | `ui.Scroll` | 纵向滚动（自动补稳定 Key，编辑描述文件不丢滚动位置） |
| `spacer` | `ui.Spacer` | 吃满剩余空间的空白 |
| `text` | `ui.Text` | 文本 |
| `button` | `ui.Button` | 按钮（见下方默认样式） |
| `icon` | `ui.IconSized` | 内置图标集名字（如 `star` / `check` / `palette`） |
| `image` | `ui.Image` | 图片路径（PNG/JPEG/GIF/BMP） |

## 2. 属性

属性名与 `ui.St()` 的方法一一对应，未写即不设置：

| 属性 | 对应 | 属性 | 对应 |
| --- | --- | --- | --- |
| `w` `h` | `W` `H` | `wp` `hp` | `Wp` `Hp`（百分比） |
| `minW` `maxW` `minH` `maxH` | 同名方法 | `pad` `padX` `padY` | `Padding` `PadXY` |
| `mar` `marX` `marY` | `Mar` `MarXY` | `gap` | `GapOf` |
| `grow` | `GrowTo` | `cols` `colW` | `ColsN` `ColWidth` |
| `bg` | `BgC`（`#rrggbb` / `rgba(…)`） | `bgFrom` `bgTo` `bgAngle` | `BgGrad` |
| `r` | `R`（圆角） | `stroke` `strokeColor` | `Stroke` |
| `shadowBlur` `shadowY` `shadowSpread` `shadowColor` | `Shadow` | `opacity` | `Opacity` |
| `size` `weight` `color` | `Text(size, weight, color)` | `align` `justify` `dir` | 对齐与主轴方向 |
| `clip` | `Clipped`（裁剪子内容） | `key` / `id` | `Node.K`（交互状态标识） |
| `onClick` | 事件名，查 `Actions` | `icon` `text` | 图标名 / 文本 / 图片路径 |

对齐取值：`start` `center` `end` `stretch`（也接受 `left/right/top/bottom/fill`）。
方向取值：`row` / `col`。

**按钮默认样式**：框架的 `ui.Button` 只设了高度与最小宽度，没有字号和文字色（不设就画不出字），
uidsl 补了一套默认值（字号 14 / 字重 500 / 浅色文字 / 深蓝底 / 圆角 8 / 高 34 / 左右内边距 14）。
描述文件里写了 `size` / `color` / `bg` 等就以你写的为准。

## 3. JSON 格式

顶层可以直接是节点，也可以包一层 `root`：

```json
{
  "type": "col",
  "gap": 12,
  "pad": 16,
  "bg": "#0f1626",
  "children": [
    {"type": "text", "text": "界面标题", "size": 22, "weight": 700, "color": "#e6edf7"},
    {
      "type": "row", "gap": 10,
      "children": [
        {"type": "button", "text": "确定", "key": "ok", "onClick": "confirm"},
        {"type": "button", "text": "取消", "key": "cancel"}
      ]
    }
  ]
}
```

## 4. DSL 格式

缩进决定层级，一行一个节点，`key=value` 是属性；行首 `#` 或 `//` 是注释。

```
# 一行一个节点
col gap=12 pad=16 bg=#0f1626
  text "界面标题" size=22 weight=700 color=#e6edf7
  row gap=10
    button "确定" id=ok onClick=confirm
    button "取消" id=cancel
  spacer
  scroll id=list grow=1 gap=6
    text "改这个文件，界面立刻变"
```

- 属性名大小写不敏感（`onClick` 与 `onclick` 等价）；
- 写错属性名会**直接报错并给出行号**（第 N 行: 未知属性 "sizee"），不会静默失效；
- `text` / `button` 后面的带引号字符串就是文本，`icon` 后面是图标名，`image` 后面是图片路径；
- 多个根节点会自动包一层 `col`。

## 5. 实时刷新 API

| 方法 | 说明 |
| --- | --- |
| `uidsl.Open(path, actions)` | 打开并开始监听（按扩展名判断格式：`.json` 走 JSON，其余按 DSL） |
| `uidsl.OpenAs(path, format, actions)` | 指定格式（例如 DSL 存成 `.txt`） |
| `(*Live).Root()` | 按当前声明新建一棵节点树（每帧调用） |
| `(*Live).Reload()` | 立刻重读文件，不等轮询 |
| `(*Live).LastError()` | 最近一次解析失败原因（成功后为 nil） |
| `(*Live).OnLoad(func(Node))` | 每次成功加载后的回调 |
| `(*Live).SetInterval(d)` | 轮询间隔（默认 `uidsl.DefaultInterval` = 200ms） |
| `(*Live).Close()` | 停止监听 |

`uidsl.Build(node, actions)` 可以把一份声明（`uidsl.Node`）直接转成 `*ui.Node`，不经过文件——
适合「描述来自数据库 / 网络 / 编辑器」的场景。

## 6. 示例

`cmd/uidemo` 是最小可跑的示例，同一份界面的两种写法都在里面：

```bash
go run ./cmd/uidemo              # 读 cmd/uidemo/demo.json
go run ./cmd/uidemo -f demo.dsl  # 读 DSL 版
```

窗口里改 `demo.json` 里的文字或颜色，保存后即时生效；把 JSON 改坏可以看到底部状态条报错、界面保持上一版。
