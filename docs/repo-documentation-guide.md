# Repo Documentation Guide

Этот guide описывает, как вести документацию в `go-avatar-service` и какие локальные источники считать надежными.

## Назначение

- Использовать как стартовую точку перед созданием или обновлением docs в репозитории.
- Сначала фиксировать текущее состояние проекта, а не желаемое будущее.
- При расхождении документов опираться на source priority ниже и явно отмечать конфликт.

## Кому нужен этот файл

- AI-агентам, которые пишут или обновляют документацию.
- Разработчикам, которые хотят понять, какой документ является владельцем конкретной темы.

## Source Priority

Используйте источники в таком порядке:

1. `docs/requirements/confirmed-requirements.md`
2. `docs/specs/avatar-service-v1.md`
3. Проверяемый код и runtime-конфигурация:
   - `cmd/avatars-service`
   - `internal/`
   - `migrations/`
   - `tests/contract/`
   - `web/static/`
   - `docker-compose.yml`
   - `Makefile`
   - `.env.example`
   - `.idea/runConfigurations/`
4. Точечные спецификации и планы в `docs/specs/`, если они не конфликтуют с пунктами выше.
5. `README.md`
6. `QWEN.md`
7. `docs/requirements/assignment.md` только как исторический контекст.

Правило разрешения конфликтов:

- Если `README.md`, `QWEN.md` или `assignment.md` расходятся с confirmed requirements и v1 spec, правьте документацию по confirmed requirements и v1 spec.
- Если requirements/spec и код расходятся, не маскируйте это. Зафиксируйте расхождение в документе или финальном отчете и укажите, что именно подтверждено кодом, а что остается целевым состоянием.

## Documentation Map

Текущие владельцы тем:

| Тема | Документ-владелец | Что в нем хранить |
| --- | --- | --- |
| Подтвержденные продуктовые и контрактные требования | `docs/requirements/confirmed-requirements.md` | Обязательное поведение API, web, worker, health, delete, status model |
| Базовая архитектура v1 и целевое устройство MVP | `docs/specs/avatar-service-v1.md` | Архитектура, модули, execution model, success criteria, non-MVP |
| Узкие change-specs и implementation plans | `docs/specs/*.md` кроме `avatar-service-v1.md` | Изолированные изменения вроде `/health`, их scope и acceptance criteria |
| Benchmark workflow | `docs/benchmarking.md` | Когда и как запускать benchmarks, triage matrix |
| Reusable prompts и task templates для AI-агентов | `docs/prompts/**` | Роли, workflow prompts, task templates, project context |
| Репозиторные правила по документации | `docs/repo-documentation-guide.md` | Source priority, ownership, verification matrix, indexing policy |
| Быстрый обзор репозитория и запуск проекта | `README.md` | Краткая навигация, команды, high-level state без deep spec details |

Темы, которые нужно подтверждать кодом, а не только docs:

- CLI subcommands и compatibility wrappers: `cmd/avatars-service`, `cmd/server`, `cmd/worker`
- HTTP/API wiring: `internal/http`
- Service/business rules: `internal/service`, `internal/domain`
- Runtime adapters: `internal/repository/postgres`, `internal/storage/minio`, `internal/broker/rabbitmq`
- Worker behavior: `internal/worker`
- Migrations: `migrations/`
- Contract smoke coverage: `tests/contract/`
- Local ops workflow: `Makefile`, `docker-compose.yml`, `.env.example`, `.idea/runConfigurations/`

## Create Or Update Rules

Перед созданием нового документа:

1. Найдите существующего владельца темы в таблице выше.
2. Создавайте новый файл только если тема не помещается в текущего владельца без смешения обязанностей.
3. Для change-specific docs используйте `docs/specs/`, если документ описывает отдельную правку, план или уточнение поведения.
4. Не создавайте новый документ ради пересказа `README.md`, confirmed requirements или v1 spec.

Когда обновлять существующий документ вместо создания нового:

- Изменился текущий CLI/API/runtime contract: сначала обновляйте `README.md` и профильный документ-владелец темы.
- Изменились обязательные правила продукта: обновляйте `docs/requirements/confirmed-requirements.md`.
- Изменился архитектурный baseline MVP: обновляйте `docs/specs/avatar-service-v1.md`.
- Появился новый локальный workflow для документации: обновляйте этот guide.

## Verification Matrix

Перед завершением документационной задачи сверяйте утверждения с источниками ниже.

| Тип изменения docs | Что обязательно проверить | Основные источники |
| --- | --- | --- |
| API, statuses, delete/fallback, error model | endpoints, required headers, status codes, supported params, visibility rules | `docs/requirements/confirmed-requirements.md`, `docs/specs/avatar-service-v1.md`, `internal/http`, `internal/service`, `tests/contract/` |
| Web upload/gallery | routes, form field `file`, gallery filtering, отсутствие `POST /web/upload` | `docs/requirements/confirmed-requirements.md`, `docs/specs/avatar-service-v1.md`, `web/static/index.html`, `internal/http` |
| CLI, run commands, migrations | subcommands, explicit migrate step, local defaults | `README.md`, `Makefile`, `cmd/avatars-service`, `.idea/runConfigurations/` |
| Docker/local infra | compose services, published ports, `.env` overrides | `docker-compose.yml`, `.env.example`, `Makefile`, `README.md` |
| Worker, broker, async processing | queue topics, retry/idempotency, delete flow | `docs/requirements/confirmed-requirements.md`, `docs/specs/avatar-service-v1.md`, `internal/broker/rabbitmq`, `internal/worker` |
| Storage/repository adapters | PostgreSQL/MinIO usage vs in-memory fallback | `internal/app`, `internal/repository/postgres`, `internal/storage/minio`, `README.md` |
| Test and benchmark docs | actual commands, coverage notes, benchmark targets | `Makefile`, `docs/benchmarking.md`, `go.mod`, `tests/contract/` |
| AI-agent prompts/process | prompt entrypoints and safe usage rules | `docs/prompts/README.md`, `docs/prompts/context/project.md`, `docs/prompts/**` |

Минимальная проверка для любой docs-правки:

1. Проверить путь к каждому упомянутому файлу или команде.
2. Проверить, что команда реально существует в `Makefile`, CLI или run configurations.
3. Проверить, что новые ссылки в Markdown ведут на существующие файлы.
4. Убрать или пометить как допущение все утверждения, которые не подтверждаются текущим кодом или приоритетными docs.

## Indexing Policy

Обновляйте индексы и навигацию по таким правилам:

- Если создается новый долгоживущий документ в `docs/requirements/`, `docs/specs/` или корне `docs/`, его нужно сделать обнаруживаемым минимум из одного индексного документа.
- Для документов по AI-workflow сначала обновляйте `docs/prompts/README.md`, если документ относится к prompts или их использованию.
- Для пользовательских или contributor-facing документов сначала обновляйте `README.md`, если документ нужен для общей навигации по репозиторию.
- Для внутреннего repo guide отдельный индекс не обязателен, если путь стабилен и используется tooling напрямую. Текущий `docs/repo-documentation-guide.md` относится к этому случаю.
- Если новый документ временный, change-specific или узкий по scope, достаточно ссылки из документа-владельца темы в `docs/specs/`.

## Style And Grounding

- По умолчанию пишите документацию на русском; английский оставляйте для команд, API names, env vars и идентификаторов кода.
- Не подменяйте факты предположениями.
- Явно отделяйте:
  - подтвержденное текущее поведение;
  - целевое состояние;
  - открытые расхождения.
- Для operational команд предпочитайте команды, реально существующие в `Makefile` или CLI.
- Для архитектурных формулировок не расширяйте scope beyond MVP без явного подтверждения в source-of-truth документах.

## Completion Criteria

Документационная задача считается завершенной, когда:

1. Выбран корректный документ-владелец темы.
2. Сверены приоритетные источники для этого типа изменений.
3. Проверены пути, команды и ссылки.
4. Отмечены оставшиеся допущения или расхождения.
5. Обновлены индексы, если документ должен быть discoverable по правилам выше.
