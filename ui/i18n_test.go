// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os"
	"strings"
	"testing"
	"time"
)

// withLocale 在测试里临时切语言 / 方向，结束后恢复（语言是进程级全局状态）。
func withLocale(t *testing.T, code string) {
	t.Helper()
	old := Locale()
	oldRTL := IsRTL()
	oldAuto := i18nRTLAuto
	t.Cleanup(func() {
		i18nRTLAuto = oldAuto
		SetLocale(old)
		setRTLFlag(oldRTL)
	})
	SetLocale(code)
}

// TestI18nTOrFallback TOr：有译文用译文，缺失用原文（渐进接入 i18n 的关键）。
func TestI18nTOrFallback(t *testing.T) {
	if err := LoadCatalog("en-US", `{"src.文件":"File"}`); err != nil {
		t.Fatal(err)
	}
	withLocale(t, "zh-CN")
	if got := TOr("src.文件", "文件"); got != "文件" {
		t.Fatalf("中文应显示原文，实际 %q", got)
	}
	withLocale(t, "en-US")
	if got := TOr("src.文件", "文件"); got != "File" {
		t.Fatalf("英文应显示译文，实际 %q", got)
	}
	if got := TOr("src.未翻译", "未翻译"); got != "未翻译" {
		t.Fatalf("缺失 key 应回退原文，实际 %q", got)
	}
}

// TestI18nBuiltinCatalogs 内置语言包随二进制发布：中英都有框架文案。
func TestI18nBuiltinCatalogs(t *testing.T) {
	codes := make(map[string]bool, 4)
	for _, in := range AvailableLocales() {
		codes[in.Code] = true
	}
	if !codes["zh-CN"] || !codes["en-US"] {
		t.Fatalf("内置语言包缺失，实际 %v", codes)
	}

	withLocale(t, "zh-CN")
	if got := T("ui.dialog.ok"); got != "确定" {
		t.Fatalf("zh-CN 文案错误：%q", got)
	}
	withLocale(t, "en-US")
	if got := T("ui.dialog.ok"); got != "OK" {
		t.Fatalf("en-US 文案错误：%q", got)
	}
}

// TestI18nFallbackChain 回退链：zh-Hant → zh-CN；未知语言 → 兜底语言 → 可读 key。
func TestI18nFallbackChain(t *testing.T) {
	withLocale(t, "zh-Hant")
	if got := T("ui.dialog.ok"); got != "确定" {
		t.Fatalf("zh-Hant 应回退到 zh-CN，实际 %q", got)
	}
	withLocale(t, "xx-YY")
	if got := T("ui.dialog.ok"); got != "OK" {
		t.Fatalf("未知语言应回退到兜底语言 en-US，实际 %q", got)
	}
	if got := T("ui.nonexistent_key"); got != "nonexistent key" {
		t.Fatalf("缺失 key 应可读化，实际 %q", got)
	}
}

// TestI18nPlural 复数：英一/多、俄 one/few/many、中只有一种形式。
func TestI18nPlural(t *testing.T) {
	if err := LoadCatalog("ru-RU", `{"items.one":"{n} элемент","items.few":"{n} элемента","items.many":"{n} элементов"}`); err != nil {
		t.Fatal(err)
	}
	withLocale(t, "en-US")
	if got := Tn("ui.time.minutesAgo", 1); got != "1 minute ago" {
		t.Fatalf("英文单数错误：%q", got)
	}
	if got := Tn("ui.time.minutesAgo", 5); got != "5 minutes ago" {
		t.Fatalf("英文复数错误：%q", got)
	}
	withLocale(t, "ru-RU")
	if got := Tn("items", 3); got != "3 элемента" {
		t.Fatalf("俄语 few 形式错误：%q", got)
	}
	if got := Tn("items", 5); got != "5 элементов" {
		t.Fatalf("俄语 many 形式错误：%q", got)
	}
	withLocale(t, "zh-CN")
	if got := Tn("ui.time.minutesAgo", 5); got != "5 分钟前" {
		t.Fatalf("中文复数错误：%q", got)
	}
}

// TestI18nNumberFormat 数字 / 百分比 / 货币按语言格式化。
func TestI18nNumberFormat(t *testing.T) {
	withLocale(t, "en-US")
	if got := LocaleNumber(1234.5, 1); got != "1,234.5" {
		t.Fatalf("en-US 数字错误：%q", got)
	}
	if got := LocaleCurrency(9.9, ""); got != "$9.90" {
		t.Fatalf("en-US 货币错误：%q", got)
	}
	withLocale(t, "de-DE")
	if got := LocaleNumber(1234.5, 1); got != "1.234,5" {
		t.Fatalf("de-DE 数字错误：%q", got)
	}
	if got := LocalePercent(12.5, 1); !strings.Contains(got, "12,5") {
		t.Fatalf("de-DE 百分比错误：%q", got)
	}
	if got := LocaleCurrency(9.9, ""); !strings.HasSuffix(got, "€") {
		t.Fatalf("de-DE 货币（后缀）错误：%q", got)
	}
	withLocale(t, "zh-CN")
	if got := LocaleNumber(1234567, 0); got != "1,234,567" {
		t.Fatalf("zh-CN 整数错误：%q", got)
	}
}

// TestI18nDateFormat 日期 / 时间 / 星期 / 月份按语言格式化。
func TestI18nDateFormat(t *testing.T) {
	d := time.Date(2026, time.November, 6, 20, 5, 9, 0, time.Local)
	withLocale(t, "zh-CN")
	if got := LocaleDate(d, "medium"); got != "2026年11月6日" {
		t.Fatalf("zh-CN 日期错误：%q", got)
	}
	if got := LocaleTime(d, "short"); got != "20:05" {
		t.Fatalf("zh-CN 时间错误：%q", got)
	}
	if got := LocaleWeekday(time.Friday, true); got != "星期五" {
		t.Fatalf("zh-CN 星期错误：%q", got)
	}
	withLocale(t, "en-US")
	if got := LocaleDate(d, "medium"); got != "Nov 6, 2026" {
		t.Fatalf("en-US 日期错误：%q", got)
	}
	if got := LocaleDatePattern(d, "yyyy-MM-dd EEE"); got != "2026-11-06 Fri" {
		t.Fatalf("自定义图案错误：%q", got)
	}
	withLocale(t, "ja-JP")
	if got := LocaleDate(d, "medium"); got != "2026年11月6日" {
		t.Fatalf("ja-JP 日期错误：%q", got)
	}
}

// TestI18nRelativeAndList 相对时间 / 时长 / 列表连接符。
func TestI18nRelativeAndList(t *testing.T) {
	now := time.Date(2026, time.November, 6, 12, 0, 0, 0, time.Local)
	withLocale(t, "en-US")
	if got := LocaleRelative(now.Add(-3*time.Minute), now); got != "3 minutes ago" {
		t.Fatalf("en-US 相对时间错误：%q", got)
	}
	if got := LocaleList([]string{"a", "b", "c"}); got != "a, b and c" {
		t.Fatalf("en-US 列表错误：%q", got)
	}
	if got := LocaleDuration(90 * time.Second); got != "1 minute and 30 seconds" {
		t.Fatalf("en-US 时长错误：%q", got)
	}
	withLocale(t, "zh-CN")
	if got := LocaleRelative(now.Add(-2*time.Hour), now); got != "2 小时前" {
		t.Fatalf("zh-CN 相对时间错误：%q", got)
	}
	if got := LocaleList([]string{"a", "b", "c"}); got != "a、b 和 c" {
		t.Fatalf("zh-CN 列表错误：%q", got)
	}
}

// TestI18nRTLLayout RTL 自动镜像：横向容器的子节点从右边开始排。
func TestI18nRTLLayout(t *testing.T) {
	defer func(old bool) { setRTLFlag(old) }(IsRTL())
	setRTLFlag(false)

	root := Row(
		Box().St(St().W(50).H(10)),
		Box().St(St().W(30).H(10)),
	).St(St().W(200).H(20))
	Layout(root, 200, 20, newTestCache())
	ltr := root.Children[0].Frame.X
	if ltr != 0 {
		t.Fatalf("LTR 首个元素应在最左，实际 x=%v", ltr)
	}

	// 新树（RTL 镜像按树记录，避免重复翻转）
	root2 := Row(
		Box().St(St().W(50).H(10)),
		Box().St(St().W(30).H(10)),
	).St(St().W(200).H(20))
	setRTLFlag(true)
	Layout(root2, 200, 20, newTestCache())
	if got := root2.Children[0].Frame.X; got != 150 {
		t.Fatalf("RTL 首个元素应贴右边（x=150），实际 %v", got)
	}
	if got := root2.Children[1].Frame.X; got != 120 {
		t.Fatalf("RTL 第二个元素 x 应为 120，实际 %v", got)
	}
	if DirAlign() != AlignEnd {
		t.Fatal("RTL 下 DirAlign 应为 AlignEnd")
	}

	// NoMirror 子树保持原样
	root3 := Row(
		Box().St(St().W(50).H(10)),
		Box().St(St().W(30).H(10)),
	).St(St().W(200).H(20))
	root3.NoMirror = true
	Layout(root3, 200, 20, newTestCache())
	if got := root3.Children[0].Frame.X; got != 0 {
		t.Fatalf("NoMirror 子树不应镜像，实际 x=%v", got)
	}
}

// TestI18nLocaleSwitchRTL 切到 RTL 语言自动镜像，切回中文自动恢复。
func TestI18nLocaleSwitchRTL(t *testing.T) {
	withLocale(t, "ar-SA")
	if !IsRTL() {
		t.Fatal("切换到阿拉伯语应自动开启 RTL")
	}
	if got := DirAlign(); got != AlignEnd {
		t.Fatalf("阿拉伯语 DirAlign 应为 AlignEnd，实际 %v", got)
	}
	if LocaleLabel("ar-SA") != "العربية" {
		t.Fatalf("阿拉伯语自称错误：%q", LocaleLabel("ar-SA"))
	}
	withLocale(t, "zh-CN")
	if IsRTL() {
		t.Fatal("切回中文应关闭 RTL")
	}
}

// TestI18nMatchLocale 语言协商：优先有译文的语言。
func TestI18nMatchLocale(t *testing.T) {
	if got := MatchLocale("en-GB"); got != "en-US" {
		t.Fatalf("en-GB 应匹配到 en-US，实际 %q", got)
	}
	if got := MatchLocale("zh-HK"); got != "zh-CN" {
		t.Fatalf("zh-HK 应匹配到 zh-CN，实际 %q", got)
	}
	if got := MatchLocale("de-DE"); got != "de-DE" {
		t.Fatalf("de-DE 已注册元数据，应匹配自身，实际 %q", got)
	}
}

// TestI18nSystemLanguages 系统语言探测（环境变量路径）。
func TestI18nSystemLanguages(t *testing.T) {
	old := os.Getenv("LC_ALL")
	defer func() {
		if old == "" {
			os.Unsetenv("LC_ALL")
		} else {
			os.Setenv("LC_ALL", old)
		}
		RefreshSystemLanguages()
	}()
	if err := os.Setenv("LC_ALL", "de_DE.UTF-8"); err != nil {
		t.Skip("无法设置环境变量")
	}
	RefreshSystemLanguages()
	found := false
	for _, c := range SystemLanguages() {
		if c == "de-DE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("LC_ALL=de_DE.UTF-8 应被识别为 de-DE，实际 %v", SystemLanguages())
	}
}

// TestI18nUseLocalePersists UseLocale 记住选择，AutoLocale 恢复偏好。
func TestI18nUseLocalePersists(t *testing.T) {
	old := Locale()
	oldPref, hadPref := GetPref(prefLocaleKey)
	defer func() {
		if hadPref {
			SetPref(prefLocaleKey, oldPref)
		} else {
			DelPref(prefLocaleKey)
		}
		SetLocale(old)
	}()
	UseLocale("en-US")
	if got := PrefLocale(); got != "en-US" {
		t.Fatalf("偏好未写入：%q", got)
	}
	SetLocale("zh-CN")
	if got := AutoLocale(); got != "en-US" {
		t.Fatalf("AutoLocale 应恢复偏好语言，实际 %q", got)
	}
}

// TestI18nSwitcher 语言切换器节点可构建且与选项数一致。
func TestI18nSwitcher(t *testing.T) {
	node := LocaleSwitcher("t-lang")
	if node == nil {
		t.Fatal("两种以上语言时切换器不应为 nil")
	}
	codes, labels := LocaleOptions()
	if len(codes) != len(labels) || len(codes) < 2 {
		t.Fatalf("语言选项异常：%v %v", codes, labels)
	}
	if len(node.Children) != len(codes) {
		t.Fatalf("切换器胶囊数 %d != 语言数 %d", len(node.Children), len(codes))
	}
}
