// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// gallery 的 API 文档页：左侧文档目录 + 右侧 Markdown 正文。
//
// 文档本体是仓库里的 docs/api/*.md，通过 guis/docs 的 go:embed 打进二进制，
// 因此桌面 / 浏览器 / 安卓三端都能直接看（不依赖外部文件，安装包也不会漏拷）。
//
// 约定（新增一篇文档只需丢文件，不用改代码）：
//
//	docs/api/NN-xxx.md
//	<!-- group: 分组名 -->         ← 可选：左侧列表的分组标题，缺省归入「其它」
//	# 文档标题                     ← 左侧列表显示的名字（正文里不再重复渲染）
//	...                            ← 其余内容原样交给 ui.Markdown 渲染
package pages

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"guis/docs"
	"guis/ui"
)

// apiDoc 一篇内嵌文档。
type apiDoc struct {
	File  string // 文件名（含 .md），排序与选中态都用它
	Title string // 首行 "# 标题"
	Group string // 前置元数据里的分组名
	Body  string // 去掉元数据与首行标题后的正文
}

var (
	apiDocsOnce sync.Once
	apiDocsList []apiDoc
	// apiDocSel 当前选中的文档文件名（跨帧保持）。
	apiDocSel = ""
	// apiDocQuery 左侧目录的过滤关键字。
	apiDocQuery = ""
	// APIInitialDoc 启动时指定的初始文档（--api-doc=20-video.md，调试 / 截图用）。
	APIInitialDoc = ""
)

// loadAPIDocs 读取并解析全部内嵌文档（只做一次）。
func loadAPIDocs() []apiDoc {
	apiDocsOnce.Do(func() {
		entries, err := fs.ReadDir(docs.API, "api")
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := docs.API.ReadFile("api/" + e.Name())
			if err != nil {
				continue
			}
			apiDocsList = append(apiDocsList, parseAPIDoc(e.Name(), string(data)))
		}
		// 按文件名前缀（00- / 01- / 90- …）排序，天然得到阅读顺序
		sort.Slice(apiDocsList, func(i, j int) bool { return apiDocsList[i].File < apiDocsList[j].File })
		if len(apiDocsList) > 0 {
			apiDocSel = apiDocsList[0].File
		}
		// --api-doc：启动即定位到指定文档（找不到就保持第一篇）
		if APIInitialDoc != "" {
			if findAPIDoc(apiDocsList, APIInitialDoc) != nil {
				apiDocSel = APIInitialDoc
			}
		}
	})
	return apiDocsList
}

// parseAPIDoc 解析单篇：抽出分组与标题，其余作为正文。
func parseAPIDoc(name, src string) apiDoc {
	d := apiDoc{File: name}
	var lines []string
	titleDone := false
	inHead := true
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimRight(raw, "\r")
		if inHead {
			trim := strings.TrimSpace(line)
			switch {
			case trim == "":
				if !titleDone {
					continue
				}
			case strings.HasPrefix(trim, "<!--") && strings.HasSuffix(trim, "-->"):
				// 前置元数据：只认 group
				inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trim, "<!--"), "-->"))
				if v, ok := strings.CutPrefix(inner, "group:"); ok {
					d.Group = strings.TrimSpace(v)
				}
				continue
			case !titleDone && strings.HasPrefix(trim, "# "):
				d.Title = strings.TrimSpace(trim[2:])
				titleDone = true
				continue
			default:
				inHead = false
			}
		}
		lines = append(lines, line)
	}
	if d.Title == "" {
		d.Title = strings.TrimSuffix(name, ".md")
	}
	if d.Group == "" {
		d.Group = "其它"
	}
	d.Body = strings.Join(lines, "\n")
	return d
}

// findAPIDoc 按文件名找文档，找不到返回 nil。
func findAPIDoc(list []apiDoc, file string) *apiDoc {
	for i := range list {
		if list[i].File == file {
			return &list[i]
		}
	}
	return nil
}

// pickAPIDoc 切换文档：回到正文顶部再重绘（选中态由 apiDocSel 跨帧保存）。
func pickAPIDoc(file string) {
	if file == apiDocSel {
		return
	}
	apiDocSel = file
	ui.ScrollTo("api-doc-scroll", 0)
	ui.RequestRedraw()
}

// displayAPITitle 列表里用的短标题：优先取 " · " 之前的主标题，
// 过长时按 rune 截断——目录条目一律单行，不折行（折行会让左栏高度失控）。
func displayAPITitle(title string) string {
	if i := strings.Index(title, " · "); i > 0 {
		head, tail := title[:i], title[i+len(" · "):]
		if strings.HasPrefix(head, "附录") {
			return head + " · " + truncRunes(tail, 9)
		}
		return head
	}
	return truncRunes(title, 14)
}

// truncRunes 按 rune 截断并补省略号。
func truncRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

// apiDocSize 正文规模的可读描述（"≈1.8k 字"）。
func apiDocSize(body string) string {
	n := utf8.RuneCountInString(body)
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// APIView 「帮助 → API 文档」整页：左侧目录 + 右侧正文。
//
// 该页面自带双栏，所以壳层在 Current == "api" 时会隐藏左侧导航
// （见 shell.go 的 View），避免出现「三栏挤在一起」。
func APIView() *ui.Node {
	list := loadAPIDocs()
	if len(list) == 0 {
		return ui.Center(ui.Text("API 文档未内嵌：请确认 docs/api/*.md 存在").
			St(ui.St().Text(13.5, 500, ui.ColTextDim)))
	}
	if findAPIDoc(list, apiDocSel) == nil {
		apiDocSel = list[0].File
	}

	// 窄屏收窄目录栏，保证正文仍有可读宽度
	listW := float32(288)
	switch vw := ui.ViewportW(); {
	case vw < 780:
		listW = 188
	case vw < 1060:
		listW = 238
	}

	return ui.Row(
		apiDocSidebar(list, listW),
		ui.Box().St(ui.St().W(1).Hp(100).BgC(ui.ColLine)), // 栏间分隔线
		apiDocContent(list, listW),
	).St(ui.St().GrowTo(1).Hp(100).BgC(ui.ColBg))
}

// apiDocHover 目录当前悬停项（只影响外观，不参与布局）。
var apiDocHover = ""

// apiDocGroupHeader 目录里的分组标题：名称 + 一条细线（计数只放在品牌头的总篇数里，
// 避免和右侧序号撞在一起像两列编号）。
func apiDocGroupHeader(name string) *ui.Node {
	return ui.Row(
		ui.Text(name).St(ui.St().Text(10.5, 700, ui.ColTextMuted)),
		ui.Box().St(ui.St().GrowTo(1).H(1).BgC(ui.ColLine)),
	).St(ui.St().GapOf(8).PadOnly(16, 2, 5, 4)).AlignTo(ui.AlignCenter)
}

// apiDocItem 一条目录项：选中态有主色指示条 + 半透明底色，
// 行尾是序号（`__` 占位保证单行不折）。
func apiDocItem(d apiDoc, index int, innerW float32, compact bool) *ui.Node {
	sel := d.File == apiDocSel
	hover := d.File == apiDocHover
	bar := "rgba(99,102,241,0)"
	bg := "rgba(0,0,0,0)"
	label := ui.ColTextDim
	weight := int32(500)
	size := float32(13)
	if compact {
		size = 12.5
	}
	if hover && !sel {
		bg = "rgba(255,255,255,0.045)"
		label = ui.ColText
	}
	if sel {
		bar = ui.ColPrimary
		bg = "rgba(99,102,241,0.15)"
		label = ui.ColText
		weight = 600
	}

	row := ui.Row(
		ui.Box().St(ui.St().W(3).H(13).R(2).BgC(bar)),
		ui.Text(displayAPITitle(d.Title)).St(ui.St().Text(size, weight, label)),
		ui.Spacer(),
		ui.Text(fmt.Sprintf("%02d", index)).
			St(ui.St().Text(9.5, 500, ui.ColTextMuted).Fade(0.45)),
	).St(ui.St().GapOf(9).PadXY(9, 7).R(7).W(innerW).BgC(bg))

	file := d.File
	return ui.TooltipOf(row.K("api-"+file).
		OnHoverEvt(func(in bool) {
			if in {
				apiDocHover = file
			} else if apiDocHover == file {
				apiDocHover = ""
			}
			ui.RequestRedraw()
		}).
		Tap(func() { pickAPIDoc(file) }), d.Title)
}

// apiDocSidebar 左侧目录：品牌头 + 搜索 + 分组列表 + 返回。
func apiDocSidebar(list []apiDoc, w float32) *ui.Node {
	innerW := w - 24
	compact := w < 220

	query := strings.ToLower(strings.TrimSpace(apiDocQuery))
	var rows []*ui.Node
	lastGroup := ""
	shown := 0
	for i, d := range list {
		if query != "" &&
			!strings.Contains(strings.ToLower(d.Title), query) &&
			!strings.Contains(strings.ToLower(d.File), query) &&
			!strings.Contains(strings.ToLower(d.Group), query) {
			continue
		}
		if d.Group != lastGroup {
			lastGroup = d.Group
			rows = append(rows, apiDocGroupHeader(d.Group))
		}
		rows = append(rows, apiDocItem(d, i+1, innerW, compact))
		shown++
	}
	if shown == 0 {
		rows = append(rows, ui.Text("没有匹配的文档").St(
			ui.St().Text(12, 400, ui.ColTextMuted).PadXY(9, 10)))
	}

	// 品牌头：渐变小方块 + 标题 / 规模
	brand := ui.Row(
		ui.Center(ui.Text("API").St(ui.St().Text(8, 700, "#ffffff"))).
			St(ui.St().W(22).H(22).R(6).BgGrad(ui.ColPrimary, ui.ColPrimary2, 120)),
		ui.Column(
			ui.Text("API 文档").St(ui.St().Text(13.5, 700, ui.ColText)),
			ui.Text(fmt.Sprintf("%d 篇 · 随二进制内嵌", len(list))).
				St(ui.St().Text(10, 400, ui.ColTextMuted)),
		).St(ui.St().GapOf(1)),
	).St(ui.St().GapOf(9).PadOnly(2, 0, 10, 2)).AlignTo(ui.AlignCenter)

	foot := ui.Row(
		ui.Btn("← 返回案例", ui.BtnGhost, func() { SwitchCase("basic") }),
		ui.Spacer(),
		ui.Tag("F1", ui.ColTextMuted),
	).St(ui.St().GapOf(8).PadOnly(8, 0, 0, 0)).AlignTo(ui.AlignCenter)

	return ui.Column(
		brand,
		ui.SearchInput("api-query", apiDocQuery, "搜索文档 / 分组", func(v string) {
			apiDocQuery = v
			ui.RequestRedraw()
		}),
		ui.Scroll(ui.Column(rows...).St(ui.St().GapOf(1))).
			K("api-doc-list").St(ui.St().GrowTo(1)),
		ui.Divider(),
		foot,
	).St(ui.St().W(w).Hp(100).Padding(12).GapOf(8).BgC(ui.ColBgDeep))
}

// apiDocPager 正文底部的上一篇 / 下一篇。
func apiDocPager(prev, next *apiDoc) *ui.Node {
	var btns []*ui.Node
	if prev != nil {
		p := prev
		btns = append(btns, ui.Btn("‹ "+displayAPITitle(p.Title), ui.BtnGhost,
			func() { pickAPIDoc(p.File) }))
	} else {
		btns = append(btns, ui.Box())
	}
	btns = append(btns, ui.Spacer())
	if next != nil {
		n := next
		btns = append(btns, ui.Btn(displayAPITitle(n.Title)+" ›", ui.BtnGhost,
			func() { pickAPIDoc(n.File) }))
	}
	return ui.Row(btns...).St(ui.St().GapOf(10).PadOnly(26, 0, 0, 0)).AlignTo(ui.AlignCenter)
}

// apiDocContent 右侧正文：面包屑 + 标题行 + 居中排版的 Markdown。
func apiDocContent(list []apiDoc, listW float32) *ui.Node {
	d := findAPIDoc(list, apiDocSel)
	if d == nil {
		return ui.Center(ui.Text("请选择左侧文档").St(ui.St().Text(13, 400, ui.ColTextMuted)))
	}

	// Markdown 需要显式宽度：0 会按整窗宽度算，右侧就会被目录栏顶出可视区
	mdW := ui.ViewportW() - listW - 96
	if mdW < 320 {
		mdW = 320
	}
	if mdW > 820 {
		mdW = 820
	}

	idx := 0
	for i := range list {
		if list[i].File == d.File {
			idx = i
			break
		}
	}
	var prev, next *apiDoc
	if idx > 0 {
		prev = &list[idx-1]
	}
	if idx+1 < len(list) {
		next = &list[idx+1]
	}

	head := ui.Column(
		ui.Row(
			ui.Box().St(ui.St().W(3).H(11).R(2).BgC(ui.ColPrimary)),
			ui.Text("API 手册").St(ui.St().Text(10.5, 500, ui.ColTextMuted)),
			ui.Text("›").St(ui.St().Text(10.5, 400, ui.ColTextMuted)),
			ui.Text(d.Group).St(ui.St().Text(10.5, 600, ui.ColPrimary)),
		).St(ui.St().GapOf(7)).AlignTo(ui.AlignCenter),
		ui.Row(
			ui.Text(d.Title).St(ui.St().Text(20, 700, ui.ColText)),
			ui.Spacer(),
			ui.Text(fmt.Sprintf("≈%s 字", apiDocSize(d.Body))).
				St(ui.St().Text(10.5, 400, ui.ColTextMuted)),
		).St(ui.St().GapOf(12)).AlignTo(ui.AlignCenter),
	).St(ui.St().GapOf(6))

	// 在可视宽度内水平居中：显式算左偏移，不依赖 Scroll 内百分比宽度
	avail := ui.ViewportW() - listW - 37 // 正文区可用宽（两侧 18 padding + 1px 分隔线）
	left := (avail - mdW) / 2
	if left < 0 {
		left = 0
	}

	body := ui.Scroll(
		ui.Column(
			ui.Markdown("api-"+d.File, d.Body, ui.MarkdownOptions{
				Width:        mdW,
				LineHeight:   13.5 * 1.75, // 正文更松，长文更耐读
				CodeMaxLines: 18,
			}),
			apiDocPager(prev, next),
		).St(ui.St().W(mdW).MarOnly(2, 0, 28, left)),
	).K("api-doc-scroll").St(ui.St().GrowTo(1))

	return ui.Column(head, ui.Divider(), body).
		St(ui.St().GrowTo(1).Hp(100).Padding(18).GapOf(12))
}
