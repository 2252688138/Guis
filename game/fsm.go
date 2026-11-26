// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "fmt"

// State 有限状态机中的一个状态（回调均可为 nil）。
type State struct {
	Name     string
	OnEnter  func(prev string)
	OnUpdate func(dt float32)
	OnExit   func(next string)
}

// Transition 状态转换规则。
//
// From 为 "*" 表示任意状态；Event 为空时是条件型转换（每帧检查 When）。
type Transition struct {
	From  string
	To    string
	Event string
	When  func() bool
}

// StateMachine 轻量有限状态机（用于角色行为、UI 流程、关卡阶段）。
//
// 用法：
//
//	fsm := game.NewStateMachine()
//	fsm.Add("idle", nil, func(dt float32) { /* 待机 */ }, nil)
//	fsm.Add("run", nil, nil, nil)
//	fsm.AddTransition("idle", "run", func() bool { return input.Down("right") })
//	fsm.Set("idle")
//	// 每帧：fsm.Update(dt)
//
// 注意：转换按注册顺序检查，第一个满足条件的生效。
type StateMachine struct {
	states     map[string]*State
	order      []string
	trans      []*Transition
	cur, prev  string
	blackboard map[string]float64
}

// NewStateMachine 创建空状态机。
func NewStateMachine() *StateMachine {
	return &StateMachine{states: map[string]*State{}, blackboard: map[string]float64{}}
}

// Add 注册一个状态（重复注册同名状态会覆盖回调，但保留注册顺序）。
func (m *StateMachine) Add(name string, onEnter func(prev string), onUpdate func(dt float32), onExit func(next string)) *StateMachine {
	if m == nil || name == "" {
		return m
	}
	if _, ok := m.states[name]; !ok {
		m.order = append(m.order, name)
	}
	m.states[name] = &State{Name: name, OnEnter: onEnter, OnUpdate: onUpdate, OnExit: onExit}
	return m
}

// AddTransition 注册条件型转换（每帧 Update 时检查，When 为 nil 时视为总是成立）。
func (m *StateMachine) AddTransition(from, to string, when func() bool) *StateMachine {
	if m == nil {
		return m
	}
	m.trans = append(m.trans, &Transition{From: from, To: to, When: when})
	return m
}

// On 注册事件型转换（Trigger(event) 或 TriggerFrom(event, from) 触发）。
func (m *StateMachine) On(event, from, to string) *StateMachine {
	if m == nil || event == "" {
		return m
	}
	m.trans = append(m.trans, &Transition{From: from, To: to, Event: event})
	return m
}

// States 已注册状态名（按注册顺序）。
func (m *StateMachine) States() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.order...)
}

// Current 当前状态名（尚未设置时为空）。
func (m *StateMachine) Current() string {
	if m == nil {
		return ""
	}
	return m.cur
}

// Previous 上一个状态名。
func (m *StateMachine) Previous() string {
	if m == nil {
		return ""
	}
	return m.prev
}

// Set 直接切换到指定状态（不走转换表）。
func (m *StateMachine) Set(name string) error {
	if m == nil {
		return fmt.Errorf("game: 状态机为空")
	}
	s, ok := m.states[name]
	if !ok {
		return fmt.Errorf("game: 未注册的状态 %q", name)
	}
	from := m.cur
	if from == name {
		return nil
	}
	if old := m.states[from]; old != nil && old.OnExit != nil {
		old.OnExit(name)
	}
	m.prev = from
	m.cur = name
	if s.OnEnter != nil {
		s.OnEnter(from)
	}
	return nil
}

// Trigger 触发事件：按注册顺序执行第一个匹配的转换。返回是否发生了转换。
func (m *StateMachine) Trigger(event string) bool {
	if m == nil || event == "" {
		return false
	}
	for _, t := range m.trans {
		if t.Event != event || !m.matchFrom(t.From) {
			continue
		}
		if _, ok := m.states[t.To]; !ok {
			continue
		}
		if m.Set(t.To) == nil {
			return true
		}
	}
	return false
}

// Update 每帧调用：先处理条件型转换（第一个成立的生效），再执行当前状态 OnUpdate。
func (m *StateMachine) Update(dt float32) {
	if m == nil {
		return
	}
	for _, t := range m.trans {
		if t.Event != "" || !m.matchFrom(t.From) {
			continue
		}
		if t.When != nil && !t.When() {
			continue
		}
		if _, ok := m.states[t.To]; !ok {
			continue
		}
		if m.Set(t.To) == nil {
			break
		}
	}
	if s := m.states[m.cur]; s != nil && s.OnUpdate != nil {
		s.OnUpdate(dt)
	}
}

func (m *StateMachine) matchFrom(from string) bool {
	return from == "*" || from == m.cur
}

// SetFloat / Float 黑板数值。
func (m *StateMachine) SetFloat(k string, v float64) {
	if m == nil {
		return
	}
	m.blackboard[k] = v
}

// Float 读黑板数值（不存在返回 0）。
func (m *StateMachine) Float(k string) float64 {
	if m == nil {
		return 0
	}
	return m.blackboard[k]
}

// SetBool / Bool 黑板布尔值。
func (m *StateMachine) SetBool(k string, v bool) {
	if v {
		m.SetFloat(k, 1)
		return
	}
	m.SetFloat(k, 0)
}

// Bool 读黑板布尔值。
func (m *StateMachine) Bool(k string) bool { return m.Float(k) != 0 }

// Reset 清空当前状态与黑板（保留状态与转换定义）。
func (m *StateMachine) Reset() {
	if m == nil {
		return
	}
	m.cur, m.prev = "", ""
	m.blackboard = map[string]float64{}
}
