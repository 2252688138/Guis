//go:build js

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 浏览器端没有「多实例」概念（每个标签页独立），单实例接口一律视为首实例，
// 参数投递为空操作。能力位 CapSingleInstance 在 Web 端同样为假。

// SingleInstance 浏览器端恒为首个实例。
func SingleInstance(key string) bool { return true }

// NotifyFirstInstance 浏览器端空实现。
func NotifyFirstInstance(key string, args []string) {}

// WatchSecondInstance 浏览器端空实现。
func WatchSecondInstance(key string, fn func(args []string)) (stop func()) { return func() {} }
