#!/usr/bin/env bash

set -euo pipefail

PRODUCT_ID=${1:-1}
INITIAL_STOCK=${2:-10}
BASE_URL=${BASE_URL:-http://localhost:8080}

FAILURE_CONSTRAINT="problem_002_force_order_failure"
CONSTRAINT_INSTALLED=false

db_query() {
  docker compose exec -T postgres \
    psql -U hydra -d hydra_market \
    -v ON_ERROR_STOP=1 \
    -t -A \
    -c "$1" | tr -d '[:space:]'
}

cleanup() {
  if [[ "$CONSTRAINT_INSTALLED" != true ]]; then
    return
  fi

  echo
  echo "Removing controlled failure..."

  db_query "
    ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS $FAILURE_CONSTRAINT;
  " >/dev/null

  echo "Controlled failure removed."
}

trap cleanup EXIT

echo "========================================"
echo "Hydra Market - Order Atomicity Failure"
echo "========================================"
echo "Product ID:    $PRODUCT_ID"
echo "Initial stock: $INITIAL_STOCK"
echo

if ! curl -fsS "$BASE_URL/health" >/dev/null; then
  echo "ERROR: Hydra Market is not available at $BASE_URL." >&2
  exit 1
fi

PRODUCT_EXISTS=$(db_query \
  "SELECT EXISTS (SELECT 1 FROM products WHERE id = $PRODUCT_ID);")

if [[ "$PRODUCT_EXISTS" != "t" ]]; then
  echo "ERROR: product $PRODUCT_ID does not exist." >&2
  exit 1
fi

echo "Setting known inventory..."

SET_STOCK_STATUS=$(curl -sS -o /dev/null \
  -w "%{http_code}" \
  -X PUT "$BASE_URL/inventory/$PRODUCT_ID" \
  -H "Content-Type: application/json" \
  -d "{\"quantity\":$INITIAL_STOCK}")

if [[ "$SET_STOCK_STATUS" != "204" ]]; then
  echo "ERROR: failed to set inventory (HTTP $SET_STOCK_STATUS)." >&2
  exit 1
fi

STOCK_BEFORE=$(db_query \
  "SELECT quantity FROM inventory WHERE product_id = $PRODUCT_ID;")

ORDER_ITEMS_BEFORE=$(db_query \
  "SELECT COUNT(*)
   FROM order_items
   WHERE product_id = $PRODUCT_ID;")

echo
echo "Before:"
echo "  Stock:       $STOCK_BEFORE"
echo "  Order items: $ORDER_ITEMS_BEFORE"
echo
echo "Installing controlled failure..."

db_query "
  ALTER TABLE orders
  ADD CONSTRAINT $FAILURE_CONSTRAINT
  CHECK (status <> 'CREATED')
  NOT VALID;
" >/dev/null

CONSTRAINT_INSTALLED=true

echo "Sending order expected to fail..."

HTTP_STATUS=$(curl -sS -o /dev/null \
  -w "%{http_code}" \
  -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -d "{\"product_id\":$PRODUCT_ID,\"quantity\":1}")

STOCK_AFTER=$(db_query \
  "SELECT quantity FROM inventory WHERE product_id = $PRODUCT_ID;")

ORDER_ITEMS_AFTER=$(db_query \
  "SELECT COUNT(*)
   FROM order_items
   WHERE product_id = $PRODUCT_ID;")

echo
echo "After:"
echo "  HTTP status: $HTTP_STATUS"
echo "  Stock:       $STOCK_AFTER"
echo "  Order items: $ORDER_ITEMS_AFTER"
echo

if [[ "$HTTP_STATUS" == "500" \
   && "$STOCK_BEFORE" -eq "$INITIAL_STOCK" \
   && "$ORDER_ITEMS_AFTER" -eq "$ORDER_ITEMS_BEFORE" ]]; then
  if [[ "$STOCK_AFTER" -eq "$STOCK_BEFORE" ]]; then
    echo "ATOMICITY PRESERVED"
    echo "The order failed and the transaction restored the previous inventory."
    exit 0
  fi

  if [[ "$STOCK_AFTER" -eq $((STOCK_BEFORE - 1)) ]]; then
    echo "INCONSISTENCY CONFIRMED"
    echo "The order failed, but inventory decreased."
    exit 0
  fi
fi

echo "UNEXPECTED RESULT" >&2
echo "Review the output before drawing a conclusion." >&2
exit 1
