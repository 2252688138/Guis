// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// 级联阴影（CSM，Cascaded Shadow Maps）。
//
// 单张 ShadowMap 用「整个场景包围球」建一张固定大小的光空间图，场景一大
// 阴影就糊成马赛克。CSM 把相机视锥按距离切成 Levels 段，每段单独建一张
// 阴影图 —— 近处像素密度高，远处覆盖大，整体分辨率利用率高得多。
//
//	csm := game.NewCSMShadow()
//	csm.Levels = 3
//	csm.Size = 512
//	renderer.Cascades = csm
//
// 与 Renderer.Shadow（单图）二选一：同时设置时 CSM 优先。每个方向的平行光
// 只有第一个（强度 > 0）会投影，其余不参与。
type CSMShadow struct {
	// On 是否启用（默认 true）。
	On bool
	// Levels 级数 1~4（默认 3；1 等价于单张阴影但按视锥而非场景包围球）。
	Levels int
	// Size 每级阴影图边长（默认 512，钳 32~2048；总显存 = Levels × Size²）。
	Size int
	// SplitLambda 分割方式 0~1：0 = 按距离均匀分，1 = 按对数分（近处更密，默认 0.7）。
	SplitLambda float32
	// Blend 级间过渡带宽（占该级距离的比例，默认 0.1；0 = 硬切，接缝可见）。
	Blend float32
	// Bias 深度偏移（默认 0.0025）。
	Bias float32
	// PCF 采样核宽 1/3/5（默认 3）。
	PCF int
	// Strength 阴影强度（默认 0.65）。
	Strength float32
	// MaxDistance 最远投影距离（0 = 用相机 Far）；超出部分不投影。
	MaxDistance float32
	// Aspect 相机宽高比（Renderer 每帧自动填；0 = 按 16:9 估算）。
	Aspect float32

	maps   []*ShadowMap
	splits []float32 // 各级远平面（相机空间距离，递增）
	valid  bool
	camPos Vec3
}

// NewCSMShadow 创建默认配置的级联阴影（3 级、512、小数混合分割）。
func NewCSMShadow() *CSMShadow {
	return &CSMShadow{
		On:          true,
		Levels:      3,
		Size:        512,
		SplitLambda: 0.7,
		Blend:       0.1,
		Bias:        0.0025,
		PCF:         3,
		Strength:    0.65,
	}
}

// Valid 阴影图是否可用（本帧构建成功）。
func (c *CSMShadow) Valid() bool { return c != nil && c.valid }

// Build 按当前相机视锥切分并逐级重建阴影图（Renderer 每帧调用）。
func (c *CSMShadow) Build(sc *Scene) {
	if c == nil || !c.On {
		return
	}
	c.valid = false
	if sc == nil || sc.Camera == nil || sc.Root == nil {
		return
	}
	cam := sc.Camera
	fwd := cam.Target.Sub(cam.Pos)
	if fwd.Len() <= 1e-6 {
		fwd = V3(0, 0, -1)
	} else {
		fwd = fwd.Normalize()
	}

	// 只有方向光投影（点光源需要立方体阴影，另做）
	var dir Vec3
	found := false
	for i := range sc.Lights {
		lt := &sc.Lights[i]
		if lt.Kind == LightDirectional && lt.Intensity > 0 && lt.Dir.Len() > 0 {
			dir = lt.Dir.Normalize()
			found = true
			break
		}
	}
	if !found {
		return
	}

	sc.BuildWorld()

	levels := c.Levels
	if levels < 1 {
		levels = 1
	}
	if levels > 4 {
		levels = 4
	}
	size := c.Size
	if size <= 0 {
		size = 512
	}

	near := cam.Near
	if near <= 0 {
		near = 0.1
	}
	far := cam.Far
	if far <= near {
		far = near + 100
	}
	if c.MaxDistance > 0 && far > c.MaxDistance {
		far = c.MaxDistance
	}
	if far <= near {
		return
	}

	// 视锥分割：均匀与对数混合（PSSM 的常见做法）
	lambda := Clamp(c.SplitLambda, 0, 1)
	splits := make([]float32, levels)
	for i := 1; i <= levels; i++ {
		p := float32(i) / float32(levels)
		logSplit := near * float32(math.Pow(float64(far/near), float64(p)))
		uniSplit := near + (far-near)*p
		splits[i-1] = uniSplit*(1-lambda) + logSplit*lambda
	}
	splits[levels-1] = far

	fov := cam.FOVY
	if fov <= 0 {
		fov = 60
	}
	aspect := c.Aspect
	if aspect <= 0 {
		aspect = 16.0 / 9.0
	}
	halfV := float32(math.Tan(float64(Radians(fov)) * 0.5))

	// 逐级建图：级内视锥用一个包围球近似（球拟合，光空间正交投影）
	maps := c.maps
	if len(maps) < levels {
		maps = make([]*ShadowMap, levels)
	}
	start := near
	for i := 0; i < levels; i++ {
		end := splits[i]
		dz := end - start
		if dz <= 1e-3 {
			dz = 1e-3
		}
		mid := start + dz*0.5
		hv := mid * halfV
		hh := hv * aspect
		radius := float32(math.Sqrt(float64(hv*hv + hh*hh)))
		if radius < 1 {
			radius = 1
		}
		mp := maps[i]
		if mp == nil {
			mp = NewShadowMap()
			maps[i] = mp
		}
		mp.On = true
		mp.Size = size
		mp.Bias = c.Bias
		mp.PCF = c.PCF
		mp.Strength = c.Strength
		mp.Once = false
		mp.Near, mp.Far = 0, 0
		mp.buildFor(sc, cam.Pos.Add(fwd.Scale(mid)), radius, dir)
		start = end
	}
	c.maps = maps[:levels]
	c.splits = splits
	c.camPos = cam.Pos
	c.valid = true
}

// FactorCSM 返回世界坐标处的受光系数（1 = 完全受光）。
//
// camPos 用来判断该片元落在哪一级；级间按 Blend 在相邻两级之间加权混合，
// 避免各级分辨率不同导致的接缝。
func (c *CSMShadow) FactorCSM(world, camPos Vec3) float32 {
	if c == nil || !c.valid || len(c.maps) == 0 {
		return 1
	}
	d := camPos.Dist(world)
	if c.MaxDistance > 0 && d > c.MaxDistance {
		return 1
	}
	level := len(c.maps) - 1
	for i, s := range c.splits {
		if i >= len(c.maps) {
			break
		}
		if d <= s {
			level = i
			break
		}
	}
	f := c.maps[level].Factor(world)
	blend := c.Blend
	if blend <= 0 || level+1 >= len(c.maps) {
		return f
	}
	prev := float32(0)
	if level > 0 {
		prev = c.splits[level-1]
	}
	band := (c.splits[level] - prev) * Clamp(blend, 0, 1)
	if band <= 0 || d <= c.splits[level]-band {
		return f
	}
	t := Clamp((d-(c.splits[level]-band))/band, 0, 1)
	f2 := c.maps[level+1].Factor(world)
	return f + (f2-f)*t
}

// Splits 各级远平面（相机空间距离），调试用。
func (c *CSMShadow) Splits() []float32 {
	if c == nil {
		return nil
	}
	return append([]float32(nil), c.splits...)
}

// Stats 返回阴影三角形总数与各级三角形数（调试用）。
func (c *CSMShadow) Stats() (total int, perLevel []int) {
	if c == nil {
		return 0, nil
	}
	perLevel = make([]int, len(c.maps))
	for i, m := range c.maps {
		if m == nil {
			continue
		}
		perLevel[i] = m.tris
		total += m.tris
	}
	return total, perLevel
}
