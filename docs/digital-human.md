# 数字人（实时语音对话）

本文说明如何在 `cmd/deepseek` 桌面端里启用「数字人」：独立无边框窗口、**OpenGL 直接渲染形象**、
与 AI 助手实时语音对话并支持抢话打断。

- 形象生成引擎：[LiveTalking](https://github.com/lipku/LiveTalking)（哔哩哔哩 lipku，Apache-2.0）
- 媒体服务器：mediamtx（单 exe，本地 RTMP → HTTP-FLV）
- 渲染：本项目的 `ui.Player`（ffmpeg 解码 → 帧 → GL 纹理 → OpenGL 绘制），**不嵌任何浏览器**

---

## 1. 架构

```
┌─ cmd/deepseek ──────────────────────────────┐
│  主窗口（对话）                               │
│   ├─ 数字人窗口（无标题栏 / 无菜单）           │
│   │   └─ ui.Video(player) → OpenGL 绘制      │
│   └─ 会话状态机 idle/listen/asr/think/speak   │
│                                              │
│   LiveTalking HTTP 客户端（驱动 / 打断 / 状态）│
└───────┬───────────────────────┬──────────────┘
   拉流 │ http-flv:8888          │ HTTP:8010
┌───────┴────────┐    rtmp:1935  ┌┴──────────────┐
│ mediamtx       │◀──────────────│ LiveTalking   │
│ RTMP→HTTP-FLV  │               │ 推理+口型+TTS   │
└────────────────┘               └───────────────┘
```

数据流：

1. agent 流式回答 → 按句切分（与本地播报同一套规则）→ `POST /human {sessionid:"0", text, type:"echo"}`
2. LiveTalking 合成语音 + 口型 + 推流 → mediamtx → ffmpeg → `ui.Player` → 窗口画面 + 声音
3. 麦克风 → VAD 断句 → whisper 转写 → 自动发送 → 回到第 1 步
4. 说话中检测到用户开口 ≥300ms（或点「打断」）→ `POST /interrupt_talk` + 清空待播句 → 回到聆听

---

## 2. 前置条件

| 项 | 要求 |
|---|---|
| 显卡 | NVIDIA GPU（LiveTalking 的 wav2lip / musetalk 推理）；无卡也能把界面跑起来，但出不了画面 |
| Python | 3.10（conda 环境，脚本会建） |
| 媒体服务器 | mediamtx.exe（脚本会下载到 `tools/mediamtx/`） |
| 解码器 | `ffmpeg` / `ffprobe` 在 PATH（项目的录屏 / 影视剧场已在用） |
| 语音识别 | whisper.cpp（可选，只在用语音输入时需要；见 `pages/asr.go` 自动探测） |

---

## 3. 部署

### 3.1 应用内一键安装（推荐）

数字人窗口 → 【设置】 → 「引擎自动安装」→ **一键安装**；画面区未就绪时也可直接点「一键安装引擎」。全程不用命令行：

1. 检查 git / Python 3.9+ / NVIDIA 驱动（缺失会直接报错并说明）
2. 获取 LiveTalking（有 git 走 `git clone --depth 1`，无 git 则下 GitHub 源码 zip）
3. 建 `.venv` → 升级 pip → 装 PyTorch（有 N 卡走 `cu124` 轮子，无卡走 `cpu`）+ `requirements.txt`
4. 下载 mediamtx 到 `mediamtx/`（GitHub 最新发布，自动挑 `windows_amd64.zip`）
5. 自动回填配置：引擎目录 / Python（venv 解释器）/ mediamtx

| 项 | 说明 |
|---|---|
| 默认目录 | `<工作目录>/tools/digital-human`（可在「安装目录」里填绝对路径改） |
| 体积 / 耗时 | 约 3-5 GB，10-40 分钟（看网速）；界面有实时日志 + 进度条，可随时取消 |
| 不自动下载 | 模型权重、形象素材（授权与体积原因），装完按第 5 节手动放 |
| 失败重试 | 进度区会显示错误与最后几行日志；换镜像 / 查代理后点「重试」即可（已完成的步骤不会重装） |

### 3.2 脚本安装（无 GUI / 批量部署）

```powershell
# 1) 安装环境（检查 GPU/conda、克隆 LiveTalking、装 torch + 依赖、下载 mediamtx）
powershell -ExecutionPolicy Bypass -File scripts/livetalking/setup.ps1 -Dir D:\LiveTalking

# 2) 放模型权重与形象素材（脚本末尾会打印具体位置）

# 3) 启动引擎（mediamtx + LiveTalking）
powershell -ExecutionPolicy Bypass -File scripts/livetalking/start.ps1 -Dir D:\LiveTalking
```

也可以不跑 `start.ps1`，直接在本应用里点「启动引擎」（需先在设置里填 LiveTalking 目录）。

---

## 4. App 内配置

入口：输入框工具条的 👤 按钮，或侧栏「更多 → 数字人」。窗口内点「设置」可改以下项（持久化到 prefs `ds.dh.*`）：

| 配置 | prefs 键 | 默认值 | 说明 |
|---|---|---|---|
| 数字人模式 | `ds.dh.on` | 关 | 开启后本地 TTS 静音，回答改为逐句驱动数字人 |
| 接口地址 | `ds.dh.api` | `http://127.0.0.1:8010` | LiveTalking HTTP |
| 拉流地址 | `ds.dh.stream` | `http://127.0.0.1:8888/live/livestream` | mediamtx HTTP-FLV |
| 会话 ID | `ds.dh.session` | `0` | `--transport=rtmp` 模式下服务端预建的固定会话 |
| 引擎目录 | `ds.dh.dir` | 空 | 含 `app.py` 的 LiveTalking 目录（填了才能一键启动） |
| Python | `ds.dh.python` | `python` | 建议填 conda 环境的 python.exe 绝对路径 |
| 启动参数 | `ds.dh.args` | `--transport=rtmp` | 追加给 app.py 的参数 |
| mediamtx | `ds.dh.mediamtx` | 空 | 填了则启动引擎时一并拉起 |
| 窗口尺寸 | `ds.dh.w` / `ds.dh.h` | 360 / 640 | 数字人窗口初始大小 |
| 连续对话 | `ds.dh.continuous` | 开 | 一轮说完自动回到聆听 |
| VAD 阈值 | `ds.dh.vad` | 6 | 能量阈值（百分比），嘈杂环境调高 |
| 静音断句 | `ds.dh.silence` | 600 | 说完多久算一句（毫秒） |

> 会话 ID 为何是 `0`：LiveTalking 只在 `POST /offer`（WebRTC）或 `--transport=rtmp|virtualcam`
> 启动时才建会话，后者直接用 `'0'`（`app.py:167`）。我们走 RTMP，所以固定 `0`。

---

## 5. 形象素材

LiveTalking 用自己的素材目录（具体路径以它官方 README 为准）：

- 说话视频 `data/avatars/<avatar_id>.mp4`：正面、光照均匀、嘴部清晰，推荐 25fps；
- 全身拼接图 `data/avatars/full_body_img/`：可选，用于半身/全身拼接；
- 启动时用 `-Avatar <avatar_id> -Model <wav2lip|musetalk|ernerf|ultralight>` 指定。

---

## 6. 故障排查

| 现象 | 原因与处理 |
|---|---|
| 窗口黑屏 / 一直「正在连接画面」 | ffmpeg 不在 PATH，或拉流地址错；先用 `ffplay http://127.0.0.1:8888/live/livestream` 验证流是否存在 |
| 引擎「启动中」卡住 90s | 显存不足或权重缺失；窗口状态条下方有日志，或看 `start.ps1` 控制台输出（重点看 CUDA out of memory / model not found） |
| 端口占用（8010 / 1935 / 8888） | `netstat -ano | findstr :8010` 找到进程结束；mediamtx 重复启动也会报错 |
| 数字人不开口 | 1) 会话 ID 不是 `0`；2) LiveTalking 启动时没带 `--transport=rtmp`；3) 看日志里 `/human` 是否返回 `session not found` |
| 听到两重声音 | 数字人模式没开（`ds.dh.on=0`），本地 TTS 与数字人同时在说；开数字人模式会自动静音本地播报 |
| 打断不生效 | LiveTalking 版本较老、无 `/interrupt_talk`；升级或改用「停止引擎」 |
| 自动安装卡在 PyTorch | 网络到 download.pytorch.org 很慢：配置代理后点「重试」，或先用 `scripts/livetalking/setup.ps1` 走 conda；已装好的步骤不会重复 |
| 自动安装提示找不到 Python | 安装 Python 3.10+ 并勾选「Add to PATH」（或直接跑脚本安装，脚本用 conda） |
| mediamtx 下载失败 | GitHub API 限流 / 网络问题：手动下载后解压到 `<安装目录>\\mediamtx\\`，再在设置里填该 exe 路径 |

---

## 7. 代码结构

| 文件 | 职责 |
|---|---|
| `pages/dh_config.go` | 配置（prefs `ds.dh.*`） |
| `pages/dh_client.go` | LiveTalking HTTP 客户端（/human、/interrupt_talk、/is_speaking、/set_audiotype） |
| `pages/dh_engine.go` | 引擎管家：mediamtx + app.py 子进程、日志、探活、进程树清理 |
| `pages/dh_window.go` | 独立无边框窗口 + Player 渲染 + 断流自愈 + 内联设置 |
| `pages/dh_drive.go` | 按句驱动（串行队列、失败重试 1 次、取消清空） |
| `pages/speech_split.go` | 流式分句器（本地播报与数字人共用） |
| `pages/dh_vad.go` | VAD 能量判定（纯逻辑） |
| `pages/dh_record_windows.go` | winmm `waveIn` 流式录音（16k/16bit/mono，20ms 帧） |
| `pages/dh_session.go` | 会话状态机（聆听→识别→思考→说话） |
| `pages/dh_install.go` | 引擎自动安装：前置检查 / 源码 / venv / torch / 依赖 / mediamtx / 回填配置，可取消 |
| `pages/dh_install_ui.go` | 设置页安装区块（进度条 / 日志 / 重试）与画面区一键安装入口 |
| `pages/dh_barge.go` | 打断（抢话 ≥300ms / 手动按钮，1s 防抖） |
| `scripts/livetalking/*.ps1` | 环境安装与启动脚本 |

测试：`go test ./cmd/deepseek/pages/ -run 'TestDH|TestSpeech|TestVAD|TestRMS'`
