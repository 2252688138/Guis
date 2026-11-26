// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "sync"

// bvhBuildMu 保护网格 BVH 缓存的构建（场景可能被多个协程同时拾取）。
var bvhBuildMu sync.Mutex

// BVH 返回惰性构建的三角形加速结构（第一次调用时构建，之后复用）。
//
// 注意：这两个字段在 Mesh 值复制时共享同一指针；修改 Positions / Indices
// 之后必须调用 InvalidateBVH()，否则拾取会用到旧几何。
func (m *Mesh) BVH() *BVH {
	if m == nil {
		return nil
	}
	bvhBuildMu.Lock()
	defer bvhBuildMu.Unlock()
	if m.bvh == nil {
		m.bvh = NewBVH(m)
	}
	return m.bvh
}

// InvalidateBVH 清掉缓存的 BVH（网格几何变更后必须调用）。
func (m *Mesh) InvalidateBVH() {
	if m == nil {
		return
	}
	bvhBuildMu.Lock()
	m.bvh = nil
	bvhBuildMu.Unlock()
}
