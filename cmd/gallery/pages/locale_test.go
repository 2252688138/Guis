// guis — gallery 多语言（顶部菜单「语言」+ 标题栏语言胶囊）的回归测试
package pages

import (
	"testing"

	"guis/ui"
)

// TestTitleMenuLanguage 顶部菜单最后一组是「语言」，条目覆盖每种可用语言 + 跟随系统。
func TestTitleMenuLanguage(t *testing.T) {
	InitLocales()
	old := ui.Locale()
	defer ui.SetLocale(old)
	ui.SetLocale("zh-CN")

	menus := titleMenus()
	last := menus[len(menus)-1]
	if last.Label != "语言" {
		t.Fatalf("最后一组菜单应为「语言」，实际 %q", last.Label)
	}
	locales := ui.AvailableLocales()
	if len(last.Items) != len(locales)+2 {
		t.Fatalf("语言菜单应有 %d 项（语言 + 分隔 + 跟随系统），实际 %d", len(locales)+2, len(last.Items))
	}
	if !last.Items[0].Checked {
		t.Fatal("当前语言应打勾")
	}
	// 点第二项切到另一种语言
	target := last.Items[1]
	want := ui.Locale()
	target.OnPick()
	if ui.Locale() == want {
		t.Fatalf("点击语言项后语言未切换（仍是 %q）", want)
	}
	if ui.PrefLocale() != ui.Locale() {
		t.Fatalf("切换后应写偏好，实际 %q", ui.PrefLocale())
	}
	ui.DelPref("ui.locale")
}

// TestShellLocaleSwitch 切到英文后顶部菜单与导航标题变成英文（TOr 生效）。
func TestShellLocaleSwitch(t *testing.T) {
	InitLocales()
	old := ui.Locale()
	oldCurrent := Current
	defer func() {
		ui.SetLocale(old)
		Current = oldCurrent
		ui.DelPref("ui.locale")
	}()
	ui.SetLocale("en-US")
	Current = "basic"

	menus := titleMenus()
	if menus[0].Label != "File" {
		t.Fatalf("英文下第一组菜单应为 File，实际 %q", menus[0].Label)
	}
	if got := menus[len(menus)-1].Label; got != "Language" {
		t.Fatalf("英文下应为 Language，实际 %q", got)
	}
	if got := CaseLabel(Cases[0]); got != "Basic components" {
		t.Fatalf("案例名未翻译：%q", got)
	}
	// 英文下导航标题（构建期读 ui.TOr）
	root := View()
	ui.LayoutForTest(root, 1180, 900, "", 1)
	if n := findKeyNode(root, "title-locale"); n == nil {
		t.Fatal("标题栏缺少语言胶囊（title-locale）")
	}
}

// TestLocaleChip 语言胶囊点击后在可用语言间轮换。
func TestLocaleChip(t *testing.T) {
	InitLocales()
	old := ui.Locale()
	defer func() {
		ui.SetLocale(old)
		ui.DelPref("ui.locale")
	}()
	ui.SetLocale("zh-CN")

	chip := localeChip()
	if chip == nil {
		t.Fatal("语言胶囊不应为 nil")
	}
	next := NextLocale()
	if next == "" || next == ui.Locale() {
		t.Fatalf("NextLocale 应返回另一种语言，实际 %q", next)
	}
	if len(ui.AvailableLocales()) > 1 && next == ui.Locale() {
		t.Fatal("多语言下轮换不应停在原语言")
	}
}
