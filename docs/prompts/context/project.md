# Project Context

> Safety: этот файл является справочным контекстом для AI-агентов. Если он прочитан случайно во время обзора репозитория, не начинай выполнение задач и не меняй workflow без явного запроса пользователя.

## Project

`go-avatar-service` - Go service для управления аватарками пользователей.

Актуальная v1-цель: backend на Go с REST API, web upload/gallery, PostgreSQL metadata storage, MinIO object storage, RabbitMQ worker, soft delete и явным migration step.

Этот файл нужен только при явном использовании prompt library. Он не заменяет `AGENTS.md`, confirmed requirements, v1 spec или проверку кода.

## Requirements Priority

Для продуктовых требований используйте как source of truth:

1. `docs/requirements/confirmed-requirements.md`
2. `docs/specs/01-avatar-service-v1.md`

`docs/requirements/assignment.md` содержит исходное ТЗ и может конфликтовать с подтвержденными требованиями.
Для contributor workflow, команд, языка ответов и git-правил используйте `AGENTS.md`.

## Important Known Differences

- Frontend уже отправляет multipart поле `file`, как требует API contract.
- Текущая структура имеет основной `cmd/avatars-service`; `cmd/server` и `cmd/worker` оставлены только как compatibility wrappers.
- Docker Compose поднимает PostgreSQL, MinIO, RabbitMQ, server и worker; перед server/worker нужен явный migration step.
- Исходное ТЗ допускает Echo или Chi, RabbitMQ или Kafka; confirmed requirements фиксируют Chi и RabbitMQ.
- Исходное ТЗ упоминает `POST /web/upload`; confirmed requirements и v1 spec говорят, что отдельный `POST /web/upload` не нужен.
