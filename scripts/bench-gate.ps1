<#
.SYNOPSIS
    guis 性能基线门禁：跑基准并与基线对比，回归超过阈值则失败。

.DESCRIPTION
    基线文件每行一个用例：`BenchmarkXxx<TAB>ns/op`。
    -Update 时把当前结果写成新基线（提交进仓库，供 CI / 本地对比）。
    本地没有 Go 基准统计工具也能用，纯 PowerShell 解析。

.EXAMPLE
    # 首次建立 / 更新基线
    .\scripts\bench-gate.ps1 -Update

.EXAMPLE
    # 日常回归检查（回归超过 10% 即失败，退出码 1）
    .\scripts\bench-gate.ps1

.EXAMPLE
    # 只跑某几个用例，阈值放宽到 20%
    .\scripts\bench-gate.ps1 -Filter "Frame|Table" -Threshold 1.20
#>
[CmdletBinding()]
param(
    [string]$Baseline = "bench/baseline.txt",
    [string]$Package = "./ui/",
    [string]$Filter = "",
    [string]$Benchtime = "200ms",
    [double]$Threshold = 1.10,
    [int]$Count = 3,
    [double]$MinAbsNs = 15000,
    [switch]$Update
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $benchArg = "."
    if ($Filter) { $benchArg = $Filter }

    Write-Host "跑基准：go test $Package -bench $benchArg -benchtime $Benchtime ..." -ForegroundColor Cyan
    $raw = & go test $Package -run '^$' -bench $benchArg -benchtime $Benchtime -count $Count 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "基准执行失败：" -ForegroundColor Red
        $raw | ForEach-Object { Write-Host $_ }
        exit 2
    }

    # 解析：BenchmarkFrameBuild-12   2280   147745 ns/op   289701 B/op   546 allocs/op
    $current = @{}
    foreach ($line in $raw) {
        if ($line -match '^(Benchmark\S+?)-\d+\s+\d+\s+([\d\.]+)\s+ns/op') {
            $name = $Matches[1]
            $v = [double]$Matches[2]
            # 多次采样取最小值：单次 -count 1 的抖动经常超过 10%，会把噪声误报成回归
            if (-not $current.ContainsKey($name) -or $v -lt $current[$name]) { $current[$name] = $v }
        }
    }
    if ($current.Count -eq 0) {
        Write-Host "没有解析到任何基准结果，请检查 -Filter 是否正确。" -ForegroundColor Red
        exit 2
    }

    # -Update：写基线
    if ($Update) {
        $dir = Split-Path -Parent $Baseline
        if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
        $lines = $current.GetEnumerator() | Sort-Object Name | ForEach-Object {
            "{0}`t{1}" -f $_.Key, [string]::Format("{0:F0}", $_.Value)
        }
        Set-Content -Path $Baseline -Value $lines -Encoding UTF8
        Write-Host ("已写入基线 {0}（{1} 个用例）" -f $Baseline, $current.Count) -ForegroundColor Green
        exit 0
    }

    if (-not (Test-Path $Baseline)) {
        Write-Host ("基线文件不存在：{0}`n用 -Update 先生成一份。" -f $Baseline) -ForegroundColor Yellow
        exit 2
    }

    # 读基线
    $base = @{}
    foreach ($line in Get-Content $Baseline) {
        if ($line -match '^\s*(Benchmark\S+)\s+([\d\.]+)\s*$') {
            $base[$Matches[1]] = [double]$Matches[2]
        }
    }

    # 对比
    $regressions = @()
    $rows = @()
    foreach ($name in ($current.Keys | Sort-Object)) {
        if (-not $base.ContainsKey($name)) {
            $rows += [pscustomobject]@{ 用例 = $name; 基线 = "（新增）"; 当前 = "{0:N0}" -f $current[$name]; 变化 = "-" ; 状态 = "new" }
            continue
        }
        $b = $base[$name]
        $c = $current[$name]
        $ratio = if ($b -gt 0) { $c / $b } else { 1 }
        $pct = ($ratio - 1) * 100
        $state = "ok"
        # 微秒级用例的相对抖动天然偏大：要求「相对超阈值」且「绝对差超门槛」才判回归
        if ($ratio -gt $Threshold -and ($c - $b) -ge $MinAbsNs) { $state = "FAIL"; $regressions += $name }
        elseif ($ratio -lt 0.9) { $state = "改善" }
        $rows += [pscustomobject]@{
            用例 = $name
            基线 = "{0:N0}" -f $b
            当前 = "{0:N0}" -f $c
            变化 = ("{0:+0.0;-0.0;0.0}%" -f $pct)
            状态 = $state
        }
    }

    $rows | Format-Table -AutoSize | Out-String | Write-Host

    if ($regressions.Count -gt 0) {
        Write-Host ("性能回归（阈值 +{0:P0}）：{1}" -f ($Threshold - 1), ($regressions -join ", ")) -ForegroundColor Red
        Write-Host "确认是预期变化后，用 -Update 更新基线并提交。" -ForegroundColor Yellow
        exit 1
    }
    Write-Host "全部用例在阈值内。" -ForegroundColor Green
    exit 0
}
finally {
    Pop-Location
}
