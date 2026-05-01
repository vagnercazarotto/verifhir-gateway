#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[quality] gofmt check"
files=$(find . -name '*.go' -not -path './.local/*')
out=$(gofmt -l $files)
if [[ -n "$out" ]]; then
  echo "Files not formatted:" >&2
  echo "$out" >&2
  exit 1
fi

echo "[quality] go vet"
go vet ./...

echo "[quality] go test"
go test ./... -count=1

echo "[quality] go build"
go build ./...

echo "[quality] all checks passed"
