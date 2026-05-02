# VeriFHIR Gateway

|         |                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CI      | [![CI](https://github.com/vagnercazarotto/verifhir-gateway/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/vagnercazarotto/verifhir-gateway/actions/workflows/ci.yml) [![codecov](https://codecov.io/gh/vagnercazarotto/verifhir-gateway/branch/main/graph/badge.svg)](https://codecov.io/gh/vagnercazarotto/verifhir-gateway)                                                                        |
| Package | [![Go Version](https://img.shields.io/github/go-mod/go-version/vagnercazarotto/verifhir-gateway)](https://github.com/vagnercazarotto/verifhir-gateway/blob/main/go.mod) [![Go Reference](https://pkg.go.dev/badge/github.com/vagnercazarotto/verifhir-gateway.svg)](https://pkg.go.dev/github.com/vagnercazarotto/verifhir-gateway)                                                                                       |
| Meta    | [![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md) [![Domain](https://img.shields.io/badge/domain-HL7v2%20%E2%86%92%20FHIR-orange)](docs/PROJECT-SCOPE.md) [![Status](https://img.shields.io/badge/status-active-brightgreen)](#current-status)                                    |

HL7v2 to FHIR gateway with quality scoring, pipeline-based routing, and a management UI.

## Why This Project

Healthcare integrations often stop at message translation, but production systems also need measurable output quality and fine-grained control over which messages reach which systems. VeriFHIR Gateway combines:

- **Multi-source HL7v2 ingestion** via MLLP TCP listeners or HTTP POST
- **HL7v2 parsing and FHIR mapping**
- **Quality scoring** — each message receives a confidence score
- **Pipeline-based routing** — filter by source, event type, and quality score before delivery
- **Web UI** — manage sources, channels, and pipelines without restarting the gateway
- **Audit trail** — every routing decision is recorded

This gives teams a practical path to migrate legacy clinical interfaces while keeping full visibility into transformation quality.

## Current Status

The gateway is fully operational with a complete processing pipeline:

1. Receive raw HL7v2 via MLLP listener or HTTP ingest endpoint
2. Parse segments and normalise fields
3. Map to FHIR resource model
4. Score message quality (completeness + conformity)
5. Route to destination channels through configurable pipelines
6. Record every decision in the audit log

## Quick Start

### Option 1 — Docker (recommended, no Go or Node required)

**Prerequisites:** Docker Desktop or Docker Engine with Compose v2.

```bash
# 1. Clone the repository
git clone https://github.com/vagnercazarotto/verifhir-gateway.git
cd verifhir-gateway

# 2. Build images and start all services
docker compose up --build -d
```

Open in your browser: **http://localhost:3000**

| Port | Service |
|---|---|
| 3000 | Web UI (React) |
| 8080 | REST API |
| 2575 | MLLP listener (default source) |

**Stop and remove:**

```bash
docker compose down
```

---

### Option 2 — Run without Docker

**Prerequisites:** Go 1.22+, Node 22+.

```bash
git clone https://github.com/vagnercazarotto/verifhir-gateway.git
cd verifhir-gateway

# Terminal 1 — Go Gateway (starts on :8080, MLLP on :2575)
mkdir -p .local
go run ./cmd/gateway

# Terminal 2 — Web UI dev server (hot-reload, proxies /api to :8080)
cd web && npm install && npm run dev
```

Web UI available at **http://localhost:5173**.

## Configuration

Environment variables:

| Variable | Default | Description |
|---|---|---|
| `GATEWAY_HTTP_PORT` | `8080` | REST API and UI proxy port |
| `DATABASE_URL` | _(SQLite)_ | PostgreSQL connection string, e.g. `postgres://user:pass@host/db`. Omit to use the embedded SQLite file at `.local/gateway.db`. |
| `MLLP_ADDR` | `0.0.0.0:2575` | Default MLLP listener address |
| `AUDIT_DIR` | `.local/audit` | Directory for audit log files |

See `configs/config.example.yaml` for a full annotated example.

## Sending HL7v2 Messages

### Via MLLP (TCP)

Any HL7v2 sender that supports MLLP framing can connect to port `2575`.

To test with the included dataset generator:

```bash
# Generate 20 synthetic ADT messages
go run ./cmd/datasetgen -count 20 -out .local/datasets/demo

# Send them to the MLLP listener
go run ./cmd/mllpsend -dir .local/datasets/demo -addr localhost:2575
```

### Via HTTP (no MLLP client required)

```bash
curl -s -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: text/plain" \
  --data-raw "MSH|^~\&|SENDER|FAC|RECV|DEST|20260502120000||ADT^A01|MSG001|P|2.5
EVN|A01|20260502120000
PID|1||MRN001^^^FAC^MR||Doe^John||19800101|M"
```

Response:
```json
{"id": "3a7f2c1b9e4d0f8a", "status": "accepted"}
```

## Managing Sources, Channels, and Pipelines

### Web UI

Open **http://localhost:3000** and use the sidebar to navigate to:

- **Sources** — register MLLP listener addresses and assign each a unique ID
- **Channels** — configure delivery destinations (FHIR servers, MLLP passthrough)
- **Pipelines** — wire a source to one or more channels with optional filters

### REST API

All resources follow the same pattern: `GET`, `POST`, `GET /{id}`, `PUT /{id}`, `DELETE /{id}`.

**Create a source:**
```bash
curl -s -X POST http://localhost:8080/api/v1/sources \
  -H "Content-Type: application/json" \
  -d '{
    "id": "ward-adt",
    "name": "Ward ADT Listener",
    "type": "mllp",
    "addr": "0.0.0.0:2575",
    "enabled": true
  }'
```

**Create a delivery channel:**
```bash
curl -s -X POST http://localhost:8080/api/v1/channels \
  -H "Content-Type: application/json" \
  -d '{
    "id": "fhir-server-1",
    "name": "HAPI FHIR Server",
    "output_type": "fhir",
    "url": "http://hapi-fhir:8080/fhir",
    "min_quality_score": 0.6,
    "enabled": true
  }'
```

**Create a pipeline:**
```bash
curl -s -X POST http://localhost:8080/api/v1/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "id": "adt-to-fhir",
    "name": "ADT to FHIR",
    "source_id": "ward-adt",
    "filters": {
      "event_types": ["ADT^A01", "ADT^A03", "ADT^A08"],
      "min_score": 0.6
    },
    "destination_ids": ["fhir-server-1"],
    "enabled": true
  }'
```

### Pipeline routing

Once at least one pipeline is registered the router switches to pipeline mode. Each incoming message is tested against every enabled pipeline in order:

| Filter | Empty / zero value | Non-empty value |
|---|---|---|
| `source_id` | Matches any source | Matches only that source ID |
| `filters.event_types` | Matches all event types | Must match one of the listed types |
| `filters.min_score` | No minimum | Score must be ≥ this value |

Messages that do not match any pipeline are not delivered (recorded as `no_channels` in the audit log).  
To return to the default fan-out behaviour (every message to every enabled channel), delete all pipelines.

## Synthetic Dataset Generator

This project includes a local-only HL7v2 dataset generator so you can develop and validate without committing synthetic data to GitHub.

```bash
# Generate with defaults (100 messages, small-hospital profile)
go run ./cmd/datasetgen

# Generate with custom parameters
go run ./cmd/datasetgen \
  -count 500 \
  -error-rate 0.25 \
  -profile large-network \
  -types A01,A03,A08 \
  -out .local/datasets/custom
```

Output written to `.local/` (git-ignored):

| File | Description |
|---|---|
| `*.hl7` | One HL7v2 message per file |
| `manifest.csv` | File-level metadata (profile, severity) |
| `summary.json` | Run summary with seed, profile, and counts |

Available profiles: `small-hospital`, `large-network`, `emergency-dept`.

See [docs/DATASET-GENERATOR.md](docs/DATASET-GENERATOR.md) for all options and error injection details.

## Project Structure

```
cmd/
  gateway/        Application entrypoint
  datasetgen/     Synthetic HL7v2 dataset generator CLI
internal/
  api/rest/       HTTP REST API server (sources, channels, pipelines, ingest, audit)
  channel/        Source, Channel, and Pipeline registries (thread-safe in-memory)
  config/         Runtime configuration loading
  connector/      Delivery adapters (FHIR HTTP, MLLP passthrough)
  hl7v2/          Low-level HL7v2 parser and serializer
  ingest/         MLLP ingest adapter
  mapping/        HL7v2 → FHIR mapping rules
  model/          Shared domain types (HL7Message, RoutedPayload)
  parser/         HL7v2 parsing logic used by the ingest layer
  quality/        Quality scoring engine
  router/         Pipeline-based and legacy fan-out routing
web/              React + TypeScript + Tailwind management UI
configs/          Sample configuration files
docs/             Architecture, scope, standards, and demo guides
scripts/          Local development helper scripts
```

For standards, terminologies, and regulations see [docs/STANDARDS.md](docs/STANDARDS.md).  
For architecture diagrams see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Processing Pipeline

```
Ingest (MLLP / HTTP)
       │
       ▼
  HL7v2 Parser
       │
       ▼
  FHIR Mapper
       │
       ▼
 Quality Scorer  ──── score attached to payload
       │
       ▼
    Router
  ┌────┴────────────────────────────────────┐
  │  Pipeline mode (when pipelines exist):  │
  │    filter by source_id                  │
  │    filter by event_type                 │
  │    filter by min_score                  │
  │    deliver to destination_ids           │
  │                                         │
  │  Legacy mode (no pipelines):            │
  │    fan-out to all enabled channels      │
  └─────────────────────────────────────────┘
       │
       ▼
Delivery Channels (FHIR HTTP / MLLP passthrough)
       │
       ▼
  Audit Log
```

## Contributing

Contributions are welcome. Open an issue describing:

1. the use case or bug
2. expected behaviour
3. proposed approach

Before opening a pull request, make sure all checks pass locally:

```bash
# Go — format, vet, test
gofmt -l ./...           # should produce no output
go vet ./...
go test ./... -count=1

# Frontend — lint and build
cd web && npm run build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contribution guide.

## Quality Gates (CI)

Checks run on every push and pull request to `main`:

- `gofmt` verification
- `go vet`
- `go test ./...` with `-race` and coverage profile
- `go build ./...`
- Coverage upload to Codecov

CI workflow: [.github/workflows/ci.yml](.github/workflows/ci.yml)


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
