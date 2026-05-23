#!/usr/bin/env bash
# Generate a test JWT for the API gateway (HS256, 24h expiry).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SECRET="${JWT_SECRET:-dev-secret-change-me}"
cd "$ROOT/scripts/genjwt"
go run . "$SECRET"
