// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os"
	"strings"
	"sync"
)

// 系统语言探测（多端适配）。
//
// 各端能拿到的信息不一样，统一收敛成「按优先级排列的语言列表」：
//
//	Windows  GetUserDefaultUILanguage → zh-CN
//	Linux    LC_ALL / LANG / LANGUAGE 环境变量（zh_CN.UTF-8 → zh-CN）
//	macOS    defaults read -g AppleLocale，取不到再看环境变量
//	Web      navigator.languages / navigator.language
//	Android  环境变量（无 JNI 桥时）；原生壳可用 SetSystemLanguages 注入
//	iOS      环境变量；原生壳可用 SetSystemLanguages 注入
//
// 业务启动时调一次 ui.AutoLocale() 即可完成「偏好 > 系统 > 兜底」的选择；
// 想自己决定顺序就先用 SystemLanguages() 拿到系统语言再 MatchLocale。

var (
	systemLangMu sync.RWMutex
	// systemLangOverride 业务注入的系统语言（移动端原生壳 / 测试用）。
	systemLangOverride []string
	// systemLangCache 系统语言缓存（探测要读环境变量 / 起子进程，只做一次）。
	systemLangCache []string
	systemLangDone  bool
	// systemLangProbe 平台探测钩子（各平台 init 里挂载）。
	systemLangProbe func() []string
)

// SetSystemLanguages 注入系统语言列表（按优先级）。
//
// 移动端没有 JNI / ObjC 桥时，原生壳可以把系统语言传进来；传空则恢复自动探测。
func SetSystemLanguages(codes ...string) {
	systemLangMu.Lock()
	systemLangOverride = systemLangOverride[:0]
	for _, c := range codes {
		if v := canonicalLocale(c); v != "" {
			systemLangOverride = append(systemLangOverride, v)
		}
	}
	systemLangCache = nil
	systemLangDone = false
	systemLangMu.Unlock()
}

// RefreshSystemLanguages 清空缓存，下次 SystemLanguages 重新探测（用户在系统里改了语言）。
func RefreshSystemLanguages() {
	systemLangMu.Lock()
	systemLangCache = nil
	systemLangDone = false
	systemLangMu.Unlock()
}

// SystemLanguages 系统语言列表（按优先级；最差返回 ["en-US"]）。
func SystemLanguages() []string {
	systemLangMu.RLock()
	if len(systemLangOverride) > 0 {
		out := append([]string{}, systemLangOverride...)
		systemLangMu.RUnlock()
		return out
	}
	if systemLangDone && len(systemLangCache) > 0 {
		out := append([]string{}, systemLangCache...)
		systemLangMu.RUnlock()
		return out
	}
	probe := systemLangProbe
	systemLangMu.RUnlock()

	out := make([]string, 0, 6)
	add := func(v string) {
		if c := canonicalLocale(v); c != "" {
			for _, x := range out {
				if x == c {
					return
				}
			}
			out = append(out, c)
		}
	}
	if probe != nil {
		for _, v := range probe() {
			add(v)
		}
	}
	// 环境变量（Linux / macOS / 部分移动端容器会设置）
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		if v := os.Getenv(k); v != "" {
			// LANGUAGE 可能是 "zh_CN:en_US" 这样的列表
			for _, part := range strings.Split(v, ":") {
				if c := envLocaleToTag(part); c != "" {
					add(c)
				}
			}
		}
	}
	// 移动端 / Web 没有环境变量时给个安全默认
	if len(out) == 0 {
		out = append(out, "en-US")
	}
	systemLangMu.Lock()
	systemLangCache = out
	systemLangDone = true
	systemLangMu.Unlock()
	return append([]string{}, out...)
}

// SystemLocaleTag 系统首选语言（等价 SystemLanguages()[0]）。
func SystemLocaleTag() string {
	list := SystemLanguages()
	if len(list) == 0 {
		return "en-US"
	}
	return list[0]
}

// envLocaleToTag 环境变量里的语言写法转 BCP-47（zh_CN.UTF-8 → zh-CN）。
func envLocaleToTag(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if i := strings.IndexByte(v, '.'); i > 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '@'); i > 0 {
		v = v[:i]
	}
	if v == "C" || v == "POSIX" {
		return ""
	}
	return canonicalLocale(v)
}
