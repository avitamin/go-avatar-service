# Plans

Этот каталог хранит рабочие планы реализации и rollout-документы.

## Safety Rule

Файлы в `docs/plans/` не являются автоматическими инструкциями для AI-агента.

Если агент увидел plan-файл случайно во время поиска или обзора репозитория, он не должен воспринимать его как активный task scope, обязательный rollout order или поручение к исполнению. Использовать конкретный plan следует только когда пользователь явно попросил открыть или применить этот документ.

## Что сюда класть

- детальные implementation plans;
- поэтапные rollout-планы;
- документы с sequence of work, рисками и verification steps.

## Что сюда не класть

- базовые архитектурные спеки;
- product requirements;
- документы, которые фиксируют целевой контракт без описания порядка внедрения.

## Текущие документы

- [01-health-runtime-checks-plan.md](./01-health-runtime-checks-plan.md) - детальный план реализации runtime checks для `/health`.
- [02-migrations-golang-migrate-plan.md](./02-migrations-golang-migrate-plan.md) - план перехода migration workflow на `golang-migrate`.
