//go:build js

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "syscall/js"

// init 挂载 Web 端系统语言探测：navigator.languages（按浏览器偏好顺序），
// 老浏览器没有 languages 时退回 navigator.language。
func init() {
	systemLangProbe = func() []string {
		nav := js.Global().Get("navigator")
		if nav.IsUndefined() || nav.IsNull() {
			return nil
		}
		out := make([]string, 0, 3)
		if langs := nav.Get("languages"); langs.Truthy() && langs.Length() > 0 {
			for i := 0; i < langs.Length(); i++ {
				if v := langs.Index(i).String(); v != "" {
					out = append(out, v)
				}
			}
		}
		if v := nav.Get("language").String(); v != "" && v != "<undefined>" {
			out = append(out, v)
		}
		return out
	}
}
