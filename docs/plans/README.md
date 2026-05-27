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

- [01-health-runtime-checks-plan.md](./01-health-runtime-checks-plan.md) - **реализовано**: runtime checks для `/health`.
- [02-migrations-golang-migrate-plan.md](./02-migrations-golang-migrate-plan.md) - **реализовано**: migration workflow на `golang-migrate`.
- [03-observability-application-instrumentation-plan.md](./03-observability-application-instrumentation-plan.md) - **реализовано**: OpenTelemetry tracing, Prometheus metrics и `slog` correlation в коде server/worker.
- [04-observability-monitoring-stack-plan.md](./04-observability-monitoring-stack-plan.md) - **реализовано**: локальный Prometheus, `prom/node-exporter`, Jaeger, Grafana, Loki и OpenTelemetry Collector stack через compose override.
- [05-observability-grafana-dashboards-plan.md](./05-observability-grafana-dashboards-plan.md) - **реализовано**: provisioned Grafana dashboards для RED metrics, infrastructure и business KPIs.
- [06-observability-alerting-plan.md](./06-observability-alerting-plan.md) - **реализовано**: Prometheus Alertmanager rules для error rate, latency, dependency и queue alerts.
- [07-observability-e2e-metrics-alerting-plan.md](./07-observability-e2e-metrics-alerting-plan.md) - **план к реализации**: TDD e2e-автотестов для application metrics, Prometheus alert rules и Alertmanager wiring.
- [07-observability-e2e-metrics-alerting-decomposition/](./07-observability-e2e-metrics-alerting-decomposition/) - **декомпозиция**: parallel work items и worker prompts для реализации e2e-автотестов observability.

## Рекомендуемый порядок observability-этапов

1. Сначала [03-observability-application-instrumentation-plan.md](./03-observability-application-instrumentation-plan.md), потому что stack и dashboards зависят от имен metrics, labels и trace propagation.
2. Затем [04-observability-monitoring-stack-plan.md](./04-observability-monitoring-stack-plan.md), чтобы поднять Prometheus, Jaeger, Grafana и Loki.
3. После стабилизации metrics выполнить [05-observability-grafana-dashboards-plan.md](./05-observability-grafana-dashboards-plan.md).
4. В конце добавить [06-observability-alerting-plan.md](./06-observability-alerting-plan.md), когда PromQL выражения проверены на реальных series.
5. После alerting добавить [07-observability-e2e-metrics-alerting-plan.md](./07-observability-e2e-metrics-alerting-plan.md), чтобы закрепить e2e-проверки scrape, rules, runtime metrics и Alertmanager wiring.
