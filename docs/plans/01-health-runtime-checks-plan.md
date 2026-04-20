# Детальный план исправления `/health`

## Что использовать как source of truth

- Поведение `/health`: [docs/requirements/confirmed-requirements.md](../requirements/confirmed-requirements.md)
  - взять требования:
    - `/health` проверяет `postgres`, `minio`, `rabbitmq`
    - возвращает общий статус и статусы компонентов
    - при частичной деградации HTTP status остаётся `200`, а `status=degraded`
- Архитектурный контекст и success criteria: [docs/specs/01-avatar-service-v1.md](../specs/01-avatar-service-v1.md)
  - взять требования:
    - `/health` должен отражать degraded state без перехода в `5xx`
    - `internal/service` допускает отдельный `health service`
- Зафиксированная целевая правка: [docs/specs/02-health-runtime-checks.md](../specs/02-health-runtime-checks.md)

## Что взять из текущей реализации

- Текущее server wiring: `internal/app/app.go`
  - взять:
    - `RunServer`
    - `newStoreFromEnv`
    - `newBrokerFromEnv`
    - `logBroker`
- Текущий HTTP handler `/health`: `internal/http/router.go`
  - взять:
    - `HealthService`
    - `NewRouter`
    - `handler.health`
    - helper `status`
- Текущие HTTP tests: `internal/http/router_test.go`
  - взять как базу для обновления сценариев `/health`
- Текущие contract tests: `tests/contract/scenarios.go`
  - взять как базу для сохранения smoke coverage `/health`

## Implementation Steps

### 1. Ввести runtime health service в `internal/service`

- Создать новый файл `internal/service/health_service.go`.
- В этом файле определить:
  - type для общего health snapshot
  - type для статуса компонента
  - интерфейс или concrete service, который умеет по `context.Context` вернуть текущий health snapshot
- В snapshot зафиксировать:
  - top-level `Status string`
  - component statuses для `postgres`, `minio`, `rabbitmq`
- Допустимые значения статусов:
  - `ok`
  - `degraded`
- В этом же файле собрать логику вычисления общего статуса:
  - если любой компонент не `ok`, общий статус = `degraded`
  - иначе общий статус = `ok`

### 2. Определить, как health service узнаёт реальный runtime mode

- Источник истины для выбора adapters взять из `internal/app/app.go`, не придумывать отдельную конфигурацию.
- Перестроить bootstrap так, чтобы после `newStoreFromEnv` и `newBrokerFromEnv` можно было понять:
  - используется реальный Postgres adapter или memory repository
  - используется реальный MinIO adapter или memory storage
  - используется реальный RabbitMQ adapter или `logBroker`
- Для этого в `internal/app/app.go` добавить явную health-конфигурацию runtime, которая строится в момент bootstrap.
- Не пытаться определять типы по строкам или логам; источник должен быть явным и кодовым:
  - либо через новый struct c признаками runtime mode
  - либо через отдельные wrappers/constructors, которые возвращают и adapter, и metadata о нём

### 3. Добавить per-request connectivity checks

- Runtime checks реализовать в `internal/service/health_service.go`.
- Для каждого компонента сделать отдельный check method:
  - `postgres`
  - `minio`
  - `rabbitmq`
- Логика check:
  - если в bootstrap выбран fallback/noop adapter, check не выполнять, сразу вернуть `degraded`
  - если выбран реальный adapter, выполнить короткий runtime check
- Источники для check:
  - `postgres`: использовать объект, который реально умеет ping/query текущего Postgres adapter
  - `minio`: использовать объект, который реально умеет lightweight existence/access check к MinIO
  - `rabbitmq`: использовать объект, который реально умеет lightweight connectivity check к RabbitMQ
- Не дублировать bootstrap-логику внутри HTTP handler.
- Не открывать новые подключения на каждый `/health`, если уже есть активный adapter и его можно проверить через существующий handle.

### 4. Зафиксировать timeout policy

- В `internal/service/health_service.go` задать короткий timeout для каждого component check.
- Timeout брать из `context.WithTimeout` внутри health service.
- Если check вернул timeout или любую ошибку:
  - component status = `degraded`
  - общий статус = `degraded`
- Не поднимать HTTP error и не переводить `/health` в `5xx`.

### 5. Перевести HTTP слой на новый health service

- Изменить `internal/http/router.go`.
- Удалить зависимость `NewRouter` от bool-based `HealthService`.
- Вместо этого `NewRouter` должен принимать health service dependency.
- Обновить `handler`:
  - убрать хранение bool-структуры
  - хранить runtime health service
- Обновить `handler.health`:
  - вызывать health service на каждый запрос
  - сериализовать его snapshot в текущий JSON response
- Сохранить совместимость contract response:
  - top-level `status` обязателен
  - `postgres`, `minio`, `rabbitmq` должны оставаться доступны в response body
  - если используется вложенный `components`, не ломать текущую smoke-проверку

### 6. Обновить server bootstrap

- Изменить `RunServer` в `internal/app/app.go`.
- Вместо `httpapi.HealthService{Postgres: true, Minio: true, RabbitMQ: true}` собрать runtime health service из реально выбранных adapters.
- Источник данных:
  - результат `newStoreFromEnv`
  - результат `newBrokerFromEnv`
  - metadata о том, какой adapter был выбран
- Передать собранный health service в `httpapi.NewRouter`.

### 7. Добавить logging только на degradation path

- Логи health checks держать внутри health service.
- Логировать:
  - timeout check
  - failed connectivity check
  - fallback/noop mode как причину `degraded`
- Не логировать каждый successful `/health`, чтобы не засорять access path.

## Testing Plan

### 1. Unit tests для `internal/service/health_service.go`

- Создать `internal/service/health_service_test.go`.
- Покрыть сценарии:
  - все три компонента доступны -> snapshot `status=ok`
  - Postgres unavailable -> `postgres=degraded`, общий `status=degraded`
  - MinIO unavailable -> `minio=degraded`, общий `status=degraded`
  - RabbitMQ unavailable -> `rabbitmq=degraded`, общий `status=degraded`
  - memory repository/storage -> соответствующие компоненты `degraded`
  - `logBroker`/noop broker -> `rabbitmq=degraded`
  - timeout любого check -> `degraded`

### 2. HTTP tests в `internal/http/router_test.go`

- Обновить тесты так, чтобы `NewRouter` работал с новым health service dependency.
- Добавить сценарии:
  - `/health` возвращает `200`
  - body содержит `status`
  - body содержит `postgres`, `minio`, `rabbitmq`
  - при одном degraded компоненте body содержит `status=degraded`
- Не ограничиваться только happy-path router тестом.

### 3. Bootstrap tests для `internal/app`

- Добавить новый test file в `internal/app`, например `app_health_test.go`.
- Проверить wiring:
  - без `POSTGRES_DSN`, `MINIO_*`, `RABBITMQ_URL` server health не полностью healthy
  - при fallback store `postgres` и `minio` становятся `degraded`
  - при отсутствии `RABBITMQ_URL` `rabbitmq` становится `degraded`
- Цель этих тестов:
  - поймать регрессию, где bootstrap снова передаёт константные `true/true/true`

### 4. Contract tests в `tests/contract/scenarios.go`

- Сохранить обязательные проверки:
  - `/health` отвечает `200`
  - `status` присутствует
  - `postgres`, `minio`, `rabbitmq` присутствуют
- Не ужесточать contract до `status=ok`, потому что после исправления допустим `degraded`.

## Acceptance Criteria

- `/health` больше не зависит от статической bool-модели.
- При fallback/noop runtime `/health` не сообщает полностью healthy response.
- При частичной деградации `/health` отвечает `200` и `status=degraded`.
- Response содержит статусы `postgres`, `minio`, `rabbitmq`.
- Есть unit coverage health service, HTTP coverage `/health` и bootstrap coverage wiring.
