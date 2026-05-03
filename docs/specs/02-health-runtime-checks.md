# Runtime checks `/health`

## Источники информации

- Основные требования: [docs/requirements/confirmed-requirements.md](../requirements/confirmed-requirements.md)
- Базовая архитектурная спека: [docs/specs/01-avatar-service-v1.md](./01-avatar-service-v1.md)
- Текущее wiring и bootstrap: `internal/app/app.go`
- Текущий HTTP handler `/health`: `internal/http/router.go`
- Текущие HTTP tests: `internal/http/router_test.go`
- Текущие contract tests: `tests/contract/scenarios.go`

## Summary

Текущая реализация `/health` выполняет runtime connectivity checks для `postgres`, `minio`, `rabbitmq` на каждый запрос и не сообщает ложный `ok`, когда сервис работает на fallback/noop adapters или dependency недоступна.

## Current Behavior

- `internal/service/health_service.go` содержит отдельный runtime health service.
- `internal/app/app.go` собирает runtime probes из реально выбранных adapters и передает их в HTTP layer.
- `GET /health` синхронно выполняет checks для `postgres`, `minio`, `rabbitmq`.
- Если компонент не сконфигурирован, использует fallback/noop adapter, не проходит check или уходит в timeout, его статус становится `degraded`.
- Если хотя бы один компонент `degraded`, общий `status` тоже `degraded`.
- HTTP status `/health` при частичной деградации остается `200`.
- Success-path `/health` отдельно не логируется; логируется только деградация и ошибки checks.

## Runtime Probes

- `postgres`: `internal/repository/postgres.Repository.HealthCheck(ctx)` использует `db.PingContext`.
- `minio`: `internal/storage/minio.Storage.HealthCheck(ctx)` проверяет доступность bucket через `BucketExists`.
- `rabbitmq`: `internal/broker/rabbitmq.Client.HealthCheck(ctx)` открывает временный channel от существующего connection и делает passive check exchange `avatars`.
- Для fallback `MemoryRepository`/`MemoryStorage` и noop `logBroker` runtime check не выполняется; компонент сразу считается `degraded`.
- Per-component timeout задается внутри health service через `context.WithTimeout`; текущее значение в bootstrap равно `500ms`.

## Response Contract

- Сохранить top-level поле `status`.
- Компонентные статусы возвращаются внутри nested `components` для `postgres`, `minio`, `rabbitmq`.
- Допустимые значения статусов: `ok`, `degraded`.
- Не ломать текущий contract smoke coverage для `/health`.

## Verified Coverage

- `internal/service/health_service_test.go` покрывает healthy path, component failure, fallback/noop probes и timeout.
- `internal/http/router_test.go` проверяет `200`, top-level `status` и nested `components`.
- `internal/app/app_health_test.go` фиксирует, что bootstrap в fallback-режиме не отдает fully healthy snapshot.
- `tests/contract/scenarios.go` сохраняет smoke coverage наличия `status` и трех component keys.

## Assumptions

- `/health` остаётся lightweight operational endpoint.
- Checks выполняются синхронно на каждый запрос с короткими timeout'ами.
- Изменение не должно менять контракт остальных API endpoints.
