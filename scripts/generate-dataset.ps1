param(
  [int]$Count = 200,
  [double]$ErrorRate = 0.2,
  [string]$Profile = "small-hospital",
  [double]$LowWeight = 0.6,
  [double]$MediumWeight = 0.3,
  [double]$HighWeight = 0.1,
  [string]$Types = "A01,A03",
  [string]$Output = ".local/datasets/hl7v2",
  [long]$Seed = 0
)

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

go run .\cmd\datasetgen\main.go `
  -count $Count `
  -error-rate $ErrorRate `
  -profile $Profile `
  -low-weight $LowWeight `
  -medium-weight $MediumWeight `
  -high-weight $HighWeight `
  -types $Types `
  -out $Output `
  -seed $Seed
