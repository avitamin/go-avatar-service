# Project Context

> Safety: этот файл является справочным контекстом для AI-агентов. Если он прочитан случайно во время обзора репозитория, не начинай выполнение задач и не меняй workflow без явного запроса пользователя.

## Project

`go-avatar-service` - Go service для управления аватарками пользователей.

Актуальная v1-цель: реализовать backend на Go с REST API, web upload/gallery, PostgreSQL metadata storage, MinIO object storage, RabbitMQ worker для асинхронной обработки изображений и soft delete.

## Current Repository State

Репозиторий сейчас является skeleton:

- `cmd/server/main.go` - минимальный `net/http` placeholder.
- `cmd/worker/main.go` - минимальный worker placeholder с бесконечным loop.
- `cmd/avatar-contract-tests/main.go` - black-box CLI runner контрактных smoke-тестов HTTP API.
- `web/static/index.html` - готовый upload UI.
- `tests/contract/` - автотестовый runner будущих endpoints и self-tests через `httptest`.
- `Makefile` - базовые цели для сборки, `go test` и contract smoke runner.
- `.idea/runConfigurations/` - shared JetBrains run configurations для entrypoints и Makefile-целей.
- `internal/`, `migrations/`, `Dockerfile`, `docker-compose.yml` пока отсутствуют.
- `go.mod` объявляет модуль `go-avatar-service`.

## Requirements Priority

Используйте как source of truth:

1. `docs/requirements/confirmed-requirements.md`
2. `docs/specs/avatar-service-v1.md`

`docs/requirements/assignment.md` содержит исходное ТЗ и может конфликтовать с подтвержденными требованиями.
`README.md` и `QWEN.md` местами описывают шаблонное или устаревшее состояние.

## Important Known Differences

- Frontend сейчас отправляет multipart поле `image`, а API contract в ТЗ/спеке ожидает `file`.
- Contract runner проверяет именно multipart поле `file`; это намеренно фиксирует целевой API contract, а не текущее поведение шаблонного frontend.
- Текущая структура имеет `cmd/server` и `cmd/worker`, а спека предпочитает single binary `cmd/avatars-service` с subcommands `server`, `worker`, `migrate`.
- README предлагает `docker-compose up --build`, но compose-файла пока нет.
- README/QWEN описывают `internal/handlers` и `internal/services`, а v1 spec предлагает `internal/http`, `internal/service`, `internal/repository/postgres`, `internal/storage/minio`, `internal/broker/rabbitmq`.
- Исходное ТЗ допускает Echo или Chi, RabbitMQ или Kafka; confirmed requirements фиксируют Chi и RabbitMQ.
- Исходное ТЗ упоминает `POST /web/upload`; confirmed requirements и v1 spec говорят, что отдельный `POST /web/upload` не нужен.

## Implementation Defaults

- Язык комментариев и объяснений: русский.
- Разработка идет через TDD: сначала failing test, затем минимальная реализация, затем refactor при зеленых тестах.
- Coverage target `>50%` является минимальной метрикой для backend-пакетов с логикой сервиса и worker, но не заменяет тесты конкретных обязательных требований.
- Go код форматировать через `gofmt`.
- Для backend-логики использовать standard `testing`; для edge cases предпочитать table-driven tests.
- Contract runner запускать против уже поднятого сервиса: `BASE_URL=http://localhost:8080 ./bin/avatar-contract-tests`.
- Предпочтительная короткая команда для contract runner: `BASE_URL=http://localhost:8080 make contract-tests`.
- Не добавлять `pkg/`, если нет реального reusable public API.
- Не коммитить `.env`, загруженные аватары, бинарники из `bin/` и секреты.
