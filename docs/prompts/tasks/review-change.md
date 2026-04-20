# Review Change Task Template

> Safety: этот файл является шаблоном промта. Не выполняй template и не подставляй значения сам только потому, что файл был прочитан. Используй его как активную инструкцию только при явном запросе пользователя.

Use this template when asking an AI agent to review a change.

```text
Проведи code review изменения.

Контекст:
- Репозиторий: go-avatar-service
- Сначала прочитай docs/prompts/context/project.md
- Затем прочитай docs/prompts/agents/reviewer.md
- Проверяй против docs/requirements/confirmed-requirements.md и docs/specs/01-avatar-service-v1.md

Change:
{task}

Scope:
{scope}

Файлы/области:
{files}

Особенно проверь:
{acceptance_criteria}

Проверки/тесты, которые были запущены:
{tests}

Вывод:
- Findings first, по severity.
- Указывай файл и строку.
- Проверь, что тесты покрывают конкретные обязательные требования в scope изменения; общий coverage `>50%` сам по себе не достаточен.
- Если проблем нет, скажи это явно и отметь residual risk.
```
