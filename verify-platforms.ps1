# guis — 跨平台校验脚本
#
# 目的：本机是 Windows，Linux / macOS 只能交叉编译、不能实机跑；这个脚本把
# 「三端都能编译 + 单测通过 + 无遗留调试代码」固化成一条命令，避免改完平台层
# 只验证了 Windows。
#
# 用法：
#   .\verify-platforms.ps1              # 全量（三端编译 + 单测 + wasm）
#   .\verify-platforms.ps1 -SkipWasm    # 跳过 wasm（省 10 秒）
#   .\verify-platforms.ps1 -Keep        # 保留 dist\verify 产物
param(
    [switch]$SkipWasm,
    [switch]$Keep
)

$ErrorActionPreference = 'Stop'
$OutputEncoding = [Console]::OutputEncoding

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$outDir = Join-Path $root 'dist\verify'
if (Test-Path $outDir) { Remove-Item $outDir -Recurse -Force }
New-Item -ItemType Directory -Path $outDir -Force | Out-Null

$failed = @()
function Step($name, $block) {
    Write-Host ("==> {0}" -f $name)
    try {
        & $block
        if ($LASTEXITCODE -ne 0) { throw "exit $LASTEXITCODE" }
        Write-Host "    OK" -ForegroundColor Green
    } catch {
        Write-Host ("    FAIL: {0}" -f $_) -ForegroundColor Red
        $script:failed += $name
    }
}

# 1) 三端编译（windows / linux / darwin）
$targets = @(
    @{ os = 'windows'; arch = 'amd64'; ext = '.exe' },
    @{ os = 'linux';   arch = 'amd64'; ext = '' },
    @{ os = 'darwin';  arch = 'amd64'; ext = '' },
    @{ os = 'darwin';  arch = 'arm64'; ext = '' }
)
foreach ($t in $targets) {
    $label = "build $($t.os)/$($t.arch)"
    Step $label {
        $env:GOOS = $t.os
        $env:GOARCH = $t.arch
        $env:CGO_ENABLED = '0'
        go build -o (Join-Path $outDir ("gallery-" + $t.os + "-" + $t.arch + $t.ext)) ./cmd/gallery
        $env:GOOS = ''
        $env:GOARCH = ''
        $env:CGO_ENABLED = ''
    }
}

# 2) Web（wasm）
if (-not $SkipWasm) {
    Step 'build js/wasm' {
        $env:GOOS = 'js'
        $env:GOARCH = 'wasm'
        go build -o (Join-Path $outDir 'gallery.wasm') ./cmd/gallery
        $env:GOOS = ''
        $env:GOARCH = ''
    }
}

# 3) 单测（含平台无关的纯逻辑：URI 列表解析、修饰键映射、X11 消息布局等）
Step 'go vet ./ui ./internal/x11 ./internal/office ./internal/version ./cmd/gallery' {
    # 注：internal/gpu、internal/text 有历史遗留的 unkeyed struct / unsafe.Pointer 告警，
    # 与平台层无关，故不在本脚本的 vet 范围内。
    go vet ./ui ./internal/x11 ./internal/office ./internal/version ./cmd/gallery
}
Step 'go test ./ui ./internal/x11 ./internal/office ./internal/version ./cmd/gallery/...' {
    go test ./ui ./internal/x11 ./internal/office ./internal/version ./cmd/gallery/...
}

# 4) 遗留调试代码检查（fmt.Print* / println 出现在非测试的平台实现里要人工确认）
Step 'no stray println in ui platform files' {
    $hits = Select-String -Path (Join-Path $root 'ui\*.go') -Pattern '^\s*println\(' -ErrorAction SilentlyContinue
    if ($hits) {
        $hits | ForEach-Object { Write-Host ("    {0}:{1}" -f $_.Filename, $_.LineNumber) -ForegroundColor Yellow }
        throw "发现调试用 println"
    }
}

if (-not $Keep) { Remove-Item $outDir -Recurse -Force -ErrorAction SilentlyContinue }

Write-Host ''
if ($failed.Count -gt 0) {
    Write-Host ("FAILED: {0}" -f ($failed -join ', ')) -ForegroundColor Red
    exit 1
}
Write-Host 'ALL GREEN' -ForegroundColor Green
