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
| Runtime | Go single binary — low memory, fast startup |
| Quality scoring | Deterministic per-message score built into the core pipeline |
| Deployment | Binary, Docker, or Kubernetes — no external runtime required |
| Configuration | Code-first via YAML — no GUI required |
| Compliance | HIPAA / GDPR audit trail included in the standard pipeline |

---

## Phase Roadmap

### Phase 1 — MVP (current)

> Status: in progress

- [x] HL7v2 ingestion stub (MLLP framing)
- [x] Segment-level parser
- [x] ADT→FHIR mapping skeleton
- [x] Deterministic quality scorer
- [x] Stdout routing adapter
- [x] Synthetic dataset generator
- [ ] MLLP network listener (TCP, VT/FS/CR framing)
- [ ] ADT A01 / A03 / A08 full field mapping
- [ ] Unit tests with real HL7v2 fixtures

**Success criteria:**
- Parse and map ADT A01/A03 end-to-end
- Produce a deterministic quality score per message
- Emit auditable processing logs for every pipeline stage

### Phase 2 — Real Transport & Routing

- Real MLLP TCP listener with concurrent channel handling
- HTTP destination adapter (POST FHIR Bundle to endpoint)
- Channel configuration via YAML (source, filter, transformer, destination)
- Message acknowledgement (ACK / NACK) per HL7v2 spec
- Retry logic and dead-letter queue for failed deliveries
- Structured JSON logging (one log line per pipeline stage)

### Phase 3 — Multi-Message Type Support

- ORM^O01 (order messages)
- ORU^R01 (lab results / observations)
- SIU^S12 (scheduling)
- MDM^T02 (clinical documents)
- Pluggable mapping rules per message type

### Phase 4 — Quality & Observability

- Expanded quality dimensions: completeness, conformity, semantic confidence
- Per-channel metrics (Prometheus-compatible)
- Processing history with searchable message log
- Alerting hooks for low-quality or failed messages
- Integration with OpenTelemetry for distributed tracing

### Phase 5 — Security & Compliance

- TLS on all transport layers (MLLP over TLS, HTTPS)
- Authentication: API key and OAuth 2.0 / SMART on FHIR
- Role-based access control (RBAC) for channel administration
- Audit log compliant with HIPAA §164.312 and GDPR Article 30
- Vulnerability management aligned with OWASP Top 10

### Phase 6 — Deployment & Operations

- Official Docker image
- Helm chart for Kubernetes
- Health check endpoints (`/healthz`, `/readyz`)
- Graceful shutdown with in-flight message drain
- Configuration hot-reload without downtime

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
