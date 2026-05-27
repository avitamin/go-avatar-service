# Переход миграций на `golang-migrate/migrate`

## Статус реализации

Статус: реализовано в коде приложения.

Подтверждено:

- `go.mod` содержит `github.com/golang-migrate/migrate/v4`.
- `internal/app/migrate.go` реализует `avatars-service migrate up|down|status` через `golang-migrate`.
- `migrate status` читает version/dirty state migration engine и возвращает `pending`, `ok version=<n> dirty=false` или `dirty version=<n>`.
- `README.md` и `Makefile` описывают explicit migration step без автозапуска при старте `server` или `worker`.
- Unit/integration coverage находится в `internal/app/migrate_test.go` и `internal/app/migrate_integration_test.go`.

## Источники информации

- Подтвержденные требования: [docs/requirements/confirmed-requirements.md](../requirements/confirmed-requirements.md)
- Архитектурный baseline v1: [docs/specs/01-avatar-service-v1.md](../specs/01-avatar-service-v1.md)
- Repo guide для docs: [docs/repo-documentation-guide.md](../repo-documentation-guide.md)
- Текущая CLI-реализация миграций: `internal/app/app.go`
- Текущие CLI tests: `internal/app/app_test.go`
- Текущие operational команды: `Makefile`, `README.md`
- Текущие SQL-файлы: `migrations/001_init.up.sql`, `migrations/001_init.down.sql`
- Официальный upstream `golang-migrate/migrate`: `README.md`, `MIGRATIONS.md`

## Зачем нужен переход

Сейчас `avatars-service migrate up|down|status` реализован вручную:

- `up` и `down` жёстко привязаны к файлам `migrations/001_init.up.sql` и `migrations/001_init.down.sql`
- `status` определяется эвристикой `SELECT to_regclass('public.avatars') IS NOT NULL`
- механизм не масштабируется на несколько версий миграций
- нет штатного понятия applied version, dirty state и migration ordering

При этом в `docs/specs/01-avatar-service-v1.md` `golang-migrate` уже зафиксирован как целевой migration tool, а confirmed requirements требуют отдельный явный migration step без автозапуска при старте `server` или `worker`.

## Текущее состояние

Подтверждено по коду и docs:

- CLI contract уже существует и должен остаться: `avatars-service migrate up|down|status`
- миграции лежат в каталоге `migrations/`
- файл `migrations/001_init.up.sql` уже совместим по naming scheme с `golang-migrate`, потому что использует versioned pair `001_init.up.sql` / `001_init.down.sql`
- `001_init.up.sql` идемпотентен для bootstrap существующих локальных БД, так как использует `CREATE TABLE IF NOT EXISTS` и `CREATE INDEX IF NOT EXISTS`
- `001_init.down.sql` удаляет только таблицу `avatars`

## Целевое состояние

После перехода сервис должен:

- продолжать использовать существующий CLI contract `avatars-service migrate up|down|status`
- выполнять миграции через библиотеку `github.com/golang-migrate/migrate/v4`, а не через ручное чтение SQL-файлов
- читать source миграций из локального каталога `migrations/`
- хранить applied version и dirty state в служебной таблице, управляемой `golang-migrate`
- корректно работать при появлении `002_*`, `003_*` и следующих миграций без переписывания CLI-кода
- сохранять правило explicit step: `server` и `worker` не запускают миграции автоматически

## Архитектурные решения

### 1. Использовать library mode, а не внешний CLI binary

Целевое решение: встроить `golang-migrate` как Go dependency внутрь существующего subcommand `avatars-service migrate ...`.

Почему:

- текущий репозиторий уже закрепляет operational contract вокруг single binary `avatars-service`
- это не ломает `Makefile`, README и shared run configurations
- не требуется отдельная установка внешнего `migrate` binary на машине разработчика или в контейнере server

Следствие:

- публичный CLI repo не меняется
- внутренняя реализация `RunMigrate` переписывается на library API

### 2. Не менять каталог `migrations/`

`golang-migrate` поддерживает file source и versioned пары файлов. Текущий каталог `migrations/` уже подходит для этого контракта.

Решение:

- сохранить каталог `migrations/`
- не переносить файлы в новый путь
- использовать его как единственный source of truth для schema migrations

### 3. Не переименовывать `001_init.*`

Текущие файлы уже соответствуют шаблону `<version>_<name>.up.sql` и `<version>_<name>.down.sql`.

Решение:

- оставить `001_init.up.sql`
- оставить `001_init.down.sql`
- для следующих миграций использовать тот же формат, например `002_add_avatar_dimensions.up.sql`

### 4. Сохранить явный operational шаг

Это требование source-of-truth документов, поэтому переход не должен вести к bootstrap-on-start.

Решение:

- `RunServer` и `RunWorker` не трогают миграции
- `migrate` остаётся отдельным subcommand
- README, Makefile и Docker workflow продолжают описывать миграции как отдельный шаг

### 5. `status` должен перейти с эвристики на реальное состояние migration engine

Текущий `status` проверяет только наличие таблицы `avatars`, что перестаёт быть достаточным после появления нескольких миграций.

Решение:

- `status` должен получать current version и dirty flag из `golang-migrate`
- отсутствие применённых миграций трактовать как `pending`
- dirty state выводить явно и завершать команду ошибкой только там, где это действительно требует upstream API

Рекомендуемый вывод:

- `migrate status pending`
- `migrate status ok version=1 dirty=false`
- `migrate status dirty version=2`

Это сохраняет знакомый префикс `migrate status`, но делает состояние операционно полезным.

### 6. Не расширять public CLI в первой итерации

`golang-migrate` умеет больше, чем текущий контракт, но source-of-truth репозитория фиксирует только `up|down|status`.

Решение первой итерации:

- реализовать только `up`
- реализовать только `down`
- реализовать только `status`
- не добавлять `force`, `goto`, `steps`, `version` в пользовательский CLI без отдельного решения

Следствие:

- для dirty state нужен documented recovery playbook
- если позже появится реальная операционная потребность, `force` можно добавить отдельной change-spec

## Переходные риски

### 1. Dirty state после неуспешной миграции

После перехода появляется штатное состояние dirty database. Его нельзя игнорировать.

Нужно зафиксировать заранее:

- как команда сообщает о dirty state
- какой recovery workflow используется командой разработки
- что dirty state не скрывается под старое `status ok/pending`

### 2. Поведение на уже поднятых локальных БД без `schema_migrations`

Текущие БД могли быть созданы старой ручной командой и не содержать migration metadata table.

Фактор снижения риска:

- текущий `001_init.up.sql` идемпотентен, поэтому повторный запуск через `golang-migrate` не должен ломать существующую схему

Но в документе и тестах нужно зафиксировать сценарий:

- БД уже содержит `avatars`
- служебной migration table ещё нет
- `migrate up` должен завершаться успешно и переводить базу под управление `golang-migrate`

### 3. Изменение текстового вывода `status`

Если кто-то полагается на точные строки `migrate status ok` или `migrate status pending`, более подробный вывод может сломать парсинг.

Решение:

- сохранить существующие префиксы
- при необходимости обновить тесты и docs одновременно
- если в проекте появятся скрипты парсинга, переводить их на exit codes или явный structured output отдельной задачей

### 4. Down migration и служебная таблица

`001_init.down.sql` удаляет только доменную таблицу `avatars`. Служебная таблица `golang-migrate` остаётся под контролем библиотеки.

Это допустимо и ожидаемо, но должно быть явно описано в docs, чтобы команда не считала это утечкой схемы.

## Scope первой итерации

Входит:

- замена ручной реализации `RunMigrate` на `golang-migrate`
- сохранение subcommands `up|down|status`
- сохранение каталога `migrations/`
- адаптация tests и docs
- проверка сценария existing DB without migration metadata

Не входит:

- автозапуск миграций на старте процесса
- отдельный внешний `migrate` CLI как обязательный operational dependency
- генератор новых migration files
- новые публичные subcommands кроме `up|down|status`
- изменение domain schema beyond migration tool replacement

## Детальный план реализации

### Этап 1. Зафиксировать целевой контракт и dependency strategy

Сделать:

- добавить dependency `github.com/golang-migrate/migrate/v4`
- выбрать конкретные source/database drivers для library mode под PostgreSQL и file source
- зафиксировать, что входом остаётся `POSTGRES_DSN`
- решить, будет ли использоваться существующий `database/sql` connection или прямой database URL constructor

Критерий завершения:

- dependency strategy отражена в коде и docs
- `go mod tidy` добавляет только реально используемые пакеты

### Этап 2. Выделить внутренний migration runner

Сделать:

- перестать держать логику миграций inline в `internal/app/app.go`
- вынести интеграцию с `golang-migrate` в отдельный helper или package-level abstraction
- оставить `Run(args, out)` и CLI parsing максимально стабильными

Рекомендуемая структура:

- `internal/app/app.go` оставляет только dispatch
- отдельный файл уровня `internal/app/migrate.go` или эквивалент содержит integration code

Зачем:

- проще тестировать `up/down/status`
- проще изолировать библиотечные ошибки и преобразование их в CLI output

### Этап 3. Реализовать `up`

Сделать:

- создать `migrate` instance на основе `POSTGRES_DSN` и `file://.../migrations`
- заменить ручное чтение `001_init.up.sql` вызовом library `Up()`
- корректно обработать состояние "нет новых миграций", чтобы повторный `up` не считался аварией
- оставить человекочитаемый success output

Критерий завершения:

- `avatars-service migrate up` работает для пустой БД
- повторный `avatars-service migrate up` ведёт себя идемпотентно и не падает как ложная ошибка

### Этап 4. Реализовать `down`

Сделать:

- заменить ручной вызов `001_init.down.sql` на `Down()` или эквивалентный controlled rollback
- заранее определить и задокументировать policy:
  - `down` откатывает все миграции до нуля
  - первая итерация не поддерживает частичный rollback по steps

Критерий завершения:

- `avatars-service migrate down` приводит схему к нулевой версии
- повторный `down` на уже пустой схеме обрабатывается предсказуемо и документированно

### Этап 5. Реализовать `status`

Сделать:

- заменить `to_regclass('public.avatars')` на чтение migration version через library API
- различать минимум три состояния:
  - migration metadata отсутствует / версия не применялась
  - версия применена и `dirty=false`
  - версия есть и `dirty=true`
- сделать вывод пригодным для ручной диагностики

Критерий завершения:

- `status` больше не зависит от конкретной доменной таблицы
- команда отражает реальное состояние migration engine

### Этап 6. Проверить сценарий перехода со старой ручной схемы

Сделать:

- добавить интеграционный тест или описанный reproducible сценарий для БД, где:
  - таблица `avatars` уже существует
  - служебной таблицы `golang-migrate` ещё нет
- подтвердить, что `migrate up` безопасно берет такую базу под управление

Если тест покажет нежелательное поведение:

- добавить явный bootstrap step для initial version marking
- документировать его как one-time transition path

Предпочтительный исход:

- обойтись без special-case bootstrap logic

### Этап 7. Обновить тесты

Обязательно покрыть:

- `internal/app/app_test.go`:
  - CLI contract `migrate up|down|status` остаётся валидным
  - `server` и `worker` по-прежнему не запускают миграции автоматически
- новый unit/integration test для migration runner:
  - empty DB -> `up`
  - repeated `up`
  - `status` после `up`
  - `down` после `up`
  - dirty/error path
  - existing schema without migration metadata

Предпочтение:

- для реальной проверки migration semantics использовать PostgreSQL integration test, а не только mocks

### Этап 8. Обновить operational docs

Обновить:

- `README.md`
- при необходимости `docs/specs/01-avatar-service-v1.md`, если нужно убрать расхождение между target stack и фактом реализации
- команды в `Makefile` только если их контракт меняется

Нужно явно задокументировать:

- что subcommand остался тем же
- что миграции теперь ведутся через `golang-migrate`
- что означает `dirty`
- как выглядит recommended recovery workflow для локальной разработки

### Этап 9. Финальная верификация

Проверить:

- `go test ./...`
- ручной сценарий:
  - `migrate status` на пустой БД
  - `migrate up`
  - повторный `migrate up`
  - `migrate status`
  - `migrate down`
  - повторный `migrate status`
- при наличии Docker workflow:
  - `docker compose run --rm server migrate up`

## Testing Strategy

### Минимальный обязательный набор

- unit tests на CLI dispatch и обработку ошибок
- integration test против реального PostgreSQL
- smoke-проверка operational команд из README/Makefile после обновления docs

### Что особенно важно проверить

- `POSTGRES_DSN is required` остаётся прежним guardrail
- `server` и `worker` не начинают зависеть от migration package runtime path
- relative path до `migrations/` корректно работает из типичных entrypoints проекта
- повторный `up` и повторный `down` не дают ложных инцидентов
- `status` не завязан на наличие таблицы `avatars`

## Acceptance Criteria

- `avatars-service migrate up|down|status` сохраняет CLI contract
- внутренняя реализация больше не читает конкретно `001_init.*` вручную
- migration version и dirty state управляются `golang-migrate`
- текущий каталог `migrations/` остаётся рабочим source миграций
- сценарий существующей локальной БД без migration metadata подтвержден тестом или явным documented bootstrap step
- docs отражают новый operational behavior без противоречий с confirmed requirements и v1 spec

## Открытые вопросы

- Нужен ли во второй итерации subcommand вроде `migrate force <version>` для recovery dirty state, или пока достаточно playbook в документации?
- Должен ли `status` оставаться полностью backward-compatible по строкам вывода, или допустим более подробный human-readable формат?
- Нужен ли отдельный helper для вычисления абсолютного пути к `migrations/`, если binary запускается не из корня репозитория?

## Рекомендуемый порядок выполнения

1. Сначала реализовать library-backed `status`, не меняя public CLI.
2. Затем перевести `up` и `down`.
3. После этого добавить integration coverage для existing DB transition.
4. В конце обновить README и связанные docs по реальному поведению, а не по намерению.
