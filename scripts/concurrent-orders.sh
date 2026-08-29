#!/usr/bin/env bash

set -euo pipefail

PRODUCT_ID=${1:-1}
REQUESTS=${2:-50}
CONCURRENCY=${3:-50}
INITIAL_STOCK=${4:-10}

BASE_URL="http://localhost:8080"

db_query() {
  docker compose exec -T postgres \
    psql -U hydra -d hydra_market \
    -t -A \
    -c "$1" | tr -d '[:space:]'
}

echo "========================================"
echo "Hydra Market - Concurrent Orders"
echo "========================================"
echo "Product ID:     $PRODUCT_ID"
echo "Initial stock:  $INITIAL_STOCK"
echo "Requests:       $REQUESTS"
echo "Concurrency:    $CONCURRENCY"
echo

echo "Resetting inventory..."

curl -s -o /dev/null \
  -X PUT "$BASE_URL/inventory/$PRODUCT_ID" \
  -H "Content-Type: application/json" \
  -d "{\"quantity\":$INITIAL_STOCK}"

STOCK_BEFORE=$(db_query \
  "SELECT quantity FROM inventory WHERE product_id = $PRODUCT_ID;")

SOLD_BEFORE=$(db_query \
  "SELECT COALESCE(SUM(quantity), 0)
   FROM order_items
   WHERE product_id = $PRODUCT_ID;")

echo
echo "Before:"
echo "  Stock: $STOCK_BEFORE"
echo "  Sold:  $SOLD_BEFORE"
echo

echo "Sending concurrent orders..."
echo

RESULT=$(seq 1 "$REQUESTS" | xargs -P "$CONCURRENCY" -I{} \
  curl -s -o /dev/null \
  -w "%{http_code}\n" \
  -X POST "$BASE_URL/orders" \
  -H "Content-Type: application/json" \
  -d "{\"product_id\":$PRODUCT_ID,\"quantity\":1}" \
  | sort | uniq -c)

STOCK_AFTER=$(db_query \
  "SELECT quantity FROM inventory WHERE product_id = $PRODUCT_ID;")

SOLD_AFTER=$(db_query \
  "SELECT COALESCE(SUM(quantity), 0)
   FROM order_items
   WHERE product_id = $PRODUCT_ID;")

NEW_SALES=$((SOLD_AFTER - SOLD_BEFORE))

echo "$RESULT"

echo
echo "After:"
echo "  Stock:       $STOCK_AFTER"
echo "  Sold total:  $SOLD_AFTER"
echo "  New sales:   $NEW_SALES"

echo
echo "========================================"