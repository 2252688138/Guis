<!-- group: 媒体与文档 -->
# 音频播放与频谱 · MusicSource / MusicPlayer / Spectrum

## 1. 一步播放

```go
src, err := ui.OpenMusicPlayer("song.mp3")   // 打开即播放
if err != nil { log.Fatal(err) }
defer src.Close()

return ui.MusicPlayer("mp", src, ui.MusicPlayerOptions{
    Track: ui.MusicTrack{Title: "曲名", Artist: "歌手", Album: "专辑", Cover: "cover.jpg"},
    Bars:  40,                                  // 频谱柱数
    Prev:  prevTrack, Next: nextTrack,          // nil 时按钮置灰
    HideSpectrum: false,
})
```

支持的容器：mp3 / m4a / m4b / aac / flac / wav / ogg / opus / wma / ape / aiff。
`ui.MusicFilters` 是现成的文件对话框过滤器，`ui.IsMusicPath` / `ui.MusicFormats` 用于判断与展示。

## 2. MusicSource API

```go
ui.OpenMusic(path)                       // 打开不自动播放
ui.OpenMusicWith(path, ui.MediaConfig{}) // 指定 ffmpeg 路径 / 音量等
ui.OpenMusicPlayer(path)                 // 打开 + 立即播放

src.Play(); src.Pause(); src.TogglePlay(); src.Playing()
src.Seek(30*time.Second); src.SeekRatio(0.5); src.Pos(); src.Progress(); src.Duration()
src.SetSpeed(1.25); src.Speed()          // 0.5~2，atempo 重采样
src.SetVolume(0.7); src.Volume(); src.ToggleMute(); src.Muted()
src.SetLoop(true); src.Loop()
src.Tick()                               // 每帧调用：播完按循环设置重播或停住
src.Spectrum(); src.NowPlaying(); src.Info(); src.HasAudio(); src.Err(); src.Close()
```

`Tick()` 返回 / 语义：组件里每帧调一次，用来驱动「播完自动下一首」。

## 3. 频谱组件

```go
ui.MusicSpectrum("s1", src.Spectrum(), 40, "#f5c518", "#8b5cf6").St(ui.St().Wp(100).H(78))
ui.MusicMirrorSpectrum("s2", src.Spectrum(), 56, "#22d3ee", "#818cf8").St(ui.St().Wp(100).H(72))
```

* 柱状（底部对齐）与镜像（中线对称的「声浪」）两种画法。
* 颜色可传渐变起止，柱高由对数频段能量驱动。

## 4. 独立使用 Spectrum（任意 PCM 来源）

```go
sp := ui.NewSpectrum(48)                 // 48 个对数频段（默认 40）
sp.PushPCM(pcm16, 2, 48000)              // 解码线程调用：s16le，多声道取平均
bands := sp.Bands()                      // UI 线程每帧读，0~1，已做 attack/release 平滑
peaks := sp.Peaks()                      // 峰值（缓慢回落），画顶端小点
sp.Reset()                               // 切歌 / 跳转时清空
```

内部是 FFT + 汉宁窗 + 对数频段边界；解码线程写、UI 线程读，锁粒度很小。

## 5. 播放列表

```go
tracks := []ui.MusicTrack{
    {Path: "a.mp3", Title: "A", Artist: "X", Dur: 3 * time.Minute},
    {Path: "b.flac", Title: "B", Artist: "Y"},
}
ui.MusicPlaylist("pl", tracks, activeIndex, func(i int) { playIndex(i) })
```

当前曲目高亮并带「播放中」标签。

## 6. 音频后端

| 平台 | 输出 | 说明 |
| --- | --- | --- |
| Windows | waveOut | 与视频共用 ffmpeg 子进程 |
| Linux | ALSA | `internal/audio` |
| macOS | AudioQueue | `internal/audio` |
| 浏览器 | `<audio>` / WebAudio | wasm 端按 `<video>` 元素推进时间轴 |
| Android | 占位桩 | 无声音、按墙钟推进时间轴 |

没装 ffmpeg 时 Windows 走 Media Foundation（`ui.BackendName()` 会返回 `"system"`）；
占位桩平台仍可编译运行，`Duration()` 返回 0（界面显示 `--:--`）。

## 7. 音量 / 倍速细节

* `SetVolume(0)` 视为静音；`ToggleMute()` 会保留静音前音量。
* 倍速通过 ffmpeg `atempo` 实现，**需要重启音频窗口**，位置会重新对齐（不会跳）。
* `Seek` 同样重开窗口，声音与频谱一起定位。

## 8. 坑位

* `Close()` 必须调（否则解码进程与音频设备泄漏）。
* `MusicPlayer` 的 `key` 在同一界面内唯一。
* 频谱未推 PCM 时全部为 0（不是 bug）；先确认解码线程在跑。
* 浏览器自动播放策略要求「静音起播」，框架会先静音 `play()` 成功后再恢复期望音量。
