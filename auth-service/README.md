# Auth Service

User registration, login, and JWT issuance for the e-commerce stack.

## Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/register` | No | Create account + return JWT |
| POST | `/api/v1/auth/login` | No | Login + return JWT |
| GET | `/api/v1/auth/me` | Bearer JWT | Current user profile |

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8086` | HTTP port |
| `JWT_SECRET` | `dev-secret-change-me` | HS256 signing secret (must match gateway) |
| `JWT_TTL_HOURS` | `24` | Access token lifetime |
| `DB_DSN` | `file:./data/app.db?_foreign_keys=on` | SQLite DSN |

## Run locally

```bash
cd auth-service/services/api
JWT_SECRET=dev-secret-change-me go run ./cmd/server
```
