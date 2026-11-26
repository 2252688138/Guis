// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"image"
	"math"
	"sync"
)

// whiteTex 1×1 白色纹理（无贴图粒子的默认贴图，全局共享）。
var (
	whiteTexOnce sync.Once
	whiteTex     *Texture
)

func whiteTexture() *Texture {
	whiteTexOnce.Do(func() {
		whiteTex = NewTexture(1, 1)
		whiteTex.Fill(RGB(255, 255, 255))
	})
	return whiteTex
}

// Emitter 粒子发射器参数（零值字段会取默认值）。
type Emitter struct {
	// Pos 发射原点（屏幕像素坐标）。
	Pos Vec2
	// Box 发射范围（以 Pos 为中心的矩形，0 表示点发射）。
	Box Vec2
	// Rate 每秒自动发射数（0 表示只能用 Emit 手动发射）。
	Rate float32
	// Life 寿命区间（秒）。
	Life [2]float32
	// Speed 初速区间（像素/秒）。
	Speed [2]float32
	// Angle 发射角区间（弧度，0 为 +X 方向）。
	Angle [2]float32
	// Gravity 重力加速度（像素/秒²，正数向下）。
	Gravity float32
	// Drag 阻尼系数（每秒衰减比例 0~1）。
	Drag float32
	// Size 起始与结束尺寸（像素）。
	Size [2]float32
	// Color 起始与结束颜色（渐淡通常用 A=0 结束）。
	Color [2]RGBA
	// Max 池上限（默认 512）。
	Max int
	// Texture 粒子贴图（nil 时画实心方块）。
	Texture *Texture
	// Additive 叠加混合（适合火花/光斑）。
	Additive bool
}

// particle 单个粒子（池元素）。
type particle struct {
	pos, vel Vec2
	life     float32
	maxLife  float32
	s0, s1   float32
	c0, c1   RGBA
}

// ParticleSystem 池化粒子系统，直接实现 Sprite2D 作为叠加层。
type ParticleSystem struct {
	Emitter
	pool []particle
	live int
	acc  float32
	seed uint32
}

// NewParticleSystem 创建粒子系统（容量由 Emitter.Max 决定，默认 512）。
func NewParticleSystem(e Emitter) *ParticleSystem {
	p := &ParticleSystem{Emitter: e, seed: 0x9e3779b9}
	p.normalize()
	p.pool = make([]particle, p.Max)
	return p
}

// normalize 补齐零值字段的默认参数。
func (p *ParticleSystem) normalize() {
	if p.Max <= 0 {
		p.Max = 512
	}
	if p.Life[1] <= 0 {
		p.Life = [2]float32{0.4, 0.8}
	}
	if p.Speed[1] <= 0 {
		p.Speed = [2]float32{30, 90}
	}
	if p.Angle[1] <= 0 {
		p.Angle = [2]float32{0, 2 * math.Pi}
	}
	if p.Size[0] <= 0 {
		p.Size = [2]float32{4, 1}
	}
	if p.Color[0].A == 0 && p.Color[0].R == 0 && p.Color[0].G == 0 && p.Color[0].B == 0 {
		p.Color[0] = RGBA{R: 255, G: 200, B: 90, A: 255}
	}
	if p.Color[1].A == 0 && p.Color[1].R == 0 && p.Color[1].G == 0 && p.Color[1].B == 0 {
		p.Color[1] = RGBA{R: 255, G: 80, B: 40, A: 0}
	}
	if p.Texture == nil {
		p.Texture = whiteTexture()
	}
}

// rnd 返回 [0,1) 伪随机数（xorshift，确定性、零分配）。
func (p *ParticleSystem) rnd() float32 {
	x := p.seed
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	p.seed = x
	return float32(x%65536) / 65536
}

func (p *ParticleSystem) rangeF32(r [2]float32) float32 {
	if r[1] <= r[0] {
		return r[0]
	}
	return r[0] + (r[1]-r[0])*p.rnd()
}

// Emit 立即发射 n 个粒子（池满时丢弃多余）。
func (p *ParticleSystem) Emit(n int) {
	for i := 0; i < n; i++ {
		if p.live >= len(p.pool) {
			return
		}
		t := p.pool[p.live]
		t.pos = p.Pos
		if p.Box.X != 0 || p.Box.Y != 0 {
			t.pos.X += (p.rnd() - 0.5) * p.Box.X
			t.pos.Y += (p.rnd() - 0.5) * p.Box.Y
		}
		a := p.rangeF32(p.Angle)
		sp := p.rangeF32(p.Speed)
		t.vel = Vec2{X: float32(math.Cos(float64(a))) * sp, Y: float32(math.Sin(float64(a))) * sp}
		t.maxLife = p.rangeF32(p.Life)
		if t.maxLife <= 0 {
			t.maxLife = 0.01
		}
		t.life = t.maxLife
		t.s0 = p.Size[0]
		t.s1 = p.Size[1]
		t.c0 = p.Color[0]
		t.c1 = p.Color[1]
		p.pool[p.live] = t
		p.live++
	}
}

// Update 推进粒子（含按 Rate 自动发射）。
func (p *ParticleSystem) Update(dt float32) {
	if dt <= 0 {
		return
	}
	if p.Rate > 0 {
		p.acc += p.Rate * dt
		n := int(p.acc)
		if n > 0 {
			p.acc -= float32(n)
			p.Emit(n)
		}
	}
	for i := 0; i < p.live; {
		t := &p.pool[i]
		t.life -= dt
		if t.life <= 0 {
			p.live--
			p.pool[i] = p.pool[p.live]
			continue
		}
		t.vel.Y += p.Gravity * dt
		if p.Drag > 0 {
			k := 1 - p.Drag*dt
			if k < 0 {
				k = 0
			}
			t.vel.X *= k
			t.vel.Y *= k
		}
		t.pos.X += t.vel.X * dt
		t.pos.Y += t.vel.Y * dt
		i++
	}
}

// Count 当前存活粒子数。
func (p *ParticleSystem) Count() int { return p.live }

// Clear 清空所有粒子。
func (p *ParticleSystem) Clear() {
	p.live = 0
	p.acc = 0
}

// DrawSprites 实现 Sprite2D：把粒子当作精灵画到目标图。
func (p *ParticleSystem) DrawSprites(dst *image.RGBA, w, h int) {
	for i := 0; i < p.live; i++ {
		t := p.pool[i]
		k := 1 - t.life/t.maxLife
		sz := t.s0 + (t.s1-t.s0)*k
		if sz <= 0 {
			continue
		}
		c := lerpRGBA(t.c0, t.c1, k)
		if p.Additive {
			c = RGBA{R: c.R, G: c.G, B: c.B, A: uint8(int(c.A) / 2)}
		}
		DrawSprite(dst, Sprite{
			Pos:     Vec2{X: t.pos.X - sz/2, Y: t.pos.Y - sz/2},
			Size:    Vec2{X: sz, Y: sz},
			Texture: p.Texture,
			Color:   c,
		})
	}
}

// lerpRGBA 颜色线性插值。
func lerpRGBA(a, b RGBA, t float32) RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	mix := func(x, y uint8) uint8 {
		return uint8(float32(x) + (float32(y)-float32(x))*t)
	}
	return RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: mix(a.A, b.A)}
}
