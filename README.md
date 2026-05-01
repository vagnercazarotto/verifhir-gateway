# VeriFHIR Gateway

HL7v2 to FHIR gateway with quality scoring for healthcare interoperability.

## Why This Project

Healthcare integrations often stop at message translation, but production systems also need measurable output quality. VeriFHIR Gateway combines:

- HL7v2 parsing
- HL7v2 to FHIR mapping
- quality scoring
- routing hooks for downstream delivery

This gives teams a practical path to migrate legacy clinical interfaces while tracking transformation quality.

## Current Status

This repository contains an initial skeleton with a working bootstrap pipeline:

1. Ingest stub HL7 message
2. Parse segments
3. Map to FHIR-like model
4. Score quality
5. Route payload (stdout stub)

## Quick Start

### Prerequisites

- Go 1.22+

### Run (Git Bash - default)

./scripts/run-local.sh

PowerShell alternative:

.\scripts\run-local.ps1

Expected output:

- gateway startup log
- router log with resource id and score
- successful pipeline completion message

## Configuration

Environment variables:

- GATEWAY_HTTP_PORT: gateway port (default: 8080)

Example config file:

- configs/config.example.yaml

## Synthetic Dataset Generator

This project includes a local-only HL7v2 dataset generator so you can develop and validate without committing synthetic data to GitHub.

Generate files with defaults (Git Bash):

./scripts/generate-dataset.sh

Generate with custom parameters (Git Bash):

./scripts/generate-dataset.sh --count 500 --error-rate 0.25 --profile large-network --types A01,A03,A08 --low-weight 0.50 --medium-weight 0.30 --high-weight 0.20 --out .local/datasets/hl7v2

PowerShell helper:

.\\scripts\\generate-dataset.ps1 -Count 500 -ErrorRate 0.25 -Profile large-network -LowWeight 0.5 -MediumWeight 0.3 -HighWeight 0.2 -Types "A01,A03,A08"

Output includes:

- .hl7 files (one message per file)
- manifest.csv (file-level metadata with profile and severity)
- summary.json (run summary, seed, profile, and severity counts)

Error injection examples:

- missing PID segment
- invalid MSH version
- missing event type in MSH-9
- wrong segment delimiter
- missing patient identifier

Scenario profiles:

- small-hospital
- large-network
- emergency-dept

Note: generated data is written under .local/ and ignored by git.

## Project Structure

- cmd/gateway: application entrypoint
- internal/config: runtime configuration loading
- internal/ingest: ingestion adapters (MLLP stub for now)
- internal/parser: HL7v2 parsing logic
- internal/mapping: HL7v2 to FHIR mapping rules
- internal/quality: scoring and warnings generation
- internal/router: output routing adapter
- pkg/model: shared domain types
- configs: sample configuration files
- docs: scope and architecture notes
- scripts: local development scripts

## Pipeline Overview

1. Ingest receives raw HL7v2 payload
2. Parser normalizes payload into segments
3. Mapper transforms parsed content into FHIR resource model
4. Quality module computes completeness/confidence score
5. Router sends final payload to destination adapter

## MVP Scope

Included in MVP:

- ADT-focused ingestion and parsing baseline
- deterministic quality scoring per message
- auditable processing flow across all stages

Out of scope for MVP:

- full terminology service integration
- multi-tenant billing and IAM
- UI dashboard

See docs/PROJECT-SCOPE.md for details.

## Roadmap (Short Term)

1. Replace ingest stub with MLLP listener
2. Add ADT A01/A03 mapping rules
3. Expand quality scoring dimensions (completeness, conformity, confidence)
4. Add test suite and sample HL7 fixtures
5. Add routing adapters (HTTP and queue)

## Contributing

Contributions are welcome. Open an issue describing:

1. use case or bug
2. expected behavior
3. proposed approach

## License

Apache 2.0
