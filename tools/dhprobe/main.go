// guis tools/dhprobe — 数字人拉流链路冒烟检查（与 dh_window.go 用同一套参数）
//
//	go run ./tools/dhprobe [流地址]
//
// 不打开窗口；默认和数字人窗口一样带声音（播放时钟以音频设备为准）。
// 加 -mute 则不音频，改为纯墙钟推进，用来看解码端能不能持续出帧。
package main

import (
	"fmt"
	"os"
	"time"

	"guis/ui"
)

func main() {
	url := "rtmp://127.0.0.1:1935/live/livestream"
	noAudio := false
	for _, a := range os.Args[1:] {
		if a == "-mute" {
			noAudio = true
			continue
		}
		url = a
	}
	fmt.Println("拉流地址:", url, "静音:", noAudio)

	info, err := ui.ProbeMedia(url)
	if err != nil {
		fmt.Println("[FAIL] ffprobe:", err)
		return
	}
	fmt.Printf("探测: %dx%d fps=%.2f video=%v audio=%v\n", info.Width, info.Height, info.FPS, info.HasVideo, info.HasAudio)

	cfg := ui.MediaConfig{HWAccel: true, FPS: 25, MaxWidth: 840, MaxHeight: 1520, NoAudio: noAudio}
	src, err := ui.OpenMediaWith(url, cfg)
	if err != nil {
		fmt.Println("[FAIL] OpenMedia:", err)
		return
	}
	defer src.Close()

	start := time.Now()
	frames, calls := 0, 0
	var firstAt time.Duration
	for time.Since(start) < 8*time.Second {
		pos := time.Since(start)
		// 和 Player 一样：有音频时钟就跟音频，否则按墙钟
		if cp, valid := src.MediaClock(); valid {
			pos = cp
		}
		f, err := src.Frame(pos)
		calls++
		if err != nil {
			fmt.Println("[FAIL] Frame:", err)
			return
		}
		if f != nil {
			if frames == 0 {
				firstAt = time.Since(start)
			}
			frames++
		}
		time.Sleep(40 * time.Millisecond)
	}
	el := time.Since(start).Seconds()
	fmt.Printf("调用 %d 次，出帧 %d 次（%.1f fps），首帧 %v，最后位置 %v\n",
		calls, frames, float64(frames)/el, firstAt.Round(time.Millisecond), src.Duration())
	if frames == 0 {
		fmt.Println("[FAIL] 8 秒内没有任何画面")
		return
	}
	fmt.Println("[OK] 拉流解码正常")
}
