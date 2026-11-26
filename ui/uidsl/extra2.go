// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package uidsl

// 第三阶段扩展：动画、终端、视频、内嵌浏览器、菜单栏、3D 视口。
// 描述文件与 Lua 脚本共用（字段名即 JSON 标签）。

import (
	"strings"

	"guis/ui"
)

// ---------------- 动画 ----------------

// AnimDef 一条动画声明。
type AnimDef struct {
	Kind  string  `json:"kind,omitempty"`  // fade/slide/scale/shake/pulse/float/bounce/flipx/flipy/rotate/spin/swing/jello/heartbeat/tada/flash/drop/rubber
	Side  string  `json:"side,omitempty"`  // left/right/top/bottom（slide 用）
	Dur   float32 `json:"dur,omitempty"`   // 秒，<=0 按 0.3
	Delay float32 `json:"delay,omitempty"` // 秒
	Dist  float32 `json:"dist,omitempty"`  // 位移距离
	Amp   float32 `json:"amp,omitempty"`   // 幅度
	Scale float32 `json:"scale,omitempty"` // 缩放起点
	Rot   float32 `json:"rot,omitempty"`   // 旋转起始角（弧度）
	Loop  bool    `json:"loop,omitempty"`  // 循环
	Out   bool    `json:"out,omitempty"`   // 出场方向
	Ease  string  `json:"ease,omitempty"`  // linear/in_out_quad/in_out_cubic/out_back
}

// animKindOf 字符串 → 动画类型（未知按 fade）。
func animKindOf(s string) ui.AnimKind {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "slide":
		return ui.AnimSlide
	case "scale", "pop", "zoom":
		return ui.AnimScale
	case "shake":
		return ui.AnimShake
	case "pulse", "breathe":
		return ui.AnimPulse
	case "float":
		return ui.AnimFloat
	case "bounce":
		return ui.AnimBounce
	case "flipx":
		return ui.AnimFlipX
	case "flipy":
		return ui.AnimFlipY
	case "rotate":
		return ui.AnimRotate
	case "spin":
		return ui.AnimSpin
	case "swing":
		return ui.AnimSwing
	case "jello":
		return ui.AnimJello
	case "heartbeat":
		return ui.AnimHeartbeat
	case "tada":
		return ui.AnimTada
	case "flash":
		return ui.AnimFlash
	case "drop":
		return ui.AnimDrop
	case "rubber":
		return ui.AnimRubber
	default:
		return ui.AnimFade
	}
}

// animSideOf 字符串 → 方向（未知按 left）。
func animSideOf(s string) ui.AnimSide {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "right":
		return ui.SideRight
	case "top", "up":
		return ui.SideTop
	case "bottom", "down":
		return ui.SideBottom
	default:
		return ui.SideLeft
	}
}

// easeOf 字符串 → 缓动（空 / 未知返回 nil，交给框架按类型取默认）。
func easeOf(s string) ui.Ease {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "in_out_quad", "inoutquad":
		return ui.EaseInOutQuad
	case "in_out_cubic", "inoutcubic":
		return ui.EaseInOutCubic
	case "out_back", "outback":
		return ui.EaseOutBack
	default:
		return nil
	}
}

// toAnimSpecs 声明 → ui.AnimSpec 列表。
func toAnimSpecs(defs []AnimDef) []ui.AnimSpec {
	out := make([]ui.AnimSpec, 0, len(defs))
	for _, d := range defs {
		out = append(out, ui.AnimSpec{
			Kind:  animKindOf(d.Kind),
			Side:  animSideOf(d.Side),
			Dur:   d.Dur,
			Delay: d.Delay,
			Ease:  easeOf(d.Ease),
			Dist:  d.Dist,
			Amp:   d.Amp,
			Scale: d.Scale,
			Rot:   d.Rot,
			Loop:  d.Loop,
			Out:   d.Out,
		})
	}
	return out
}

// ---------------- 终端 ----------------

var (
	extraTerms  = map[string]*ui.TermState{}
	extraTermTx = map[string]string{}
)

// termStateFor 终端状态：同 key 复用，内容只增量写入，避免每帧刷屏。
func termStateFor(d Node, path string) *ui.TermState {
	key := nodeKeyOf(d, path)
	st, ok := extraTerms[key]
	if !ok {
		st = ui.NewTermState(key)
		extraTerms[key] = st
		extraTermTx[key] = ""
	}
	if d.Size > 0 {
		st.FontSize = d.Size
	}
	src := termSource(d)
	prev := extraTermTx[key]
	if src != prev {
		if strings.HasPrefix(src, prev) {
			st.Feed([]byte(src[len(prev):]))
		} else {
			st = ui.NewTermState(key)
			extraTerms[key] = st
			st.Feed([]byte(src))
		}
		extraTermTx[key] = src
	}
	return st
}

// termSource 终端内容：lines 优先，其次 src / text。
func termSource(d Node) string {
	if len(d.Lines) > 0 {
		var b strings.Builder
		for i, ln := range d.Lines {
			if i > 0 {
				b.WriteString("\r\n")
			}
			b.WriteString(ln)
		}
		return b.String()
	}
	if d.Src != "" {
		return d.Src
	}
	return d.Text
}

// ---------------- 视频 ----------------

var (
	extraPlayers  = map[string]*ui.Player{}
	extraPlayerEr = map[string]string{}
)

// videoNode 视频播放器节点：同 key 复用 Player；打开失败降级为提示文本。
func videoNode(d Node, path string) *ui.Node {
	key := nodeKeyOf(d, path)
	if p, ok := extraPlayers[key]; ok && p != nil {
		return ui.Video(p)
	}
	if msg, ok := extraPlayerEr[key]; ok {
		return ui.Text("[video] " + msg)
	}
	src := d.Src
	if src == "" {
		src = d.Text
	}
	p, err := ui.NewMediaPlayer(src)
	if err != nil || p == nil {
		msg := "打开失败"
		if err != nil {
			msg += ": " + err.Error()
		}
		extraPlayerEr[key] = msg
		noteDegrade("video " + src + " " + msg)
		return ui.Text("[video] " + src + " " + msg)
	}
	extraPlayers[key] = p
	return ui.Video(p)
}

// ---------------- 内嵌浏览器 ----------------

var (
	extraWebviews = map[string]*ui.WebView{}
	extraWebviewE = map[string]string{}
)

// webviewNode 内嵌浏览器节点：同 key 复用；失败降级为提示文本。
func webviewNode(d Node, path string) *ui.Node {
	key := nodeKeyOf(d, path)
	if wv, ok := extraWebviews[key]; ok && wv != nil {
		return ui.WebViewNode(wv)
	}
	if msg, ok := extraWebviewE[key]; ok {
		return ui.Text("[webview] " + msg)
	}
	src := d.Src
	if src == "" {
		src = d.Text
	}
	wv, err := ui.NewWebView(ui.WebViewOptions{
		URL:         src,
		UserDataDir: d.UserDataDir,
		Zoom:        d.Zoom,
		UserAgent:   d.UserAgent,
	})
	if err != nil || wv == nil {
		msg := "打开失败"
		if err != nil {
			msg += ": " + err.Error()
		}
		extraWebviewE[key] = msg
		noteDegrade("webview " + src + " " + msg)
		return ui.Text("[webview] " + src + " " + msg)
	}
	extraWebviews[key] = wv
	return ui.WebViewNode(wv)
}

// ---------------- 菜单栏 ----------------

// MenuItemDef 菜单项声明。
type MenuItemDef struct {
	Label     string        `json:"label,omitempty"`
	Hint      string        `json:"hint,omitempty"`
	Icon      string        `json:"icon,omitempty"`
	OnClick   string        `json:"onClick,omitempty"`
	Separator bool          `json:"separator,omitempty"`
	Disabled  bool          `json:"disabled,omitempty"`
	Checked   bool          `json:"checked,omitempty"`
	Children  []MenuItemDef `json:"children,omitempty"`
}

// MenuDefDef 顶级菜单声明。
type MenuDefDef struct {
	Label string        `json:"label,omitempty"`
	Items []MenuItemDef `json:"items,omitempty"`
}

var extraMenuBars = map[string]*ui.MenuBarState{}

// menuBarStateFor 菜单栏状态：同 key 复用。
func menuBarStateFor(d Node, path string) *ui.MenuBarState {
	key := nodeKeyOf(d, path)
	if st, ok := extraMenuBars[key]; ok {
		return st
	}
	st := ui.NewMenuBar()
	extraMenuBars[key] = st
	return st
}

// toMenuDefs 声明 → ui.MenuDef（onClick 走脚本事件名）。
func toMenuDefs(defs []MenuDefDef, h Handlers) []ui.MenuDef {
	out := make([]ui.MenuDef, 0, len(defs))
	for _, d := range defs {
		out = append(out, ui.MenuDef{Label: d.Label, Items: toMenuItems(d.Items, h)})
	}
	return out
}

// toMenuItems 递归转换菜单项。
func toMenuItems(items []MenuItemDef, h Handlers) []ui.MenuItem {
	out := make([]ui.MenuItem, 0, len(items))
	for _, it := range items {
		out = append(out, ui.MenuItem{
			Label:     it.Label,
			Hint:      it.Hint,
			Icon:      it.Icon,
			OnPick:    clickOf(it.OnClick, h),
			Children:  toMenuItems(it.Children, h),
			Separator: it.Separator,
			Disabled:  it.Disabled,
			Checked:   it.Checked,
		})
	}
	return out
}

// ---------------- 3D 视口 ----------------

// viewportProvider 3D 视口的渲染源提供者（由宿主注册；脚本只声明布局）。
var viewportProvider func(key string) *ui.Viewport3DOpts

// RegisterViewport3DProvider 注册 3D 视口渲染源提供者（宿主调用）。
//
//	uidsl.RegisterViewport3DProvider(func(key string) *ui.Viewport3DOpts {
//	    if key == "main" { return &ui.Viewport3DOpts{Src: engine, Key: key} }
//	    return nil
//	})
func RegisterViewport3DProvider(fn func(key string) *ui.Viewport3DOpts) {
	viewportProvider = fn
}

// viewportNode 3D 视口节点：没注册渲染源就降级为提示文本。
func viewportNode(d Node, path string) *ui.Node {
	key := nodeKeyOf(d, path)
	if viewportProvider != nil {
		if opts := viewportProvider(key); opts != nil {
			if opts.Key == "" {
				opts.Key = key
			}
			return ui.Viewport3D(opts)
		}
	}
	return ui.Text("[3D 视口需要宿主注册渲染源: " + key + "]")
}
// ---------------- 清理 ----------------

// ---------------- 降级记录 ----------------

// 组件降级（video / webview / viewport 打不开）时的提示，由宿主/脚本引擎取走。
var degrades []string

// noteDegrade 记一条降级信息（最多留 32 条，避免刷屏）。
func noteDegrade(msg string) {
	if len(degrades) < 32 {
		degrades = append(degrades, msg)
	}
}

// DrainDegrades 取走并清空降级记录。
func DrainDegrades() []string {
	out := degrades
	degrades = nil
	return out
}

// resetStage3 清掉第三阶段组件缓存（热重载 / 重建界面时调用）。
func resetStage3() {
	extraTerms = map[string]*ui.TermState{}
	extraTermTx = map[string]string{}
	for _, p := range extraPlayers {
		if p != nil {
			p.Close()
		}
	}
	extraPlayers = map[string]*ui.Player{}
	extraPlayerEr = map[string]string{}
	for _, wv := range extraWebviews {
		if wv != nil {
			wv.Close()
		}
	}
	extraWebviews = map[string]*ui.WebView{}
	extraWebviewE = map[string]string{}
	extraMenuBars = map[string]*ui.MenuBarState{}
}
