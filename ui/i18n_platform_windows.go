//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// init 挂载 Windows 系统语言探测：GetUserDefaultUILanguage → LCIDToLocaleName
// （见 internal/win32/sysops.go 的 SysInfo，这里只是把结果接进 SystemLanguages）。
func init() {
	systemLangProbe = func() []string {
		if l := SystemInfo().Locale; l != "" {
			return []string{l}
		}
		return nil
	}
}
