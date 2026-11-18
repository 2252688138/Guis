# 启动 LiveTalking + mediamtx（Windows）
#
# 用法（在仓库根目录执行）：
#   powershell -ExecutionPolicy Bypass -File scripts/livetalking/start.ps1 -Dir D:\LiveTalking -EnvName livetalking
#
# 做的事：
#   1. 先起 mediamtx（本地 RTMP:1935 + HTTP-FLV:8888）
#   2. 再起 LiveTalking：python app.py --transport=rtmp --push_url rtmp://127.0.0.1:1935/live/livestream
#   3. 打印 App 里需要填的接口/拉流地址
#   Ctrl+C 退出时一并结束子进程

param(
    [string]$Dir = "D:\LiveTalking",
    [string]$EnvName = "livetalking",
    [string]$Avatar = "",
    [string]$Model = "",
    [string]$ExtraArgs = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$mtxExe = Join-Path $root "tools\mediamtx\mediamtx.exe"
$pushUrl = "rtmp://127.0.0.1:1935/live/livestream"

if (-not (Test-Path (Join-Path $Dir "app.py"))) {
    Write-Host "[start] 未找到 $Dir\app.py，请先跑 setup.ps1" -ForegroundColor Red
    exit 1
}

$procs = @()
try {
    if (Test-Path $mtxExe) {
        Write-Host "[start] 启动 mediamtx：$mtxExe" -ForegroundColor Cyan
        $procs += Start-Process -FilePath $mtxExe -WorkingDirectory (Split-Path $mtxExe) -PassThru
        Start-Sleep -Milliseconds 800
    } else {
        Write-Host "[start] 未找到 mediamtx（$mtxExe），假定已有 RTMP 服务在 1935" -ForegroundColor Yellow
    }

    $args = @("app.py", "--transport=rtmp", "--push_url=$pushUrl")
    if ($Avatar -ne "") { $args += "--avatar_id=$Avatar" }
    if ($Model -ne "") { $args += "--model=$Model" }
    if ($ExtraArgs -ne "") { $args += ($ExtraArgs -split "\s+") }

    Write-Host "[start] 启动 LiveTalking：conda run -n $EnvName python $($args -join ' ')" -ForegroundColor Cyan
    Push-Location $Dir
    $env:PYTHONUNBUFFERED = "1"
    & conda run -n $EnvName --no-capture-output python @args
    Pop-Location
} finally {
    foreach ($p in $procs) {
        if ($p -and -not $p.HasExited) {
            Write-Host "[start] 结束进程 $($p.Id)" -ForegroundColor Cyan
            Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        }
    }
}

# 记录（供 App 里对参）
Write-Host @"

App 里填写：
  接口地址  http://127.0.0.1:8010        （LiveTalking HTTP API）
  拉流地址  http://127.0.0.1:8888/live/livestream  （mediamtx HTTP-FLV）
  会话 ID   0
"@ -ForegroundColor Green
