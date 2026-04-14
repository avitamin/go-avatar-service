# Сервис "Аватарница"

`go-avatar-service` - Go-сервис для управления аватарками пользователей. Репозиторий сейчас находится в состоянии skeleton: есть минимальные entrypoints сервера и worker, базовый web upload UI и документация требований.

## Актуальные источники требований

Основные документы для разработки:

- [Подтвержденные требования](docs/requirements/confirmed-requirements.md)
- [Спека разработки v1](docs/specs/avatar-service-v1.md)

Исторический контекст:

- [Исходное ТЗ](docs/requirements/assignment.md)
- [QWEN.md](QWEN.md)

Если README, QWEN.md или исходное ТЗ конфликтуют с подтвержденными требованиями и v1 spec, используйте `confirmed-requirements.md` и `avatar-service-v1.md`.

## Текущее состояние

Сейчас в репозитории есть:

- `cmd/server/main.go` - минимальный HTTP server placeholder на `:8080`.
- `cmd/worker/main.go` - минимальный worker placeholder с бесконечным loop.
- `web/static/index.html` - шаблонный upload UI.
- `go.mod` - модуль `go-avatar-service`, Go `1.25.1`.
- `docs/` - требования, спека и reusable prompts для AI-агентов.

Пока отсутствуют:

- `internal/` с основной backend-логикой.
- `migrations/`.
- `tests/`.
- `Dockerfile`.
- `docker-compose.yml`.
- `Makefile`.

Docker Compose обязателен для MVP по подтвержденным требованиям, но файл compose пока не добавлен. Не используйте `docker-compose up --build` как рабочий сценарий до появления конфигурации в репозитории.

## Планируемый MVP

MVP должен реализовать:

- HTTP API на Chi.
- PostgreSQL для metadata.
- MinIO для original и thumbnails.
- RabbitMQ + worker для асинхронной обработки.
- Soft delete.
- Upload validation по размеру, MIME и magic bytes.
- Поддержку `jpeg`, `png`, `webp`, максимум 10 MB.
- Thumbnails `100x100` и `300x300`, всегда в `jpeg`.
- Public read endpoints.
- `X-User-ID` только для изменяющих операций.
- Единый JSON-формат ошибок.
- `/health` со статусами `postgres`, `minio`, `rabbitmq`.
- Access logs.
- Покрытие `>50%` по backend-пакетам с логикой сервиса и worker.

## API v1

Обязательные API endpoints:

| Method | Path | Назначение |
| --- | --- | --- |
| `POST` | `/api/v1/avatars` | Upload avatar, требует `X-User-ID`, multipart поле `file` |
| `GET` | `/api/v1/avatars/{avatar_id}` | Получить original или variant по `size` |
| `GET` | `/api/v1/users/{user_id}/avatar` | Получить текущую аватарку пользователя с fallback |
| `DELETE` | `/api/v1/avatars/{avatar_id}` | Soft delete конкретной записи, требует владельца в `X-User-ID` |
| `DELETE` | `/api/v1/users/{user_id}/avatar` | Soft delete последней неудаленной записи пользователя с доступным original |
| `GET` | `/api/v1/avatars/{avatar_id}/metadata` | Metadata записи |
| `GET` | `/api/v1/users/{user_id}/avatars` | Список неудаленных аватарок пользователя |
| `GET` | `/health` | Healthcheck зависимостей |

Поддерживаемый query parameter для file endpoints:

- `size=original`
- `size=100x100`
- `size=300x300`

Без `size` возвращается `original`. Параметр `format` в MVP не поддерживается.

## Web

Обязательные web endpoints для MVP:

- `GET /web/upload`
- `GET /web/gallery/{user_id}`

Upload из web должен идти напрямую в `POST /api/v1/avatars`. Отдельный `POST /web/upload` не нужен. Пользователь вводит `user_id` в форме.

Текущий файл `web/static/index.html` - стартовый шаблон upload UI. Важно: в текущем skeleton он может не полностью совпадать с финальным API-контрактом, поэтому при реализации сверяйте форму с v1 spec и confirmed requirements.

## Проектная структура

Текущая структура:

```text
.
├── cmd/
│   ├── server/
│   │   └── main.go
│   └── worker/
│       └── main.go
├── docs/
├── web/
│   └── static/
│       └── index.html
├── go.mod
├── README.md
└── QWEN.md
```

Целевая структура по v1 spec предпочитает один binary с subcommands:

```bash
avatars-service server
avatars-service worker
avatars-service migrate up
avatars-service migrate down
avatars-service migrate status
```

Для приватной логики используйте `internal/`:

- `internal/http`
- `internal/service`
- `internal/repository`
- `internal/storage`
- `internal/broker`
- `internal/domain`
- `internal/config`
- `internal/worker`
- `internal/imageproc`

`pkg/` добавляйте только при появлении реального public reusable API.

## Локальная разработка

Доступные сейчас команды:

```bash
go mod tidy
go run ./cmd/server
go run ./cmd/worker
go build -o ./bin/server ./cmd/server
go build -o ./bin/worker ./cmd/worker
go test ./...
```

`go test ./...` сейчас проверяет только существующий skeleton, пока тесты не добавлены.

## Подход к разработке

- Перед реализацией, ревью и тестированием AI-агенты должны читать `docs/prompts/README.md` и `docs/prompts/context/project.md`.
- Разработка ведется через TDD: сначала тест ожидаемого поведения, затем минимальная реализация, затем refactor при зеленых тестах.
- Go-код форматируется через `gofmt`.
- HTTP-обработчики должны оставаться тонкими, бизнес-логика - в service layer, доступ к данным - в repository/storage/broker adapters.
- Миграции выполняются отдельным явным шагом и не запускаются автоматически при старте server/worker.

## Безопасность и конфигурация

- Не коммитьте `.env`, секреты, загруженные аватары и бинарники из `bin/`.
- Конфигурацию изолируйте в `internal/config`.
- Для upload проверяйте размер, content type, magic bytes и storage path до сохранения.
- Read endpoints публичные по требованиям MVP, поэтому в дальнейшем может понадобиться rate limiting, но он не входит в обязательный MVP.
