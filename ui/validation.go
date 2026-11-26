// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// 表单校验：规则（Rule）+ 表单状态（FormState）。
//
// form.go 里原有 ValidateRequired / ValidateMinLen / ValidateEmail 是「单个函数
// 校验一个值」，调用方要自己把错误文案传回 FormItem。这里在其上补两件事：
//
//  1. Rule：可组合的规则值（返回空串 = 通过），能塞进切片批量跑；
//  2. FormState：把「字段值 + 规则 + 错误」收在一起，配 Bind/Field 直接接到输入框，
//     提交时一次校验全部字段，错误文案自动回填到 FormItem。
//
// 用法：
//
//	form := ui.NewFormState()
//	form.Rule("email", ui.Rules(ui.RequiredRule("邮箱"), ui.EmailRule("邮箱")))
//	form.Rule("pwd", ui.Rules(ui.RequiredRule("密码"), ui.MinLenRule(8, "密码")))
//	...
//	col := ui.Column(
//	    form.Field("email", "邮箱", "you@example.com", true),
//	    form.Field("pwd", "密码", "至少 8 位", true),
//	    ui.Button("提交", func() {
//	        form.Submit(func(v map[string]string) { log.Println(v["email"]) })
//	    }),
//	)

// Rule 单字段校验规则：返回空串表示通过，否则返回错误文案。
type Rule func(value string) string

// Rules 把多条规则合成一条，返回**第一条**未通过的错误（顺序即优先级）。
func Rules(rs ...Rule) Rule {
	return func(v string) string {
		for _, r := range rs {
			if r == nil {
				continue
			}
			if msg := r(v); msg != "" {
				return msg
			}
		}
		return ""
	}
}

// RequiredRule 必填。
func RequiredRule(label string) Rule {
	return func(v string) string { return ValidateRequired(label, v) }
}

// MinLenRule 最小长度（按 rune 计）。
func MinLenRule(n int, label string) Rule {
	return func(v string) string { return ValidateMinLen(label, v, n) }
}

// MaxLenRule 最大长度（按 rune 计）。
func MaxLenRule(n int, label string) Rule {
	return func(v string) string {
		if l := len([]rune(strings.TrimSpace(v))); l > n {
			return fmt.Sprintf("%s最多 %d 个字符", label, n)
		}
		return ""
	}
}

// EmailRule 邮箱格式（复用 ValidateEmail 的宽松规则）。
func EmailRule(label string) Rule {
	return func(v string) string { return ValidateEmail(label, v) }
}

// PatternRule 正则校验（空值视为通过，必填请配 RequiredRule）。
func PatternRule(pat *regexp.Regexp, label string) Rule {
	return func(v string) string {
		s := strings.TrimSpace(v)
		if s == "" || pat == nil {
			return ""
		}
		if !pat.MatchString(s) {
			return label + "格式不正确"
		}
		return ""
	}
}

// NumberRule 必须是数字（空值视为通过）。
func NumberRule(label string) Rule {
	return func(v string) string {
		s := strings.TrimSpace(v)
		if s == "" {
			return ""
		}
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return label + "必须是数字"
		}
		return ""
	}
}

// RangeRule 数值范围（空值或非数字交给 RequiredRule/NumberRule，不重复报错）。
func RangeRule(min, max float64, label string) Rule {
	return func(v string) string {
		s := strings.TrimSpace(v)
		if s == "" {
			return ""
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return ""
		}
		if n < min || n > max {
			return fmt.Sprintf("%s应在 %v ~ %v 之间", label, min, max)
		}
		return ""
	}
}

// PhoneRule 中国大陆手机号（11 位、1 开头）。
func PhoneRule(label string) Rule {
	return func(v string) string {
		s := strings.TrimSpace(v)
		if s == "" {
			return ""
		}
		if len(s) != 11 || s[0] != '1' {
			return label + "格式不正确"
		}
		for _, r := range s {
			if r < '0' || r > '9' {
				return label + "格式不正确"
			}
		}
		return ""
	}
}

// ---- 表单状态 ----

// FormState 字段值 + 规则 + 错误文案。并发安全（后台提交任务里读值也安全）。
type FormState struct {
	mu     sync.Mutex
	values map[string]string
	rules  map[string][]Rule
	errors map[string]string
	live   bool
}

// NewFormState 创建空表单状态。
func NewFormState() *FormState {
	return &FormState{
		values: map[string]string{},
		rules:  map[string][]Rule{},
		errors: map[string]string{},
	}
}

// ValidateOnChange 开启后每次 Set 都会立刻校验该字段（默认关闭，
// 默认行为是「提交时统一校验」，避免用户刚敲第一个字符就看到红字）。
func (f *FormState) ValidateOnChange(on bool) *FormState {
	f.mu.Lock()
	f.live = on
	f.mu.Unlock()
	return f
}

// Rule 给字段登记规则（可多次调用累加）。
func (f *FormState) Rule(key string, rs ...Rule) *FormState {
	if key == "" {
		return f
	}
	f.mu.Lock()
	f.rules[key] = append(f.rules[key], rs...)
	f.mu.Unlock()
	return f
}

// Set 写入字段值；值变了会清掉该字段的旧错误（开启 ValidateOnChange 则立刻重算）。
func (f *FormState) Set(key, value string) *FormState {
	f.mu.Lock()
	changed := f.values[key] != value
	f.values[key] = value
	var liveErr string
	if changed {
		delete(f.errors, key)
		if f.live {
			liveErr = f.checkLocked(key, value)
			if liveErr != "" {
				f.errors[key] = liveErr
			}
		}
	}
	f.mu.Unlock()
	if changed {
		RequestRedraw()
	}
	return f
}

// Value 读字段值。
func (f *FormState) Value(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.values[key]
}

// Error 读字段当前错误（空串 = 通过或尚未校验）。
func (f *FormState) Error(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.errors[key]
}

// Errors 返回错误副本（key → 文案）。
func (f *FormState) Errors() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]string, len(f.errors))
	for k, v := range f.errors {
		out[k] = v
	}
	return out
}

// Values 返回值副本。
func (f *FormState) Values() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]string, len(f.values))
	for k, v := range f.values {
		out[k] = v
	}
	return out
}

// Valid 是否没有任何错误（未校验过也算通过）。
func (f *FormState) Valid() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.errors) == 0
}

// Validate 全量校验：登记过规则的字段与已赋值字段都会被检查，返回是否全部通过。
func (f *FormState) Validate() bool {
	f.mu.Lock()
	keys := make([]string, 0, len(f.rules))
	for k := range f.rules {
		keys = append(keys, k)
	}
	for k := range f.values {
		if _, ok := f.rules[k]; !ok {
			keys = append(keys, k)
		}
	}
	f.errors = map[string]string{}
	for _, k := range keys {
		if msg := f.checkLocked(k, f.values[k]); msg != "" {
			f.errors[k] = msg
		}
	}
	ok := len(f.errors) == 0
	f.mu.Unlock()
	RequestRedraw()
	return ok
}

// ValidateField 只校验单个字段（失焦时用）。
func (f *FormState) ValidateField(key string) bool {
	f.mu.Lock()
	msg := f.checkLocked(key, f.values[key])
	if msg == "" {
		delete(f.errors, key)
	} else {
		f.errors[key] = msg
	}
	f.mu.Unlock()
	RequestRedraw()
	return msg == ""
}

// Reset 清空值与错误（保留规则）。
func (f *FormState) Reset() *FormState {
	f.mu.Lock()
	f.values = map[string]string{}
	f.errors = map[string]string{}
	f.mu.Unlock()
	RequestRedraw()
	return f
}

// Submit 全量校验通过时用值快照调 fn，并返回是否提交。任一字段不通过则不调 fn。
func (f *FormState) Submit(fn func(values map[string]string)) bool {
	if !f.Validate() {
		return false
	}
	if fn != nil {
		fn(f.Values())
	}
	return true
}

// Bind 生成回写该字段的 onChange 回调（接到 Input 上）。
func (f *FormState) Bind(key string) func(string) {
	return func(v string) { f.Set(key, v) }
}

// Field 渲染一个带校验的表单项：标签 + 输入框 + 错误文案（错误来自当前状态）。
// key 同时作为 Input 的节点 Key（焦点 / 编辑态 / 光标定位都按它走）。
func (f *FormState) Field(key, label, placeholder string, required bool) *Node {
	f.mu.Lock()
	val := f.values[key]
	errMsg := f.errors[key]
	f.mu.Unlock()
	return FormItem(label, required, errMsg, Input(key, val, placeholder, f.Bind(key)))
}

// checkLocked 跑字段规则（调用方持锁）。
func (f *FormState) checkLocked(key, value string) string {
	for _, r := range f.rules[key] {
		if r == nil {
			continue
		}
		if msg := r(value); msg != "" {
			return msg
		}
	}
	return ""
}
