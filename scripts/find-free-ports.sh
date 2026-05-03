#!/usr/bin/env bash

set -euo pipefail

port_is_busy() {
  local port="$1"

  if command -v ss >/dev/null 2>&1; then
    if ss -ltn "( sport = :$port )" | awk 'NR > 1 { found = 1 } END { exit found ? 0 : 1 }'; then
      return 0
    fi
  fi

  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      return 0
    fi
  fi

  return 1
}

declare -A reserved_ports=()

pick_port() {
  local candidate="$1"

  while true; do
    if [[ -z "${reserved_ports[$candidate]+x}" ]] && ! port_is_busy "$candidate"; then
      reserved_ports["$candidate"]=1
      printf '%s\n' "$candidate"
      return 0
    fi

    candidate=$((candidate + 1))
  done
}

local_http_port="$(pick_port 18080)"
compose_http_port="$(pick_port 8080)"
compose_postgres_port="$(pick_port 5432)"
compose_rabbitmq_port="$(pick_port 5672)"
compose_rabbitmq_management_port="$(pick_port 15672)"
compose_minio_api_port="$(pick_port 9000)"
compose_minio_console_port="$(pick_port 9001)"

cat <<EOF
LOCAL_HTTP_PORT=$local_http_port
LOCAL_HTTP_ADDR=:$local_http_port
LOCAL_BASE_URL=http://localhost:$local_http_port
COMPOSE_HTTP_PORT=$compose_http_port
COMPOSE_POSTGRES_PORT=$compose_postgres_port
COMPOSE_RABBITMQ_PORT=$compose_rabbitmq_port
COMPOSE_RABBITMQ_MANAGEMENT_PORT=$compose_rabbitmq_management_port
COMPOSE_MINIO_API_PORT=$compose_minio_api_port
COMPOSE_MINIO_CONSOLE_PORT=$compose_minio_console_port
EOF
