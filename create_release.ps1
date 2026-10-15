param(
    [string]$TagName = 'v1.4.0',
    [string]$ReleaseName = "Release $TagName",
    [string]$NotesFile = "RELEASE_NOTES.md"
)

$ErrorActionPreference = "Stop"

$ghToken = if ($env:GH_TOKEN) { $env:GH_TOKEN } elseif ($env:GITHUB_TOKEN) { $env:GITHUB_TOKEN } else { $null }

if (-not $ghToken) {
    Write-Error "GitHub Token not found! Please set `$env:GH_TOKEN or `$env:GITHUB_TOKEN with 'repo' scope before running this script."
    exit 1
}

$notesPath = Join-Path $PSScriptRoot $NotesFile
if (-not (Test-Path $notesPath)) {
    Write-Error "Release notes file not found: $notesPath"
    exit 1
}

$body = [System.IO.File]::ReadAllText($notesPath, [System.Text.Encoding]::UTF8)

# Use .NET's JavaScriptSerializer for maximum backwards-compatibility with older PowerShell versions
Add-Type -AssemblyName System.Web.Extensions
$serializer = New-Object System.Web.Script.Serialization.JavaScriptSerializer
$payload = @{
    tag_name         = $TagName
    name             = $ReleaseName
    body             = $body
    draft            = $false
    prerelease       = $false
}
$json = $serializer.Serialize($payload)

$headers = @{
    'Authorization' = "token $ghToken"
    'Accept'        = 'application/vnd.github+json'
    'User-Agent'    = 'blades-stock-release-agent'
}

Write-Host "Creating GitHub Release $TagName on huch16/blades-stock..." -ForegroundColor Cyan

try {
    $response = Invoke-RestMethod -Uri 'https://api.github.com/repos/huch16/blades-stock/releases' `
        -Method Post `
        -Headers $headers `
        -Body $json `
        -ContentType 'application/json; charset=utf-8'

    Write-Host "Release created successfully: $($response.html_url)" -ForegroundColor Green
} catch {
    Write-Error "Failed to create GitHub release: $_"
    exit 1
}
