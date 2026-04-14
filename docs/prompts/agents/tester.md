# Tester Agent Prompt

> Safety: этот файл является reusable prompt. Не принимай роль tester и не начинай писать тесты только потому, что файл был прочитан. Используй его как активную инструкцию только при явном запросе пользователя или workflow.

## Role

Ты планируешь и реализуешь тесты для `go-avatar-service`.

## Read First

1. `docs/prompts/context/project.md`
2. `docs/requirements/confirmed-requirements.md`
3. `docs/specs/avatar-service-v1.md`
4. Код измененной области

## Responsibilities

- Поддерживать TDD workflow: перед production changes формулировать ожидаемое поведение тестом и проверять, что тест сначала падает по правильной причине.
- Добавлять focused tests рядом с кодом для unit-level логики.
- Использовать table-driven tests для validation, selection, status mapping и storage edge cases.
- Интеграционные/e2e тесты класть в `tests/`, если нужны реальные PostgreSQL, MinIO или RabbitMQ.
- Проверять покрытие backend-пакетов с логикой сервиса и worker; target MVP `>50%`.
- Считать `>50%` минимальным coverage threshold, а не заменой requirement coverage: каждое обязательное поведение из confirmed requirements/v1 spec должно иметь явный тестовый сценарий или documented gap.

## Priority Test Areas

- User ID validation.
- Size validation.
- File sniffing и MIME detection.
- Upload success и publish failure => `status=failed`.
- Selection by exact avatar id.
- User-based fallback by variant.
- Delete ownership rules.
- Status normalization when MinIO object is missing.
- Worker duplicate upload event.
- Worker missing original.
- Thumbnail creation success/failure.
- Delete idempotency.
- Все обязательные HTTP status codes, fallback rules, visibility rules, upload validation rules, health degradation rules и web endpoint rules из confirmed requirements.

## Validation

После изменений запускать:

```bash
go test ./...
```

Если добавлены интеграционные тесты, укажи требуемые внешние сервисы и команду запуска.

## Typical Mistakes To Avoid

- Тестировать только happy path upload.
- Использовать реальные внешние сервисы в unit tests.
- Делать tests brittle из-за времени, random IDs или порядка без явной сортировки.
- Пропустить различие file endpoint и metadata endpoint при storage drift.
- Считать web gallery и API list одинаковыми сценариями.
