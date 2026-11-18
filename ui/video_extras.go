// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"time"
)

// 播放器增强层：封面、播放结束遮罩、缓冲指示、信息浮层、快捷键与全屏切换。
// 布局与绘制仍由 VideoPlayer 组合，本文件只提供状态判定与图层节点。

// videoClickAt 各播放器上一次点击画面的时间（双击 = 全屏的判定用）。
var videoClickAt = map[string]float32{}

// ToggleFullscreen 切换播放器全屏：画面铺满整窗（走窗口根层 portal，嵌在
// 固定高度 / Clipped 面板里也能真全屏）+ 系统窗口全屏（覆盖当前显示器）。
func (p *Player) ToggleFullscreen() {
	if p == nil {
		return
	}
	p.Fullscreen = !p.Fullscreen
	if cur := currentApp.Load(); cur != nil {
		cur.SetFullscreen(p.Fullscreen)
	}
	if p.OnFullscreen != nil {
		p.OnFullscreen()
	}
	RequestRedraw()
}

// SetFullscreen 设置播放器全屏状态（内部会同步系统窗口全屏）。
func (p *Player) SetFullscreen(v bool) {
	if p == nil || p.Fullscreen == v {
		return
	}
	p.ToggleFullscreen()
}

// SetCover 设置封面图（本地图片路径；空串 = 关闭封面）。
func (p *Player) SetCover(path string) {
	if p != nil {
		p.Cover = path
	}
}

// CoverVisible 封面当前是否显示（设置了封面且停在起点、未播放）。
func (p *Player) CoverVisible() bool { return videoCoverShown(p) }

// videoSurfaceClick 点击画面的默认行为：单击 播放/暂停（有封面时直接开播），
// 双击 切全屏（并撤销单击引起的播放切换，双击只负责全屏）。
func videoSurfaceClick(key string, p *Player) {
	if p == nil || p.Locked {
		return
	}
	now := AnimTime()
	if now-videoClickAt[key] <= 0.32 {
		videoClickAt[key] = 0
		p.TogglePlay() // 撤销单击的切换
		p.ToggleFullscreen()
		return
	}
	videoClickAt[key] = now
	if videoCoverShown(p) {
		p.Play()
		return
	}
	p.TogglePlay()
}

// videoCoverShown 封面是否显示：设置了封面、暂停在起点（未开始 / 回到开头）。
func videoCoverShown(p *Player) bool {
	if p == nil || p.Cover == "" || p.Playing {
		return false
	}
	return p.Pos() <= 0
}

// videoEndedShown 是否处于「播放结束」状态（有总时长、已到末尾）。
func videoEndedShown(p *Player) bool {
	if p == nil || p.Playing {
		return false
	}
	d := p.Duration()
	return d > 0 && p.Pos() >= d
}

// videoBuffering 是否正在等待画面（外部解码源在播放中但还没拿到首帧）。
func videoBuffering(p *Player) bool {
	return p != nil && p.Playing && p.Source != nil && p.frame == nil
}

// videoCoverLayer 封面层：封面图 + 暗色渐变 + 中央大播放按钮 + 左下角标题。
func videoCoverLayer(key string, p *Player) *Node {
	kids := []*Node{
		Image(p.Cover).St(St().AbsFill()),
		Box().St(St().AbsFill().BgGrad("rgba(4,6,12,0.08)", "rgba(3,5,9,0.92)", 180)),
		Stack(
			Text("▶").St(St().AbsFill().Text(20, 600, "#0b0e14").TAlign(AlignCenter).CenterYOn()),
		).St(St().W(58).H(58).R(29).BgC(videoGold).AbsCenter()),
	}
	if p.CoverTitle != "" || p.CoverSub != "" {
		rows := make([]*Node, 0, 2)
		if p.CoverTitle != "" {
			rows = append(rows, Text(p.CoverTitle).St(St().Text(19, 700, "#ffffff").WrapOn()))
		}
		if p.CoverSub != "" {
			rows = append(rows, Text(p.CoverSub).St(St().Text(12, 400, "rgba(226,232,240,0.72)").WrapOn()))
		}
		kids = append(kids, Column(rows...).St(St().
			AbsBottom(76).Wp(100).PadOnly(0, 30, 0, 30).GapOf(6)))
	}
	n := Stack(kids...).St(St().AbsFill()).K(key + "-cover")
	n.Tap(func() { p.Play() })
	return n
}

// videoEndedLayer 播放结束遮罩：「播放结束」文案 + 重播（+ 上一集/下一集）。
func videoEndedLayer(key string, p *Player) *Node {
	btns := []*Node{
		Button("↻ 重播", func() {
			p.SetPos(0)
			p.Play()
		}),
	}
	if p.OnPrev != nil {
		btns = append(btns, Button("上一集", func() { p.OnPrev() }))
	}
	if p.OnNext != nil {
		btns = append(btns, Button("下一集", func() { p.OnNext() }))
	}
	body := Column(
		Text("播放结束").St(St().Text(13, 500, "rgba(226,232,240,0.78)").TAlign(AlignCenter)),
		Row(btns...).St(St().GapOf(10)),
	).St(St().GapOf(10)).AlignTo(AlignCenter)
	return Stack(
		Box().St(St().AbsFill().BgC("rgba(3,5,9,0.66)")),
		Stack(body).St(St().AbsFill()).AlignTo(AlignCenter).JustifyTo(AlignCenter),
	).St(St().AbsFill()).K(key + "-ended")
}

// videoBufferingLayer 缓冲层：中央转圈。
func videoBufferingLayer(key string) *Node {
	return Stack(Spinner(28, videoGold)).St(St().AbsFill()).
		AlignTo(AlignCenter).JustifyTo(AlignCenter).K(key + "-buffering")
}

// videoInfoLayer 信息浮层：分辨率、时间、倍速、旋转（I 键切换）。
func videoInfoLayer(key string, p *Player) *Node {
	w, h := p.Size()
	if w <= 0 || h <= 0 {
		if c := p.Clip; c != nil {
			w, h = c.Size()
		}
	}
	rows := []string{
		fmt.Sprintf("画面 %d×%d", w, h),
		fmt.Sprintf("时间 %s / %s", formatVideoTime(p.Pos()), formatVideoTime(p.Duration())),
		fmt.Sprintf("倍速 %.2fx · 旋转 %d°", p.SpeedValue(), p.RotationDeg()),
	}
	kids := make([]*Node, 0, len(rows))
	for _, s := range rows {
		kids = append(kids, Text(s).St(St().Text(11.5, 500, "rgba(236,241,252,0.92)")))
	}
	return Stack(
		Column(kids...).St(St().W(184).GapOf(2).PadOnly(7, 10, 7, 10).
			R(8).BgC("rgba(3,5,9,0.62)")),
	).St(St().AbsFill().PadOnly(10, 10, 0, 0)).K(key + "-info")
}

// videoHandleKey 键盘快捷键（画面获得焦点时由 VideoPlayer 调用）。
//
//	空格 播放/暂停 · ←/→ 5s · J/L 10s · Home/End 首尾 · 0~9 跳百分比
//	↑/↓ 音量 · M 静音 · F 全屏 · I 信息 · [ ] 倍速 · Esc 退出全屏
func videoHandleKey(key string, p *Player, code uint32) {
	if p == nil {
		return
	}
	// 锁定后除 Esc 外的快捷键失效
	if p.Locked && code != 0x1B {
		return
	}
	switch code {
	case 0x20: // Space
		p.TogglePlay()
	case 0x25: // ←
		p.SeekBy(-5 * time.Second)
	case 0x27: // →
		p.SeekBy(5 * time.Second)
	case 0x4A: // J
		p.SeekBy(-10 * time.Second)
	case 0x4C: // L
		p.SeekBy(10 * time.Second)
	case 0x26: // ↑
		p.SetVolume(p.Volume + 0.1)
	case 0x28: // ↓
		p.SetVolume(p.Volume - 0.1)
	case 0x24: // Home
		p.SetPos(0)
	case 0x23: // End
		p.SetPos(p.Duration())
	case 0x4D: // M
		p.ToggleMute()
	case 0x46: // F
		p.ToggleFullscreen()
	case 0x49: // I
		p.ShowInfo = !p.ShowInfo
		RequestRedraw()
	case 0x5B: // [
		p.SetSpeed(p.SpeedValue() - 0.25)
	case 0x5D: // ]
		p.SetSpeed(p.SpeedValue() + 0.25)
	case 0x1B: // Esc：退出播放器全屏
		if p.Fullscreen {
			p.SetFullscreen(false)
		}
		return
	default:
		// 0~9：跳到对应百分比（0 回到开头）
		if code >= 0x30 && code <= 0x39 {
			p.SeekRatio(float32(code-0x30) / 10)
		}
		return
	}
	markAnim()
}

// videoWheelVolume 滚轮调音量：dy 为滚轮滚动量（向上为正），一格 ≈ 10% 音量。
func videoWheelVolume(key string, p *Player, dy float32) {
	if p == nil || p.Locked || dy == 0 {
		return
	}
	p.SetVolume(p.Volume + dy/96*0.1)
	// 音量浮层跟着显示一会儿，滚轮调节有即时反馈
	videoVolOpen[key] = AnimTime()
	markAnim()
}
