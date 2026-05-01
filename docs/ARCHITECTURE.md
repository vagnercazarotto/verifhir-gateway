# VeriFHIR Gateway — Architecture

This document describes the target architecture for the full project scope, maps it to the existing codebase, and defines the package contracts each phase must implement.

---

## 1. High-Level Topology

```
┌─────────────────────────────────────────────────────────────────────┐
│                         VeriFHIR Gateway                            │
│                                                                     │
│  ┌──────────────┐     ┌─────────────────────────────────────────┐  │
│  │   Transport  │     │              Channel Manager             │  │
│  │   Layer      │────▶│  (one goroutine per channel)             │  │
│  │  MLLP / HTTP │     └──────────────┬──────────────────────────┘  │
│  └──────────────┘                    │                             │
│                          ┌───────────▼──────────────┐             │
│                          │       Channel Pipeline    │             │
│                          │                          │             │
│                          │  [Source Connector]       │             │
│                          │        │                 │             │
│                          │  [Filter]                │             │
│                          │        │                 │             │
│                          │  [Transformer]            │             │
│                          │    Parse → Map → Score   │             │
│                          │        │                 │             │
│                          │  [Destination Connector]  │             │
│                          └──────────────────────────┘             │
│                                    │                               │
│              ┌─────────────────────┼──────────────┐               │
│              ▼                     ▼              ▼               │
│         [Audit Log]          [Metrics]       [Health]             │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Channel Pipeline (Core Abstraction)

Every message is processed through a **channel** — an independently configured, observable pipeline. A channel has exactly four stages:

```
[Source Connector] ──▶ [Filter] ──▶ [Transformer] ──▶ [Destination Connector]
        │                  │               │                    │
     emit audit         emit audit      emit audit          emit audit
```

### Stage Contracts (Go interfaces)

```go
// Source produces raw HL7v2 messages.
type Source interface {
    Receive(ctx context.Context) (<-chan model.HL7Message, error)
}

// Filter decides whether a message continues through the pipeline.
type Filter interface {
    Allow(msg model.HL7Message) bool
}

// Transformer parses, maps, and scores a raw message.
type Transformer interface {
    Transform(msg model.HL7Message) (model.RoutedPayload, error)
}

// Destination delivers the final payload.
type Destination interface {
    Send(ctx context.Context, payload model.RoutedPayload) error
}
```

Each stage receives and emits an `audit.Entry` so every transformation step is traceable independently of the destination.

---

## 3. Package Layout

### Current (Phase 1)

```
cmd/
  gateway/            ← entrypoint (exists)
  datasetgen/         ← dataset generator (exists)
internal/
  config/             ← YAML config loader (exists)
  ingest/             ← MLLP stub (exists)
  hl7v2/              ← parser, path resolver, serializer (exists)
  parser/             ← HL7v2 parse orchestration (exists)
  mapping/            ← HL7v2 → FHIR mapping (exists)
  quality/            ← quality scorer (exists)
  router/             ← stdout routing stub (exists)
  model/              ← shared domain types (exists)
```

### Target (all phases)

```
cmd/
  gateway/            ← entrypoint
  datasetgen/         ← dataset generator

internal/
  channel/            ← [Phase 2] channel orchestrator
    channel.go        ← Channel type, pipeline coordination
    config.go         ← ChannelConfig YAML struct
    registry.go       ← start/stop/list channels

  connector/          ← [Phase 2+] pluggable connectors
    source/
      mllp/           ← [Phase 2] TCP MLLP listener
      http/           ← [Phase 3] HTTP source
      file/           ← [dev/test] file-based replay
    destination/
      stdout/         ← [Phase 1] current stub → promoted to adapter
      http/           ← [Phase 2] POST FHIR Bundle to endpoint
      queue/          ← [Phase 4] message queue output

  filter/             ← [Phase 2] routing rules
    filter.go         ← Filter interface
    msgtype.go        ← filter by MSH-9 event type

  ingest/             ← MLLP ingestion
    mllp_stub.go      ← current stub (Phase 1)
    mllp/             ← [Phase 2] real TCP listener (VT/FS/CR framing)

  hl7v2/              ← parser, path resolver, serializer (exists)
  parser/             ← parse orchestration (exists)

  mapping/            ← transformation rules
    hl7_to_fhir.go    ← generic mapper (exists)
    adt/              ← [Phase 2] ADT A01/A03/A08 full field rules
    orm/              ← [Phase 3] ORM O01
    oru/              ← [Phase 3] ORU R01
    siu/              ← [Phase 3] SIU S12
    mdm/              ← [Phase 3] MDM T02

  quality/            ← scoring engine
    scorer.go         ← base scorer (exists)
    dimensions/       ← [Phase 4] completeness, conformity, confidence

  audit/              ← [Phase 2] structured audit log
    audit.go          ← AuditEntry type, emit per pipeline stage

  transport/          ← [Phase 2] transport layer
    mllp/             ← MLLP TCP server (VT 0x0B / FS 0x1C / CR 0x0D)
    http/             ← HTTP server for REST/FHIR ingestion

  metrics/            ← [Phase 4] Prometheus-compatible metrics
  health/             ← [Phase 6] /healthz and /readyz handlers

  config/             ← runtime config (exists)
  model/              ← shared domain types (exists)
  router/             ← routing stub (exists → replaced by connector/destination)
```

---

## 4. Data Flow

### Phase 1 (current — stub pipeline)

```
mllp_stub.ReceiveStub()
    │  model.HL7Message
    ▼
parser.Parse()
    │  *parser.ParsedHL7
    ▼
mapping.ToFHIR()
    │  model.FHIRResource
    ▼
quality.Score()
    │  model.QualityReport
    ▼
router.Route()        ← stdout
```

### Phase 2 (real transport + channel model)

```
transport/mllp.Server.Accept()   ← TCP connection, VT/FS/CR framing
    │  raw []byte
    ▼
channel.Manager.Dispatch()
    │  model.HL7Message  +  audit.Entry{stage="ingest"}
    ▼
filter.Allow()                   ← MSH-9 event type check
    │  bool  +  audit.Entry{stage="filter"}
    ▼
transformer.Transform()
    │  parser.Parse → mapping.ToFHIR → quality.Score
    │  model.RoutedPayload  +  audit.Entry{stage="transform"}
    ▼
connector/destination/http.Send()  ← POST FHIR Bundle
    │  HTTP 200 / error  +  audit.Entry{stage="deliver"}
    ▼
transport/mllp.ACK() / NACK()     ← acknowledgement back to sender
```

---

## 5. Configuration Model (YAML)

Channel configuration drives the runtime without code changes:

```yaml
# configs/config.example.yaml  (target structure, Phase 2)

gateway:
  http_port: 8080

channels:
  - id: adt-to-fhir
    enabled: true
    source:
      type: mllp
      address: "0.0.0.0:2575"
      tls: false                 # Phase 5: set to true + cert paths
    filter:
      message_types: [ADT^A01, ADT^A03, ADT^A08]
    transformer:
      mapping: adt               # selects internal/mapping/adt
      quality_threshold: 0.7     # reject payload below this score
    destination:
      type: http
      url: "http://fhir-server/fhir"
      retry_max: 3
      dead_letter: ".local/dead-letter"
```

---

## 6. Audit Log Schema

Every pipeline stage emits one structured JSON line:

```json
{
  "ts":        "2026-05-01T12:00:00.000Z",
  "channel":   "adt-to-fhir",
  "msg_id":    "msg-001",
  "stage":     "transform",
  "status":    "ok",
  "duration_ms": 4,
  "quality_score": 0.95,
  "warnings":  []
}
```

`stage` is one of: `ingest` | `filter` | `transform` | `deliver` | `ack`

`status` is one of: `ok` | `filtered` | `error` | `dead-lettered`

---

## 7. Metrics (Phase 4)

All metrics are Prometheus-compatible and scoped by `channel` label.

| Metric | Type | Description |
|---|---|---|
| `verifhir_messages_received_total` | Counter | Messages received per channel |
| `verifhir_messages_filtered_total` | Counter | Messages dropped by filter |
| `verifhir_messages_delivered_total` | Counter | Successfully delivered |
| `verifhir_messages_dead_lettered_total` | Counter | Failed after max retries |
| `verifhir_transform_duration_seconds` | Histogram | Parse+map+score latency |
| `verifhir_quality_score` | Histogram | Distribution of quality scores |
| `verifhir_active_channels` | Gauge | Channels currently running |

---

## 8. Security Architecture (Phase 5)

```
┌──────────────────────────────────────────────┐
│  Transport Layer                             │
│  MLLP: TCP + TLS (mutual TLS optional)       │
│  HTTP: HTTPS + API key or OAuth 2.0 token    │
└──────────────────────┬───────────────────────┘
                       │ authenticated, encrypted
┌──────────────────────▼───────────────────────┐
│  Channel Manager                             │
│  RBAC: only authorized channels can start    │
│  Config changes require authenticated admin  │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│  Audit Log                                   │
│  Immutable append-only structured log        │
│  Covers: ingest, transform, deliver, ack     │
│  HIPAA §164.312 / GDPR Article 30 compliant  │
└──────────────────────────────────────────────┘
```

---

## 9. Deployment Topology (Phase 6)

```
         ┌────────────────────────────────┐
         │   Kubernetes Cluster           │
         │                               │
         │  ┌─────────────────────────┐  │
         │  │  verifhir-gateway Pod   │  │
         │  │  (single Go binary)     │  │
         │  │  port 2575 ← MLLP       │  │
         │  │  port 8080 ← HTTP/FHIR  │  │
         │  │  port 9090 ← metrics    │  │
         │  └───────────┬─────────────┘  │
         │              │                │
         │  ┌───────────▼─────────────┐  │
         │  │  ConfigMap (YAML)       │  │  ← hot-reload via inotify
         │  └─────────────────────────┘  │
         │                               │
         │  ┌─────────────────────────┐  │
         │  │  Prometheus + Grafana   │  │  ← scrapes :9090/metrics
         │  └─────────────────────────┘  │
         └────────────────────────────────┘
```

Health endpoints:
- `GET /healthz` — process alive
- `GET /readyz` — all channels up and transport listening

---

## 10. Roadmap ↔ Architecture Mapping

| Phase | Key Deliverable | New Packages |
|---|---|---|
| 1 — MVP | Stub pipeline end-to-end | *(existing)* |
| 2 — Real Transport | MLLP TCP, ACK/NACK, HTTP dest, channel YAML | `transport/mllp`, `channel/`, `connector/`, `filter/`, `audit/` |
| 3 — Multi-Message | ORM, ORU, SIU, MDM mappers | `mapping/adt`, `mapping/orm`, `mapping/oru`, `mapping/siu`, `mapping/mdm` |
| 4 — Observability | Prometheus metrics, OTel tracing, quality dimensions | `metrics/`, `quality/dimensions/` |
| 5 — Security | TLS, OAuth 2.0, RBAC, audit compliance | `transport/http` (TLS), auth middleware |
| 6 — Operations | Docker, Helm, health endpoints, hot-reload | `health/`, Dockerfile, helm/ |
