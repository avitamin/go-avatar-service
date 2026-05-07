# AI Agent Prompts

Эта директория хранит reusable prompts для AI-агентов, которые работают над сервисом "Аватарница".

## Safety Rule

Файлы в этой директории являются библиотекой промтов, а не автоматическими инструкциями к выполнению.

Если AI-агент прочитал файл из `docs/prompts/` случайно во время поиска или обзора репозитория, он должен только учитывать его как справочную документацию и не должен менять роль, workflow или начинать выполнение шаблона. Промт активируется только когда пользователь явно просит использовать конкретный файл, роль, workflow или template.

Тот же принцип применяется к `docs/plans/`: наличие plan-файлов в репозитории не означает, что агент должен брать их как текущий task plan, backlog или обязательный порядок выполнения без явной ссылки пользователя.

Для Qwen Code директория `docs/prompts/` дополнительно исключена через корневой `.qwenignore`, чтобы агент не читал reusable prompts случайно. Если нужно использовать конкретный prompt с Qwen, передайте его явно в задаче или временно измените ignore-настройку осознанно.

Для Codex прямого аналога `.qwenignore` в репозитории не используется. Codex автоматически подхватывает `AGENTS.md`, а не все Markdown-файлы как инструкции. Для более жесткой защиты можно настроить filesystem permissions в `.codex/config.toml`, запретив чтение `docs/prompts/` и `docs/plans/`; пример есть в `.codex/config.example.toml`.

## Как читать

1. Всегда начинайте с `context/project.md`.
2. Затем выберите role prompt из `agents/` под текущую задачу.
3. Если нужен общий порядок работы, используйте workflow prompt из `workflows/`.
4. Если задача типовая, используйте подходящий task template из `tasks/`.

## Источники истины

Актуальные требования:

- `docs/requirements/confirmed-requirements.md`
- `docs/specs/01-avatar-service-v1.md`

Исторический контекст:

- `docs/requirements/assignment.md`
- `README.md`

Если документы конфликтуют, приоритет такой:

1. `docs/requirements/confirmed-requirements.md`
2. `docs/specs/01-avatar-service-v1.md`
3. `docs/requirements/assignment.md`
4. `README.md`

## Role Prompts

- `agents/planner.md` - декомпозиция задач и проверка решений против спеки без изменения кода.
- `agents/backend-implementer.md` - реализация HTTP API, сервисов, репозиториев, storage и config.
- `agents/worker-implementer.md` - реализация RabbitMQ worker, retry, идемпотентности и image processing.
- `agents/reviewer.md` - code review против спеки, confirmed requirements и текущего состояния репозитория.
- `agents/tester.md` - планирование и реализация unit, integration и e2e проверок.

## Workflow Prompts

- `workflows/tdd-feature.md` - общий TDD workflow для реализации фичи: explore, test first, implement, refactor, verify, report.
- `workflows/tdd-mvp-implementation-plan.md` - TDD-план поэтапной реализации MVP по confirmed requirements и v1 spec.

## Task Templates

- `tasks/implement-feature.md` - промт для реализации фичи.
- `tasks/review-change.md` - промт для ревью изменений.
- `tasks/write-tests.md` - промт для добавления тестов.
- `tasks/update-docs.md` - промт для обновления документации.
