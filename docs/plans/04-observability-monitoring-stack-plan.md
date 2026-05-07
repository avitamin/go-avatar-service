# Инфраструктура мониторинга и логирования

## Источники и контекст

- Базовая инфраструктура сейчас описана в `docker-compose.yml`: server, worker, PostgreSQL, MinIO, RabbitMQ.
- Локальные порты переопределяются через `.env.example`.
- Application instrumentation должен быть реализован по `docs/plans/03-observability-application-instrumentation-plan.md`.

## Цель

Добавить внешний observability stack для локальной и demo-среды:

- Prometheus для scrape metrics;
- Jaeger для distributed tracing;
- Grafana для dashboards;
- Loki и Grafana Alloy для централизованного сбора JSON stdout logs;
- optional exporters для PostgreSQL и RabbitMQ infrastructure metrics.

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

### Сервисы

Prometheus:

- image `prom/prometheus`
- config: `configs/observability/prometheus/prometheus.yml`
- scrape targets:
  - `server:8080/metrics`
  - `worker:<METRICS_ADDR>/metrics`, если worker metrics server включен
  - `rabbitmq:15692/metrics`, если включен RabbitMQ management metrics endpoint
  - postgres exporter, если добавлен

Jaeger:

- image `jaegertracing/all-in-one`
- включить OTLP gRPC/HTTP receivers;
- UI port через `.env.example`, например `COMPOSE_JAEGER_UI_PORT=16686`;
- server/worker отправляют traces в `jaeger:4317`.

Grafana:

- image `grafana/grafana`
- provisioning datasources и dashboards из `configs/observability/grafana`;
- datasource names: `Prometheus`, `Jaeger`, `Loki`.

Loki:

- image `grafana/loki`
- config: `configs/observability/loki/loki.yml`;
- хранение локальное docker volume.

Grafana Alloy:

- image `grafana/alloy`
- config: `configs/observability/alloy/config.alloy`;
- читает Docker container stdout logs;
- labels: `service`, `container`, `compose_service`;
- парсит JSON fields из `slog` logs и отправляет в Loki.

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
    ├── grafana/
    │   ├── provisioning/
    │   │   ├── datasources/
    │   │   └── dashboards/
    │   └── dashboards/
    ├── loki/
    │   └── loki.yml
    └── alloy/
        └── config.alloy
```

`alerts.yml` можно добавить пустым или с базовой группой, если alerting этап еще не реализован.

## Env и порты

Обновить `.env.example`:

- `COMPOSE_PROMETHEUS_PORT=9090`
- `COMPOSE_GRAFANA_PORT=3000`
- `COMPOSE_JAEGER_UI_PORT=16686`
- `COMPOSE_JAEGER_OTLP_GRPC_PORT=4317`
- `COMPOSE_JAEGER_OTLP_HTTP_PORT=4318`
- `COMPOSE_LOKI_PORT=3100`
- `COMPOSE_RABBITMQ_METRICS_PORT=15692`

Server env в compose override:

- `OTEL_EXPORTER_OTLP_ENDPOINT=jaeger:4317`
- `OTEL_TRACES_ENABLED=true`
- `SERVICE_NAME=avatar-service-server`

Worker env:

- `OTEL_EXPORTER_OTLP_ENDPOINT=jaeger:4317`
- `OTEL_TRACES_ENABLED=true`
- `SERVICE_NAME=avatar-service-worker`
- `METRICS_ADDR=:9091`

## Implementation Steps

1. Создать `docker-compose.observability.yml`.
2. Добавить config tree под `configs/observability`.
3. Настроить Prometheus scrape для application metrics и infrastructure metrics.
4. Настроить Jaeger OTLP receiver и проброс UI port.
5. Настроить Loki + Alloy на сбор JSON stdout logs из Docker containers.
6. Настроить Grafana provisioning datasources.
7. Добавить Makefile targets для observability compose workflow.
8. Обновить `.env.example` с новыми optional ports.
9. Обновить `docs/development-workflow.md` или README только если stack должен быть виден обычным разработчикам; иначе достаточно ссылки из `docs/plans/README.md`.

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

4. Проверить Prometheus:

- target `server` healthy;
- target `worker` healthy, если `METRICS_ADDR` включен;
- HTTP и business metrics видны через Prometheus UI.

5. Проверить Jaeger:

- выполнить upload через contract runner или curl;
- найти trace для `avatar-service-server`;
- увидеть HTTP span, service span, storage/repository/broker spans и worker span в одном trace.

6. Проверить Loki:

- найти logs по `{service="avatar-service-server"}`;
- отфильтровать по `trace_id`;
- убедиться, что logs не содержат secrets и upload bytes.

7. Проверить Grafana:

- datasources provisioned и healthy;
- dashboards открываются без ручной настройки.

## Acceptance Criteria

- Observability stack поднимается отдельным compose override без поломки базового `docker-compose.yml`.
- Prometheus scrapes server/worker metrics.
- Jaeger принимает traces от server и worker.
- Loki индексирует JSON stdout logs и позволяет искать по `trace_id`.
- Grafana имеет provisioned datasources Prometheus, Jaeger и Loki.
- Все новые ports документированы в `.env.example`.
