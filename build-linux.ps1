<#
  guis — 官网单文件 Linux 打包（CentOS / RHEL / Ubuntu 通用）

  纯 Go（CGO_ENABLED=0）交叉编译，产物是**一个静态链接的 ELF 可执行文件**，
  不依赖 libc / glibc 版本、不需要 runtime、不需要安装任何包：

      ./build-linux.ps1                 # linux/amd64 → dist/guis-site-linux-amd64
      ./build-linux.ps1 -Arch arm64     # linux/arm64
      ./build-linux.ps1 -Strip          # 额外裁剪符号表（-s -w）

  运行环境要求：
    * X11 图形会话（CentOS 最小化安装需 `yum groupinstall "Server with GUI"`，
      或至少有 Xorg + GLX；无显示器时可用 Xvfb + x11vnc 远程查看）
    * 中文字体（裸机常缺）：`yum install -y wqy-zenhei-fonts`
      或 `yum install -y google-noto-sans-cjk-ttc-fonts`
#>
param(
    [string]$Arch = 'amd64',
    [string]$OutDir = 'dist',
    [switch]$Strip
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Definition
Set-Location $root

$name = "guis-site-linux-$Arch"
$exe = Join-Path $OutDir $name
New-Item -ItemType Directory -Force -Path (Join-Path $root $OutDir) | Out-Null

$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = $Arch
$env:GOFLAGS = '' # 避免本机 GOFLAGS 干扰交叉编译

$ld = if ($Strip) { '-s -w' } else { '-s -w' }
Write-Host "[linux] 编译 $name (CGO_ENABLED=0, GOOS=linux, GOARCH=$Arch)" -ForegroundColor Cyan
& go build -trimpath -buildvcs=false -ldflags $ld -o $exe ./cmd/guis-site
if ($LASTEXITCODE -ne 0) { throw "编译失败（exit $LASTEXITCODE）" }

$item = Get-Item $exe
$hash = (Get-FileHash $exe -Algorithm SHA256).Hash

Write-Host ("[linux] 产物: {0}" -f $item.FullName) -ForegroundColor Green
Write-Host ("[linux] 大小: {0:N1} MB    SHA256: {1}" -f ($item.Length / 1MB), $hash)
Write-Host ""
Write-Host "拷到 CentOS 后：" -ForegroundColor Cyan
Write-Host "  chmod +x $name"
Write-Host "  ./$name"
Write-Host ""
Write-Host "无显示器（纯服务器）远程查看：" -ForegroundColor Cyan
Write-Host "  Xvfb :99 -screen 0 1440x900x24 & DISPLAY=:99 ./$name"
