// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// JointKind 关节类型。
type JointKind uint8

const (
	// JointFixed 固定关节：锁定相对位置与相对角速度。
	JointFixed JointKind = iota
	// JointHinge 铰链关节：锁定相对位置，只允许绕 Axis 相对旋转。
	JointHinge
	// JointBall 球关节：锁定相对位置，允许自由相对旋转。
	JointBall
	// JointDistance 距离关节：保持两锚点距离为 RestLength。
	JointDistance
	// JointSpring 弹簧关节：软距离约束（Stiffness / Damping）。
	JointSpring
)

// String 返回关节名称。
func (k JointKind) String() string {
	switch k {
	case JointFixed:
		return "fixed"
	case JointHinge:
		return "hinge"
	case JointBall:
		return "ball"
	case JointDistance:
		return "distance"
	case JointSpring:
		return "spring"
	}
	return "unknown"
}

// Joint 刚体关节。AnchorA / AnchorB 为各自刚体局部空间的锚点，Axis 为世界空间铰链轴。
type Joint struct {
	Kind JointKind
	A, B *RigidBody

	AnchorA, AnchorB Vec3
	Axis             Vec3

	RestLength float32
	Stiffness  float32
	Damping    float32

	Broken bool

	// 运行时缓存（每子步刷新）
	accum  Vec3
	accumN float32
	ra, rb Vec3
	kInv   [3][3]float32
	ok     bool
	n      Vec3
	biasV  Vec3
	kn     float32
	bias   float32
	gamma  float32
	massN  float32
	u, v   Vec3
	axLA   Vec3
	axLB   Vec3
	world  *RigidWorld
}

// NewFixedJoint 构造固定关节（anchor 为世界空间锚点，两刚体该点重合）。
func NewFixedJoint(a, b *RigidBody, anchor Vec3) *Joint {
	j := &Joint{Kind: JointFixed, A: a, B: b}
	j.setAnchor(anchor)
	return j
}

// NewHingeJoint 构造铰链关节（绕世界空间 axis 轴旋转）。
func NewHingeJoint(a, b *RigidBody, anchor, axis Vec3) *Joint {
	j := &Joint{Kind: JointHinge, A: a, B: b}
	j.setAnchor(anchor)
	ax := axis.Normalize()
	if ax.Len() == 0 {
		ax = V3(0, 1, 0)
	}
	j.Axis = ax
	j.axLA = a.LocalDir(ax)
	j.axLB = b.LocalDir(ax)
	return j
}

// NewBallJoint 构造球关节（锚点重合，相对旋转自由）。
func NewBallJoint(a, b *RigidBody, anchor Vec3) *Joint {
	j := &Joint{Kind: JointBall, A: a, B: b}
	j.setAnchor(anchor)
	return j
}

// NewDistanceJoint 构造距离关节（rest<=0 时取当前两刚体间距）。
func NewDistanceJoint(a, b *RigidBody, rest float32) *Joint {
	j := &Joint{Kind: JointDistance, A: a, B: b}
	if rest <= 0 {
		rest = a.Pos.Dist(b.Pos)
	}
	j.RestLength = rest
	return j
}

// NewSpringJoint 构造弹簧关节（软距离约束）。
func NewSpringJoint(a, b *RigidBody, rest, stiffness, damping float32) *Joint {
	j := &Joint{Kind: JointSpring, A: a, B: b}
	if rest <= 0 {
		rest = a.Pos.Dist(b.Pos)
	}
	j.RestLength = rest
	j.Stiffness = stiffness
	j.Damping = damping
	return j
}

// setAnchor 把世界锚点拆到两刚体局部空间。
func (j *Joint) setAnchor(anchor Vec3) {
	if j.A != nil {
		j.AnchorA = j.A.LocalPoint(anchor)
	}
	if j.B != nil {
		j.AnchorB = j.B.LocalPoint(anchor)
	}
}

// AnchorWorldA 返回 A 侧锚点的世界坐标。
func (j *Joint) AnchorWorldA() Vec3 { return j.A.WorldPoint(j.AnchorA) }

// AnchorWorldB 返回 B 侧锚点的世界坐标。
func (j *Joint) AnchorWorldB() Vec3 { return j.B.WorldPoint(j.AnchorB) }

// ======================= 求解 =======================

// prepare 计算本子步的缓存量（有效质量、偏置等）。
func (j *Joint) prepare(h float32) {
	j.ok = false
	if j.Broken || j.A == nil || j.B == nil || h <= 0 {
		return
	}
	if j.A.InvMass == 0 && j.B.InvMass == 0 {
		return
	}
	j.ra = j.A.Rot.Rotate(j.AnchorA)
	j.rb = j.B.Rot.Rotate(j.AnchorB)
	const beta = 0.2

	switch j.Kind {
	case JointFixed, JointHinge, JointBall:
		c := j.A.Pos.Add(j.ra).Sub(j.B.Pos.Add(j.rb))
		e := [3]Vec3{V3(1, 0, 0), V3(0, 1, 0), V3(0, 0, 1)}
		var k [3][3]float32
		for col := 0; col < 3; col++ {
			t := rigImpulseResponse(j.A, j.ra, e[col]).Add(rigImpulseResponse(j.B, j.rb, e[col]))
			k[0][col] = t.X
			k[1][col] = t.Y
			k[2][col] = t.Z
		}
		kinv, ok := rigMat3Inv(k)
		if !ok {
			return
		}
		j.kInv = kinv
		j.biasV = c.Scale(beta / h)
		j.accum = Vec3{}
		if j.Kind == JointHinge || j.Kind == JointFixed {
			j.u = rigPerp(j.Axis)
			if j.Kind == JointFixed {
				j.u = V3(1, 0, 0)
			}
			j.v = j.Axis.Cross(j.u).Normalize()
			if j.Kind == JointFixed {
				j.v = V3(0, 1, 0)
			}
		}
		j.ok = true

	case JointDistance, JointSpring:
		pa := j.A.Pos.Add(j.ra)
		pb := j.B.Pos.Add(j.rb)
		d := pb.Sub(pa)
		l := d.Len()
		if l < 1e-6 {
			return
		}
		j.n = d.Scale(1 / l)
		c := l - j.RestLength
		j.bias = beta / h * c
		kn := j.n.Dot(rigInvInertiaMul(j.A, j.ra.Cross(j.n)).Cross(j.ra)) +
			j.n.Dot(rigInvInertiaMul(j.B, j.rb.Cross(j.n)).Cross(j.rb)) +
			j.A.InvMass + j.B.InvMass
		if j.Kind == JointSpring {
			stiff := j.Stiffness
			damp := j.Damping
			if stiff < 0 {
				stiff = 0
			}
			if damp < 0 {
				damp = 0
			}
			den := damp + h*stiff
			if den <= 1e-9 {
				j.gamma = 0
			} else {
				j.gamma = 1 / (h * den)
			}
			j.bias = c * h * stiff * j.gamma
			j.massN = 1 / (kn + j.gamma)
		} else {
			j.gamma = 0
			if kn <= 0 {
				return
			}
			j.massN = 1 / kn
		}
		j.kn = kn
		j.accumN = 0
		j.ok = true
	}
}

// solveVel 做一次顺序冲量迭代。
func (j *Joint) solveVel() {
	if !j.ok {
		return
	}
	switch j.Kind {
	case JointFixed, JointHinge, JointBall:
		cdot := rigPointVel(j.A, j.ra).Sub(rigPointVel(j.B, j.rb))
		rhs := cdot.Add(j.biasV)
		imp := rigMat3MulVec(j.kInv, rhs)
		j.accum = j.accum.Add(imp)
		rigApplyImpulse(j.A, j.B, j.ra, j.rb, imp)

		if j.Kind == JointFixed {
			dirs := [3]Vec3{V3(1, 0, 0), V3(0, 1, 0), V3(0, 0, 1)}
			for _, d := range dirs {
				j.solveAngular(d)
			}
		} else if j.Kind == JointHinge {
			j.solveAngular(j.u)
			j.solveAngular(j.v)
		}

	case JointDistance, JointSpring:
		cdot := rigPointVel(j.B, j.rb).Sub(rigPointVel(j.A, j.ra)).Dot(j.n)
		var lambda float32
		if j.Kind == JointSpring {
			lambda = -j.massN * (cdot + j.bias + j.gamma*j.accumN)
		} else {
			lambda = -j.massN * (cdot + j.bias)
		}
		j.accumN += lambda
		rigApplyImpulse(j.A, j.B, j.ra, j.rb, j.n.Scale(lambda))
	}
}

// solveAngular 锁定相对角速度在 d 方向上的分量。
func (j *Joint) solveAngular(d Vec3) {
	wrel := j.B.AngVel.Sub(j.A.AngVel)
	k := d.Dot(rigInvInertiaMul(j.A, d)) + d.Dot(rigInvInertiaMul(j.B, d))
	if k <= 1e-12 {
		return
	}
	lambda := -wrel.Dot(d) / k
	p := d.Scale(lambda)
	if j.A.InvMass > 0 {
		j.A.AngVel = j.A.AngVel.Sub(rigInvInertiaMul(j.A, p))
	}
	if j.B.InvMass > 0 {
		j.B.AngVel = j.B.AngVel.Add(rigInvInertiaMul(j.B, p))
	}
}

// correct 位置修正（NGS，线性项 + 铰链/固定轴的角向对齐）。
func (j *Joint) correct() {
	if j.Broken || j.A == nil || j.B == nil {
		return
	}
	wa, wb := j.A.InvMass, j.B.InvMass
	sum := wa + wb
	if sum <= 0 {
		return
	}
	switch j.Kind {
	case JointFixed, JointHinge, JointBall:
		ra := j.A.Rot.Rotate(j.AnchorA)
		rb := j.B.Rot.Rotate(j.AnchorB)
		c := j.A.Pos.Add(ra).Sub(j.B.Pos.Add(rb))
		if c.Len() > 1e-5 {
			corr := c.Scale(0.2 / sum)
			if wa > 0 && j.A.Awake {
				j.A.Pos = j.A.Pos.Sub(corr.Scale(wa))
			}
			if wb > 0 && j.B.Awake {
				j.B.Pos = j.B.Pos.Add(corr.Scale(wb))
			}
		}
		if j.Kind == JointHinge {
			j.correctAxis()
		}
	case JointDistance, JointSpring:
		pa := j.A.Pos.Add(j.A.Rot.Rotate(j.AnchorA))
		pb := j.B.Pos.Add(j.B.Rot.Rotate(j.AnchorB))
		d := pb.Sub(pa)
		l := d.Len()
		if l < 1e-6 {
			return
		}
		n := d.Scale(1 / l)
		c := l - j.RestLength
		if j.Kind == JointSpring && j.Stiffness > 0 {
			// 软约束不做位置修正，避免与弹簧力对抗
			return
		}
		corr := n.Scale(c * 0.2 / sum)
		if wa > 0 && j.A.Awake {
			j.A.Pos = j.A.Pos.Add(corr.Scale(wa))
		}
		if wb > 0 && j.B.Awake {
			j.B.Pos = j.B.Pos.Sub(corr.Scale(wb))
		}
	}
}

// correctAxis 让铰链两侧局部轴重新对齐。
func (j *Joint) correctAxis() {
	wa, wb := j.A.InvMass, j.B.InvMass
	sum := wa + wb
	if sum <= 0 {
		return
	}
	axA := j.A.Rot.Rotate(j.axLA)
	axB := j.B.Rot.Rotate(j.axLB)
	cr := axA.Cross(axB)
	s := cr.Len()
	if s < 1e-5 {
		return
	}
	axis := cr.Scale(1 / s)
	ang := float32(math.Asin(float64(rigMin(s, 1))))
	const k = 0.3
	if wa > 0 {
		j.A.Rot = QuatAxisAngle(axis, ang*k*wa/sum).Mul(j.A.Rot).Normalize()
	}
	if wb > 0 {
		j.B.Rot = QuatAxisAngle(axis, -ang*k*wb/sum).Mul(j.B.Rot).Normalize()
	}
}
