#!/usr/bin/env bash
# End-to-end smoke test for the e-commerce microservices stack.
# Prerequisites: docker compose up, curl, python3, go
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
JWT_SECRET="${JWT_SECRET:-dev-secret-change-me}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-golang-ecommerce-microservice}"
CONFIRM_WAIT="${CONFIRM_WAIT:-6}"
CANCEL_WAIT="${CANCEL_WAIT:-3}"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

pass=0
fail=0

ok()   { echo -e "${GREEN}PASS${NC} $*"; pass=$((pass + 1)); }
bad()  { echo -e "${RED}FAIL${NC} $*"; fail=$((fail + 1)); }

json_field() {
  python3 -c "import sys,json; print(json.load(sys.stdin)['data']['$1'])"
}

json_auth_field() {
  python3 -c "import sys,json; print(json.load(sys.stdin)['data']['$1'])"
}

json_user_field() {
  python3 -c "import sys,json; print(json.load(sys.stdin)['data']['user']['$1'])"
}

http_code() {
  curl -s -o /dev/null -w "%{http_code}" "$@"
}

echo "=== E2E test ==="
echo "Gateway: $GATEWAY_URL"
echo ""

# Wait for gateway
echo "Waiting for gateway..."
for _ in $(seq 1 30); do
  if [ "$(http_code "$GATEWAY_URL/health/live")" = "200" ]; then
    break
  fi
  sleep 1
done
if [ "$(http_code "$GATEWAY_URL/health/live")" != "200" ]; then
  bad "gateway not reachable at $GATEWAY_URL (run: docker compose up -d)"
  exit 1
fi
ok "gateway /health/live"

SUFFIX="$(python3 -c "import uuid; print(uuid.uuid4().hex[:8])")"
TEST_EMAIL="e2e-${SUFFIX}@example.com"
TEST_PASSWORD="password123"
TEST_NAME="E2E User $SUFFIX"

# --- Auth: register + login (public routes) ---
REGISTER_RESP="$(curl -sf -H "Content-Type: application/json" \
  -d "{\"email\":\"$TEST_EMAIL\",\"name\":\"$TEST_NAME\",\"password\":\"$TEST_PASSWORD\"}" \
  "$GATEWAY_URL/api/v1/auth/register")"
[ -n "$REGISTER_RESP" ] && ok "register user" || bad "register user failed"

code="$(http_code -H "Content-Type: application/json" \
  -d "{\"email\":\"$TEST_EMAIL\",\"name\":\"$TEST_NAME\",\"password\":\"$TEST_PASSWORD\"}" \
  "$GATEWAY_URL/api/v1/auth/register")"
[ "$code" = "409" ] && ok "reject duplicate email ($code)" || bad "duplicate register (expected 409, got $code)"

LOGIN_RESP="$(curl -sf -H "Content-Type: application/json" \
  -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\"}" \
  "$GATEWAY_URL/api/v1/auth/login")"
JWT="$(echo "$LOGIN_RESP" | json_auth_field access_token)"
USER_ID="$(echo "$LOGIN_RESP" | json_user_field id)"
[ -n "$JWT" ] && [ -n "$USER_ID" ] && ok "login returns JWT (user=$USER_ID)" || bad "login failed"

ME="$(curl -sf -H "Authorization: Bearer $JWT" "$GATEWAY_URL/api/v1/auth/me")"
ME_EMAIL="$(echo "$ME" | json_field email)"
[ "$ME_EMAIL" = "$TEST_EMAIL" ] && ok "auth /me returns profile" || bad "auth /me (email=$ME_EMAIL)"

# --- JWT auth ---
code="$(http_code "$GATEWAY_URL/api/v1/products")"
[ "$code" = "401" ] && ok "rejects missing JWT ($code)" || bad "expected 401 without JWT, got $code"

code="$(http_code -H "Authorization: Bearer invalid" "$GATEWAY_URL/api/v1/products")"
[ "$code" = "401" ] && ok "rejects invalid JWT ($code)" || bad "expected 401 for bad JWT, got $code"

code="$(http_code -H "Authorization: Bearer $JWT" "$GATEWAY_URL/api/v1/products")"
[ "$code" = "200" ] && ok "accepts valid JWT ($code)" || bad "expected 200 with JWT, got $code"

# --- Catalog ---
SUFFIX="$SUFFIX-catalog"

CAT="$(curl -sf -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d "{\"name\":\"E2E-Category-$SUFFIX\"}" "$GATEWAY_URL/api/v1/categories")"
CAT_ID="$(echo "$CAT" | json_field id)"
[ -n "$CAT_ID" ] && ok "create category ($CAT_ID)" || bad "create category"

PROD="$(curl -sf -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d "{\"name\":\"E2E-Product-$SUFFIX\",\"description\":\"e2e test\",\"price\":19.99,\"category_id\":\"$CAT_ID\",\"stock\":5}" \
  "$GATEWAY_URL/api/v1/products")"
PROD_ID="$(echo "$PROD" | json_field id)"
INIT_STOCK="$(echo "$PROD" | json_field stock)"
[ "$INIT_STOCK" = "5" ] && ok "create product stock=$INIT_STOCK ($PROD_ID)" || bad "create product (stock=$INIT_STOCK)"

# --- Order: create → confirm → stock decrement ---
ORDER="$(curl -sf -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID\",\"product_id\":\"$PROD_ID\",\"quantity\":2}" \
  "$GATEWAY_URL/api/v1/orders")"
ORDER_ID="$(echo "$ORDER" | json_field id)"
ORDER_STATUS="$(echo "$ORDER" | json_field status)"
[ "$ORDER_STATUS" = "pending" ] && ok "create order pending ($ORDER_ID)" || bad "create order (status=$ORDER_STATUS)"

echo "Waiting ${CONFIRM_WAIT}s for payment stub confirm..."
sleep "$CONFIRM_WAIT"

ORDER_AFTER="$(curl -sf -H "Authorization: Bearer $JWT" "$GATEWAY_URL/api/v1/orders/$ORDER_ID")"
CONFIRMED_STATUS="$(echo "$ORDER_AFTER" | json_field status)"
[ "$CONFIRMED_STATUS" = "confirmed" ] && ok "order auto-confirmed" || bad "order status=$CONFIRMED_STATUS (expected confirmed)"

PROD_AFTER="$(curl -sf -H "Authorization: Bearer $JWT" "$GATEWAY_URL/api/v1/products/$PROD_ID")"
STOCK_AFTER="$(echo "$PROD_AFTER" | json_field stock)"
[ "$STOCK_AFTER" = "3" ] && ok "stock decremented 5→3" || bad "stock=$STOCK_AFTER (expected 3)"

# --- Order: confirm → cancel → stock restore ---
ORDER2="$(curl -sf -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID\",\"product_id\":\"$PROD_ID\",\"quantity\":1}" \
  "$GATEWAY_URL/api/v1/orders")"
ORDER2_ID="$(echo "$ORDER2" | json_field id)"

sleep "$CONFIRM_WAIT"
curl -sf -X PUT -H "Authorization: Bearer $JWT" "$GATEWAY_URL/api/v1/orders/$ORDER2_ID/cancel" > /dev/null
ok "cancel order $ORDER2_ID"

sleep "$CANCEL_WAIT"
PROD_FINAL="$(curl -sf -H "Authorization: Bearer $JWT" "$GATEWAY_URL/api/v1/products/$PROD_ID")"
STOCK_FINAL="$(echo "$PROD_FINAL" | json_field stock)"
[ "$STOCK_FINAL" = "3" ] && ok "stock restored after cancel 2→3" || bad "stock=$STOCK_FINAL (expected 3 after cancel restore)"

# --- Docker log checks (optional) ---
if command -v docker >/dev/null 2>&1; then
  NOTIF_LOG="$(docker logs "${COMPOSE_PROJECT}-notification-service-1" 2>&1 | grep "EMAIL STUB.*$ORDER_ID" || true)"
  if echo "$NOTIF_LOG" | grep -q "order.confirmed"; then
    ok "notification logged order.confirmed"
  else
    bad "notification missing order.confirmed for $ORDER_ID"
  fi

  OUTBOX_LOG="$(docker logs "${COMPOSE_PROJECT}-order-worker-1" 2>&1 | grep "published event" | tail -1 || true)"
  if [ -n "$OUTBOX_LOG" ]; then
    ok "outbox worker publishing events"
  else
    bad "outbox worker log empty"
  fi

  RESTORE_LOG="$(docker logs "${COMPOSE_PROJECT}-inventory-worker-1" 2>&1 | grep "stock restored" | tail -1 || true)"
  if [ -n "$RESTORE_LOG" ]; then
    ok "inventory worker restored stock"
  else
    bad "inventory worker restore log missing"
  fi
else
  echo "(skip docker log checks — docker not installed)"
fi

# --- Rate limit (burst) ---
echo ""
echo "Rate limit burst (65 parallel requests)..."
TMPDIR_RATELIMIT="$(mktemp -d)"
for i in $(seq 1 65); do
  curl -s -o /dev/null -w "%{http_code}\n" \
    -H "Authorization: Bearer $JWT" \
    "$GATEWAY_URL/api/v1/products" > "$TMPDIR_RATELIMIT/$i" &
done
wait
RATELIMIT_200=0
RATELIMIT_429=0
for f in "$TMPDIR_RATELIMIT"/*; do
  code="$(cat "$f")"
  case "$code" in
    200) RATELIMIT_200=$((RATELIMIT_200 + 1)) ;;
    429) RATELIMIT_429=$((RATELIMIT_429 + 1)) ;;
  esac
done
rm -rf "$TMPDIR_RATELIMIT"

if [ "$RATELIMIT_200" -ge 1 ] && [ "$RATELIMIT_429" -ge 1 ]; then
  ok "rate limit (${RATELIMIT_200}x 200, ${RATELIMIT_429}x 429)"
else
  bad "rate limit (${RATELIMIT_200}x 200, ${RATELIMIT_429}x 429 — expected some 429)"
fi

# --- Summary ---
echo ""
echo "=== Results: $pass passed, $fail failed ==="
[ "$fail" -eq 0 ] && exit 0 || exit 1
