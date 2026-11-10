// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "strings"

// 内置语言的元数据（月份 / 星期 / 日期图案 / 货币 / 列表连接符）。
//
// 按「基础语言」收录：zh-Hant-TW 会落在 zh 上再覆盖地区差异（见 deriveLocaleInfo）。
// 业务要支持表里没有的语言，只需 LoadCatalog + RegisterLocale（后者可只填差异字段，
// 其余按语言代码自动补齐）。

// langData 一套语言的本地化数据。
type langData struct {
	name    string // 英文名（日志 / 排序）
	native  string // 本地写法（语言切换器显示）
	months  [12]string
	monthsS [12]string
	days    [7]string // 周日在前
	daysS   [7]string

	dateShort  string // 01/02/2006 风格
	dateMedium string
	dateLong   string
	timeShort  string
	timeLong   string
	am, pm     string
	firstDay   int // 0 = 周日，1 = 周一

	currency  string
	curSymbol string
	curSuffix bool
	pctSpace  bool
	decimal   string
	group     string
	listSep   string
	listLast  string
}

// langTable 内置语言数据表（键为基础语言代码）。
var langTable = map[string]langData{
	"en": {
		name: "English", native: "English",
		months: [12]string{"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December"},
		monthsS: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun",
			"Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		days:   [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
		daysS:  [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		dateShort: "M/d/yyyy", dateMedium: "MMM d, yyyy", dateLong: "EEEE, MMMM d, yyyy",
		timeShort: "h:mm a", timeLong: "h:mm:ss a", am: "AM", pm: "PM", firstDay: 0,
		currency: "USD", curSymbol: "$", decimal: ".", group: ",",
		listSep: ", ", listLast: " and ",
	},
	"zh": {
		name: "Chinese", native: "中文",
		months: [12]string{"一月", "二月", "三月", "四月", "五月", "六月",
			"七月", "八月", "九月", "十月", "十一月", "十二月"},
		monthsS: [12]string{"1月", "2月", "3月", "4月", "5月", "6月",
			"7月", "8月", "9月", "10月", "11月", "12月"},
		days:  [7]string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"},
		daysS: [7]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"},
		dateShort: "yyyy/M/d", dateMedium: "yyyy年M月d日", dateLong: "yyyy年M月d日 EEEE",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 1,
		currency: "CNY", curSymbol: "¥", decimal: ".", group: ",",
		listSep: "、", listLast: " 和 ",
	},
	"ja": {
		name: "Japanese", native: "日本語",
		months: [12]string{"1月", "2月", "3月", "4月", "5月", "6月",
			"7月", "8月", "9月", "10月", "11月", "12月"},
		monthsS: [12]string{"1月", "2月", "3月", "4月", "5月", "6月",
			"7月", "8月", "9月", "10月", "11月", "12月"},
		days:  [7]string{"日曜日", "月曜日", "火曜日", "水曜日", "木曜日", "金曜日", "土曜日"},
		daysS: [7]string{"日", "月", "火", "水", "木", "金", "土"},
		dateShort: "yyyy/MM/dd", dateMedium: "yyyy年M月d日", dateLong: "yyyy年M月d日 EEEE",
		timeShort: "H:mm", timeLong: "H:mm:ss", firstDay: 0,
		currency: "JPY", curSymbol: "¥", decimal: ".", group: ",",
		listSep: "、", listLast: "と",
	},
	"ko": {
		name: "Korean", native: "한국어",
		months: [12]string{"1월", "2월", "3월", "4월", "5월", "6월",
			"7월", "8월", "9월", "10월", "11월", "12월"},
		monthsS: [12]string{"1월", "2월", "3월", "4월", "5월", "6월",
			"7월", "8월", "9월", "10월", "11월", "12월"},
		days:  [7]string{"일요일", "월요일", "화요일", "수요일", "목요일", "금요일", "토요일"},
		daysS: [7]string{"일", "월", "화", "수", "목", "금", "토"},
		dateShort: "yyyy. M. d.", dateMedium: "yyyy년 M월 d일", dateLong: "yyyy년 M월 d일 EEEE",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 0,
		currency: "KRW", curSymbol: "₩", decimal: ".", group: ",",
		listSep: ", ", listLast: " 및 ",
	},
	"de": {
		name: "German", native: "Deutsch",
		months: [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni",
			"Juli", "August", "September", "Oktober", "November", "Dezember"},
		monthsS: [12]string{"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni",
			"Juli", "Aug.", "Sept.", "Okt.", "Nov.", "Dez."},
		days:  [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"},
		daysS: [7]string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"},
		dateShort: "dd.MM.yyyy", dateMedium: "dd.MM.yyyy", dateLong: "EEEE, d. MMMM yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 1,
		currency: "EUR", curSymbol: "€", curSuffix: true, pctSpace: true,
		decimal: ",", group: ".", listSep: ", ", listLast: " und ",
	},
	"fr": {
		name: "French", native: "Français",
		months: [12]string{"janvier", "février", "mars", "avril", "mai", "juin",
			"juillet", "août", "septembre", "octobre", "novembre", "décembre"},
		monthsS: [12]string{"janv.", "févr.", "mars", "avr.", "mai", "juin",
			"juil.", "août", "sept.", "oct.", "nov.", "déc."},
		days:  [7]string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"},
		daysS: [7]string{"dim.", "lun.", "mar.", "mer.", "jeu.", "ven.", "sam."},
		dateShort: "dd/MM/yyyy", dateMedium: "d MMM yyyy", dateLong: "EEEE d MMMM yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 1,
		currency: "EUR", curSymbol: "€", curSuffix: true, pctSpace: true,
		decimal: ",", group: " ", listSep: ", ", listLast: " et ",
	},
	"es": {
		name: "Spanish", native: "Español",
		months: [12]string{"enero", "febrero", "marzo", "abril", "mayo", "junio",
			"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"},
		monthsS: [12]string{"ene", "feb", "mar", "abr", "may", "jun",
			"jul", "ago", "sept", "oct", "nov", "dic"},
		days:  [7]string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"},
		daysS: [7]string{"dom", "lun", "mar", "mié", "jue", "vie", "sáb"},
		dateShort: "d/M/yyyy", dateMedium: "d MMM yyyy", dateLong: "EEEE, d 'de' MMMM 'de' yyyy",
		timeShort: "H:mm", timeLong: "H:mm:ss", firstDay: 1,
		currency: "EUR", curSymbol: "€", curSuffix: true,
		decimal: ",", group: ".", listSep: ", ", listLast: " y ",
	},
	"ru": {
		name: "Russian", native: "Русский",
		months: [12]string{"января", "февраля", "марта", "апреля", "мая", "июня",
			"июля", "августа", "сентября", "октября", "ноября", "декабря"},
		monthsS: [12]string{"янв.", "февр.", "мар.", "апр.", "мая", "июн.",
			"июл.", "авг.", "сент.", "окт.", "нояб.", "дек."},
		days:  [7]string{"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота"},
		daysS: [7]string{"вс", "пн", "вт", "ср", "чт", "пт", "сб"},
		dateShort: "dd.MM.yyyy", dateMedium: "d MMM yyyy 'г.'", dateLong: "d MMMM yyyy 'г.',' EEEE",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 1,
		currency: "RUB", curSymbol: "₽", curSuffix: true, pctSpace: true,
		decimal: ",", group: " ", listSep: ", ", listLast: " и ",
	},
	"ar": {
		name: "Arabic", native: "العربية",
		months: [12]string{"يناير", "فبراير", "مارس", "أبريل", "مايو", "يونيو",
			"يوليو", "أغسطس", "سبتمبر", "أكتوبر", "نوفمبر", "ديسمبر"},
		monthsS: [12]string{"يناير", "فبراير", "مارس", "أبريل", "مايو", "يونيو",
			"يوليو", "أغسطس", "سبتمبر", "أكتوبر", "نوفمبر", "ديسمبر"},
		days:  [7]string{"الأحد", "الاثنين", "الثلاثاء", "الأربعاء", "الخميس", "الجمعة", "السبت"},
		daysS: [7]string{"أحد", "اثنين", "ثلاثاء", "أربعاء", "خميس", "جمعة", "سبت"},
		dateShort: "d/M/yyyy", dateMedium: "d MMM yyyy", dateLong: "EEEE، d MMMM yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 6,
		currency: "SAR", curSymbol: "ر.س", curSuffix: true,
		decimal: ".", group: ",",
		listSep: "، ", listLast: " و",
	},
	"he": {
		name: "Hebrew", native: "עברית",
		months: [12]string{"ינואר", "פברואר", "מרץ", "אפריל", "מאי", "יוני",
			"יולי", "אוגוסט", "ספטמבר", "אוקטובר", "נובמבר", "דצמבר"},
		monthsS: [12]string{"ינו׳", "פבר׳", "מרץ", "אפר׳", "מאי", "יוני",
			"יולי", "אוג׳", "ספט׳", "אוק׳", "נוב׳", "דצמ׳"},
		days:  [7]string{"ראשון", "שני", "שלישי", "רביעי", "חמישי", "שישי", "שבת"},
		daysS: [7]string{"א׳", "ב׳", "ג׳", "ד׳", "ה׳", "ו׳", "ש׳"},
		dateShort: "d.M.yyyy", dateMedium: "d בMMM yyyy", dateLong: "EEEE, d בMMMM yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 0,
		currency: "ILS", curSymbol: "₪", decimal: ".", group: ",",
		listSep: ", ", listLast: " ו",
	},
	"hi": {
		name: "Hindi", native: "हिन्दी",
		months: [12]string{"जनवरी", "फ़रवरी", "मार्च", "अप्रैल", "मई", "जून",
			"जुलाई", "अगस्त", "सितंबर", "अक्टूबर", "नवंबर", "दिसंबर"},
		monthsS: [12]string{"जन॰", "फ़र॰", "मार्च", "अप्रैल", "मई", "जून",
			"जुल॰", "अग॰", "सित॰", "अक्तू॰", "नव॰", "दिस॰"},
		days:  [7]string{"रविवार", "सोमवार", "मंगलवार", "बुधवार", "गुरुवार", "शुक्रवार", "शनिवार"},
		daysS: [7]string{"रवि", "सोम", "मंगल", "बुध", "गुरु", "शुक्र", "शनि"},
		dateShort: "d/M/yyyy", dateMedium: "d MMM yyyy", dateLong: "EEEE, d MMMM yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 0,
		currency: "INR", curSymbol: "₹", decimal: ".", group: ",",
		listSep: ", ", listLast: " और ",
	},
	"th": {
		name: "Thai", native: "ไทย",
		months: [12]string{"มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
			"กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม"},
		monthsS: [12]string{"ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.",
			"ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."},
		days:  [7]string{"วันอาทิตย์", "วันจันทร์", "วันอังคาร", "วันพุธ", "วันพฤหัสบดี", "วันศุกร์", "วันเสาร์"},
		daysS: [7]string{"อา.", "จ.", "อ.", "พ.", "พฤ.", "ศ.", "ส."},
		dateShort: "d/M/yyyy", dateMedium: "d MMM yyyy", dateLong: "EEEE d MMMM yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 0,
		currency: "THB", curSymbol: "฿", decimal: ".", group: ",",
		listSep: ", ", listLast: " และ ",
	},
	"pt": {
		name: "Portuguese", native: "Português",
		months: [12]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho",
			"julho", "agosto", "setembro", "outubro", "novembro", "dezembro"},
		monthsS: [12]string{"jan.", "fev.", "mar.", "abr.", "mai.", "jun.",
			"jul.", "ago.", "set.", "out.", "nov.", "dez."},
		days:  [7]string{"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"},
		daysS: [7]string{"dom.", "seg.", "ter.", "qua.", "qui.", "sex.", "sáb."},
		dateShort: "dd/MM/yyyy", dateMedium: "d 'de' MMM 'de' yyyy", dateLong: "EEEE, d 'de' MMMM 'de' yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 0,
		currency: "BRL", curSymbol: "R$", decimal: ",", group: ".",
		listSep: ", ", listLast: " e ",
	},
	"it": {
		name: "Italian", native: "Italiano",
		months: [12]string{"gennaio", "febbraio", "marzo", "aprile", "maggio", "giugno",
			"luglio", "agosto", "settembre", "ottobre", "novembre", "dicembre"},
		monthsS: [12]string{"gen", "feb", "mar", "apr", "mag", "giu",
			"lug", "ago", "set", "ott", "nov", "dic"},
		days:  [7]string{"domenica", "lunedì", "martedì", "mercoledì", "giovedì", "venerdì", "sabato"},
		daysS: [7]string{"dom", "lun", "mar", "mer", "gio", "ven", "sab"},
		dateShort: "dd/MM/yyyy", dateMedium: "d MMM yyyy", dateLong: "EEEE d MMMM yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 1,
		currency: "EUR", curSymbol: "€", curSuffix: true,
		decimal: ",", group: ".", listSep: ", ", listLast: " e ",
	},
	"nl": {
		name: "Dutch", native: "Nederlands",
		months: [12]string{"januari", "februari", "maart", "april", "mei", "juni",
			"juli", "augustus", "september", "oktober", "november", "december"},
		monthsS: [12]string{"jan.", "feb.", "mrt.", "apr.", "mei", "jun.",
			"jul.", "aug.", "sep.", "okt.", "nov.", "dec."},
		days:  [7]string{"zondag", "maandag", "dinsdag", "woensdag", "donderdag", "vrijdag", "zaterdag"},
		daysS: [7]string{"zo", "ma", "di", "wo", "do", "vr", "za"},
		dateShort: "dd-MM-yyyy", dateMedium: "d MMM yyyy", dateLong: "EEEE d MMMM yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 1,
		currency: "EUR", curSymbol: "€", curSuffix: true,
		decimal: ",", group: ".", listSep: ", ", listLast: " en ",
	},
	"pl": {
		name: "Polish", native: "Polski",
		months: [12]string{"stycznia", "lutego", "marca", "kwietnia", "maja", "czerwca",
			"lipca", "sierpnia", "września", "października", "listopada", "grudnia"},
		monthsS: [12]string{"sty", "lut", "mar", "kwi", "maj", "cze",
			"lip", "sie", "wrz", "paź", "lis", "gru"},
		days:  [7]string{"niedziela", "poniedziałek", "wtorek", "środa", "czwartek", "piątek", "sobota"},
		daysS: [7]string{"niedz.", "pon.", "wt.", "śr.", "czw.", "pt.", "sob."},
		dateShort: "dd.MM.yyyy", dateMedium: "d MMM yyyy", dateLong: "EEEE, d MMMM yyyy",
		timeShort: "HH:mm", timeLong: "HH:mm:ss", firstDay: 1,
		currency: "PLN", curSymbol: "zł", curSuffix: true,
		decimal: ",", group: " ", listSep: ", ", listLast: " i ",
	},
}

// localeNativeName 地区化语言自称（覆盖常见「简体 / 繁体」等差异）。
var localeNativeName = map[string]string{
	"zh-CN": "中文（简体）", "zh-SG": "中文（新加坡）", "zh-TW": "中文（繁體）",
	"zh-HK": "中文（香港）", "zh-Hant": "中文（繁體）", "zh-Hans": "中文（简体）",
	"en-US": "English (US)", "en-GB": "English (UK)", "en-AU": "English (AU)",
	"pt-BR": "Português (Brasil)", "pt-PT": "Português (Portugal)",
	"es-ES": "Español (España)", "es-MX": "Español (México)",
}

// localeEnglishName 地区化英文名（日志 / 排序）。
var localeEnglishName = map[string]string{
	"zh-CN": "Chinese (Simplified)", "zh-TW": "Chinese (Traditional)",
	"zh-HK": "Chinese (Hong Kong)", "zh-Hant": "Chinese (Traditional)",
	"en-US": "English (United States)", "en-GB": "English (United Kingdom)",
	"pt-BR": "Portuguese (Brazil)", "es-MX": "Spanish (Mexico)",
}

// rtlLanguages RTL 语言（含常见变体）。
var rtlLanguages = map[string]bool{
	"ar": true, "he": true, "iw": true, "fa": true, "ur": true, "ps": true,
	"sd": true, "ug": true, "yi": true, "dv": true, "ckb": true, "ku": true,
	"nqo": true, "syr": true, "arc": true,
}

// localeDirection 语言 → 文字方向（"az-Arab" 这类脚本后缀也认）。
func localeDirection(code string) TextDirection {
	if code == "" {
		return DirLTR
	}
	if rtlLanguages[baseLanguage(code)] {
		return DirRTL
	}
	if strings.Contains(strings.ToLower(code), "-arab") || strings.Contains(strings.ToLower(code), "-hebr") {
		return DirRTL
	}
	return DirLTR
}

// deriveLocaleInfo 按语言代码派生一份完整元数据。
func deriveLocaleInfo(code string) LocaleInfo {
	base := baseLanguage(code)
	d, ok := langTable[base]
	if !ok {
		d = langTable["en"]
	}
	name, native := d.name, d.native
	if v, ok := localeEnglishName[code]; ok {
		name = v
	}
	if v, ok := localeNativeName[code]; ok {
		native = v
	} else if v, ok := localeNativeName[base]; ok {
		native = v
	}
	if base == "zh" {
		// 繁体地区默认用繁体写法（月份 / 星期名与简体相同，仅自称不同）
		if strings.Contains(code, "Hant") || strings.HasSuffix(code, "-TW") || strings.HasSuffix(code, "-HK") {
			native = localeNativeName["zh-TW"]
		}
	}
	info := LocaleInfo{
		Code:           code,
		Name:           name,
		NativeName:     native,
		Dir:            localeDirection(code),
		DateShort:      d.dateShort,
		DateMedium:     d.dateMedium,
		DateLong:       d.dateLong,
		TimeShort:      d.timeShort,
		TimeLong:       d.timeLong,
		Months:         d.months,
		MonthsShort:    d.monthsS,
		Weekdays:       d.days,
		WeekdaysShort:  d.daysS,
		FirstWeekday:   d.firstDay,
		AM:             d.am,
		PM:             d.pm,
		Currency:       d.currency,
		CurrencySymbol: d.curSymbol,
		CurrencySuffix: d.curSuffix,
		PercentSpace:   d.pctSpace,
		Decimal:        d.decimal,
		Group:          d.group,
		ListSep:        d.listSep,
		ListLastSep:    d.listLast,
	}
	// 数字符号随地区调整（de-CH「1'234.50」、es-ES「1.234,50」等常用差异）。
	switch {
	case strings.HasPrefix(code, "de-CH"):
		info.Group = "'"
	case strings.HasPrefix(code, "en-IN"):
		info.Group = ","
	case strings.HasPrefix(code, "fr-CH"):
		info.Group = " "
	}
	// 货币随地区调整（只覆盖最常见的几组，其余沿用基础语言）。
	switch {
	case code == "zh-CN" || code == "zh-SG":
		info.Currency, info.CurrencySymbol, info.CurrencySuffix = "CNY", "¥", false
	case code == "zh-TW" || code == "zh-HK" || code == "zh-Hant":
		info.Currency, info.CurrencySymbol, info.CurrencySuffix = "TWD", "NT$", false
	case code == "en-GB":
		info.Currency, info.CurrencySymbol, info.CurrencySuffix = "GBP", "£", false
	case strings.HasPrefix(code, "en-CA"):
		info.Currency, info.CurrencySymbol = "CAD", "CA$"
	case strings.HasPrefix(code, "en-AU"):
		info.Currency, info.CurrencySymbol = "AUD", "A$"
	case code == "ar-SA":
		info.Currency, info.CurrencySymbol = "SAR", "ر.س"
	case code == "ar-AE":
		info.Currency, info.CurrencySymbol = "AED", "د.إ"
	}
	return info
}

// mergeLocaleInfo 用 base 补齐 dst 的零值字段（业务只填差异即可）。
func mergeLocaleInfo(dst *LocaleInfo, base LocaleInfo) {
	if dst.Name == "" {
		dst.Name = base.Name
	}
	if dst.NativeName == "" {
		dst.NativeName = base.NativeName
	}
	if dst.DateShort == "" {
		dst.DateShort = base.DateShort
	}
	if dst.DateMedium == "" {
		dst.DateMedium = base.DateMedium
	}
	if dst.DateLong == "" {
		dst.DateLong = base.DateLong
	}
	if dst.TimeShort == "" {
		dst.TimeShort = base.TimeShort
	}
	if dst.TimeLong == "" {
		dst.TimeLong = base.TimeLong
	}
	for i := 0; i < 12; i++ {
		if dst.Months[i] == "" {
			dst.Months[i] = base.Months[i]
		}
		if dst.MonthsShort[i] == "" {
			dst.MonthsShort[i] = base.MonthsShort[i]
		}
	}
	for i := 0; i < 7; i++ {
		if dst.Weekdays[i] == "" {
			dst.Weekdays[i] = base.Weekdays[i]
		}
		if dst.WeekdaysShort[i] == "" {
			dst.WeekdaysShort[i] = base.WeekdaysShort[i]
		}
	}
	if dst.AM == "" {
		dst.AM = base.AM
	}
	if dst.PM == "" {
		dst.PM = base.PM
	}
	if dst.FirstWeekday == 0 {
		dst.FirstWeekday = base.FirstWeekday
	}
	if dst.Dir == DirLTR {
		dst.Dir = base.Dir
	}
	if !dst.CurrencySuffix {
		dst.CurrencySuffix = base.CurrencySuffix
	}
	if !dst.PercentSpace {
		dst.PercentSpace = base.PercentSpace
	}
	if dst.Currency == "" {
		dst.Currency = base.Currency
	}
	if dst.CurrencySymbol == "" {
		dst.CurrencySymbol = base.CurrencySymbol
	}
	if dst.Decimal == "" {
		dst.Decimal = base.Decimal
	}
	if dst.Group == "" {
		dst.Group = base.Group
	}
	if dst.ListSep == "" {
		dst.ListSep = base.ListSep
	}
	if dst.ListLastSep == "" {
		dst.ListLastSep = base.ListLastSep
	}
}
