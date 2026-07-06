param(
    [ValidateSet('frontend', 'go', 'wails', 'dev', 'help')]
    [string]$Command = 'help'
)

$ErrorActionPreference = "Stop"
$env:PATH = "G:\stock\Go\bin;C:\Users\huch\go\bin;C:\Users\huch\AppData\Roaming\npm;C:\Windows\System32;C:\Windows;C:\Windows\System32\WindowsPowerShell\v1.0;C:\Program Files\nodejs"
$Root = "G:\stock\blades-stock"
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
        go build -ldflags "-s -w -H=windowsgui" -o build/bin/blades-stock.exe .
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
        Write-Host "Go build OK: build/bin/blades-stock.exe" -ForegroundColor Green
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
        $env:GOTOOLCHAIN = "go1.26.4"
        wails build -skipbindings -skipembedcreate
        if ($LASTEXITCODE -ne 0) { throw "wails build failed" }
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
