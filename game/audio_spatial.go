// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// Listener 3D 听觉参考系（通常由相机每帧写入 Mixer.SetListener）。
type Listener struct {
	Pos     Vec3
	Forward Vec3
	Up      Vec3
	// MinDist 全音量半径（默认 1）；MaxDist 完全听不到的距离（默认 32）。
	MinDist, MaxDist float32
}

func (l Listener) norm() Listener {
	if l.MinDist <= 0 {
		l.MinDist = 1
	}
	if l.MaxDist <= l.MinDist {
		l.MaxDist = l.MinDist * 32
	}
	if l.Forward == (Vec3{}) {
		l.Forward = Vec3{Z: -1}
	}
	if l.Up == (Vec3{}) {
		l.Up = Vec3{Y: 1}
	}
	return l
}

// SpatialGain 距离衰减（线性收盘）：dist ≤ minD 为 1，≥ maxD 为 0。
func SpatialGain(dist, minD, maxD float32) float32 {
	if minD <= 0 {
		minD = 1
	}
	if maxD <= minD {
		maxD = minD * 32
	}
	if dist <= minD {
		return 1
	}
	if dist >= maxD {
		return 0
	}
	return 1 - (dist-minD)/(maxD-minD)
}

// SetListener 更新监听者（声像与距离衰减都以它为准）。
func (m *Mixer) SetListener(l Listener) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.listener = l.norm()
	m.mu.Unlock()
}

// ListenerInfo 当前监听者（未设置时返回默认值）。
func (m *Mixer) ListenerInfo() Listener {
	if m == nil {
		return Listener{}.norm()
	}
	m.mu.Lock()
	l := m.listener
	m.mu.Unlock()
	return l.norm()
}

// SpatialParams 计算声源相对监听者的声像与增益。
func (m *Mixer) SpatialParams(pos Vec3) (pan, gain float32) {
	l := m.ListenerInfo()
	dx, dy, dz := pos.X-l.Pos.X, pos.Y-l.Pos.Y, pos.Z-l.Pos.Z
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
	gain = SpatialGain(dist, l.MinDist, l.MaxDist)
	if dist <= 1e-4 {
		return 0, gain
	}
	// 右向量 = forward × up（归一化后声源方向点积得到 -1~1）
	fx, fy, fz := l.Forward.X, l.Forward.Y, l.Forward.Z
	ux, uy, uz := l.Up.X, l.Up.Y, l.Up.Z
	rx := fy*uz - fz*uy
	ry := fz*ux - fx*uz
	rz := fx*uy - fy*ux
	rl := float32(math.Sqrt(float64(rx*rx + ry*ry + rz*rz)))
	if rl <= 1e-6 {
		return 0, gain
	}
	pan = (dx*rx + dy*ry + dz*rz) / (rl * dist)
	if pan > 1 {
		pan = 1
	} else if pan < -1 {
		pan = -1
	}
	return pan, gain
}

// PlayAt 播放 3D 音源：自动按监听者位置算声像与距离衰减（超过 MaxDist 时不播）。
func (m *Mixer) PlayAt(s *Sound, pos Vec3, opt PlayOpt) VoiceID {
	pan, gain := m.SpatialParams(pos)
	if gain <= 0 {
		return 0
	}
	opt.Pan = pan
	if opt.Volume <= 0 {
		opt.Volume = 1
	}
	opt.Volume *= gain
	return m.Play(s, opt)
}
