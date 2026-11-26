// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

const (
	rigSlop       = 0.005 // 允许的穿透余量
	rigBaumgarte  = 0.2   // 位置偏置系数
	rigMaxBias    = 4.0   // 位置偏置速度上限（m/s）
	rigRestCut    = 1.0   // 弹性生效的最小接近速度（m/s）
	rigSleepLin   = 0.08  // 休眠线速度阈值
	rigSleepAng   = 0.12  // 休眠角速度阈值
	rigSleepTime  = 0.5   // 休眠停留时间
	rigMargin     = 0.02  // 宽相 AABB 外扩
	rigEdgePar    = 1e-4  // 棱-棱轴叉积长度平方阈值（低于此视为近平行，跳过）
	rigEdgeBias   = 0.002 // 棱-棱轴相对面轴的采纳偏置（米）
	rigFixedStep  = 1.0 / 120.0
	rigMaxSubStep = 8
)

// RigidStats 刚体世界统计。
type RigidStats struct {
	Bodies     int
	Contacts   int
	Iterations int
}

// RigidWorld 刚体世界（固定子步 + 顺序冲量求解）。
type RigidWorld struct {
	// Gravity 重力加速度（世界空间）。
	Gravity Vec3

	bodies   []*RigidBody
	joints   []*Joint
	contacts []Contact

	iterations int
	subStep    float32
	maxSub     int

	warm map[rigPairKey]rigWarmImpulse
}

// rigPairKey 热启动缓存键。
type rigPairKey struct {
	a, b int
	f    uint8
}

// rigWarmImpulse 热启动缓存的累积冲量。
type rigWarmImpulse struct {
	n, t1, t2 float32
}

// NewRigidWorld 构造刚体世界。
func NewRigidWorld(gravity Vec3) *RigidWorld {
	return &RigidWorld{
		Gravity:    gravity,
		iterations: 10,
		subStep:    rigFixedStep,
		maxSub:     rigMaxSubStep,
		warm:       make(map[rigPairKey]rigWarmImpulse),
	}
}

// Add 添加刚体并返回它。
func (w *RigidWorld) Add(b *RigidBody) *RigidBody {
	if b == nil {
		return nil
	}
	for _, o := range w.bodies {
		if o == b {
			return b
		}
	}
	b.idx = len(w.bodies)
	b.world = w
	b.Awake = true
	w.bodies = append(w.bodies, b)
	return b
}

// Remove 移除刚体（同时移除与其相连的关节）。
func (w *RigidWorld) Remove(b *RigidBody) {
	if b == nil {
		return
	}
	for i, o := range w.bodies {
		if o != b {
			continue
		}
		w.bodies = append(w.bodies[:i], w.bodies[i+1:]...)
		break
	}
	for i := 0; i < len(w.joints); {
		if w.joints[i].A == b || w.joints[i].B == b {
			w.joints = append(w.joints[:i], w.joints[i+1:]...)
			continue
		}
		i++
	}
	for i, o := range w.bodies {
		o.idx = i
	}
	b.world = nil
	b.idx = -1
}

// AddJoint 添加关节并返回它。
func (w *RigidWorld) AddJoint(j *Joint) *Joint {
	if j == nil {
		return nil
	}
	j.world = w
	w.joints = append(w.joints, j)
	return j
}

// RemoveJoint 移除关节。
func (w *RigidWorld) RemoveJoint(j *Joint) {
	for i, o := range w.joints {
		if o == j {
			w.joints = append(w.joints[:i], w.joints[i+1:]...)
			return
		}
	}
}

// SetIterations 设置顺序冲量迭代次数（至少 1）。
func (w *RigidWorld) SetIterations(n int) {
	if n < 1 {
		n = 1
	}
	if n > 100 {
		n = 100
	}
	w.iterations = n
}

// Contacts 返回上一子步的接触列表（副本）。
func (w *RigidWorld) Contacts() []Contact {
	out := make([]Contact, len(w.contacts))
	copy(out, w.contacts)
	return out
}

// Stats 返回世界统计信息。
func (w *RigidWorld) Stats() RigidStats {
	return RigidStats{Bodies: len(w.bodies), Contacts: len(w.contacts), Iterations: w.iterations}
}

// Step 推进世界 dt 秒（内部按 1/120s 切成最多 8 个子步）。
func (w *RigidWorld) Step(dt float32) {
	if dt <= 0 || len(w.bodies) == 0 {
		return
	}
	n := int(math.Ceil(float64(dt / w.subStep)))
	if n < 1 {
		n = 1
	}
	if n > w.maxSub {
		n = w.maxSub
	}
	h := dt / float32(n)
	for i := 0; i < n; i++ {
		w.subStepOnce(h)
	}
}

// subStepOnce 单个固定子步。
func (w *RigidWorld) subStepOnce(h float32) {
	w.integrateForces(h)
	w.generateContacts()
	w.prepareContacts(h)
	w.warmStart()
	for i := 0; i < w.iterations; i++ {
		w.solveContacts()
	}
	w.prepareJoints(h)
	for i := 0; i < w.iterations; i++ {
		w.solveJoints()
	}
	w.integrateVelocities(h)
	w.correctPositions()
	w.correctJoints()
	w.updateSleep(h)
	w.cacheWarm()
}

// integrateForces 积分外力（重力 + 累积力/力矩）并施加阻尼。
func (w *RigidWorld) integrateForces(h float32) {
	for _, b := range w.bodies {
		if b.InvMass == 0 {
			b.ClearForces()
			continue
		}
		if !b.Awake {
			b.ClearForces()
			continue
		}
		acc := w.Gravity.Add(b.force.Scale(b.InvMass))
		b.Vel = b.Vel.Add(acc.Scale(h))
		if b.torque != (Vec3{}) {
			b.AngVel = b.AngVel.Add(rigInvInertiaMul(b, b.torque).Scale(h))
		}
		b.Vel = b.Vel.Scale(1 / (1 + h*b.LinearDamping))
		b.AngVel = b.AngVel.Scale(1 / (1 + h*b.AngularDamping))
		b.ClearForces()
	}
}

// integrateVelocities 用速度积分位置与姿态（四元数每步归一化）。
func (w *RigidWorld) integrateVelocities(h float32) {
	for _, b := range w.bodies {
		if b.InvMass == 0 || !b.Awake {
			continue
		}
		b.Pos = b.Pos.Add(b.Vel.Scale(h))
		b.Rot = rigQuatIntegrate(b.Rot, b.AngVel, h)
	}
}

// ======================= 宽相 + 窄相 =======================

// generateContacts 宽相筛选 + 窄相生成接触流形。
func (w *RigidWorld) generateContacts() {
	w.contacts = w.contacts[:0]
	n := len(w.bodies)
	if n == 0 {
		return
	}
	// 缓存 AABB，避免 O(n²) 重复计算
	boxes := make([]AABB, n)
	for i, b := range w.bodies {
		b.touching = false
		boxes[i] = b.AABB().Expanded(rigMargin)
	}
	// 平面与任意非平面体配对（平面 AABB 无意义，直接全配对）
	for i, p := range w.bodies {
		if p.Shape.Kind != ShapePlane {
			continue
		}
		for j, b := range w.bodies {
			if i == j || b.Shape.Kind == ShapePlane {
				continue
			}
			if p.InvMass == 0 && !b.Awake {
				continue
			}
			rigCollide(p, b, &w.contacts)
		}
	}
	// 非平面体两两 AABB 粗筛
	for i := 0; i < n; i++ {
		a := w.bodies[i]
		if a.Shape.Kind == ShapePlane {
			continue
		}
		for j := i + 1; j < n; j++ {
			b := w.bodies[j]
			if b.Shape.Kind == ShapePlane {
				continue
			}
			if !a.Awake && !b.Awake {
				continue
			}
			if a.InvMass == 0 && b.InvMass == 0 {
				continue
			}
			if (a.InvMass == 0 && !b.Awake) || (b.InvMass == 0 && !a.Awake) {
				continue
			}
			if !boxes[i].Overlaps(boxes[j]) {
				continue
			}
			rigCollide(a, b, &w.contacts)
		}
	}
	// 唤醒与活动体接触的休眠体
	for i := range w.contacts {
		c := &w.contacts[i]
		if !c.A.Awake && c.B.InvMass > 0 && c.B.Awake {
			c.A.Wake()
		}
		if !c.B.Awake && c.A.InvMass > 0 && c.A.Awake {
			c.B.Wake()
		}
		c.A.touching = true
		c.B.touching = true
	}
}

// rigCollide 形状对分发（生成的法线均为 A→B 方向）。
func rigCollide(a, b *RigidBody, out *[]Contact) {
	ka, kb := a.Shape.Kind, b.Shape.Kind
	if ka == ShapePlane {
		if kb == ShapePlane {
			return
		}
		rigPlaneShape(a, b, false, out)
		return
	}
	if kb == ShapePlane {
		rigPlaneShape(b, a, true, out)
		return
	}
	switch {
	case ka == ShapeSphere && kb == ShapeSphere:
		rigSphereSphere(a, b, out)
	case ka == ShapeSphere && kb == ShapeBox:
		rigSphereBox(a, b, true, out)
	case ka == ShapeBox && kb == ShapeSphere:
		rigSphereBox(b, a, false, out)
	case ka == ShapeBox && kb == ShapeBox:
		rigBoxBox(a, b, out)
	case ka == ShapeSphere && kb == ShapeCapsule:
		rigSphereCapsule(a, b, true, out)
	case ka == ShapeCapsule && kb == ShapeSphere:
		rigSphereCapsule(b, a, false, out)
	case ka == ShapeCapsule && kb == ShapeCapsule:
		rigCapsuleCapsule(a, b, out)
	}
}

// rigSphereSphere 球-球。
func rigSphereSphere(a, b *RigidBody, out *[]Contact) {
	d := b.Pos.Sub(a.Pos)
	dist := d.Len()
	ra, rb := a.Shape.Radius, b.Shape.Radius
	if dist-rb-ra > rigMargin {
		return
	}
	var n Vec3
	if dist > 1e-5 {
		n = d.Scale(1 / dist)
	} else {
		n = V3(0, 1, 0)
	}
	c := Contact{A: a, B: b, Normal: n, Depth: ra + rb - dist, feat: 0}
	c.Point = a.Pos.Add(n.Scale(ra - c.Depth*0.5))
	*out = append(*out, c)
}

// rigSphereBox 球-盒（OBB）。sphereIsA 表示球是 A 侧。
func rigSphereBox(sphere, box *RigidBody, sphereIsA bool, out *[]Contact) {
	h := box.Shape.Half
	r := sphere.Shape.Radius
	sc := box.LocalPoint(sphere.Pos)
	q := V3(clampF(sc.X, -h.X, h.X), clampF(sc.Y, -h.Y, h.Y), clampF(sc.Z, -h.Z, h.Z))
	delta := sc.Sub(q)
	dist := delta.Len()

	var nLocal Vec3 // 盒 → 球
	var depth float32
	var point Vec3
	if dist > 1e-5 {
		if dist-r > rigMargin {
			return
		}
		nLocal = delta.Scale(1 / dist)
		depth = r - dist
		point = box.WorldPoint(q)
	} else {
		// 球心在盒内：沿最浅轴推出
		dx := h.X - rigAbs(sc.X)
		dy := h.Y - rigAbs(sc.Y)
		dz := h.Z - rigAbs(sc.Z)
		nLocal = V3(1, 0, 0)
		m := dx
		if dy < m {
			m, nLocal = dy, V3(0, 1, 0)
		}
		if dz < m {
			m, nLocal = dz, V3(0, 0, 1)
		}
		if rigComp(sc, 0)*nLocal.X+rigComp(sc, 1)*nLocal.Y+rigComp(sc, 2)*nLocal.Z < 0 {
			nLocal = nLocal.Neg()
		}
		depth = r + m
		point = sphere.Pos.Sub(box.Rot.Rotate(nLocal).Scale(r))
	}
	ns := box.Rot.Rotate(nLocal)
	c := Contact{Depth: depth, Point: point, feat: 0}
	if sphereIsA {
		c.A, c.B, c.Normal = sphere, box, ns.Neg()
	} else {
		c.A, c.B, c.Normal = box, sphere, ns
	}
	*out = append(*out, c)
}

// rigSegEnds 返回胶囊轴线段端点。
func rigSegEnds(b *RigidBody) (Vec3, Vec3) {
	ay := b.Rot.Rotate(V3(0, 1, 0))
	half := ay.Scale(b.Shape.Height * 0.5)
	return b.Pos.Add(half), b.Pos.Sub(half)
}

// rigSphereCapsule 球-胶囊。
func rigSphereCapsule(sphere, cap *RigidBody, sphereIsA bool, out *[]Contact) {
	p0, p1 := rigSegEnds(cap)
	cl := rigClosestOnSegment(p0, p1, sphere.Pos)
	rad := cap.Shape.Radius + sphere.Shape.Radius
	d := cl.Sub(sphere.Pos)
	dist := d.Len()
	if dist-rad > rigMargin {
		return
	}
	var n Vec3 // 球 → 胶囊
	if dist > 1e-5 {
		n = d.Scale(1 / dist)
	} else {
		n = V3(0, 1, 0)
	}
	depth := rad - dist
	pt := sphere.Pos.Add(n.Scale(sphere.Shape.Radius - depth*0.5))
	c := Contact{Depth: depth, Point: pt, feat: 0}
	if sphereIsA {
		c.A, c.B, c.Normal = sphere, cap, n
	} else {
		c.A, c.B, c.Normal = cap, sphere, n.Neg()
	}
	*out = append(*out, c)
}

// rigCapsuleCapsule 胶囊-胶囊。
func rigCapsuleCapsule(a, b *RigidBody, out *[]Contact) {
	a0, a1 := rigSegEnds(a)
	b0, b1 := rigSegEnds(b)
	ca, cb := rigSegSegClosest(a0, a1, b0, b1)
	rad := a.Shape.Radius + b.Shape.Radius
	d := cb.Sub(ca)
	dist := d.Len()
	if dist-rad > rigMargin {
		return
	}
	var n Vec3
	if dist > 1e-5 {
		n = d.Scale(1 / dist)
	} else {
		n = V3(0, 1, 0)
	}
	depth := rad - dist
	pt := ca.Add(n.Scale(a.Shape.Radius - depth*0.5))
	*out = append(*out, Contact{A: a, B: b, Normal: n, Depth: depth, Point: pt, feat: 0})
}

// rigPlaneShape 任意形状-平面。planeIsB 表示平面在 B 侧（法线需取反）。
func rigPlaneShape(plane, other *RigidBody, planeIsB bool, out *[]Contact) {
	n := plane.Shape.Half.Normalize()
	if n.Len() == 0 {
		return
	}
	n = plane.Rot.Rotate(n)
	p0 := plane.Pos
	add := func(pt Vec3, depth float32, feat uint8) {
		c := Contact{Point: pt, Depth: depth, feat: feat}
		if planeIsB {
			c.A, c.B, c.Normal = other, plane, n.Neg()
		} else {
			c.A, c.B, c.Normal = plane, other, n
		}
		c.A.touching = true
		c.B.touching = true
		*out = append(*out, c)
	}
	switch other.Shape.Kind {
	case ShapeSphere:
		sep := other.Pos.Sub(p0).Dot(n) - other.Shape.Radius
		if sep > rigMargin {
			return
		}
		add(other.Pos.Sub(n.Scale(other.Shape.Radius)), -sep, 0)
	case ShapeCapsule:
		p1, p2 := rigSegEnds(other)
		r := other.Shape.Radius
		sep1 := p1.Sub(p0).Dot(n) - r
		sep2 := p2.Sub(p0).Dot(n) - r
		if sep1 > rigMargin && sep2 > rigMargin {
			return
		}
		if sep1 <= rigMargin {
			add(p1.Sub(n.Scale(r)), -sep1, 0)
		}
		if sep2 <= rigMargin {
			add(p2.Sub(n.Scale(r)), -sep2, 1)
		}
	case ShapeBox:
		ax := [3]Vec3{
			other.Rot.Rotate(V3(1, 0, 0)),
			other.Rot.Rotate(V3(0, 1, 0)),
			other.Rot.Rotate(V3(0, 0, 1)),
		}
		h := other.Shape.Half
		type cand struct {
			p Vec3
			s float32
		}
		cands := make([]cand, 0, 8)
		for i := 0; i < 8; i++ {
			sx := float32(1)
			if i&1 != 0 {
				sx = -1
			}
			sy := float32(1)
			if i&2 != 0 {
				sy = -1
			}
			sz := float32(1)
			if i&4 != 0 {
				sz = -1
			}
			v := other.Pos.
				Add(ax[0].Scale(sx * h.X)).
				Add(ax[1].Scale(sy * h.Y)).
				Add(ax[2].Scale(sz * h.Z))
			sep := v.Sub(p0).Dot(n)
			if sep <= rigMargin {
				cands = append(cands, cand{p: v, s: sep})
			}
		}
		if len(cands) == 0 {
			return
		}
		// 取最靠近平面的 4 个（穿透最浅）
		for i := 1; i < len(cands); i++ {
			for k := i; k > 0 && cands[k].s > cands[k-1].s; k-- {
				cands[k], cands[k-1] = cands[k-1], cands[k]
			}
		}
		if len(cands) > 4 {
			cands = cands[:4]
		}
		for i, cd := range cands {
			add(cd.p.Sub(n.Scale(cd.s * 0.5)), -cd.s, uint8(i))
		}
	}
}

// rigClip 用平面 dot(p,nrm) <= off 裁剪多边形。
func rigClip(poly []Vec3, nrm Vec3, off float32) []Vec3 {
	if len(poly) == 0 {
		return poly
	}
	out := make([]Vec3, 0, len(poly)+2)
	for i := 0; i < len(poly); i++ {
		p := poly[i]
		q := poly[(i+1)%len(poly)]
		dp := p.Dot(nrm) - off
		dq := q.Dot(nrm) - off
		if dp <= 0 {
			out = append(out, p)
		}
		if (dp < 0 && dq > 0) || (dp > 0 && dq < 0) {
			t := dp / (dp - dq)
			out = append(out, p.Add(q.Sub(p).Scale(t)))
		}
	}
	return out
}

// rigBoxBox 盒-盒（SAT + 参考面裁剪，1~4 个接触点）。
func rigBoxBox(a, b *RigidBody, out *[]Contact) {
	ha, hb := a.Shape.Half, b.Shape.Half
	A := [3]Vec3{a.Rot.Rotate(V3(1, 0, 0)), a.Rot.Rotate(V3(0, 1, 0)), a.Rot.Rotate(V3(0, 0, 1))}
	B := [3]Vec3{b.Rot.Rotate(V3(1, 0, 0)), b.Rot.Rotate(V3(0, 1, 0)), b.Rot.Rotate(V3(0, 0, 1))}
	t := b.Pos.Sub(a.Pos)

	var absR [3][3]float32
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			absR[i][j] = rigAbs(A[i].Dot(B[j])) + 1e-6
		}
	}

	best := float32(math.MaxFloat32)
	kind := 0 // 0=A 面轴, 1=B 面轴, 2=棱棱
	bi, bj := 0, 0
	axis := Vec3{}

	for i := 0; i < 3; i++ {
		ra := rigComp(ha, i)
		rb := hb.X*absR[i][0] + hb.Y*absR[i][1] + hb.Z*absR[i][2]
		ov := ra + rb - rigAbs(t.Dot(A[i]))
		if ov < best {
			best, kind, bi = ov, 0, i
		}
	}
	for j := 0; j < 3; j++ {
		rb := rigComp(hb, j)
		ra := ha.X*absR[0][j] + ha.Y*absR[1][j] + ha.Z*absR[2][j]
		ov := ra + rb - rigAbs(t.Dot(B[j]))
		if ov < best {
			best, kind, bj = ov, 1, j
		}
	}
	faceBest := best
	edgeBest := float32(math.MaxFloat32)
	var edgeAxis Vec3
	ei, ej := 0, 0
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			ax := A[i].Cross(B[j])
			l2 := ax.Dot(ax)
			// 近平行棱轴退化：叉积长度趋零时半径也趋零，会算出伪穿透轴抢走面接触
			if l2 < rigEdgePar {
				continue
			}
			l := float32(math.Sqrt(float64(l2)))
			ra := (rigComp(ha, (i+1)%3)*absR[(i+2)%3][j] + rigComp(ha, (i+2)%3)*absR[(i+1)%3][j]) / l
			rb := (rigComp(hb, (j+1)%3)*absR[i][(j+2)%3] + rigComp(hb, (j+2)%3)*absR[i][(j+1)%3]) / l
			ov := ra + rb - rigAbs(t.Dot(ax))/l
			if ov < edgeBest {
				edgeBest, ei, ej, edgeAxis = ov, i, j, ax.Scale(1/l)
			}
		}
	}
	// 棱轴需明显优于面轴才采纳，保证堆叠得到稳定支撑面
	if edgeBest+rigEdgeBias < faceBest {
		best, kind, bi, bj, axis = edgeBest, 2, ei, ej, edgeAxis
	}
	if best <= -rigMargin {
		return
	}

	if kind == 0 {
		axis = A[bi]
	} else if kind == 1 {
		axis = B[bj]
	}
	if t.Dot(axis) < 0 {
		axis = axis.Neg()
	}
	n := axis.Normalize()

	if kind == 2 {
		sa := [3]float32{1, 1, 1}
		for k := 0; k < 3; k++ {
			if k != bi && A[k].Dot(t) < 0 {
				sa[k] = -1
			}
		}
		pa := a.Pos
		for k := 0; k < 3; k++ {
			if k != bi {
				pa = pa.Add(A[k].Scale(sa[k] * rigComp(ha, k)))
			}
		}
		qa := pa.Add(A[bi].Scale(rigComp(ha, bi)))
		tn := t.Neg()
		sb := [3]float32{1, 1, 1}
		for k := 0; k < 3; k++ {
			if k != bj && B[k].Dot(tn) < 0 {
				sb[k] = -1
			}
		}
		pb := b.Pos
		for k := 0; k < 3; k++ {
			if k != bj {
				pb = pb.Add(B[k].Scale(sb[k] * rigComp(hb, k)))
			}
		}
		qb := pb.Add(B[bj].Scale(rigComp(hb, bj)))
		c1, c2 := rigSegSegClosest(pa, qa, pb, qb)
		pt := c1.Add(c2).Scale(0.5)
		*out = append(*out, Contact{A: a, B: b, Normal: n, Depth: rigMax(best, 0), Point: pt, feat: 0})
		return
	}

	// 面接触：参考体 + 参考面
	ref, inc := a, b
	refAxes, incAxes := A, B
	refHalf, incHalf := ha, hb
	refN := n
	if kind == 1 {
		ref, inc = b, a
		refAxes, incAxes = B, A
		refHalf, incHalf = hb, ha
		refN = n.Neg()
	}
	ri, bestDot := 0, float32(-1)
	for i := 0; i < 3; i++ {
		if d := rigAbs(refAxes[i].Dot(refN)); d > bestDot {
			bestDot, ri = d, i
		}
	}
	rsgn := rigSign(refAxes[ri].Dot(refN))
	refC := ref.Pos.Add(refAxes[ri].Scale(rsgn * rigComp(refHalf, ri)))
	u := refAxes[(ri+1)%3]
	hu := rigComp(refHalf, (ri+1)%3)
	v := refAxes[(ri+2)%3]
	hv := rigComp(refHalf, (ri+2)%3)

	ii, bestDot := 0, float32(-2)
	for j := 0; j < 3; j++ {
		if d := rigAbs(incAxes[j].Dot(refN)); d > bestDot {
			bestDot, ii = d, j
		}
	}
	isgn := -rigSign(incAxes[ii].Dot(refN))
	incC := inc.Pos.Add(incAxes[ii].Scale(isgn * rigComp(incHalf, ii)))
	iu := incAxes[(ii+1)%3].Scale(rigComp(incHalf, (ii+1)%3))
	iv := incAxes[(ii+2)%3].Scale(rigComp(incHalf, (ii+2)%3))

	poly := []Vec3{
		incC.Add(iu).Add(iv),
		incC.Sub(iu).Add(iv),
		incC.Sub(iu).Sub(iv),
		incC.Add(iu).Sub(iv),
	}
	poly = rigClip(poly, u, refC.Dot(u)+hu)
	poly = rigClip(poly, u.Neg(), -refC.Dot(u)+hu)
	poly = rigClip(poly, v, refC.Dot(v)+hv)
	poly = rigClip(poly, v.Neg(), -refC.Dot(v)+hv)
	if len(poly) == 0 {
		return
	}

	type cand struct {
		p     Vec3
		depth float32
	}
	cands := make([]cand, 0, len(poly))
	for _, p := range poly {
		sep := p.Sub(refC).Dot(refN)
		if -sep >= -rigMargin {
			cands = append(cands, cand{p: p, depth: -sep})
		}
	}
	if len(cands) == 0 {
		return
	}
	for i := 1; i < len(cands); i++ {
		for k := i; k > 0 && cands[k].depth > cands[k-1].depth; k-- {
			cands[k], cands[k-1] = cands[k-1], cands[k]
		}
	}
	if len(cands) > 4 {
		cands = cands[:4]
	}
	for i, cd := range cands {
		pt := cd.p.Sub(refN.Scale(cd.depth * 0.5))
		*out = append(*out, Contact{A: a, B: b, Normal: n, Depth: cd.depth, Point: pt, feat: uint8(i)})
	}
}

// ======================= 接触求解 =======================

// prepareContacts 计算接触的有效质量、切向基与偏置。
func (w *RigidWorld) prepareContacts(h float32) {
	for i := range w.contacts {
		c := &w.contacts[i]
		n := c.Normal.Normalize()
		if n.Len() == 0 {
			c.mN = 0
			continue
		}
		c.Normal = n
		c.ra = c.Point.Sub(c.A.Pos)
		c.rb = c.Point.Sub(c.B.Pos)
		c.t1 = rigPerp(n)
		c.t2 = n.Cross(c.t1).Normalize()

		kn := c.A.InvMass + c.B.InvMass +
			n.Dot(rigInvInertiaMul(c.A, c.ra.Cross(n)).Cross(c.ra)) +
			n.Dot(rigInvInertiaMul(c.B, c.rb.Cross(n)).Cross(c.rb))
		if kn <= 1e-12 {
			c.mN = 0
			continue
		}
		c.mN = 1 / kn
		c.mT1 = 1 / (c.A.InvMass + c.B.InvMass +
			c.t1.Dot(rigInvInertiaMul(c.A, c.ra.Cross(c.t1)).Cross(c.ra)) +
			c.t1.Dot(rigInvInertiaMul(c.B, c.rb.Cross(c.t1)).Cross(c.rb)))
		c.mT2 = 1 / (c.A.InvMass + c.B.InvMass +
			c.t2.Dot(rigInvInertiaMul(c.A, c.ra.Cross(c.t2)).Cross(c.ra)) +
			c.t2.Dot(rigInvInertiaMul(c.B, c.rb.Cross(c.t2)).Cross(c.rb)))

		// 位置偏置（Baumgarte）
		pos := rigBaumgarte / h * rigMax(c.Depth-rigSlop, 0)
		if pos > rigMaxBias {
			pos = rigMaxBias
		}
		// 弹性
		rv := rigPointVel(c.B, c.rb).Sub(rigPointVel(c.A, c.ra))
		vn := rv.Dot(n)
		rest := float32(0)
		if vn < -rigRestCut {
			rest = -0.5 * (c.A.Restitution + c.B.Restitution) * vn
		}
		c.bias = pos + rest

		// 热启动
		key := rigPairKey{a: c.A.idx, b: c.B.idx, f: c.feat}
		if wm, ok := w.warm[key]; ok {
			c.NormalImpulse = wm.n
			c.TangentImpulse = wm.t1
			c.jT2 = wm.t2
		}
	}
}

// warmStart 施加上一子步的累积冲量。
func (w *RigidWorld) warmStart() {
	for i := range w.contacts {
		c := &w.contacts[i]
		if c.mN == 0 {
			continue
		}
		if c.NormalImpulse == 0 && c.TangentImpulse == 0 && c.jT2 == 0 {
			continue
		}
		p := c.Normal.Scale(c.NormalImpulse).
			Add(c.t1.Scale(c.TangentImpulse)).
			Add(c.t2.Scale(c.jT2))
		rigApplyImpulse(c.A, c.B, c.ra, c.rb, p)
	}
}

// solveContacts 一轮顺序冲量（法向 + 双切向摩擦）。
func (w *RigidWorld) solveContacts() {
	for i := range w.contacts {
		c := &w.contacts[i]
		if c.mN == 0 {
			continue
		}
		n := c.Normal

		rv := rigPointVel(c.B, c.rb).Sub(rigPointVel(c.A, c.ra))
		vn := rv.Dot(n)
		dj := -c.mN * (vn - c.bias)
		nj := rigMax(c.NormalImpulse+dj, 0)
		dj = nj - c.NormalImpulse
		c.NormalImpulse = nj
		if dj != 0 {
			rigApplyImpulse(c.A, c.B, c.ra, c.rb, n.Scale(dj))
		}

		fric := float32(math.Sqrt(float64(c.A.Friction * c.B.Friction)))
		if fric <= 0 {
			continue
		}
		maxF := fric * c.NormalImpulse

		rv = rigPointVel(c.B, c.rb).Sub(rigPointVel(c.A, c.ra))
		vt1 := rv.Dot(c.t1)
		dj1 := -c.mT1 * vt1
		nj1 := clampF(c.TangentImpulse+dj1, -maxF, maxF)
		dj1 = nj1 - c.TangentImpulse
		c.TangentImpulse = nj1
		if dj1 != 0 {
			rigApplyImpulse(c.A, c.B, c.ra, c.rb, c.t1.Scale(dj1))
		}

		rv = rigPointVel(c.B, c.rb).Sub(rigPointVel(c.A, c.ra))
		vt2 := rv.Dot(c.t2)
		dj2 := -c.mT2 * vt2
		nj2 := clampF(c.jT2+dj2, -maxF, maxF)
		dj2 = nj2 - c.jT2
		c.jT2 = nj2
		if dj2 != 0 {
			rigApplyImpulse(c.A, c.B, c.ra, c.rb, c.t2.Scale(dj2))
		}
	}
}

// cacheWarm 缓存本子步累积冲量供下一子步热启动。
func (w *RigidWorld) cacheWarm() {
	if len(w.warm) > 4096 {
		w.warm = make(map[rigPairKey]rigWarmImpulse)
	}
	for i := range w.contacts {
		c := &w.contacts[i]
		if c.mN == 0 {
			continue
		}
		w.warm[rigPairKey{a: c.A.idx, b: c.B.idx, f: c.feat}] =
			rigWarmImpulse{n: c.NormalImpulse, t1: c.TangentImpulse, t2: c.jT2}
	}
}

// correctPositions 位置修正：推出深度穿透。
func (w *RigidWorld) correctPositions() {
	for i := range w.contacts {
		c := &w.contacts[i]
		if c.Depth <= 0.01 {
			continue
		}
		wa, wb := c.A.InvMass, c.B.InvMass
		if !c.A.Awake {
			wa = 0
		}
		if !c.B.Awake {
			wb = 0
		}
		sum := wa + wb
		if sum <= 0 {
			continue
		}
		corr := rigMin((c.Depth-rigSlop)*0.2, 0.2)
		if corr <= 0 {
			continue
		}
		push := c.Normal.Scale(corr / sum)
		if wa > 0 {
			c.A.Pos = c.A.Pos.Sub(push.Scale(wa))
		}
		if wb > 0 {
			c.B.Pos = c.B.Pos.Add(push.Scale(wb))
		}
	}
}

// ======================= 关节求解 =======================

// prepareJoints 每子步刷新关节缓存。
func (w *RigidWorld) prepareJoints(h float32) {
	for _, j := range w.joints {
		j.prepare(h)
	}
}

// solveJoints 一轮关节顺序冲量。
func (w *RigidWorld) solveJoints() {
	for _, j := range w.joints {
		j.solveVel()
	}
}

// correctJoints 关节位置修正。
func (w *RigidWorld) correctJoints() {
	for _, j := range w.joints {
		j.correct()
	}
}

// ======================= 休眠 =======================

// updateSleep 低速刚体累计时间后休眠。
func (w *RigidWorld) updateSleep(h float32) {
	for _, b := range w.bodies {
		if b.InvMass == 0 {
			continue
		}
		if !b.Awake {
			continue
		}
		lin := b.Vel.Len()
		ang := b.AngVel.Len()
		if lin < rigSleepLin && ang < rigSleepAng {
			b.SleepTimer += h
			if b.SleepTimer > rigSleepTime {
				b.Sleep()
			}
		} else {
			b.SleepTimer = 0
		}
	}
}
