# Build script for blades-stock frontend
$ErrorActionPreference = "Stop"
$frontendDir = "G:\stock\blades-stock\frontend"

Write-Host "Starting Vite build..."
$env:NODE_ENV = "production"
$process = Start-Process -FilePath "cmd.exe" -ArgumentList "/c", "npm run build" -WorkingDirectory $frontendDir -NoNewWindow -Wait -PassThru

if ($process.ExitCode -ne 0) {
    Write-Host "Build failed with exit code: $($process.ExitCode)"
    exit 1
} else {
    Write-Host "Build completed successfully"
    exit 0
}
