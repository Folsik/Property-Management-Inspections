#!/usr/bin/env bash
set -euo pipefail
curl --fail --silent --show-error --retry 12 --retry-all-errors --retry-delay 5 --max-time 15 https://api.staging.groupmax.ru/healthz
echo
curl --fail --silent --show-error --location --max-time 20 https://app.staging.groupmax.ru/ >/dev/null
echo "HTTPS smoke tests passed"
