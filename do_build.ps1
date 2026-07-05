# 干净 PowerShell 进程，避免 QClaw 环境干扰
& {
    $env:PATH = "C:\Users\huch\AppData\Roaming\npm;C:\Windows\System32;C:\Windows;C:\Windows\System32\WindowsPowerShell\v1.0;C:\Program Files\nodejs"
    Remove-Item Env:\QCLAW_NPM_CLI_JS -ErrorAction SilentlyContinue
    Remove-Item Env:\QCLAW_NPM_CLI_JS_DIR -ErrorAction SilentlyContinue
    Write-Host "Node: $(node --version)"
    Write-Host "NPM: $(npm --version)"

    $frontend = "G:\stock\blades-stock\frontend"
    if (Test-Path "$frontend\src\wailsjs") {
        Remove-Item "$frontend\src\wailsjs" -Recurse -Force
    }

    Push-Location $frontend
    try {
        npm run build 2>&1
    } finally {
        Pop-Location
    }
}
