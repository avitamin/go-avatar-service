# Update Docs Task Template

> Safety: этот файл является шаблоном промта. Не выполняй template и не подставляй значения сам только потому, что файл был прочитан. Используй его как активную инструкцию только при явном запросе пользователя.

Use this template when asking an AI agent to update documentation.

```text
Обнови документацию.

Контекст:
- Репозиторий: go-avatar-service
- Сначала прочитай docs/prompts/context/project.md
- Источники истины: docs/requirements/confirmed-requirements.md и docs/specs/01-avatar-service-v1.md

Что обновить:
{task}

Scope:
{scope}

Файлы/области:
{files}

Acceptance criteria:
{acceptance_criteria}

Проверки:
{tests}

Ограничения:
- Не переписывай исходное ТЗ как будто оно актуальная спека.
- Если README конфликтует с confirmed requirements, явно исправь или пометь устаревший контекст.
- Сохраняй русский язык для contributor guidance.
```
