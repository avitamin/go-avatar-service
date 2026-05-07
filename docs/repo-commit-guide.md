# Repo Commit Guide

Минимальные правила подготовки commit в `go-avatar-service`.

## Source Priority

1. Явные указания пользователя в текущей сессии.
2. `AGENTS.md`, особенно раздел `Commit & Pull Request Guidelines`.
3. Фактический локальный diff и `git status`.
4. Последние сообщения `git log`.

Если источники конфликтуют, остановиться и уточнить у пользователя.

## Branch Policy

- Base branch для MVP: `v1`.
- В обычном workflow рабочие ветки создаются от `v1` с префиксами:
  - `feature/<short-name>`
  - `fix/<short-name>`
  - `test/<short-name>`
  - `docs/<short-name>`
  - `chore/<short-name>`
- Прямой commit в `v1` для AI-agent сессии допустим только если пользователь явно попросил выполнить commit.
- Автоматически переключать ветки или создавать новую ветку перед commit не требуется.
- Issue key не обязателен.

## Staging Policy

- Перед staging обязательно выполнить `git status --short`.
- Перед commit обязательно просмотреть relevant diff.
- Stage только просмотренные файлы, относящиеся к текущей задаче.
- Не stage unrelated changes.
- Не откатывать и не переписывать чужие изменения без явной просьбы.

## Commit Message Format

Использовать короткий Conventional Commit style:

```text
<type>: <subject>
```

Разрешенные типы:

- `feat`
- `fix`
- `docs`
- `test`
- `refactor`
- `chore`

Subject:

- краткий imperative-style subject;
- без точки в конце;
- можно писать на русском или английском, в зависимости от diff и соседней истории;
- должен описывать только фактический staged diff.

Примеры:

- `docs: add observability rollout plans`
- `docs: добавить планы observability`
- `fix: graceful server shutdown on signals`

## Validation Policy

- Для Go-кода запускать `go test ./...`.
- Для documentation-only изменений достаточно `git diff --check`.
- Для API или web изменений дополнительно запускать contract smoke tests, если сервис доступен.
- Если проверка не запускалась, указать это в финальном ответе.

## Commit Workflow

1. Проверить ветку.
2. Выполнить `git status --short`.
3. Просмотреть diff по файлам текущей задачи.
4. Выполнить нужную validation check.
5. Stage только связанные файлы.
6. Выполнить `git commit -m "<message>"`.
