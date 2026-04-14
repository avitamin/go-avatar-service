# Repository Guidelines

## Project Structure & Module Organization

Это Go-модуль `go-avatar-service`. Точки входа сейчас находятся в `cmd/`: `cmd/server/main.go` запускает HTTP-сервер на `:8080`, `cmd/worker/main.go` запускает фоновый worker, а `cmd/avatar-contract-tests/main.go` собирает black-box runner контрактных smoke-тестов HTTP API. Веб-интерфейс сейчас представлен файлом `web/static/index.html`. По мере развития проекта держите приватную логику в `internal/` по структуре из актуальной спеки v1: `http/`, `service/`, `repository/`, `storage/`, `broker/`, `domain/`, `config/`, `worker/`. Публичный код кладите в `pkg/` только если он действительно рассчитан на переиспользование вне сервиса.

## AI Agent Prompts

Промты и роли для AI-агентов находятся в `docs/prompts/`. Перед планированием, реализацией, ревью или тестированием задач читайте `docs/prompts/README.md` и `docs/prompts/context/project.md`.

Актуальные источники требований: `docs/requirements/confirmed-requirements.md` и `docs/specs/avatar-service-v1.md`. Если они конфликтуют с README, QWEN.md или исходным ТЗ, используйте confirmed requirements и v1 spec как более приоритетные документы.

## Build, Test, and Development Commands

- `go mod tidy`: синхронизирует зависимости модуля.
- `go run ./cmd/server`: запускает локальный HTTP-сервер.
- `go run ./cmd/worker`: запускает worker-процесс.
- `go build -o ./bin/server ./cmd/server`: собирает бинарник сервера.
- `go build -o ./bin/worker ./cmd/worker`: собирает бинарник worker.
- `go build -o ./bin/avatar-contract-tests ./cmd/avatar-contract-tests`: собирает black-box бинарник контрактных автотестов.
- `BASE_URL=http://localhost:8080 ./bin/avatar-contract-tests`: запускает contract smoke tests против уже поднятого сервиса.
- `make test`, `make build-server`, `make build-worker`, `make build-contract-tests`: короткие Makefile-цели для базовых команд.
- `BASE_URL=http://localhost:8080 make contract-tests`: собирает и запускает contract smoke runner.
- `go test ./...`: запускает все Go-тесты, включая self-tests contract runner'а.

В `.idea/runConfigurations/` сохранены shared JetBrains конфигурации: `Server`, `Worker`, `Avatar Contract Tests`, `Make Test`, `Make Build Contract Tests`, `Make Contract Tests`.

Docker Compose упомянут в README как будущий сценарий, но compose-файла сейчас нет. Не опирайтесь на него, пока конфигурация не появится в репозитории.

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
