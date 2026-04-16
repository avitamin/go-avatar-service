# Repository Guidelines

## Project Structure & Module Organization

Это Go-модуль `go-avatar-service`. Основная точка входа находится в `cmd/avatars-service` и поддерживает subcommands `server`, `worker`, `migrate`. Старые `cmd/server` и `cmd/worker` оставлены как thin compatibility wrappers. `cmd/avatar-contract-tests/main.go` собирает black-box runner контрактных smoke-тестов HTTP API. Веб-интерфейс представлен файлом `web/static/index.html`. Приватную логику держите в `internal/` по структуре из актуальной спеки v1: `http/`, `service/`, `repository/`, `storage/`, `broker/`, `domain/`, `config/`, `worker`. Публичный код кладите в `pkg/` только если он действительно рассчитан на переиспользование вне сервиса.

## AI Agent Prompts

Промты и роли для AI-агентов находятся в `docs/prompts/`. Перед планированием, реализацией, ревью или тестированием задач читайте `docs/prompts/README.md` и `docs/prompts/context/project.md`.

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
- `make test`, `make build`, `make build-server`, `make build-worker`, `make build-contract-tests`: короткие Makefile-цели для базовых команд.
- `make run-server`: запускает server с локальным default `HTTP_ADDR=:18080`.
- `make contract-tests`: собирает и запускает contract smoke runner с локальным default `BASE_URL=http://localhost:18080`.
- `BASE_URL=http://localhost:8080 make contract-tests`: запускает contract smoke runner против явно указанного сервиса, например compose-порта.
- `go test ./...`: запускает все Go-тесты, включая self-tests contract runner'а.

В `.idea/runConfigurations/` сохранены shared JetBrains конфигурации: `Server`, `Worker`, `Avatar Contract Tests`, `Make Test`, `Make Build Contract Tests`, `Make Contract Tests`. JetBrains конфигурации считаются локальными и используют `http://localhost:18080`, чтобы не занимать compose-порт `8080`.

Docker Compose присутствует и публикует server на `http://localhost:8080`. Runtime adapters PostgreSQL/MinIO/RabbitMQ еще не подключены к bootstrap, поэтому текущий server/worker используют in-memory core.

## Coding Style & Naming Conventions

Перед коммитом форматируйте Go-код через `gofmt`. Разделяйте ответственность: обработка HTTP-запросов в handlers, бизнес-логика в services, хранение данных в repositories, доменные типы в domain. Следуйте Go-неймингу: экспортируемые идентификаторы в `PascalCase`, неэкспортируемые в `camelCase`, тесты в файлах `*_test.go`, директории команд называйте по бинарнику.

## Testing Guidelines

Разработка ведется через TDD: сначала формулируйте ожидаемое поведение тестом, убедитесь, что он падает по правильной причине, затем реализуйте минимальный код и выполните refactor при зеленых тестах. Используйте стандартный пакет `testing`, пока проект явно не выберет другой фреймворк. Unit-тесты размещайте рядом с кодом, например `internal/service/avatar_test.go`. Для валидации, HTTP handlers и storage edge cases предпочитайте table-driven tests. Интеграционные и e2e-тесты кладите в `tests/`, если им нужны реальные внешние сервисы. Contract smoke runner в `tests/contract` не импортирует `internal/` код и проверяет API только через HTTP.

## Commit & Pull Request Guidelines

История использует краткий Conventional Commit style, например `docs: добавить QWEN.md с описанием проекта`. Пишите короткие imperative subject lines с типами `feat:`, `fix:`, `docs:`, `test:` или `refactor:`. В PR добавляйте описание, ссылку на issue или задачу, результат `go test ./...`, а для изменений UI или API - скриншоты либо примеры запросов.

## Security & Configuration Tips

Не коммитьте `.env`, секреты, загруженные аватары и бинарники из `bin/`. Когда появится конфигурация, изолируйте ее загрузку в `internal/config`. Для uploads проверяйте размер, content type и storage path до сохранения.

## Language Preference

По умолчанию используйте русский для объяснений, ревью, рабочих заметок и contributor guidance. Английский оставляйте там, где он точнее: идентификаторы кода, команды, API names, commit type prefixes, сообщения ошибок и внешние протокольные термины.
