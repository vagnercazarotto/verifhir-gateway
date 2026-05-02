# VeriFHIR Gateway — Phase B Changelog & How-To Guide

This document covers every change introduced in Phase B of the gateway, explains
the design rationale, and provides step-by-step instructions for running,
testing, and using each new feature.

---

## What Changed

### 1. Multiple MLLP Sources (Phase A — completed before Phase B)

The gateway can now listen on **multiple MLLP TCP ports simultaneously**, each
identified by a unique `source_id` that is stamped onto every message it
receives. Sources are configurable via YAML and via the REST API at runtime.

### 2. Pipeline Model (Phase B)

A **Pipeline** wires one ingest source to one or more delivery channels through
a set of message-level filters. Where previously the router would fan out to
every eligible channel on every message, it now only delivers to channels that
are explicitly named in a matching pipeline.

### 3. HTTP Ingest Endpoint (Phase B)

A new `POST /api/v1/ingest` endpoint accepts raw HL7v2 messages over plain HTTP,
running them through the same parse → map → score → route pipeline used by the
MLLP listener. This allows any HTTP client to submit messages without requiring
MLLP framing or a persistent TCP connection.

### 4. Management UI Pages

Three new pages were added to the React frontend:

| Page | Route | Purpose |
|---|---|---|
| Sources | `/sources` | Manage MLLP source listeners |
| Pipelines | `/pipelines` | Manage routing pipelines |
| Channels | `/channels` | Manage delivery channel destinations |

---

## Architecture Overview

```
Ingest Layer
  ┌─────────────────────────────────┐
  │  MLLP Source (port 2575, ...)   │ ← one goroutine per source
  │  HTTP Ingest (POST /api/v1/ingest)│ ← any HTTP client
  └────────────────┬────────────────┘
                   │ model.HL7Message {ID, SourceID, Payload}
                   ▼
Processing Pipeline (parse → map → score)
                   │ model.RoutedPayload {Resource, Quality, SourceID}
                   ▼
Router
  ┌─────────────────────────────────────────────────────┐
  │  If pipelines registered:                           │
  │    For each Pipeline (enabled):                     │
  │      Match source_id?  (empty = any)                │
  │      Match event_type? (empty = any)                │
  │      Score ≥ min_score?                             │
  │      → deliver to each destination_id               │
  │                                                     │
  │  If no pipelines registered (legacy fallback):      │
  │    Fan-out to every enabled Channel in registry     │
  └─────────────────────────────────────────────────────┘
                   │
                   ▼
Delivery Channels (HTTP/FHIR, MLLP passthrough)
```

---

## New REST API Endpoints

### Sources

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/sources` | List all sources |
| `POST` | `/api/v1/sources` | Create a source |
| `GET` | `/api/v1/sources/{id}` | Get a source by ID |
| `PUT` | `/api/v1/sources/{id}` | Update a source |
| `DELETE` | `/api/v1/sources/{id}` | Delete a source |

**Source JSON shape:**

```json
{
  "id": "ward-adt",
  "name": "Ward ADT Listener",
  "type": "mllp",
  "addr": "0.0.0.0:2575",
  "enabled": true,
  "created_at": "2026-05-02T10:00:00Z",
  "updated_at": "2026-05-02T10:00:00Z"
}
```

### Pipelines

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/pipelines` | List all pipelines |
| `POST` | `/api/v1/pipelines` | Create a pipeline |
| `GET` | `/api/v1/pipelines/{id}` | Get a pipeline by ID |
| `PUT` | `/api/v1/pipelines/{id}` | Update a pipeline |
| `DELETE` | `/api/v1/pipelines/{id}` | Delete a pipeline |

**Pipeline JSON shape:**

```json
{
  "id": "adt-to-fhir",
  "name": "ADT → FHIR Server",
  "source_id": "ward-adt",
  "filters": {
    "event_types": ["ADT^A01", "ADT^A03", "ADT^A08"],
    "min_score": 0.7
  },
  "destination_ids": ["fhir-server-1"],
  "enabled": true,
  "created_at": "2026-05-02T10:00:00Z",
  "updated_at": "2026-05-02T10:00:00Z"
}
```

**Filter fields:**

| Field | Type | Default | Description |
|---|---|---|---|
| `source_id` | string | `""` | Restrict to messages from this source. Empty = any source. |
| `filters.event_types` | `[]string` | `[]` | Restrict to these HL7 event types (e.g. `"ADT^A01"`). Empty = all. |
| `filters.min_score` | float64 | `0` | Discard messages scoring below this threshold. |
| `destination_ids` | `[]string` | `[]` | Channel IDs to deliver matching messages to. |

### HTTP Ingest

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/ingest` | Submit a raw HL7v2 message |

**Request:** raw HL7v2 text in the body. No MLLP framing needed. Max 1 MiB.

**Response (202 Accepted):**

```json
{
  "id": "3a7f2c1b9e4d0f8a",
  "status": "accepted"
}
```

**Error responses:**

| Status | Reason |
|---|---|
| `400` | Empty body or read failure |
| `422` | Parse or processing error |
| `501` | Ingest not wired (configuration issue) |

---

## Step-by-Step: Running the Gateway

### Prerequisites

- Go 1.22+
- Docker + Docker Compose (for the full stack including the UI)
- Port `8080` available for the REST API
- Port `2575` available for the default MLLP listener

### 1. Build and verify

```bash
# From the repository root
go build ./...
go test ./... -count=1
```

Expected: all 19 packages pass, zero failures.

### 2. Run locally (binary only, no UI)

```bash
go run ./cmd/gateway
```

The gateway starts with:
- MLLP listener on `:2575` (default source `"default"`)
- REST API on `:8080`
- SQLite DB at `.local/gateway.db`

### 3. Run with Docker Compose (full stack: gateway + UI + nginx)

```bash
docker compose up --build -d
```

- UI available at [http://localhost:3000](http://localhost:3000)
- REST API available at [http://localhost:8080](http://localhost:8080)

### 4. Send a test message via MLLP

```bash
go run ./scripts/mllp_send -dir .local/datasets/demo -addr localhost:2575
```

Or generate a fresh dataset first:

```bash
go run ./cmd/datasetgen -count 20 -out .local/datasets/demo
go run ./scripts/mllp_send -dir .local/datasets/demo -addr localhost:2575
```

### 5. Send a test message via HTTP Ingest

```bash
curl -s -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: text/plain" \
  --data-raw "MSH|^~\&|SENDING|FACILITY|RECV|DEST|20260502120000||ADT^A01|MSG001|P|2.5|||AL|NE
EVN|A01|20260502120000
PID|1||MRN001^^^FAC^MR||Doe^John^A||19800101|M|||123 Main St^^Springfield^IL^62701|||||||123-45-6789"
```

Expected response:
```json
{"id":"<hex-id>","status":"accepted"}
```

---

## Step-by-Step: Configuring Pipelines via the API

The following example wires the full path: Source → Pipeline → Channel.

### Step 1 — Create a Channel (delivery destination)

```bash
curl -s -X POST http://localhost:8080/api/v1/channels \
  -H "Content-Type: application/json" \
  -d '{
    "id": "fhir-server-1",
    "name": "Main FHIR Server",
    "output_type": "fhir",
    "url": "http://hapi-fhir:8080/fhir",
    "timeout_ms": 10000,
    "min_quality_score": 0,
    "enabled": true,
    "retry": {"max_attempts": 3, "initial_backoff_ms": 500, "multiplier": 2}
  }'
```

### Step 2 — Create a Pipeline

```bash
curl -s -X POST http://localhost:8080/api/v1/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "id": "adt-to-fhir",
    "name": "ADT → FHIR Server",
    "source_id": "",
    "filters": {
      "event_types": ["ADT^A01", "ADT^A03", "ADT^A08"],
      "min_score": 0.6
    },
    "destination_ids": ["fhir-server-1"],
    "enabled": true
  }'
```

> **Note:** once at least one pipeline is registered, the router switches to
> pipeline-based dispatch. Messages that do not match any pipeline are not
> delivered (audit status: `no_channels`). To revert to legacy fan-out, delete
> all pipelines.

### Step 3 — Verify pipeline is active

```bash
curl -s http://localhost:8080/api/v1/pipelines | jq .
```

### Step 4 — Send a message and watch the audit log

```bash
# Send via HTTP ingest
curl -s -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: text/plain" \
  --data-binary @.local/datasets/demo/sample_001.hl7

# Check the message was stored
curl -s "http://localhost:8080/api/v1/messages?limit=5" | jq '.[] | {id, status, quality_score}'

# Check the audit trail
curl -s "http://localhost:8080/api/v1/audit?limit=10" | jq '.[] | {stage, status, channel_id}'
```

---

## Step-by-Step: Configuring Sources via the API

Sources are managed at runtime. Changes to the source registry do **not**
automatically start or stop MLLP listeners (that requires a restart); they are
stored for reference and for the pipeline `source_id` filter.

```bash
# Create a source record
curl -s -X POST http://localhost:8080/api/v1/sources \
  -H "Content-Type: application/json" \
  -d '{
    "id": "ward-adt",
    "name": "Ward ADT Listener",
    "type": "mllp",
    "addr": "0.0.0.0:2575",
    "enabled": true
  }'

# List sources
curl -s http://localhost:8080/api/v1/sources | jq .

# Disable a source
curl -s -X PUT http://localhost:8080/api/v1/sources/ward-adt \
  -H "Content-Type: application/json" \
  -d '{"id":"ward-adt","name":"Ward ADT Listener","type":"mllp","addr":"0.0.0.0:2575","enabled":false}'
```

---

## Pipeline Routing Logic — Decision Table

| Pipeline `source_id` | Payload `SourceID` | Match? |
|---|---|---|
| `""` (empty) | anything | ✅ yes — accepts any source |
| `"ward-adt"` | `"ward-adt"` | ✅ yes |
| `"ward-adt"` | `"icu-monitor"` | ❌ no |

| Pipeline `event_types` | Message event type | Match? |
|---|---|---|
| `[]` (empty) | anything | ✅ yes — accepts all event types |
| `["ADT^A01"]` | `"ADT^A01"` | ✅ yes |
| `["ADT^A01"]` | `"ORU^R01"` | ❌ no |

| Pipeline `min_score` | Message quality score | Match? |
|---|---|---|
| `0` | anything | ✅ yes |
| `0.7` | `0.85` | ✅ yes |
| `0.7` | `0.65` | ❌ no — message discarded |

---

## Running the Test Suite

```bash
# All packages
go test ./... -count=1

# Router tests only (includes pipeline routing tests)
go test ./internal/router/... -count=1 -v

# REST API tests only
go test ./internal/api/rest/... -count=1 -v
```

Pipeline routing test coverage added in this phase:

| Test | What it verifies |
|---|---|
| `TestPipelineRoutingDeliversToDestination` | Basic delivery via a pipeline |
| `TestPipelineRoutingSourceIDFilterExcludes` | `source_id` mismatch skips pipeline |
| `TestPipelineRoutingEventTypeFilterExcludes` | Event type filter — exclude and include |
| `TestPipelineRoutingMinScoreFilter` | Quality score threshold — below and above |
| `TestPipelineRoutingMissingDestinationChannelSkips` | Unknown `destination_id` → `skipped` |
| `TestPipelineRoutingDisabledPipelineSkipped` | Disabled pipelines are ignored |
| `TestPipelineRoutingFanOutMultiplePipelines` | Two matching pipelines → two deliveries |

---

## Formatting Check

The project enforces `gofmt` on all Go source files. To verify:

```bash
files=$(find . -name '*.go' -not -path './.local/*' -not -path './web/*')
unformatted=$(gofmt -l $files)
[ -z "$unformatted" ] && echo "All files formatted" || echo "Unformatted: $unformatted"
```

To auto-fix:

```bash
gofmt -w $(find . -name '*.go' -not -path './.local/*' -not -path './web/*')
```

---

## Frontend Build

```bash
cd web
npm install       # first time only
npm run build     # production build → web/dist/
npm run dev       # local dev server (Vite, proxies /api to :8080)
```

---

## Files Changed in Phase B

### Backend

| File | Change |
|---|---|
| `internal/channel/channel.go` | Added `PipelineFilters`, `Pipeline`, `PipelineRegistry` (thread-safe CRUD) |
| `internal/router/router.go` | Added `pipelines` field, `WithPipelines()`, `routeViaPipelines()` |
| `internal/router/router_test.go` | 7 new pipeline routing tests |
| `internal/api/rest/server.go` | Added `pipelines` + `ingestFn` fields; `WithIngestFn()`; 5 pipeline routes; `POST /api/v1/ingest` handler |
| `cmd/gateway/main.go` | `NewPipelineRegistry()`, `.WithPipelines()`, `.WithIngestFn()` wired |

### Frontend

| File | Change |
|---|---|
| `web/src/types/index.ts` | `SourceType`, `Source`, `PipelineFilters`, `Pipeline`, `IngestResponse` |
| `web/src/api/client.ts` | `api.sources.*`, `api.pipelines.*`, `api.ingest.send()` |
| `web/src/App.tsx` | Routes for `/sources` and `/pipelines` |
| `web/src/components/Sidebar.tsx` | `IconSources`, `IconPipelines`, nav links for Sources and Pipelines |
| `web/src/pages/Sources.tsx` | New page — full CRUD for MLLP sources |
| `web/src/pages/Pipelines.tsx` | New page — full CRUD for pipelines |
