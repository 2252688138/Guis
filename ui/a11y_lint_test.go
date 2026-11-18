// guis — A11yLint 回归用例

package ui

import "testing"

// a11yFind 在问题清单里找指定代码 + Key 的条目。
func a11yFind(issues []A11yIssue, code, key string) (A11yIssue, bool) {
	for _, it := range issues {
		if it.Code == code && it.Key == key {
			return it, true
		}
	}
	return A11yIssue{}, false
}

// 合规节点应当零告警。
func TestA11yLintClean(t *testing.T) {
	root := Column(
		&Node{Kind: KButton, Key: "ok", Text: "确定",
			OnClick: func() {},
			A11y:    A11yInfo{Role: A11yButton, Name: "确定"}},
		&Node{Kind: KImage, Key: "logo", A11y: A11yInfo{Role: A11yImage, Name: "品牌标识"}},
	)
	if issues := A11yLint(root); len(issues) != 0 {
		for _, it := range issues {
			t.Log(it.String())
		}
		t.Fatalf("期望零告警，实际 %d 条", len(issues))
	}
}

// 可交互但没角色 / 没名字，两条都要报。
func TestA11yLintMissingRoleAndName(t *testing.T) {
	root := Column(&Node{Kind: KButton, Key: "go", OnClick: func() {}})
	issues := A11yLint(root)

	if _, ok := a11yFind(issues, A11yIssueMissingRole, "go"); !ok {
		t.Fatalf("缺少 missing-role：%+v", issues)
	}
	if _, ok := a11yFind(issues, A11yIssueMissingName, "go"); !ok {
		t.Fatalf("缺少 missing-name：%+v", issues)
	}
}

// 有文本但没角色：只报 missing-role。
func TestA11yLintTextCountsAsName(t *testing.T) {
	root := &Node{Kind: KButton, Key: "go", Text: "确定", OnClick: func() {}}
	issues := A11yLint(root)

	if _, ok := a11yFind(issues, A11yIssueMissingName, "go"); ok {
		t.Fatalf("有文本时不应报 missing-name：%+v", issues)
	}
	if _, ok := a11yFind(issues, A11yIssueMissingRole, "go"); !ok {
		t.Fatalf("应报 missing-role：%+v", issues)
	}
}

// 可聚焦但没 Key。
func TestA11yLintFocusWithoutKey(t *testing.T) {
	n := Box()
	n.Style.Focusable = true
	issues := A11yLint(n)

	if _, ok := a11yFind(issues, A11yIssueFocusNoKey, ""); !ok {
		t.Fatalf("缺少 focus-without-key：%+v", issues)
	}
}

// Key 重复只报一次（第二次出现时告警）。
func TestA11yLintDuplicateKey(t *testing.T) {
	root := Column(
		Box().K("dup"),
		Box().K("dup"),
		Box().K("dup"),
	)
	issues := A11yLint(root)

	count := 0
	for _, it := range issues {
		if it.Code == A11yIssueDuplicateKey {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("重复 Key 应只报 1 次，实际 %d：%+v", count, issues)
	}
}

// 图片缺替代文本。
func TestA11yLintImageNoName(t *testing.T) {
	issues := A11yLint(Image("a.png"))
	if _, ok := a11yFind(issues, A11yIssueImageNoName, ""); !ok {
		t.Fatalf("缺少 image-without-name：%+v", issues)
	}
}

// 路径要可读：无 Key 的节点用「类型[序号]」。
func TestA11yLintPath(t *testing.T) {
	root := Box(
		&Node{Kind: KButton, OnClick: func() {}},
		&Node{Kind: KButton, OnClick: func() {}},
	)
	issues := A11yLint(root)
	if len(issues) == 0 {
		t.Fatal("应有告警")
	}
	for _, it := range issues {
		if it.Path == "" || it.Path[0] != '/' {
			t.Fatalf("路径格式不对：%q", it.Path)
		}
	}
	if issues[0].Path != "/box/button" {
		t.Fatalf("首个告警路径 = %q，期望 /box/button", issues[0].Path)
	}
}
