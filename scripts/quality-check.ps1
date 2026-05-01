$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Write-Host "[quality] gofmt check"
$files = Get-ChildItem -Recurse -Filter *.go | Where-Object { $_.FullName -notlike "*.local*" }
$notFormatted = @()
foreach ($f in $files) {
  $result = gofmt -l $f.FullName
  if ($result) { $notFormatted += $result }
}
if ($notFormatted.Count -gt 0) {
  Write-Host "Files not formatted:" -ForegroundColor Red
  $notFormatted | ForEach-Object { Write-Host $_ -ForegroundColor Red }
  exit 1
}

Write-Host "[quality] go vet"
go vet ./...

Write-Host "[quality] go test"
go test ./... -count=1

Write-Host "[quality] go build"
go build ./...

Write-Host "[quality] all checks passed"
