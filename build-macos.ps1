<#
.SYNOPSIS
GUIS macOS 打包脚本：交叉编译 darwin 二进制并组装成 .app bundle（拷到 Mac 上可直接双击）。

.DESCRIPTION
macOS 的 GUI 程序需要 .app bundle，只拷裸二进制会出现 Dock 名称 / 图标异常。
本脚本在 Windows / Linux 上完成全部交叉编译与打包（不需要 Mac）：

  Contents/Info.plist           版本号、bundle id、最低系统版本
  Contents/MacOS/<AppName>      darwin 可执行文件（CGO_ENABLED=0 + purego 动态加载 AppKit）
  Contents/Resources/icon.icns  由 ui/assets/icon_*.png 合成的 icns（PNG-based）
  Contents/PkgInfo              APPL????

版本号唯一来源是仓库根目录 VERSION.txt，与 build.ps1 / build-multi.ps1 一致，
通过 -ldflags -X 注入 guis/internal/version.*。

与 build-multi.ps1 的分工：
  build-multi.ps1 -Only darwin   只产出裸二进制（验证能否编译）
  build-macos.ps1                产出可分发的 .app bundle（可加 -Zip）

.EXAMPLE
./build-macos.ps1                      # dist\GUIS-arm64.app + dist\GUIS-amd64.app
./build-macos.ps1 -Arch arm64 -Zip     # 只出 arm64，并打成 zip
./build-macos.ps1 -AppName Guis -SkipIcon
#>
param(
    [ValidateSet('all', 'arm64', 'amd64')]
    [string]$Arch = 'all',
    [string]$OutDir = 'dist',
    [string]$Pkg = './cmd/gallery',
    [string]$AppName = 'GUIS',
    [string]$BundleId = 'com.cq6k.guis',
    [ValidateSet('dev', 'release')]
    [string]$Channel = 'release',
    [switch]$Zip,
    [switch]$SkipIcon
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $MyInvocation.MyCommand.Path)

# ================= 版本信息 =================
$versionFile = Join-Path $PSScriptRoot 'VERSION.txt'
$Version = '0.0.0'
if (Test-Path -LiteralPath $versionFile) {
    $Version = (Get-Content -LiteralPath $versionFile -TotalCount 1).Trim()
}
if (-not $Version) { $Version = '0.0.0' }

$Commit = 'unknown'
try {
    $c = & git rev-parse --short HEAD 2>$null
    if ($LASTEXITCODE -eq 0 -and $c) { $Commit = $c.Trim() }
} catch { }
$BuildTime = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')

$ldflags = @(
    '-s', '-w', '-buildid=',
    "-X guis/internal/version.Number=$Version",
    "-X guis/internal/version.Commit=$Commit",
    "-X guis/internal/version.BuildTime=$BuildTime",
    "-X guis/internal/version.Channel=$Channel"
) -join ' '

if (-not (Test-Path -LiteralPath $OutDir)) {
    New-Item -ItemType Directory -Path $OutDir | Out-Null
}
$OutDirAbs = (Resolve-Path $OutDir).Path

# ================= icns 合成 =================
# 现代 macOS 支持「PNG-based icns」：icns 头 + 若干 (4 字节类型 + 大端长度 + PNG 原始字节)。
# 类型码：icp4=16, icp5=32, ic07=128, ic08=256, ic12=64。
function New-Icns {
    param([string]$Path, [hashtable]$Map)

    $body = New-Object System.IO.MemoryStream
    foreach ($type in $Map.Keys) {
        $png = [System.IO.File]::ReadAllBytes($Map[$type])
        $typeBytes = [System.Text.Encoding]::ASCII.GetBytes($type)
        $lenBytes = [BitConverter]::GetBytes([uint32]($png.Length + 8))
        [Array]::Reverse($lenBytes)
        $body.Write($typeBytes, 0, 4)
        $body.Write($lenBytes, 0, 4)
        $body.Write($png, 0, $png.Length)
    }
    $data = $body.ToArray()
    $total = [BitConverter]::GetBytes([uint32]($data.Length + 8))
    [Array]::Reverse($total)

    $fs = [System.IO.File]::Create($Path)
    try {
        $fs.Write([System.Text.Encoding]::ASCII.GetBytes('icns'), 0, 4)
        $fs.Write($total, 0, 4)
        $fs.Write($data, 0, $data.Length)
    } finally {
        $fs.Close()
    }
}

$assets = Join-Path (Join-Path $PSScriptRoot 'ui') 'assets'
$icnsMap = @{}
foreach ($pair in @(
        @('icp4', 'icon_16x16.png'),
        @('icp5', 'icon_32x32.png'),
        @('ic12', 'icon_64x64.png'),
        @('ic07', 'icon_128x128.png'),
        @('ic08', 'icon_256x256.png'))) {
    $p = Join-Path $assets $pair[1]
    if (Test-Path -LiteralPath $p) { $icnsMap[$pair[0]] = $p }
}

# ================= 构建 + 组装 bundle =================
$targets = switch ($Arch) {
    'all' { @('arm64', 'amd64') }
    default { @($Arch) }
}

$bundles = @()
foreach ($a in $targets) {
    $bundleName = "$AppName-$a"
    $appDir = Join-Path $OutDirAbs "$bundleName.app"
    # 用 Join-Path 逐级拼，不用 'Contents\MacOS' 这类反斜杠字符串：
    # 同一个脚本要能在 Linux CI（pwsh）里跑，反斜杠在那里不是路径分隔符。
    $contentsDir = Join-Path $appDir 'Contents'
    $macosDir = Join-Path $contentsDir 'MacOS'
    $resDir = Join-Path $contentsDir 'Resources'

    if (Test-Path -LiteralPath $appDir) { Remove-Item -LiteralPath $appDir -Recurse -Force }
    New-Item -ItemType Directory -Path $macosDir -Force | Out-Null
    New-Item -ItemType Directory -Path $resDir -Force | Out-Null

    $exe = Join-Path $macosDir "$AppName"
    Write-Host ("构建 {0}（darwin/{1}，v{2}）..." -f $bundleName, $a, $Version) -ForegroundColor Cyan
    $env:GOOS = 'darwin'
    $env:GOARCH = $a
    $env:CGO_ENABLED = '0'
    & go build -trimpath -buildvcs=false -ldflags $ldflags -o $exe $Pkg
    if ($LASTEXITCODE -ne 0) { throw ("构建失败：darwin/{0}（exit {1}）" -f $a, $LASTEXITCODE) }

    # Info.plist
    $plist = @"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>$AppName</string>
	<key>CFBundleDisplayName</key><string>$AppName</string>
	<key>CFBundleExecutable</key><string>$AppName</string>
	<key>CFBundleIdentifier</key><string>$BundleId</string>
	<key>CFBundleVersion</key><string>$Version</string>
	<key>CFBundleShortVersionString</key><string>$Version</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleSignature</key><string>????</string>
	<key>CFBundleIconFile</key><string>icon.icns</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
"@
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText((Join-Path $contentsDir 'Info.plist'), $plist, $utf8)
    [System.IO.File]::WriteAllText((Join-Path $contentsDir 'PkgInfo'), 'APPL????', $utf8)

    if (-not $SkipIcon -and $icnsMap.Count -gt 0) {
        New-Icns -Path (Join-Path $resDir 'icon.icns') -Map $icnsMap
    }

    $sizeMB = [math]::Round((Get-Item $exe).Length / 1MB, 1)
    Write-Host ("  -> {0}.app（{1} MB）" -f $bundleName, $sizeMB) -ForegroundColor Green
    $bundles += $appDir
}

if ($Zip) {
    foreach ($d in $bundles) {
        $zipPath = "$d.zip"
        if (Test-Path -LiteralPath $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
        Compress-Archive -Path $d -DestinationPath $zipPath -CompressionLevel Optimal
        Write-Host ("  zip -> {0}" -f (Split-Path -Leaf $zipPath)) -ForegroundColor Green
    }
}

Write-Host ''
Write-Host ("完成：{0} 个 bundle -> {1}" -f $bundles.Count, $OutDirAbs) -ForegroundColor Green
Write-Host '首次运行会被 Gatekeeper 拦截：xattr -dr com.apple.quarantine <bundle>.app 放行；正式分发需 codesign + notarytool 签名公证。' -ForegroundColor Yellow
