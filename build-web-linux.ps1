<#
  guis — 官网「网页版」打包：把 WebAssembly 站点 + 内嵌资源打成**一个 Linux 可执行文件**。

  产物：dist\guis-site-web-linux-amd64
  在 CentOS 上：
      chmod +x guis-site-web-linux-amd64
      ./guis-site-web-linux-amd64 -addr :522        # 浏览器访问 http://服务器IP:522/

  内部流程：
      1) go build -o main.wasm ./cmd/guis-site          （GOOS=js GOARCH=wasm）
      2) 复制 wasm_exec.js / web\site.html → index.html
      3) 字体子集化（GB2312+拉丁，36 MB → 约 9.6 MB，gzip 后约 4 MB）
      4) 全部复制到 cmd\guis-site-web\assets → go:embed
      5) GOOS=linux CGO_ENABLED=0 交叉编译单文件

  运行时提示：服务首次启动要做 brotli/gzip 预压缩（单核小机器几秒），
  结果按内容哈希缓存到可执行文件同目录的 .compress-cache（-cache <dir> 可改，
  -cache off 关闭）；之后重启只读缓存，通常 100ms 内完成。

  参数：
      ./build-web-linux.ps1                 # linux/amd64
      ./build-web-linux.ps1 -Arch arm64     # linux/arm64
      ./build-web-linux.ps1 -Serve          # 构建后在本机（Windows）起服务预览
#>
param(
    [string]$Arch = 'amd64',
    [string]$OutDir = 'dist',
    [switch]$Serve,
    [switch]$RebuildFont,
    [switch]$SkipFont,
    [ValidateSet('site', 'site+', 'full', 'gbk')][string]$FontProfile = 'site',
    [int]$ServePort = 8099
)

$ErrorActionPreference = 'Stop'
$OutputEncoding = [Console]::OutputEncoding
$root = Split-Path -Parent $MyInvocation.MyCommand.Definition
Set-Location $root

$work = Join-Path $root '.zym\build\wasm'
$assets = Join-Path $root 'cmd\guis-site-web\assets'
New-Item -ItemType Directory -Force -Path $work, $assets, (Join-Path $root $OutDir) | Out-Null

# ---------- 1/5 wasm ----------
Write-Host '[1/5] go build ./cmd/guis-site -> main.wasm' -ForegroundColor Cyan
$env:CGO_ENABLED = '0'
$env:GOOS = 'js'
$env:GOARCH = 'wasm'
$env:GOFLAGS = ''
& go build -trimpath -buildvcs=false -ldflags '-s -w' -o (Join-Path $work 'main.wasm') ./cmd/guis-site
if ($LASTEXITCODE -ne 0) { throw 'wasm 编译失败：./cmd/guis-site' }

# ---------- 2/5 wasm_exec.js + index.html ----------
Write-Host '[2/5] wasm_exec.js + index.html' -ForegroundColor Cyan
$goroot = (& go env GOROOT).Trim()
$execJs = Join-Path $goroot 'lib\wasm\wasm_exec.js'
if (-not (Test-Path $execJs)) { $execJs = Join-Path $goroot 'misc\wasm\wasm_exec.js' }
if (-not (Test-Path $execJs)) { throw "找不到 wasm_exec.js（GOROOT=$goroot）" }
Copy-Item $execJs (Join-Path $work 'wasm_exec.js') -Force
Copy-Item (Join-Path $root 'web\site.html') (Join-Path $work 'index.html') -Force

# ---------- 3/5 字体 ----------
Write-Host '[3/5] 字体：首屏子集 + 全站字体 + 全站粗体（.zym\site_fonts.ps1）' -ForegroundColor Cyan
$scan = @('cmd\guis-site', 'docs', 'web\site.html')
if (-not $SkipFont) {
    & (Join-Path $root '.zym\site_fonts.ps1') -OutDir $assets -Profile $FontProfile
    if ($LASTEXITCODE -ne 0) { throw '字体生成失败（先执行：python -m pip install fonttools）' }
    # 缺字校验：全站字体应覆盖站点全部文案；报出来的缺字说明字体源（雅黑+符号字体）本身没有该字形。
    $check = Join-Path $root '.zym\check_font_coverage.py'
    if (Test-Path $check) {
        # 注意：& python 的输出是字符串数组，直接 -match 不会填 $Matches（会报
        # "Cannot index into a null array"），必须先 Out-String 拼成单个字符串。
        $missing = (& python $check --font (Join-Path $assets 'font-full.ttf') --scan @scan 2>&1 | Out-String)
        if ($missing -match '缺 (\d+) 个字符') {
            Write-Host "[font] 警告：font-full.ttf 仍缺 $($Matches[1]) 个字形（见上行，多为 emoji/阿拉伯等雅黑没有的字符）" -ForegroundColor Yellow
        } else {
            Write-Host '[font] 覆盖校验：全站字体已包含站点全部文案字符' -ForegroundColor Green
        }
    }
} else {
    Write-Host '  已跳过字体生成（-SkipFont）' -ForegroundColor DarkGray
}

# ---------- 4/5 复制到 embed 目录 ----------
Write-Host '[4/5] 资源 → cmd\guis-site-web\assets' -ForegroundColor Cyan
Copy-Item (Join-Path $work 'main.wasm') $assets -Force
Copy-Item (Join-Path $work 'wasm_exec.js') $assets -Force
Copy-Item (Join-Path $work 'index.html') $assets -Force
Get-ChildItem $assets -File | Sort-Object Length -Descending |
    ForEach-Object { Write-Host ('  {0,-16} {1,8:N2} MB' -f $_.Name, ($_.Length / 1MB)) }

# ---------- 5/5 交叉编译 ----------
$name = "guis-site-web-linux-$Arch"
$exe = Join-Path $OutDir $name
Write-Host "[5/5] 交叉编译 $name (GOOS=linux, GOARCH=$Arch, CGO_ENABLED=0)" -ForegroundColor Cyan
$env:GOOS = 'linux'
$env:GOARCH = $Arch
$env:CGO_ENABLED = '0'
& go build -trimpath -buildvcs=false -ldflags '-s -w' -o $exe ./cmd/guis-site-web
if ($LASTEXITCODE -ne 0) { throw "编译失败（exit $LASTEXITCODE）" }

$item = Get-Item $exe
$hash = (Get-FileHash $exe -Algorithm SHA256).Hash
Write-Host ('[web] 产物: {0}' -f $item.FullName) -ForegroundColor Green
Write-Host ('[web] 大小: {0:N1} MB    SHA256: {1}' -f ($item.Length / 1MB), $hash)
Write-Host ''
Write-Host 'CentOS 上运行：' -ForegroundColor Cyan
Write-Host "  chmod +x $name"
Write-Host "  ./$name -addr :522            # 522 < 1024 需要 root 或 setcap cap_net_bind_service=+ep"
Write-Host "  ./$name -addr :8080           # 普通用户端口，nginx 反代到 80/443"
Write-Host ''
Write-Host 'systemd（/etc/systemd/system/guis-web.service）：' -ForegroundColor Cyan
Write-Host "  先建缓存目录并交给运行用户（否则重启后要重新压缩）：" -ForegroundColor DarkGray
Write-Host "  sudo mkdir -p /var/cache/guis-site && sudo chown www-data:www-data /var/cache/guis-site" -ForegroundColor DarkGray
Write-Host @"
[Unit]
Description=GUIS website
After=network.target

[Service]
ExecStart=/opt/guis/$name -addr :8080 -cache /var/cache/guis-site
Restart=always
User=www-data

[Install]
WantedBy=multi-user.target
"@

# ---------- 可选：本机预览（Windows 版） ----------
if ($Serve) {
    Write-Host '[preview] 编译 Windows 版并起服务…' -ForegroundColor Yellow
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $winExe = Join-Path $work 'guis-site-web.exe'
    & go build -trimpath -buildvcs=false -o $winExe ./cmd/guis-site-web
    if ($LASTEXITCODE -ne 0) { throw 'Windows 预览版编译失败' }
    & $winExe -addr ("127.0.0.1:{0}" -f $ServePort)
}
