// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// BonePose 一根骨在某一时刻的局部变换。
type BonePose struct {
	Pos Vec3
	Rot Quat
	Scl Vec3
}

// KeyFrame 关键帧（时间 + 局部变换）。
type KeyFrame struct {
	Time float32
	Pos  Vec3
	Rot  Quat
	Scl  Vec3
}

// BoneTrack 某根骨的动画轨道（Keys 按 Time 升序）。
type BoneTrack struct {
	Bone int
	Keys []KeyFrame
}

// SkinClip 一段骨骼动画（多个骨轨道共用一条时间轴）。
type SkinClip struct {
	Name     string
	Duration float32
	Tracks   []BoneTrack
}

// quatSlerp 四元数球面线性插值（自动选短弧，退化为 nlerp 避免除零）。
func quatSlerp(a, b Quat, t float32) Quat {
	dot := a.X*b.X + a.Y*b.Y + a.Z*b.Z + a.W*b.W
	if dot < 0 {
		b = Quat{-b.X, -b.Y, -b.Z, -b.W}
		dot = -dot
	}
	if dot > 0.9995 {
		return Quat{
			X: a.X + (b.X-a.X)*t,
			Y: a.Y + (b.Y-a.Y)*t,
			Z: a.Z + (b.Z-a.Z)*t,
			W: a.W + (b.W-a.W)*t,
		}.Normalize()
	}
	theta := float32(math.Acos(float64(dot)))
	s := float32(math.Sin(float64(theta)))
	sa := float32(math.Sin(float64((1-t)*theta))) / s
	sb := float32(math.Sin(float64(t*theta))) / s
	return Quat{
		X: a.X*sa + b.X*sb,
		Y: a.Y*sa + b.Y*sb,
		Z: a.Z*sa + b.Z*sb,
		W: a.W*sa + b.W*sb,
	}.Normalize()
}

// sampleTrack 在 keys 上采样 t（秒），两端夹持、中间线性 / slerp。
func sampleTrack(keys []KeyFrame, t float32) BonePose {
	if len(keys) == 0 {
		return BonePose{Scl: Vec3{X: 1, Y: 1, Z: 1}}
	}
	if len(keys) == 1 || t <= keys[0].Time {
		k := keys[0]
		return BonePose{Pos: k.Pos, Rot: k.Rot, Scl: k.Scl}
	}
	last := keys[len(keys)-1]
	if t >= last.Time {
		return BonePose{Pos: last.Pos, Rot: last.Rot, Scl: last.Scl}
	}
	// 二分找到 t 所在区间
	lo, hi := 0, len(keys)-1
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if keys[mid].Time <= t {
			lo = mid
		} else {
			hi = mid
		}
	}
	a, b := keys[lo], keys[hi]
	span := b.Time - a.Time
	if span <= 1e-6 {
		return BonePose{Pos: b.Pos, Rot: b.Rot, Scl: b.Scl}
	}
	f := (t - a.Time) / span
	return BonePose{
		Pos: a.Pos.Lerp(b.Pos, f),
		Rot: quatSlerp(a.Rot, b.Rot, f),
		Scl: a.Scl.Lerp(b.Scl, f),
	}
}

// Sample 采样整段动画写入 out（长度 = 骨数）。
//
// 轨道没覆盖的骨保持 out 中原值（SkinAnimator 会先用绑定姿势填充）。
func (c *SkinClip) Sample(t float32, out []BonePose) {
	if c == nil {
		return
	}
	for _, tr := range c.Tracks {
		if tr.Bone < 0 || tr.Bone >= len(out) || len(tr.Keys) == 0 {
			continue
		}
		out[tr.Bone] = sampleTrack(tr.Keys, t)
	}
}

// SkinAnimator 骨骼动画播放器（支持循环、倍速与两段动画交叉淡入）。
type SkinAnimator struct {
	sm   *SkinnedMesh
	clip *SkinClip
	time float32
	loop bool
	// Speed 播放倍速（0 视为 1）。
	Speed float32

	prev      *SkinClip
	prevTime  float32
	fade      float32
	fadeTotal float32

	bones  int
	cur    []BonePose
	prevPs []BonePose
	world  []Mat4
	pose   []Mat4
}

// NewSkinAnimator 为蒙皮网格创建播放器（骨架为空时返回 nil）。
func NewSkinAnimator(sm *SkinnedMesh) *SkinAnimator {
	if sm == nil || sm.Skeleton == nil || len(sm.Skeleton.Bones) == 0 {
		return nil
	}
	n := len(sm.Skeleton.Bones)
	a := &SkinAnimator{sm: sm, Speed: 1, bones: n}
	a.cur = make([]BonePose, n)
	a.prevPs = make([]BonePose, n)
	a.world = make([]Mat4, n)
	a.pose = make([]Mat4, n)
	a.resetBind()
	return a
}

// resetBind 把姿态恢复成绑定姿势。
func (a *SkinAnimator) resetBind() {
	for i, b := range a.sm.Skeleton.Bones {
		scl := b.Scl
		if scl.X == 0 && scl.Y == 0 && scl.Z == 0 {
			scl = Vec3{X: 1, Y: 1, Z: 1}
		}
		a.cur[i] = BonePose{Pos: b.Pos, Rot: b.Rot, Scl: scl}
	}
}

// Play 直接切换到 clip（无淡入）。
func (a *SkinAnimator) Play(clip *SkinClip, loop bool, speed float32) {
	if a == nil {
		return
	}
	a.clip = clip
	a.time = 0
	a.loop = loop
	if speed != 0 {
		a.Speed = speed
	}
	a.prev = nil
	a.fade = 0
	a.fadeTotal = 0
}

// CrossFade 在 seconds 秒内从当前动画淡入到 clip。
func (a *SkinAnimator) CrossFade(clip *SkinClip, seconds float32) {
	if a == nil || clip == nil {
		return
	}
	if a.clip == nil || seconds <= 0 {
		a.Play(clip, true, a.Speed)
		return
	}
	// 把当前动画快照成 prev，从它的当前时间继续采样
	a.prev = a.clip
	a.prevTime = a.time
	a.clip = clip
	a.time = 0
	a.loop = true
	a.fade = seconds
	a.fadeTotal = seconds
}

// Clip 当前动画。
func (a *SkinAnimator) Clip() *SkinClip {
	if a == nil {
		return nil
	}
	return a.clip
}

// Time 当前动画时间（秒）。
func (a *SkinAnimator) Time() float32 {
	if a == nil {
		return 0
	}
	return a.time
}

// FadeWeight 当前淡入权重（1 = 已完全切到新动画）。
func (a *SkinAnimator) FadeWeight() float32 {
	if a == nil || a.fadeTotal <= 0 {
		return 1
	}
	w := 1 - a.fade/a.fadeTotal
	if w < 0 {
		return 0
	}
	if w > 1 {
		return 1
	}
	return w
}

// Update 推进动画并重算骨骼矩阵。
func (a *SkinAnimator) Update(dt float32) {
	if a == nil || a.clip == nil || a.bones == 0 {
		return
	}
	sp := a.Speed
	if sp == 0 {
		sp = 1
	}
	a.time += dt * sp
	a.clipTimeWrap(&a.time, a.clip, a.loop)

	a.resetBind()
	a.clip.Sample(a.time, a.cur)

	// 交叉淡入：prev 动画继续推进并按权重混合
	if a.prev != nil && a.fadeTotal > 0 {
		a.prevTime += dt * sp
		a.clipTimeWrap(&a.prevTime, a.prev, true)
		for i := range a.prevPs {
			b := a.sm.Skeleton.Bones[i]
			scl := b.Scl
			if scl.X == 0 && scl.Y == 0 && scl.Z == 0 {
				scl = Vec3{X: 1, Y: 1, Z: 1}
			}
			a.prevPs[i] = BonePose{Pos: b.Pos, Rot: b.Rot, Scl: scl}
		}
		a.prev.Sample(a.prevTime, a.prevPs)
		a.fade -= dt
		w := a.FadeWeight()
		for i := range a.cur {
			a.cur[i] = BonePose{
				Pos: a.prevPs[i].Pos.Lerp(a.cur[i].Pos, w),
				Rot: quatSlerp(a.prevPs[i].Rot, a.cur[i].Rot, w),
				Scl: a.prevPs[i].Scl.Lerp(a.cur[i].Scl, w),
			}
		}
		if a.fade <= 0 {
			a.prev = nil
			a.fade = 0
			a.fadeTotal = 0
		}
	}

	a.buildPose()
}

// clipTimeWrap 处理循环 / 夹持时间。
func (a *SkinAnimator) clipTimeWrap(t *float32, clip *SkinClip, loop bool) {
	if clip == nil || clip.Duration <= 0 {
		return
	}
	d := clip.Duration
	if loop {
		for *t >= d {
			*t -= d
		}
		for *t < 0 {
			*t += d
		}
		return
	}
	if *t > d {
		*t = d
	}
}

// buildPose 由每骨局部姿势算出关节矩阵（要求骨骼拓扑有序，父在前）。
func (a *SkinAnimator) buildPose() {
	bones := a.sm.Skeleton.Bones
	for i := range bones {
		local := a.cur[i]
		scl := local.Scl
		if scl.X == 0 && scl.Y == 0 && scl.Z == 0 {
			scl = Vec3{X: 1, Y: 1, Z: 1}
		}
		lm := Translate(local.Pos).Mul(local.Rot.Mat4()).Mul(Scale(scl))
		p := bones[i].Parent
		if p >= 0 && p < i {
			a.world[i] = a.world[p].Mul(lm)
		} else {
			a.world[i] = lm
		}
	}
	for i := range bones {
		a.pose[i] = a.world[i].Mul(bones[i].InverseBind)
	}
}

// Pose 当前关节矩阵（长度 = 骨数），直接交给 SkinnedMesh.SkinnedVertices。
func (a *SkinAnimator) Pose() []Mat4 {
	if a == nil {
		return nil
	}
	return a.pose
}
