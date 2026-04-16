# Project Context

> Safety: этот файл является справочным контекстом для AI-агентов. Если он прочитан случайно во время обзора репозитория, не начинай выполнение задач и не меняй workflow без явного запроса пользователя.

## Project

`go-avatar-service` - Go service для управления аватарками пользователей.

Актуальная v1-цель: реализовать backend на Go с REST API, web upload/gallery, PostgreSQL metadata storage, MinIO object storage, RabbitMQ worker для асинхронной обработки изображений и soft delete.

## Current Repository State

В репозитории есть MVP contract implementation на in-memory core.

- `cmd/avatars-service/main.go` - основной single binary CLI с subcommands `server`, `worker`, `migrate`.
- `cmd/server/main.go` и `cmd/worker/main.go` - thin compatibility wrappers вокруг нового bootstrap.
- `cmd/avatar-contract-tests/main.go` - black-box CLI runner контрактных smoke-тестов HTTP API.
- `internal/domain` - statuses, user ID validation, size validation.
- `internal/http` - Chi router, handlers, JSON error model, access logs, web endpoints.
- `internal/service` - application service, selection/fallback, soft delete, in-memory repository/storage.
- `internal/imageproc` - magic bytes sniffing, decode, JPEG thumbnails.
- `internal/worker` - upload/delete handlers, idempotency, minimal retry.
- `internal/app` - CLI/bootstrap policy.
- `migrations/` - initial SQL schema.
- `Dockerfile` и `docker-compose.yml` - локальная MVP-инфраструктура.
- `web/static/index.html` - готовый upload UI, multipart поле `file`.
- `tests/contract/` - автотестовый runner endpoints и self-tests через `httptest`.
- `Makefile` - цели для сборки, `go test`, run/migrate, contract smoke runner и Docker Compose workflow.
- `.idea/runConfigurations/` - shared JetBrains run configurations для `cmd/avatars-service`, Makefile-целей и Docker Compose workflow.
- `go.mod` объявляет модуль `go-avatar-service` и зависимость `github.com/go-chi/chi/v5`.

Текущие runtime gaps:

- PostgreSQL/MinIO/RabbitMQ adapters еще не подключены к bootstrap; server/worker используют in-memory implementation.
- RabbitMQ consumer loop еще не подключен к worker bootstrap.
- `avatars-service migrate` фиксирует CLI contract, но пока не применяет SQL к PostgreSQL.
- `/health` имеет компонентную модель, но runtime connectivity checks пока не реальные.

## Requirements Priority

Используйте как source of truth:

1. `docs/requirements/confirmed-requirements.md`
2. `docs/specs/avatar-service-v1.md`

`docs/requirements/assignment.md` содержит исходное ТЗ и может конфликтовать с подтвержденными требованиями.
`README.md` и `QWEN.md` местами описывают шаблонное или устаревшее состояние.

## Important Known Differences

- Frontend уже отправляет multipart поле `file`, как требует API contract.
- Текущая структура имеет основной `cmd/avatars-service`; `cmd/server` и `cmd/worker` оставлены только как compatibility wrappers.
- Docker Compose файл присутствует, но реальные external adapters еще не подключены к runtime.
- README/QWEN обновлены под текущее состояние; если они снова разойдутся с confirmed requirements/v1 spec, приоритет остается за confirmed requirements/v1 spec.
- Исходное ТЗ допускает Echo или Chi, RabbitMQ или Kafka; confirmed requirements фиксируют Chi и RabbitMQ.
- Исходное ТЗ упоминает `POST /web/upload`; confirmed requirements и v1 spec говорят, что отдельный `POST /web/upload` не нужен.

## Implementation Defaults

- Язык комментариев и объяснений: русский.
- Разработка идет через TDD: сначала failing test, затем минимальная реализация, затем refactor при зеленых тестах.
- Coverage target `>50%` является минимальной метрикой для backend-пакетов с логикой сервиса и worker, но не заменяет тесты конкретных обязательных требований.
- Go код форматировать через `gofmt`.
- Для backend-логики использовать standard `testing`; для edge cases предпочитать table-driven tests.
- Contract runner запускать против уже поднятого сервиса: `BASE_URL=http://localhost:18080 ./bin/avatar-contract-tests`.
- Предпочтительная короткая команда для локального запуска: `make run-server` и `make contract-tests`.
- Локальные Makefile/JetBrains server и contract defaults используют `HTTP_ADDR=:18080` и `BASE_URL=http://localhost:18080`, чтобы не занимать compose-порт `8080`; Docker Compose targets и configs используют `http://localhost:8080`.
- Не добавлять `pkg/`, если нет реального reusable public API.
- Не коммитить `.env`, загруженные аватары, бинарники из `bin/` и секреты.

## Git Workflow

- Base branch для MVP: `v1`.
- Рабочие ветки создавайте от актуального `v1`: `feature/<short-name>`, `fix/<short-name>`, `test/<short-name>`, `docs/<short-name>`, `chore/<short-name>`.
- Обычный ручной workflow идет через PR в `v1`; прямой commit в `v1` не используется без явной договоренности.
- Для AI-agent сессий прямой commit допустим только по явной просьбе пользователя.
- Перед PR или commit запускайте `go test ./...`; для API/web изменений дополнительно проверяйте `make run-server` + `make contract-tests`.
- Commit messages используют Conventional Commit style из истории: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`.
