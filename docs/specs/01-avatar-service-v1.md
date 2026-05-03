# Спека разработки v1

Этот документ описывает планируемую реализацию первой версии сервиса "Аватарница".

## Источники требований

- [Исходное ТЗ](../requirements/assignment.md)
- [Подтвержденные требования](../requirements/confirmed-requirements.md)

# 1. REQUIREMENT BASELINE

## Confirmed Requirements

### Functional

* Go HTTP-сервер на **Chi**.
* Хранилище метаданных: **PostgreSQL**.
* Хранилище файлов: **MinIO**.
* Асинхронная обработка: **RabbitMQ + worker**.
* Обязательные API-операции:

    * `POST /api/v1/avatars`
    * `GET /api/v1/avatars/{avatar_id}`
    * `GET /api/v1/users/{user_id}/avatar`
    * `DELETE /api/v1/avatars/{avatar_id}`
    * `DELETE /api/v1/users/{user_id}/avatar`
    * `GET /api/v1/avatars/{avatar_id}/metadata`
    * `GET /api/v1/users/{user_id}/avatars`
    * `GET /health`
* Обязательные web endpoints:

    * `GET /web/upload`
    * `GET /web/gallery/{user_id}`
* Frontend не пишется с нуля, используется шаблонный фронт из репозитория.
* Upload из web идёт напрямую в `POST /api/v1/avatars`.
* `user_id` вводится пользователем в форме.
* Галерея только для списка, без удаления.

### Upload / image rules

* Разрешённые форматы: `jpeg`, `png`, `webp`.
* Валидация по:

    * размеру файла
    * MIME
    * magic bytes
* Максимальный размер: **10 MB**.
* Создаются миниатюры: **100x100**, **300x300**.
* Миниатюры всегда хранятся в **jpeg**.
* Оригинал хранится в исходном формате.

### Read behavior

* `GET /api/v1/avatars/{id}` без `size` возвращает `original`.
* Поддерживается только `size`:

    * `original`
    * `100x100`
    * `300x300`
* `format` в MVP не поддерживается.
* Неподдерживаемый `size` → `400`.
* Read-endpoints публичные.
* `X-User-ID` обязателен только для изменяющих операций.
* `X-User-ID` — строка длиной `1..255` и проверяется по allowlist pattern.
* `url` в API — относительные URL API сервиса, не S3 URL.

### External status model

* Во внешнем API только:

    * `processing`
    * `completed`
    * `failed`
* Если publish в RabbitMQ не удался после сохранения ресурса:

    * `201`
    * `status=failed`
* `failed` записи видимы в list и metadata.
* Soft-deleted записи снаружи выглядят как `404` и отсутствуют в list.

### Delete behavior

* Soft delete обязателен.
* Физическое удаление файлов делает только worker после soft delete.
* `DELETE /api/v1/avatars/{avatar_id}`:

    * `404`, если не найдена или уже удалена
    * `403`, если `X-User-ID` не владелец
    * `204`, если успешно
* `DELETE /api/v1/users/{user_id}/avatar`:

    * разрешён только если `X-User-ID == user_id`
    * удаляет последнюю неудалённую запись с доступным `original`

### Fallback / selection rules

* Текущая аватарка пользователя для user-based read:

    * выбирается как последняя неудалённая запись с доступным нужным variant
    * fallback на предыдущую доступную запись обязателен
* Для `GET /api/v1/users/{user_id}/avatar?size=...`:

    * fallback работает и для `original`, и для thumbnails
    * если после fallback подходящего variant нет → `404`
* Для `GET /api/v1/avatars/{avatar_id}?size=...`:

    * если конкретный variant доступен → `200`
    * если недоступен и статус `processing` → `409`
    * если недоступен и статус `failed` → `409`
* Если original отсутствует в MinIO:

    * file endpoint → `404`
    * metadata endpoint → `200`
    * внешний `status` нормализуется как `failed`

### Metadata / list / health

* Metadata возвращается для существующей неудалённой записи даже при проблемах storage.
* `dimensions` желательны, но не обязательны в контракте MVP.
* Thumbnails в metadata включаются только если реально готовы.
* Список аватарок пользователя:

    * только неудалённые записи
    * сортировка `created_at DESC`
    * без пагинации
    * минимальные поля: `id`, `user_id`, `url`, `status`, `created_at`
* `/health`:

    * проверяет `postgres`, `minio`, `rabbitmq`
    * отдаёт общий статус и статусы компонентов
    * при частичной деградации → `200` и `status=degraded`

### Error model / ops / quality

* Единый JSON-формат ошибок:

    * `error.code` обязателен
    * `error.message` обязателен
    * `error.details` опционален
* Worker обязателен.
* Worker должен быть базово идемпотентным.
* Worker должен логировать дубликаты.
* Retry обязателен в минимальном виде.
* Миграции:

    * отдельный явный шаг
    * не автозапускаются при старте server/worker
* Dockerfile и Docker Compose обязательны.
* Kubernetes вне MVP.
* Покрытие `>50%` по backend-пакетам с логикой сервиса и worker.
* Access logs для HTTP обязательны.

## Assumptions Used

1. **Один репозиторий, один deployable backend codebase**, но с разными режимами запуска:

    * `server`
    * `worker`
    * `migrate`
2. **Single binary с subcommands** предпочтительнее двух независимых приложений, чтобы упростить конфигурацию, сборку и тестирование.
3. **RabbitMQ topology**:

    * один exchange `avatars`
    * отдельные очереди для `avatar.uploaded` и `avatar.delete_requested`
4. **В БД хранятся отдельные флаги наличия variants**, а не только JSON со ссылками, чтобы упростить выбор доступного варианта и fallback.
5. **MinIO bucket один**, например `avatars`, а ключи объектов организованы по префиксам.
6. **Размеры и MIME определяются на сервере до записи в S3**, без доверия к заголовкам клиента.
7. **Обработка изображений синхронно в upload request не делается**; сервер только сохраняет original и ставит задачу.
8. **Web gallery использует тот же backend API**, а не прямое чтение из БД в шаблоне.
9. **Read endpoints не требуют auth вообще**, кроме проверки валидности входных параметров.
10. **Для идемпотентности worker достаточно состояния в БД + детерминированных S3 keys**, без отдельного distributed dedup store.
11. **Транзакционный outbox не входит в MVP**; при сбое publish после сохранения записи статус переводится в `failed` согласно confirmed requirements.
12. **`GET /web/gallery/{user_id}` при наличии записей в БД, но отсутствии доступных original возвращает 200 и пустую страницу**, не 404.
13. **Frontend-шаблоны рендерятся сервером как статические/templated страницы**, без отдельного frontend build pipeline внутри MVP, кроме выдачи готовых asset-файлов.
14. **Access logs и application logs — в stdout в структурированном JSON**, без отдельной log aggregation системы.
15. **Healthcheck не делает глубоких бизнес-проверок**, только connectivity / basic operation.

## Risk from Assumptions

* Single binary упрощает MVP, но при росте продукта может ухудшить независимое версионирование server/worker.
* Отсутствие outbox означает риск рассинхронизации между БД и брокером; он частично принят требованиями MVP.
* Выбор API-backed web gallery упрощает консистентность, но может привести к лишнему внутреннему HTTP-вызову, если реализовать буквально через loopback. Это нужно избежать и использовать общий service layer.
* Хранение availability flags в БД увеличит схему, но уменьшит сложность runtime-логики.
* Публичные read endpoints повышают риск abuse; для MVP это допустимо, но rate limiting и CDN остаются вне scope.

---

# 2. PRODUCT DECISION

## MVP Scope

### Backend

* HTTP API на Chi.
* Upload original в MinIO.
* Сохранение metadata в PostgreSQL.
* Публикация задач в RabbitMQ.
* Worker для:

    * генерации thumbnails
    * физического удаления файлов после soft delete
* Public read endpoints.
* Полная реализация confirmed business rules по fallback, status normalization, delete behavior.

### Web

* `GET /web/upload` с формой и JS-вызовом `POST /api/v1/avatars`.
* `GET /web/gallery/{user_id}` со списком доступных original avatars.
* Без редактирования, удаления, прогресса, drag&drop как обязательных фич.

### Infra / ops

* Dockerfile multi-stage.
* Docker Compose для локального запуска:

    * app/server
    * app/worker
    * postgres
    * rabbitmq
    * minio
* Отдельная команда миграций.
* Unit + integration tests до покрытия >50%.

## Non-MVP

* Пагинация list.
* Поддержка `format` query parameter.
* Presigned URLs.
* CDN/cache invalidation.
* AuthN/AuthZ кроме `X-User-ID` для mutate.
* Outbox / saga / exactly-once delivery.
* Admin API.
* Kubernetes manifests.
* Virus scanning / EXIF stripping / content moderation.
* Background reconciliation job.
* Rate limiting.
* OpenAPI-first design and codegen.
* Metrics/Tracing stack уровня production.

## Success Criteria

1. Пользователь может загрузить аватар до 10 MB в `jpeg/png/webp`.
2. Сервер всегда валидирует content type по magic bytes.
3. Original доступен сразу после upload, thumbnails появляются асинхронно.
4. При отказе RabbitMQ ресурс остаётся созданным со статусом `failed`.
5. User-based read корректно делает fallback по последним доступным variants.
6. Soft delete скрывает запись снаружи немедленно.
7. Worker физически удаляет файлы позже и делает это идемпотентно.
8. `/health` отражает degraded state без падения HTTP status в 5xx при частичной проблеме.
9. Сервис поднимается локально одной командой через Docker Compose.
10. Покрытие тестами >50% по пакетам с логикой сервиса и worker.

---

# 3. ARCHITECTURE (CLI-FIRST)

## Architecture Overview

Для MVP выбирается **монолитный backend** с двумя исполняемыми режимами:

* `server` — HTTP API + web pages
* `worker` — асинхронная обработка очередей

Физически это один кодовый репозиторий и один application boundary.
Логические компоненты:

* HTTP layer
* application services
* repository layer
* object storage client
* message broker publisher/consumer
* image processor
* worker handlers

Это не распределённая архитектура, а **простая service + worker схема**, где worker вынесен только потому, что асинхронная обработка явно обязательна.

## Execution Model

### Command structure

Предпочтительный контракт:

```bash
avatars-service server
avatars-service worker
avatars-service migrate up
avatars-service migrate down
avatars-service migrate status
```

### Why

* Один binary проще собирать, тестировать и конфигурировать.
* Subcommands закрывают требование CLI-first в части эксплуатации.
* Нет необходимости плодить отдельные приложения там, где достаточно разных режимов процесса.

### Runtime processes

* `server`

    * слушает HTTP
    * обслуживает API и web
    * пишет metadata в DB
    * сохраняет original в MinIO
    * публикует сообщения в RabbitMQ
* `worker`

    * читает очереди RabbitMQ
    * генерирует thumbnails
    * удаляет S3 objects после soft delete
    * обновляет статусы в DB
* `migrate`

    * применяется вручную как отдельный operational step

## Module Breakdown

* `cmd/app`

    * main entrypoint
    * subcommands bootstrap
* `internal/config`

    * env parsing
    * config validation
* `internal/http`

    * router
    * middleware
    * handlers
    * web pages
* `internal/service`

    * avatar service
    * health service
* `internal/repository`

    * postgres repositories
* `internal/storage`

    * minio adapter
* `internal/broker`

    * publisher / consumer
* `internal/worker`

    * event handlers
    * retry logic
* `internal/imageproc`

    * decode / inspect / resize / encode jpeg
* `internal/domain`

    * entities
    * statuses
    * errors
    * value objects
* `internal/migrations`

    * migration management wrapper
* `pkg/` не нужен для MVP, если нет реального reusable public API

## Data Flow

### Upload

1. HTTP request приходит в `POST /api/v1/avatars`.
2. Handler валидирует `X-User-ID`, multipart, размер, MIME, magic bytes.
3. Service генерирует `avatar_id`.
4. Original сохраняется в MinIO по детерминированному key.
5. Metadata сохраняется в PostgreSQL:

    * original present
    * thumbs absent
    * status `processing`
6. Публикуется `avatar.uploaded`.
7. Если publish упал:

    * запись не откатывается
    * status ставится `failed`
    * ответ `201`
8. Worker читает сообщение, создаёт thumbnails, обновляет availability/status.

### Read by avatar id

1. Handler валидирует `size`.
2. Service получает metadata.
3. Если запись удалена/не найдена → `404`.
4. Выбирается variant key.
5. Если variant не готов:

    * для конкретного avatar endpoint → `409` при `processing|failed`
6. Если key есть, но объект в MinIO отсутствует:

    * file endpoint → `404`
    * наружный status интерпретируется как `failed`

### Read by user id with fallback

1. Handler валидирует `user_id`, `size`.
2. Service запрашивает неудалённые записи пользователя по `created_at DESC`.
3. Ищется первая запись с доступным нужным variant.
4. Если ничего не найдено → `404`.
5. Возвращается файл.

### Delete

1. Handler валидирует `X-User-ID`.
2. Service находит запись.
3. Проверяет ownership.
4. Помечает `deleted_at`.
5. Публикует `avatar.delete_requested`.
6. Worker физически удаляет original и thumbs; отсутствие объекта трактуется идемпотентно.

## Error Handling

### Error categories

* Validation errors → `400`
* Auth/ownership errors → `403`
* Not found / soft deleted → `404`
* Variant not ready / failed → `409`
* Too large → `413`
* Internal errors → `500`

### Error model

Единая структура:

```json
{
  "error": {
    "code": "invalid_size",
    "message": "Unsupported size",
    "details": {
      "allowed": ["original", "100x100", "300x300"]
    }
  }
}
```

### Internal strategy

* Domain errors typed and mapped centrally middleware/renderer layer.
* Не размазывать HTTP codes по service layer.
* Storage missing object для original/read path маппить отдельно от DB not found.

## Configuration

Только через environment variables + optional `.env` для local dev.

Пример:

* `APP_ENV`
* `HTTP_ADDR`
* `POSTGRES_DSN`
* `MINIO_ENDPOINT`
* `MINIO_ACCESS_KEY`
* `MINIO_SECRET_KEY`
* `MINIO_BUCKET`
* `MINIO_USE_SSL`
* `RABBITMQ_URL`
* `RABBITMQ_EXCHANGE`
* `RABBITMQ_UPLOAD_QUEUE`
* `RABBITMQ_DELETE_QUEUE`
* `MAX_UPLOAD_BYTES`
* `USER_ID_PATTERN`
* `LOG_LEVEL`

Правила:

* config load один раз на старте
* строгая валидация обязательных полей
* fail-fast при неверной конфигурации
* migrations используют те же config primitives

## Logging

Обязательно:

* structured JSON logs в stdout
* access logs для HTTP
* application logs для service/worker

Поля:

* `ts`
* `level`
* `msg`
* `component`
* `request_id`
* `user_id`
* `avatar_id`
* `event_type`
* `delivery_tag`
* `error`

Отдельно логировать:

* upload accepted
* upload publish failed
* duplicate worker processing
* thumbnail generation success/failure
* delete requested / delete executed
* health degradation

---

# 4. ENGINEERING DESIGN

## Project Structure

```text
avatars-service/
├── cmd/
│   └── avatars-service/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── server.go
│   │   ├── worker.go
│   │   └── migrate.go
│   ├── config/
│   │   ├── config.go
│   │   └── validate.go
│   ├── domain/
│   │   ├── avatar.go
│   │   ├── status.go
│   │   ├── errors.go
│   │   └── events.go
│   ├── http/
│   │   ├── router.go
│   │   ├── middleware/
│   │   │   ├── logging.go
│   │   │   ├── request_id.go
│   │   │   └── recover.go
│   │   ├── handlers/
│   │   │   ├── avatars_post.go
│   │   │   ├── avatars_get.go
│   │   │   ├── avatars_delete.go
│   │   │   ├── users_avatar_get.go
│   │   │   ├── users_avatar_delete.go
│   │   │   ├── users_avatars_list.go
│   │   │   ├── metadata_get.go
│   │   │   ├── health_get.go
│   │   │   └── web.go
│   │   └── render/
│   │       ├── json.go
│   │       └── errors.go
│   ├── service/
│   │   ├── avatar_service.go
│   │   ├── health_service.go
│   │   └── selection_service.go
│   ├── repository/
│   │   ├── postgres/
│   │   │   ├── avatar_repository.go
│   │   │   ├── queries.sql
│   │   │   └── models.go
│   │   └── interfaces.go
│   ├── storage/
│   │   ├── minio/
│   │   │   └── client.go
│   │   └── interfaces.go
│   ├── broker/
│   │   ├── rabbitmq/
│   │   │   ├── publisher.go
│   │   │   ├── consumer.go
│   │   │   └── topology.go
│   │   └── interfaces.go
│   ├── imageproc/
│   │   ├── sniff.go
│   │   ├── decode.go
│   │   ├── resize.go
│   │   └── metadata.go
│   ├── worker/
│   │   ├── handler_upload.go
│   │   ├── handler_delete.go
│   │   ├── retry.go
│   │   └── runner.go
│   └── web/
│       ├── templates/
│       │   ├── upload.html
│       │   └── gallery.html
│       └── static/
├── migrations/
│   ├── 001_init.up.sql
│   ├── 001_init.down.sql
│   └── 002_indexes.up.sql
├── tests/
│   ├── integration/
│   └── e2e/
├── docker/
│   └── compose/
│       └── dev.yaml
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## Core Abstractions

### Domain entity

`Avatar`

* `ID`
* `UserID`
* `FileName`
* `OriginalMimeType`
* `SizeBytes`
* `OriginalS3Key`
* `Thumb100S3Key`
* `Thumb300S3Key`
* `OriginalAvailable`
* `Thumb100Available`
* `Thumb300Available`
* `UploadStatusInternal`
* `ProcessingStatusInternal`
* `CreatedAt`
* `UpdatedAt`
* `DeletedAt`

### Service-level interfaces

```go
type AvatarRepository interface {
    Create(ctx context.Context, a *domain.Avatar) error
    GetActiveByID(ctx context.Context, id string) (*domain.Avatar, error)
    ListActiveByUser(ctx context.Context, userID string) ([]domain.Avatar, error)
    GetLatestActiveWithOriginalByUser(ctx context.Context, userID string) (*domain.Avatar, error)
    SoftDeleteByID(ctx context.Context, id string, deletedAt time.Time) error
    UpdateProcessingResult(ctx context.Context, id string, patch domain.ProcessingPatch) error
    MarkPublishFailed(ctx context.Context, id string) error
}
```

```go
type ObjectStorage interface {
    PutObject(ctx context.Context, key string, data io.Reader, size int64, contentType string) error
    GetObject(ctx context.Context, key string) (io.ReadCloser, ObjectMeta, error)
    DeleteObject(ctx context.Context, key string) error
    StatObject(ctx context.Context, key string) (ObjectMeta, error)
}
```

```go
type Broker interface {
    Publish(ctx context.Context, topic string, msg []byte, messageID string) error
}
```

### Selection abstraction

Нужен отдельный selector, потому что бизнес-правила выбора файла нетривиальны:

* exact avatar + exact variant
* latest user avatar + fallback
* skip soft deleted
* normalize failed if storage broken

Это лучше не смешивать с handler или repository.

## CLI Contract

```bash
avatars-service server
```

Запускает HTTP-сервер.

```bash
avatars-service worker
```

Запускает consumer loop и обработчики очередей.

```bash
avatars-service migrate up
avatars-service migrate down
avatars-service migrate status
```

Явный lifecycle для схемы БД.

Дополнительно через Makefile:

```bash
make run-server
make run-worker
make migrate-up
make test
make lint
```

## Validation Strategy

### Request validation

* `X-User-ID`

    * required only for mutate operations
    * длина `1..255`
    * regex allowlist
* `avatar_id`

    * UUID parse
* `user_id`

    * тот же allowlist pattern
* `size`

    * enum: `original|100x100|300x300`
* multipart

    * наличие `file`
    * размер ≤ 10 MB

### File validation

В строгом порядке:

1. Проверка max bytes на уровне request body
2. Чтение initial bytes
3. Magic bytes sniffing
4. MIME normalization
5. Попытка decode изображения
6. Проверка, что формат входит в allowlist

### Worker validation

* event schema decode
* presence of required ids
* existence of DB record
* skip if already deleted / already processed

## Test Strategy

### Unit tests

Приоритет:

* size validation
* user ID validation
* file sniffing / MIME detection
* service rules:

    * upload success
    * upload publish failure => `failed`
    * selection by avatar id
    * fallback by user id
    * delete ownership rules
    * status normalization when MinIO object missing
* worker handlers:

    * duplicate upload event
    * missing original
    * thumbnail creation success/failure
    * delete idempotency

### Integration tests

* HTTP handlers with mocked service
* repository tests against PostgreSQL via testcontainers
* MinIO integration for object existence / missing cases
* RabbitMQ integration for publish/consume smoke

### E2E minimal

* upload → worker process → get thumb
* upload → delete → hidden from reads/list → worker removes files

### Coverage focus

Не тратить усилия на:

* generated templates
* trivial wiring
* main bootstrap
  Тестировать только logic-heavy packages.

## Complexity Points

1. **Разделение внутреннего и внешнего статуса**

    * внешне только `processing/completed/failed`
    * внутри понадобится чуть более детальное состояние

2. **Fallback logic**

    * user-based read должен уметь искать по нескольким записям и variants

3. **Storage inconsistency**

    * БД говорит, что original есть, а MinIO потерял объект
    * file endpoint и metadata endpoint должны вести себя по-разному

4. **Delete semantics**

    * soft delete немедленно скрывает ресурс
    * physical delete позже, асинхронно

5. **Publish failure after DB save**

    * требуется явно поддерживаемая деградация, а не rollback

---

# 5. STACK SELECTION

## Stack Options (2–4)

### Option A — Chi + pgx + MinIO SDK + amqp091-go + imaging

**dev speed:** high
**simplicity:** high
**portability:** high
**ecosystem:** strong
**testability:** high

Плюсы:

* минималистичный HTTP stack
* `pgx` хорошо работает с PostgreSQL
* `amqp091-go` — прямой и понятный клиент RabbitMQ
* `imaging` достаточно для resize в MVP
* мало скрытой магии

Минусы:

* больше ручного кода вокруг SQL и mapping
* нет ORM convenience

### Option B — Chi + sqlc + pgx + MinIO SDK + amqp091-go + imaging

**dev speed:** medium-high
**simplicity:** medium-high
**portability:** high
**ecosystem:** strong
**testability:** very high

Плюсы:

* типобезопасный SQL
* хороший баланс между ручным SQL и boilerplate
* удобное тестирование репозиториев
* меньше риска runtime mapping bugs

Минусы:

* дополнительный toolchain
* чуть сложнее старт проекта

### Option C — Echo + GORM + MinIO SDK + amqp091-go + imaging

**dev speed:** medium
**simplicity:** medium
**portability:** medium-high
**ecosystem:** strong
**testability:** medium

Плюсы:

* быстрый CRUD старт
* меньше SQL руками

Минусы:

* Echo против confirmed requirement Chi
* GORM избыточен для несложной схемы
* сложнее контролировать точные запросы для fallback/select logic

### Option D — Chi + Bun ORM + MinIO SDK + amqp091-go + bimg

**dev speed:** medium
**simplicity:** medium
**portability:** medium
**ecosystem:** good
**testability:** medium

Плюсы:

* ORM/sql builder удобнее голого SQL
* `bimg` быстрый

Минусы:

* `bimg` тянет libvips и усложняет Docker/runtime
* для MVP это лишняя операционная сложность

## Final Stack (Architect decision)

**Option B**:

* Go 1.21+
* Chi
* pgx + sqlc
* MinIO Go SDK
* RabbitMQ via `amqp091-go`
* `disintegration/imaging` для resize
* `golang-migrate` для миграций
* `zap` или `slog` для structured logging
* `testify`
* `testcontainers-go`
* `golangci-lint`

## Why

Это лучший компромисс для MVP:

* соответствует confirmed stack
* не перегружен framework-магией
* оставляет SQL под контролем
* лучше тестируется, чем ORM-first решение
* проще в Docker и CI, чем image stack на libvips
* хорошо поддерживает сложные query rules для fallback/select logic

## Rejected options

### Echo

Отклонён, потому что зафиксирован Chi.

### GORM

Отклонён, потому что бизнес-правила выбора записей и состояний здесь важнее скорости написания простого CRUD.

### Kafka

Отклонён, потому что RabbitMQ already fixed for MVP и проще операционно.

### bimg/libvips

Отклонён, потому что ускорение обработки не критично для MVP, а сложность контейнеризации вырастает.

---

# 6. CRITICAL REVIEW

## Weak points

1. **Нет outbox**

    * Возможна потеря сообщения между DB commit и Rabbit publish.
    * Частично допустимо требованиями, потому что publish failure должен приводить к `status=failed`.

2. **Публичные read endpoints**

    * Возможны скачивания/abuse.
    * В MVP нет rate limiting.

3. **Worker idempotency базового уровня**

    * Дубликаты будут обработаны через проверки состояния, но не исключены полностью на transport level.

4. **Gallery rule “показывать только записи с доступным original”**

    * Это означает, что web gallery и API list имеют разную фильтрацию.
    * Нужно не смешать их в одном service method без явного режима.

5. **Storage drift**

    * БД и MinIO могут расходиться.
    * Это уже учитывается, но без reconciliation job drift со временем будет накапливаться.

## Overengineering risks

1. Вводить outbox, saga, orchestration — лишнее для MVP.
2. Вводить DDD-слои сверх необходимого — лишнее.
3. Делать separate image service — лишнее.
4. Делать full CQRS/event sourcing — лишнее.
5. Делать OpenAPI codegen-first и клиента для собственного web UI — лишнее.
6. Добавлять Redis ради temporary state — лишнее.
7. Добавлять ORM плюс repository abstraction plus generic base repositories — лишнее.

## Failure scenarios

### 1. Publish failed after DB + MinIO success

* Поведение:

    * вернуть `201`
    * status `failed`
* Последствие:

    * original может быть доступен, thumbnails не будут созданы

### 2. Worker получил duplicate upload event

* Поведение:

    * проверить, не готовы ли уже thumbs
    * залогировать duplicate
    * завершить без ошибки

### 3. Original object missing in MinIO

* File read:

    * `404`
* Metadata:

    * `200`
    * наружный status `failed`

### 4. Thumbnail missing, status processing/failed

* `GET /api/v1/avatars/{id}?size=thumb`:

    * `409`

### 5. RabbitMQ недоступен при delete publish

* Assumption:

    * soft delete уже сделан, запись скрыта
    * deletion event publish failure логируется
    * статус ресурса снаружи уже неважен, так как запись 404
* Риск:

    * физические файлы могут остаться в storage

### 6. MinIO деградировал

* `/health`:

    * `200`, `status=degraded`
* Upload/read/delete operations могут частично ломаться
* Это ожидаемо

---

# 7. FINAL SPEC

## Overview

Сервис аватарок — это простой backend на Go, реализующий:

* загрузку изображений пользователей
* хранение original в MinIO
* хранение metadata в PostgreSQL
* асинхронную генерацию thumbnails через RabbitMQ + worker
* публичное чтение файлов и metadata
* soft delete с последующим физическим удалением файлов worker’ом
* базовый web UI для upload и gallery

Архитектурно это **монолитный backend с отдельным worker mode**, а не распределённая система.

## Scope

### In scope

* REST API по confirmed endpoints
* Web upload page
* Web gallery page
* Upload validation
* Async thumbnail generation
* Soft delete + async physical delete
* Health endpoint
* Docker / Compose
* Migrations as explicit step
* Tests >50%

### Out of scope

* Auth platform
* pagination
* rate limiting
* CDN
* K8s
* outbox
* format conversion on read
* edit/crop UI
* admin panel

## Use Cases

1. Пользователь загружает аватар.
2. Клиент сразу получает `avatar_id` и статус.
3. Пока идёт обработка, original уже может быть доступен.
4. Worker создаёт thumbnails и переводит статус в `completed`.
5. Публичный клиент может читать avatar by id или latest user avatar с fallback.
6. Владелец может удалить конкретный avatar или текущий avatar пользователя.
7. Web UI позволяет загрузить avatar и посмотреть gallery пользователя.

## Input Model

### Headers

* `X-User-ID` для mutate endpoints
* строка `1..255`
* allowlist pattern, например:

    * `^[a-zA-Z0-9._@:-]+$`

### Query params

* `size`:

    * `original`
    * `100x100`
    * `300x300`
* default: `original`

### Upload body

* multipart/form-data
* поле `file`

### Assumption

`X-User-ID` pattern должен быть достаточно строгим для защиты от мусорных значений, но не превращаться в полноценную identity model.

## Output Model

### Upload / list / metadata status

Только:

* `processing`
* `completed`
* `failed`

### File endpoints

* binary response
* `Content-Type` по variant:

    * original — исходный mime
    * thumbs — `image/jpeg`

### Error response

```json
{
  "error": {
    "code": "string",
    "message": "string",
    "details": {}
  }
}
```

## State Model

### Internal record state

Рекомендуемая внутренняя модель:

* `processing_status`:

    * `pending`
    * `completed`
    * `failed`
* `original_available`: bool
* `thumb_100_available`: bool
* `thumb_300_available`: bool
* `deleted_at`: nullable timestamp

### External status mapping

* if `deleted_at != null` → resource invisible
* else if processing failed OR required original missing in storage → `failed`
* else if thumbs not yet completed → `processing`
* else → `completed`

## Architecture

### Components

* HTTP server
* PostgreSQL
* MinIO
* RabbitMQ
* Worker

### Interaction model

* server writes DB + storage
* server publishes events
* worker consumes events
* worker mutates DB + storage
* web pages reuse same application services

### RabbitMQ topology

* exchange: `avatars` (topic)
* routing keys:

    * `avatar.uploaded`
    * `avatar.delete_requested`

### Queues

* `avatars.uploads`
* `avatars.deletes`

## CLI Interface

```bash
avatars-service server
avatars-service worker
avatars-service migrate up
avatars-service migrate down
avatars-service migrate status
```

### Execution policy

* one process = one responsibility
* config through env
* fail-fast on invalid config
* graceful shutdown on SIGTERM/SIGINT

## Project Structure

```text
cmd/avatars-service
internal/app
internal/config
internal/domain
internal/http
internal/service
internal/repository/postgres
internal/storage/minio
internal/broker/rabbitmq
internal/imageproc
internal/worker
internal/web
migrations
tests
```

## Tech Stack

* Go 1.21+
* Chi
* PostgreSQL
* pgx + sqlc
* MinIO
* RabbitMQ
* amqp091-go
* imaging
* golang-migrate
* slog or zap
* testify
* testcontainers-go
* golangci-lint
* Docker / Docker Compose

## NFR

### Reliability

* graceful degradation on broker/storage issues where specified
* worker idempotency basic level
* retry minimal with backoff

### Performance

* no pagination for MVP
* thumbnails async
* direct streaming from MinIO to response where possible

### Maintainability

* clear service boundaries
* SQL explicit
* no hidden ORM behavior
* shared domain rules for API and web

### Observability

* structured logs
* access logs
* health endpoint

### Security

* upload validation by magic bytes
* constrained user_id pattern
* no direct S3 URLs exposed

## Risks

1. No outbox can leave records stuck in `failed`.
2. Storage drift not automatically repaired.
3. Public read endpoints may be abused.
4. Async delete can leave orphan files if broker/downstream fails.
5. Web gallery and API list have intentionally different filters; easy to implement incorrectly.

## Assumptions

1. Один binary с subcommands допустим и предпочтителен.
2. RabbitMQ topology остаётся минимальной: один exchange и две очереди.
3. Внутренняя DB schema может быть расширена bool-флагами доступности variants.
4. Web templates обслуживаются тем же сервером без отдельного frontend runtime.
5. Для MVP не нужен transactional outbox.
6. Idempotency достигается через state checks + deterministic object keys.
7. Отсутствие original в storage не ломает metadata endpoint.
8. Gallery page использует только записи с доступным original.
9. Healthcheck — connectivity level, не deep business validation.
10. JSON structured logging в stdout достаточно для MVP.

---

## Appendix: Recommended DB Schema Adjustment

Базовую схему из входа стоит упростить для runtime-логики чтения и fallback:

```sql
CREATE TABLE avatars (
    id UUID PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    original_mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,

    original_s3_key VARCHAR(500) NOT NULL,
    thumb_100_s3_key VARCHAR(500),
    thumb_300_s3_key VARCHAR(500),

    original_available BOOLEAN NOT NULL DEFAULT TRUE,
    thumb_100_available BOOLEAN NOT NULL DEFAULT FALSE,
    thumb_300_available BOOLEAN NOT NULL DEFAULT FALSE,

    processing_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    publish_failed BOOLEAN NOT NULL DEFAULT FALSE,

    width INT,
    height INT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_avatars_user_created_active
ON avatars(user_id, created_at DESC)
WHERE deleted_at IS NULL;

CREATE INDEX idx_avatars_processing_active
ON avatars(processing_status, publish_failed)
WHERE deleted_at IS NULL;
```

Это решение лучше исходного `thumbnail_s3_keys JSONB` для MVP, потому что:

* меньше runtime parsing
* проще SQL-отбор по доступности variants
* проще fallback
* проще тестирование
* меньше риск ошибок в handler logic

Итоговое архитектурное решение: **простой Go-монолит с subcommands `server/worker/migrate`, PostgreSQL + MinIO + RabbitMQ, explicit SQL, без outbox и без лишних сервисов.**
