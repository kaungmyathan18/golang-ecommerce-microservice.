#!/usr/bin/env bash
# Load test for the e-commerce microservices stack via API gateway.
# Prerequisites: docker compose up, go
#
# Env:
#   GATEWAY_URL           default http://localhost:8080
#   LOAD_WORKERS          concurrent virtual users (default 10)
#   LOAD_DURATION_SEC     test duration (default 10)
#   LOAD_SCENARIO         health|products|orders|mixed|realistic (default realistic)
#   LOAD_THINK_MS         pause between steps in realistic mode (default 200)
#   LOAD_CATALOG_PRODUCTS products seeded before test (default 5)
#
# Examples:
#   make loadtest
#   LOAD_SCENARIO=products LOAD_WORKERS=20 make loadtest
#   LOAD_WORKERS=30 LOAD_DURATION_SEC=60 make loadtest
#   RATE_LIMIT_RPM=10000 docker compose up -d api-gateway && make loadtest
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/scripts/loadtest"

export GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
export LOAD_WORKERS="${LOAD_WORKERS:-10}"
export LOAD_DURATION_SEC="${LOAD_DURATION_SEC:-10}"
export LOAD_SCENARIO="${LOAD_SCENARIO:-realistic}"
export LOAD_THINK_MS="${LOAD_THINK_MS:-200}"
export LOAD_CATALOG_PRODUCTS="${LOAD_CATALOG_PRODUCTS:-5}"

echo "Gateway:   $GATEWAY_URL"
echo "Workers:   $LOAD_WORKERS"
echo "Duration:  ${LOAD_DURATION_SEC}s"
echo "Scenario:  $LOAD_SCENARIO"
if [ "$LOAD_SCENARIO" = "realistic" ]; then
  echo "Think:     ${LOAD_THINK_MS}ms"
  echo "Catalog:   $LOAD_CATALOG_PRODUCTS products"
fi
echo ""

go run . "$@"
