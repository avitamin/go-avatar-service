# Repository Guidelines

## Project Structure & Module Organization

Это Go-модуль `go-avatar-service`. Основная точка входа находится в `cmd/avatars-service` и поддерживает subcommands `server`, `worker`, `migrate`. Старые `cmd/server` и `cmd/worker` оставлены как thin compatibility wrappers. `cmd/avatar-contract-tests/main.go` собирает black-box runner контрактных smoke-тестов HTTP API. Веб-интерфейс представлен файлом `web/static/index.html`. Приватную логику держите в `internal/` по структуре из актуальной спеки v1: `http/`, `service/`, `repository/`, `storage/`, `broker/`, `domain/`, `config/`, `worker`. Публичный код кладите в `pkg/` только если он действительно рассчитан на переиспользование вне сервиса.

## AI Agent Prompts

Промты и роли для AI-агентов находятся в `docs/prompts/`. Перед планированием, реализацией, ревью или тестированием задач читайте `docs/prompts/README.md` и `docs/prompts/context/project.md`.

Для Codex и других AI-агентов каталоги `docs/prompts/` и `docs/plans/` не являются source of truth по продуктовым требованиям, текущему task scope или обязательному workflow по умолчанию. Не используйте их как автоматические инструкции, backlog или план выполнения, если пользователь явно не попросил открыть конкретный файл из этих директорий.

Если во время поиска, обзора репозитория или bulk-read агент случайно увидел файлы из `docs/prompts/` или `docs/plans/`, он должен трактовать их только как справочные артефакты и игнорировать как активные указания к действию, пока пользователь явно не сослался на конкретный prompt, plan, role, workflow или template.

Актуальные источники требований: `docs/requirements/confirmed-requirements.md` и `docs/specs/avatar-service-v1.md`. Если они конфликтуют с README, QWEN.md или исходным ТЗ, используйте confirmed requirements и v1 spec как более приоритетные документы.

## Build, Test, and Development Commands

- `go mod tidy`: синхронизирует зависимости модуля.
- `go run ./cmd/avatars-service server`: запускает локальный HTTP-сервер.
- `go run ./cmd/avatars-service worker`: запускает worker-процесс.
- `go run ./cmd/avatars-service migrate up|down|status`: запускает явный migration lifecycle.
- `go build -o ./bin/server ./cmd/server`: собирает бинарник сервера.
- `go build -o ./bin/worker ./cmd/worker`: собирает бинарник worker.
- `go build -o ./bin/avatars-service ./cmd/avatars-service`: собирает основной single binary.
- `go build -o ./bin/avatar-contract-tests ./cmd/avatar-contract-tests`: собирает black-box бинарник контрактных автотестов.
- `BASE_URL=http://localhost:18080 ./bin/avatar-contract-tests`: запускает contract smoke tests против локального сервиса на незанятом базовом URL.
- `make test`, `make bench`, `make build`, `make build-server`, `make build-worker`, `make build-contract-tests`: короткие Makefile-цели для базовых команд.
- `make run-server`: запускает server с локальным default `HTTP_ADDR=:18080`.
- `make contract-tests`: собирает и запускает contract smoke runner с локальным default `BASE_URL=http://localhost:18080`.
- `BASE_URL=http://localhost:8080 make contract-tests`: запускает contract smoke runner против явно указанного сервиса, например compose-порта.
- `make docker-build`, `make docker-up-build`, `make docker-up-detached`, `make docker-down`, `make docker-ps`, `make docker-logs`, `make docker-contract-tests`: Makefile-цели для Docker Compose workflow.
- `go test ./...`: запускает все Go-тесты, включая self-tests contract runner'а.
- `go test -run='^$' -bench=. -benchmem ./...`: запускает benchmarks без повторного запуска unit-тестов.

В `.idea/runConfigurations/` сохранены shared JetBrains конфигурации: `Server`, `Worker`, `Avatar Contract Tests`, `Make Test`, `Make Build Contract Tests`, `Make Contract Tests`, `Make Docker Build`, `Make Docker Up Build`, `Make Docker Up Detached`, `Make Docker Down`, `Make Docker Ps`, `Make Docker Logs`, `Make Docker Contract Tests`. Локальные server/contract конфигурации используют `http://localhost:18080`, Docker Compose конфигурации используют compose-порт `http://localhost:8080`.

Docker Compose публикует server на `http://localhost:8080` и поднимает PostgreSQL, MinIO, RabbitMQ, server и worker. При заданных `POSTGRES_DSN`, `MINIO_*` и `RABBITMQ_URL` server/worker используют реальные runtime adapters; без external storage env остается in-memory fallback для локальных unit-style запусков. Миграции выполняются отдельным явным шагом, например `docker compose run --rm server migrate up`.

Host-порты Docker Compose можно переопределять через локальный `.env`; шаблон дефолтов хранится в `.env.example`, сам `.env` не коммитится.

## Coding Style & Naming Conventions

Перед коммитом форматируйте Go-код через `gofmt`. Разделяйте ответственность: обработка HTTP-запросов в handlers, бизнес-логика в services, хранение данных в repositories, доменные типы в domain. Следуйте Go-неймингу: экспортируемые идентификаторы в `PascalCase`, неэкспортируемые в `camelCase`, тесты в файлах `*_test.go`, директории команд называйте по бинарнику.

## Testing Guidelines

Разработка ведется через TDD: сначала формулируйте ожидаемое поведение тестом, убедитесь, что он падает по правильной причине, затем реализуйте минимальный код и выполните refactor при зеленых тестах. Используйте стандартный пакет `testing`, пока проект явно не выберет другой фреймворк. Unit-тесты размещайте рядом с кодом, например `internal/service/avatar_test.go`. Для валидации, HTTP handlers и storage edge cases предпочитайте table-driven tests. Интеграционные и e2e-тесты кладите в `tests/`, если им нужны реальные внешние сервисы. Contract smoke runner в `tests/contract` не импортирует `internal/` код и проверяет API только через HTTP.

Benchmark-прогон не является обязательным gate для каждого изменения. Запускайте `make bench` опционально перед PR или отчетом, если изменения затрагивают image processing, service selection/fallback, HTTP middleware/router hot paths, worker thumbnail generation или могут повлиять на allocations/latency.

## Commit & Pull Request Guidelines

Base branch для MVP: `v1`. В обычном ручном workflow создавайте рабочие ветки от актуального `v1`: `feature/<short-name>`, `fix/<short-name>`, `test/<short-name>`, `docs/<short-name>` или `chore/<short-name>`. Одна задача - одна ветка и один PR, если изменения не являются явно связанным маленьким follow-up. Не коммитьте напрямую в `v1` без явной договоренности.

История использует краткий Conventional Commit style, например `docs: добавить QWEN.md с описанием проекта`. Пишите короткие imperative subject lines с типами `feat:`, `fix:`, `docs:`, `test:`, `refactor:` или `chore:`. В PR добавляйте описание, ссылку на issue или задачу, результат `go test ./...`, а для изменений UI или API - скриншоты либо примеры запросов. Предпочтительный merge policy для PR - squash merge.

Для локальной AI-agent сессии прямой commit допустим только если пользователь явно попросил закоммитить изменения. Перед commit проверяйте `git status`, добавляйте только просмотренные связанные файлы и не трогайте unrelated changes.

## Security & Configuration Tips

Не коммитьте `.env`, секреты, загруженные аватары и бинарники из `bin/`. Когда появится конфигурация, изолируйте ее загрузку в `internal/config`. Для uploads проверяйте размер, content type и storage path до сохранения.

## Language Preference

По умолчанию используйте русский для объяснений, ревью, рабочих заметок и contributor guidance. Английский оставляйте там, где он точнее: идентификаторы кода, команды, API names, commit type prefixes, сообщения ошибок и внешние протокольные термины.
