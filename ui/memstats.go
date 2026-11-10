// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
)

// memStatsEnabled 由环境变量 GUIS_MEMSTATS=1 开启：每 120 帧输出一次内存构成，
// 便于定位占用来源（字形图集 / Go 堆 / 每帧节点树 / GPU 纹理）。
var memStatsEnabled = os.Getenv("GUIS_MEMSTATS") != ""

// logTrim 输出一次空闲/最小化回收的结果（需 GUIS_MEMSTATS=1）。
// 回收后不会再有新帧，逐 120 帧的常规统计刷新不到，因此单独打一行。
func (a *App) logTrim(offBefore, offAfter int64, deep bool) {
	if !memStatsEnabled {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	level := "cache"
	if deep {
		level = "cache+pages"
	}
	fmt.Fprintf(os.Stderr, "[guis] idle-trim(%s) frame=%d heap=%.1fMB offscreen=%.1fMB->%.1fMB\n",
		level, a.frameCount, float64(m.HeapAlloc)/(1<<20),
		float64(offBefore)/(1<<20), float64(offAfter)/(1<<20))
}

// heapLimitMB Go 堆软上限（MB，0 = 不设限）。GUIS_HEAP_MB 覆盖，默认 0。
//
// 为何默认关闭：Go 的软上限靠“GC 提前介入”实现，万一业务一侧的存活堆本来就大
//（大文档 / 图片解码缓冲），上限低于存活量时 GC 会连续运行，帧率会崩。
// 内存吃紧的机器可以设 GUIS_HEAP_MB=96 之类的值，让 GC 早点开工。
var heapLimitMB = func() int {
	if v := os.Getenv("GUIS_HEAP_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 0
}()

// applyHeapLimit 设置 Go 堆软上限（幂等，newApp 时调用一次）。
func applyHeapLimit() {
	if heapLimitMB <= 0 {
		return
	}
	debug.SetMemoryLimit(int64(heapLimitMB) << 20)
}

func (a *App) logMemStats() {
	// 首帧也输出一次：字体图集/纹理在首帧就已建好，早期数据最好定位「一开窗就占多少」；
	// 之后每 120 帧一次。
	if !memStatsEnabled || (a.frameCount != 1 && a.frameCount%120 != 0) {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	atlas := 0.0
	fonts := 0
	if a.fonts != nil {
		atlas = float64(a.fonts.atlasBytes()) / (1 << 20)
		fonts = len(a.fonts.fonts)
	}
	imgs := 0
	if a.textures != nil {
		imgs = a.textures.count()
	}
	// GPU 侧：离屏目标（超采样 RT + 毛玻璃中间层）与纹理缓存。
	// 这两项才是常驻显存的大头，Go 堆通常只有十几 MB。
	var off, imgBytes, vertBytes int64
	if a.r != nil {
		off = a.r.OffscreenBytes()
		vertBytes = a.r.VertBytes()
	}
	if a.textures != nil {
		a.textures.mu.Lock()
		imgBytes = a.textures.bytes
		a.textures.mu.Unlock()
	}
	fmt.Fprintf(os.Stderr,
		"[guis] frames=%d heap=%.1fMB inuse=%.1fMB sys=%.1fMB gc=%d fonts=%d atlas=%.1fMB images=%d imgtex=%.1fMB charttex=%.1fMB offscreen=%.1fMB verts=%.1fMB layers=%.1fMB/%d\n",
		a.frameCount,
		float64(m.HeapAlloc)/(1<<20),
		float64(m.HeapInuse)/(1<<20),
		float64(m.Sys)/(1<<20),
		m.NumGC, fonts, atlas, imgs,
		float64(imgBytes)/(1<<20),
		float64(a.chartBytes)/(1<<20),
		float64(off)/(1<<20),
		float64(vertBytes)/(1<<20),
		float64(a.layerBytes())/(1<<20), len(a.layers))
}
