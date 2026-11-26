<#
.SYNOPSIS
构建「组件库演示」cmd/gallery 的各端产物，输出到 bin/（由 scripts/release-bin.ps1 发布到 Releases）。

.DESCRIPTION
产出（桌面端全部 CGO_ENABLED=0 + -s -w -trimpath，版本由 VERSION.txt 注入）：

  guis-gallery-windows-amd64.exe
  guis-gallery-linux-amd64
  guis-gallery-linux-arm64
  guis-gallery-macos-arm64.app.zip   ← .app bundle（含 Unix 权限位，解压双击即用）
  guis-gallery-macos-amd64.app.zip   ← 同上
  guis-gallery.wasm                 （GOOS=js GOARCH=wasm，配 wasm_exec.js 用）
  wasm_exec.js                      （从 GOROOT 复制）
  README.md / SHA256SUMS.txt        （版本、大小、校验和）

macOS 不是裸二进制而是 .app bundle：裸二进制在 Mac 上 Dock 名称/图标不完整，
且 Gatekeeper 会直接拒绝。这部分逻辑复用仓库根的 build-macos.ps1。

与其它脚本的分工：

  build.ps1        Windows 发布（图标资源 + NSIS 安装器 + UPX）
  build-multi.ps1  多端编译矩阵，产物到 dist/（本地验证用，不入库）
  build-macos.ps1  macOS 的 .app bundle（可分发，含 icns 合成与权限保留打包）
  build-bin.ps1    本脚本：汇总各端产物到 bin/
  release-bin.ps1  把 bin/ 的产物发布到 GitHub Releases

.EXAMPLE
./scripts/build-bin.ps1                 # 全平台
./scripts/build-bin.ps1 -Only darwin    # 只出两个 macOS .app.zip
./scripts/build-bin.ps1 -SkipReadme     # 不重写 README.md
#>
param(
    [ValidateSet('all', 'windows', 'linux', 'darwin', 'wasm')]
    [string]$Only = 'all',
    [string]$OutDir = 'bin',
    [switch]$SkipReadme
)

$ErrorActionPreference = 'Stop'
# 脚本在 scripts/ 下，切到仓库根目录再构建
Set-Location (Split-Path -Parent $PSScriptRoot)

# ================= 版本信息 =================
$versionFile = Join-Path (Get-Location) 'VERSION.txt'
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
    '-X guis/internal/version.Channel=release'
) -join ' '

if (-not (Test-Path -LiteralPath $OutDir)) {
    New-Item -ItemType Directory -Path $OutDir | Out-Null
}
$OutDirAbs = (Resolve-Path $OutDir).Path

# ================= 目标矩阵 =================
# Kind: bin = 直接 go build；app = 走 build-macos.ps1 出 .app 并打成 zip
$matrix = @(
    @{ Group = 'windows'; Name = 'windows-amd64'; Kind = 'bin'; GOOS = 'windows'; GOARCH = 'amd64'; Out = 'guis-gallery-windows-amd64.exe' },
    @{ Group = 'linux';   Name = 'linux-amd64';   Kind = 'bin'; GOOS = 'linux';   GOARCH = 'amd64'; Out = 'guis-gallery-linux-amd64' },
    @{ Group = 'linux';   Name = 'linux-arm64';   Kind = 'bin'; GOOS = 'linux';   GOARCH = 'arm64'; Out = 'guis-gallery-linux-arm64' },
    @{ Group = 'darwin';  Name = 'macos-arm64';   Kind = 'app'; GOOS = 'darwin';  GOARCH = 'arm64'; Out = 'guis-gallery-macos-arm64.app.zip' },
    @{ Group = 'darwin';  Name = 'macos-amd64';   Kind = 'app'; GOOS = 'darwin';  GOARCH = 'amd64'; Out = 'guis-gallery-macos-amd64.app.zip' },
    @{ Group = 'wasm';    Name = 'wasm';          Kind = 'bin'; GOOS = 'js';      GOARCH = 'wasm';  Out = 'guis-gallery.wasm' }
)

$targets = $matrix | Where-Object { $Only -eq 'all' -or $_.Group -eq $Only }
if (-not $targets) { throw ("没有匹配的目标：{0}" -f $Only) }

# ================= 构建 =================
$results = @()
foreach ($t in $targets) {
    $out = Join-Path $OutDirAbs $t.Out

    if ($t.Kind -eq 'app') {
        # macOS：交给 build-macos.ps1（Info.plist / icns / 权限保留 zip 都在那边）
        Write-Host ("构建 {0}（darwin/{1}，.app bundle，v{2}）..." -f $t.Name, $t.GOARCH, $Version) -ForegroundColor Cyan
        & (Join-Path (Get-Location) 'build-macos.ps1') -Arch $t.GOARCH -OutDir $OutDir -Zip -RemoveAppAfterZip
        if ($LASTEXITCODE -ne 0) { throw ("构建失败：{0}" -f $t.Name) }

        $produced = Join-Path $OutDirAbs ("GUIS-{0}.app.zip" -f $t.GOARCH)
        if (-not (Test-Path -LiteralPath $produced)) { throw ("没有找到预期产物：{0}" -f $produced) }
        if ($produced -ne $out) {
            if (Test-Path -LiteralPath $out) { Remove-Item -LiteralPath $out -Force }
            Move-Item -LiteralPath $produced -Destination $out -Force
        }
    } else {
        Write-Host ("构建 {0}（{1}/{2}，v{3}）..." -f $t.Name, $t.GOOS, $t.GOARCH, $Version) -ForegroundColor Cyan
        $env:GOOS = $t.GOOS
        $env:GOARCH = $t.GOARCH
        $env:CGO_ENABLED = '0'
        & go build -trimpath -buildvcs=false -ldflags $ldflags -o $out ./cmd/gallery
        if ($LASTEXITCODE -ne 0) { throw ("构建失败：{0}（exit {1}）" -f $t.Name, $LASTEXITCODE) }
    }

    $item = Get-Item -LiteralPath $out
    $sha = (Get-FileHash -LiteralPath $out -Algorithm SHA256).Hash.ToLower()
    $results += [pscustomobject]@{
        Name   = $t.Name
        GOOS   = $t.GOOS
        GOARCH = $t.GOARCH
        File   = $t.Out
        Bytes  = $item.Length
        MB     = [math]::Round($item.Length / 1MB, 2)
        SHA256 = $sha
    }
    Write-Host ("  -> {0}（{1} MB）" -f $t.Out, [math]::Round($item.Length / 1MB, 2)) -ForegroundColor Green
}

# wasm 的宿主胶水：从 GOROOT 复制，方便直接跑
if ($Only -eq 'all' -or $Only -eq 'wasm') {
    $goroot = (& go env GOROOT).Trim()
    $candidates = @(
        (Join-Path $goroot 'lib\wasm\wasm_exec.js'),
        (Join-Path $goroot 'lib\wasm\go_js_wasm_exec'),
        (Join-Path $goroot 'misc\wasm\wasm_exec.js')
    )
    $src = $candidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    if ($src) {
        $dst = Join-Path $OutDirAbs 'wasm_exec.js'
        Copy-Item -LiteralPath $src -Destination $dst -Force
        Write-Host ("  -> wasm_exec.js（from {0}）" -f $src) -ForegroundColor Green
    } else {
        Write-Host '  未找到 wasm_exec.js（Go 版本可能把胶水移到了别处），跳过。' -ForegroundColor Yellow
    }
}

# ================= 校验和 + README =================
$sumPath = Join-Path $OutDirAbs 'SHA256SUMS.txt'
$sumLines = $results | ForEach-Object { "{0}  {1}" -f $_.SHA256, $_.File }
$sumLines | Set-Content -LiteralPath $sumPath -Encoding ascii

if (-not $SkipReadme) {
    $rows = ($results | ForEach-Object {
            "| {0} | ``{1}`` | {2} MB | ``{3}`` |" -f $_.Name, $_.File, $_.MB, $_.SHA256.Substring(0, 16)
        }) -join "`n"

    $readme = @"
# bin/ —— 组件库演示（cmd/gallery）各端产物

由 ``scripts/build-bin.ps1`` 生成，对应版本 **v$Version**（commit ``$Commit``，构建于 $BuildTime UTC）。

gallery 是 guis 唯一的演示入口：``cmd/gallery/main.go`` 一份代码，桌面 / 浏览器 / 安卓三端共用，
内容覆盖 ui 包里全部组件（布局、图表、表格、树、3D 视口、虚拟摇杆、WebView、终端等）。

## 产物

| 端 | 文件 | 体积 | SHA256（前 16 位） |
| --- | --- | --- | --- |
$rows

完整校验和见 ``SHA256SUMS.txt``：

``````powershell
# Windows
Get-FileHash .\guis-gallery-windows-amd64.exe -Algorithm SHA256
# Linux / macOS
sha256sum -c SHA256SUMS.txt
``````

## macOS

macOS 分发包是 **``.app`` bundle 的 zip**（``guis-gallery-macos-*.app.zip``），不是裸二进制：

``````bash
unzip guis-gallery-macos-arm64.app.zip    # 得到 GUIS-arm64.app
open GUIS-arm64.app
xattr -dr com.apple.quarantine GUIS-arm64.app   # 首次被 Gatekeeper 拦时放行
``````

zip 里已经写好 Unix 权限位（``Contents/MacOS/GUIS`` 是 0755），解压即可双击运行。
未签名/未公证，正式分发需在 Mac 上 ``codesign`` + ``notarytool``。

## Web（wasm）

``````bash
# 需要本地静态服务器（wasm 不能 file:// 打开）
python -m http.server 8089
# 页面里用 wasm_exec.js 载入 guis-gallery.wasm
``````

``wasm_exec.js`` 从本机 ``$GOROOT`` 复制，需与编译用的 Go 版本一致（本仓库 go.mod 指定 Go 1.26）。

## 备注

- 全部 CGO_ENABLED=0 静态编译，无需运行库；macOS / Linux 的窗口后端通过 purego 动态加载系统框架/so。
- 重新生成：``./scripts/build-bin.ps1``（或 ``-Only darwin`` 只出 macOS）。
- 发布：``./scripts/release-bin.ps1``（需要 ``$env:GITHUB_TOKEN``）。
"@
    $readmePath = Join-Path $OutDirAbs 'README.md'
    [System.IO.File]::WriteAllText($readmePath, $readme, (New-Object System.Text.UTF8Encoding($false)))
    Write-Host ("  -> README.md / SHA256SUMS.txt") -ForegroundColor Green
}

Write-Host ''
$totalMB = [math]::Round(($results | Measure-Object -Property Bytes -Sum).Sum / 1MB, 1)
Write-Host ("完成：{0} 个产物，共 {1} MB -> {2}" -f $results.Count, $totalMB, $OutDirAbs) -ForegroundColor Green
