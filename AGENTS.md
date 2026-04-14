# Repository Guidelines

## Project Structure & Module Organization

Это Go-модуль `go-avatar-service`. Точки входа находятся в `cmd/`: `cmd/server/main.go` запускает HTTP-сервер на `:8080`, а `cmd/worker/main.go` запускает фоновый worker. Веб-интерфейс сейчас представлен файлом `web/static/index.html`. По мере развития проекта держите приватную логику в `internal/` по структуре из README: `handlers/`, `services/`, `repository/`, `domain/`, `config/`, `worker/`. Публичный код кладите в `pkg/` только если он действительно рассчитан на переиспользование вне сервиса.

## Build, Test, and Development Commands

- `go mod tidy`: синхронизирует зависимости модуля.
- `go run ./cmd/server`: запускает локальный HTTP-сервер.
- `go run ./cmd/worker`: запускает worker-процесс.
- `go build -o ./bin/server ./cmd/server`: собирает бинарник сервера.
- `go build -o ./bin/worker ./cmd/worker`: собирает бинарник worker.
- `go test ./...`: запускает все Go-тесты после их добавления.

Docker Compose упомянут в README как будущий сценарий, но compose-файла сейчас нет. Не опирайтесь на него, пока конфигурация не появится в репозитории.

## Coding Style & Naming Conventions

Перед коммитом форматируйте Go-код через `gofmt`. Разделяйте ответственность: обработка HTTP-запросов в handlers, бизнес-логика в services, хранение данных в repositories, доменные типы в domain. Следуйте Go-неймингу: экспортируемые идентификаторы в `PascalCase`, неэкспортируемые в `camelCase`, тесты в файлах `*_test.go`, директории команд называйте по бинарнику.

## Testing Guidelines

Используйте стандартный пакет `testing`, пока проект явно не выберет другой фреймворк. Unit-тесты размещайте рядом с кодом, например `internal/services/avatar_test.go`. Для валидации, handlers и storage edge cases предпочитайте table-driven tests. Интеграционные и e2e-тесты кладите в `tests/`, если им нужны реальные внешние сервисы.

## Commit & Pull Request Guidelines

История использует краткий Conventional Commit style, например `docs: добавить QWEN.md с описанием проекта`. Пишите короткие imperative subject lines с типами `feat:`, `fix:`, `docs:`, `test:` или `refactor:`. В PR добавляйте описание, ссылку на issue или задачу, результат `go test ./...`, а для изменений UI или API - скриншоты либо примеры запросов.

## Security & Configuration Tips

Не коммитьте `.env`, секреты, загруженные аватары и бинарники из `bin/`. Когда появится конфигурация, изолируйте ее загрузку в `internal/config`. Для uploads проверяйте размер, content type и storage path до сохранения.

## Language Preference

По умолчанию используйте русский для объяснений, ревью, рабочих заметок и contributor guidance. Английский оставляйте там, где он точнее: идентификаторы кода, команды, API names, commit type prefixes, сообщения ошибок и внешние протокольные термины.
