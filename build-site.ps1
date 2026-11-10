# guis — 官网构建：把 cmd/guis-site 编译成 WebAssembly 并组装静态目录
#
#   ./build-site.ps1                # 构建到 dist/site
#   ./build-site.ps1 -Serve         # 构建后起本地服务器并打开浏览器
#   ./build-site.ps1 -Serve -Port 8090
param(
    [switch]$Serve,
    [int]$Port = 8099,
    [string]$OutDir = 'dist\site'
)
$ErrorActionPreference = 'Stop'
$OutputEncoding = [Console]::OutputEncoding
$root = $PSScriptRoot
Set-Location $root

$out = Join-Path $root $OutDir
New-Item -ItemType Directory -Force -Path $out | Out-Null

Write-Host '[site 1/5] go build ./cmd/guis-site -> main.wasm' -ForegroundColor Cyan
$env:CGO_ENABLED = '0'
$env:GOOS = 'js'
$env:GOARCH = 'wasm'
& go build -trimpath -buildvcs=false -ldflags '-s -w' -o (Join-Path $out 'main.wasm') ./cmd/guis-site
if ($LASTEXITCODE -ne 0) { throw 'go build 失败：./cmd/guis-site' }

Write-Host '[site 2/5] 复制 wasm_exec.js' -ForegroundColor Cyan
$goroot = (& go env GOROOT).Trim()
$exec = Join-Path $goroot 'lib\wasm\wasm_exec.js'
if (-not (Test-Path $exec)) { $exec = Join-Path $goroot 'misc\wasm\wasm_exec.js' }
Copy-Item $exec (Join-Path $out 'wasm_exec.js') -Force

Write-Host '[site 3/5] 复制 index.html' -ForegroundColor Cyan
Copy-Item (Join-Path $root 'web\site.html') (Join-Path $out 'index.html') -Force

Write-Host '[site 4/5] 字体：首屏子集 + 全站字体 + 全站粗体' -ForegroundColor Cyan
& (Join-Path $root '.zym\site_fonts.ps1') -OutDir $out
if ($LASTEXITCODE -ne 0) { throw '字体生成失败（先执行：python -m pip install fonttools）' }

Write-Host '[site 5/5] 输出清单' -ForegroundColor Cyan
Get-ChildItem $out | Where-Object { -not $_.PSIsContainer } |
    Sort-Object Length -Descending |
    ForEach-Object { Write-Host ('  {0,-16} {1,8:N2} MB' -f $_.Name, ($_.Length / 1MB)) }

Write-Host ''
Write-Host ('预览地址 http://127.0.0.1:{0}/' -f $Port) -ForegroundColor Yellow

if ($Serve) {
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    & go run ./web/server -dir $out -addr ('127.0.0.1:{0}' -f $Port) -open
}
