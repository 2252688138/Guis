// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "strings"

// 复数形式（CLDR cardinal 规则的实用子集）。
//
// 语言文件里用后缀区分形式：items.zero / items.one / items.two / items.few /
// items.many / items.other；Tn 按当前语言挑一条，缺失时退回 .other，
// 再退回不带后缀的 key（所以「不需要复数」的语言只写一条 items 也可以）。
const (
	pluralZero  = "zero"
	pluralOne   = "one"
	pluralTwo   = "two"
	pluralFew   = "few"
	pluralMany  = "many"
	pluralOther = "other"
)

// pluralMessage 复数取值：key.form → key.other → key。
//
// 注意查找顺序是「先语言、后形态」：同一个 key 的 key.other 在兜底语言里存在，
// 但它不该盖过当前语言里不带后缀的那条（中文只有一条 "items": "{n} 项"）。
func pluralMessage(key string, n int) string {
	code, fb := localeState()
	form := pluralForm(code, n)
	cands := make([]string, 0, 3)
	cands = append(cands, key+"."+form)
	if form != pluralOther {
		cands = append(cands, key+"."+pluralOther)
	}
	cands = append(cands, key)
	for _, lang := range localeChain(code) {
		if v, ok := catalogLookupIn(lang, cands); ok {
			return v
		}
	}
	if fb != "" {
		if v, ok := catalogLookupIn(fb, cands); ok {
			return v
		}
	}
	return humanizeKey(key)
}

// catalogLookupIn 在指定语言的字符串表里按顺序查若干候选 key。
func catalogLookupIn(lang string, keys []string) (string, bool) {
	i18nMu.RLock()
	defer i18nMu.RUnlock()
	m, ok := i18nCatalogs[lang]
	if !ok {
		return "", false
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != "" {
			return v, true
		}
	}
	return "", false
}

// pluralForm 按语言与数量返回 CLDR 复数形式。
//
// 只覆盖「数量词」的实际用法（界面上几乎只有个位数到大数字），
// 不做分数与序数；未收录的语言按英语规则处理（one / other），
// 这是最不容易出错的安全选择。
func pluralForm(code string, n int) string {
	lang := baseLanguage(code)
	switch lang {
	// ---- 无复数变化（中文 / 日 / 韩 / 越 / 泰 / 印尼 / 土 …）----
	case "zh", "ja", "ko", "vi", "th", "id", "ms", "tr", "fa", "lo", "my", "km", "yo", "ig":
		return pluralOther

	// ---- 阿拉伯语：zero / one / two / few / many / other ----
	case "ar":
		mod := n % 100
		switch {
		case n == 0:
			return pluralZero
		case n == 1:
			return pluralOne
		case n == 2:
			return pluralTwo
		case mod >= 3 && mod <= 10:
			return pluralFew
		case mod >= 11 && mod <= 99:
			return pluralMany
		}
		return pluralOther

	// ---- 希伯来语：one / two / other ----
	case "he", "iw":
		switch n {
		case 1:
			return pluralOne
		case 2:
			return pluralTwo
		}
		return pluralOther

	// ---- 斯拉夫语族（俄 / 乌 / 白 / 波兰 / 捷克 / 斯洛伐克 / 塞尔维亚 / 克罗地亚 …）----
	case "ru", "uk", "be", "pl", "cs", "sk", "hr", "sr", "bs", "sl":
		mod10, mod100 := n%10, n%100
		switch {
		case mod10 == 1 && mod100 != 11:
			return pluralOne
		case mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14):
			return pluralFew
		}
		return pluralMany

	// ---- 法语：0 与 1 都算单数 ----
	case "fr":
		if n == 0 || n == 1 {
			return pluralOne
		}
		return pluralOther

	// ---- 印地语：0 与 1 用单数 ----
	case "hi", "bn", "gu", "mr":
		if n == 0 || n == 1 {
			return pluralOne
		}
		return pluralOther

	// ---- 默认（英 / 德 / 西 / 葡 / 意 / 荷 …）：n==1 单数，其余复数 ----
	default:
		if n == 1 {
			return pluralOne
		}
		return pluralOther
	}
}

// baseLanguage 取语言代码的基础语言（"zh-Hant-TW" → "zh"）。
func baseLanguage(code string) string {
	if i := strings.IndexByte(code, '-'); i > 0 {
		return strings.ToLower(code[:i])
	}
	return strings.ToLower(code)
}
