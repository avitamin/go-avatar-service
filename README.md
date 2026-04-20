# Сервис "Аватарница"

`go-avatar-service` - Go-сервис для управления аватарками пользователей. Текущая реализация закрывает MVP contract smoke на runtime adapters PostgreSQL/MinIO/RabbitMQ и сохраняет локальный in-memory fallback для unit tests и быстрого запуска без внешней инфраструктуры.

## Источники требований

Основные документы для разработки:

- [Подтвержденные требования](docs/requirements/confirmed-requirements.md)
- [Спека разработки v1](docs/specs/avatar-service-v1.md)

Если README, QWEN.md или исходное ТЗ конфликтуют с подтвержденными требованиями и v1 spec, используйте `confirmed-requirements.md` и `avatar-service-v1.md`.

## Текущее состояние

Сейчас в репозитории есть:

- `cmd/avatars-service/main.go` - основной CLI entrypoint: `avatars-service server|worker|migrate`.
- `cmd/server/main.go` и `cmd/worker/main.go` - compatibility wrappers вокруг нового bootstrap.
- `cmd/avatar-contract-tests/main.go` - black-box runner контрактных smoke-тестов HTTP API.
- `internal/domain` - value objects, statuses, user ID и size validation.
- `internal/http` - Chi router, handlers, JSON error model, access logs, web pages.
- `internal/service` - application service, selection/fallback, soft delete, in-memory repository/storage для unit tests и fallback-режима без external adapters.
- `internal/repository/postgres` - PostgreSQL adapter для metadata.
- `internal/storage/minio` - MinIO adapter для original и thumbnail objects.
- `internal/broker/rabbitmq` - RabbitMQ publisher/consumer topology для `avatar.uploaded` и `avatar.delete_requested`.
- `internal/imageproc` - magic bytes sniffing, decode jpeg/png, thumbnail JPEG generation.
- `internal/worker` - upload/delete handlers, consumer runner, idempotency checks, minimal retry.
- `internal/app` - CLI/bootstrap policy.
- `migrations/` - initial SQL schema.
- `Dockerfile` и `docker-compose.yml` - локальная MVP-инфраструктура.
- `web/static/index.html` - upload UI, отправляет multipart поле `file`.
- `tests/contract/` - contract smoke runner и self-tests.

Текущий runtime переключается по env-конфигурации. Если заданы `POSTGRES_DSN`, полный набор `MINIO_*` и `RABBITMQ_URL`, `server` и `worker` используют реальные PostgreSQL/MinIO/RabbitMQ adapters. Если external storage env не задан, bootstrap оставляет in-memory repository/storage fallback для локальных unit-style запусков. `avatars-service migrate up|down|status` применяет SQL к PostgreSQL и остается отдельным явным operational step. `GET /health` выполняет runtime checks по `postgres`, `minio`, `rabbitmq`; при fallback/noop runtime или недоступности dependency endpoint остается `200`, но возвращает `status=degraded`.

## CLI

Основной контракт:

```bash
go run ./cmd/avatars-service server
go run ./cmd/avatars-service worker
go run ./cmd/avatars-service migrate up
go run ./cmd/avatars-service migrate down
go run ./cmd/avatars-service migrate status
```

Миграции являются отдельным явным шагом и не запускаются автоматически при старте `server` или `worker`.

Legacy wrappers оставлены временно:

```bash
go run ./cmd/server
go run ./cmd/worker
```

Новый код расширяйте через `cmd/avatars-service` и `internal/app`, а не через legacy entrypoints.

## API v1

| Method | Path | Назначение |
| --- | --- | --- |
| `POST` | `/api/v1/avatars` | Upload avatar, требует `X-User-ID`, multipart поле `file` |
| `GET` | `/api/v1/avatars/{avatar_id}` | Получить original или variant по `size` |
| `GET` | `/api/v1/users/{user_id}/avatar` | Получить текущую аватарку пользователя с fallback |
| `DELETE` | `/api/v1/avatars/{avatar_id}` | Soft delete конкретной записи, требует владельца в `X-User-ID` |
| `DELETE` | `/api/v1/users/{user_id}/avatar` | Soft delete последней неудаленной записи пользователя с доступным original |
| `GET` | `/api/v1/avatars/{avatar_id}/metadata` | Metadata записи |
| `GET` | `/api/v1/users/{user_id}/avatars` | Список неудаленных аватарок пользователя |
| `GET` | `/health` | Healthcheck компонентов |

Поддерживаемый `size`:

- `original`
- `100x100`
- `300x300`

Без `size` возвращается `original`. Query parameter `format` в MVP не поддерживается и возвращает `400`.

`/health` возвращает top-level `status` и nested `components`:

```json
{
  "status": "ok",
  "components": {
    "postgres": "ok",
    "minio": "ok",
    "rabbitmq": "ok"
  }
}
```

Допустимые значения статусов: `ok`, `degraded`. При частичной деградации или fallback/noop adapters HTTP status остается `200`, а проблемный компонент и общий `status` становятся `degraded`.

Read endpoints публичные. `X-User-ID` обязателен только для изменяющих операций. Ошибки возвращаются в едином JSON shape:

```json
{
  "error": {
    "code": "invalid_size",
    "message": "Unsupported size"
  }
}
```

## Web

Обязательные web endpoints:

- `GET /web/upload`
- `GET /web/gallery/{user_id}`

Upload из web идет напрямую в `POST /api/v1/avatars`; отдельный `POST /web/upload` не нужен. Форма использует multipart поле `file`.

Галерея показывает только записи с доступным `original`, без удаления. API list и web gallery намеренно не смешиваются: API list показывает failed записи, web gallery фильтрует по доступному original.

## Локальная разработка

```bash
go mod tidy
go test ./...
go test ./internal/... -cover
go test -run='^$' -bench=. -benchmem ./...
go build ./cmd/avatars-service ./cmd/server ./cmd/worker ./cmd/avatar-contract-tests
```

Makefile:

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
make docker-build
make docker-up-build
make docker-up-detached
make docker-down
make docker-ps
make docker-logs
make docker-contract-tests
```

Makefile настроен под локальную разработку:

- `make run-server` использует `HTTP_ADDR=:18080` по умолчанию.
- `make contract-tests` использует `BASE_URL=http://localhost:18080` по умолчанию.

Значения можно переопределить:

```bash
make run-server HTTP_ADDR=:18081
make contract-tests BASE_URL=http://localhost:18081
```

Contract smoke runner запускается против уже поднятого сервиса:

```bash
BASE_URL=http://localhost:18080 go run ./cmd/avatar-contract-tests
BASE_URL=http://localhost:18080 make contract-tests
```

Exit codes contract runner:

- `0` - все сценарии прошли.
- `1` - есть проваленные контрактные сценарии.
- `2` - неверная конфигурация runner'а, например не задан `BASE_URL`.

## Docker Compose

Локальный compose описывает:

- `server`
- `worker`
- `postgres`
- `rabbitmq`
- `minio`

Команда:

```bash
docker compose run --rm server migrate up
docker compose up --build
```

Docker Compose поднимает PostgreSQL, MinIO, RabbitMQ, server и worker. Перед первым запуском server/worker нужен явный migration step; миграции не запускаются автоматически при старте процессов.

Docker Compose публикует server на `http://localhost:8080`. Локальные Makefile/JetBrains конфигурации используют `http://localhost:18080`, чтобы не занимать стандартный compose-порт.

Host-порты Docker Compose можно переопределить через локальный `.env`. Шаблон лежит в `.env.example`, сам `.env` игнорируется git. Например, если порт MinIO console `9001` занят:

```bash
cp .env.example .env
printf 'COMPOSE_MINIO_CONSOLE_PORT=19001\n' >> .env
make docker-up-detached
```

## JetBrains Run Configurations

В `.idea/runConfigurations/` сохранены shared конфигурации:

- `Server` - запускает `cmd/avatars-service server` с `HTTP_ADDR=:18080`.
- `Worker` - запускает `cmd/avatars-service worker`.
- `Avatar Contract Tests` - запускает contract runner с `BASE_URL=http://localhost:18080`.
- `Make Test` - выполняет `make test`.
- `Make Build Contract Tests` - выполняет `make build-contract-tests`.
- `Make Contract Tests` - выполняет `make contract-tests`; локальный `BASE_URL=http://localhost:18080` берется из Makefile.
- `Make Docker Build` - выполняет `make docker-build`.
- `Make Docker Up Build` - выполняет `make docker-up-build`.
- `Make Docker Up Detached` - выполняет `make docker-up-detached`.
- `Make Docker Down` - выполняет `make docker-down`.
- `Make Docker Ps` - выполняет `make docker-ps`.
- `Make Docker Logs` - выполняет `make docker-logs`.
- `Make Docker Contract Tests` - выполняет `make docker-contract-tests` против compose-порта `http://localhost:8080`.

## Проектная структура

```text
.
├── cmd/
│   ├── avatars-service/
│   ├── server/
│   ├── worker/
│   └── avatar-contract-tests/
├── internal/
│   ├── app/
│   ├── domain/
│   ├── http/
│   ├── imageproc/
│   ├── repository/
│   │   └── postgres/
│   ├── service/
│   ├── storage/
│   │   └── minio/
│   ├── broker/
│   │   └── rabbitmq/
│   └── worker/
├── migrations/
├── tests/
│   └── contract/
├── web/
│   └── static/
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── Makefile
```

Новые external adapters размещайте по v1 spec. Текущие PostgreSQL, MinIO и RabbitMQ adapters уже находятся в `internal/repository/postgres`, `internal/storage/minio` и `internal/broker/rabbitmq`. Конфигурация пока читается в `internal/app`; при росте bootstrap-логики ее стоит вынести в `internal/config`.

`pkg/` добавляйте только при появлении реального public reusable API.

## Тестирование и покрытие

Разработка ведется через TDD: сначала focused failing test, затем минимальная реализация, затем refactor.

Текущие проверки:

```bash
go test ./...
go test ./internal/... -cover
```

Backend-пакеты с логикой сервиса и worker должны держать покрытие выше 50%. Coverage не заменяет requirement coverage: обязательное поведение из confirmed requirements/v1 spec должно иметь явный тест или documented gap.

## Benchmarking

Локальный benchmark workflow описан в [docs/benchmarking.md](docs/benchmarking.md).

Основная команда:

```bash
make bench
```

Она запускает:

```bash
go test -run='^$' -bench=. -benchmem ./...
```

Бенчмарки покрывают domain validation, image processing, service fallback/list paths, HTTP router paths, worker thumbnail generation, а также opt-in PostgreSQL/RabbitMQ adapter paths.

External adapters запускаются отдельно, если подняты сервисы и заданы env:

```bash
POSTGRES_DSN='postgres://avatars:avatars@localhost:5432/avatars?sslmode=disable' \
RABBITMQ_URL='amqp://guest:guest@localhost:5672/' \
make bench-external
```

Запускайте benchmarks опционально для изменений, которые могут повлиять на CPU, allocations или latency hot paths; для обычных изменений обязательной проверкой остается `go test ./...`.

## Git Workflow

Base branch для MVP: `v1`.

Обычный порядок работы:

```bash
git checkout v1
git pull --ff-only
git checkout -b feature/<short-name>
```

Типы рабочих веток:

- `feature/<short-name>` - новая функциональность.
- `fix/<short-name>` - исправление поведения или багов.
- `test/<short-name>` - тесты без production changes.
- `docs/<short-name>` - документация и contributor guidance.
- `chore/<short-name>` - инфраструктура, локальные run configs, Makefile, housekeeping.

Правила:

- Одна задача - одна рабочая ветка и один PR, если изменения не являются явно связанным маленьким follow-up.
- Не коммитьте напрямую в `v1` в обычном ручном workflow; открывайте PR в `v1`.
- Перед PR запустите `go test ./...`.
- Для API/web изменений дополнительно запустите contract smoke: `make run-server` в одном терминале и `make contract-tests` в другом.
- Для изменений Docker/миграций укажите в PR, какие внешние сервисы нужны для проверки.
- Commit message пишите в текущем стиле истории: `feat: ...`, `fix: ...`, `docs: ...`, `test: ...`, `refactor: ...`, `chore: ...`.
- Предпочтительный merge policy для PR: squash merge, чтобы история `v1` оставалась короткой и читаемой.

Для локальной AI-agent сессии прямой commit допустим только если пользователь явно попросил закоммитить изменения. В этом случае агент сначала проверяет `git status`, коммитит только просмотренные связанные файлы и не трогает unrelated changes.

## Безопасность и конфигурация

- Не коммитьте `.env`, секреты, загруженные аватары и бинарники из `bin/`.
- Read endpoints публичные по требованиям MVP.
- Upload validation должна проверять размер, magic bytes и декодирование изображения без доверия клиентскому `Content-Type`.
- Физическое удаление файлов выполняет только worker после soft delete.
- Access logs не должны логировать тело запроса, секреты и содержимое uploaded files.
