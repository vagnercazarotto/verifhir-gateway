#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COUNT=200
ERROR_RATE=0.2
PROFILE="small-hospital"
LOW_WEIGHT=0.6
MEDIUM_WEIGHT=0.3
HIGH_WEIGHT=0.1
TYPES="A01,A03"
OUTPUT=".local/datasets/hl7v2"
SEED=0
PREFIX="sample"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --count)
      COUNT="$2"; shift 2 ;;
    --error-rate)
      ERROR_RATE="$2"; shift 2 ;;
    --profile)
      PROFILE="$2"; shift 2 ;;
    --low-weight)
      LOW_WEIGHT="$2"; shift 2 ;;
    --medium-weight)
      MEDIUM_WEIGHT="$2"; shift 2 ;;
    --high-weight)
      HIGH_WEIGHT="$2"; shift 2 ;;
    --types)
      TYPES="$2"; shift 2 ;;
    --out)
      OUTPUT="$2"; shift 2 ;;
    --seed)
      SEED="$2"; shift 2 ;;
    --prefix)
      PREFIX="$2"; shift 2 ;;
    -h|--help)
      cat <<'EOF'
Usage: ./scripts/generate-dataset.sh [options]

Options:
  --count <n>          Number of files (default: 200)
  --error-rate <f>     Error fraction 0.0-1.0 (default: 0.2)
  --profile <name>     small-hospital|large-network|emergency-dept
  --low-weight <f>     Low severity weight (default: 0.6)
  --medium-weight <f>  Medium severity weight (default: 0.3)
  --high-weight <f>    High severity weight (default: 0.1)
  --types <csv>        ADT event types csv (default: A01,A03)
  --out <path>         Output directory (default: .local/datasets/hl7v2)
  --seed <n>           RNG seed (0 = auto)
  --prefix <text>      Output filename prefix (default: sample)
EOF
      exit 0 ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1 ;;
  esac
done

go run ./cmd/datasetgen/main.go \
  -count "$COUNT" \
  -error-rate "$ERROR_RATE" \
  -profile "$PROFILE" \
  -low-weight "$LOW_WEIGHT" \
  -medium-weight "$MEDIUM_WEIGHT" \
  -high-weight "$HIGH_WEIGHT" \
  -types "$TYPES" \
  -out "$OUTPUT" \
  -seed "$SEED" \
  -prefix "$PREFIX"
