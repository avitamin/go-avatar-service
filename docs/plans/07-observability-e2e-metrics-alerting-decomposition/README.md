# Parallel Decomposition: observability e2e metrics and alerting

## Outcome

План `../07-observability-e2e-metrics-alerting-plan.md` разбит на foundation-контракты, независимые slices для e2e-автотестов и отдельную integration/hardening фазу. Главный принцип: сначала стабилизировать CLI runner, test-only hook contract и scenario registration, затем параллельно добавлять Prometheus/Alertmanager проверки, alert fixtures и отдельные signal scenarios без изменения production alert thresholds.

## Assumptions

- Работа ведется от `docs/plans/07-observability-e2e-metrics-alerting-plan.md`.
- Runtime stack уже существует через `docker-compose.yml` и `docker-compose.observability.yml`.
- Runner должен быть black-box: новый `cmd/observability-e2e` не импортирует `internal/`.
- Test-only hooks допустимы только за `OBSERVABILITY_E2E_HOOKS_ENABLED=true`.
- RabbitMQ topology актуальна: exchange `avatars`, queues `avatars.uploads` / `avatars.deletes`, routing keys `avatar.uploaded` / `avatar.delete_requested`.

## Shared Contracts

- API: `cmd/observability-e2e` CLI flags, scenario result model, Prometheus `/api/v1/*`, Alertmanager `/api/v2/*`, RabbitMQ Management API.
- Data: None.
- Events/jobs: RabbitMQ exchange `avatars`, queues `avatars.uploads`, `avatars.deletes`, routing keys `avatar.uploaded`, `avatar.delete_requested`.
- Config/flags: `OBSERVABILITY_E2E_HOOKS_ENABLED`, `BASE_URL`, `PROMETHEUS_URL`, `ALERTMANAGER_URL`, `RABBITMQ_MANAGEMENT_URL`, timeout/verbose flags.
- Permissions: hooks unavailable without explicit env flag; no secrets/env/storage paths in responses.
- Public module interfaces: runner-local client interfaces only; no public Go package export required.
- UX behavior: CLI exits `0` success, `1` scenario failure, `2` config error; verbose prints diagnostics.

## Dependency Model

- TASK-002 blocked by TASK-001
- TASK-004 blocked by TASK-001
- TASK-005 blocked by TASK-001
- TASK-006 blocked by TASK-002
- TASK-007 blocked by TASK-002
- TASK-008 blocked by TASK-002
- TASK-009 blocked by TASK-001, TASK-002
- TASK-010 blocked by TASK-001, TASK-003, TASK-004, TASK-005, TASK-006, TASK-007, TASK-008, TASK-009
- TASK-011 blocked by TASK-010

## Work Items

- [TASK-001: Observability E2E Runner Foundation](./tasks/TASK-001-observability-e2e-runner-foundation.md)
- [TASK-002: Test-Only Hook Contract Foundation](./tasks/TASK-002-test-only-hook-contract-foundation.md)
- [TASK-003: Promtool Alert Rule Fixtures](./tasks/TASK-003-promtool-alert-rule-fixtures.md)
- [TASK-004: Prometheus And Alertmanager Baseline Scenarios](./tasks/TASK-004-prometheus-alertmanager-baseline-scenarios.md)
- [TASK-005: Public API Metrics Scenarios](./tasks/TASK-005-public-api-metrics-scenarios.md)
- [TASK-006: HTTP Error And HTTP Latency Signal Slice](./tasks/TASK-006-http-error-http-latency-signal-slice.md)
- [TASK-007: Upload Latency Signal Slice](./tasks/TASK-007-upload-latency-signal-slice.md)
- [TASK-008: Dependency Error Signal Slice](./tasks/TASK-008-dependency-error-signal-slice.md)
- [TASK-009: RabbitMQ Backlog And Worker Failure Slice](./tasks/TASK-009-rabbitmq-backlog-worker-failure-slice.md)
- [TASK-010: Make And Compose Integration](./tasks/TASK-010-make-compose-integration.md)
- [TASK-011: Final Hardening And Documentation Update](./tasks/TASK-011-final-hardening-documentation-update.md)

## Parallel Waves

Wave 0 - Foundation:

- TASK-001
- TASK-002

Wave 1 - Parallel Implementation:

- TASK-003
- TASK-004
- TASK-005
- TASK-006
- TASK-007
- TASK-008
- TASK-009

Wave 2 - Integration:

- TASK-010

Wave 3 - Hardening/Release:

- TASK-011

## Integration Risks

- Risk: Runner scenario files still collide on a central registry.
  Impact: merge conflicts across QA/Test tasks.
  Mitigation: TASK-001 must define low-contention scenario registration before parallel work.
  Owner: QA/Test

- Risk: Hooks accidentally become reachable in normal runtime.
  Impact: production-only behavior and security exposure.
  Mitigation: env gate, negative tests, no public docs, isolated namespace.
  Owner: Backend

- Risk: Production alert `for` windows make e2e slow/flaky.
  Impact: long or unreliable CI runs.
  Mitigation: promtool validates expressions; e2e validates runtime series/wiring and only waits where explicitly acceptable.
  Owner: QA/Test

- Risk: RabbitMQ backlog/failure scenarios leave dirty state.
  Impact: later scenarios fail or worker loops on poison messages.
  Mitigation: cleanup/resume in `defer`, queue purge where appropriate, final state check.
  Owner: Backend

## Source Notes

This decomposition is a planning artifact for e2e autotest implementation. It is not an automatic task scope unless a user explicitly asks to use it.
