# VeriFHIR Gateway

|         |                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CI      | [![CI](https://github.com/vagnercazarotto/verifhir-gateway/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/vagnercazarotto/verifhir-gateway/actions/workflows/ci.yml) [![codecov](https://codecov.io/gh/vagnercazarotto/verifhir-gateway/branch/main/graph/badge.svg)](https://codecov.io/gh/vagnercazarotto/verifhir-gateway)                                                                        |
| Package | [![Go Version](https://img.shields.io/github/go-mod/go-version/vagnercazarotto/verifhir-gateway)](https://github.com/vagnercazarotto/verifhir-gateway/blob/main/go.mod) [![Go Reference](https://pkg.go.dev/badge/github.com/vagnercazarotto/verifhir-gateway.svg)](https://pkg.go.dev/github.com/vagnercazarotto/verifhir-gateway)                                                                                       |
| Meta    | [![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md) [![Domain](https://img.shields.io/badge/domain-HL7v2%20%E2%86%92%20FHIR-orange)](docs/PROJECT-SCOPE.md) [![Status](https://img.shields.io/badge/status-MVP-yellow)](#current-status)                                            |

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

### Option 1 — Docker (recommended, no source code required)

**Prerequisites:** Docker Desktop or Docker Engine with Compose v2.

```bash
# 1. Download the production compose file
curl -O https://raw.githubusercontent.com/vagnercazarotto/verifhir-gateway/main/docker-compose.prod.yml

# 2. Start (images are pulled automatically from GHCR)
docker compose -f docker-compose.prod.yml up -d
```

Open in your browser: **http://localhost:3000**

| Port | Service |
|---|---|
| 3000 | Web UI (React) |
| 8080 | REST API |
| 2575 | MLLP listener |

**Stop and remove:**

```bash
docker compose -f docker-compose.prod.yml down
```

---

### Option 2 — Local development (with source code)

**Prerequisites:** Go 1.25+, Node 22+, Docker Desktop.

```bash
git clone https://github.com/vagnercazarotto/verifhir-gateway.git
cd verifhir-gateway

# Start everything with a local build
docker compose up --build -d
```

Or run each service separately:

```bash
# Terminal 1 — Go Gateway
mkdir -p .local
go run ./cmd/gateway

# Terminal 2 — Web UI (dev server with hot-reload)
cd web && npm install && npm run dev
```

Web UI available at **http://localhost:5173** (automatic proxy to the API at :8080).

### Prerequisites (legacy scripts)

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

For the standards, terminologies, regulations, and tooling this gateway depends on, see [docs/STANDARDS.md](docs/STANDARDS.md).

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

## Quality Gates (GitHub + Local)

CI workflow:

- .github/workflows/ci.yml

Checks executed on push and pull requests to main:

- gofmt verification
- go vet
- go test ./... (with `-race` and coverage profile)
- go build ./...
- coverage upload to Codecov

Run locally before push (Git Bash default):

./scripts/quality-check.sh

PowerShell alternative:

.\scripts\quality-check.ps1

Recommended branch protection for main:

1. Require pull request before merge
2. Require status checks to pass
3. Select required check: CI / quality
4. Require branch to be up to date before merge

## VS Code Tasks

This repository includes preconfigured VS Code tasks in .vscode/tasks.json.

Open the command palette and run `Tasks: Run Task`, then choose:

- Quality Check (Git Bash)
- Go Test ./...
- Run Gateway (Git Bash)
- Generate Dataset (Git Bash)

## License

Apache 2.0
