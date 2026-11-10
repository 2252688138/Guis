<#
.SYNOPSIS
GUIS 组件演示（gallery）Android 构建脚本：NDK 编 libgallery.so → aapt2 打 APK → apksigner 签名。

.DESCRIPTION
  产物：dist/android-gallery/gallery.apk（已签名，可直接 adb install）

  依赖（都可自动探测）：
    - Android NDK       ANDROID_NDK_HOME / ANDROID_NDK_ROOT / <SDK>\ndk\*
    - Android build-tools  <SDK>\build-tools\*（aapt2 / zipalign / apksigner）
    - android.jar        <SDK>\platforms\android-34\android.jar
    - JDK                keytool（首次运行自动生成 debug 签名）

  说明：
    - Android 端使用 cgo（移动端唯一可行路径），因此必须 CGO_ENABLED=1 且指定 NDK 的 clang。
    - 不依赖 gradle：manifest 由 aapt2 link 编译，.so 直接塞进 lib/<abi>/，
      最后 zipalign 对齐 + apksigner 签名（顺序不能反：先对齐再签名）。

.EXAMPLE
./build-android-apk.ps1                                # 3 个 ABI + 签名 → gallery.apk
./build-android-apk.ps1 -Abi arm64-v8a                 # 只编 64 位（最快）
./build-android-apk.ps1 -NoPackage                     # 只编 so，不打包 APK
#>
param(
    [ValidateSet('arm64-v8a', 'armeabi-v7a', 'x86', 'x86_64', 'all')]
    [string]$Abi = 'all',
    [switch]$NoPackage,
    [string]$OutDir = 'dist/android-gallery',
    [string]$AndroidPackage = 'com.guis.gallery',
    [string]$LibName = 'gallery'
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $MyInvocation.MyCommand.Path)

$minSdk = 24
$targetSdk = 34

# ---- 工具链探测 ----

function Find-Ndk {
    foreach ($p in @($env:ANDROID_NDK_HOME, $env:ANDROID_NDK_ROOT)) {
        if ($p -and (Test-Path $p)) { return $p }
    }
    $sdk = Find-Sdk
    if ($sdk -and (Test-Path "$sdk\ndk")) {
        $latest = Get-ChildItem "$sdk\ndk" -Directory | Sort-Object Name -Descending | Select-Object -First 1
        if ($latest) { return $latest.FullName }
    }
    return $null
}

function Find-Sdk {
    foreach ($p in @($env:ANDROID_HOME, $env:ANDROID_SDK_ROOT, "$env:LOCALAPPDATA\Android\Sdk")) {
        if ($p -and (Test-Path $p)) { return $p }
    }
    return $null
}

function Find-BuildTools([string]$sdk) {
    if (-not $sdk -or -not (Test-Path "$sdk\build-tools")) { return $null }
    $latest = Get-ChildItem "$sdk\build-tools" -Directory | Sort-Object Name -Descending | Select-Object -First 1
    if ($latest) { return $latest.FullName }
    return $null
}

$sdk = Find-Sdk
$ndk = Find-Ndk
if (-not $ndk) { throw '未找到 Android NDK：请安装 NDK 并设置 ANDROID_NDK_HOME' }
Write-Host "[ndk] $ndk" -ForegroundColor Cyan

$hostTag = 'windows-x86_64'
$toolchain = Join-Path $ndk "toolchains\llvm\prebuilt\$hostTag\bin"
if (-not (Test-Path $toolchain)) { throw "找不到 NDK 工具链目录：$toolchain（非 Windows 主机请改用对应 prebuilt 目录）" }

# ---- 1. 编译 lib<LibName>.so ----

# GOARCH / GOARM → NDK clang 前缀（minSdk 与清单保持一致）。
# 为什么还要打 x86 / x86_64：模拟器（MEmu / 夜神 / AVD x86 镜像）跑 ARM 版会走
# libhoudini 翻译层，Go 运行时的 vDSO nanotime 在那里返回 0，启动即 crash
# （logcat: "fatal error: nanotime returning zero"），必须用原生 ABI。
$targets = @(
    @{ Abi = 'arm64-v8a';   GOARCH = 'arm64'; GOARM = '';  CC = "aarch64-linux-android$minSdk-clang.cmd" },
    @{ Abi = 'armeabi-v7a'; GOARCH = 'arm';   GOARM = '7'; CC = "armv7a-linux-androideabi$minSdk-clang.cmd" },
    @{ Abi = 'x86';         GOARCH = '386';   GOARM = '';  CC = "i686-linux-android$minSdk-clang.cmd" },
    @{ Abi = 'x86_64';      GOARCH = 'amd64'; GOARM = '';  CC = "x86_64-linux-android$minSdk-clang.cmd" }
)
if ($Abi -ne 'all') { $targets = $targets | Where-Object { $_.Abi -eq $Abi } }

$built = @()
foreach ($t in $targets) {
    $cc = Join-Path $toolchain $t.CC
    if (-not (Test-Path $cc)) {
        Write-Host "[skip] 缺少编译器 $cc" -ForegroundColor Yellow
        continue
    }
    $abiDir = Join-Path $OutDir $t.Abi
    New-Item -ItemType Directory -Force -Path $abiDir | Out-Null
    $so = Join-Path $abiDir "lib$LibName.so"

    Write-Host ("[build] {0}  GOARCH={1}" -f $t.Abi, $t.GOARCH) -ForegroundColor Cyan
    $env:CC = $cc
    $env:CGO_ENABLED = '1'
    $env:GOOS = 'android'
    $env:GOARCH = $t.GOARCH
    if ($t.GOARM) { $env:GOARM = $t.GOARM } else { Remove-Item Env:\GOARM -ErrorAction SilentlyContinue }

    & go build -buildmode=c-shared -trimpath -ldflags '-s -w' -o $so ./cmd/gallery
    if ($LASTEXITCODE -ne 0) { throw "构建失败：$($t.Abi)" }
    $built += @{ Abi = $t.Abi; Path = $so }
}

# c-shared 会额外生成 .h（App 不需要）
Get-ChildItem $OutDir -Recurse -Filter '*.h' -ErrorAction SilentlyContinue | Remove-Item -Force

Write-Host ''
foreach ($b in $built) {
    $f = Get-Item -LiteralPath $b.Path
    '{0,-24} {1,10:N2} MB' -f $b.Path, ($f.Length / 1MB) | Write-Host
}

if ($NoPackage) { return }

# ---- 2. 打包 APK（aapt2 → 加 so → zipalign → apksigner）----

$bt = Find-BuildTools $sdk
if (-not $bt) { throw '未找到 Android build-tools（需要 aapt2 / zipalign / apksigner）' }
$aapt2 = Join-Path $bt 'aapt2.exe'
$zipalign = Join-Path $bt 'zipalign.exe'
$apksigner = Join-Path $bt 'apksigner.bat'
foreach ($x in @($aapt2, $zipalign, $apksigner)) {
    if (-not (Test-Path $x)) { throw "缺少构建工具：$x" }
}

$androidJar = Join-Path $sdk "platforms\android-$targetSdk\android.jar"
if (-not (Test-Path $androidJar)) { throw "缺少 $androidJar（请安装 platforms;android-$targetSdk）" }

$manifest = Join-Path 'mobile\gallery' 'AndroidManifest.xml'
if (-not (Test-Path $manifest)) { throw "缺少清单：$manifest" }

$versionName = '0.1.0'
if (Test-Path 'VERSION.txt') {
    $v = (Get-Content 'VERSION.txt' -Raw).Trim()
    if ($v) { $versionName = $v }
}

Write-Host ''
Write-Host "[apk] 编译清单 $manifest" -ForegroundColor Cyan
$stage = Join-Path $OutDir 'apk-stage'
$baseApk = Join-Path $OutDir 'base.apk'
$unsigned = Join-Path $OutDir 'app-unsigned.apk'
$aligned = Join-Path $OutDir 'app-aligned.apk'
$signedApk = Join-Path $OutDir "$LibName.apk"

if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
foreach ($f in @($baseApk, $unsigned, $aligned, $signedApk)) {
    if (Test-Path $f) { Remove-Item $f -Force }
}
New-Item -ItemType Directory -Force -Path $stage | Out-Null

# aapt2 link：清单 → 二进制 XML + resources.arsc（本 App 没有自绘资源，Theme 走 @android:）
& $aapt2 link -o $baseApk --manifest $manifest -I $androidJar `
    --min-sdk-version $minSdk --target-sdk-version $targetSdk `
    --version-code 1 --version-name $versionName --auto-add-overlay
if ($LASTEXITCODE -ne 0) { throw 'aapt2 link 失败' }

# 解出 base.apk，补上原生库后重新压包
Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::ExtractToDirectory($baseApk, $stage)

# ---- Java 桥（classes.dex）----
# NativeActivity 没有 InputConnection，软键盘的中文上屏必须靠 Java 侧的隐藏 EditText
# 承接（mobile/gallery/java/com/guis/gallery/ImeBridge.java），所以 APK 要带 dex：
# javac（用 android.jar 当 classpath）→ d8（--min-api 对齐清单）→ classes.dex。
$javaSrc = Join-Path 'mobile\gallery' 'java'
if (Test-Path $javaSrc) {
    Write-Host '[apk] 编译 Java 输入法桥 → classes.dex' -ForegroundColor Cyan
    $classDir = Join-Path $OutDir 'classes'
    $dexDir = Join-Path $OutDir 'dex'
    if (Test-Path $classDir) { Remove-Item $classDir -Recurse -Force }
    if (Test-Path $dexDir) { Remove-Item $dexDir -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $classDir, $dexDir | Out-Null

    $sources = Get-ChildItem $javaSrc -Recurse -Filter *.java | ForEach-Object { $_.FullName }
    if (-not $sources) { throw "Java 源目录为空：$javaSrc" }
    & javac -encoding UTF-8 -source 8 -target 8 -nowarn -classpath $androidJar -d $classDir $sources
    if ($LASTEXITCODE -ne 0) { throw 'javac 编译 Java 桥失败（需要 JDK）' }

    $classes = Get-ChildItem $classDir -Recurse -Filter *.class | ForEach-Object { $_.FullName }
    & (Join-Path $bt 'd8.bat') --release --min-api $minSdk --output $dexDir $classes
    if ($LASTEXITCODE -ne 0) { throw 'd8 生成 classes.dex 失败' }
    Copy-Item (Join-Path $dexDir 'classes.dex') (Join-Path $stage 'classes.dex') -Force
}
foreach ($b in $built) {
    $dest = Join-Path $stage "lib\$($b.Abi)"
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Copy-Item -LiteralPath $b.Path -Destination (Join-Path $dest "lib$LibName.so") -Force
}
# 重新压包：resources.arsc 必须 stored（不压缩），否则 Android 11+ 直接拒装：
#   Failure [-124]: Targeting R+ (version 30 and above) requires the resources.arsc
#   of installed APKs to be stored uncompressed and aligned on a 4-byte boundary
# 其余条目（含 .so，清单里 extractNativeLibs=true 允许压缩）用 Optimal。
$stageFull = (Resolve-Path $stage).Path
$zip = [System.IO.Compression.ZipFile]::Open($unsigned, [System.IO.Compression.ZipArchiveMode]::Create)
try {
    foreach ($f in (Get-ChildItem $stageFull -Recurse -File)) {
        $rel = $f.FullName.Substring($stageFull.Length + 1).Replace('\', '/')
        $level = [System.IO.Compression.CompressionLevel]::Optimal
        if ($rel -eq 'resources.arsc') { $level = [System.IO.Compression.CompressionLevel]::NoCompression }
        $entry = $zip.CreateEntry($rel, $level)
        $es = $entry.Open()
        $fs = [System.IO.File]::OpenRead($f.FullName)
        try { $fs.CopyTo($es) } finally { $fs.Dispose(); $es.Dispose() }
    }
} finally {
    $zip.Dispose()
}

Write-Host '[apk] zipalign' -ForegroundColor Cyan
& $zipalign -f -p 4 $unsigned $aligned
if ($LASTEXITCODE -ne 0) { throw 'zipalign 失败' }

# debug 签名（keystore 放 .zym 下，已被 .gitignore 忽略；换正式签名改这三个 -Pass 参数即可）
$ksDir = '.zym\android'
New-Item -ItemType Directory -Force -Path $ksDir | Out-Null
$ks = Join-Path $ksDir 'debug.jks'
if (-not (Test-Path $ks)) {
    Write-Host "[apk] 生成 debug 签名 $ks" -ForegroundColor Cyan
    $keytool = 'keytool'
    & $keytool -genkeypair -v -keystore $ks -storepass android -keypass android `
        -alias guis -keyalg RSA -keysize 2048 -validity 10000 `
        -dname 'CN=GUIS Debug, O=GUIS, C=CN' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'keytool 生成签名失败（需要 JDK）' }
}

Write-Host '[apk] apksigner 签名' -ForegroundColor Cyan
& $apksigner sign --ks $ks --ks-pass pass:android --key-pass pass:android `
    --ks-key-alias guis --out $signedApk $aligned
if ($LASTEXITCODE -ne 0) { throw 'apksigner 签名失败' }

& $apksigner verify --print-certs $signedApk | Select-Object -First 6 | Write-Host

# 中间产物不留
Remove-Item $stage -Recurse -Force -ErrorAction SilentlyContinue
foreach ($f in @($baseApk, $unsigned, $aligned)) {
    if (Test-Path $f) { Remove-Item $f -Force }
}

Write-Host ''
$apk = Get-Item -LiteralPath $signedApk
'{0,-40} {1,8:N2} MB' -f $apk.FullName, ($apk.Length / 1MB) | Write-Host
Write-Host ("安装：adb install -r {0}" -f $apk.Name) -ForegroundColor Green
