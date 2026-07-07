#!/usr/bin/env bash
set -euo pipefail

echo "== unit + scenario tests =="
go test ./... -v

if [ "${MODE:-}" = "integration" ]; then
  echo "== integration tests (real DOKU sandbox) =="
  : "${DOKU_CLIENT_ID:?DOKU_CLIENT_ID must be set for MODE=integration}"
  : "${DOKU_SECRET_KEY:?DOKU_SECRET_KEY must be set for MODE=integration}"
  : "${DOKU_PRIVATE_KEY_PATH:?DOKU_PRIVATE_KEY_PATH must be set for MODE=integration}"
  go test -tags=integration -run Integration -v .
else
  echo "(skipping integration tests — set MODE=integration to run them)"
fi
