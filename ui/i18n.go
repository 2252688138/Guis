// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/text/language"
)

// 国际化（i18n）与多端语言适配。
//
// 一套 UI 代码在 Windows / Linux / macOS / Web / Android / iOS 上共用同一份字符串表、
// 同一套格式化规则、同一套布局镜像逻辑：
//
//	ui.AutoLocale()                    // 偏好 > 系统语言 > en-US（启动时调一次）
//	ui.UseLocale("en-US")              // 切换并记住（写偏好 ui.locale）
//	ui.T("dialog.ok")                  // 取翻译（缺失回退：当前语言 → 基础语言 → 默认语言 → 可读 key）
//	ui.Tf("greet", map[string]string{"name": "舒"})
//	ui.Tn("items", n)                  // 复数：自动在 items.one / items.other / items.few… 里选
//	ui.LocaleNumber(1234.5, 1)         // 1,234.5 / 1 234,5 / ١٬٢٣٤٫٥
//	ui.LocaleDate(time.Now(), "medium")
//
// 字符串表是扁平 JSON（{"key":"文本"}），内置 zh-CN / en-US 由 go:embed 打进二进制
// （见 ui/locale/*.json），桌面 / 浏览器 / 移动端都不依赖外部文件；业务追加语言
// 用 LoadCatalog / LoadCatalogFile 即可。
//
// 复数后缀遵循 CLDR：.zero / .one / .two / .few / .many / .other。
// 方向（LTR/RTL）由语言元数据决定，SetLocale 会自动切换并镜像布局（见 SetRTLAuto）。

// TextDirection 文字方向。
type TextDirection uint8

const (
	// DirLTR 从左到右（中 / 英 / 日 / 韩…）。
	DirLTR TextDirection = iota
	// DirRTL 从右到左（阿拉伯语 / 希伯来语 / 波斯语…）。
	DirRTL
)

// LocaleInfo 一种语言的元数据：名称、方向、日期格式、月份 / 星期名、货币等。
//
// 内置语言自带完整元数据；业务用 LoadCatalog 载入新语言时如果没有注册元数据，
// 会按语言代码派生一份（方向、月份、日期格式都按语言家族推断）。
type LocaleInfo struct {
	// Code BCP-47 语言代码（如 "zh-CN" / "en-US"）。
	Code string
	// Name 语言名称（英文，便于日志与排序）。
	Name string
	// NativeName 该语言的本地写法（"中文（简体）" / "English (US)"），语言切换器显示它。
	NativeName string
	// Dir 文字方向（RTL 语言会自动镜像布局）。
	Dir TextDirection

	// DateShort / DateMedium / DateLong 日期图案（yyyy / MMMM / dd / EEEE 等占位符）。
	DateShort  string
	DateMedium string
	DateLong   string
	// TimeShort / TimeLong 时间图案（HH / mm / ss / a）。
	TimeShort string
	TimeLong  string

	// Months / MonthsShort 月份名（1 月在前）。
	Months      [12]string
	MonthsShort [12]string
	// Weekdays / WeekdaysShort 星期名（周日在前）。
	Weekdays      [7]string
	WeekdaysShort [7]string
	// FirstWeekday 一周第一天（0 = 周日，1 = 周一）。
	FirstWeekday int

	// AM / PM 上下午标记（24 小时制语言留空）。
	AM string
	PM string

	// Currency 默认货币代码（"CNY" / "USD"）。
	Currency string
	// CurrencySymbol 货币符号；CurrencySuffix 为真时符号跟在数字后面（"1 234,56 €"）。
	CurrencySymbol string
	CurrencySuffix bool
	// PercentSpace 百分号前是否有空格（"12 %"）。
	PercentSpace bool

	// Decimal / Group 小数点与千分位符号（仅作提示，实际格式化由 x/text 的
	// 本地化数字系统完成，阿拉伯语等会输出本地数字）。
	Decimal string
	Group   string

	// ListSep / ListLastSep 列表连接符（"1, 2 and 3" / "1、2 和 3"）。
	ListSep     string
	ListLastSep string
}

var (
	i18nMu sync.RWMutex
	// i18nCatalogs 语言代码 → 扁平字符串表。
	i18nCatalogs = map[string]map[string]string{}
	// i18nInfos 语言代码 → 元数据。
	i18nInfos = map[string]LocaleInfo{}
	// i18nOrder 注册顺序（语言切换器按此顺序展示）。
	i18nOrder = []string{}
	// i18nCur 当前语言（canonical）。
	i18nCur = "zh-CN"
	// i18nFallback 兜底语言（默认 en-US；可用 SetFallbackLocale 改）。
	i18nFallback = "en-US"
	// i18nRTLFlag 热路径（布局 / 绘制）无锁读取的 RTL 标志。
	i18nRTLFlag atomic.Bool
	// i18nRTLAuto 切换语言时按语言方向自动开关 RTL。
	i18nRTLAuto = true
	// i18nListeners 语言切换回调。
	i18nListeners []func(code string)
	// i18nGen 语言代数（业务可据此让缓存失效）。
	i18nGen atomic.Uint64
	// i18nHuman 缺失 key 的可读化缓存（避免每帧重复处理）。
	i18nHuman sync.Map
	// i18nPrinter 本地化数字格式化器缓存（见 i18n_format.go）。
	i18nPrinter sync.Map
)

// prefLocaleKey 语言偏好的存储键。
const prefLocaleKey = "ui.locale"

// ---- 语言注册与目录加载 ----

// LoadCatalog 载入一个语言的字符串表（JSON：{"key":"文本"}）。
//
// 可重复调用（同语言后载入的条目覆盖先前的），便于「内置 + 业务覆盖」。
func LoadCatalog(locale, jsonBody string) error {
	code := canonicalLocale(locale)
	if code == "" {
		return os.ErrInvalid
	}
	m := map[string]string{}
	if err := json.Unmarshal([]byte(jsonBody), &m); err != nil {
		return err
	}
	registerCatalog(code, m)
	return nil
}

// LoadCatalogFile 从文件载入字符串表（桌面端；Web / 移动端请用 LoadCatalog + go:embed）。
func LoadCatalogFile(locale, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return LoadCatalog(locale, string(raw))
}

// registerCatalog 合并一张字符串表并确保该语言有元数据。
func registerCatalog(code string, m map[string]string) {
	i18nMu.Lock()
	if old, ok := i18nCatalogs[code]; ok {
		for k, v := range m {
			old[k] = v
		}
	} else {
		i18nCatalogs[code] = m
		if _, ok := i18nInfos[code]; !ok {
			i18nInfos[code] = deriveLocaleInfo(code)
			i18nOrder = append(i18nOrder, code)
		}
	}
	i18nMu.Unlock()
}

// RegisterLocale 注册 / 覆盖一种语言的元数据（名称、方向、日期与月份名等）。
//
// 只想翻译字符串用 LoadCatalog 即可；需要「本地化的日期、月份名、货币符号」
// 时才需要它（也可以只覆盖部分字段：零值字段会按语言代码自动补齐）。
func RegisterLocale(info LocaleInfo) {
	code := canonicalLocale(info.Code)
	if code == "" {
		return
	}
	info.Code = code
	base := deriveLocaleInfo(code)
	mergeLocaleInfo(&info, base)
	i18nMu.Lock()
	if _, ok := i18nInfos[code]; !ok {
		i18nOrder = append(i18nOrder, code)
	}
	i18nInfos[code] = info
	i18nMu.Unlock()
}

// Locales 已注册语言（按注册顺序；内置 zh-CN / en-US 在最前）。
func Locales() []LocaleInfo {
	i18nMu.RLock()
	out := make([]LocaleInfo, 0, len(i18nOrder))
	for _, code := range i18nOrder {
		if info, ok := i18nInfos[code]; ok {
			out = append(out, info)
		}
	}
	i18nMu.RUnlock()
	return out
}

// HasLocale 是否已载入该语言的字符串表或元数据。
func HasLocale(locale string) bool {
	code := canonicalLocale(locale)
	if code == "" {
		return false
	}
	i18nMu.RLock()
	_, hasCat := i18nCatalogs[code]
	_, hasInfo := i18nInfos[code]
	i18nMu.RUnlock()
	return hasCat || hasInfo
}

// LocaleInfoOf 取某种语言的元数据。
func LocaleInfoOf(locale string) (LocaleInfo, bool) {
	code := canonicalLocale(locale)
	i18nMu.RLock()
	info, ok := i18nInfos[code]
	i18nMu.RUnlock()
	return info, ok
}

// ---- 当前语言 ----

// SetLocale 切换语言（未知语言也接受：只影响格式化与方向，翻译走回退链）。
//
// 切换后：自动按语言方向开关 RTL（可用 SetRTLAuto 关闭）、通知 OnLocaleChange
// 注册的回调、请求重绘。不会写偏好——需要记住选择请用 UseLocale。
func SetLocale(locale string) {
	code := canonicalLocale(locale)
	if code == "" {
		return
	}
	i18nMu.Lock()
	if i18nCur == code {
		i18nMu.Unlock()
		return
	}
	i18nCur = code
	dir := localeDirection(code)
	auto := i18nRTLAuto
	listeners := append([]func(string){}, i18nListeners...)
	i18nMu.Unlock()

	i18nGen.Add(1)
	if auto {
		setRTLFlag(dir == DirRTL)
	}
	for _, fn := range listeners {
		fn(code)
	}
	RequestRedraw()
}

// UseLocale 切换语言并记住选择（写偏好 ui.locale），下次启动 AutoLocale 会恢复它。
func UseLocale(locale string) {
	code := canonicalLocale(locale)
	if code == "" {
		return
	}
	SetLocale(code)
	SetPref(prefLocaleKey, code)
}

// Locale 当前语言（canonical，如 "zh-CN"）。
func Locale() string {
	i18nMu.RLock()
	code := i18nCur
	i18nMu.RUnlock()
	return code
}

// LocaleGeneration 语言代数：每次切换 +1，业务可据此判断缓存是否失效。
func LocaleGeneration() uint64 { return i18nGen.Load() }

// SetFallbackLocale 设置兜底语言（翻译缺失时最后查它，默认 en-US）。
func SetFallbackLocale(locale string) {
	code := canonicalLocale(locale)
	if code == "" {
		return
	}
	i18nMu.Lock()
	i18nFallback = code
	i18nMu.Unlock()
	RequestRedraw()
}

// FallbackLocale 兜底语言。
func FallbackLocale() string {
	i18nMu.RLock()
	code := i18nFallback
	i18nMu.RUnlock()
	return code
}

// PrefLocale 偏好里保存的语言（未设置返回空串）。
func PrefLocale() string {
	if v, ok := GetPref(prefLocaleKey); ok {
		return canonicalLocale(v)
	}
	return ""
}

// AutoLocale 自动选择语言：偏好 > 系统语言 > 兜底语言，并立即生效。
//
// 启动时调一次即可（Web / 移动端同样适用：系统语言的探测方式见
// SystemLanguages）。返回值是最终选中的语言代码。
func AutoLocale() string {
	cands := make([]string, 0, 4)
	if v := PrefLocale(); v != "" {
		cands = append(cands, v)
	}
	cands = append(cands, SystemLanguages()...)
	code := MatchLocale(cands...)
	if code == "" {
		code = FallbackLocale()
	}
	SetLocale(code)
	return code
}

// initStartupLocale 启动时确定界面语言：Options.Locale 显式指定优先，否则自动探测。
func initStartupLocale(want string) {
	if c := canonicalLocale(want); c != "" {
		SetLocale(c)
		return
	}
	AutoLocale()
}

// MatchLocale 在已注册语言里挑出与候选列表最匹配的一个（BCP-47 协商）。
//
// 候选按优先级排列（第一个是首选）；无法匹配返回空串。
// 逐个候选比较，因此 "zh-HK" 会优先命中 zh-TW / zh-CN 这类同族语言，
// 而不是被排在后面的任意候选抢走。
func MatchLocale(prefs ...string) string {
	infos := Locales()
	if len(infos) == 0 || len(prefs) == 0 {
		return ""
	}
	tags := make([]language.Tag, 0, len(infos))
	codes := make([]string, 0, len(infos))
	for _, in := range infos {
		t, err := language.Parse(in.Code)
		if err != nil {
			continue
		}
		tags = append(tags, t)
		codes = append(codes, in.Code)
	}
	if len(tags) == 0 {
		return ""
	}
	m := language.NewMatcher(tags)
	for _, p := range prefs {
		code := canonicalLocale(p)
		if code == "" {
			continue
		}
		want, err := language.Parse(code)
		if err != nil {
			continue
		}
		_, idx, conf := m.Match(want)
		if conf == language.No || idx < 0 || idx >= len(codes) {
			continue
		}
		return preferTranslated(codes[idx], codes)
	}
	return ""
}

// preferTranslated 最佳匹配语言如果没有随包译文，优先换成「同语族里带译文」的语言。
//
// zh-HK 在 x/text 里会匹配到 zh-TW（都是繁体），但如果只有 zh-CN 有译文，
// 界面文案会整套退回简体 / 英文；换成 zh-CN 至少文案是通顺的中文。
// 同语族里没有译文时保留原匹配（日期 / 数字仍然是用户习惯的格式）。
func preferTranslated(matched string, codes []string) string {
	if localeHasCatalog(matched) {
		return matched
	}
	base := baseLanguage(matched)
	for _, c := range codes {
		if baseLanguage(c) == base && localeHasCatalog(c) {
			return c
		}
	}
	return matched
}

// localeHasCatalog 该语言是否有字符串表。
func localeHasCatalog(code string) bool {
	i18nMu.RLock()
	_, ok := i18nCatalogs[code]
	i18nMu.RUnlock()
	return ok
}

// OnLocaleChange 注册语言切换回调（返回取消函数）。
//
// 界面每帧重建的框架不需要它；缓存了字符串的业务（菜单、托盘、窗口标题）用它刷新。
func OnLocaleChange(fn func(code string)) func() {
	if fn == nil {
		return func() {}
	}
	i18nMu.Lock()
	i18nListeners = append(i18nListeners, fn)
	idx := len(i18nListeners) - 1
	i18nMu.Unlock()
	return func() {
		i18nMu.Lock()
		if idx < len(i18nListeners) {
			i18nListeners = append(i18nListeners[:idx], i18nListeners[idx+1:]...)
		}
		i18nMu.Unlock()
	}
}

// ---- 翻译 ----

// T 取当前语言的翻译。
//
// 查找顺序：当前语言 → 语言基础链（zh-Hant-TW→zh-Hant→zh）→ 兜底语言 → 可读 key。
// 缺失时把 key 的可读化结果返回（"open_file" → "open file"，"ui.dialog.ok" → "ok"），
// 界面不会出现空白或原始键名。
func T(key string) string {
	if key == "" {
		return ""
	}
	code, fb := localeState()
	if v, ok := lookupCatalog(code, fb, key); ok {
		return v
	}
	return humanizeKey(key)
}

// Tf 带命名参数的翻译（占位 {name}），例：
//
//	ui.Tf("greet", map[string]string{"name": "舒"})   // "你好，舒"
func Tf(key string, args map[string]string) string {
	s := T(key)
	for k, v := range args {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// TOr 取翻译，缺失时返回 fallback。
//
// 适合「已有中文原文、想渐进接入 i18n」的场景：key 还没翻译时界面显示原文而不是
// 可读化的 key，翻译到位后自动切换。示例工程 gallery 的顶部菜单就是这样接入的：
//
//	ui.TOr("src.文件", "文件")   // zh-CN → "文件"，en-US → "File"
//
// 与 T 的区别：只在「当前语言链」里查（zh-Hant-TW → zh-Hant → zh → zh-CN），
// 不跨到兜底语言——否则中文界面里一旦只有英文译文，就会冒出英文而不是原文。
func TOr(key, fallback string) string {
	if key == "" {
		return fallback
	}
	code, _ := localeState()
	for _, lang := range localeChain(code) {
		if v, ok := catalogLookupIn(lang, []string{key}); ok {
			return v
		}
	}
	return fallback
}

// Tn 复数翻译：按当前语言的 CLDR 规则在 key.one / key.other / key.few… 中取值，
// 并把 {n}（存在时）替换为本地化数字。
//
//	// zh-CN: "3 项"   en-US: "3 items"   ru-RU: "3 элемента"
//	ui.Tn("items", 3)
func Tn(key string, n int) string {
	s := pluralMessage(key, n)
	return strings.ReplaceAll(s, "{n}", LocaleNumber(float64(n), 0))
}

// Tnf 复数 + 命名参数。
func Tnf(key string, n int, args map[string]string) string {
	s := Tn(key, n)
	for k, v := range args {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// Ts 位置参数版本（把 {0} {1}… 依次替换）。
func Ts(key string, args ...string) string {
	s := T(key)
	for i, v := range args {
		s = strings.ReplaceAll(s, "{"+itoa(i)+"}", v)
	}
	return s
}

// lookupCatalog 在语言链 + 兜底语言里查一条。
func lookupCatalog(code, fallback, key string) (string, bool) {
	i18nMu.RLock()
	defer i18nMu.RUnlock()
	for _, c := range localeChain(code) {
		if m, ok := i18nCatalogs[c]; ok {
			if v, ok := m[key]; ok && v != "" {
				return v, true
			}
		}
	}
	if fallback != "" && fallback != code {
		if m, ok := i18nCatalogs[fallback]; ok {
			if v, ok := m[key]; ok && v != "" {
				return v, true
			}
		}
	}
	return "", false
}

// localeChain 语言回退链：zh-Hant-TW → [zh-Hant-TW, zh-Hant, zh, zh-CN]。
func localeChain(code string) []string {
	if code == "" {
		return nil
	}
	out := []string{code}
	cur := code
	for {
		i := strings.LastIndexByte(cur, '-')
		if i <= 0 {
			break
		}
		cur = cur[:i]
		out = append(out, cur)
	}
	// 纯语言代码补一份「该语言的首选地区」（zh → zh-CN），
	// 这样 LoadCatalog("zh-CN") 后 zh-Hant 也能命中。
	base := out[len(out)-1]
	if d := defaultRegionLocale(base); d != "" {
		out = append(out, d)
	}
	return out
}

// defaultRegionLocale 语言代码 → 首选地区语言（仅内置语言有映射）。
func defaultRegionLocale(base string) string {
	switch base {
	case "zh":
		return "zh-CN"
	case "en":
		return "en-US"
	case "pt":
		return "pt-BR"
	case "es":
		return "es-ES"
	}
	return ""
}

// localeState 一次性读取当前语言与兜底语言（避免每帧多次加锁）。
func localeState() (code, fallback string) {
	i18nMu.RLock()
	code, fallback = i18nCur, i18nFallback
	i18nMu.RUnlock()
	return
}

// humanizeKey 缺失 key 的可读化：取最后一段，下划线转空格。
func humanizeKey(key string) string {
	if v, ok := i18nHuman.Load(key); ok {
		return v.(string)
	}
	s := key
	if i := strings.LastIndexAny(s, "./:"); i >= 0 && i+1 < len(s) {
		s = s[i+1:]
	}
	s = strings.ReplaceAll(s, "_", " ")
	out := strings.TrimSpace(s)
	if out == "" {
		out = key
	}
	i18nHuman.Store(key, out)
	return out
}

// ---- 方向与镜像 ----

// SetRTL 手动开关从右到左布局（会关闭语言方向的自动切换）。
//
// 业务明确要做 RTL 界面时用；按语言自动切换请用 SetLocale（默认行为）。
func SetRTL(on bool) {
	i18nRTLAuto = false
	setRTLFlag(on)
	RequestRedraw()
}

// SetRTLAuto 是否随语言自动切换 RTL（默认开启）。
func SetRTLAuto(on bool) {
	i18nMu.Lock()
	i18nRTLAuto = on
	cur := i18nCur
	i18nMu.Unlock()
	if on {
		setRTLFlag(localeDirection(cur) == DirRTL)
	}
}

// IsRTL 当前是否 RTL 布局。
func IsRTL() bool { return i18nRTLFlag.Load() }

// setRTLFlag 写 RTL 标志（变化时重绘）。
func setRTLFlag(on bool) {
	if i18nRTLFlag.Swap(on) != on {
		RequestRedraw()
	}
}

// DirAlign 文本默认对齐方向：RTL 时返回 AlignEnd。
func DirAlign() Align {
	if IsRTL() {
		return AlignEnd
	}
	return AlignStart
}

// MirrorRow 行布局镜像：RTL 时反转子节点顺序。
//
// 框架的 Row / Wrap / Grid 在 RTL 下会自动镜像，只有手工拼接的节点序列
// （例如业务自己按索引摆放的卡片行）需要它。
func MirrorRow(children ...*Node) *Node {
	if IsRTL() {
		for i, j := 0, len(children)-1; i < j; i, j = i+1, j-1 {
			children[i], children[j] = children[j], children[i]
		}
	}
	return Row(children...)
}

// MirrorIcon 需要镜像的方向图标（← →、‹ ›）。
func MirrorIcon(glyph string) string {
	if !IsRTL() {
		return glyph
	}
	switch glyph {
	case "→":
		return "←"
	case "←":
		return "→"
	case "›":
		return "‹"
	case "‹":
		return "›"
	}
	return glyph
}

// ---- 工具 ----

// canonicalLocale 归一化语言代码（zh_CN.UTF-8 → zh-CN），非法返回空串。
func canonicalLocale(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '.'); i > 0 && !strings.Contains(s[:i], "-") {
		// zh_CN.UTF-8
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "_", "-")
	if s == "C" || s == "POSIX" {
		return ""
	}
	if t, err := language.Parse(s); err == nil && t != language.Und {
		return t.String()
	}
	return s
}

// itoa 小整数转字符串（避免只为拼占位符引入 fmt）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
