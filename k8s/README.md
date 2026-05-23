# Kubernetes Deployment

Kustomize manifests for the full e-commerce stack.

## Layout

```
k8s/
├── base/                    # All Deployments, Services, PVCs, ConfigMaps
│   ├── infrastructure/      # mongo, mysql, redis, rabbitmq
│   └── apps/                # microservices + workers + gateway
└── overlays/
    └── local/               # kind/minikube: NodePort + imagePullPolicy: Never
```

## Prerequisites

- Docker
- [kind](https://kind.sigs.k8s.io/) or any Kubernetes cluster
- kubectl
- kustomize (built into kubectl 1.14+)

## Quick start (kind)

```bash
# One command: create cluster, build images, deploy
./scripts/k8s-deploy-kind.sh

# Gateway available at http://localhost:30080
GATEWAY_URL=http://localhost:30080 make e2e
```

## Manual steps

```bash
# 1. Build images
./scripts/k8s-build-images.sh

# 2. Load into kind (skip if using a remote registry)
kind load docker-image ecommerce/api-gateway:local --name ecommerce
# ... repeat for each service, or use k8s-deploy-kind.sh

# 3. Deploy
kubectl apply -k k8s/overlays/local

# 4. Verify
kubectl -n ecommerce get pods
kubectl -n ecommerce port-forward svc/api-gateway 8080:8080
```

## Services

| Service | Internal port | Notes |
|---------|---------------|-------|
| api-gateway | 8080 | NodePort 30080 (local overlay) |
| auth-service | 8086 | SQLite PVC |
| product-service | 8081 / 9091 gRPC | MongoDB |
| order-service | 8082 | MySQL |
| inventory-service | 8084 / 9093 gRPC | MongoDB |
| payment-service | 8085 | SQLite PVC |
| notification-service | 8083 | SQLite PVC |
| order-worker | — | background |
| inventory-worker | — | background |

## Configuration

- **Secrets** (`k8s/base/secret.yaml`): JWT secret, MySQL credentials
- **ConfigMap** (`k8s/base/configmap.yaml`): service URLs, feature flags

For production, replace dev secrets and push images to a registry:

```bash
IMAGE_REGISTRY=ghcr.io/yourorg IMAGE_TAG=v1.0.0 ./scripts/k8s-build-images.sh
docker push ghcr.io/yourorg/api-gateway:v1.0.0
# Update k8s/base/kustomization.yaml images section
kubectl apply -k k8s/base
```

## Ingress (optional)

The local overlay includes an Ingress for `ecommerce.local` (requires nginx-ingress):

```bash
# /etc/hosts: 127.0.0.1 ecommerce.local
curl http://ecommerce.local/health/live
```

## Teardown

```bash
kubectl delete -k k8s/overlays/local
kind delete cluster --name ecommerce
```
