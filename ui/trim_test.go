// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"testing"

	"guis/internal/gpu"
)

// TestTrimNilSafe 回收接口必须对空 App / 空渲染器安全：
// 关窗竞态、后台 goroutine 调用、无 GL 上下文的平台都会走到这些路径。
func TestTrimNilSafe(t *testing.T) {
	var a *App
	a.trimCaches(true) // nil App
	(&App{}).trimCaches(false)
	(&App{}).chartReleaseAll()
	(&App{}).chartReleaseIdle()
	(&imageTextures{}).releaseAll(nil)
	(&imageTextures{}).releaseIdle(nil, 0)
	(&App{}).logTrim(0, 0, true) // 统计关闭时不输出
}

// TestImageReleaseAll 释放后条数保留但纹理记账归零，便于回到页面时重新解码。
func TestImageReleaseAll(t *testing.T) {
	tx := newImageTextures()
	if n := tx.count(); n != 0 {
		t.Fatalf("初始应为 0，实际 %d", n)
	}
	e := tx.entry("a.png")
	e.bytes = 1024
	tx.bytes = 1024
	tx.releaseAll(nil) // 无渲染器：只清记账，不删纹理
	if tx.bytes != 0 {
		t.Fatalf("记账应归零，实际 %d", tx.bytes)
	}
	if n := tx.count(); n != 1 {
		t.Fatalf("条目应保留以便重新解码，实际 %d", n)
	}
}

// TestImageReleaseIdle 浅空闲只放掉「最后一帧没画到」的图片纹理。
//
// 可见图片（皮肤壁纸 / 头像 / 缩略图）的纹理必须留着：一旦释放，
// 用户一动鼠标唤醒的首帧要重新异步解码，纹理回来之前那块区域是空的，
// 整窗看起来就闪一下。翻走的图片照常释放。
func TestImageReleaseIdle(t *testing.T) {
	tx := newImageTextures()
	vis := tx.entry("wall.png") // 最后一帧画在屏幕上
	vis.tex, vis.bytes, vis.stamp = &gpu.Texture{}, 2048, 10
	gone := tx.entry("old.png") // 上一页的图，最后一帧没画到
	gone.tex, gone.bytes, gone.stamp = &gpu.Texture{}, 1024, 7
	gone.started.Store(true)
	tx.bytes = 3072

	tx.releaseIdle(&gpu.Renderer{}, 10) // 最后一帧帧号 = 10

	if vis.tex == nil || vis.bytes != 2048 {
		t.Fatal("可见图片纹理不应被释放")
	}
	if gone.tex != nil || gone.bytes != 0 || gone.started.Load() {
		t.Fatal("没画到的图片纹理应被释放并允许重新解码")
	}
	if tx.bytes != 2048 {
		t.Fatalf("记账应剩 2048，实际 %d", tx.bytes)
	}
	if n := tx.count(); n != 2 {
		t.Fatalf("条目应保留 2 条，实际 %d", n)
	}
}

// TestChartReleaseIdle 浅空闲只放掉「最后一帧没画到」的图表纹理：
// 可见图表若被释放，唤醒首帧要同步重跑 rasterPlot，重建前该区域是空的。
func TestChartReleaseIdle(t *testing.T) {
	live := &chartCache{tex: &gpu.Texture{}, bytes: 100, stamp: 20}
	old := &chartCache{tex: &gpu.Texture{}, bytes: 50, stamp: 12}
	a := &App{
		r:              &gpu.Renderer{},
		chartList:      []*chartCache{live, old},
		chartBytes:     150,
		chartFrameBase: 20, // 最后一帧从图号 20 开始
	}

	a.chartReleaseIdle()

	if len(a.chartList) != 1 || a.chartList[0] != live {
		t.Fatalf("只应保留可见图表，实际 %d 条", len(a.chartList))
	}
	if a.chartBytes != 100 {
		t.Fatalf("记账应剩 100，实际 %d", a.chartBytes)
	}
	if old.tex != nil || old.bytes != 0 {
		t.Fatal("没画到的图表纹理应被释放")
	}
}
