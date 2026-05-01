#!/usr/bin/env bash
set -euo pipefail

# Run gateway locally with Git Bash defaults.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export GATEWAY_HTTP_PORT="${GATEWAY_HTTP_PORT:-8080}"

go run ./cmd/gateway/main.go
