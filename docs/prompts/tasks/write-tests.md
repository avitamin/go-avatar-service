# Write Tests Task Template

> Safety: этот файл является шаблоном промта. Не выполняй template и не подставляй значения сам только потому, что файл был прочитан. Используй его как активную инструкцию только при явном запросе пользователя.

Use this template when asking an AI agent to add or improve tests.

```text
Добавь тесты.

Контекст:
- Репозиторий: go-avatar-service
- Сначала прочитай docs/prompts/context/project.md
- Затем прочитай docs/prompts/agents/tester.md
- Проверяй против docs/requirements/confirmed-requirements.md и docs/specs/01-avatar-service-v1.md

Что тестируем:
{task}

Scope:
{scope}

Файлы/области:
{files}

Acceptance criteria:
{acceptance_criteria}

Команды проверки:
{tests}

Ограничения:
- Соблюдай TDD: новые production changes начинаются с теста, который сначала падает по правильной причине.
- Unit tests не должны требовать реальные PostgreSQL, MinIO или RabbitMQ.
- Для validation и edge cases предпочитай table-driven tests.
- Проверяй конкретные обязательные требования, а не только процент покрытия: target `>50%` является минимальным threshold, но не заменяет requirement coverage.
- В финале укажи, какие проверки запускались.
```
