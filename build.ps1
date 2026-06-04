$ErrorActionPreference = "Stop"

$go = (Get-Command go -ErrorAction SilentlyContinue).Source
$gofmt = (Get-Command gofmt -ErrorAction SilentlyContinue).Source

if (-not $go -and (Test-Path "C:\Program Files\Go\bin\go.exe")) {
    $go = "C:\Program Files\Go\bin\go.exe"
}
if (-not $gofmt -and (Test-Path "C:\Program Files\Go\bin\gofmt.exe")) {
    $gofmt = "C:\Program Files\Go\bin\gofmt.exe"
}

if (-not $go) {
    throw "go was not found on PATH or at C:\Program Files\Go\bin\go.exe"
}
if (-not $gofmt) {
    throw "gofmt was not found on PATH or at C:\Program Files\Go\bin\gofmt.exe"
}

$goFiles = Get-ChildItem -Path . -Filter "*.go" -File | ForEach-Object { $_.Name }
if ($goFiles.Count -gt 0) {
    & $gofmt -w @goFiles
}

$testFiles = Get-ChildItem -Path . -Filter "*_test.go" -File
if ($testFiles.Count -gt 0) {
    & $go test ./...
} else {
    Write-Host "No tests found; skipping go test ./..."
}

New-Item -ItemType Directory -Force -Path "dist" | Out-Null

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Output = "dist\chupa-windows-amd64.exe" },
    @{ GOOS = "linux"; GOARCH = "amd64"; Output = "dist\chupa-linux-amd64" },
    @{ GOOS = "linux"; GOARCH = "arm64"; Output = "dist\chupa-linux-arm64" }
)

foreach ($target in $targets) {
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    & $go build -o $target.Output .
    Write-Host $target.Output
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
