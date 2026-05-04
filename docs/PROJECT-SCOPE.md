# VeriFHIR Gateway — Project Scope & Vision

## Goal

Build a production-ready HL7v2 to FHIR gateway with quality scoring for healthcare interoperability — fully open-source, code-first, and built for modern cloud-native deployments.

---

## Vision

VeriFHIR Gateway aims to be a lightweight, auditable, and extensible integration engine focused on the HL7v2 → FHIR R4 conversion pipeline. It adopts a channel-based architecture designed for modern, cloud-native Go deployments.

### Core Concepts (Channel Model)

Every message flowing through the gateway follows a **channel pipeline**:

```
[Source Connector] → [Filter] → [Transformer] → [Destination Connector]
```

- **Source Connector:** receives raw clinical messages (MLLP, HTTP, file, queue)
- **Filter:** applies routing rules to decide whether a message is processed
- **Transformer:** converts the message between formats (HL7v2 → FHIR R4)
- **Destination Connector:** delivers the transformed payload (HTTP/FHIR endpoint, queue, stdout)

Each channel is independently configurable, observable, and restartable.

### Design Principles

| Principle | Approach |
|---|---|
| License | Apache 2.0 — fully open, no enterprise tier |
| Runtime | Go binary + React UI — ships as a single `docker-compose up` |
| Quality scoring | Deterministic per-message score built into the core pipeline |
| Deployment | Docker Compose (default), binary (dev), Kubernetes (advanced) |
| Configuration | Web UI (forms) + YAML files — both are supported |
| Compliance | HIPAA / GDPR audit trail, PDF report export, searchable message history |
| Storage | SQLite (zero-config default) or PostgreSQL (external, for scale) |
| Log archival | Local filesystem (default) → S3-compatible bucket (optional expansion) |

---

## Phase Roadmap

### Phase 1 — Core Pipeline ✅ Complete

- [x] MLLP TCP listener (VT/FS/CR framing, concurrent, ACK/NACK)
- [x] Segment-level HL7v2 parser with path syntax
- [x] ADT A01 / A03 / A08 full field mapping → FHIR R4 Bundle
- [x] Quality scorer: completeness + conformity + confidence (weighted)
- [x] Per-stage JSON audit logging (5 lines per message)
- [x] Synthetic dataset generator with configurable error injection
- [x] Batch validation tool (`batchval`)
- [x] MLLP sender tool for live demo (`mllp_send`)

**Proven results (seed=42, 20 msgs, 40% error rate):**
- 20/20 messages processed without parse error
- Average quality score: 0.91 | Perfect (1.00): 5/20
- All injected errors captured as QualityFindings

---

### Phase 2 — Real Transport & Database

> **Goal:** the gateway delivers real FHIR Bundles and persists every event.

- [ ] HTTP destination adapter — POST FHIR Bundle to configurable endpoint
- [ ] Retry logic with exponential backoff (max N attempts)
- [ ] Dead-letter queue — failed deliveries written to `.local/dead-letter/`
- [ ] SQLite persistence layer — messages, audit entries, pipeline configs
- [ ] PostgreSQL adapter — drop-in swap via `DATABASE_URL` env variable
- [ ] REST API (Go) — CRUD for pipelines, message history, audit log queries
- [ ] Channel config via YAML + API (source → filter → transformer → destination)
- [ ] Health endpoints (`/healthz`, `/readyz`)

**Success criteria:**
- End-to-end delivery: EHR → MLLP → gateway → FHIR server
- Every message stored with full audit trail
- REST API queryable by ID, score, event type, date range

---

### Phase 3 — Web Interface (React + TypeScript)

> **Goal:** any operator can manage the gateway from a browser. Zero CLI required.

**Dashboard**
- Messages processed / hour (sparkline)
- Average quality score per channel (gauge)
- Findings breakdown by rule and field
- Dead-letter count with drill-down

**Pipeline Manager**
- Create / edit / delete channels via form
- Source: MLLP (addr, port), HTTP, file replay
- Filter: allowed event types, minimum score threshold
- Transformer: ADT, ORM, ORU (Phase 4)
- Destination: HTTP FHIR endpoint (URL, auth, timeout, retry)
- Enable / disable channel without restart

**Message History**
- Searchable table: ID, event type, score, status, timestamp
- Filter by channel, score range, date range
- Click any message → full detail view

**Audit Log Viewer**
- Per-message: 5 pipeline stages expandable
- Shows raw HL7 payload, mapped FHIR Bundle, quality findings
- Export single message audit as PDF

**Reports**
- Date-range report: total messages, avg score, findings per rule, errors
- Export as PDF or CSV
- Configurable report schedule (daily / weekly) — Phase 5

**Docker Compose deployment:**
```yaml
services:
  gateway:   # Go binary — MLLP :2575, REST API :8080
  web:       # React app served via nginx — :3000
  db:        # SQLite volume (default) or external PostgreSQL
```
`docker-compose up` → working system on first launch.

---

### Phase 4 — Multi-Message Type Support ✅ Complete

- [x] ORM^O01 (order messages) → FHIR R4 ServiceRequest
- [x] ORU^R01 (lab results / observations) → FHIR R4 DiagnosticReport + Observation
- [x] SIU^S12 (scheduling) → FHIR R4 Appointment
- [x] MDM^T02 (clinical documents) → FHIR R4 DocumentReference
- [x] Pluggable mapping rules per message type (dispatcher by MSH-9.1)
- [x] Extended quality rules per message type

**Also completed (outside original scope):**
- [x] HL7 passthrough output — channels can deliver raw HL7v2 via MLLP TCP instead of FHIR JSON
  - New `output_type: fhir | hl7_passthrough` field on Channel (YAML + REST API + UI)
  - MLLP client destination adapter with VT/FS/CR framing, ACK/NACK detection, timeout
  - Fanout already works: one message → FHIR channel + HL7 passthrough channel simultaneously
- [x] UI improvements: SVG icons, skeleton loading, auto-refresh Dashboard, Toast notifications,
  Channel confirm-delete modal, Message search, Output type badge in Channels table

---

### Phase 4.5 — Multi-Source / Multi-Pipeline (In Progress)

> **Goal:** a single gateway instance can listen on multiple MLLP ports and/or HTTP endpoints,
> each bound to its own set of destination channels. Messages from source A only go to channels
> configured for that source.

```
[MLLP :2575  (EHR A)] ──→ Pipeline 1 ──→ [FHIR channel A1] [HL7 channel A2]
[MLLP :2576  (EHR B)] ──→ Pipeline 2 ──→ [FHIR channel B1]
[HTTP /ingest (API )] ──→ Pipeline 3 ──→ [FHIR channel C1] [HL7 channel C2]
```

**Implementation roadmap:**

#### Phase A — Multiple MLLP Sources (current)
- [ ] `SourceConfig` struct: `{ id, type: mllp|http|file, addr, tls_cert, tls_key }`
- [ ] `sources:` list in channels YAML / REST API
- [ ] `main.go` launches one `mllp.Server` goroutine per enabled source
- [ ] `HL7Message.SourceID` carries which source received the message
- [ ] Router filters channels by `SourceID` when set (empty = accept all)

**Success criteria:** two simultaneous MLLP listeners on different ports, each delivering
to independent channel sets, verified by integration test.

#### Phase B — Pipeline Model
- [ ] `Pipeline` struct: `{ id, name, source_id, filters{event_types, min_score}, destination_ids[] }`
- [ ] Pipeline Registry (CRUD via REST API + YAML)
- [ ] Router dispatches per pipeline instead of global channel list
- [ ] UI: Pipeline Manager page (source + filters + destinations)

#### Phase C — HTTP Ingest Source
- [ ] `POST /api/ingest/hl7` — accepts raw HL7v2 text body, feeds into pipeline
- [ ] `POST /api/ingest/fhir` — accepts FHIR JSON, validates, routes to channels
- [ ] Source authentication via API key (Phase 5 prerequisite)

---

### Phase 5 — Security & Compliance

- TLS on MLLP and HTTPS on REST API / UI
- API key authentication for REST API
- OAuth 2.0 / SMART on FHIR for UI login
- Role-based access: admin, operator, read-only
- Audit log export compliant with HIPAA §164.312 and GDPR Article 30
- Scheduled PDF report delivery by email
- Log archival to S3-compatible storage (MinIO, AWS S3, GCS)

---

### Phase 5.5 — IG Baseline Validation & Deviation Classification

> **Goal:** Compare mapping output against the official HL7 v2-to-FHIR Implementation Guide,
> and classify deviations as conformant, local variation, or true non-conformance.
> (Prompted by community feedback from Hans Buitendijk, Oracle Health.)

**Context:** The current quality scorer checks completeness, conformity, and confidence — but it
does not compare results against the official IG (https://hl7.org/fhir/uv/v2mappings/), and it
cannot distinguish between a hospital's valid local interpretation of HL7v2 vs. a genuine
mapping error. In practice, almost no hospital follows HL7v2 strictly — local variations are
the norm. Treating every deviation as an error generates noise and reduces trust in the score.

**Deviation Classification Model:**

```
HL7v2 message → Mapper → FHIR output
                              │
                     Compare against IG baseline
                              │
                 ┌────────────┼────────────┐
                 │            │            │
            conformant   local_var   non_conformant
            (matches IG)  (known      (true error)
                          deviation,
                          accepted)
```

**Implementation:**

1. **IG Baseline Rules**
   - [ ] Define `IGRule` struct: source path, target path, required flag, expected code system
   - [ ] Populate baseline rules for supported message types (ADT, ORM, ORU, SIU, MDM)
   - [ ] Reference specific IG sections per rule (e.g., `v2-FHIR IG §PID[Patient]`)
   - [ ] New file: `internal/quality/ig_baseline.go`

2. **Mapping Profiles (per facility/vendor)**
   - [ ] Define `MappingProfile` struct: ID, name, list of accepted deviations
   - [ ] Each accepted deviation: field, rule type, reason
   - [ ] Load profiles from JSON files in `configs/profiles/`
   - [ ] Default profile = no accepted deviations (strict IG mode)
   - [ ] New file: `internal/quality/profile.go`

3. **Deviation Classifier**
   - [ ] Extend `QualityFinding` with deviation type: `conformant | local_variation | non_conformant`
   - [ ] Add IG reference string per finding
   - [ ] Classification logic: check IG → check profile → classify
   - [ ] Only `non_conformant` findings penalize the quality score
   - [ ] `local_variation` findings logged but do not reduce score
   - [ ] New file: `internal/quality/classifier.go`

4. **Scorer Integration**
   - [ ] Accept optional `MappingProfile` parameter in scoring pipeline
   - [ ] Adjust score calculation: skip deduction for accepted deviations
   - [ ] Include deviation breakdown in output (N conformant / N local / N errors)

5. **Output & Reporting**
   - [ ] Quality report includes deviation classification per finding
   - [ ] Filter message history by deviation type in REST API and UI
   - [ ] Profile management via REST API (CRUD) and UI

**Estimated effort:** ~10-12 days

**Success criteria:**
- Same message scored with default profile vs. hospital-specific profile produces different results
- Local variations (Z-segments, alternative code systems) do not penalize quality score
- True non-conformance (wrong resource type, lost required fields) still flagged as errors
- IG baseline covers all fields for ADT, ORM, ORU, SIU, MDM message types

---

### Phase 6 — Deployment & Operations

- Official Docker image on Docker Hub
- Helm chart for Kubernetes
- Prometheus metrics endpoint (`/metrics`)
- OpenTelemetry tracing
- Configuration hot-reload without downtime
- Alerting webhooks (Slack, PagerDuty) on score drop or dead-letter spike

---

## Out of Scope (all phases)

- Full terminology service (SNOMED CT, LOINC lookup) — external dependency
- Multi-tenant billing and subscription management
- Graphical channel designer (GUI) — config-as-code is the model
- HL7 v3 / CDA support — FHIR R4 is the target output

---

## Compliance References

- HIPAA Security Rule: 45 CFR Part 164
- GDPR: Regulation (EU) 2016/679, Articles 5, 25, 30, 32
- HL7v2 transport: MLLP spec — https://www.hl7.org/documentcenter/public/wg/inm/mllp_transport_specification.pdf
- FHIR R4: https://hl7.org/fhir/R4/
- OWASP Top 10: https://owasp.org/www-project-top-ten/
