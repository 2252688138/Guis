// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package uidsl

// 扩展组件：card / badge / avatar / chip / spinner / skeleton / markdown /
// richtext / masonry / arrow / segmented / select / chart / codeeditor / alert /
// confirm / drawer。描述文件与 Lua 脚本共用同一套类型名与字段。

import (
	"strings"

	"guis/ui"
)

// 需要跨帧复用的组件状态（下拉框 / 代码编辑器），按节点 key 缓存。
var (
	extraSelects  = map[string]*ui.SelectState{}
	extraCodeSt   = map[string]*ui.CodeEditorState{}
	extraCodeSrc  = map[string]string{}
)

// ResetExtraStates 清掉扩展组件的缓存状态（脚本热重载 / 宿主重建界面时调用）。
func ResetExtraStates() {
	extraSelects = map[string]*ui.SelectState{}
	extraCodeSt = map[string]*ui.CodeEditorState{}
	extraCodeSrc = map[string]string{}
	resetStage3()
	resetCanvases()
}

// nodeKeyOf 取节点 key，没写就用描述路径生成稳定 key。
func nodeKeyOf(d Node, path string) string {
	if k := strings.TrimSpace(d.Key); k != "" {
		return k
	}
	return "uidsl-" + path
}

// selectStateFor 下拉框状态：同 key 复用，否则新建并登记。
func selectStateFor(d Node, path string) *ui.SelectState {
	key := nodeKeyOf(d, path)
	if s, ok := extraSelects[key]; ok {
		return s
	}
	s := ui.NewSelect(key)
	extraSelects[key] = s
	return s
}

// codeStateFor 代码编辑器状态：内容变了才重建，避免每帧重置光标与选区。
func codeStateFor(d Node, path string) *ui.CodeEditorState {
	key := nodeKeyOf(d, path)
	src := d.Src
	if src == "" {
		src = d.Text
	}
	st, ok := extraCodeSt[key]
	if !ok || extraCodeSrc[key] != src {
		st = ui.NewCodeEditorState(key, src)
		extraCodeSt[key] = st
		extraCodeSrc[key] = src
	}
	st.ReadOnly = d.ReadOnly
	return st
}

// toOptions 把描述选项转成 ui 选框选项。
func toOptions(items []OptionDef) []ui.SelectOption {
	out := make([]ui.SelectOption, 0, len(items))
	for _, it := range items {
		out = append(out, ui.SelectOption{
			Value:    it.Value,
			Label:    it.Label,
			Hint:     it.Hint,
			Disabled: it.Disabled,
		})
	}
	return out
}

// optionKeys 给 segmented 生成 keys / labels：优先 items，退化到 labels。
func optionKeys(items []OptionDef, labels []string) ([]string, []string) {
	if len(items) == 0 {
		keys := make([]string, len(labels))
		copy(keys, labels)
		return keys, labels
	}
	keys := make([]string, 0, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		v := it.Value
		if v == "" {
			v = it.Label
		}
		keys = append(keys, v)
		out = append(out, it.Label)
	}
	return keys, out
}

// toSpans 富文本片段。
func toSpans(spans []SpanDef) []ui.TextSpan {
	out := make([]ui.TextSpan, 0, len(spans))
	for _, s := range spans {
		out = append(out, ui.TextSpan{
			Text:      s.Text,
			Color:     s.Color,
			BgColor:   s.BgColor,
			Size:      s.Size,
			Weight:    s.Weight,
			Link:      s.Link,
			Underline: s.Underline,
			Strike:    s.Strike,
			Code:      s.Code,
		})
	}
	return out
}

// toSeries 图表数据系列。
func toSeries(defs []SeriesDef) []ui.Series {
	out := make([]ui.Series, 0, len(defs))
	for _, d := range defs {
		out = append(out, ui.Series{Name: d.Name, Color: d.Color, Values: d.Values})
	}
	return out
}

// chartTypeOf 字符串 → 图表类型（未知按折线）。
func chartTypeOf(s string) ui.ChartType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "area":
		return ui.ChartArea
	case "bar", "column":
		return ui.ChartBar
	case "pie":
		return ui.ChartPie
	case "donut", "ring":
		return ui.ChartDonut
	case "radar":
		return ui.ChartRadar
	default:
		return ui.ChartLine
	}
}

// dlgKindOf 字符串 → 弹窗语义（未知按 info）。
func dlgKindOf(s string) ui.DialogKind {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "warn", "warning":
		return ui.DlgWarn
	case "error", "err":
		return ui.DlgError
	default:
		return ui.DlgInfo
	}
}

// markdownSource markdown 内容：优先 src，其次 text。
func markdownSource(d Node) string {
	if d.Src != "" {
		return d.Src
	}
	return d.Text
}

// clickOf / submitOf / changeOf 把事件名转成对应签名回调（未注册返回 nil）。
func clickOf(name string, h Handlers) func() {
	if name == "" || h.Click == nil {
		return nil
	}
	return h.Click[name]
}

func submitOf(name string, h Handlers) func(string) {
	if name == "" || h.Submit == nil {
		return nil
	}
	return h.Submit[name]
}
