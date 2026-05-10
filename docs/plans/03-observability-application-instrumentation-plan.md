# Инструментирование observability в приложении

## Статус реализации

Статус: реализовано в коде приложения.

Подтверждено:

- `internal/observability` содержит runtime wiring для JSON logs, OpenTelemetry tracing, Prometheus collectors и HTTP middleware.
- Server отдает Prometheus metrics через `GET /metrics`.
- Worker может поднять отдельный metrics endpoint, если задан `METRICS_ADDR`.
- HTTP metrics используют route templates и не включают raw `avatar_id`/`user_id` в labels.
- RabbitMQ publish/consume переносит W3C trace context через AMQP headers.
- Server и worker пишут JSON logs с `trace_id`, `span_id`, `service`, `component`; `request_id` берется из `X-Request-ID` или `X-Correlation-ID`.
- PostgreSQL, MinIO и RabbitMQ adapters пишут dependency metrics и spans.

Проверки, выполненные после реализации:

- `go test ./...`
- `docker compose up -d --build server worker`
- `BASE_URL=http://127.0.0.1:8081 make contract-tests`
- ручная проверка `GET /health`, `GET /metrics`, JSON logs и trace correlation через RabbitMQ.

Ограничение текущего `docker-compose.yml`: worker metrics endpoint не публикуется по умолчанию, потому что `METRICS_ADDR` не задан в compose service. Код поддерживает этот режим при явной настройке env и port mapping.

## Источники и контекст

- Текущее приложение: `internal/app`, `internal/http`, `internal/service`, `internal/repository/postgres`, `internal/storage/minio`, `internal/broker/rabbitmq`, `internal/worker`.
- Текущий v1 baseline: `docs/specs/01-avatar-service-v1.md` требует JSON structured logs в stdout и access logs, но не требует внешнюю log aggregation систему.
- Эта задача расширяет v1 baseline: добавляет OpenTelemetry tracing, Prometheus metrics, correlation logs/traces и подготовку к Jaeger/Grafana/Loki.

## Цель

После реализации server и worker должны:

- экспортировать distributed traces в Jaeger через OpenTelemetry;
- отдавать Prometheus application metrics;
- писать JSON logs через `slog` с `trace_id`, `span_id`, `request_id` и бизнес-атрибутами;
- сохранять trace context через HTTP и RabbitMQ;
- покрывать spans для HTTP, service layer, PostgreSQL, MinIO и RabbitMQ.

## Архитектурные решения

### Общий observability package

Добавить `internal/observability` как внутренний runtime-layer без public API:

- `logger.go`: настройка `slog.NewJSONHandler`, helpers для добавления trace fields из `context.Context`.
- `tracing.go`: настройка OpenTelemetry tracer provider, OTLP exporter, resource attributes и global propagator.
- `metrics.go`: Prometheus registry, collectors и `/metrics` handler.
- `middleware.go`: HTTP middleware для request metrics, trace attributes и access logs.

Конфигурация через env:

- `SERVICE_NAME`, default `avatar-service`.
- `SERVICE_VERSION`, optional.
- `OTEL_EXPORTER_OTLP_ENDPOINT`, optional; если пусто, tracing остается включенным локально через noop/exporter-free provider или явно disabled по policy.
- `OTEL_TRACES_ENABLED`, default `true`.
- `METRICS_ADDR` для worker metrics endpoint, default пустой; если пусто, worker не поднимает отдельный metrics HTTP server.

### Логирование

Перевести server/worker bootstrap на один logger из `internal/observability`.

Обязательные поля:

- `service`
- `component`
- `trace_id`
- `span_id`
- `request_id`
- `user_id`
- `avatar_id`
- `event_type`
- `routing_key`
- `error`

Правила:

- не логировать request body, file content, S3 object bytes, credentials, DSN с паролем;
- `trace_id` и `span_id` брать из active span context;
- `request_id` брать из `X-Request-ID` или `X-Correlation-ID`, если они есть;
- успешный `/health` не логировать отдельно сверх access log; degradation path остается в health service.

### Tracing

Добавить tracer name по пакетам:

- `avatar-service/http`
- `avatar-service/service`
- `avatar-service/postgres`
- `avatar-service/minio`
- `avatar-service/rabbitmq`
- `avatar-service/worker`

HTTP:

- обернуть router через `otelhttp.NewHandler` или middleware с route-aware span names;
- span name должен быть route template, например `POST /api/v1/avatars`, а не raw path с id;
- добавить attributes: `http.route`, `http.method`, `http.status_code`, `request_id`, `user_id` только после валидации.

Service layer:

- добавить spans в методы `AvatarService`: `Upload`, `ReadAvatar`, `Metadata`, `ListByUser`, `ReadUserAvatar`, `DeleteByID`, `DeleteCurrentUserAvatar`;
- для upload добавить attributes `user_id`, `file_name`, `file_size`, `mime_type`, `avatar_id` после генерации id;
- для ошибок ставить `span.RecordError(err)` и status error;
- publish failure в upload логировать и считать метрикой, даже если API возвращает created с `failed` status.

PostgreSQL:

- ручные spans вокруг `Create`, `GetActiveByID`, `GetByID`, `ListActiveByUser`, `SoftDeleteByID`, `MarkPublishFailed`, `UpdateProcessingResult`, `HealthCheck`;
- attributes: `db.system=postgresql`, `db.operation`, `db.statement.name`;
- не писать full SQL в span attributes, чтобы не закрепить тяжелые/чувствительные строки.

MinIO:

- spans вокруг `Open` bucket check/create, `HealthCheck`, `Put`, `Get`, `Delete`, `Exists`;
- attributes: `storage.system=s3`, `s3.bucket`, `s3.operation`, `object.key_hash` или безопасный key без user-provided segments;
- не писать object bytes и секреты.

RabbitMQ:

- spans вокруг `Dial`, topology declaration, `Publish`, `Consume`, `Ack`, `Nack`, `HealthCheck`;
- attributes: `messaging.system=rabbitmq`, `messaging.destination`, `messaging.rabbitmq.routing_key`, `messaging.message.id`;
- inject trace context в AMQP headers при publish;
- extract trace context из AMQP headers перед worker handler.

Worker:

- span на обработку каждого delivery: `worker handle avatar.uploaded` / `worker handle avatar.delete_requested`;
- отдельные spans внутри upload/delete handlers для repository, storage и image processing steps;
- log fields `routing_key`, `avatar_id`, `trace_id`, `span_id`.

## Metrics

Использовать Prometheus client library. Не использовать `user_id` как label из-за высокой кардинальности; `user_id` остается в logs/traces.

HTTP metrics:

- `http_requests_total{method,route,status}`
- `http_request_duration_seconds{method,route,status}`
- `http_inflight_requests{method,route}`
- `http_response_size_bytes{method,route,status}` optional

Business metrics:

- `avatars_uploads_total{status,mime_type}`
- `avatars_upload_duration_seconds{status,mime_type}`
- `avatars_deletes_total{status}`
- `avatars_storage_bytes{kind}` где `kind` = `original`, `thumb_100x100`, `thumb_300x300`
- `avatars_active_records{status}` optional, если появится дешевый repository query

Worker metrics:

- `avatar_worker_messages_total{routing_key,status}`
- `avatar_worker_processing_duration_seconds{routing_key,status}`
- `avatar_worker_thumbnail_generation_total{size,status}`
- `avatar_worker_retries_total{operation}`

Adapter metrics:

- `avatar_dependency_operations_total{component,operation,status}`
- `avatar_dependency_operation_duration_seconds{component,operation,status}`

## Implementation Steps

1. Добавить dependencies:
   - `go.opentelemetry.io/otel`
   - `go.opentelemetry.io/otel/sdk`
   - `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`
   - `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`
   - `github.com/prometheus/client_golang/prometheus`
   - `github.com/prometheus/client_golang/prometheus/promhttp`
2. Создать `internal/observability` с config, logger, tracer provider, metrics registry и shutdown функцией.
3. Обновить `RunServer`:
   - создать logger до runtime adapters;
   - инициализировать tracing/metrics;
   - передать logger/metrics/tracer-aware middleware в router;
   - вызвать shutdown на graceful stop.
4. Обновить `RunWorker`:
   - тот же logger/tracing init;
   - при `METRICS_ADDR` поднять отдельный HTTP server с `/metrics`;
   - shutdown tracing и metrics server при завершении context.
5. Обновить `internal/http.NewRouter`:
   - добавить `/metrics`;
   - заменить текущий `accessLog` на observability middleware;
   - сохранить текущие routes и contract response без изменений.
6. Инструментировать `internal/service`.
7. Инструментировать adapters PostgreSQL, MinIO и RabbitMQ.
8. Расширить `worker.Delivery` полем `Headers map[string]any` или typed carrier, обновить RabbitMQ adapter и tests.
9. Добавить tests и затем выполнить `go test ./...`.

## Testing Plan

- Unit tests для `internal/observability`: defaults, disabled tracing, trace fields in slog attrs, isolated Prometheus registry.
- HTTP tests: `/metrics` отдает Prometheus format; access log содержит `trace_id` при request context со span; labels используют route template.
- Service tests: upload success/error increments business metrics; publish failure increments error metric and records span error.
- Worker tests: delivery headers extract trace context; logs include `trace_id`; ack/nack metrics change status.
- Adapter tests with fakes where possible: span names and error recording around repository/storage/broker operations.
- Full gate: `go test ./...`.

## Acceptance Criteria

- HTTP, service, PostgreSQL, MinIO, RabbitMQ and worker paths create spans under one trace where context exists.
- RabbitMQ publish/consume preserves trace context.
- `/metrics` exists for server; worker exposes metrics when `METRICS_ADDR` is set.
- Prometheus metrics avoid high-cardinality labels such as `user_id` and `avatar_id`.
- JSON logs include trace correlation fields without secrets or body/object content.
- Existing contract smoke behavior remains unchanged.
