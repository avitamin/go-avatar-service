# Grafana dashboards для avatar-service

## Источники и зависимости

- Application metrics и trace/log correlation: `docs/plans/03-observability-application-instrumentation-plan.md`.
- Compose stack и datasource provisioning: `docs/plans/04-observability-monitoring-stack-plan.md`.

## Цель

Создать provisioned Grafana dashboards, которые покрывают:

- service overview;
- RED metrics;
- infrastructure metrics;
- business KPIs;
- links между metrics, traces и logs.

## Общие правила dashboard design

- Dashboards хранить как JSON под `configs/observability/grafana/dashboards/`.
- Provisioning хранить под `configs/observability/grafana/provisioning/dashboards/`.
- Datasource names фиксировать: `Prometheus`, `Jaeger`, `Loki`.
- Все PromQL queries должны использовать низкокардинальные labels.
- `user_id` и `avatar_id` не использовать в Prometheus queries; для них использовать Loki/Jaeger filtering.

## Dashboard 1. Avatar Service Overview

Файл: `configs/observability/grafana/dashboards/avatar-service-overview.json`.

Panels:

- Request rate by route:
  - `sum by (route, method) (rate(http_requests_total[5m]))`
- Error rate:
  - `sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))`
- p95 latency by route:
  - `histogram_quantile(0.95, sum by (le, route, method) (rate(http_request_duration_seconds_bucket[5m])))`
- In-flight requests:
  - `sum by (route, method) (http_inflight_requests)`
- Health degradation signals:
  - `avatar_dependency_operations_total{operation="health_check",status="error"}` или эквивалентная health metric, если она добавлена на этапе instrumentation.
- Recent error logs:
  - Loki query `{service=~"avatar-service-.*", level="ERROR"}`.

Links:

- panel link в Jaeger по `service=avatar-service-server`;
- panel link в Loki по selected service label.

## Dashboard 2. Avatar Business KPIs

Файл: `configs/observability/grafana/dashboards/avatar-business-kpis.json`.

Panels:

- Upload rate by status:
  - `sum by (status) (rate(avatars_uploads_total[5m]))`
- Upload error ratio:
  - `sum(rate(avatars_uploads_total{status="error"}[5m])) / sum(rate(avatars_uploads_total[5m]))`
- Upload duration p95:
  - `histogram_quantile(0.95, sum by (le, status) (rate(avatars_upload_duration_seconds_bucket[5m])))`
- Storage bytes by kind:
  - `sum by (kind) (avatars_storage_bytes)`
- Delete rate:
  - `sum by (status) (rate(avatars_deletes_total[5m]))`
- Worker messages by routing key:
  - `sum by (routing_key, status) (rate(avatar_worker_messages_total[5m]))`
- Worker processing p95:
  - `histogram_quantile(0.95, sum by (le, routing_key) (rate(avatar_worker_processing_duration_seconds_bucket[5m])))`
- Thumbnail generation success/failure:
  - `sum by (size, status) (rate(avatar_worker_thumbnail_generation_total[5m]))`

Links:

- upload panels link to Jaeger traces for `POST /api/v1/avatars`;
- error panels link to Loki query filtered by `msg` or `level`.

## Dashboard 3. Infrastructure

Файл: `configs/observability/grafana/dashboards/avatar-infrastructure.json`.

Panels:

- PostgreSQL connections:
  - use postgres exporter metric such as `pg_stat_activity_count` if exporter is enabled.
- PostgreSQL up:
  - `up{job="postgres"}` or exporter-specific availability metric.
- Node Exporter up:
  - `up{job="node-exporter"}`.
- Host CPU usage/load:
  - use standard `node_cpu_seconds_total` and node load metrics from `prom/node-exporter`.
- Host memory usage:
  - use standard memory metrics such as `node_memory_MemAvailable_bytes` and total memory metrics.
- Host filesystem usage:
  - use `node_filesystem_size_bytes` and available/free filesystem metrics, excluding ephemeral mounts where needed.
- Host disk IO:
  - use standard disk read/write metrics from Node Exporter.
- Host network traffic:
  - use `node_network_receive_bytes_total` and `node_network_transmit_bytes_total`.
- RabbitMQ queue depth:
  - use RabbitMQ Prometheus metric for `avatars.uploads` and `avatars.deletes`.
- RabbitMQ consumers:
  - queue consumers by queue.
- MinIO availability:
  - application dependency operation metrics for MinIO health checks if direct MinIO metrics are not enabled.
- Dependency operation errors:
  - `sum by (component, operation) (rate(avatar_dependency_operations_total{status="error"}[5m]))`
- Dependency latency:
  - `histogram_quantile(0.95, sum by (le, component, operation) (rate(avatar_dependency_operation_duration_seconds_bucket[5m])))`

## Dashboard Variables

Добавить variables:

- `service`: label values from `up` or logs, default all avatar services.
- `route`: label values from `http_requests_total`.
- `routing_key`: label values from `avatar_worker_messages_total`.
- `interval`: default `$__rate_interval`.

## Implementation Steps

1. Создать dashboard JSON files вручную или экспортировать из Grafana после сборки prototype.
2. Добавить dashboard provider config:
   - folder `Avatar Service`;
   - path `/etc/grafana/provisioning-dashboards`.
3. Убедиться, что dashboards не требуют ручной datasource selection после старта compose.
4. Проверить PromQL against actual metric names после этапа application instrumentation.
5. Добавить links из panels:
   - Prometheus metric panel -> Jaeger service/operation search;
   - logs panel -> Loki query by `trace_id`, когда trace id доступен из log line.

## Verification Plan

- Поднять observability compose stack.
- Выполнить upload/list/read/delete сценарии через contract runner.
- Проверить, что panels не показывают PromQL errors.
- Проверить, что Node Exporter panels используют реальные `node_*` series из Prometheus target `node-exporter`.
- Проверить, что empty panels объяснимы отсутствием traffic, а не неверными labels.
- Проверить, что links открывают Jaeger/Loki с ожидаемыми фильтрами.

## Acceptance Criteria

- Grafana стартует с тремя dashboard файлами без ручного импорта.
- Service overview показывает RED metrics.
- Business KPI dashboard показывает upload, delete, storage и worker metrics.
- Infrastructure dashboard показывает PostgreSQL/RabbitMQ/Node Exporter/dependency health signals.
- Datasource links работают для traces и logs.
