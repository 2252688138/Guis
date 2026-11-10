// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 语言切换组件。
//
//	// 设置页里一行：语言 / 中文（简体）| English
//	LocaleSwitcher("set-lang")
//
//	// 自己控制外观：拿到选项后交给 Segmented / 菜单
//	codes, labels := LocaleOptions()
//
// 切换走 UseLocale（写偏好 ui.locale），重启后 AutoLocale 会恢复选择；
// 界面每帧重建，切完自动重绘，不需要额外刷新。

// LocaleOptions 语言切换器的选项（代码 + 本地名称）。
//
// 只包含随包或业务载入过字符串表的语言；「只有元数据」的语言（日期格式正确但
// 没有译文）不会出现在这里，避免用户选中后看到兜底语言的文案。
func LocaleOptions() (codes, labels []string) {
	infos := AvailableLocales()
	codes = make([]string, 0, len(infos))
	labels = make([]string, 0, len(infos))
	for _, in := range infos {
		code := in.Code
		label := in.NativeName
		if label == "" {
			label = in.Name
		}
		if label == "" {
			label = code
		}
		codes = append(codes, code)
		labels = append(labels, label)
	}
	return codes, labels
}

// LocaleLabel 某语言的展示名（本地写法，缺失时退回英文名 / 代码）。
func LocaleLabel(locale string) string {
	if info, ok := LocaleInfoOf(locale); ok {
		if info.NativeName != "" {
			return info.NativeName
		}
		if info.Name != "" {
			return info.Name
		}
	}
	return locale
}

// LocaleSwitcher 现成的语言切换器（横向胶囊按钮，选中项高亮）。
//
// key 用于区分不同位置的实例（同一页面多处出现也不冲突）。语言少于两种时
// 返回 nil（单语言应用不必显示切换入口）。
func LocaleSwitcher(key string) *Node {
	codes, labels := LocaleOptions()
	if len(codes) < 2 {
		return nil
	}
	cur := Locale()
	chips := make([]*Node, 0, len(codes))
	for i, code := range codes {
		c := code
		label := labels[i]
		chips = append(chips, Chip(label, c == cur, func() {
			UseLocale(c)
		}).K(key + "-" + c))
	}
	return Wrap(chips...).St(St().GapOf(8)).AlignTo(AlignStart).K(key)
}

// LocaleSegmented 语言切换器（分段控件，适合 2~4 种语言放一行）。
func LocaleSegmented(key string) *Node {
	codes, labels := LocaleOptions()
	if len(codes) < 2 {
		return nil
	}
	cur := Locale()
	if cur == "" && len(codes) > 0 {
		cur = codes[0]
	}
	if _, ok := LocaleInfoOf(cur); !ok && len(codes) > 0 {
		cur = codes[0]
	}
	n := Segmented(codes, labels, cur, func(v string) {
		UseLocale(v)
	})
	if n != nil {
		n.K(key)
	}
	return n
}

// LocaleSettingsRow 设置页里的一行：「语言 + 切换器」。
func LocaleSettingsRow(key string) *Node {
	sw := LocaleSwitcher(key + "-sw")
	if sw == nil {
		return nil
	}
	return Column(
		Text(T("ui.locale.switch")).St(St().Text(13, 600, ColText)),
		Box().St(St().H(8)),
		sw,
	).St(St().GapOf(2)).K(key)
}
