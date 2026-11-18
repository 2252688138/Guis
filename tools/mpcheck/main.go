// guis tools/mpcheck — 在线音乐链路冒烟检查（网易云歌单 / 歌词 / 封面，36DJ 列表 / 音频）
//
//	go run ./tools/mpcheck
//
// 只验证接口与代理是否通（下载前 64KB 音频），不打开窗口、不占用声卡。
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"guis/cmd/deepseek/musicsvc"
	"guis/ui"
)

// hasFlag 参数里是否带某个开关。
func hasFlag(f string) bool {
	for _, a := range os.Args[1:] {
		if a == f {
			return true
		}
	}
	return false
}

// playProbe 用与界面完全相同的链路打开音频并播 4 秒：
// 验证 ffprobe 能拿到时长、waveOut 能出声、频谱能从 PCM 算出能量。
func playProbe(u string) {
	fmt.Println("       正在用 ui.OpenMusic 打开音频（4 秒）…")
	src, err := ui.OpenMusic(u)
	if err != nil {
		fmt.Println("[FAIL] OpenMusic：", err)
		return
	}
	defer src.Close()
	src.SetVolume(0.2)
	src.Play()
	time.Sleep(4 * time.Second)
	peak := float32(0)
	if sp := src.Spectrum(); sp != nil {
		for _, v := range sp.Bands() {
			if v > peak {
				peak = v
			}
		}
	}
	fmt.Printf("       时长=%s 位置=%s 出声=%v 频谱峰值=%.2f\n",
		src.Duration().Round(time.Millisecond), src.Pos().Round(time.Millisecond), src.HasAudio(), peak)
	if src.HasAudio() && src.Pos() <= 0 {
		fmt.Println("[FAIL] 播放位置没有推进")
	}
}

func main() {
	fail := 0
	check := func(name string, err error) {
		if err != nil {
			fail++
			fmt.Printf("[FAIL] %-28s %v\n", name, err)
			return
		}
		fmt.Printf("[ ok ] %s\n", name)
	}

	if err := musicsvc.Start(); err != nil {
		fmt.Println("代理服务启动失败：", err)
		os.Exit(1)
	}
	fmt.Println("代理地址：", musicsvc.Base())

	name, songs, err := musicsvc.Playlist(3778678)
	check(fmt.Sprintf("歌单 %s（%d 首）", name, len(songs)), err)
	if err == nil && len(songs) > 0 {
		s := songs[0]
		fmt.Printf("       首曲：%s - %s（%d ms）\n", s.Name, s.Artist, s.Duration)
		_, lerr := musicsvc.Lyric(s.ID)
		check("歌词接口", lerr)
		if s.Cover != "" {
			if _, _, cerr := musicsvc.FetchPic(s.Cover); cerr == nil {
				check("封面下载", nil)
				fmt.Printf("       主色：%s\n", musicsvc.CoverColor(s.Cover))
			} else {
				check("封面下载", cerr)
			}
		}
		check("网易云音频代理（前 64KB）", headBytes(musicsvc.StreamURL(s.ID)))
		if hasFlag("-play") {
			playProbe(musicsvc.StreamURL(s.ID))
		}
	}

	res, serr := musicsvc.Search("周杰伦", 5)
	check(fmt.Sprintf("搜索（%d 条）", len(res)), serr)

	dj, derr := musicsvc.DJList("xc", 1)
	check(fmt.Sprintf("DJ 列表（%d 首）", len(dj)), derr)
	if derr == nil && len(dj) > 0 {
		fmt.Printf("       首曲：%s %s\n", dj[0].Name, dj[0].Hot)
		check("DJ 音频代理（前 64KB）", headBytes(musicsvc.DJStreamURL(dj[0].ID)))
	}

	if fail > 0 {
		fmt.Printf("\n%d 项失败\n", fail)
		os.Exit(1)
	}
	fmt.Println("\n全部通过")
}

// headBytes 取前 64KB 音频，确认代理返回音频流而不是错误页。
func headBytes(u string) error {
	if u == "" {
		return fmt.Errorf("地址为空")
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", "bytes=0-65535")
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("HTTP %d：%s", resp.StatusCode, string(body))
	}
	n, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if n < 1024 {
		return fmt.Errorf("只读到 %d 字节", n)
	}
	fmt.Printf("       已读取 %d 字节 · %s\n", n, resp.Header.Get("Content-Type"))
	return nil
}
