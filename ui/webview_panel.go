// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 浏览器组件的多端 UI 组合件：自适应工具条、页面内查找栏、标签页、一体化面板。
//
// 这些构建器与平台无关（只用框架自绘控件），因此 Windows / 浏览器 / 移动端
// 表现一致；网页区域的真实像素由各端后端提供（桌面是子窗口、浏览器是 iframe、
// 其余平台是占位卡片）。

// webViewFindOpen 查找栏展开状态（按工具条 key 区分，UI 线程单线程访问）。
var webViewFindOpen = map[string]bool{}

// WebViewFindOpen 查找栏当前是否展开（key 与工具条地址栏 key 一致）。
func WebViewFindOpen(key string) bool { return webViewFindOpen[key] }

// WebViewSetFindOpen 设置查找栏展开状态。
func WebViewSetFindOpen(key string, open bool) {
	if key == "" {
		key = "wv-addr"
	}
	webViewFindOpen[key] = open
	RequestRedraw()
}

// WebViewToggleFind 切换查找栏展开状态。
func WebViewToggleFind(key string) { WebViewSetFindOpen(key, !WebViewFindOpen(key)) }

// webViewBarAuto 自适应工具条：按选项决定宽度、按钮尺寸与是否显示文字按钮。
//
// 与固定布局的 WebViewBar 相比：
//   - Compact 时按钮收成字形（34px 触摸目标）、隐藏「打开」文字按钮与标题；
//   - ShowZoom 时出现 − / 百分比 / + 缩放控件；
//   - ShowFind 时出现页面内查找入口（配合 WebViewFindBar）。
func webViewBarAuto(wv *WebView, o WebViewBarOptions) *Node {
	key := o.Key
	if key == "" {
		key = "wv-addr"
	}
	canBack, canFwd, loading := false, false, false
	if wv != nil {
		canBack, canFwd, loading = wv.CanGoBack(), wv.CanGoForward(), wv.Loading()
	}

	side := float32(34)
	gap := float32(8)
	if o.AutoCompact {
		side = 30
		gap = 6
	}

	navBtn := func(glyph string, enabled bool, fn func()) *Node {
		st := St().W(side).H(side).MinWidth(side).Text(13.5, 600, ColTextDim).R(9).
			BgC(ColBgSoft).Stroke(1, ColStroke)
		if !enabled {
			st = St().W(side).H(side).MinWidth(side).Text(13.5, 600, "#3a4358").R(9).
				BgC(ColInput).Stroke(1, ColStroke)
		}
		n := Button(glyph, fn).St(st)
		if !enabled {
			n.Disabled = true
		}
		return n
	}

	left := navBtn("‹", canBack, func() {
		if wv != nil {
			wv.Back()
		}
	})
	right := navBtn("›", canFwd, func() {
		if wv != nil {
			wv.Forward()
		}
	})
	reloadGlyph := "⟳"
	if loading {
		reloadGlyph = "×"
	}
	reload := navBtn(reloadGlyph, wv != nil, func() {
		if wv == nil {
			return
		}
		if wv.Loading() {
			wv.Stop()
		} else {
			wv.Reload()
		}
	})

	cells := []*Node{left, right, reload}
	if o.ShowFind {
		cells = append(cells, navBtn("⌕", true, func() { WebViewToggleFind(key) }))
	}

	addr := Box(InputSubmit(key, o.Address, "输入网址，回车打开", func(v string) {
		if o.OnAddress != nil {
			o.OnAddress(v)
		}
	}, func() {
		if o.OnGo != nil {
			o.OnGo()
		}
	}).St(St().H(32).R(9))).Grow(1)
	cells = append(cells, addr)

	if o.ShowZoom && wv != nil {
		cells = append(cells, navBtn("−", wv.ZoomFactor() > 0.26, func() { wv.ZoomOut() }))
		cells = append(cells, Text(percentText(wv.ZoomFactor())).St(St().Text(11.5, 600, ColTextMuted).MinWidth(38)))
		cells = append(cells, navBtn("+", wv.ZoomFactor() < 4.9, func() { wv.ZoomIn() }))
	}

	if loading {
		cells = append(cells, Spinner(16, ColPrimary))
	}
	if !o.AutoCompact && o.ShowTitle && wv != nil && wv.Title() != "" {
		cells = append(cells, Text(ellipsize(wv.Title(), 34)).St(St().Text(11.5, 500, ColTextMuted)))
	}
	if !o.AutoCompact {
		cells = append(cells, Btn("打开", BtnPrimary, func() {
			if o.OnGo != nil {
				o.OnGo()
			}
		}))
	}

	pad := float32(8)
	if o.AutoCompact {
		pad = 6
	}
	return Row(cells...).St(St().GapOf(gap).Padding(pad).R(12).
		BgC(ColBgSoft).Stroke(1, ColStroke)).AlignTo(AlignCenter)
}

// WebViewFindBarOptions 页面内查找栏参数。
type WebViewFindBarOptions struct {
	// Text 当前关键字（由调用方持有）。
	Text string
	// Key 输入框 key（同一界面内唯一）。
	Key string
	// Placeholder 占位提示，空 = "页面内查找"。
	Placeholder string
	// OnText 关键字变化（同时会调用 wv.Find）。
	OnText func(string)
	// OnClose 关闭查找栏（空 = 仅收起）。
	OnClose func()
}

// WebViewFindBar 页面内查找栏：输入框 + 上一个 / 下一个 / 关闭。
func WebViewFindBar(wv *WebView, o WebViewFindBarOptions) *Node {
	key := o.Key
	if key == "" {
		key = "wv-find"
	}
	ph := o.Placeholder
	if ph == "" {
		ph = "页面内查找"
	}
	box := Box(InputSubmit(key, o.Text, ph, func(v string) {
		if o.OnText != nil {
			o.OnText(v)
		}
		if wv != nil {
			wv.Find(v)
		}
	}, func() {
		if wv != nil {
			wv.FindNext()
		}
	}).St(St().H(30).R(8))).Grow(1)

	btn := func(glyph string, fn func()) *Node {
		return Button(glyph, fn).St(St().W(30).H(30).MinWidth(30).Text(13, 600, ColTextDim).R(8).
			BgC(ColBgSoft).Stroke(1, ColStroke))
	}

	cells := []*Node{
		box,
		btn("↑", func() {
			if wv != nil {
				wv.FindPrev()
			}
		}),
		btn("↓", func() {
			if wv != nil {
				wv.FindNext()
			}
		}),
		btn("×", func() {
			if wv != nil {
				wv.ClearFind()
			}
			if o.OnClose != nil {
				o.OnClose()
			}
		}),
	}
	return Row(cells...).St(St().GapOf(6).Padding(6).R(10).
		BgC(ColInput).Stroke(1, ColStroke)).AlignTo(AlignCenter)
}

// WebViewTabsOptions 标签页参数（标签标题由调用方给，便于自行决定命名策略）。
type WebViewTabsOptions struct {
	// Labels 标签标题（通常取 wv.Title()，空则用地址）。
	Labels []string
	// Active 当前激活下标。
	Active int
	// OnSelect 点击标签。
	OnSelect func(i int)
	// OnClose 关闭标签（nil = 不显示关闭按钮）。
	OnClose func(i int)
	// OnAdd 新建标签（nil = 不显示 + 按钮）。
	OnAdd func()
	// Compact 紧凑模式（更小的按钮与间距，适合移动端）。
	Compact bool
}

// WebViewTabs 自绘标签栏：切换 / 关闭 / 新建，跨端表现一致。
func WebViewTabs(o WebViewTabsOptions) *Node {
	if len(o.Labels) == 0 && o.OnAdd == nil {
		return nil
	}
	h := float32(30)
	gap := float32(6)
	pad := float32(6)
	if o.Compact {
		h = 26
		gap = 4
		pad = 4
	}
	cells := make([]*Node, 0, len(o.Labels)*2+1)
	for i, label := range o.Labels {
		i, label := i, label
		if label == "" {
			label = "新标签页"
		}
		kind := BtnSecondary
		if i == o.Active {
			kind = BtnPrimary
		}
		tab := Btn(ellipsize(label, 18), kind, func() {
			if o.OnSelect != nil {
				o.OnSelect(i)
			}
		})
		tab.St(St().H(h).MinWidth(72))
		cells = append(cells, tab)
		if i == o.Active && o.OnClose != nil {
			cells = append(cells, Button("×", func() {
				o.OnClose(i)
			}).St(St().W(26).H(26).MinWidth(26).Text(12.5, 600, ColTextDim).R(7).
				BgC(ColInput).Stroke(1, ColStroke)))
		}
	}
	if o.OnAdd != nil {
		cells = append(cells, Btn("+", BtnSecondary, o.OnAdd).St(St().H(h).MinWidth(34)))
	}
	return Row(cells...).St(St().GapOf(gap).Padding(pad).R(10).
		BgC(ColBgSoft).Stroke(1, ColStroke)).AlignTo(AlignCenter)
}

// BrowserPanelOptions 一体化浏览器面板参数。
type BrowserPanelOptions struct {
	// WebViewBarOptions 工具条参数（地址栏 / 回调 / 自适应开关）。
	WebViewBarOptions
	// ShowFindBar 强制展开查找栏（true 时常驻；false 时跟随工具条的 ⌕ 按钮）。
	ShowFindBar bool
	// FindText 查找关键字（由调用方持有）。
	FindText string
	// OnFindText 查找关键字变化。
	OnFindText func(string)
	// ShowStatus 是否显示状态行（地址 / 标题 / 加载提示）。
	ShowStatus bool
}

// BrowserPanel 工具条 + 查找栏 + 状态行 + 网页区域的一体化封装。
//
// 用法：
//
//	wv, _ := ui.NewWebView(ui.WebViewOptions{URL: "https://go.dev"})
//	ui.BrowserPanel(wv, ui.BrowserPanelOptions{
//	    WebViewBarOptions: ui.WebViewBarOptions{Address: addr, OnAddress: setAddr, OnGo: ...},
//	    ShowStatus:        true,
//	}).St(ui.St().Hp(100))
func BrowserPanel(wv *WebView, o BrowserPanelOptions) *Node {
	key := o.Key
	if key == "" {
		key = "wv-addr"
	}
	rows := make([]*Node, 0, 5)
	rows = append(rows, WebViewBar(wv, o.WebViewBarOptions))

	if o.ShowFindBar || WebViewFindOpen(key) {
		rows = append(rows, WebViewFindBar(wv, WebViewFindBarOptions{
			Text:   o.FindText,
			Key:    key + "-find",
			OnText: o.OnFindText,
			OnClose: func() {
				WebViewSetFindOpen(key, false)
			},
		}))
	}
	if tip := webViewFallbackTip(wv); tip != nil {
		rows = append(rows, tip)
	}
	if o.ShowStatus {
		if st := WebViewStatus(wv); st != nil {
			rows = append(rows, st)
		}
	}
	if wv != nil {
		rows = append(rows, WebViewNode(wv).St(St().GrowTo(1).R(10)))
	}
	return Column(rows...).St(St().Hp(100).GapOf(6))
}

// webViewFallbackTip 降级提示行：没有内嵌浏览器（或内核失败）时给一条出路。
func webViewFallbackTip(wv *WebView) *Node {
	if wv == nil {
		return nil
	}
	err := wv.Err()
	if err == nil && wv.Ready() {
		return nil
	}
	label := "当前平台未接入内嵌浏览器"
	if err != nil && !WebViewAvailable() {
		label = "当前平台未接入内嵌浏览器（网页请用系统浏览器打开）"
	} else if err != nil {
		label = "浏览器内核不可用：" + ellipsize(err.Error(), 60)
	}
	return Row(
		Text(label).St(St().Text(11.5, 500, ColTextMuted).GrowTo(1)),
		Btn("用系统浏览器打开", BtnSecondary, func() { wv.OpenExternal() }),
	).St(St().GapOf(8).Padding(8).R(10).BgC(ColInput).Stroke(1, ColStroke)).AlignTo(AlignCenter)
}

// percentText 把缩放比格式化成 "125%"。
func percentText(z float64) string {
	if z <= 0 {
		z = 1
	}
	n := int(z*100 + 0.5)
	return itoaSmall(n) + "%"
}

// itoaSmall 小整数转字符串（避免为一次格式化引入 strconv 依赖链）。
func itoaSmall(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
