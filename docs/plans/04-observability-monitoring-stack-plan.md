# Инфраструктура мониторинга и логирования

## Статус реализации

Статус: реализовано в локальном Compose stack.

Подтверждено:

- `docker-compose.observability.yml` добавляет Prometheus, Alertmanager, Grafana, Jaeger, Loki, OpenTelemetry Collector, node-exporter, postgres-exporter и RabbitMQ Prometheus metrics.
- `Makefile` содержит `docker-observability-up`, `docker-observability-down`, `docker-observability-logs`, `docker-observability-ps`.
- `configs/observability/prometheus/prometheus.yml` scrape-ит server, worker, postgres-exporter, RabbitMQ и node-exporter.
- `configs/observability/otel-collector/config.yml`, `configs/observability/loki/loki.yml` и Grafana provisioning находятся в `configs/observability/`.
- Конфигурационные ожидания покрыты тестами в `internal/observability/grafana_dashboards_test.go`.

## Источники и контекст

- Базовая инфраструктура сейчас описана в `docker-compose.yml`: server, worker, PostgreSQL, MinIO, RabbitMQ.
- Локальные порты переопределяются через `.env.example`.
- Application instrumentation описан в `docs/plans/03-observability-application-instrumentation-plan.md`: server/worker уже пишут JSON `slog`, экспортируют traces по OTLP и отдают Prometheus metrics.
- Для целевой цепочки логов нужен дополнительный application step: экспорт logs из Go через OpenTelemetry Logs SDK и `otlploggrpc`.

## Цель

Добавить внешний observability stack для локальной и demo-среды:

- Prometheus для scrape metrics;
- Jaeger для distributed tracing;
- OpenTelemetry Collector как единый OTLP endpoint для traces и logs;
- Loki для хранения logs, полученных через native OTLP ingestion;
- Grafana для dashboards и просмотра metrics/traces/logs;
- `prom/node-exporter` для host-level infrastructure metrics в local/demo stack;
- optional exporters для PostgreSQL и RabbitMQ infrastructure metrics.

Целевая цепочка логов:

```text
Go application -> otlploggrpc -> OpenTelemetry Collector -> Loki -> Grafana
```

## Архитектурные решения

### Compose override вместо расширения базового compose

Создать `docker-compose.observability.yml`, чтобы обычный MVP workflow оставался легким.

Запуск:

```sh
docker compose -f docker-compose.yml -f docker-compose.observability.yml up -d --build
```

Makefile может получить отдельные targets:

- `docker-observability-up`
- `docker-observability-down`
- `docker-observability-logs`
- `docker-observability-ps`

### OpenTelemetry Collector как telemetry gateway

Server и worker отправляют traces и logs в `otel-collector:4317` по OTLP gRPC. Приложения не отправляют traces напрямую в Jaeger и не отправляют logs напрямую в Loki.

Collector отвечает за:

- прием OTLP gRPC/HTTP на `4317`/`4318`;
- traces pipeline в Jaeger;
- logs pipeline в Loki через OTLP HTTP endpoint `/otlp`;
- batching и базовую защиту от burst-нагрузки через processors.

### Логи из Go через `otlploggrpc`

Текущий JSON `slog` stdout остается полезным для локальной диагностики и `docker compose logs`, но не является источником централизованного Loki ingestion.

Для цепочки Go -> Collector -> Loki нужно расширить `internal/observability`:

- добавить dependency `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc`;
- добавить OpenTelemetry Logs SDK provider с batch processor;
- добавить config flags:
  - `OTEL_LOGS_ENABLED`;
  - `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT`, если logs endpoint должен отличаться от общего `OTEL_EXPORTER_OTLP_ENDPOINT`;
  - `OTEL_EXPORTER_OTLP_LOGS_INSECURE`, если нужно переопределить общий insecure flag;
- сохранить correlation fields `trace_id`, `span_id`, `request_id`, `service`, `component` в log records;
- вызвать shutdown logs provider при graceful stop server/worker.

## Сервисы

Prometheus:

- image `prom/prometheus`
- config: `configs/observability/prometheus/prometheus.yml`
- scrape targets:
  - `server:8080/metrics`
  - `worker:<METRICS_ADDR>/metrics`, если worker metrics server включен
  - `node-exporter:9100`
  - `rabbitmq:15692/metrics`, если включен RabbitMQ management metrics endpoint
  - postgres exporter, если добавлен

OpenTelemetry Collector:

- image `otel/opentelemetry-collector-contrib`
- config: `configs/observability/otel-collector/config.yml`
- receivers:
  - `otlp` gRPC endpoint `0.0.0.0:4317`
  - `otlp` HTTP endpoint `0.0.0.0:4318`
- processors:
  - `memory_limiter`
  - `batch`
- exporters:
  - `otlp/jaeger` to `jaeger:4317` for traces
  - `otlphttp/loki` to `http://loki:3100/otlp` for logs

Jaeger:

- image `jaegertracing/all-in-one`
- включить OTLP gRPC/HTTP receivers внутри compose network;
- UI port через `.env.example`, например `COMPOSE_JAEGER_UI_PORT=16686`;
- OTLP ports Jaeger не публиковать на host, чтобы не конфликтовать с Collector `4317`/`4318`;
- traces принимает от Collector, а не напрямую от server/worker.

Grafana:

- image `grafana/grafana`
- provisioning datasources и dashboards из `configs/observability/grafana`;
- datasource names: `Prometheus`, `Jaeger`, `Loki`.

Loki:

- image `grafana/loki`
- config: `configs/observability/loki/loki.yml`;
- хранение локальное docker volume;
- включить OTLP ingestion и structured metadata, например `limits_config.allow_structured_metadata: true`;
- labels из OTLP resource attributes учитывать в нормализованном виде, например `service.name` становится `service_name`.

Node Exporter:

- image `prom/node-exporter`;
- local/demo-only service для host-level CPU, memory, filesystem, disk и network metrics;
- scrape target в Prometheus: `node-exporter:9100`;
- port `9100` не публиковать на host по умолчанию, потому что Prometheus scrapes внутри compose network;
- readonly mounts:
  - `/proc:/host/proc:ro`;
  - `/sys:/host/sys:ro`;
  - `/:/rootfs:ro`;
- command flags:
  - `--path.procfs=/host/proc`;
  - `--path.sysfs=/host/sys`;
  - `--path.rootfs=/rootfs`;
- для Docker Desktop явно учитывать ограничение: metrics могут отражать Linux VM/container host, а не полную macOS/Windows host-систему.

Postgres exporter:

- optional service `postgres-exporter`;
- подключение к `postgres://avatars:avatars@postgres:5432/avatars?sslmode=disable`;
- scrape target в Prometheus.

RabbitMQ metrics:

- использовать management image и включить Prometheus plugin или endpoint, доступный на `15692`;
- если endpoint недоступен в выбранном image, заменить отдельным exporter в том же plan step.

## Конфигурационные файлы

Создать структуру:

```text
configs/
└── observability/
    ├── prometheus/
    │   ├── prometheus.yml
    │   └── alerts.yml
    ├── otel-collector/
    │   └── config.yml
    ├── grafana/
    │   ├── provisioning/
    │   │   ├── datasources/
    │   │   └── dashboards/
    │   └── dashboards/
    └── loki/
        └── loki.yml
```

`alerts.yml` можно добавить пустым или с базовой группой, если alerting этап еще не реализован.

## Env и порты

Обновить `.env.example`:

- `COMPOSE_PROMETHEUS_PORT=9090`
- `COMPOSE_GRAFANA_PORT=3000`
- `COMPOSE_JAEGER_UI_PORT=16686`
- `COMPOSE_OTEL_COLLECTOR_GRPC_PORT=4317`
- `COMPOSE_OTEL_COLLECTOR_HTTP_PORT=4318`
- `COMPOSE_LOKI_PORT=3100`
- `COMPOSE_RABBITMQ_METRICS_PORT=15692`

Server env в compose override:

- `OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317`
- `OTEL_EXPORTER_OTLP_INSECURE=true`
- `OTEL_TRACES_ENABLED=true`
- `OTEL_LOGS_ENABLED=true`
- `SERVICE_NAME=avatar-service-server`

Worker env:

- `OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317`
- `OTEL_EXPORTER_OTLP_INSECURE=true`
- `OTEL_TRACES_ENABLED=true`
- `OTEL_LOGS_ENABLED=true`
- `SERVICE_NAME=avatar-service-worker`
- `METRICS_ADDR=:9091`

Если logs endpoint нужно отделить от traces endpoint, использовать стандартные logs-specific переменные:

- `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT`
- `OTEL_EXPORTER_OTLP_LOGS_INSECURE`

## Implementation Steps

1. Расширить application instrumentation: добавить OTLP logs exporter на `otlploggrpc` и подключить его в server/worker startup.
2. Создать `docker-compose.observability.yml`.
3. Добавить config tree под `configs/observability`.
4. Настроить OpenTelemetry Collector OTLP receiver, traces pipeline в Jaeger и logs pipeline в Loki через `otlphttp/loki`.
5. Настроить Loki OTLP ingestion и structured metadata.
6. Добавить `node-exporter` service с readonly host mounts в observability compose override.
7. Настроить Prometheus scrape для application metrics и infrastructure metrics, включая `node-exporter`.
8. Настроить Jaeger OTLP receiver и проброс UI port.
9. Настроить Grafana provisioning datasources.
10. Добавить Makefile targets для observability compose workflow.
11. Обновить `.env.example` с новыми optional ports для опубликованных UI/API endpoints; для Node Exporter port не добавлять, пока `9100` не публикуется на host.
12. Обновить `docs/development-workflow.md` или README только если stack должен быть виден обычным разработчикам; иначе достаточно ссылки из `docs/plans/README.md`.

## Verification Plan

1. Поднять stack:

```sh
docker compose -f docker-compose.yml -f docker-compose.observability.yml up -d --build
```

2. Выполнить миграции:

```sh
docker compose run --rm server migrate up
```

3. Проверить API:

```sh
make docker-contract-tests
```

4. Проверить Collector:

- `otel-collector` стартует без config errors;
- OTLP gRPC endpoint принимает telemetry от server и worker;
- logs pipeline экспортирует записи в Loki;
- traces pipeline экспортирует spans в Jaeger.

5. Проверить Prometheus:

- target `server` healthy;
- target `worker` healthy, если `METRICS_ADDR` включен;
- target `node-exporter` healthy;
- HTTP и business metrics видны через Prometheus UI.
- базовые host metrics присутствуют: `node_cpu_seconds_total`, `node_memory_MemAvailable_bytes`, `node_filesystem_size_bytes`, `node_network_receive_bytes_total`.

6. Проверить Jaeger:

- выполнить upload через contract runner или curl;
- найти trace для `avatar-service-server`;
- увидеть HTTP span, service span, storage/repository/broker spans и worker span в одном trace;
- убедиться, что trace прошел через Collector, а не прямой endpoint Jaeger в server/worker env.

7. Проверить Loki:

- найти logs по `service_name="avatar-service-server"` или другому label, полученному из OTLP resource attributes;
- отфильтровать по `trace_id`;
- убедиться, что logs не содержат secrets, request body и upload bytes.

8. Проверить Grafana:

- datasources provisioned и healthy;
- dashboards открываются без ручной настройки;
- Loki datasource показывает logs, пришедшие через OTLP metadata.

## Acceptance Criteria

- Observability stack поднимается отдельным compose override без поломки базового `docker-compose.yml`.
- Server и worker экспортируют logs в Collector через `otlploggrpc`.
- Collector отправляет logs в Loki через OTLP HTTP endpoint `/otlp`.
- Prometheus scrapes server/worker metrics.
- Prometheus scrapes `node-exporter` metrics without publishing Node Exporter on a host port.
- Jaeger принимает traces от Collector.
- Loki хранит OTLP logs и позволяет искать по service labels и `trace_id`.
- Grafana имеет provisioned datasources Prometheus, Jaeger и Loki.
- Все новые ports документированы в `.env.example`.
