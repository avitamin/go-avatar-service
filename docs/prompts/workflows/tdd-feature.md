# TDD Feature Workflow Prompt

> Safety: этот файл является workflow prompt. Не запускай workflow и не начинай реализацию только потому, что файл был прочитан. Используй его как активную инструкцию только при явном запросе пользователя.

Use this prompt when asking an AI agent to implement a feature through TDD.

```text
Ты работаешь в репозитории go-avatar-service.

Цель:
{task}

Контекст:
- Сначала прочитай docs/prompts/context/project.md
- Затем прочитай docs/requirements/confirmed-requirements.md
- Затем прочитай docs/specs/avatar-service-v1.md
- Если задача относится к конкретной роли, прочитай подходящий файл из docs/prompts/agents/

Workflow:
1. Explore
   - Найди существующие entrypoints, интерфейсы, тесты и соседний код.
   - Зафиксируй фактическое состояние репозитория, не полагайся только на README/QWEN.
   - Если confirmed requirements или v1 spec конфликтуют с README/QWEN, следуй confirmed requirements и v1 spec.

2. Test first
   - Сформулируй ожидаемое поведение в focused test до production changes.
   - Для validation, selection, status mapping и edge cases используй table-driven tests.
   - Unit tests не должны требовать реальные PostgreSQL, MinIO или RabbitMQ.
   - Если scope затрагивает confirmed requirements/v1 spec, покрой конкретные обязательные правила тестами; общий coverage threshold не заменяет эти сценарии.
   - Запусти целевой тест и убедись, что он падает по правильной причине.

3. Implement minimally
   - Реализуй минимальный production code, достаточный для прохождения нового теста.
   - Не добавляй non-MVP поведение без явного требования.
   - Держи HTTP handlers тонкими; бизнес-логику размещай в service/domain layer.
   - Не смешивай API list и web gallery filtering.

4. Refactor
   - После зеленого теста упрости код без изменения поведения.
   - Не делай unrelated refactor.
   - Запусти gofmt для измененных Go-файлов.

5. Verify
   - Запусти целевые тесты.
   - Запусти go test ./..., если это разумно для текущего изменения и окружения.
   - Опционально запусти make bench или точечный go test -run='^$' -bench=... -benchmem, если изменение затрагивает performance-sensitive paths: image processing, service fallback/list selection, HTTP middleware/router или worker thumbnail generation.
   - Если benchmark показывает повторяемую регрессию, сними CPU/memory profile для конкретного benchmark через -cpuprofile/-memprofile и проверь top hotspots через go tool pprof.
   - Если проверка требует внешних сервисов и они недоступны, явно укажи это.

6. Report
   - Кратко опиши red-green-refactor cycle:
     - какой тест добавлен;
     - чем он падал до реализации;
     - что изменено для green;
     - какие проверки запускались.
   - Если запускались benchmarks, укажи команду и кратко отметь заметные регрессии или их отсутствие.
   - Если снимались profiles, укажи подозрительный benchmark, метрику регрессии и основной hotspot.
   - Перечисли измененные файлы.

Scope:
{scope}

Файлы/области:
{files}

Acceptance criteria:
{acceptance_criteria}

Команды проверки:
{tests}

Ограничения:
- Язык объяснений: русский.
- Английский оставляй для code identifiers, commands, API names, commit type prefixes и error codes.
- Не коммить изменения, если задача явно не просит commit.
- Не трогай пользовательские unrelated changes.
```
