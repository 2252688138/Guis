<#
.SYNOPSIS
    文档与导出 API 一致性检查：docs/api 里用反引号引用的标识符必须真实存在于 ui / uidsl 的导出 API 里。

.DESCRIPTION
    文档滞后是自研框架最常见的“慢性病”：函数改名、删除、合并后文档没人改，新用户照着写必错。
    本脚本用 `go doc -all` 采集真实 API（含函数 / 类型 / 常量 / 变量 / 方法 / 字段），
    再从 Markdown 里抽出反引号标识符逐一比对，缺失占比超过阈值就以退出码 1 失败。

    原则：只检查“看起来是 Go 导出标识符”的词（首字母大写、长度 >= 3），
    按键名、缩写等误报源放在 $ignore 里；宁可漏报也不要误报，否则门禁会被绕过。

.EXAMPLE
    # 日常检查（缺失超过 5% 失败）
    .\scripts\doc-api-check.ps1

.EXAMPLE
    # 只打印不拦截（整理文档时用）
    .\scripts\doc-api-check.ps1 -MaxMissingRatio 1.0
#>
[CmdletBinding()]
param(
    [string]$Docs = "docs/api",
    [string]$Package = "./ui/",
    [string]$PackageExtra = "./ui/uidsl/",
    [double]$MaxMissingRatio = 0.02
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    if (-not (Test-Path $Docs)) {
        Write-Host "找不到文档目录：$Docs" -ForegroundColor Red
        exit 2
    }

    # ---- 1. 采集真实导出 API ----
    $symbols = New-Object 'System.Collections.Generic.HashSet[string]'
    $morePkgs = @('./internal/office/', './internal/geom/', './internal/audio/', './internal/display/', './internal/text/')
    foreach ($pkg in (@($Package, $PackageExtra) + $morePkgs)) {
        if (-not $pkg) { continue }
        $txt = & go doc -all $pkg 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Host "go doc 失败：$pkg" -ForegroundColor Red
            $txt | ForEach-Object { Write-Host $_ }
            exit 2
        }
        foreach ($line in $txt) {
            foreach ($m in [regex]::Matches([string]$line, '[A-Za-z_][A-Za-z0-9_]{2,}')) {
                [void]$symbols.Add($m.Value)
            }
        }
    }
    Write-Host "采集到导出标识符：$($symbols.Count) 个（来自 $Package $PackageExtra）" -ForegroundColor Cyan

    # 非 API 但常出现于反引号里的词：按键名、常见缩写（否则误报会埋掉真问题）
    $ignore = @(
        'Backspace', 'Delete', 'Enter', 'Space', 'Escape', 'Shift', 'Ctrl', 'Cmd', 'Alt',
        'Tab', 'Home', 'End', 'PageUp', 'PageDown', 'Insert', 'Left', 'Right', 'Up', 'Down',
        'NumLock', 'CapsLock', 'AltGr', 'Esc', 'Meta', 'Super', 'Hyper',
        'F1', 'F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10', 'F11', 'F12',
        'TODO', 'FIXME', 'NOTE', 'OK', 'Cancel', 'ID', 'URL', 'URI', 'JSON', 'YAML', 'HTTP',
        'PNG', 'JPEG', 'JPG', 'SVG', 'GIF', 'WEBP', 'UTF', 'ASCII', 'CPU', 'GPU', 'DPI', 'PPI',
        'API', 'UI', 'UX', 'DSL', 'CRLF', 'LF', 'FPS', 'IME', 'LSP', 'PCM', 'RGBA', 'ARGB',
        'Linux', 'Windows', 'macOS', 'Android', 'iOS', 'Web', 'WASM', 'X11', 'Wayland', 'DirectX', 'OpenGL', 'Metal', 'Vulkan',
        # 字体名 / 文档占位符 / 组件泛称（不是 Go 标识符）
        'SimHei', 'SimSun', 'Segoe', 'Symbol', 'ColXxx', 'Col', 'Dock', 'Tour', 'Composer', 'Tint',
        # Win32 / X11 / COM 底层接口名（文档在描述实现机制，不是 guis 导出 API）
        'BadAccess', 'ExitWindowsEx', 'KeyPress', 'SeShutdownPrivilege', 'XdndFinished', 'XTranslateCoordinates',
        'HrInit', 'SetOverlayIcon', 'GetUserDefaultUILanguage', 'LCIDToLocaleName', 'ErrNotSupported'
    )

    # ---- 2. 遍历文档 ----
    $files = Get-ChildItem $Docs -Filter *.md -File | Sort-Object Name
    $checked = 0
    $missingTotal = New-Object 'System.Collections.Generic.HashSet[string]'
    $report = @()

    foreach ($f in $files) {
        $text = Get-Content $f.FullName -Raw
        $tokens = [regex]::Matches($text, '`([A-Za-z_][A-Za-z0-9_]*)`') |
            ForEach-Object { $_.Groups[1].Value } |
            Where-Object {
                $_.Length -ge 3 -and
                $_ -cmatch '^[A-Z]' -and
                $ignore -notcontains $_ -and
                # 含下划线的一律是环境变量 / Win32 常量（GUIS_LSP_CMD、WM_DROPFILES）
                $_ -notmatch '_' -and
                # 全大写缩写不是 API 名
                $_ -cnotmatch '^[A-Z0-9]+$' -and
                # Win32 风格的 ...W / ...A 结尾（MessageBoxW、SHGetFileInfoW）
                -not ($_.Length -ge 8 -and $_ -cmatch '[WA]$') -and
                # 全大驼峰但以 2 位平台前缀开头的第三方接口（NSMenu/CF_HTML/CGEventTap/Xdnd*）
                $_ -cnotmatch '^(NS|CF|CG|IT|ID|HR|SH|XD|XG|WM|LP|MOD|SE|IAudio)'
            } |
            Sort-Object -Unique
        if (-not $tokens) { continue }

        $miss = @($tokens | Where-Object { -not $symbols.Contains($_) })
        $checked += $tokens.Count
        foreach ($m in $miss) { [void]$missingTotal.Add($m) }
        if ($miss.Count -gt 0) {
            $report += [pscustomobject]@{ File = $f.Name; Total = $tokens.Count; Missing = $miss.Count; Names = ($miss -join ', ') }
        }
    }

    Write-Host ""
    if ($report.Count -eq 0) {
        Write-Host "✓ 所有文档引用的 API 均存在（共检查 $checked 个标识符 / $($files.Count) 篇）" -ForegroundColor Green
        exit 0
    }

    Write-Host "文档里引用了不存在的 API：" -ForegroundColor Yellow
    foreach ($r in $report) {
        Write-Host ("  {0}  ({1}/{2})  →  {3}" -f $r.File, $r.Missing, $r.Total, $r.Names)
    }

    $ratio = 0.0
    if ($checked -gt 0) { $ratio = [math]::Round($missingTotal.Count / $checked, 4) }
    $budget = [math]::Round($checked * $MaxMissingRatio)
    Write-Host ""
    Write-Host ("总计：检查 {0} 个标识符，缺失 {1} 个（{2:P2}），阈值 {3:P0} ⇒ 允许 {4} 个" -f `
        $checked, $missingTotal.Count, $ratio, $MaxMissingRatio, $budget)

    if ($missingTotal.Count -gt $budget) {
        Write-Host "✗ 文档与 API 不一致：请修正文档，或在 docs 中标注为规划中的 API" -ForegroundColor Red
        exit 1
    }
    Write-Host "✓ 缺失在阈值内" -ForegroundColor Green
    exit 0
}
finally {
    Pop-Location
}
