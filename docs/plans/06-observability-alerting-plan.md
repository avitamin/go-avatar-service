# Alerting для avatar-service

## Источники и зависимости

- Metrics names и labels должны соответствовать `docs/plans/03-observability-application-instrumentation-plan.md`.
- Prometheus stack должен быть добавлен по `docs/plans/04-observability-monitoring-stack-plan.md`.

## Цель

Добавить бонусный alerting для критичных runtime symptoms:

- высокий процент HTTP/API ошибок;
- высокий процент upload ошибок;
- рост latency;
- degraded dependencies;
- backlog в RabbitMQ queues;
- worker failures.

## Архитектурные решения

### Alertmanager как optional compose service

Добавить Alertmanager в `docker-compose.observability.yml` только на alerting этапе.

Конфиги:

- `configs/observability/prometheus/alerts.yml`
- `configs/observability/alertmanager/alertmanager.yml`

Для локального dev не добавлять реальные секреты. Receiver по умолчанию:

- `null` или local webhook stub;
- external integrations описывать как future extension.

### Severity labels

Использовать:

- `warning`: симптом заметен, но сервис может продолжать работу.
- `critical`: пользовательский путь или dependency деградировали достаточно сильно, чтобы требовать реакции.

Обязательные labels:

- `severity`
- `service="avatar-service"`
- `component`, если alert относится к dependency или worker.

## Alert Rules

### HighHTTPErrorRate

```yaml
- alert: HighHTTPErrorRate
  expr: |
    sum(rate(http_requests_total{status=~"5.."}[5m]))
    /
    sum(rate(http_requests_total[5m])) > 0.05
  for: 5m
  labels:
    severity: warning
    service: avatar-service
  annotations:
    summary: "High HTTP 5xx error rate"
    description: "More than 5% of HTTP requests are returning 5xx for 5 minutes."
```

### HighUploadErrorRate

```yaml
- alert: HighUploadErrorRate
  expr: |
    sum(rate(avatars_uploads_total{status="error"}[5m]))
    /
    sum(rate(avatars_uploads_total[5m])) > 0.10
  for: 5m
  labels:
    severity: warning
    service: avatar-service
    component: api
  annotations:
    summary: "High avatar upload error rate"
    description: "More than 10% of avatar uploads are failing for 5 minutes."
```

### HighResponseTimeP95

```yaml
- alert: HighResponseTimeP95
  expr: |
    histogram_quantile(
      0.95,
      sum by (le, route) (rate(http_request_duration_seconds_bucket[5m]))
    ) > 2
  for: 5m
  labels:
    severity: warning
    service: avatar-service
  annotations:
    summary: "High HTTP p95 response time"
    description: "HTTP p95 latency is above 2 seconds for at least one route."
```

### UploadLatencyCritical

```yaml
- alert: UploadLatencyCritical
  expr: |
    histogram_quantile(
      0.95,
      sum by (le) (rate(avatars_upload_duration_seconds_bucket[5m]))
    ) > 5
  for: 2m
  labels:
    severity: critical
    service: avatar-service
    component: api
  annotations:
    summary: "Avatar upload p95 latency is critical"
    description: "Avatar upload p95 latency is above 5 seconds for 2 minutes."
```

### DependencyOperationErrors

```yaml
- alert: DependencyOperationErrors
  expr: |
    sum by (component) (
      rate(avatar_dependency_operations_total{status="error"}[5m])
    ) > 0
  for: 5m
  labels:
    severity: warning
    service: avatar-service
  annotations:
    summary: "Dependency operation errors"
    description: "Dependency {{ $labels.component }} has operation errors."
```

### RabbitMQQueueBacklog

```yaml
- alert: RabbitMQQueueBacklog
  expr: |
    rabbitmq_queue_messages{queue=~"avatars\\.uploads|avatars\\.deletes"} > 100
  for: 10m
  labels:
    severity: warning
    service: avatar-service
    component: rabbitmq
  annotations:
    summary: "RabbitMQ queue backlog"
    description: "Queue {{ $labels.queue }} has more than 100 messages for 10 minutes."
```

Metric name for RabbitMQ queue depth must be checked against the selected RabbitMQ exporter before final implementation.

### WorkerProcessingFailures

```yaml
- alert: WorkerProcessingFailures
  expr: |
    sum by (routing_key) (
      rate(avatar_worker_messages_total{status="error"}[5m])
    ) > 0
  for: 5m
  labels:
    severity: warning
    service: avatar-service
    component: worker
  annotations:
    summary: "Worker processing failures"
    description: "Worker has processing failures for {{ $labels.routing_key }}."
```

## Implementation Steps

1. Добавить `configs/observability/prometheus/alerts.yml`.
2. Подключить `rule_files` в `prometheus.yml`.
3. Добавить `alerting.alertmanagers` в `prometheus.yml`.
4. Добавить Alertmanager service в `docker-compose.observability.yml`.
5. Добавить `configs/observability/alertmanager/alertmanager.yml` с local/dev receiver.
6. Обновить `.env.example`:
   - `COMPOSE_ALERTMANAGER_PORT=9093`
7. Добавить Grafana dashboard panel или annotation source для active alerts.
8. Проверить rule syntax через `promtool`, если binary доступен в контейнере или локально.

## Тестовые сценарии

- High upload error rate:
  - выполнить серию invalid uploads с неподдерживаемыми bytes;
  - проверить firing или pending alert в Prometheus UI.
- High response time:
  - использовать временный test-only delay flag только если он уже введен отдельной задачей;
  - без delay flag проверить PromQL syntax и оставить runtime trigger manual.
- Dependency errors:
  - остановить MinIO или RabbitMQ в compose;
  - выполнить affected API path или `/health`;
  - проверить alert pending/firing.
- Queue backlog:
  - остановить worker;
  - выполнить серию upload requests;
  - проверить RabbitMQ queue depth alert.
- Worker failures:
  - создать delivery с некорректным payload через RabbitMQ management UI или test publisher;
  - проверить worker error metric and alert.

## Acceptance Criteria

- Prometheus загружает `alerts.yml` без ошибок.
- Alertmanager доступен через compose stack.
- Alerts имеют severity, service и component labels.
- High error rate и queue backlog alerts можно вызвать локальными сценариями.
- Alert rules не используют high-cardinality labels.
