# VeriFHIR Gateway — Phase 1 Demo & Validation

This guide reproduces the full Phase 1 validation: dataset generation, live
MLLP traffic, audit log capture, and batch quality scoring.

**Prerequisites:** Go 1.22+ installed (`go version`), port `2575` available.

---

## 1. Clone and build

```bash
git clone https://github.com/vagnercazarotto/verifhir-gateway.git
cd verifhir-gateway
go build ./...
```

---

## 2. Run the test suite

```bash
go test ./... -count=1
```

Expected: all packages pass, no failures.

To see per-package coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
# total: (statements) ~64%
```

Packages with tests and their coverage:

| Package | Coverage |
|---|---|
| `internal/audit` | 100% |
| `internal/hl7v2` | 93.6% |
| `internal/ingest/mllp` | ~88% |
| `internal/mapping/adt` | ~85% |
| `internal/quality` | ~95% |

---

## 3. Generate a synthetic dataset

### Parameters

| Flag | Default | Description |
|---|---|---|
| `-count` | `200` | Number of HL7v2 files to generate |
| `-error-rate` | `0.20` | Fraction of files with injected errors (0.0 – 1.0) |
| `-out` | `.local/datasets/hl7v2` | Output directory |
| `-prefix` | `sample` | File name prefix |
| `-seed` | `0` (random) | Fixed seed for reproducible runs |
| `-profile` | `small-hospital` | Scenario profile — see profiles below |
| `-types` | *(profile defaults)* | Comma-separated ADT event types, e.g. `A01,A03,A08` |
| `-low-weight` | `0.60` | Relative weight for **low** severity errors |
| `-medium-weight` | `0.30` | Relative weight for **medium** severity errors |
| `-high-weight` | `0.10` | Relative weight for **high** severity errors |

**Profiles:**

| Profile | Description | Default event types |
|---|---|---|
| `small-hospital` | Single-facility, low volume | A01, A03 |
| `large-network` | Multi-facility, high volume | A01, A03, A08 |
| `emergency-dept` | High error rate, A01-heavy | A01, A03 |

**Error types injected and their severity:**

| Error type | Severity | What it does |
|---|---|---|
| `missing_event` | low | Removes the event type from MSH-9 |
| `missing_patient_id` | low | Clears PID-3 (MRN) |
| `truncated_message` | high | Cuts the message after the EVN segment |
| `missing_pid` | high | Removes the entire PID segment |

### Example commands

Minimal demo (20 messages, 40% errors, reproducible):
```bash
go run ./cmd/datasetgen/ \
  -count=20 \
  -error-rate=0.40 \
  -out=.local/datasets/demo \
  -prefix=demo \
  -seed=42
```

Stress test (500 messages, mostly high-severity errors):
```bash
go run ./cmd/datasetgen/ \
  -count=500 \
  -error-rate=0.60 \
  -profile=large-network \
  -low-weight=0.1 \
  -medium-weight=0.2 \
  -high-weight=0.7 \
  -out=.local/datasets/stress \
  -prefix=stress
```

Emergency department scenario (A01-only, 30% errors):
```bash
go run ./cmd/datasetgen/ \
  -count=100 \
  -error-rate=0.30 \
  -profile=emergency-dept \
  -types=A01 \
  -out=.local/datasets/ed \
  -prefix=ed
```

Clean dataset (no errors — baseline for comparison):
```bash
go run ./cmd/datasetgen/ \
  -count=50 \
  -error-rate=0.0 \
  -out=.local/datasets/clean \
  -prefix=clean \
  -seed=1
```

Output (example for the minimal demo):

```
Generated 20 files in .local/datasets/demo
Valid: 11 | Invalid: 9
Profile: small-hospital
Severity counts: low=6 medium=0 high=3
Manifest: .local/datasets/demo/manifest.csv
Summary:  .local/datasets/demo/summary.json
Seed: 42
```

The `manifest.csv` lists every file with its injected error type and severity.

---

## 4. Live MLLP demo

### 4a. Start the gateway

```bash
go build -o /tmp/verifhir-gateway ./cmd/gateway/
/tmp/verifhir-gateway 2>.local/datasets/demo/audit.log &
```

The gateway listens on `0.0.0.0:2575` (MLLP) by default.
All audit log lines go to **stderr**, redirected here to `audit.log`.

Environment overrides:

| Variable | Default | Description |
|---|---|---|
| `GATEWAY_MLLP_ADDR` | `0.0.0.0:2575` | TCP bind address |
| `GATEWAY_HTTP_PORT` | `8080` | HTTP port (reserved, Phase 2) |

### 4b. Send the dataset via MLLP

```bash
go run ./scripts/mllp_send/ \
  -dir .local/datasets/demo \
  -addr 127.0.0.1:2575 \
  -delay 100ms
```

Expected output (20 messages, all `ACK AA`):

```
Sending 20 messages to 127.0.0.1:2575

[01] demo-00001-ADT_A03.hl7                    ACK AA
[02] demo-00002-ADT_A01.hl7                    ACK AA
...
[20] demo-00020-ADT_A03.hl7                    ACK AA

Done: 20 ACK  0 NACK  0 FAIL  (total 20)
```

> The gateway always responds `ACK AA` — it does not reject messages at the
> transport layer. Quality issues are recorded as findings in the audit log,
> not as NACKs, so the sending system can keep operating.

### 4c. Stop the gateway

```bash
kill %1   # or: kill <PID printed at startup>
```

---

## 5. Audit logs

The gateway emits **5 JSON lines per message** to stderr (one per pipeline stage):

```
ingest → parse → map → score → route
```

Example — a complete message (score 0.95, one finding):

```json
{"ts":"2026-05-02T10:03:03Z","msg_id":"8810da09feb60bda","stage":"ingest","duration_ms":0,"status":"ok"}
{"ts":"2026-05-02T10:03:03Z","msg_id":"8810da09feb60bda","stage":"parse","duration_ms":0,"status":"ok","segments":4}
{"ts":"2026-05-02T10:03:03Z","msg_id":"8810da09feb60bda","stage":"map","duration_ms":0,"status":"ok","resource_type":"Bundle","event_type":"ADT^A03"}
{"ts":"2026-05-02T10:03:03Z","msg_id":"8810da09feb60bda","stage":"score","duration_ms":0,"status":"ok","score":0.95,"completeness":0.9,"conformity":1,"confidence":1,"findings":1}
{"ts":"2026-05-02T10:03:03Z","msg_id":"8810da09feb60bda","stage":"route","duration_ms":0,"status":"ok"}
```

Example — a truncated message (score 0.68, four findings):

```json
{"ts":"2026-05-02T10:03:03Z","msg_id":"25f40ac8a6cba9c0","stage":"ingest","duration_ms":0,"status":"ok"}
{"ts":"2026-05-02T10:03:03Z","msg_id":"25f40ac8a6cba9c0","stage":"parse","duration_ms":0,"status":"ok","segments":2}
{"ts":"2026-05-02T10:03:03Z","msg_id":"25f40ac8a6cba9c0","stage":"map","duration_ms":0,"status":"ok","resource_type":"Bundle","event_type":"ADT^A01"}
{"ts":"2026-05-02T10:03:03Z","msg_id":"25f40ac8a6cba9c0","stage":"score","duration_ms":0,"status":"ok","score":0.68,"completeness":0.35,"conformity":1,"confidence":1,"findings":4}
{"ts":"2026-05-02T10:03:03Z","msg_id":"25f40ac8a6cba9c0","stage":"route","duration_ms":0,"status":"ok"}
```

The audit log is machine-readable. To extract all scores from a run:

```bash
grep '"stage":"score"' .local/datasets/demo/audit.log | \
  python3 -c "
import sys, json
lines = [json.loads(l) for l in sys.stdin]
for l in lines:
    print(f\"{l['msg_id'][:8]}  score={l['score']:.2f}  completeness={l['completeness']:.2f}  findings={l['findings']}\")
"
```

---

## 6. Batch quality report

Run the entire dataset through the pipeline without a live gateway:

```bash
go run ./cmd/batchval/ -dir .local/datasets/demo
```

Produces a JSON report on stdout with per-message scores, findings, and
aggregate statistics. The report is also saved to
`.local/datasets/demo/validation-report.json` when piped:

```bash
go run ./cmd/batchval/ -dir .local/datasets/demo \
  > .local/datasets/demo/validation-report.json
```

### Phase 1 results (seed=42, 20 messages, 40% error rate)

| Metric | Value |
|---|---|
| Total processed | 20 |
| Parse errors | 0 |
| Average score | **0.91** |
| Average completeness | **0.82** |
| Average conformity | **1.00** |
| Average confidence | **1.00** |
| Perfect score (1.00) | 5 messages |
| Total findings | 27 |

The scorer correctly detected all injected clinical data errors:
- `truncated_message` → score 0.68 (4 findings: PID.3, PID.5, PID.7, PV1.44)
- `missing_pid` → score 0.68 (same findings)
- `missing_patient_id` → score 0.68 (same findings)
- `missing_event` → score 1.00 (no clinical context to score — correct behaviour)

---

## 7. Quality scorer dimensions

| Dimension | Weight | Rules |
|---|---|---|
| Completeness | 50% | PID.3 MRN present, PID.5 name present, PID.7 birthDate present, PID.8 gender present, PV1.2 status present, PV1.44 period.start present |
| Conformity | 30% | gender ∈ {male,female,other,unknown}, birthDate format YYYY-MM-DD, PV1.2 ∈ {in-progress,finished,cancelled} |
| Confidence | 20% | birthDate not in the future |

`score = 0.50 × completeness + 0.30 × conformity + 0.20 × confidence`

Each violation produces a `QualityFinding{Field, Rule, Value, Impact}` in the
audit log, enabling field-level traceability for HIPAA/GDPR audits.
