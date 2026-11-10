<#
.SYNOPSIS
DeepSeek Harness（cmd/deepseek）构建脚本：图标资源 + 版本注入 + 片头素材 + 体积优化 + NSIS 安装器。

.DESCRIPTION
与另外两个脚本的分工：
  build.ps1          GUIS gallery 演示程序的 Windows 发布（资源 + 安装器 + UPX）
  build-multi.ps1    gallery 的多端编译矩阵（Windows / Linux / macOS / Web）
  build-deepseek.ps1 cmd/deepseek 桌面 Agent 的 Windows 发布（本脚本）

版本号与 build.ps1 / build-multi.ps1 共用仓库根目录的 VERSION.txt（唯一来源），做三件事：
  1. 生成 cmd/deepseek/rsrc_windows_<arch>.syso（图标 + 文件属性版本，见 tools/winres）
  2. 通过 -ldflags -X 注入 guis/internal/version.*（运行时 version.String() / version.Full()）
  3. 传给 NSIS 作为安装器版本与「应用和功能」里的显示版本

片头素材：pages/intro_assets.go 在 exe 同级的 assets/intro 下找内置素材，
因此构建后会把仓库 assets/intro 复制到 dist\assets\intro，安装器打包时一并装入
（guis.nsi 的 ASSETS_DIR 可选块），否则发布版就没有内置片头。

发布构建参数（与 build.ps1 一致）：
  -trimpath -buildvcs=false                去掉本机绝对路径与 VCS 信息
  -ldflags "-s -w -H windowsgui -buildid=" 去符号/调试信息，GUI 子系统不弹控制台
  CGO_ENABLED=0                            纯 syscall，无 cgo 依赖
  GOAMD64=v1                               保守指令集，兼容旧 CPU

.EXAMPLE
./build-deepseek.ps1                        # 发布构建 -> dist\deepseek.exe（含 assets\intro）
./build-deepseek.ps1 -Dev                   # 开发构建：保留符号、带控制台（stderr 日志可见）
./build-deepseek.ps1 -Installer             # 构建 + 打安装器 -> dist\DeepSeekHarness-0.1.0-setup.exe
./build-deepseek.ps1 -Installer -Upx        # 再叠加 UPX -9（体积最小）
./build-deepseek.ps1 -Bump patch -Installer # 版本号 +1 后打包（0.1.0 -> 0.1.1）
./build-deepseek.ps1 -Test -Run             # 先跑 go test ./cmd/deepseek/...，成功后启动程序
./build-deepseek.ps1 -NoRes -NoAssets       # 跳过 syso 与素材复制（快速迭代）
#>
param(
    [switch]$Dev,
    [switch]$Upx,
    [switch]$Installer,
    [string]$OutDir = 'dist',
    # ---- 版本号管理（VERSION.txt 与其他构建脚本共用）----
    [ValidateSet('none', 'patch', 'minor', 'major')]
    [string]$Bump = 'none',
    [string]$VersionOverride = '',
    [ValidateSet('dev', 'beta', 'release')]
    [string]$Channel = 'dev',
    # ---- Windows 资源 / 素材 / 安装器 ----
    [switch]$NoRes,
    [switch]$NoAssets,
    [switch]$Test,
    [switch]$Run,
    [string]$ProductName = 'DeepSeek Harness',
    [string]$Company = 'guis',
    [string]$Description = 'DeepSeek Harness · Go 版桌面 Agent',
    [string]$Copyright = 'Copyright (c) 2026 Mr.Shu',
    [string]$AppExe = 'deepseek.exe',
    [string]$IconFile = 'ui\assets\icon.ico',
    [string]$IntroDir = 'assets\intro',
    [string]$PetDir = 'assets\pet',
    [string]$SkinDir = 'assets\skins',
    # 编译架构（Go 的 syso 按 _<arch> 后缀自动匹配）
    [ValidateSet('amd64', '386', 'arm64')]
    [string]$Arch = 'amd64'
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $MyInvocation.MyCommand.Path)

$Pkg = './cmd/deepseek'
$PkgDir = Join-Path $PSScriptRoot 'cmd\deepseek'

Write-Host ''
Write-Host '======== DeepSeek Harness build ========' -ForegroundColor Magenta

# ================= 版本号：VERSION.txt 是唯一来源 =================
$versionFile = Join-Path $PSScriptRoot 'VERSION.txt'

function Get-ProjectVersion {
    param([string]$File)
    if (-not (Test-Path -LiteralPath $File)) { throw "找不到版本文件：$File" }
    $raw = [IO.File]::ReadAllText($File).Trim().TrimStart([char]0xFEFF).Trim()
    if ($raw -notmatch '^v?(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.\-]+)?$') {
        throw "VERSION.txt 内容不合法：'$raw'（应为 x.y.z，如 1.2.3）"
    }
    return $raw.TrimStart('v', 'V')
}

$Version = if ($VersionOverride) { $VersionOverride.Trim().TrimStart('v', 'V') } else { Get-ProjectVersion $versionFile }

if ($Bump -ne 'none') {
    if ($Version -notmatch '^(\d+)\.(\d+)\.(\d+)') { throw "无法递增版本号：'$Version'" }
    $maj = [int]$Matches[1]; $min = [int]$Matches[2]; $pat = [int]$Matches[3]
    switch ($Bump) {
        'major' { $maj++; $min = 0; $pat = 0 }
        'minor' { $min++; $pat = 0 }
        'patch' { $pat++ }
    }
    $Version = "$maj.$min.$pat"
    [IO.File]::WriteAllText($versionFile, "$Version`n")
    Write-Host "[version] 版本号递增 -> $Version（已写回 VERSION.txt）" -ForegroundColor Yellow
}

# 4 段版本号（安装器 VIProductVersion 要求 x.x.x.x）
$viVersion = if ($Version -match '^(\d+)\.(\d+)\.(\d+)') { "$($Matches[1]).$($Matches[2]).$($Matches[3]).0" } else { '0.0.0.0' }
$ProductSlug = ($ProductName -replace '\s+', '')

# 构建元信息
$Commit = 'none'
try {
    $gitCommit = (& git rev-parse --short HEAD 2>$null)
    if ($LASTEXITCODE -eq 0 -and $gitCommit) { $Commit = $gitCommit.Trim() }
} catch { $Commit = 'none' }
$BuildTime = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')

Write-Host ("[version] {0} {1} · {2} · {3} · commit {4} · {5}/{6}" -f $ProductName, $Version, $Channel, $BuildTime, $Commit, 'windows', $Arch) -ForegroundColor Green

$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = $Arch
if ($Arch -eq 'amd64' -and -not $env:GOAMD64) { $env:GOAMD64 = 'v1' }

# ================= Windows 资源（图标 + 文件属性版本）=================
$iconPath = Join-Path $PSScriptRoot $IconFile
$needRes = (-not $NoRes) -and (-not $Dev)
$sysoPath = Join-Path $PkgDir ("rsrc_windows_{0}.syso" -f $Arch)

if ($needRes -and -not (Test-Path -LiteralPath $iconPath)) {
    Write-Warning "图标不存在，跳过资源生成：$iconPath"
    $needRes = $false
}

if ($needRes) {
    Write-Host ("[res] {0}  (v{1}, {2})" -f (Split-Path -Leaf $sysoPath), $Version, $Channel) -ForegroundColor Cyan
    $winresArgs = @(
        'run', './tools/winres',
        '-arch', $Arch,
        '-ico', $iconPath,
        '-out', $sysoPath,
        '-version', $Version,
        '-product', $ProductName,
        '-company', $Company,
        '-desc', $Description,
        '-copyright', $Copyright,
        '-commit', $Commit,
        '-channel', $Channel,
        '-q'
    )
    & go @winresArgs
    if ($LASTEXITCODE -ne 0) { throw "生成 Windows 资源失败：$Arch" }
}

# ================= 测试（可选）=================
if ($Test) {
    Write-Host '[test] go test ./cmd/deepseek/...' -ForegroundColor Cyan
    & go test ./cmd/deepseek/...
    if ($LASTEXITCODE -ne 0) { throw 'go test 失败：./cmd/deepseek/...' }
}

# ================= 构建 =================
# 运行时版本信息（guis/internal/version.*）通过链接期变量注入
# 运行时版本信息（guis/internal/version.*）通过链接期变量注入。
# 注意：产品名 / 描述含空格，每个 -X 定义必须用单引号包住，
# 否则 go 的 -ldflags 会被拆成多个参数（linker 直接报 usage）。
$versionLd = (@(
    "guis/internal/version.Number=$Version",
    "guis/internal/version.Commit=$Commit",
    "guis/internal/version.BuildTime=$BuildTime",
    "guis/internal/version.Channel=$Channel",
    "guis/internal/version.Product=$ProductName",
    "guis/internal/version.Company=$Company",
    "guis/internal/version.Description=$Description",
    "guis/internal/version.Copyright=$Copyright"
) | ForEach-Object { "-X '$_'" }) -join ' '

if ($Dev) {
    $ldArgs = "-H windowsgui $versionLd"
    $extra = @('-buildvcs=false')
} else {
    $ldArgs = "-s -w -H windowsgui -buildid= $versionLd"
    $extra = @('-trimpath', '-buildvcs=false')
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$OutDirAbs = (Resolve-Path $OutDir).Path
$exePath = Join-Path $OutDirAbs $AppExe

Write-Host ("[build] {0} <- {1}" -f $exePath, $Pkg) -ForegroundColor Cyan
& go build @extra -ldflags $ldArgs -o $exePath $Pkg
if ($LASTEXITCODE -ne 0) { throw "go build 失败：$Pkg" }

$results = @(Get-Item -LiteralPath $exePath)

# ================= 片头素材（exe 同级 assets\intro）=================
$introSrc = Join-Path $PSScriptRoot $IntroDir
$assetDest = Join-Path $OutDirAbs 'assets\intro'
if (-not $NoAssets) {
    if (Test-Path -LiteralPath $introSrc) {
        Write-Host ("[assets] {0} -> {1}" -f $introSrc, $assetDest) -ForegroundColor Cyan
        New-Item -ItemType Directory -Force -Path $assetDest | Out-Null
        Copy-Item (Join-Path $introSrc '*') $assetDest -Recurse -Force
    } else {
        Write-Warning "片头素材目录不存在，跳过：$introSrc"
    }
}

# ================= 桌宠素材（exe 同级 assets\pet）=================
$petSrc = Join-Path $PSScriptRoot $PetDir
$petDest = Join-Path $OutDirAbs 'assets\pet'
if (-not $NoAssets) {
    if (Test-Path -LiteralPath $petSrc) {
        Write-Host ("[pet] {0} -> {1}" -f $petSrc, $petDest) -ForegroundColor Cyan
        New-Item -ItemType Directory -Force -Path $petDest | Out-Null
        Copy-Item (Join-Path $petSrc '*') $petDest -Recurse -Force
    } else {
        Write-Warning "桌宠素材目录不存在，跳过：$petSrc"
    }
}

# ================= 皮肤素材（exe 同级 assets\skins）=================
# pages/skin.go 在 exe 同级 / 各上级目录的 assets\skins 下找预设主题（hero 图 + theme.json）。
$skinSrc = Join-Path $PSScriptRoot $SkinDir
$skinDest = Join-Path $OutDirAbs 'assets\skins'
if (-not $NoAssets) {
    if (Test-Path -LiteralPath $skinSrc) {
        Write-Host ("[skin] {0} -> {1}" -f $skinSrc, $skinDest) -ForegroundColor Cyan
        New-Item -ItemType Directory -Force -Path $skinDest | Out-Null
        Copy-Item (Join-Path $skinSrc '*') $skinDest -Recurse -Force
    } else {
        Write-Warning "皮肤素材目录不存在，跳过：$skinSrc"
    }
}

# ================= UPX =================
if ($Upx) {
    $candidates = @()
    if ($env:UPX_EXE) { $candidates += $env:UPX_EXE }
    $candidates += @('H:\JIMaX\bin\upx.exe', '.\bin\upx.exe', '.\third_party\upx.exe')
    $upxPath = $null
    foreach ($c in $candidates) {
        if (-not $c) { continue }
        if (Test-Path -LiteralPath $c) { $upxPath = $c; break }
    }
    if (-not $upxPath) {
        $cmd = Get-Command upx -ErrorAction SilentlyContinue
        if ($cmd) { $upxPath = $cmd.Source }
    }
    if (-not $upxPath) {
        Write-Warning '未找到 upx.exe（可用环境变量 UPX_EXE 指定），跳过压缩'
    } else {
        Write-Host ("[upx] {0}" -f (Split-Path -Leaf $exePath)) -ForegroundColor Yellow
        & $upxPath -9 -q $exePath
        if ($LASTEXITCODE -ne 0) { Write-Warning "UPX 压缩失败：$(Split-Path -Leaf $exePath)" }
    }
}

# ================= NSIS 安装器 =================
function Find-Makensis {
    $candidates = @()
    if ($env:NSIS_EXE) { $candidates += $env:NSIS_EXE }
    $candidates += @(
        'C:\Program Files (x86)\NSIS\makensis.exe',
        'C:\Program Files\NSIS\makensis.exe',
        'H:\JIMaX\bin\makensis.exe',
        '.\bin\makensis.exe'
    )
    foreach ($c in $candidates) {
        if ($c -and (Test-Path -LiteralPath $c)) { return $c }
    }
    $cmd = Get-Command makensis -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    return $null
}

if ($Installer) {
    $makensis = Find-Makensis
    if (-not $makensis) {
        throw '未找到 makensis.exe：请安装 NSIS 3（https://nsis.sourceforge.io/），或用环境变量 NSIS_EXE 指定路径'
    }

    $setupName = '{0}-{1}-setup.exe' -f $ProductSlug, $Version
    $setupPath = Join-Path $OutDirAbs $setupName
    $nsiPath = Join-Path $PSScriptRoot 'installer\guis.nsi'

    Write-Host ("[nsis] {0}  ->  {1}" -f $nsiPath, $setupPath) -ForegroundColor Cyan
    $nsisArgs = @(
        '/V2',
        "/DPRODUCT=$ProductName",
        "/DVERSION=$Version",
        "/DVI_VERSION=$viVersion",
        "/DCOMPANY=$Company",
        "/DDESC=$Description",
        "/DCOPYRIGHT=$Copyright",
        "/DCHANNEL=$Channel",
        "/DAPPEXE=$AppExe",
        "/DSRC_EXE=$exePath",
        "/DOUT_FILE=$setupPath",
        "/DICON_FILE=$iconPath"
    )
    # 只有确实带了素材才让 NSIS 打包 assets\intro
    if ((-not $NoAssets) -and (Test-Path -LiteralPath $assetDest)) {
        $nsisArgs += "/DASSETS_DIR=$assetDest"
    }
    # 桌宠素材同理：只有真正拷到了才让 NSIS 打包 assets\pet
    if ((-not $NoAssets) -and (Test-Path -LiteralPath $petDest)) {
        $nsisArgs += "/DPET_DIR=$petDest"
    }
    $nsisArgs += $nsiPath

    & $makensis @nsisArgs
    if ($LASTEXITCODE -ne 0) { throw "makensis 打包失败（退出码 $LASTEXITCODE）" }

    if (Test-Path -LiteralPath $setupPath) {
        $results += (Get-Item -LiteralPath $setupPath)
    }
}

# ================= 汇总 =================
Write-Host ''
'{0,-40} {1,10}' -f '产物', '体积(MB)' | Write-Host
'-' * 52 | Write-Host
foreach ($f in $results) {
    '{0,-40} {1,10:N2}' -f $f.Name, ($f.Length / 1MB) | Write-Host
}
if ((-not $NoAssets) -and (Test-Path -LiteralPath $assetDest)) {
    $n = (Get-ChildItem -LiteralPath $assetDest -File | Measure-Object).Count
    Write-Host ("素材 assets\intro：{0} 个文件" -f $n) -ForegroundColor DarkGray
}
if ((-not $NoAssets) -and (Test-Path -LiteralPath $petDest)) {
    $n = (Get-ChildItem -LiteralPath $petDest -File | Measure-Object).Count
    Write-Host ("素材 assets\pet：{0} 个文件" -f $n) -ForegroundColor DarkGray
}
Write-Host ''
Write-Host ("版本 {0} · 通道 {1} · 提交 {2}" -f $Version, $Channel, $Commit) -ForegroundColor Green
Write-Host ("程序 {0}" -f $exePath) -ForegroundColor Green
if ($Installer) {
    Write-Host ("安装器 {0}" -f (Join-Path $OutDirAbs ('{0}-{1}-setup.exe' -f $ProductSlug, $Version))) -ForegroundColor Green
}

if ($Run) {
    Write-Host ("[run] {0}" -f $exePath) -ForegroundColor Cyan
    Start-Process -FilePath $exePath -WorkingDirectory $OutDirAbs
}
