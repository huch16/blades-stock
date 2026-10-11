$GH_TOKEN = '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'
$body = [IO.File]::ReadAllText('RELEASE_NOTES.md', [Text.Encoding]::UTF8)

# Use .NET's JavaScriptSerializer for JSON (available in older PowerShell)
Add-Type -AssemblyName System.Web.Extensions
$serializer = New-Object System.Web.Script.Serialization.JavaScriptSerializer
$json = $serializer.Serialize(@{ tag_name = 'v1.2.0'; name = 'v1.2.0'; body = $body })

[IO.File]::WriteAllText('C:\tmp\release.json', $json, [Text.Encoding]::UTF8)

$headers = @{
    'Authorization' = "token $GH_TOKEN"
    'Accept'        = 'application/vnd.github+json'
}

Invoke-RestMethod -Uri 'https://api.github.com/repos/huch16/blades-stock/releases' -Method Post -Headers $headers -Body $json -ContentType 'application/json'