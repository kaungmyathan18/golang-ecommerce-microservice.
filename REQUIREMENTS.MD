# E-Commerce Order System — Project Brief

---

## Overview

A backend-only microservice system that handles product management, order processing, and notifications. Three independent services communicate via gRPC (synchronous) and RabbitMQ (async events). Each service owns its own database.

---

## Services

### 1. Product Service
Manages products and inventory stock.

**Endpoints (REST)**
```
POST   /api/v1/products
GET    /api/v1/products
GET    /api/v1/products/:id
PUT    /api/v1/products/:id
DELETE /api/v1/products/:id

POST   /api/v1/categories
GET    /api/v1/categories
PUT    /api/v1/categories/:id
DELETE /api/v1/categories/:id
```

**gRPC (internal only)**
```protobuf
service ProductService {
  rpc GetProduct(GetProductRequest) returns (ProductResponse);
  rpc CheckStock(CheckStockRequest) returns (CheckStockResponse);
  rpc DecrementStock(DecrementStockRequest) returns (DecrementStockResponse);
}
```

**DB Schema (MySQL)**
```sql
products    (id, name, description, price, category_id, stock, created_at)
categories  (id, name, created_at)
```

---

### 2. Order Service
Creates and manages orders. Orchestrates the order lifecycle.

**Endpoints (REST)**
```
POST   /api/v1/orders
GET    /api/v1/orders
GET    /api/v1/orders/:id
PUT    /api/v1/orders/:id/cancel
GET    /api/v1/orders/user/:user_id
```

**Order Status Flow**
```
pending → confirmed → shipped → delivered
        ↘ cancelled (from pending or confirmed only)
```

**Business Rules**
- On `POST /orders`: validate product exists via gRPC, check stock via gRPC, create order with status `pending`
- On confirm (internal, triggered after payment stub): decrement stock via gRPC, publish `order.confirmed` event
- On cancel: if stock was already decremented, publish `order.cancelled` event so Product Service can restore stock
- Uses **outbox pattern**: event rows written in same DB transaction as order update, separate worker publishes to RabbitMQ

**DB Schema (MySQL)**
```sql
orders (
  id, user_id, product_id, quantity,
  total_price, status, created_at, updated_at
)
outbox (
  id, event_type, payload, published, created_at
)
```

---

### 3. Notification Service
Consumes events and records notification history. No REST API except health check.

**Consumes**
- `order.confirmed` → send email (stubbed, just log it)
- `order.cancelled` → send email (stubbed)

**DB Schema (SQLite)**
```sql
notifications (id, user_id, order_id, event_type, message, sent_at)
```

---

### 4. API Gateway
Single entry point. No business logic.

**Responsibilities**
- JWT validation (all routes except `/health`)
- Route to correct upstream service
- Request ID injection
- Rate limiting per IP (Redis)
- Access logging

**Routing**
```
/api/v1/products/*   → product-service:8081
/api/v1/categories/* → product-service:8081
/api/v1/orders/*     → order-service:8082
```

---

## Infrastructure

```yaml
services:
  api-gateway          # :8080
  product-service      # :8081 REST, :9091 gRPC
  order-service        # :8082 REST, :9092 gRPC
  notification-service # no public port
  rabbitmq             # :5672, management :15672
  mysql-product        # :5433  ← separate DB per service
  mysql-order          # :5434
  redis                # :6379 (gateway rate limiting)
```

---

## Key Patterns To Implement

### Outbox Pattern (Order Service)
```
HTTP request arrives
  → BEGIN transaction
  → INSERT into orders (status = pending)
  → INSERT into outbox (event_type = order.created, published = false)
  → COMMIT

Background worker (every 1s):
  → SELECT * FROM outbox WHERE published = false
  → Publish each to RabbitMQ
  → UPDATE outbox SET published = true
```
This guarantees no event is lost even if the service crashes mid-publish.

### Stock Decrement Safety (Product Service)
```sql
UPDATE products
SET stock = stock - ?
WHERE id = ? AND stock >= ?
-- check rows affected = 1, else return insufficient stock error
```
Atomic at DB level, no application-level locking needed.

---

## Folder Structure

```
ecommerce/
├── api-gateway/
│   ├── cmd/server/main.go
│   └── internal/
│       ├── proxy/
│       ├── middleware/
│       └── ratelimit/
│
├── product-service/
│   ├── cmd/server/main.go
│   └── internal/
│       ├── handler/
│       ├── service/
│       ├── repository/
│       └── grpc/
│
├── order-service/
│   ├── cmd/server/main.go
│   └── internal/
│       ├── handler/
│       ├── service/
│       ├── repository/
│       ├── outbox/        ← outbox worker lives here
│       └── grpc/          ← gRPC client for product-service
│
├── notification-service/
│   ├── cmd/server/main.go
│   └── internal/
│       ├── consumer/
│       └── repository/
│
├── proto/
│   └── product/
│       └── product.proto  ← shared proto definitions
│
├── docker/
│   ├── rabbitmq/
│   ├── mysql/
│   └── ...
│
└── compose.yaml
```

---

## Out of Scope (keep it simple)

- No real payment processing (stub it — order auto-confirms after 5 seconds via a goroutine)
- No real email sending (just log the notification)
- No frontend
- No Kubernetes (Docker Compose only)
- No auth service (gateway just validates a hardcoded JWT secret)
- No user service (user_id is just a UUID passed in the request)

---

## Done Means

- [ ] All REST endpoints work end to end
- [ ] gRPC calls between order-service and product-service work
- [ ] Stock correctly decrements on confirm, restores on cancel
- [ ] Outbox worker reliably publishes events even after simulated crash
- [ ] Notification service logs received events and saves to DB
- [ ] Gateway rejects requests with missing/invalid JWT
- [ ] Gateway rate limits by IP
- [ ] `docker compose up` brings the whole thing up cleanly
- [ ] Each service has `/health/live` and `/health/ready`

---

## Suggested Build Order

1. Write the proto file, generate Go code first
2. Product Service (REST + gRPC server) — foundation everything else depends on
3. Order Service (REST only, no events yet) talking to Product via gRPC
4. Add outbox worker and RabbitMQ publishing to Order Service
5. Notification Service consuming from RabbitMQ
6. API Gateway last — everything works without it, it's just a shell on top