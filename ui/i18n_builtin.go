// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"embed"
	"sort"
	"strings"
)

// 内置语言包：随二进制发布，桌面 / 浏览器 / 移动端行为一致。
//
// 新增语言只需往 ui/locale/ 丢一个 <code>.json（扁平 {"key":"文本"}），
// 编译时自动打进二进制并出现在语言切换器里；业务仍可用 LoadCatalog 覆盖或补充。

//go:embed locale/*.json
var builtinLocales embed.FS

// builtinLocaleOrder 内置语言的展示顺序（其余按代码排序追加在后面）。
var builtinLocaleOrder = []string{"zh-CN", "en-US"}

// builtinMetaLocales 只内置元数据（日期 / 月份 / 数字 / 方向）的语言。
//
// 这些语言没有随包翻译（业务用 LoadCatalog 补），但切换过去时日期、数字、
// 月份名与布局方向都是正确的——阿拉伯语 / 希伯来语会自动切成 RTL。
var builtinMetaLocales = []string{
	"zh-TW", "ja-JP", "ko-KR", "de-DE", "fr-FR", "es-ES", "ru-RU", "pt-BR",
	"it-IT", "nl-NL", "pl-PL", "ar-SA", "he-IL", "hi-IN", "th-TH",
}

func init() {
	registerBuiltinLocales()
}

// registerBuiltinLocales 载入内置语言包与元数据（重复调用无副作用）。
func registerBuiltinLocales() {
	for _, code := range builtinMetaLocales {
		RegisterLocale(LocaleInfo{Code: code})
	}
	entries, err := builtinLocales.ReadDir("locale")
	if err != nil {
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	// 先按内置顺序载入（zh-CN / en-US 排在最前），其余按文件名顺序。
	ordered := make([]string, 0, len(names))
	used := make(map[string]bool, len(names))
	for _, want := range builtinLocaleOrder {
		for _, name := range names {
			if used[name] {
				continue
			}
			if name == want+".json" {
				ordered = append(ordered, name)
				used[name] = true
			}
		}
	}
	for _, name := range names {
		if !used[name] {
			ordered = append(ordered, name)
		}
	}
	for _, name := range ordered {
		raw, err := builtinLocales.ReadFile("locale/" + name)
		if err != nil {
			continue
		}
		code := strings.TrimSuffix(name, ".json")
		if err := LoadCatalog(code, string(raw)); err != nil {
			continue
		}
		RegisterLocale(LocaleInfo{Code: code})
	}
}

// AvailableLocales 随包提供翻译的语言（语言切换器只列这些）。
//
// 与 Locales() 的区别：Locales() 还包括「只有元数据」的语言
// （切过去日期 / 数字正确，但译文回退到兜底语言）。
func AvailableLocales() []LocaleInfo {
	all := Locales()
	out := make([]LocaleInfo, 0, len(all))
	i18nMu.RLock()
	for _, info := range all {
		if _, ok := i18nCatalogs[info.Code]; ok {
			out = append(out, info)
		}
	}
	i18nMu.RUnlock()
	return out
}
