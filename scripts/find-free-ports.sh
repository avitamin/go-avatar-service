#!/usr/bin/env bash

set -euo pipefail

port_is_busy() {
  local port="$1"

  if command -v ss >/dev/null 2>&1; then
    if ss -ltn "( sport = :$port )" 2>/dev/null | awk 'NR > 1 { found = 1 } END { exit found ? 0 : 1 }'; then
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
  local -n output_var="$1"
  local candidate="$2"

  while true; do
    if [[ -z "${reserved_ports[$candidate]+x}" ]] && ! port_is_busy "$candidate"; then
      reserved_ports["$candidate"]=1
      output_var="$candidate"
      return 0
    fi

    candidate=$((candidate + 1))
  done
}

pick_port local_http_port 18080
pick_port compose_http_port 8080
pick_port compose_postgres_port 5432
pick_port compose_rabbitmq_port 5672
pick_port compose_rabbitmq_management_port 15672
pick_port compose_minio_api_port 9000
pick_port compose_minio_console_port 9001
pick_port compose_prometheus_port 9090
pick_port compose_grafana_port 3000
pick_port compose_jaeger_ui_port 16686
pick_port compose_otel_collector_grpc_port 4317
pick_port compose_otel_collector_http_port 4318
pick_port compose_loki_port 3100
pick_port compose_rabbitmq_metrics_port 15692

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
COMPOSE_PROMETHEUS_PORT=$compose_prometheus_port
COMPOSE_GRAFANA_PORT=$compose_grafana_port
COMPOSE_JAEGER_UI_PORT=$compose_jaeger_ui_port
COMPOSE_OTEL_COLLECTOR_GRPC_PORT=$compose_otel_collector_grpc_port
COMPOSE_OTEL_COLLECTOR_HTTP_PORT=$compose_otel_collector_http_port
COMPOSE_LOKI_PORT=$compose_loki_port
COMPOSE_RABBITMQ_METRICS_PORT=$compose_rabbitmq_metrics_port
EOF
