<#
.SYNOPSIS
把「组件库演示（cmd/gallery）」的各端二进制发布到 GitHub Releases。

.DESCRIPTION
流程：构建 -> 计算 tag -> 建/复用 release -> 逐个上传资产 -> 打印下载地址。

二进制不进仓库（见 bin/README.md 与 .gitignore 的说明），只作为 release 资产分发。

token 需要 repo scope（推送 workflow 文件另需 workflow scope）：

  $env:GITHUB_TOKEN = 'ghp_xxx'
  ./scripts/release-bin.ps1

标签规则：默认 "v<版本>-<短提交>"（如 v1.2.0-b150582），作为预发布；
加 -Stable 则去掉预发布标记。/ -Tag 可显式指定。

.EXAMPLE
./scripts/release-bin.ps1                     # 构建 + 预发布
./scripts/release-bin.ps1 -SkipBuild          # 复用 bin/ 里已有产物
./scripts/release-bin.ps1 -Tag v1.2.1 -Stable # 指定 tag，正式版
./scripts/release-bin.ps1 -Force              # 同名 release 已存在时删掉重建
#>
param(
    [string]$Tag = '',
    [switch]$Stable,
    [string]$Token = $env:GITHUB_TOKEN,
    [string]$Repo = '2252688138/Guis',
    [string]$OutDir = 'bin',
    [switch]$SkipBuild,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

if (-not $Token) {
    throw '缺少 token：请先设置 $env:GITHUB_TOKEN，或用 -Token 传入（需要 repo scope）。'
}

# ================= 版本与 tag =================
$versionFile = Join-Path (Get-Location) 'VERSION.txt'
$Version = '0.0.0'
if (Test-Path -LiteralPath $versionFile) {
    $Version = (Get-Content -LiteralPath $versionFile -TotalCount 1).Trim()
}
if (-not $Version) { $Version = '0.0.0' }

$sha = (& git rev-parse --short HEAD).Trim()
if (-not $sha) { throw '取不到当前提交（git rev-parse 失败）。' }

if (-not $Tag) {
    $Tag = if ($Stable) { "v$Version" } else { "v$Version-$sha" }
}

# ================= 构建 =================
if (-not $SkipBuild) {
    & (Join-Path $PSScriptRoot 'build-bin.ps1') -OutDir $OutDir
    if ($LASTEXITCODE -ne 0) { throw '构建失败，已中止发布。' }
}

$OutDirAbs = (Resolve-Path $OutDir).Path
$allowed = @('guis-gallery-windows-amd64.exe', 'guis-gallery-linux-amd64', 'guis-gallery-linux-arm64',
    'guis-gallery-darwin-arm64', 'guis-gallery-darwin-amd64', 'guis-gallery.wasm',
    'wasm_exec.js', 'SHA256SUMS.txt')
$assets = $allowed | ForEach-Object { Join-Path $OutDirAbs $_ } | Where-Object { Test-Path -LiteralPath $_ }
if (-not $assets) { throw ("{0} 里没有可发布的产物，先跑 build-bin.ps1。" -f $OutDir) }

$headers = @{
    'User-Agent'    = 'guis-release-bin'
    'Accept'        = 'application/vnd.github+json'
    'Authorization' = "Bearer $Token"
}
$apiBase = "https://api.github.com/repos/$Repo"

# ================= 建 / 复用 release =================
$existing = $null
try {
    $existing = Invoke-RestMethod -Uri "$apiBase/releases/tags/$Tag" -Headers $headers -ErrorAction Stop
} catch {
    $existing = $null
}

if ($existing -and $Force) {
    Write-Host ("删除已有 release {0}（-Force）..." -f $Tag) -ForegroundColor Yellow
    Invoke-RestMethod -Uri "$apiBase/releases/$($existing.id)" -Method Delete -Headers $headers | Out-Null
    $existing = $null
}

if ($existing) {
    $release = $existing
    Write-Host ("复用已有 release：{0}（id={1}）" -f $Tag, $release.id) -ForegroundColor Yellow
} else {
    Write-Host ("创建 release {0}..." -f $Tag) -ForegroundColor Cyan
    $note = @"
组件库演示程序 ``cmd/gallery`` 的各端预编译二进制。

- 源码提交：$sha
- 版本：$Version（VERSION.txt）
- 构建：``go build -trimpath -buildvcs=false -ldflags "-s -w -buildid="``，全部 CGO_ENABLED=0

| 端 | 文件 |
| --- | --- |
| Windows amd64 | guis-gallery-windows-amd64.exe |
| Linux amd64 | guis-gallery-linux-amd64 |
| Linux arm64 | guis-gallery-linux-arm64 |
| macOS arm64 | guis-gallery-darwin-arm64 |
| macOS amd64 | guis-gallery-darwin-amd64 |
| Web (wasm) | guis-gallery.wasm + wasm_exec.js |

校验：``sha256sum -c SHA256SUMS.txt``。
"@
    $payload = @{
        tag_name         = $Tag
        target_commitish = (& git rev-parse HEAD).Trim()
        name             = "组件库演示二进制 v$Version ($sha)"
        body             = $note
        prerelease       = (-not $Stable)
        draft            = $false
    } | ConvertTo-Json -Depth 3
    $release = Invoke-RestMethod -Uri "$apiBase/releases" -Method Post -Headers $headers `
        -Body ([Text.Encoding]::UTF8.GetBytes($payload)) -ContentType 'application/json'
    Write-Host ("  release id = {0}" -f $release.id) -ForegroundColor Green
}

# ================= 上传资产 =================
$existingNames = @{}
if ($release.assets) { $release.assets | ForEach-Object { $existingNames[$_.name] = $_.id } }

foreach ($path in $assets) {
    $name = Split-Path -Leaf $path
    if ($existingNames.ContainsKey($name)) {
        Write-Host ("  跳过已存在：{0}" -f $name) -ForegroundColor Yellow
        continue
    }
    $sizeMB = [math]::Round((Get-Item -LiteralPath $path).Length / 1MB, 2)
    Write-Host ("  上传 {0}（{1} MB）..." -f $name, $sizeMB) -ForegroundColor Cyan
    $upload = "https://uploads.github.com/repos/$Repo/releases/$($release.id)/assets?name=$name"
    $resp = curl.exe -sS -X POST -H "Authorization: Bearer $Token" `
        -H 'Content-Type: application/octet-stream' --data-binary "@$path" $upload 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) { throw ("上传失败：{0} -> {1}" -f $name, $resp) }
}

Write-Host ''
Write-Host ("完成：{0}" -f $release.html_url) -ForegroundColor Green
Write-Host ("下载目录：https://github.com/{0}/releases/tag/{1}" -f $Repo, $Tag) -ForegroundColor Green
