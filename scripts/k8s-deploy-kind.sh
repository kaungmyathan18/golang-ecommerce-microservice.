#!/usr/bin/env bash
# Deploy to a local kind cluster: build images, load into kind, apply manifests.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CLUSTER_NAME="${KIND_CLUSTER_NAME:-ecommerce}"
OVERLAY="${K8S_OVERLAY:-local}"

if ! command -v kind >/dev/null 2>&1; then
  echo "kind is not installed. See https://kind.sigs.k8s.io/"
  exit 1
fi
if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is not installed."
  exit 1
fi

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  echo "Creating kind cluster '$CLUSTER_NAME'..."
  kind create cluster --name "$CLUSTER_NAME" --config k8s/kind-config.yaml
fi

echo "Building images..."
./scripts/k8s-build-images.sh

echo "Loading images into kind..."
REGISTRY="${IMAGE_REGISTRY:-ecommerce}"
TAG="${IMAGE_TAG:-local}"
for img in api-gateway auth-service inventory-service inventory-worker \
  product-service payment-service order-service order-worker notification-service; do
  kind load docker-image "${REGISTRY}/${img}:${TAG}" --name "$CLUSTER_NAME"
done

echo "Applying k8s overlay: $OVERLAY"
kubectl apply -k "k8s/overlays/${OVERLAY}"

echo ""
echo "Waiting for rollouts (this may take a few minutes)..."
kubectl -n ecommerce rollout status deployment/mongo --timeout=120s
kubectl -n ecommerce rollout status deployment/mysql-order --timeout=180s
kubectl -n ecommerce rollout status deployment/redis --timeout=60s
kubectl -n ecommerce rollout status deployment/rabbitmq --timeout=120s
kubectl -n ecommerce rollout status deployment/api-gateway --timeout=180s

echo ""
echo "=== Deployed ==="
echo "Gateway (NodePort): http://localhost:30080"
echo "Or port-forward:    kubectl -n ecommerce port-forward svc/api-gateway 8080:8080"
echo ""
echo "Check pods:  kubectl -n ecommerce get pods"
echo "Run E2E:     GATEWAY_URL=http://localhost:30080 make e2e"
