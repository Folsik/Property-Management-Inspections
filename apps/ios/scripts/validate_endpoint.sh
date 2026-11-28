#!/usr/bin/env bash
set -euo pipefail

# These are public, compile-time build settings. Never embed server credentials.
value="${INSPECTLY_API_BASE_URL:-}"
case "${CONFIGURATION:-}" in
  Debug)
    if [[ "$value" != "http://localhost:8080" ]]; then
      echo "error: Local Debug must point to simulator localhost:8080" >&2
      exit 1
    fi
    ;;
  Staging|Release)
    if [[ ! "$value" =~ ^https://([A-Za-z0-9-]+\.)+[A-Za-z][A-Za-z0-9-]*$ ]] ||
       [[ "$value" =~ (\.invalid|\.test|\.example|\.local)$ ]] ||
       [[ "$value" == "https://example.com" ]] ||
       [[ "$value" == "https://localhost" ]]; then
      echo "error: $CONFIGURATION requires an explicitly supplied HTTPS DNS origin without path, port or placeholder" >&2
      exit 1
    fi
    ;;
  *)
    echo "error: Unknown Inspectly configuration: ${CONFIGURATION:-unset}" >&2
    exit 1
    ;;
esac
