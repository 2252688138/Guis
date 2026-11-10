//go:build !windows && !js && !android && !ios && !linux && !freebsd && !darwin

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 其余类 Unix（OpenBSD / NetBSD 等）：暂无电源状态读取实现。
func powerCaps() []Cap { return nil }
