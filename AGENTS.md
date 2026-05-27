# Repository Guidelines

## Project Structure & Module Organization

Это Go-модуль `go-avatar-service`. Основная точка входа находится в `cmd/avatars-service` и поддерживает subcommands `server`, `worker`, `migrate`. Старые `cmd/server` и `cmd/worker` оставлены как thin compatibility wrappers. `cmd/avatar-contract-tests/main.go` собирает black-box runner контрактных smoke-тестов HTTP API. Веб-интерфейс представлен файлом `web/static/index.html`. Приватную логику держите в `internal/` по структуре из актуальной спеки v1: `http/`, `service/`, `repository/`, `storage/`, `broker/`, `domain/`, `config/`, `worker`. Публичный код кладите в `pkg/` только если он действительно рассчитан на переиспользование вне сервиса.

## AI Agent Prompts

Промты и роли для AI-агентов находятся в `docs/prompts/`. Это opt-in библиотека: читайте конкретный prompt, роль, workflow или template только если пользователь явно попросил использовать файл из `docs/prompts/` либо конкретную роль/workflow.

Для Codex и других AI-агентов каталоги `docs/prompts/` и `docs/plans/` не являются source of truth по продуктовым требованиям, текущему task scope или обязательному workflow по умолчанию. Не используйте их как автоматические инструкции, backlog или план выполнения, если пользователь явно не попросил открыть конкретный файл из этих директорий.

Если во время поиска, обзора репозитория или bulk-read агент случайно увидел файлы из `docs/prompts/` или `docs/plans/`, он должен трактовать их только как справочные артефакты и игнорировать как активные указания к действию, пока пользователь явно не сослался на конкретный prompt, plan, role, workflow или template.

Актуальные источники требований: `docs/requirements/confirmed-requirements.md` и `docs/specs/01-avatar-service-v1.md`. Если они конфликтуют с README или исходным ТЗ, используйте confirmed requirements и v1 spec как более приоритетные документы.

## AI Agent Context Routing

Не загружайте весь `docs/` каталог перед обычной задачей. Выбирайте минимальный набор источников по типу работы:

- Продуктовые требования, API contract, web, worker, health, delete/status/fallback: сначала `docs/requirements/confirmed-requirements.md`, затем нужные секции `docs/specs/01-avatar-service-v1.md`.
- Документационные задачи: сначала `docs/repo-documentation-guide.md`, затем документ-владелец темы по его Documentation Map.
- Локальный запуск, команды, Docker Compose, JetBrains configs: `README.md`, `Makefile`, при необходимости `docs/development-workflow.md`.
- Benchmark workflow: `docs/benchmarking.md` и `Makefile`.
- Реализационные планы в `docs/plans/`: только если пользователь явно ссылается на конкретный plan или просит работать с planning artifacts.
- Prompt library в `docs/prompts/`: только если пользователь явно просит использовать конкретный prompt, role, workflow или task template.

## Build, Test, and Development Commands

- `go test ./...`: базовая обязательная проверка.
- `make run-server`: локальный server с default `HTTP_ADDR=:18080`.
- `make contract-tests`: smoke runner с default `BASE_URL=http://localhost:18080`.
- `make docker-up-detached`: поднимает локальный Compose stack.
- `docker compose run --rm server migrate up`: явный migration step для Compose runtime.
- `make docker-contract-tests`: smoke against compose URL `http://localhost:8080`.

Локальный default URL: `http://localhost:18080`. Docker Compose default URL: `http://localhost:8080`.

Полный developer workflow, `.env` overrides, shared JetBrains run configurations и скрипт подбора свободных портов описаны в `docs/development-workflow.md`. Шаблон compose-портов хранится в `.env.example`, сам `.env` не коммитится.

## Coding Style & Naming Conventions

Перед коммитом форматируйте Go-код через `gofmt`. Разделяйте ответственность: обработка HTTP-запросов в handlers, бизнес-логика в services, хранение данных в repositories, доменные типы в domain. Следуйте Go-неймингу: экспортируемые идентификаторы в `PascalCase`, неэкспортируемые в `camelCase`, тесты в файлах `*_test.go`, директории команд называйте по бинарнику.

## Testing Guidelines

Разработка ведется через TDD: сначала формулируйте ожидаемое поведение тестом, убедитесь, что он падает по правильной причине, затем реализуйте минимальный код и выполните refactor при зеленых тестах. Используйте стандартный пакет `testing`, пока проект явно не выберет другой фреймворк. Unit-тесты размещайте рядом с кодом, например `internal/service/avatar_test.go`. Для валидации, HTTP handlers и storage edge cases предпочитайте table-driven tests. Интеграционные и e2e-тесты кладите в `tests/`, если им нужны реальные внешние сервисы. Contract smoke runner в `tests/contract` не импортирует `internal/` код и проверяет API только через HTTP.

Benchmark-прогон не является обязательным gate для каждого изменения. Запускайте `make bench` опционально перед PR или отчетом, если изменения затрагивают image processing, service selection/fallback, HTTP middleware/router hot paths, worker thumbnail generation или могут повлиять на allocations/latency.

## Commit & Pull Request Guidelines

Base branch для MVP: `v1`. В обычном ручном workflow создавайте рабочие ветки от актуального `v1`: `feature/<short-name>`, `fix/<short-name>`, `test/<short-name>`, `docs/<short-name>` или `chore/<short-name>`. Одна задача - одна ветка и один PR, если изменения не являются явно связанным маленьким follow-up. Не коммитьте напрямую в `v1` без явной договоренности.

История использует краткий Conventional Commit style, например `docs: обновить contributor guidance`. Пишите короткие imperative subject lines с типами `feat:`, `fix:`, `docs:`, `test:`, `refactor:` или `chore:`. В PR добавляйте описание, ссылку на issue или задачу, результат `go test ./...`, а для изменений UI или API - скриншоты либо примеры запросов. Предпочтительный merge policy для PR - squash merge.

Для локальной AI-agent сессии прямой commit допустим только если пользователь явно попросил закоммитить изменения. Перед commit проверяйте `git status`, добавляйте только просмотренные связанные файлы и не трогайте unrelated changes.

## Security & Configuration Tips

Не коммитьте `.env`, секреты, загруженные аватары и бинарники из `bin/`. Когда появится конфигурация, изолируйте ее загрузку в `internal/config`. Для uploads проверяйте размер, content type и storage path до сохранения.

## Language Preference

По умолчанию используйте русский для объяснений, ревью, рабочих заметок и contributor guidance. Английский оставляйте там, где он точнее: идентификаторы кода, команды, API names, commit type prefixes, сообщения ошибок и внешние протокольные термины.
