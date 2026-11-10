<!-- group: 媒体与文档 -->
# 视频播放 · Player / VideoPlayer / 解码后端

## 1. 三步播放一个文件

```go
p, err := ui.NewMediaPlayer("movie.mp4")
if err != nil { log.Fatal(err) }
defer p.Close()          // 务必释放解码进程与音频设备

p.Play()
return ui.VideoPlayer("vp", p)
```

`VideoPlayer(key, p)` 自带：画面 + 底部渐变控制条（进度拖动、音量、倍速、循环、旋转、锁定、
全屏、上/下一集）。只要画面就 `ui.Video(p)`。

## 2. 播放器状态 Player（业务持有）

| 字段 | 说明 |
| --- | --- |
| `Clip *VideoClip` | 内存帧序列（GIF / 图片序列）；与 `Source` 二选一，同时存在优先 `Clip` |
| `Source VideoSource` | 外部帧来源（ffmpeg / Media Foundation / 自研） |
| `Playing / Loop / Speed / Volume / Muted` | 播放状态 |
| `Fit VideoFit` | `FitContain / FitCover / FitFill` |
| `StepMode` | 逐帧模式（按帧序号 + 毫秒精确定位） |
| `Rotation` | 0~3，单位 90° 顺时针 |
| `Locked` | 锁定画面：点击与空格/方向键失效 |
| `AutoHide` | 播放中自动隐藏控制条 |
| `HideControls` | 纯画面 |
| `Fullscreen` | 播放器铺满窗口（需放在根 Stack 中） |
| `Subtitle` | 底部字幕 |
| `OnPlay / OnPause / OnSeek / OnVolume / OnLoopChange / OnFullscreen / OnPrev / OnNext` | 回调 |

方法：

```go
p.Play(); p.Pause(); p.TogglePlay()
p.SetPos(30*time.Second); p.SeekBy(-5*time.Second); p.SeekRatio(0.5)
p.Pos(); p.Duration(); p.Progress(); p.PlayingNow()
p.SetSpeed(1.5); p.SpeedValue()        // 夹在 0.25~4
p.SetVolume(0.8); p.ToggleMute()
p.SetLoop(true)
p.SetRotation(1); p.RotateBy(-1); p.RotationDeg()
p.SetLocked(true); p.ToggleLock()
p.Frame(); p.FrameIndex(); p.Size(); p.Info(); p.HasAudio(); p.Err()
p.SetClip(clip); p.Close()
```

## 3. 打开方式

```go
ui.NewMediaPlayer(path)                     // 默认跟随源尺寸 + 声音
ui.NewMediaPlayerWith(path, ui.MediaConfig{
    FFmpeg: "D:/ffmpeg/bin/ffmpeg.exe",     // 空 = PATH
    MaxWidth: 1280, MaxHeight: 720,         // <=0 跟随源
    FPS: 30,                                // 0 = 跟随源
    NoAudio: false,
    Volume: 1,
    HWAccel: true,                          // 尝试 D3D11VA，失败自动软解
})
ui.OpenMediaInto(p, otherPath)              // 换片（旧解码源自动关闭）
ui.ProbeMedia(path)                         // 只探测信息
ui.MediaFormats(); ui.IsMediaPath(path); ui.MediaFilters   // 界面辅助
```

## 4. 内存帧序列（GIF / 图片序列）

```go
clip, err := ui.LoadGIF("anim.gif")            // 按 disposal 规则合成完整 RGBA 帧
p := ui.NewPlayer(clip)
p.Loop = true
ui.VideoPlayer("gif", p)

clip2, _ := ui.LoadImageSequence([]string{"1.png", "2.png"}, 24, true)
frames := []ui.VideoFrame{{Pix: rgbaBytes, W: 640, H: 360, Delay: 40 * time.Millisecond}}
clip3 := ui.NewVideoClip(frames, 24, true)
```

`VideoClip` 方法：`FrameDelay(i) / Duration() / Size() / FrameCount() / FrameStart(i) /
FrameIndexAt(pos)`；`VideoFrame{Pix, W, H, Delay}` 的 `Pix` 是 RGBA8 非预乘像素。

## 5. 平台解码后端

| 平台 | 后端 | 说明 |
| --- | --- | --- |
| Windows | ffmpeg（优先） / Media Foundation（回退） | 无 ffmpeg 时自动走系统解码器（RGB32→NV12→YUY2 逐级尝试） |
| Linux / macOS | ffmpeg | 需 PATH 里有 `ffmpeg` / `ffprobe` |
| 浏览器 | `<video>` 原生 | `.m3u8` 走 hls.js；`blob:` / `data:` 按视频处理 |
| Android | 占位桩 | 无解码后端（接口可用，不出画面） |

```go
ui.BackendName()      // "ffmpeg" / "system" / "browser" / ""
ui.BackendLabel(name) // 中文标签
ui.FormatBackend(name)
ui.DecoderPreference() // 读 GUIS_DECODER（"system" 强制系统解码）
```

能力差异：系统解码器只支持系统已装 MFT（H.264 / AAC / MP3 / WMA…），倍速固定 1x，
拖动预览保留上一帧；ffmpeg 支持全部容器 + `atempo` 变速 + 单帧抓取。

## 6. 全屏

```go
p.Fullscreen = true         // 播放器铺满窗口（必须放在根 Stack 里）
ui.Stack(pageBody(), ui.Video(p).St(ui.St().AbsFill()))
```

窗口级全屏用 `app.SetFullscreen(true)`（ESC 退出）；最小化恢复时框架调用
`ui.ResumeMediaAll()` 自动重建解码窗口，画面不会停在最小化前那一帧。

## 7. 自定义视频源

只需实现三个方法：

```go
type VideoSource interface {
    Size() (int, int)
    Duration() time.Duration
    Frame(pos time.Duration) (*ui.VideoFrame, error)   // nil, nil = 该位置暂无画面
}

p := ui.NewPlayerOf(mySource)
```

`ui.VideoSurface(key, p)` 是纯画面节点（每帧自行推进播放时钟），
`ui.VideoSurfaceTick(key, p)` 同样推进但不点亮全局续帧（省 CPU，适合后台小窗）。

## 8. 坑位

* 忘记 `Close()` → ffmpeg 子进程残留 + 音频设备不释放。
* 同一界面多个播放器的 `key` 必须唯一。
* 浏览器端 `.m3u8` 需要 `hls.js`（仓库 `web/hls.min.js` 已带）。
* 播放器默认懒创建；未打开过视频时写 `Fullscreen` 前要判空（gallery 曾因此全屏黑屏）。
