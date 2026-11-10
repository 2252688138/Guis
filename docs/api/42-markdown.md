<!-- group: 数据展示 -->
# Markdown 渲染

## 1. 一行渲染

```go
func view() *ui.Node {
    return ui.Stack(
        ui.Scroll(
            ui.Markdown("doc", src, ui.MarkdownOptions{Width: 820}),
        ).K("scroll-md").St(ui.St().GrowTo(1)),
        ui.MarkdownLightbox(),     // 图片放大层：必须在根 Stack 末尾
    ).St(ui.St().Hp(100))
}
```

`ui.Markdown(key, src, opts...)` 返回一个纵向节点，宽度固定为 `opts.Width`（0 = `min(视口-96, 860)`）。

## 2. MarkdownOptions 全部字段

```go
type MarkdownOptions struct {
    Width            float32   // 排版宽度；0 = min(ViewportW()-96, 860)
    BaseSize         float32   // 正文字号，默认 13.5
    FontFamily       string    // 正文字体（空 = 全局默认）
    MonoFamily       string    // 代码字体，默认 Consolas
    LineHeight       float32   // 行高，默认 BaseSize*1.65
    MaxImageHeight   float32   // 图片最大高度，默认 360
    CodeMaxLines     int       // 代码块折叠阈值（0 = 22，<0 不折叠）
    CodeNoWrap       bool      // 代码块横向平移而不是折行
    OnLink func(url string)    // 链接点击；nil 时用系统浏览器打开
    NoTOC            bool      // 不渲染 [toc]
}
```

## 3. 支持的语法

| 类别 | 支持情况 |
| --- | --- |
| 块级 | 标题 1~6、段落、分割线、引用、列表（有序/无序、嵌套）、任务列表、表格、代码块、脚注 |
| 行内 | 粗体、斜体、粗斜体、删除线、`==高亮==`、行内代码、链接、图片、`<kbd>` |
| GitHub 提示块 | `> [!NOTE]` / `[!TIP]` / `[!WARNING]` / `[!IMPORTANT]` / `[!CAUTION]` |
| 代码块 | 行号 + 语法着色 + 复制按钮 + 超 `CodeMaxLines` 行折叠 |
| 扩展块 | `[toc]`、`[tabs]`、`[details]`、`[card]`、`[badge]`、`[progress]`、`[gallery]` |
| 占位 | `[video]` / `[audio]` 显示为占位卡（真实播放用 `VideoPlayer` / `MusicPlayer`） |
| 不支持 | Mermaid、数学公式、远程图片（降级为占位卡，仅本地文件） |

语法着色的语言：go / js / ts / py / json / bash / sql / html / css / yaml
（关键字表在 `ui/markdown_keywords.go`）。

## 4. 扩展块写法

```markdown
[toc]

[tabs]
[tab label="概览"]
第一个页签的内容
[/tab]
[tab label="代码"]
```go
fmt.Println("hi")
```
[/tab]
[/tabs]

[details 点击展开]
被折叠的内容
[/details]

[card 标题|副标题]
卡片正文
[/card]

[badge 推荐|#22d3ee]
[progress 0.62]
[gallery a.png,b.png,c.png]
```

## 5. 图片与链接

```go
ui.Markdown("doc", src, ui.MarkdownOptions{
    OnLink: func(url string) {
        // 拦截外链：内链跳转、外链询问后再打开
        if strings.HasPrefix(url, "app://") { gotoPage(url) } else { ui.ShellOpen(url) }
    },
})
```

* 图片点击后进入全屏灯箱（`MarkdownLightbox()` 提供浮层）。
* 也可以手动打开：`ui.MarkdownOpenImage(src, alt)`。
* 灯箱是全局单例，同一窗口所有 Markdown 共用。

## 6. 缓存与状态

渲染树按 `(窗口, key, 源文, 宽度, 缩放, 主题, 交互版本, 排版选项)` 缓存，
滚动长文档时每帧只做一次 map 查找（旧实现每帧重排上千节点，会吃掉大半帧预算）。

```go
ui.MarkdownResetState()   // 清空页签 / 折叠 / 图片预览等交互状态（比如切文档时）
```

* 文档超大（节点数超阈值）时**不缓存**，避免常驻内存。
* 改了 `src` 会自动重建；只改主题 / 宽度也会命中不同的缓存键，不会出现「画面不刷新」。

## 7. 解析但不着色（目录提取 / 搜索）

```go
doc := ui.MarkdownParse(src)   // 返回解析结果（块级结构，调试与自定义渲染用）
```

## 8. 典型「文档浏览器」布局

左侧章节列表 + 右侧正文（本页所在的帮助页就是这么写的）：

```go
ui.Row(
    ui.Column(ui.Each(chapters, func(c Chapter, i int) *ui.Node {
        bg := "rgba(0,0,0,0)"
        if i == cur {
            bg = ui.ColBgSoft        // 选中态底色
        }
        return ui.Text(c.Title).K("ch-" + c.ID).Tap(func() { cur = i }).
            St(ui.St().PadXY(12, 9).R(8).BgC(bg))
    })...).St(ui.St().W(240).GapOf(2)),
    ui.Scroll(ui.Markdown("doc", chapters[cur].Body, ui.MarkdownOptions{Width: 0})).
        K("scroll-doc").St(ui.St().GrowTo(1)),
)
```
