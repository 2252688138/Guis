# LiveTalking 数字人环境安装（Windows）
#
# 用法（在仓库根目录执行）：
#   powershell -ExecutionPolicy Bypass -File scripts/livetalking/setup.ps1 -Dir D:\LiveTalking
#
# 做的事：
#   1. 检查 NVIDIA 驱动 / conda / python 版本
#   2. 克隆 lipku/LiveTalking（如目录已存在则跳过）
#   3. 建 conda 环境（python 3.10）+ 装 torch(cu124) + requirements.txt
#   4. 下载 mediamtx 到 tools\mediamtx（本地 RTMP → HTTP-FLV 媒体服务器）
#   5. 打印权重 / 形象素材 / 启动方式提示
#
# 注意：不自动下载第三方模型权重（合规 + 体积大），按第 5 步提示手动放置。

param(
    [string]$Dir = "D:\LiveTalking",
    [string]$EnvName = "livetalking",
    [string]$TorchIndex = "https://download.pytorch.org/whl/cu124",
    [switch]$SkipMediaMtx
)

$ErrorActionPreference = "Stop"

function Info($m) { Write-Host "[setup] $m" -ForegroundColor Cyan }
function Warn($m) { Write-Host "[setup] $m" -ForegroundColor Yellow }
function Fail($m) { Write-Host "[setup] $m" -ForegroundColor Red; exit 1 }

# ---- 1. 环境检查 ----
Info "检查 NVIDIA 驱动"
if (-not (Get-Command nvidia-smi -ErrorAction SilentlyContinue)) {
    Warn "未找到 nvidia-smi：LiveTalking 需要 NVIDIA 显卡（wav2lip/musetalk 推理）"
} else {
    nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader
}

if (-not (Get-Command conda -ErrorAction SilentlyContinue)) {
    Fail "未找到 conda：请先安装 Miniconda（或用 -EnvName 配合已有的 venv 手动装）"
}

# ---- 2. 拉取代码 ----
if (Test-Path (Join-Path $Dir "app.py")) {
    Info "已有 LiveTalking：$Dir（跳过克隆）"
} else {
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Fail "未找到 git" }
    Info "克隆 LiveTalking 到 $Dir"
    git clone --depth 1 https://github.com/lipku/LiveTalking.git $Dir
}

# ---- 3. Python 环境 ----
Info "创建 / 复用 conda 环境 $EnvName (python 3.10)"
$envExists = (conda env list) -match "^\s*$EnvName\s"
if (-not $envExists) {
    conda create -y -n $EnvName python=3.10
}

Info "安装 PyTorch（CUDA 12.4 轮子，GPU 机器专用）"
conda run -n $EnvName python -m pip install --upgrade pip
conda run -n $EnvName pip install torch torchvision torchaudio --index-url $TorchIndex

Info "安装 LiveTalking 依赖"
Push-Location $Dir
conda run -n $EnvName pip install -r requirements.txt
Pop-Location

# ---- 4. mediamtx ----
if (-not $SkipMediaMtx) {
    $mtxDir = Join-Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) "tools\mediamtx"
    $mtxExe = Join-Path $mtxDir "mediamtx.exe"
    if (Test-Path $mtxExe) {
        Info "已有 mediamtx：$mtxExe"
    } else {
        Info "下载 mediamtx（bluenviron/mediamtx 官方发布）"
        New-Item -ItemType Directory -Force -Path $mtxDir | Out-Null
        try {
            $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/bluenviron/mediamtx/releases/latest" -Headers @{ 'User-Agent' = 'guis-setup' }
            $asset = $rel.assets | Where-Object { $_.name -like "*windows_amd64.zip" } | Select-Object -First 1
            if (-not $asset) { throw "发布里没有 windows_amd64.zip" }
            $zip = Join-Path $env:TEMP $asset.name
            Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zip -UseBasicParsing
            Expand-Archive -Path $zip -DestinationPath $mtxDir -Force
            Info "mediamtx 已就绪：$mtxExe"
        } catch {
            Warn "mediamtx 下载失败（$($_.Exception.Message)）：可手动下载后放到 $mtxDir"
        }
    }
}

# ---- 5. 权重与素材提示 ----
Write-Host @"

=====================================================================
下一步（手动，一次性）：

1) 模型权重（放在 $Dir 对应子目录，具体路径以 LiveTalking 官方 README 为准）：
   - wav2lip：models/wav2lip.pth
   - musetalk / ernerf / ultralight：按官方 README 的下载步骤

2) 形象素材（自定义数字人）：
   - 说话视频：data/avatars/<avatar_id>.mp4（推荐 25fps、256x256 或 512x512）
   - 全身拼接图：data/avatars/full_body_img/*（可选）

3) 启动（本仓库脚本）：
   powershell -ExecutionPolicy Bypass -File scripts/livetalking/start.ps1 -Dir $Dir -EnvName $EnvName

4) 在 App 里：设置 → 更多 → 数字人，或点输入框工具条的 👤 按钮：
   - 接口地址 http://127.0.0.1:8010
   - 拉流地址 http://127.0.0.1:8888/live/livestream
   - 会话 ID 0
   - 引擎目录 $Dir（填了就能在 App 里一键启动/停止引擎）
=====================================================================
"@ -ForegroundColor Green

Info "完成"
