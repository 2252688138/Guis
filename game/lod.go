// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// LODLevel 一个细节层级：相机距离 ≤ Distance 时使用 Mesh。
//
// Node.LODs 需按 Distance 升序排列（最近的在前），最后一项作为兜底；
// Node.Mesh 作为没有任何 LOD 命中时的默认网格。
type LODLevel struct {
	Distance float32
	Mesh     *Mesh
}

// PickLOD 按相机距离选层级（dist 单位与场景一致，即米）。
func (n *Node) PickLOD(dist float32) *Mesh {
	if n == nil {
		return nil
	}
	if len(n.LODs) == 0 {
		return n.Mesh
	}
	for _, l := range n.LODs {
		if l.Mesh != nil && dist <= l.Distance {
			return l.Mesh
		}
	}
	// 超出最远层级：从后往前取第一个有效网格
	for i := len(n.LODs) - 1; i >= 0; i-- {
		if n.LODs[i].Mesh != nil {
			return n.LODs[i].Mesh
		}
	}
	return n.Mesh
}
