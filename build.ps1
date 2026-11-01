param(
    [ValidateSet('frontend', 'go', 'wails', 'dev', 'help')]
    [string]$Command = 'help'
)

$ErrorActionPreference = "Stop"
$env:PATH = "C:\Users\huch\go\go\bin;C:\Users\huch\go\bin;C:\Users\huch\AppData\Roaming\npm;C:\Windows\System32;C:\Windows;C:\Windows\System32\WindowsPowerShell\v1.0;C:\Program Files\nodejs"
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
        $commit = (& git rev-parse --short HEAD) 2>$null
        if ([string]::IsNullOrEmpty($commit)) { $commit = "dev" }
        $version = "2.0.0"
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
        $commit = (& git rev-parse --short HEAD) 2>$null
        if ([string]::IsNullOrEmpty($commit)) { $commit = "unknown" }
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
