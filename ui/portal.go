// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "sort"

// 窗口根层浮层（portal）
//
// 组件树里的绝对定位（St().AbsFill() 等）以「最近的父容器」为包含块，并且会被
// 祖先的 Clipped() / 滚动容器裁剪。这对「全屏」这类必须铺满整窗、又要跳出固定
// 高度面板与裁剪区的浮层是不够的：播放器嵌在「400px 高 + Clipped」的面板里时，
// 全屏只能铺满那块面板。
//
// portal 提供一个逃逸通道：组件在构建期用 Portal(key, node) 把节点登记到窗口根层，
// App 在根层（普通内容之上、任何裁剪区之外）渲染它。登记只对当前帧有效 —— App
// 每帧调用 view() 之前会清空登记表，所以组件只要在构建时登记一次即可。
//
// 注意：登记必须发生在 view() 构建期；在事件回调里登记的节点会在下一帧被清掉。
var portals = map[string]*Node{}

// Portal 把 n 登记到窗口根层渲染（同 key 覆盖；n 为 nil 表示移除）。
// 典型用法：全屏视频、全屏图片预览、自定义模态层。
func Portal(key string, n *Node) {
	if key == "" {
		return
	}
	if n == nil {
		delete(portals, key)
		return
	}
	portals[key] = n
}

// PortalRemove 移除一个已登记的窗口级浮层。
func PortalRemove(key string) { delete(portals, key) }

// PortalActive 当前是否有窗口级浮层在渲染。
func PortalActive() bool { return len(portals) > 0 }

// PortalKeys 返回本帧登记的全部 key（已排序，便于调试与测试）。
func PortalKeys() []string { return keysOf(portals) }

// portalReset 清空本帧登记（App.frame 在调用 view() 之前调用）。
func portalReset() {
	for k := range portals {
		delete(portals, k)
	}
}

// portalList 按 key 排序返回本帧登记的节点，保证叠放顺序稳定。
func portalList() []*Node {
	if len(portals) == 0 {
		return nil
	}
	ks := keysOf(portals)
	out := make([]*Node, 0, len(ks))
	for _, k := range ks {
		if n := portals[k]; n != nil {
			out = append(out, n)
		}
	}
	return out
}

// portalWrap 把根视图与窗口级浮层合成新的根：浮层挂在根 Stack 里，
// 包含块是整窗，不再受普通内容的布局与裁剪影响。
func portalWrap(root *Node) *Node {
	ps := portalList()
	if len(ps) == 0 || root == nil {
		return root
	}
	kids := make([]*Node, 0, len(ps)+1)
	kids = append(kids, root)
	kids = append(kids, ps...)
	return Stack(kids...).St(St().Wp(100).Hp(100)).K("ui-portal-root")
}

// keysOf 取 map 的 key 并排序。
func keysOf(m map[string]*Node) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
