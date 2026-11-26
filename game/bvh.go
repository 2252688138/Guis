// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"math"
	"sort"
)

// bvhNode 层次包围盒节点（叶节点 left = -1）。
type bvhNode struct {
	bounds       AABB
	left, right  int32
	start, count int32
}

// BVH 三角形层次包围盒：加速射线拾取（拾取开销从 O(n) 降到 O(log n)）。
//
// 构建策略是最长轴中位数分割（SAH 的简化版）：对绝大多数场景足够，
// 而构建开销与三角形数同阶、可一次性完成后复用。
type BVH struct {
	nodes []bvhNode
	order []int32
	tri   [][3]Vec3
	src   []int32
}

const bvhLeafSize = 8

// NewBVH 为网格构建 BVH（三角形少于 2 个时仍能工作）。
func NewBVH(m *Mesh) *BVH {
	if m == nil || len(m.Indices) < 3 {
		return nil
	}
	n := len(m.Indices) / 3
	b := &BVH{
		tri:   make([][3]Vec3, 0, n),
		src:   make([]int32, 0, n),
		order: make([]int32, 0, n),
	}
	for i := 0; i < n; i++ {
		ia, ib, ic := int(m.Indices[3*i]), int(m.Indices[3*i+1]), int(m.Indices[3*i+2])
		if ia >= len(m.Positions) || ib >= len(m.Positions) || ic >= len(m.Positions) {
			continue
		}
		b.tri = append(b.tri, [3]Vec3{m.Positions[ia], m.Positions[ib], m.Positions[ic]})
		b.src = append(b.src, int32(i))
		b.order = append(b.order, int32(len(b.tri)-1))
	}
	if len(b.tri) == 0 {
		return nil
	}
	b.nodes = make([]bvhNode, 0, 2*len(b.tri)/bvhLeafSize+1)
	b.build(0, int32(len(b.tri)), 0)
	return b
}

// centroid 三角形质心（分区依据）。
func (b *BVH) centroid(i int32) Vec3 {
	t := b.tri[i]
	return t[0].Add(t[1]).Add(t[2]).Scale(1.0 / 3)
}

func (b *BVH) boundsOf(start, count int32) AABB {
	b0 := b.tri[b.order[start]][0]
	box := AABB{Min: b0, Max: b0}
	for i := start; i < start+count; i++ {
		for _, v := range b.tri[b.order[i]] {
			box.Min = box.Min.Min(v)
			box.Max = box.Max.Max(v)
		}
	}
	return box
}

// axisOf 取分量（axis：0=X / 1=Y / 2=Z）。
func axisOf(v Vec3, axis int) float32 {
	switch axis {
	case 1:
		return v.Y
	case 2:
		return v.Z
	}
	return v.X
}

// build 递归建树，返回节点索引。
func (b *BVH) build(start, count int32, depth int) int32 {
	idx := int32(len(b.nodes))
	b.nodes = append(b.nodes, bvhNode{bounds: b.boundsOf(start, count), left: -1, start: start, count: count})
	if count <= bvhLeafSize || depth >= 48 {
		return idx
	}
	// 最长轴分割
	box := b.nodes[idx].bounds
	ext := box.Max.Sub(box.Min)
	axis := 0
	if ext.Y > ext.X {
		axis = 1
	}
	if ext.Z > axisOf(ext, axis) {
		axis = 2
	}
	seg := b.order[start : start+count]
	sort.Slice(seg, func(i, j int) bool {
		return axisOf(b.centroid(seg[i]), axis) < axisOf(b.centroid(seg[j]), axis)
	})
	mid := count / 2
	leftIdx := b.build(start, mid, depth+1)
	rightIdx := b.build(start+mid, count-mid, depth+1)
	b.nodes[idx].left = leftIdx
	b.nodes[idx].right = rightIdx
	b.nodes[idx].start = 0
	b.nodes[idx].count = 0
	return idx
}

// bvhEnter 射线进入 AABB 的距离（起点在盒内返回 0）。
//
// 不用 RayAABB：它在起点位于盒内时返回出口距离，拿来剪枝会把"盒子在眼前"
// 的子树误剪掉。
func bvhEnter(origin, dir Vec3, b AABB) (float32, bool) {
	tmin := float32(math.Inf(-1))
	tmax := float32(math.Inf(1))
	for axis := 0; axis < 3; axis++ {
		var o, d, lo, hi float32
		switch axis {
		case 0:
			o, d, lo, hi = origin.X, dir.X, b.Min.X, b.Max.X
		case 1:
			o, d, lo, hi = origin.Y, dir.Y, b.Min.Y, b.Max.Y
		default:
			o, d, lo, hi = origin.Z, dir.Z, b.Min.Z, b.Max.Z
		}
		if float32(math.Abs(float64(d))) < 1e-8 {
			if o < lo || o > hi {
				return 0, false
			}
			continue
		}
		t1 := (lo - o) / d
		t2 := (hi - o) / d
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
		if tmin > tmax {
			return 0, false
		}
	}
	if tmax < 0 {
		return 0, false
	}
	if tmin < 0 {
		return 0, true
	}
	return tmin, true
}

// NodeCount 节点数（诊断用：≈ 2 * 叶数）。
func (b *BVH) NodeCount() int {
	if b == nil {
		return 0
	}
	return len(b.nodes)
}

// TriCount 三角形总数。
func (b *BVH) TriCount() int {
	if b == nil {
		return 0
	}
	return len(b.tri)
}

// Ray 射线与网格求交（语义与 RayMesh 完全一致，但只访问可能命中的叶节点）。
func (b *BVH) Ray(origin, dir Vec3, model Mat4) (float32, int, bool) {
	if b == nil || len(b.nodes) == 0 {
		return 0, -1, false
	}
	inv, ok := model.Invert()
	if !ok {
		return 0, -1, false
	}
	o := inv.TransformPoint(origin)
	d := inv.TransformDir(dir)
	best := float32(math.MaxFloat32)
	bestTri := -1

	var stack [64]int32
	sp := 1
	stack[0] = 0
	for sp > 0 {
		sp--
		n := &b.nodes[stack[sp]]
		if tEnter, hit := bvhEnter(o, d, n.bounds); !hit || tEnter > best {
			continue
		}
		if n.left < 0 {
			for i := int32(0); i < n.count; i++ {
				ti := b.order[n.start+i]
				r := b.tri[ti]
				if t, hit := RayTriangle(o, d, r[0], r[1], r[2]); hit && t < best {
					best = t
					bestTri = int(b.src[ti])
				}
			}
			continue
		}
		if sp+2 <= len(stack) {
			stack[sp] = n.left
			sp++
			stack[sp] = n.right
			sp++
		}
	}
	if bestTri < 0 {
		return 0, -1, false
	}
	scale := d.Len()
	if scale <= 1e-9 {
		return best, bestTri, true
	}
	return best / scale * dir.Len(), bestTri, true
}
