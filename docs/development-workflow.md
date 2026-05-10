# Development Workflow

Этот документ - основной источник для локального developer workflow в `go-avatar-service`: базовые команды, local vs Docker Compose запуск, переопределение портов и вспомогательные инструменты.

## Quick Start

Минимальный локальный цикл:

```bash
go test ./...
make run-server
make contract-tests
```

Минимальный Docker Compose цикл:

```bash
make docker-up-detached
docker compose run --rm server migrate up
make docker-contract-tests
```

Локальный `make run-server` по умолчанию слушает `http://localhost:18080`. Docker Compose публикует server на `http://localhost:8080`.

Быстрая проверка observability для любого запущенного server:

```bash
curl -fsS "$BASE_URL/health"
curl -fsS "$BASE_URL/metrics"
curl -fsS -H 'X-Request-ID: manual-observability-check' "$BASE_URL/health"
```

## Local Development

Базовые команды:

```bash
go mod tidy
go test ./...
go test ./internal/... -cover
go test -run='^$' -bench=. -benchmem ./...
go build ./cmd/avatars-service ./cmd/server ./cmd/worker ./cmd/avatar-contract-tests
```

Основные `Makefile` targets:

```bash
make test
make bench
make build
make build-server
make build-worker
make build-contract-tests
make run-server
make run-worker
make contract-tests
make migrate-up
make migrate-down
make migrate-status
```

Локальные defaults:

- `make run-server` использует `HTTP_ADDR=:18080`.
- `make contract-tests` использует `BASE_URL=http://localhost:18080`.

Переопределение local порта:

```bash
make run-server HTTP_ADDR=:18081
make contract-tests BASE_URL=http://localhost:18081
```

Contract smoke runner можно запускать и напрямую:

```bash
BASE_URL=http://localhost:18080 go run ./cmd/avatar-contract-tests
BASE_URL=http://localhost:18080 make contract-tests
```

Exit codes contract runner:

- `0` - все сценарии прошли.
- `1` - есть проваленные контрактные сценарии.
- `2` - runner запущен с неверной конфигурацией, например без `BASE_URL`.

## Docker Compose Workflow

Локальный compose stack поднимает:

- `server`
- `worker`
- `postgres`
- `rabbitmq`
- `minio`

Compose flow:

```bash
make docker-build
make docker-up-build
make docker-up-detached
make docker-down
make docker-ps
make docker-logs
make docker-contract-tests
```

Базовый сценарий:

```bash
make docker-up-detached
docker compose run --rm server migrate up
make docker-contract-tests
```

Детали runtime:

- Compose server слушает контейнерный `:8080` и публикуется как `http://localhost:8080`.
- `server` и `worker` используют реальные adapters при заданных `POSTGRES_DSN`, `MINIO_*` и `RABBITMQ_URL`.
- Миграции не запускаются автоматически при старте процессов и остаются отдельным явным шагом.
- `docker compose up` гарантирует только start order, не readiness dependency-сервисов.

Если `server` или `worker` упали из-за раннего `connect: connection refused`, после готовности зависимостей достаточно повторить:

```bash
docker compose up -d server worker
```

После `make docker-contract-tests` можно проверить Prometheus metrics на server:

```bash
curl -fsS http://localhost:8080/metrics | rg '^(http_requests_total|avatars_uploads_total|avatars_deletes_total|avatar_dependency_operations_total)'
```

Проверка JSON access logs и trace correlation:

```bash
curl -fsS -H 'X-Request-ID: manual-observability-check' http://localhost:8080/health
docker compose logs --tail=40 server worker
```

Ожидаемые признаки:

- `server` logs содержат `service`, `component`, `trace_id`, `span_id`, `route`, `status`; для запроса выше также `request_id`.
- `worker` logs для thumbnail/delete событий содержат `trace_id` и `span_id`.
- Для upload/delete через RabbitMQ `trace_id` в worker log совпадает с `trace_id` исходного server log.
- HTTP metrics используют route templates, например `/api/v1/avatars/{avatar_id}`, без raw `avatar_id` или `user_id` в labels.

## Ports and Overrides

Текущие defaults:

| Назначение | Local default | Compose host default |
| --- | --- | --- |
| HTTP server | `18080` | `8080` |
| PostgreSQL | - | `5432` |
| RabbitMQ AMQP | - | `5672` |
| RabbitMQ Management | - | `15672` |
| MinIO API | - | `9000` |
| MinIO Console | - | `9001` |

Локальный workflow и Compose intentionally разведены по HTTP-портам: `18080` для local run configs и `8080` для Docker Compose.

Host-порты Compose читаются из `.env`, если файл существует. Шаблон defaults хранится в `.env.example`, сам `.env` не коммитится.

Пример ручного override одного порта:

```bash
cp .env.example .env
printf 'COMPOSE_MINIO_CONSOLE_PORT=19001\n' >> .env
make docker-up-detached
```

Пример override всего compose-набора:

```bash
cp .env.example .env
cat >> .env <<'EOF'
COMPOSE_HTTP_PORT=18081
COMPOSE_POSTGRES_PORT=15432
COMPOSE_RABBITMQ_PORT=15673
COMPOSE_RABBITMQ_MANAGEMENT_PORT=15674
COMPOSE_MINIO_API_PORT=19000
COMPOSE_MINIO_CONSOLE_PORT=19001
EOF
make docker-up-detached
docker compose run --rm server migrate up
make docker-contract-tests
```

## Free Port Helper

Для подбора незанятых host-портов используйте:

```bash
scripts/find-free-ports.sh
```

Скрипт печатает готовые `KEY=value` assignments:

- `LOCAL_HTTP_PORT`
- `LOCAL_HTTP_ADDR`
- `LOCAL_BASE_URL`
- `COMPOSE_HTTP_PORT`
- `COMPOSE_POSTGRES_PORT`
- `COMPOSE_RABBITMQ_PORT`
- `COMPOSE_RABBITMQ_MANAGEMENT_PORT`
- `COMPOSE_MINIO_API_PORT`
- `COMPOSE_MINIO_CONSOLE_PORT`

Если preferred default свободен, скрипт оставляет его. Если занят, выбирает следующий свободный TCP port вверх по диапазону и не переиспользует уже выбранные значения в рамках одного запуска.

Примеры:

```bash
scripts/find-free-ports.sh
scripts/find-free-ports.sh > .env
```

Локальный server/contract flow можно запускать с выведенными значениями так:

```bash
eval "$(scripts/find-free-ports.sh)"
make run-server HTTP_ADDR="$LOCAL_HTTP_ADDR"
make contract-tests BASE_URL="$LOCAL_BASE_URL"
```

Для Compose `.env` понадобится только compose-префикс. Если вы перенаправляете полный вывод скрипта в `.env`, строки `LOCAL_*` будут проигнорированы `docker compose`, но останутся доступны для shell tooling.

## Конфигурация observability

Runtime observability env vars:

| Env var | Default | Используется | Назначение |
| --- | --- | --- | --- |
| `SERVICE_NAME` | `avatar-service` | `server`, `worker` | `service` field в logs и OpenTelemetry resource attribute |
| `SERVICE_VERSION` | empty | `server`, `worker` | optional OpenTelemetry resource attribute |
| `OTEL_TRACES_ENABLED` | `true` | `server`, `worker` | включает создание spans |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | empty | `server`, `worker` | OTLP/gRPC endpoint; если пусто, spans не экспортируются |
| `METRICS_ADDR` | empty | `worker` | адрес отдельного worker metrics HTTP server |

Server metrics всегда доступны на основном HTTP server endpoint `GET /metrics`.

Worker metrics поднимаются только если задан `METRICS_ADDR`. Текущий `docker-compose.yml` не публикует worker metrics port по умолчанию; для проверки worker metrics добавьте `METRICS_ADDR` и port mapping в локальный compose override или временно запустите worker вне Compose с нужным env.

Prometheus metric groups:

- HTTP: `http_requests_total`, `http_request_duration_seconds`, `http_inflight_requests`.
- Business: `avatars_uploads_total`, `avatars_upload_duration_seconds`, `avatars_deletes_total`, `avatars_storage_bytes`.
- Worker: `avatar_worker_messages_total`, `avatar_worker_processing_duration_seconds`, `avatar_worker_thumbnail_generation_total`.
- Dependencies: `avatar_dependency_operations_total`, `avatar_dependency_operation_duration_seconds`.

Label policy:

- Use bounded labels such as `method`, `route`, `status`, `component`, `operation`, `routing_key`, `mime_type`, and thumbnail `size`.
- Do not use `user_id`, `avatar_id`, raw object keys, request bodies, file bytes, credentials, or DSNs in metric labels or logs.

## JetBrains Run Configurations

В `.idea/runConfigurations/` сохранены shared конфигурации:

- `Server`
- `Worker`
- `Avatar Contract Tests`
- `Make Test`
- `Make Build Contract Tests`
- `Make Contract Tests`
- `Make Docker Build`
- `Make Docker Up Build`
- `Make Docker Up Detached`
- `Make Docker Down`
- `Make Docker Ps`
- `Make Docker Logs`
- `Make Docker Contract Tests`

Локальные server/contract конфигурации используют `http://localhost:18080`. Docker Compose конфигурации используют compose URL `http://localhost:8080`.

## Related Docs

- [README.md](../README.md) - быстрый обзор репозитория и стартовые команды.
- [docs/benchmarking.md](./benchmarking.md) - benchmark workflow и triage.
- [docs/repo-documentation-guide.md](./repo-documentation-guide.md) - правила ведения документации.
