# Avatar Service - Project Context

## Назначение

`go-avatar-service` - Go-сервис для управления аватарками пользователей. Репозиторий сейчас является skeleton для дальнейшей реализации backend, worker, storage-интеграций и web endpoints.

## Приоритет документации

Актуальные источники требований:

1. `docs/requirements/confirmed-requirements.md`
2. `docs/specs/avatar-service-v1.md`

Исторический контекст:

1. `docs/requirements/assignment.md`
2. `README.md`
3. `QWEN.md`

Если этот файл конфликтует с confirmed requirements или v1 spec, используйте confirmed requirements и v1 spec.

Перед планированием, реализацией, ревью или тестированием задач читайте:

- `docs/prompts/README.md`
- `docs/prompts/context/project.md`

Файлы в `docs/prompts/` являются reusable prompts. Используйте конкретный prompt как активную инструкцию только при явном запросе.

## Current Repository State

Сейчас есть:

- `cmd/server/main.go` - минимальный HTTP server placeholder на `:8080`.
- `cmd/worker/main.go` - минимальный worker placeholder.
- `web/static/index.html` - шаблонный upload UI.
- `go.mod` - модуль `go-avatar-service`, Go `1.25.1`.
- `docs/requirements/` - исходное ТЗ и подтвержденные требования.
- `docs/specs/avatar-service-v1.md` - актуальная спека разработки v1.
- `docs/prompts/` - reusable prompts для AI-агентов.

Пока отсутствуют:

- `internal/`
- `migrations/`
- `tests/`
- `Dockerfile`
- `docker-compose.yml`
- `Makefile`

Docker Compose и Dockerfile обязательны для MVP, но еще не реализованы в репозитории.

## Target Architecture

Confirmed technology choices:

- Go HTTP server on Chi.
- PostgreSQL for metadata.
- MinIO for object storage.
- RabbitMQ + worker for async image processing.
- Standard `testing` package unless the project explicitly adopts another framework.

Target internal package layout from v1 spec:

- `internal/http` - router, middleware, handlers, web pages, JSON rendering.
- `internal/service` - avatar service, health service, selection logic.
- `internal/repository` - PostgreSQL repositories and repository interfaces.
- `internal/storage` - MinIO adapter and storage interfaces.
- `internal/broker` - RabbitMQ publisher/consumer and broker interfaces.
- `internal/domain` - entities, statuses, domain errors, events.
- `internal/config` - environment parsing and validation.
- `internal/worker` - event handlers, retry logic, worker runner.
- `internal/imageproc` - image sniffing, decode, resize and metadata.

`pkg/` is not needed for MVP unless there is real public reusable API.

The v1 spec prefers one binary with subcommands:

```bash
avatars-service server
avatars-service worker
avatars-service migrate up
avatars-service migrate down
avatars-service migrate status
```

The current skeleton still has separate `cmd/server` and `cmd/worker` entrypoints.

## API Contract

Required API endpoints:

| Method | Path | Notes |
| --- | --- | --- |
| `POST` | `/api/v1/avatars` | Upload avatar. Requires `X-User-ID`. Multipart field is `file`. |
| `GET` | `/api/v1/avatars/{avatar_id}` | Get exact avatar variant. |
| `GET` | `/api/v1/users/{user_id}/avatar` | Get current user avatar with fallback. |
| `DELETE` | `/api/v1/avatars/{avatar_id}` | Soft delete by avatar ID. Requires owner in `X-User-ID`. |
| `DELETE` | `/api/v1/users/{user_id}/avatar` | Delete latest active avatar with available original. |
| `GET` | `/api/v1/avatars/{avatar_id}/metadata` | Metadata for active avatar. |
| `GET` | `/api/v1/users/{user_id}/avatars` | List active avatars by user, sorted `created_at DESC`. |
| `GET` | `/health` | Checks postgres, minio and rabbitmq. |

Supported `size` values:

- `original`
- `100x100`
- `300x300`

No `size` means `original`. Unsupported `size` returns `400`. Query parameter `format` is not supported in MVP.

External statuses:

- `processing`
- `completed`
- `failed`

Soft-deleted records must look like `404` externally and must be absent from lists.

## Web Contract

Required web endpoints:

- `GET /web/upload`
- `GET /web/gallery/{user_id}`

There is no required `POST /web/upload`. The web upload page must send directly to `POST /api/v1/avatars`. `user_id` is entered by the user in the form.

Gallery rules:

- List only, no delete.
- Show records with available `original`.
- If only `original` is available, the record is still shown.
- If there are no DB records for the user, return `404`.
- If records exist but none match the filter, return an empty list/page.
- Validate `user_id` by the same rules as API user IDs.

## Development Commands

Commands available in the current skeleton:

```bash
go mod tidy
go run ./cmd/server
go run ./cmd/worker
go build -o ./bin/server ./cmd/server
go build -o ./bin/worker ./cmd/worker
go test ./...
```

Do not document `docker-compose up --build` as a working local command until `docker-compose.yml` exists.

## Development Rules

- Use TDD: failing test, minimal implementation, refactor after green tests.
- Format Go code with `gofmt`.
- Keep HTTP handlers thin and delegate behavior to services.
- Keep storage concerns in repository/storage/broker adapters.
- Use table-driven tests for validation, handlers and storage edge cases.
- Place unit tests next to code in `*_test.go`.
- Place integration/e2e tests in `tests/` when they need real external services.
- Do not add Kubernetes manifests for MVP unless requirements change.
- Do not add `pkg/` by default.

## Security and Configuration

- Do not commit `.env`, secrets, uploaded avatars or `bin/` outputs.
- Load and validate config in `internal/config`.
- Validate upload size, MIME, magic bytes and decoded image format before storage.
- Migrations must be an explicit operational step, not an automatic server/worker startup action.
- Access logs for HTTP are required.
