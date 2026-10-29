param()
$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
Push-Location $project
try {
    $env:GOCACHE = Join-Path $project '.gocache'
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests fehlgeschlagen' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet fehlgeschlagen' }
    go build -trimpath -ldflags '-s -w -H windowsgui' -o mm.exe ./cmd/mm
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path 'mm.exe')) { throw 'Build fehlgeschlagen' }
    Write-Host "Fertig: $(Join-Path $project 'mm.exe')"
} finally {
    Pop-Location
}
