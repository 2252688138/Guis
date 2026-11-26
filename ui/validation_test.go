// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"regexp"
	"strings"
	"testing"
)

// TestRulesReturnsFirstError Rules 返回第一条未通过的错误。
func TestRulesReturnsFirstError(t *testing.T) {
	r := Rules(RequiredRule("邮箱"), EmailRule("邮箱"))
	if msg := r(""); !strings.Contains(msg, "请填写") {
		t.Fatalf("空值应报必填，实际 %q", msg)
	}
	if msg := r("abc"); !strings.Contains(msg, "格式") {
		t.Fatalf("非法邮箱应报格式，实际 %q", msg)
	}
	if msg := r("a@b.com"); msg != "" {
		t.Fatalf("合法值不应报错，实际 %q", msg)
	}
}

// TestBuiltinRules 内置规则的正反例。
func TestBuiltinRules(t *testing.T) {
	cases := []struct {
		name  string
		rule  Rule
		ok    string
		bad   string
		want  string
		empty bool // 空值是否应通过
	}{
		{"MaxLen", MaxLenRule(3, "昵称"), "abc", "abcd", "最多", false},
		{"Number", NumberRule("年龄"), "18", "abc", "数字", true},
		{"Range", RangeRule(1, 10, "数量"), "5", "50", "之间", true},
		{"Phone", PhoneRule("手机号"), "13800138000", "12345", "格式", true},
		{"Pattern", PatternRule(regexp.MustCompile(`^\d+$`), "编号"), "123", "12a", "格式", true},
	}
	for _, c := range cases {
		if msg := c.rule(c.ok); msg != "" {
			t.Fatalf("%s 合法值不应报错，实际 %q", c.name, msg)
		}
		if msg := c.rule(c.bad); !strings.Contains(msg, c.want) {
			t.Fatalf("%s 非法值应含 %q，实际 %q", c.name, c.want, msg)
		}
		if c.empty {
			if msg := c.rule(""); msg != "" {
				t.Fatalf("%s 空值应交给 RequiredRule，实际 %q", c.name, msg)
			}
		}
	}
}

// TestFormValidateAll 全量校验把每个字段的错误都填进来。
func TestFormValidateAll(t *testing.T) {
	f := NewFormState()
	f.Rule("email", Rules(RequiredRule("邮箱"), EmailRule("邮箱")))
	f.Rule("name", RequiredRule("姓名"))

	if f.Validate() {
		t.Fatal("空表单不应通过校验")
	}
	errs := f.Errors()
	if len(errs) != 2 || !strings.Contains(errs["email"], "请填写") {
		t.Fatalf("应有 2 条错误，实际 %v", errs)
	}

	f.Set("email", "a@b.com").Set("name", "张三")
	if !f.Validate() {
		t.Fatalf("修正后应通过校验，实际错误 %v", f.Errors())
	}
	if !f.Valid() {
		t.Fatal("Valid 应为 true")
	}
}

// TestFormSetClearsError 改值后旧错误立即消失（默认非即时校验）。
func TestFormSetClearsError(t *testing.T) {
	f := NewFormState()
	f.Rule("name", RequiredRule("姓名"))
	f.Validate()
	if f.Error("name") == "" {
		t.Fatal("应先产生错误")
	}
	f.Set("name", "李四")
	if msg := f.Error("name"); msg != "" {
		t.Fatalf("改值后旧错误应清掉，实际 %q", msg)
	}
}

// TestValidateOnChange 开启即时校验后，输入非法值立刻带错误。
func TestValidateOnChange(t *testing.T) {
	f := NewFormState().ValidateOnChange(true)
	f.Rule("email", EmailRule("邮箱"))

	f.Set("email", "abc")
	if msg := f.Error("email"); !strings.Contains(msg, "格式") {
		t.Fatalf("即时校验应立刻报错，实际 %q", msg)
	}
	f.Set("email", "a@b.com")
	if msg := f.Error("email"); msg != "" {
		t.Fatalf("合法值应清掉错误，实际 %q", msg)
	}
}

// TestSubmitOnlyWhenValid 校验不通过不执行提交回调。
func TestSubmitOnlyWhenValid(t *testing.T) {
	f := NewFormState()
	f.Rule("name", RequiredRule("姓名"))

	called := 0
	if f.Submit(func(map[string]string) { called++ }) {
		t.Fatal("空表单不应提交成功")
	}
	if called != 0 {
		t.Fatal("校验不通过不应调用提交回调")
	}

	f.Set("name", "王五")
	var got string
	if !f.Submit(func(v map[string]string) { got = v["name"] }) {
		t.Fatal("合法表单应提交成功")
	}
	if got != "王五" {
		t.Fatalf("提交应拿到值快照，实际 %q", got)
	}
}

// TestFormFieldRendersError Field 渲染出的节点里带错误文案。
func TestFormFieldRendersError(t *testing.T) {
	f := NewFormState()
	f.Rule("email", RequiredRule("邮箱"))
	f.Validate()

	n := f.Field("email", "邮箱", "you@example.com", true)
	if !nodeTextContains(n, "请填写邮箱") {
		t.Fatal("表单项应渲染出错误文案")
	}
	if findKey(n, "email") == nil {
		t.Fatal("表单项应包含 Key 为 email 的输入框")
	}
}

// nodeTextContains 递归找文本节点是否包含子串。
func nodeTextContains(n *Node, sub string) bool {
	if n == nil {
		return false
	}
	if n.Kind == KText && strings.Contains(n.Text, sub) {
		return true
	}
	for _, c := range n.Children {
		if nodeTextContains(c, sub) {
			return true
		}
	}
	return false
}
