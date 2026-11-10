# 启动片头素材

来源：https://github.com/NativeDog1/dsh-boot-animation （v0.4.4，BSD-3-Clause，作者 NativeDog1）
原始路径：`media/` 目录下的四段 mp4。仅作片头演示素材使用，版权与许可见 `LICENSE-dsh-boot-animation.txt`。

| 文件 | 时长 | 分辨率 | 大小 |
| --- | --- | --- | --- |
| deepseek-awakening-intro.mp4 | 7.05s | 1280×720 H.264/AAC | 2,600,325 B |
| deepseek-brand-intro.mp4 | 7.05s | 1280×720 H.264/AAC | 1,308,725 B |
| deepseek-cyberpunk-intro.mp4 | 7.05s | 1280×720 H.264/AAC | 1,856,280 B |
| deepseek-startup-intro.mp4 | 8.05s | 1280×720 H.264/AAC | 3,305,269 B |

## 怎么用

桌面端启动动画会读取 `%APPDATA%\guis\deepseek\intro\`：

- 放任意一个 mp4（可多个，按文件名排序取第一个）即优先全窗口播放；
- 也支持 `.gif`，或 ≥3 张 `.png/.jpg` 组成的序列（24fps）；
- 没有素材时回落到 `cmd/deepseek/pages/splash.go` 的程序化机器人动画（5s）；
- 播放中按 `Esc` 跳过；`GUIS_NO_SPLASH=1` 完全不显示片头。

同步素材到运行目录（PowerShell）：

```powershell
$dst = Join-Path $env:APPDATA 'guis\deepseek\intro'
New-Item -ItemType Directory -Force -Path $dst | Out-Null
Copy-Item H:\JIMaX\guis\assets\intro\*.mp4 $dst -Force
```
