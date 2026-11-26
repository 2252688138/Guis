// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"strconv"
	"strings"
)

// 穿梭框：左右两个候选列表 + 中间转移按钮。
// 状态（右侧集合 / 两侧勾选 / 两侧过滤词）由 TransferState 持有，调用方长期保存。

// TransferItem 穿梭框条目。
type TransferItem struct {
	Key      string
	Label    string
	Hint     string // 右侧附加说明（可选）
	Disabled bool
}

// TransferOptions 穿梭框选项。
type TransferOptions struct {
	Key               string
	TitleLeft         string // 默认「待选」
	TitleRight        string // 默认「已选」
	Height            float32 // 面板高度，默认 260
	PanelWidth        float32 // 单侧宽度，默认 200
	Filterable        bool
	FilterPlaceholder string // 默认「搜索」
	EmptyLeft         string // 默认「无匹配项」
	EmptyRight        string // 默认「尚未选择」
	OnChange          func(left, right []string)
}

// TransferState 穿梭框状态（用 NewTransfer 创建）。
type TransferState struct {
	Key      string
	Right    map[string]bool // 已在右侧的条目
	SelLeft  map[string]bool
	SelRight map[string]bool
	FilterL  string
	FilterR  string

	onChange func(left, right []string)
}

// transferStates 已创建的穿梭框状态（TransferOf 按 Key 取回）。
var transferStates = map[string]*TransferState{}

// NewTransfer 创建穿梭框状态（同一个 key 重复创建会覆盖注册项）。
func NewTransfer(key string) *TransferState {
	s := &TransferState{
		Key:      key,
		Right:    map[string]bool{},
		SelLeft:  map[string]bool{},
		SelRight: map[string]bool{},
	}
	transferStates[key] = s
	return s
}

// TransferOf 取回（必要时创建）指定 Key 的穿梭框状态。
func TransferOf(key string) *TransferState {
	if s := transferStates[key]; s != nil {
		return s
	}
	return NewTransfer(key)
}

// MoveRight 把左侧勾选项移到右侧。
func (s *TransferState) MoveRight() {
	if s == nil {
		return
	}
	for k, on := range s.SelLeft {
		if on {
			s.Right[k] = true
			delete(s.SelLeft, k)
		}
	}
	s.fire()
}

// MoveLeft 把右侧勾选项移回左侧。
func (s *TransferState) MoveLeft() {
	if s == nil {
		return
	}
	for k, on := range s.SelRight {
		if on {
			delete(s.Right, k)
			delete(s.SelRight, k)
		}
	}
	s.fire()
}

// RightKeys 按 items 顺序返回右侧条目键（结果稳定，便于直接提交）。
func (s *TransferState) RightKeys(items []TransferItem) []string {
	out := make([]string, 0, len(s.Right))
	if s == nil {
		return out
	}
	for _, it := range items {
		if s.Right[it.Key] {
			out = append(out, it.Key)
		}
	}
	return out
}

// RightLabels 按 items 顺序返回右侧条目文案。
func (s *TransferState) RightLabels(items []TransferItem) []string {
	out := make([]string, 0, len(s.Right))
	if s == nil {
		return out
	}
	for _, it := range items {
		if s.Right[it.Key] {
			out = append(out, it.Label)
		}
	}
	return out
}

// Reset 清空两侧（右侧集合与全部勾选）。
func (s *TransferState) Reset() {
	if s == nil {
		return
	}
	s.Right = map[string]bool{}
	s.SelLeft = map[string]bool{}
	s.SelRight = map[string]bool{}
	s.fire()
}

func (s *TransferState) fire() {
	if s != nil && s.onChange != nil {
		s.onChange(nil, nil)
	}
}

// Transfer 穿梭框。
func Transfer(s *TransferState, items []TransferItem, opts TransferOptions) *Node {
	if s == nil {
		return nil
	}
	if opts.Height <= 0 {
		opts.Height = 260
	}
	if opts.PanelWidth <= 0 {
		opts.PanelWidth = 200
	}
	if opts.TitleLeft == "" {
		opts.TitleLeft = "待选"
	}
	if opts.TitleRight == "" {
		opts.TitleRight = "已选"
	}
	if opts.FilterPlaceholder == "" {
		opts.FilterPlaceholder = "搜索"
	}
	if opts.EmptyLeft == "" {
		opts.EmptyLeft = "无匹配项"
	}
	if opts.EmptyRight == "" {
		opts.EmptyRight = "尚未选择"
	}
	s.onChange = opts.OnChange

	left := transferPanel(s, items, false, &opts)
	right := transferPanel(s, items, true, &opts)
	mid := Column(
		transferArrow("arrow-right", s.anySel(s.SelLeft), func() { s.MoveRight(); RequestRedraw() }),
		transferArrow("arrow-left", s.anySel(s.SelRight), func() { s.MoveLeft(); RequestRedraw() }),
	).St(St().GapOf(8)).AlignTo(AlignCenter)

	return Row(left, mid, right).St(St().GapOf(10)).AlignTo(AlignCenter).K(opts.Key)
}

func (s *TransferState) anySel(m map[string]bool) bool {
	for _, on := range m {
		if on {
			return true
		}
	}
	return false
}

func transferArrow(icon string, enabled bool, fn func()) *Node {
	c := any(ColTextMuted)
	if enabled {
		c = ColText
	}
	n := Center(IconColored(icon, 16, c)).
		St(St().W(32).H(28).R(8).BgC(ColBgSoft).Stroke(1, ColStroke))
	if !enabled {
		return n.DisabledOn(true)
	}
	return n.Tap(fn)
}

// transferPanel 渲染一侧面板：标题 + 计数 +（可选）过滤框 + 勾选列表。
func transferPanel(s *TransferState, items []TransferItem, right bool, opts *TransferOptions) *Node {
	title, empty := opts.TitleLeft, opts.EmptyLeft
	sel, filter := s.SelLeft, s.FilterL
	if right {
		title, empty = opts.TitleRight, opts.EmptyRight
		sel, filter = s.SelRight, s.FilterR
	}

	rows := make([]*Node, 0, len(items))
	count := 0
	for _, it := range items {
		if s.Right[it.Key] != right {
			continue
		}
		count++
		if filter != "" && !strings.Contains(strings.ToLower(it.Label), strings.ToLower(filter)) &&
			!strings.Contains(strings.ToLower(it.Hint), strings.ToLower(filter)) {
			continue
		}
		rows = append(rows, transferRow(s, it, right, sel))
	}

	head := Row(
		Text(title).St(St().Text(12, 600, ColText)),
		Spacer(),
		Text(strconv.Itoa(count)+" 项").St(St().Text(11.5, 400, ColTextMuted)),
	).St(St().PadXY(10, 8))

	parts := []*Node{head, Divider()}
	bodyH := opts.Height - 34
	if opts.Filterable {
		fk := opts.Key + ":flt"
		val := s.FilterL
		if right {
			fk, val = opts.Key+":fltR", s.FilterR
		}
		parts = append(parts, Row(
			Input(fk, val, opts.FilterPlaceholder, func(v string) {
				if right {
					s.FilterR = v
				} else {
					s.FilterL = v
				}
				RequestRedraw()
			}).St(St().W(opts.PanelWidth - 20)),
		).St(St().PadXY(10, 6)))
		bodyH -= 34
	}
	if len(rows) == 0 {
		parts = append(parts, Row(Text(empty).St(St().Text(12, 400, ColTextMuted))).
			St(St().PadXY(10, 14)))
	} else {
		parts = append(parts, Scroll(rows...).K(opts.Key+":list"+transferSide(right)).
			St(St().H(bodyH)))
	}
	return Column(parts...).St(St().W(opts.PanelWidth).H(opts.Height).R(10).
		BgC(ColInput).Stroke(1, ColStroke).Clipped())
}

func transferSide(right bool) string {
	if right {
		return "R"
	}
	return "L"
}

func transferRow(s *TransferState, it TransferItem, right bool, sel map[string]bool) *Node {
	ck := s.Key + ":ck" + transferSide(right) + ":" + it.Key
	n := Checkbox(ck, it.Label, sel[it.Key], func(v bool) {
		sel[it.Key] = v
		RequestRedraw()
	}).St(St().PadXY(10, 5))
	if it.Hint != "" {
		n = Row(n, Spacer(), Text(it.Hint).St(St().Text(11, 400, ColTextMuted)))
	}
	if it.Disabled {
		return n.DisabledOn(true)
	}
	return n
}
