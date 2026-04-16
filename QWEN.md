# Avatar Service - Project Context

## Назначение

`go-avatar-service` - Go-сервис для управления аватарками пользователей. Текущая кодовая база содержит MVP contract implementation на in-memory core: HTTP API на Chi, web upload/gallery, worker handlers, CLI bootstrap, SQL migration files и Docker Compose.

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

## Current Repository State

Сейчас есть:

- `cmd/avatars-service/main.go` - основной single binary CLI.
- `cmd/server/main.go`, `cmd/worker/main.go` - thin compatibility wrappers.
- `cmd/avatar-contract-tests/main.go` - black-box contract smoke runner.
- `internal/app` - CLI/bootstrap.
- `internal/domain` - statuses, size и user ID validation.
- `internal/http` - Chi router, handlers, JSON errors, access logs, web pages.
- `internal/service` - application service, in-memory repository/storage, selection/fallback, soft delete.
- `internal/imageproc` - image sniff/decode/thumbnail helpers.
- `internal/worker` - upload/delete handlers, retry, idempotency.
- `migrations/` - initial SQL schema files.
- `Dockerfile` и `docker-compose.yml`.
- `web/static/index.html` - upload UI с multipart field `file`.
- `tests/contract/` - contract runner и self-tests.
- `Makefile` - build/test/run/migrate targets.

Current runtime gaps:

- PostgreSQL/MinIO/RabbitMQ adapters еще не подключены; server/worker используют in-memory implementation.
- RabbitMQ consumer loop еще не подключен к worker bootstrap.
- `avatars-service migrate` фиксирует CLI contract, но пока не применяет SQL к PostgreSQL.
- `/health` возвращает компонентную модель, но runtime connectivity checks пока не реальные.

## Target Architecture

Confirmed technology choices:

- Go HTTP server on Chi.
- PostgreSQL for metadata.
- MinIO for object storage.
- RabbitMQ + worker for async image processing.
- Standard `testing` package unless the project explicitly adopts another framework.

Target package additions for future adapters:

- `internal/repository/postgres`
- `internal/storage/minio`
- `internal/broker/rabbitmq`
- `internal/config`

`pkg/` is not needed for MVP unless there is real public reusable API.

## CLI Contract

```bash
avatars-service server
avatars-service worker
avatars-service migrate up
avatars-service migrate down
avatars-service migrate status
```

Migrations are an explicit operational step and must not be auto-run by `server` or `worker`.

## API Contract

| Method | Path | Notes |
| --- | --- | --- |
| `POST` | `/api/v1/avatars` | Upload avatar. Requires `X-User-ID`. Multipart field is `file`. |
| `GET` | `/api/v1/avatars/{avatar_id}` | Get exact avatar variant. |
| `GET` | `/api/v1/users/{user_id}/avatar` | Get current user avatar with fallback. |
| `DELETE` | `/api/v1/avatars/{avatar_id}` | Soft delete by avatar ID. Requires owner in `X-User-ID`. |
| `DELETE` | `/api/v1/users/{user_id}/avatar` | Delete latest active avatar with available original. |
| `GET` | `/api/v1/avatars/{avatar_id}/metadata` | Metadata for active avatar. |
| `GET` | `/api/v1/users/{user_id}/avatars` | List active avatars by user, sorted `created_at DESC`. |
| `GET` | `/health` | Component health response. |

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

There is no required `POST /web/upload`. The web upload page sends directly to `POST /api/v1/avatars` with multipart field `file`.

Gallery rules:

- List only, no delete.
- Show records with available `original`.
- If only `original` is available, the record is still shown.
- If there are no DB records for the user, return `404`.
- If records exist but none match the filter, return an empty list/page.
- Validate `user_id` by the same rules as API user IDs.

## Development Commands

```bash
go mod tidy
go test ./...
go test ./internal/... -cover
go build ./cmd/avatars-service ./cmd/server ./cmd/worker ./cmd/avatar-contract-tests
go run ./cmd/avatars-service server
go run ./cmd/avatars-service worker
go run ./cmd/avatars-service migrate status
```

Makefile:

```bash
make test
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

Contract runner:

```bash
BASE_URL=http://localhost:18080 go run ./cmd/avatar-contract-tests
BASE_URL=http://localhost:18080 make contract-tests
```

Local Makefile defaults:

- `LOCAL_HTTP_ADDR=:18080`
- `LOCAL_BASE_URL=http://localhost:18080`
- `HTTP_ADDR ?= $(LOCAL_HTTP_ADDR)`
- `BASE_URL ?= $(LOCAL_BASE_URL)`

JetBrains shared run configurations use `cmd/avatars-service` and `http://localhost:18080` for local server/contract runs.

Docker Compose:

```bash
docker compose up --build
```

Docker Compose publishes the server on `http://localhost:8080`; local Makefile and JetBrains configs use `http://localhost:18080` to avoid that port.

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

## Git Workflow

Base branch for MVP: `v1`.

Use short-lived task branches from `v1`:

- `feature/<short-name>`
- `fix/<short-name>`
- `test/<short-name>`
- `docs/<short-name>`
- `chore/<short-name>`

Default flow:

```bash
git checkout v1
git pull --ff-only
git checkout -b feature/<short-name>
```

Rules:

- One task should usually map to one branch and one PR.
- Do not commit directly to `v1` in normal manual workflow.
- Before PR, run `go test ./...`.
- For API/web changes, also run `make run-server` and `make contract-tests`.
- Commit messages follow the current Conventional Commit style: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`.
- Preferred PR merge policy: squash merge.
- Direct commits from an AI-agent session are allowed only when the user explicitly asks for a commit; commit only reviewed related files and never include unrelated changes.

## Security and Configuration

- Do not commit `.env`, secrets, uploaded avatars or `bin/` outputs.
- Load and validate config in `internal/config` when external adapters are connected.
- Validate upload size, magic bytes and decoded image format before storage.
- Migrations must be an explicit operational step, not an automatic server/worker startup action.
- Access logs must not include request bodies, secrets or uploaded file contents.
