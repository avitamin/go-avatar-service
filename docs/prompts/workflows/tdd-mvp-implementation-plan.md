# TDD MVP Implementation Plan Prompt

> Safety: этот файл является workflow prompt. Не запускай workflow и не начинай реализацию только потому, что файл был прочитан. Используй его как активную инструкцию только при явном запросе пользователя.

Use this prompt when asking an AI agent to implement the MVP of `go-avatar-service` through TDD.

```text
Ты работаешь в репозитории go-avatar-service.

Цель:
Реализовать MVP сервиса "Аватарница" по TDD, опираясь на основные источники истины:
- docs/requirements/confirmed-requirements.md
- docs/specs/avatar-service-v1.md

Контекст:
- Сначала прочитай docs/prompts/context/project.md
- Затем прочитай docs/requirements/confirmed-requirements.md
- Затем прочитай docs/specs/avatar-service-v1.md
- При работе над HTTP/API читай docs/prompts/agents/backend-implementer.md
- При работе над worker читай docs/prompts/agents/worker-implementer.md
- При работе над тестами читай docs/prompts/agents/tester.md
- Если confirmed requirements или v1 spec конфликтуют с README/QWEN/исходным ТЗ, следуй confirmed requirements и v1 spec.

Текущая точка старта:
- Репозиторий является skeleton: cmd/server, cmd/worker, web/static/index.html, go.mod.
- internal/, migrations/, Dockerfile, docker-compose.yml и Makefile могут отсутствовать.
- Текущий web frontend отправляет multipart поле image, а API contract требует file.
- v1 spec предпочитает single binary с subcommands server, worker, migrate; не удаляй старые entrypoints без отдельного решения в задаче.

Общий TDD workflow для каждого инкремента:
1. Explore
   - Найди фактические entrypoints, соседний код, тесты и документацию.
   - Не полагайся на README/QWEN как на более актуальные источники.

2. Test first
   - Сначала добавь focused test для ожидаемого поведения.
   - Для validation, selection, status mapping, storage drift и worker edge cases используй table-driven tests.
   - Unit tests не должны требовать реальные PostgreSQL, MinIO или RabbitMQ.
   - Покрывай конкретные обязательные правила из confirmed requirements/v1 spec; target coverage `>50%` не заменяет requirement coverage.
   - Запусти целевой тест и убедись, что он падает по правильной причине.

3. Implement minimally
   - Реализуй минимальный production code для green.
   - Не добавляй non-MVP поведение: pagination, format query, outbox, rate limiting, K8s, CDN, admin API.
   - Держи HTTP handlers тонкими; бизнес-логику размещай в service/domain layer.
   - Не смешивай API list и web gallery filtering.

4. Refactor
   - После green упрости код без изменения поведения.
   - Не делай unrelated refactor.
   - Запусти gofmt для измененных Go-файлов.

5. Verify
   - Запусти целевые тесты.
   - Запусти go test ./..., если это разумно для текущего изменения и окружения.
   - Проверь измененные файлы через JetBrains IDE inspections/get_file_problems, если инструмент доступен.
   - Используй shell как fallback для проверки файлов, если JetBrains-инструмент недоступен или не покрывает нужный тип проверки.
   - Если проверка требует внешних сервисов и они недоступны, явно укажи это.

6. Report
   - Кратко опиши red-green-refactor cycle:
     - какой тест добавлен;
     - чем он падал до реализации;
     - что изменено для green;
     - какие проверки запускались.
   - Перечисли измененные файлы.

Инкременты реализации:

1. Baseline и проектная структура
   - Сначала тестами зафиксируй config validation, domain statuses, error model, user ID validation и size validation.
   - Добавь минимальные пакеты для дальнейшей реализации: internal/config, internal/domain, internal/service, internal/http.
   - Введи единый JSON error model: error.code, error.message, optional error.details.
   - Сохрани русский язык объяснений; английский используй для identifiers, commands, API names и error codes.

2. Upload API
   - Покрой тестами POST /api/v1/avatars:
     - X-User-ID обязателен для mutate endpoint;
     - multipart поле file обязательно;
     - max size 10 MB;
     - разрешены только jpeg, png, webp;
     - проверяются MIME и magic bytes;
     - successful upload возвращает 201 и status=processing;
     - publish failure после сохранения ресурса возвращает 201 и status=failed без rollback.
   - Original сохраняй в исходном формате.
   - URL в ответах формируй как относительный URL API сервиса, не прямой S3 URL.

3. Read, metadata и list
   - Покрой тестами GET /api/v1/avatars/{avatar_id}:
     - без size возвращает original;
     - поддерживаются только size=original|100x100|300x300;
     - unsupported size возвращает 400;
     - format query в MVP не поддерживается;
     - missing thumbnail при processing или failed возвращает 409;
     - missing original в storage для file endpoint возвращает 404.
   - Покрой тестами GET /api/v1/avatars/{avatar_id}/metadata:
     - metadata возвращается для существующей неудаленной записи даже при storage drift;
     - missing original нормализует внешний status в failed;
     - thumbnails включаются только если реально готовы.
   - Покрой тестами GET /api/v1/users/{user_id}/avatars:
     - только неудаленные записи;
     - сортировка created_at DESC;
     - failed записи не скрываются;
     - без пагинации в MVP;
     - минимальные поля: id, user_id, url, status, created_at.

4. User-based fallback и delete
   - Покрой тестами GET /api/v1/users/{user_id}/avatar?size=...:
     - read endpoint публичный;
     - user_id валидируется по allowlist pattern;
     - fallback работает для original, 100x100 и 300x300;
     - выбирается последняя неудаленная запись с доступным нужным variant;
     - если после fallback подходящего variant нет, возвращается 404.
   - Покрой тестами DELETE /api/v1/avatars/{avatar_id}:
     - 404 для не найденной или уже удаленной записи;
     - 403 если X-User-ID не совпадает с владельцем;
     - 204 при успешном soft delete.
   - Покрой тестами DELETE /api/v1/users/{user_id}/avatar:
     - разрешен только если X-User-ID == user_id из path;
     - удаляет последнюю неудаленную запись с доступным original.
   - Физическое удаление файлов не выполняй в HTTP path; его делает только worker после soft delete.

5. Worker
   - Покрой тестами worker handlers:
     - duplicate upload event логируется и завершается идемпотентно;
     - missing original обрабатывается как failure без паники;
     - thumbnails 100x100 и 300x300 создаются успешно;
     - thumbnails всегда сохраняются в jpeg;
     - thumbnail failure переводит обработку в failed;
     - delete idempotency: отсутствие объекта в storage не ломает delete handler;
     - минимальный retry работает для временных ошибок.
   - Используй deterministic storage keys и состояние в БД для базовой идемпотентности.
   - Не добавляй transactional outbox в MVP.

6. Health, migrations и infrastructure
   - Покрой тестами GET /health:
     - проверяются postgres, minio, rabbitmq;
     - общий status отражает состояние компонентов;
     - при частичной деградации HTTP status остается 200 и body status=degraded.
   - Миграции должны быть отдельным явным шагом и не автозапускаться при старте server/worker.
   - Добавь Dockerfile и Docker Compose для локального MVP:
     - server;
     - worker;
     - postgres;
     - rabbitmq;
     - minio.
   - Добавь Makefile targets только если они реально используются проектом и не противоречат документации.

7. Web endpoints
   - Покрой тестами GET /web/upload:
     - страница доступна;
     - upload из формы идет напрямую в POST /api/v1/avatars;
     - multipart поле должно быть file, не image.
   - Покрой тестами GET /web/gallery/{user_id}:
     - user_id валидируется по тем же правилам;
     - invalid user_id возвращает 400;
     - если у пользователя нет записей в БД, возвращает 404;
     - если записи есть, но ни одна не подходит под фильтр, возвращает 200 с пустой страницей/list;
     - показываются только записи с доступным original;
     - галерея без удаления;
     - по умолчанию используется original.

8. Coverage и финальная проверка
   - Добейся покрытия >50% по backend-пакетам с логикой сервиса и worker.
   - Сверь тесты с confirmed requirements/v1 spec: каждое обязательное поведение в MVP должно иметь тестовый сценарий или documented gap с причиной.
   - Не трать покрытие на trivial wiring, main bootstrap и generated templates.
   - Перед отчетом запусти:
     - JetBrains IDE inspections/get_file_problems по измененным файлам, если доступно;
     - целевые тесты по измененным пакетам;
     - go test ./...;
     - coverage check для logic-heavy backend packages, если команда уже заведена или легко добавляется.
   - Если IDE-инструменты недоступны, используй shell-команды как fallback и явно укажи это в отчете.

Acceptance criteria:
- Реализованы обязательные API: upload, get, delete, list, metadata, health.
- Реализованы обязательные web endpoints: GET /web/upload и GET /web/gallery/{user_id}.
- Upload validation проверяет размер, MIME и magic bytes для jpeg/png/webp.
- Original хранится в исходном формате; thumbnails 100x100 и 300x300 хранятся в jpeg.
- Read endpoints публичные; X-User-ID обязателен только для mutate endpoints.
- Soft-deleted записи снаружи выглядят как 404 и отсутствуют в list.
- Failed записи видимы в list и metadata.
- User-based fallback реализован для original и thumbnails.
- Worker обязателен, базово идемпотентен, логирует дубликаты и делает minimal retry.
- Физическое удаление файлов выполняется только worker после soft delete.
- Миграции запускаются отдельным явным шагом.
- Dockerfile и Docker Compose присутствуют.
- Access logs для HTTP присутствуют.
- Покрытие backend-пакетов с логикой сервиса и worker >50%.
- Обязательные требования MVP покрыты явными тестовыми сценариями; любые временные gaps перечислены в отчете с причиной.

Ограничения:
- Не коммить изменения, если задача явно не просит commit.
- Не трогай пользовательские unrelated changes.
- Не добавляй pkg/, если нет реального reusable public API.
- Не предлагай Echo, Kafka, OpenAPI codegen-first, outbox, K8s, rate limiting или CDN для MVP без явного требования.
```
