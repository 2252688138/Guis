// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"testing"
	"time"
)

// TestPortalWrapAndReset 窗口级浮层：登记后挂到根层，重置/移除后消失。
func TestPortalWrapAndReset(t *testing.T) {
	portalReset()
	defer portalReset()

	base := Stack(Box()).St(St().Wp(100).Hp(100))
	if portalWrap(base) != base {
		t.Fatal("无浮层时不应包装根视图")
	}

	Portal("p", Box().K("p-layer"))
	root := portalWrap(base)
	if root == base {
		t.Fatal("有浮层时应包装根视图")
	}
	if findKey(root, "p-layer") == nil {
		t.Fatal("浮层节点未挂到根层")
	}
	if ks := PortalKeys(); len(ks) != 1 || ks[0] != "p" {
		t.Fatalf("PortalKeys = %v, want [p]", ks)
	}

	Portal("p", nil) // 显式移除
	if PortalActive() {
		t.Fatal("Portal(key, nil) 应移除登记")
	}
}

// TestVideoCover 封面：停在起点且未播放时显示，开播后隐藏。
func TestVideoCover(t *testing.T) {
	p := NewPlayer(NewVideoClip([]VideoFrame{
		testFrame(8, 8, 10, 20, 30, 255, 50*time.Millisecond),
	}, 20, true))
	p.Cover = "cover.png"

	if !p.CoverVisible() {
		t.Fatal("停在起点的未播放状态应显示封面")
	}
	if findKey(VideoPlayer("cv", p), "cv-cover") == nil {
		t.Fatal("封面层未渲染")
	}

	p.Play()
	if p.CoverVisible() {
		t.Fatal("开播后应隐藏封面")
	}
	if findKey(VideoPlayer("cv", p), "cv-cover") != nil {
		t.Fatal("开播后不应渲染封面层")
	}
}

// TestVideoEndedOverlay 播放结束：显示重播遮罩，隐藏后不显示。
func TestVideoEndedOverlay(t *testing.T) {
	p := NewPlayer(NewVideoClip([]VideoFrame{
		testFrame(8, 8, 10, 20, 30, 255, 50*time.Millisecond),
	}, 20, false))
	p.Loop = false
	p.SetPos(p.Duration())
	if !videoEndedShown(p) {
		t.Fatalf("应处于播放结束状态 pos=%v dur=%v", p.Pos(), p.Duration())
	}
	if findKey(VideoPlayer("ed", p), "ed-ended") == nil {
		t.Fatal("结束遮罩未渲染")
	}
	p.HideEndedOverlay = true
	if findKey(VideoPlayer("ed", p), "ed-ended") != nil {
		t.Fatal("HideEndedOverlay 后不应渲染结束遮罩")
	}
}

// bufStubSrc 只用于缓冲判定测试的解码源（始终不返回画面）。
type bufStubSrc struct{}

func (bufStubSrc) Size() (int, int)                         { return 8, 8 }
func (bufStubSrc) Duration() time.Duration                  { return time.Second }
func (bufStubSrc) Frame(time.Duration) (*VideoFrame, error) { return nil, nil }

// TestVideoBuffering 外部源播放中但还没有画面时显示缓冲层。
func TestVideoBuffering(t *testing.T) {
	p := &Player{Source: bufStubSrc{}, Playing: true}
	if !videoBuffering(p) {
		t.Fatal("外部源无画面时应判定为缓冲中")
	}
	if findKey(VideoPlayer("bf", p), "bf-buffering") == nil {
		t.Fatal("缓冲层未渲染")
	}
	p.Playing = false
	if videoBuffering(p) {
		t.Fatal("暂停时不应判定为缓冲中")
	}
}

// TestVideoHandleKey 快捷键：空格、静音、倍速、百分比跳转与锁定失效。
func TestVideoHandleKey(t *testing.T) {
	p := NewPlayer(NewVideoClip([]VideoFrame{
		testFrame(8, 8, 10, 20, 30, 255, 100*time.Millisecond),
		testFrame(8, 8, 40, 50, 60, 255, 100*time.Millisecond),
	}, 20, true))

	videoHandleKey("k", p, 0x20)
	if !p.Playing {
		t.Fatal("空格应开始播放")
	}
	videoHandleKey("k", p, 0x20)
	if p.Playing {
		t.Fatal("空格应暂停")
	}

	videoHandleKey("k", p, 0x4D) // M
	if !p.Muted {
		t.Fatal("M 应静音")
	}

	sp := p.SpeedValue()
	videoHandleKey("k", p, 0x5D) // ]
	if p.SpeedValue() <= sp {
		t.Fatalf("] 应提高倍速: %.2f -> %.2f", sp, p.SpeedValue())
	}

	videoHandleKey("k", p, 0x39) // 9 -> 90%
	if r := p.Progress(); r < 0.85 || r > 0.95 {
		t.Fatalf("9 键应跳到 90%%, got %.2f", r)
	}

	p.Locked = true
	was := p.Playing
	videoHandleKey("k", p, 0x20)
	if p.Playing != was {
		t.Fatal("锁定后快捷键应失效")
	}
}

// TestVideoWheelVolume 滚轮调音量：向上加、向下减，锁定时不响应。
func TestVideoWheelVolume(t *testing.T) {
	p := NewPlayer(NewVideoClip([]VideoFrame{
		testFrame(8, 8, 10, 20, 30, 255, 50*time.Millisecond),
	}, 20, true))
	p.Volume = 0.5

	videoWheelVolume("w", p, 96)
	if p.Volume <= 0.5 {
		t.Fatalf("滚轮向上应加音量, got %.2f", p.Volume)
	}
	videoWheelVolume("w", p, -96)
	if p.Volume >= 0.6 {
		t.Fatalf("滚轮向下应减音量, got %.2f", p.Volume)
	}

	p.Locked = true
	before := p.Volume
	videoWheelVolume("w", p, 96)
	if p.Volume != before {
		t.Fatal("锁定后滚轮不应改音量")
	}
}

// TestVideoFullscreenButton 控制条全屏按钮必须真的挂了点击处理（回归：
// 曾因批量替换误删该行，导致点全屏按钮毫无反应）。
func TestVideoFullscreenButton(t *testing.T) {
	portalReset()
	defer portalReset()

	p := NewPlayer(NewVideoClip([]VideoFrame{
		testFrame(8, 8, 10, 20, 30, 255, 50*time.Millisecond),
	}, 20, true))
	p.Playing = true // 播放中：控制条可见

	btn := findKey(VideoPlayer("fb", p), "fb-full")
	if btn == nil {
		t.Fatal("未找到全屏按钮节点 fb-full")
	}
	if btn.OnClick == nil {
		t.Fatal("全屏按钮没有点击处理，点击不会有任何反应")
	}
	btn.OnClick()
	if !p.Fullscreen {
		t.Fatal("点击全屏按钮后应进入全屏")
	}
	btn.OnClick()
	if p.Fullscreen {
		t.Fatal("再次点击全屏按钮应退出全屏")
	}
}

// TestVideoFullscreenToggle 全屏切换：状态翻转 + 回调只触发一次。
func TestVideoFullscreenToggle(t *testing.T) {
	p := NewPlayer(NewVideoClip([]VideoFrame{
		testFrame(8, 8, 10, 20, 30, 255, 50*time.Millisecond),
	}, 20, true))
	n := 0
	p.OnFullscreen = func() { n++ }

	p.ToggleFullscreen()
	if !p.Fullscreen || n != 1 {
		t.Fatalf("全屏 = %v, 回调次数 = %d", p.Fullscreen, n)
	}
	p.SetFullscreen(true) // 同值不应重复触发
	if n != 1 {
		t.Fatalf("同值 SetFullscreen 触发了回调: %d", n)
	}
	p.SetFullscreen(false)
	if p.Fullscreen || n != 2 {
		t.Fatalf("退出全屏 = %v, 回调次数 = %d", p.Fullscreen, n)
	}
}
