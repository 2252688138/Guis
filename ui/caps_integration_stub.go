//go:build !windows && !js && !android && !ios && !linux && !freebsd && !darwin

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// platformIntegrationCaps 其余类 Unix 系统（OpenBSD / NetBSD 等）：
// 目前只有占位实现，因此不报告热键 / 托盘 / 拖入能力，业务侧会走降级分支。
func platformIntegrationCaps() []Cap { return nil }
