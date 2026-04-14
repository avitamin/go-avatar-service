# Implement Feature Task Template

> Safety: этот файл является шаблоном промта. Не выполняй template и не подставляй значения сам только потому, что файл был прочитан. Используй его как активную инструкцию только при явном запросе пользователя.

Use this template when asking an AI agent to implement a feature.

```text
Задача: {task}

Контекст:
- Репозиторий: go-avatar-service
- Сначала прочитай docs/prompts/context/project.md
- Затем прочитай docs/requirements/confirmed-requirements.md и docs/specs/avatar-service-v1.md

Scope:
{scope}

Файлы/области:
{files}

Acceptance criteria:
{acceptance_criteria}

Тесты:
{tests}

Ограничения:
- Работай через TDD: сначала failing test, затем минимальная реализация, затем refactor при зеленых тестах.
- Следуй confirmed requirements поверх README/QWEN.
- Не добавляй non-MVP возможности без необходимости.
- Не ограничивайся общей метрикой покрытия: target `>50%` не заменяет тесты конкретных обязательных требований в scope задачи.
- Сохраняй Go conventions и запускай gofmt для измененных Go-файлов.
- В финале укажи red-green-refactor summary и какие проверки запускались.
```
