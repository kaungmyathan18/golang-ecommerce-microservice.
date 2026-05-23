# api-gateway

Generated HTTP service (`github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway`).

## Run locally

```bash
make tidy
make run
```

Live reload with [Air](https://github.com/air-verse/air) (install once: `go install github.com/air-verse/air/v2@latest`):

```bash
make dev
```

## Docker Compose

```bash
docker compose -f compose.yaml up --build
```

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`) runs on push and pull requests: `go mod verify`, `go vet`, `go test`, a local **Docker** build, and **Trivy** scans (filesystem and image). [Dependabot](.github/dependabot.yml) opens weekly PRs for Go modules and Actions.

## Features

- Database: none
- Cache: Redis
- Queue: none
- Observability: structured logs (zap)
