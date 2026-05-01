# Dataset Generator (Local Only)

## Purpose

Provide reproducible HL7v2 synthetic files for development and validation without storing synthetic data in the repository.

## Command

Git Bash (default):

./scripts/generate-dataset.sh --count 200 --error-rate 0.20 --types A01,A03 --out .local/datasets/hl7v2

Direct Go command:

go run .\\cmd\\datasetgen\\main.go -count 200 -error-rate 0.20 -types A01,A03 -out .local/datasets/hl7v2

Advanced example:

./scripts/generate-dataset.sh --count 500 --error-rate 0.35 --profile emergency-dept --types A01,A08 --low-weight 0.4 --medium-weight 0.3 --high-weight 0.3 --out .local/datasets/hl7v2-ed

PowerShell alternative:

.\\scripts\\generate-dataset.ps1 -Count 500 -ErrorRate 0.35 -Profile emergency-dept -Types "A01,A08" -LowWeight 0.4 -MediumWeight 0.3 -HighWeight 0.3 -Output ".local/datasets/hl7v2-ed"

## Parameters

- count: number of files to generate
- error-rate: fraction of files with injected faults
- profile: scenario profile (`small-hospital`, `large-network`, `emergency-dept`)
- types: comma-separated ADT event types (empty uses profile defaults)
- low-weight: relative frequency for low severity injected errors
- medium-weight: relative frequency for medium severity injected errors
- high-weight: relative frequency for high severity injected errors
- out: output directory
- seed: optional random seed for reproducibility
- prefix: optional file name prefix

## Output

- .hl7 files
- manifest.csv
- summary.json

Manifest columns:

- file
- message_type
- profile
- valid
- error_type
- severity
- description

Severity levels:

- low: recoverable data quality issues
- medium: parser-impacting format/data defects
- high: major structural issues

## Compliance Notes

- Generated data is synthetic
- Output directory is ignored by git via .gitignore
- Do not commit generated .local dataset artifacts
