// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

// 本地化格式化：数字 / 百分比 / 货币 / 日期 / 时间 / 相对时间 / 时长 / 列表 / 字节。
//
//	ui.LocaleNumber(1234.5, 1)   // zh-CN "1,234.5"   de-DE "1.234,5"
//	ui.LocalePercent(0.125, 1)   // "12.5%"
//	ui.LocaleCurrency(9.9, "")   // 按语言默认货币："¥9.90" / "$9.90" / "9,90 €"
//	ui.LocaleDate(t, "medium")   // zh-CN "2026年11月6日"   en-US "Nov 6, 2026"
//	ui.LocaleDateTime(t, "short")// "2026/11/6 20:48"
//
// 数字、百分比、货币走 x/text 的本地化数字系统（本地数字形状、分组、符号位置），
// 日期与时间用语言元数据里的图案（见 LocaleInfo），业务也可以用自定义图案：
//
//	ui.LocaleDatePattern(t, "yyyy-MM-dd EEE")
//
// 全部函数在没有注册语言元数据时也安全（按语言代码派生，最差退回 en-US 风格）。

// i18nTag 当前语言的 language.Tag（解析失败时按 und 处理）。
func i18nTag(code string) language.Tag {
	if code == "" {
		return language.Und
	}
	if t, err := language.Parse(code); err == nil {
		return t
	}
	return language.Und
}

// i18nPrinterFor 取（缓存的）本地化打印器。
//
// message.Printer 内部带语言相关的数字 / 复数数据，构造有一定成本，
// 而格式化函数在列表、图表刻度里会被每帧调用，因此按语言缓存。
func i18nPrinterFor(code string) *message.Printer {
	if v, ok := i18nPrinter.Load(code); ok {
		return v.(*message.Printer)
	}
	p := message.NewPrinter(i18nTag(code))
	i18nPrinter.Store(code, p)
	return p
}

// LocaleNumber 按当前语言格式化数字（decimals 为小数位；-1 表示最多 3 位且去掉尾零）。
func LocaleNumber(v float64, decimals int) string {
	return localeNumberIn(Locale(), v, decimals)
}

// localeNumberIn 指定语言的数字格式化。
func localeNumberIn(code string, v float64, decimals int) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "—"
	}
	p := i18nPrinterFor(code)
	var opts []number.Option
	if decimals < 0 {
		opts = append(opts, number.MaxFractionDigits(3))
	} else {
		opts = append(opts, number.MinFractionDigits(decimals), number.MaxFractionDigits(decimals))
	}
	return p.Sprintf("%v", number.Decimal(v, opts...))
}

// LocaleInt 整数（带千分位）。
func LocaleInt(n int64) string {
	p := i18nPrinterFor(Locale())
	return p.Sprintf("%d", n)
}

// LocalePercent 按当前语言格式化百分比（v 已经是百分比数值，如 12.5 → "12.5%"）。
func LocalePercent(v float64, decimals int) string {
	p := i18nPrinterFor(Locale())
	if decimals < 0 {
		decimals = 0
	}
	return p.Sprintf("%v", number.Percent(v/100, number.Scale(decimals)))
}

// currencySymbols 常见货币符号（未收录时直接用货币代码）。
var currencySymbols = map[string]string{
	"CNY": "¥", "USD": "$", "EUR": "€", "JPY": "¥", "GBP": "£", "KRW": "₩",
	"RUB": "₽", "INR": "₹", "BRL": "R$", "TWD": "NT$", "HKD": "HK$",
	"AUD": "A$", "CAD": "C$", "SGD": "S$", "CHF": "CHF", "THB": "฿",
	"SAR": "ر.س", "AED": "د.إ", "ILS": "₪", "PLN": "zł", "SEK": "kr",
	"NOK": "kr", "DKK": "kr", "TRY": "₺", "VND": "₫", "IDR": "Rp",
	"MYR": "RM", "PHP": "₱", "MXN": "MX$", "ZAR": "R", "UAH": "₴",
}

// LocaleCurrency 按当前语言格式化金额。
//
// code 为空时用语言默认货币（zh-CN → CNY、en-US → USD、de-DE → EUR）；
// 业务指定货币时用该货币符号，符号位置仍遵循语言习惯。
func LocaleCurrency(v float64, code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	info, _ := LocaleInfoOf(Locale())
	if code == "" {
		code = info.Currency
	}
	sym := currencySymbols[code]
	if sym == "" {
		sym = code
	}
	body := LocaleNumber(v, 2)
	if info.CurrencySuffix {
		return body + " " + sym
	}
	if v < 0 {
		// "-$9.90" 更符合英语习惯，避免 "$-9.90"
		return "-" + sym + strings.TrimPrefix(body, "-")
	}
	return sym + body
}

// ---- 日期 / 时间 ----

// LocaleDate 按当前语言格式化日期：style = "short" / "medium" / "long" / "full"。
func LocaleDate(t time.Time, style string) string {
	info, _ := LocaleInfoOf(Locale())
	return LocaleDatePattern(t, localeDatePattern(info, style))
}

// LocaleTime 按当前语言格式化时间：style = "short" / "long"。
func LocaleTime(t time.Time, style string) string {
	info, _ := LocaleInfoOf(Locale())
	pat := info.TimeShort
	if style == "long" || style == "full" {
		pat = info.TimeLong
	}
	if pat == "" {
		pat = "15:04"
	}
	return LocaleDatePattern(t, pat)
}

// LocaleDateTime 日期 + 时间（用空格连接，RTL 语言用 "，" 之外的普通空格）。
func LocaleDateTime(t time.Time, style string) string {
	return LocaleDate(t, style) + " " + LocaleTime(t, style)
}

// localeDatePattern 取某语言某风格的日期图案。
func localeDatePattern(info LocaleInfo, style string) string {
	switch style {
	case "short":
		if info.DateShort != "" {
			return info.DateShort
		}
	case "long", "full":
		if info.DateLong != "" {
			return info.DateLong
		}
	default:
		if info.DateMedium != "" {
			return info.DateMedium
		}
	}
	if info.DateShort != "" {
		return info.DateShort
	}
	return "yyyy-MM-dd"
}

// LocaleDatePattern 按自定义图案格式化日期。
//
// 支持的占位符（区分大小写）：
//
//	yyyy 年（4 位）  yy 年（2 位）  MMMM 月名  MMM 月名缩写  MM 两位月  M 月
//	dd 两位日  d 日  EEEE 星期名  EEE 星期缩写  e 星期数字（1 = 周一）
//	HH 两位时（24h）  H 时  hh 两位时（12h）  h 时（12h）  mm 分  m 分  ss 秒  s 秒
//	a 上下午标记
//
// 单引号包住的部分按字面输出（"d 'de' MMMM"），'' 表示一个单引号。
func LocaleDatePattern(t time.Time, pattern string) string {
	info, _ := LocaleInfoOf(Locale())
	return datePatternWith(t, pattern, info)
}

// datePatternWith 用给定元数据渲染图案（避免每次查表）。
func datePatternWith(t time.Time, pattern string, info LocaleInfo) string {
	if pattern == "" {
		pattern = "yyyy-MM-dd"
	}
	var b strings.Builder
	b.Grow(len(pattern) + 8)
	for i := 0; i < len(pattern); {
		c := pattern[i]
		if c == '\'' {
			// 字面量段
			j := i + 1
			for j < len(pattern) {
				if pattern[j] == '\'' {
					if j+1 < len(pattern) && pattern[j+1] == '\'' {
						b.WriteString(pattern[i+1 : j+1])
						i = j + 2
						continue
					}
					break
				}
				j++
			}
			if j >= len(pattern) {
				b.WriteString(pattern[i+1:])
				i = len(pattern)
			} else {
				b.WriteString(pattern[i+1 : j])
				i = j + 1
			}
			continue
		}
		n := 1
		for i+n < len(pattern) && pattern[i+n] == c {
			n++
		}
		tok := pattern[i : i+n]
		b.WriteString(dateToken(t, tok, info))
		i += n
	}
	return b.String()
}

// dateToken 渲染单个占位符；不认识的字符原样输出。
func dateToken(t time.Time, tok string, info LocaleInfo) string {
	switch tok {
	case "yyyy":
		return fmt.Sprintf("%04d", t.Year())
	case "yy":
		return fmt.Sprintf("%02d", t.Year()%100)
	case "MMMM":
		return info.Months[int(t.Month())-1]
	case "MMM":
		return info.MonthsShort[int(t.Month())-1]
	case "MM":
		return fmt.Sprintf("%02d", int(t.Month()))
	case "M":
		return strconv.Itoa(int(t.Month()))
	case "dd":
		return fmt.Sprintf("%02d", t.Day())
	case "d":
		return strconv.Itoa(t.Day())
	case "EEEE":
		return info.Weekdays[int(t.Weekday())]
	case "EEE":
		return info.WeekdaysShort[int(t.Weekday())]
	case "e":
		// ISO 星期：周一 = 1 … 周日 = 7
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		return strconv.Itoa(wd)
	case "HH":
		return fmt.Sprintf("%02d", t.Hour())
	case "H":
		return strconv.Itoa(t.Hour())
	case "hh":
		return fmt.Sprintf("%02d", hour12(t.Hour()))
	case "h":
		return strconv.Itoa(hour12(t.Hour()))
	case "mm":
		return fmt.Sprintf("%02d", t.Minute())
	case "m":
		return strconv.Itoa(t.Minute())
	case "ss":
		return fmt.Sprintf("%02d", t.Second())
	case "s":
		return strconv.Itoa(t.Second())
	case "a":
		if t.Hour() < 12 {
			return info.AM
		}
		return info.PM
	}
	return tok
}

// hour12 12 小时制的时（0 → 12，13 → 1）。
func hour12(h int) int {
	h %= 12
	if h == 0 {
		return 12
	}
	return h
}

// LocaleWeekday 星期名（full = 完整写法，否则缩写）。
func LocaleWeekday(wd time.Weekday, full bool) string {
	info, _ := LocaleInfoOf(Locale())
	if full {
		return info.Weekdays[int(wd)]
	}
	return info.WeekdaysShort[int(wd)]
}

// LocaleMonth 月份名（full = 完整写法，否则缩写）。
func LocaleMonth(m time.Month, full bool) string {
	info, _ := LocaleInfoOf(Locale())
	i := int(m) - 1
	if i < 0 || i > 11 {
		i = 0
	}
	if full {
		return info.Months[i]
	}
	return info.MonthsShort[i]
}

// LocaleWeekStart 一周第一天（0 = 周日，1 = 周一）。
func LocaleWeekStart() int {
	info, _ := LocaleInfoOf(Locale())
	return info.FirstWeekday
}

// ---- 相对时间 / 时长 / 列表 ----

// LocaleRelative 相对时间（"3 分钟前" / "in 2 hours"）。
//
// now 传零值时用 time.Now()。
func LocaleRelative(t, now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	d := now.Sub(t)
	future := d < 0
	if future {
		d = -d
	}
	key, n := relativeKey(d)
	if key == "" {
		return LocaleDate(t, "short")
	}
	if future {
		if fk, ok := relativeFutureNames[key]; ok {
			key = fk
		}
	}
	s := Tn(key, n)
	if s == "" || strings.HasPrefix(s, key) {
		// 没有对应文案时退化为日期，避免露出原始 key
		return LocaleDate(t, "short")
	}
	return s
}

// relativeFutureNames 过去时 key → 将来时 key。
var relativeFutureNames = map[string]string{
	"ui.time.justNow":    "ui.time.justNow",
	"ui.time.minutesAgo": "ui.time.inMinutes",
	"ui.time.hoursAgo":   "ui.time.inHours",
	"ui.time.daysAgo":    "ui.time.inDays",
	"ui.time.weeksAgo":   "ui.time.inWeeks",
	"ui.time.monthsAgo":  "ui.time.inMonths",
	"ui.time.yearsAgo":   "ui.time.inYears",
}

// relativeKey 时长 → 文案 key 与数量。
func relativeKey(d time.Duration) (string, int) {
	switch {
	case d < time.Minute:
		return "ui.time.justNow", 0
	case d < time.Hour:
		return "ui.time.minutesAgo", int(d / time.Minute)
	case d < 24*time.Hour:
		return "ui.time.hoursAgo", int(d / time.Hour)
	case d < 7*24*time.Hour:
		return "ui.time.daysAgo", int(d / (24 * time.Hour))
	case d < 30*24*time.Hour:
		return "ui.time.weeksAgo", int(d / (7 * 24 * time.Hour))
	case d < 365*24*time.Hour:
		return "ui.time.monthsAgo", int(d / (30 * 24 * time.Hour))
	}
	return "ui.time.yearsAgo", int(d / (365 * 24 * time.Hour))
}

// LocaleDuration 时长（"1 小时 5 分钟" / "1 h 5 min"）。
func LocaleDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	sec := int(math.Round(d.Seconds()))
	if sec < 60 {
		return Tn("ui.time.seconds", sec)
	}
	h, m, s := sec/3600, (sec%3600)/60, sec%60
	parts := make([]string, 0, 3)
	if h > 0 {
		parts = append(parts, Tn("ui.time.hours", h))
	}
	if m > 0 || h > 0 {
		parts = append(parts, Tn("ui.time.minutes", m))
	}
	if s > 0 && h == 0 {
		parts = append(parts, Tn("ui.time.seconds", s))
	}
	return LocaleList(parts)
}

// LocaleList 用当前语言的连接符拼接列表（"1、2 和 3" / "1, 2 and 3"）。
func LocaleList(items []string) string {
	items = trimEmpty(items)
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	info, _ := LocaleInfoOf(Locale())
	sep, last := info.ListSep, info.ListLastSep
	if sep == "" {
		sep = ", "
	}
	if last == "" {
		last = sep
	}
	return strings.Join(items[:len(items)-1], sep) + last + items[len(items)-1]
}

// trimEmpty 去掉空字符串项（保留小容量切片，避免额外分配）。
func trimEmpty(items []string) []string {
	out := items[:0]
	for _, s := range items {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// LocaleBytes 字节数（"1.5 MB" / "1,5 MB"）。
func LocaleBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return LocaleInt(n) + " B"
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	v := float64(n)
	i := -1
	for v >= unit && i < len(units)-1 {
		v /= unit
		i++
	}
	dec := 1
	if v >= 100 {
		dec = 0
	}
	return LocaleNumber(v, dec) + " " + units[i]
}
