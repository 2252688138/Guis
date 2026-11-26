// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"sort"
	"strings"
)

// ActionBinding 动作绑定（键盘 + 手柄）。
type ActionBinding struct {
	Keys    []string
	Buttons []PadButton
}

// ActionMap 动作映射：把物理输入变成语义动作（"jump" / "fire"）。
//
// 键位支持组合写法 "ctrl+s"（+ 连接，除最后一项外其余要求按住，最后一项判断 Pressed）。
type ActionMap struct {
	bindings map[string]*ActionBinding
	// Pad 可选：非 nil 时动作也会查询手柄。
	Pad *Gamepad
}

// NewActions 创建空映射。
func NewActions() *ActionMap {
	return &ActionMap{bindings: map[string]*ActionBinding{}}
}

// Bind 绑定按键（可多次调用追加）。
func (m *ActionMap) Bind(action string, keys ...string) *ActionBinding {
	if m == nil {
		return nil
	}
	b := m.bindings[action]
	if b == nil {
		b = &ActionBinding{}
		m.bindings[action] = b
	}
	for _, k := range keys {
		if k == "" {
			continue
		}
		b.Keys = append(b.Keys, strings.ToLower(k))
	}
	return b
}

// BindPad 绑定手柄按钮。
func (m *ActionMap) BindPad(action string, btns ...PadButton) *ActionBinding {
	if m == nil {
		return nil
	}
	b := m.bindings[action]
	if b == nil {
		b = &ActionBinding{}
		m.bindings[action] = b
	}
	b.Buttons = append(b.Buttons, btns...)
	return b
}

// Unbind 解除全部绑定。
func (m *ActionMap) Unbind(action string) {
	if m == nil {
		return
	}
	delete(m.bindings, action)
}

// Actions 已绑定的动作名（排序）。
func (m *ActionMap) Actions() []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.bindings))
	for k := range m.bindings {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Keys 某动作绑定的键位（副本）。
func (m *ActionMap) Keys(action string) []string {
	if m == nil || m.bindings[action] == nil {
		return nil
	}
	return append([]string(nil), m.bindings[action].Keys...)
}

// Combo 把 "ctrl+s" 拆成 ["ctrl","s"]，普通键返回单元素。
func Combo(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(strings.ToLower(s), "+")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// comboDown 组合键是否全部按住。
func comboDown(in *Input, combo string) bool {
	parts := Combo(combo)
	if len(parts) == 0 {
		return false
	}
	for _, p := range parts {
		if !in.KeyDown(p) {
			return false
		}
	}
	return true
}

// comboPressed 组合键是否本帧触发（最后一项刚按下，其余按住）。
func comboPressed(in *Input, combo string) bool {
	parts := Combo(combo)
	if len(parts) == 0 {
		return false
	}
	for i, p := range parts {
		if i == len(parts)-1 {
			if !in.KeyPressed(p) {
				return false
			}
			continue
		}
		if !in.KeyDown(p) {
			return false
		}
	}
	return true
}

// Down 动作是否按住。
func (m *ActionMap) Down(in *Input, action string) bool {
	if m == nil || in == nil {
		return false
	}
	b := m.bindings[action]
	if b == nil {
		return false
	}
	for _, k := range b.Keys {
		if comboDown(in, k) {
			return true
		}
	}
	if m.Pad != nil && m.Pad.Connected {
		for _, btn := range b.Buttons {
			if m.Pad.Button(btn) {
				return true
			}
		}
	}
	return false
}

// Pressed 动作本帧是否刚触发。
func (m *ActionMap) Pressed(in *Input, action string) bool {
	if m == nil || in == nil {
		return false
	}
	b := m.bindings[action]
	if b == nil {
		return false
	}
	for _, k := range b.Keys {
		if comboPressed(in, k) {
			return true
		}
	}
	if m.Pad != nil && m.Pad.Connected {
		for _, btn := range b.Buttons {
			if m.Pad.Pressed(btn) {
				return true
			}
		}
	}
	return false
}

// Released 动作本帧是否松开（只看单键绑定）。
func (m *ActionMap) Released(in *Input, action string) bool {
	if m == nil || in == nil {
		return false
	}
	b := m.bindings[action]
	if b == nil {
		return false
	}
	for _, k := range b.Keys {
		if !strings.Contains(k, "+") && in.KeyReleased(k) {
			return true
		}
	}
	return false
}

// Axis 由两个动作组成 +/- 轴（返回 -1/0/1）。
func (m *ActionMap) Axis(in *Input, neg, pos string) float32 {
	var v float32
	if m.Down(in, neg) {
		v -= 1
	}
	if m.Down(in, pos) {
		v += 1
	}
	return v
}

// Conflicts 返回同时被多个动作绑定的键位（"键 → 动作" 形式）。
func (m *ActionMap) Conflicts() []string {
	if m == nil {
		return nil
	}
	seen := map[string][]string{}
	for action, b := range m.bindings {
		for _, k := range b.Keys {
			seen[k] = append(seen[k], action)
		}
	}
	var out []string
	for k, as := range seen {
		if len(as) > 1 {
			sort.Strings(as)
			out = append(out, k+" → "+strings.Join(as, ","))
		}
	}
	sort.Strings(out)
	return out
}
