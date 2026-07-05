# 清理 QClaw 环境变量干扰，构建前端
$env:PATH = "C:\Users\huch\AppData\Roaming\npm;C:\Windows\System32;C:\Windows;C:\Windows\System32\WindowsPowerShell\v1.0"
$env:Path = $env:Path -replace 'C:\\Users\\huch\\AppData\\Roaming\\QClaw;', '' -replace 'C:\\Users\\huch\\AppData\\Roaming\\QClaw\\bin;', ''
Remove-Item Env:\QCLAW_NPM_CLI_JS -ErrorAction SilentlyContinue
Remove-Item Env:\QCLAW_NPM_CLI_JS_DIR -ErrorAction SilentlyContinue
Write-Host "Node: $(node --version)"
Write-Host "NPM: $(npm --version)"

# 清理旧 wailsjs 并重新生成
$frontend = "G:\stock\blades-stock\frontend"
Remove-Item "$frontend\src\wailsjs" -Recurse -Force -ErrorAction SilentlyContinue

Push-Location $frontend
try {
    npm run build 2>&1
} finally {
    Pop-Location
}
