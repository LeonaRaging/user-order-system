#!/usr/bin/env bash

set -euo pipefail

USER_ID="${1:-u1}"
COUNT="${2:-20}"
TOKEN="${JWT_TOKEN:-}"
AUTH_HEADER="Bearer ${TOKEN}"

if [[ -z "$TOKEN" ]]; then
  echo "Set JWT_TOKEN to benchmark gateway-protected endpoints." >&2
  exit 1
fi

measure() {
  local name="$1"
  local url="$2"
  local auth_header="${3:-}"
  local samples
  samples="$(mktemp)"

  echo "$name"

  for _ in $(seq 1 "$COUNT"); do
    curl_args=(-sS -o /dev/null -w "%{time_total} %{http_code}\n")
    if [[ -n "$auth_header" ]]; then
      curl_args+=(-H "$auth_header")
    fi
    curl "${curl_args[@]}" "$url" | tee -a "$samples"
  done

  awk '
    { total += $1; count += 1; status[$2] += 1 }
    END {
      if (count == 0) exit 1
      printf "average=%.3fs requests=%d", total / count, count
      for (code in status) printf " http_%s=%d", code, status[code]
      print ""
    }
  ' "$samples"
  rm -f "$samples"
}

measure "Direct User service" \
  "http://localhost:8081/users/$USER_ID"

measure "User service through Nginx" \
  "http://localhost/users/$USER_ID" \
  "$AUTH_HEADER"

measure "Direct Order service" \
  "http://localhost:8082/orders?userId=$USER_ID"

measure "Order service through Nginx" \
  "http://localhost/orders?userId=$USER_ID" \
  "$AUTH_HEADER"

measure "Dashboard through Nginx" \
  "http://localhost/dashboard?userId=$USER_ID" \
  "$AUTH_HEADER"
