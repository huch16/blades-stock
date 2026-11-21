param(
    [ValidateSet('frontend', 'go', 'wails', 'dev', 'help')]
    [string]$Command = 'help'
)

$ErrorActionPreference = "Stop"

# 【修复】只**追加**缺失的工具目录，绝不整体覆盖 $env:PATH。
#
# 原实现把 PATH 直接赋值为一串硬编码路径（还写死了某台机器的用户目录），
# 后果有两条，且两条都是硬故障：
#   1. 用户 PATH 里的 git / python / 其他工具全部被丢掉；
#   2. 配合上面的 $ErrorActionPreference = "Stop"，脚本第 32 行首次调用
#      `git rev-parse` 会抛 CommandNotFoundException —— 该异常是**终止性**的
#      （`2>$null` 只能压住原生命令的 stderr，压不住 PowerShell 自身的
#      命令找不到异常），于是整个构建在第 32 行直接中止，一个产物都不生成。
# 现在改为「存在且未在 PATH 中才追加」，并用 $env:USERPROFILE / $env:APPDATA
# 派生路径，使脚本不再绑定特定机器。
$toolDirs = @(
    "$env:USERPROFILE\go\go\bin",
    "$env:USERPROFILE\go\bin",
    "$env:APPDATA\npm",
    "C:\Program Files\nodejs",
    "C:\Program Files\Git\cmd"
)
foreach ($dir in $toolDirs) {
    if ($dir -and (Test-Path -LiteralPath $dir) -and ($env:PATH -notlike "*$dir*")) {
        $env:PATH = "$env:PATH;$dir"
    }
}

# Get-ShortCommit 返回当前提交的短哈希；git 不可用时回退 "dev" 而**不中断构建**。
# 构建产物只是少了版本溯源信息，不该因此完全构建不出来。
function Get-ShortCommit {
    $fallback = "dev"
    try {
        $c = & git rev-parse --short HEAD 2>$null
        if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($c)) {
            return $c.Trim()
        }
    } catch {
        Write-Host "  [warn] git 不可用，版本号回退为 $fallback" -ForegroundColor Yellow
    }
    return $fallback
}

$Root = $PSScriptRoot
$FrontendDir = "$Root\frontend"

function Build-Frontend {
    Write-Host "=== Building Frontend ===" -ForegroundColor Cyan
    if (Test-Path "$FrontendDir\src\wailsjs") {
        Remove-Item "$FrontendDir\src\wailsjs" -Recurse -Force
    }
    Push-Location $FrontendDir
    try {
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "npm build failed" }
        Write-Host "Frontend build OK" -ForegroundColor Green
    } finally {
        Pop-Location
    }
}

function Build-Go {
    Write-Host "=== Building Go Binary ===" -ForegroundColor Cyan
    Push-Location $Root
    try {
        # P3.4: build-time 自动重建 graph manifest
        Rebuild-Graph
        $commit = Get-ShortCommit
        $version = "3.2.0"
        $buildKey = "blades-stock-release-key"
        $ldflags = "-s -w -H=windowsgui -X main.Version=$version -X main.VersionCommit=$commit -X main.BuildKey=$buildKey"
        Write-Host "  Building desktop binary with tags desktop,production..." -ForegroundColor Gray
        go build -tags "desktop,production" -ldflags $ldflags -o build/bin/blades-stock.exe .
        if ($LASTEXITCODE -ne 0) { throw "go build blades-stock.exe failed" }
        Write-Host "  Building server binary cmd/server..." -ForegroundColor Gray
        go build -ldflags "-s -w -X main.Version=$version -X main.VersionCommit=$commit" -o build/bin/blades-stock-server.exe ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw "go build blades-stock-server.exe failed" }
        Copy-DataDir
        Write-Host "Go build OK: build/bin/blades-stock.exe & build/bin/blades-stock-server.exe" -ForegroundColor Green
    } finally {
        Pop-Location
    }
}

function Rebuild-Graph {
    Write-Host "=== Rebuilding Graph Manifest (P3.4) ===" -ForegroundColor Cyan
    Push-Location $Root
    try {
        if (-not (Test-Path "$Root\.venv\Scripts\python.exe")) {
            Write-Host "  python.exe not found at .venv\Scripts\python.exe, skipping graph rebuild" -ForegroundColor Yellow
            return
        }
        $commit = Get-ShortCommit
        & "$Root\.venv\Scripts\python.exe" "$Root\scripts\build_graph.py" `
            --root "$Root" `
            --output "$Root\.graph" `
            --commit $commit
        if ($LASTEXITCODE -ne 0) {
            Write-Host "  graph manifest rebuild failed (non-fatal, continuing)" -ForegroundColor Yellow
        }
    } finally {
        Pop-Location
    }
}

function Copy-DataDir {
    $destDir = "$Root\build\bin\data"
    if (-not (Test-Path -LiteralPath $destDir)) {
        New-Item -ItemType Directory -Path $destDir -Force | Out-Null
    }
    # checkpoint WAL then copy
    Push-Location $Root
    try {
        go run ./cmd/checkpoint_db/ "data/stock.db"
        Copy-Item -LiteralPath "data\stock.db" -Destination "$destDir\stock.db" -Force
        Remove-Item -LiteralPath "$destDir\stock.db-wal" -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath "$destDir\stock.db-shm" -Force -ErrorAction SilentlyContinue
    } finally {
        Pop-Location
    }
    Write-Host "Data dir synced to build/bin/data" -ForegroundColor Green
}

function Build-Wails {
    Write-Host "=== Building Wails Desktop ===" -ForegroundColor Cyan
    Push-Location $Root
    try {
        $runtimeJs = "$Root\frontend\wailsjs\runtime\runtime.js"
        $runtimeBackup = $null
        if (Test-Path -LiteralPath $runtimeJs) {
            $runtimeBackup = Get-Content -LiteralPath $runtimeJs -Raw -Encoding UTF8
        }
        $env:GOTOOLCHAIN = "go1.26.0"
        wails build -skipbindings -skipembedcreate 
        if ($LASTEXITCODE -ne 0) { throw "wails build failed" }
        # 保护 runtime.js 的幂等防重机制
        if ($runtimeBackup -and (Test-Path -LiteralPath $runtimeJs)) {
            [System.IO.File]::WriteAllText($runtimeJs, $runtimeBackup, [System.Text.Encoding]::UTF8)
            Write-Host "Protected custom runtime.js idempotency deduplication" -ForegroundColor Green
        }
        Copy-DataDir
        Write-Host "Wails build OK" -ForegroundColor Green
    } finally {
        Pop-Location
    }
}

function Dev-Wails {
    Write-Host "=== Starting Wails Dev ===" -ForegroundColor Cyan
    Push-Location $Root
    try {
        $env:GOTOOLCHAIN = "go1.26.4"
        wails dev -skipbindings -skipembedcreate
        if ($LASTEXITCODE -ne 0) { throw "wails dev failed" }
    } finally {
        Pop-Location
    }
}

function Show-Help {
    Write-Host @"
Usage: .\build.ps1 <command>

Commands:
  frontend    Build Vue 3 frontend (npm run build)
  go          Build Go backend binary only
  wails       Full Wails desktop build (frontend + Go)
  dev         Run Wails dev mode with hot-reload
  help        Show this help
"@
}

switch ($Command) {
    'frontend' { Build-Frontend }
    'go'       { Build-Go }
    'wails'    { Build-Wails }
    'dev'      { Dev-Wails }
    'help'     { Show-Help }
}
