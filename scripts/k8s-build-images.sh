#!/usr/bin/env bash
# Build all microservice Docker images for Kubernetes (tag: ecommerce/<name>:local).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TAG="${IMAGE_TAG:-local}"
REGISTRY="${IMAGE_REGISTRY:-ecommerce}"

build() {
  local name="$1"
  local dockerfile="$2"
  echo "==> Building ${REGISTRY}/${name}:${TAG}"
  docker build -t "${REGISTRY}/${name}:${TAG}" -f "$dockerfile" .
}

build api-gateway           api-gateway/Dockerfile
build auth-service          auth-service/services/api/Dockerfile
build inventory-service     inventory-service/services/api/Dockerfile
build inventory-worker      inventory-service/services/worker/Dockerfile
build product-service       product-service/services/api/Dockerfile
build payment-service       payment-service/services/api/Dockerfile
build order-service         order-service/services/api/Dockerfile
build order-worker          order-service/services/worker/Dockerfile
build notification-service  notification-service/services/worker/Dockerfile

echo ""
echo "Built ${REGISTRY}/*:${TAG}"
