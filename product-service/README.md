# product-service

Microservices monorepo (`github.com/kaungmyathan18/golang-ecommerce-microservice/product-service`).

## Structure

| Path | Description |
|------|-------------|
| `internal/` | Shared Go module — config, observability, DB, cache, queue, repository |
| `services/api/` | HTTP API service (Chi router, handlers, business logic) |
| `services/worker/` | Background worker service (queue consumer) |

## Run locally

```bash
make tidy-all
make run-api
# In another terminal:
make run-worker
```

Build all services:

```bash
make build-all
```

Run all tests:

```bash
make test-all
```

## Docker Compose

```bash
docker compose -f compose.yaml up --build
```

### Observability stack

Same as the monolith template: Grafana http://localhost:3000 (`admin`/`admin`), Prometheus :9090, Pyroscope :4040, Tempo + OTLP collector for traces.

## Go workspace

This monorepo uses [Go workspaces](https://go.dev/doc/tutorial/workspaces) (`go.work`) for local development so that changes to `internal/` are immediately visible to all services without publishing.

Each service `go.mod` also carries a `replace` directive pointing to the local `internal/` so that CI and Docker builds work without `go.work`.

## Features

- Database: PostgreSQL (pgx)
- Cache: Redis
- Queue: Redis + workers
- Observability: OTel Prometheus metrics structured logs (zap)
