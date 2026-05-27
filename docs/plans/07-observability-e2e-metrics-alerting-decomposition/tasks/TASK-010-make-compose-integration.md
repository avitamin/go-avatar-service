# TASK-010: Make And Compose Integration

Phase: Integration
Owner: Infra
Owned surface: Make targets and compose env wiring
Parallel-safe: No
Can start after: TASK-001, TASK-003, TASK-004, TASK-005, TASK-006, TASK-007, TASK-008, TASK-009

Goal: Expose the complete workflow as opt-in commands.

Scope:
- `build-observability-e2e`.
- `observability-e2e`.
- `docker-observability-e2e`.
- Optional promtool target/container fallback.
- Compose env flag for e2e hooks only where needed.

Out of scope:
- Making e2e mandatory PR gate.

Likely touched areas:
- `Makefile`
- `docker-compose.observability.yml`
- Possibly `.env.example`

High-conflict surfaces:
- `Makefile`
- Compose env.

Dependencies:
- All implementation slices.

Contract constraints:
- Existing `make contract-tests` and `make docker-contract-tests` unchanged.

Acceptance criteria:
- Documented commands match plan defaults.

Verification:
- `go test ./...`
- `promtool test rules configs/observability/prometheus/alert-rule-tests.yml`
- `make docker-observability-e2e`

Ready for integration when:
- One command runs the full opt-in e2e suite.

Integration notes:
- Keep hooks opt-in and do not enable them for ordinary compose/contract-test workflows.

## Worker Prompt

```text
Implement TASK-010: Make And Compose Integration

Goal:
Expose the observability e2e workflow as opt-in commands.

Owned surface:
Makefile and compose/env wiring.

Scope:
Add build-observability-e2e, observability-e2e, docker-observability-e2e, and promtool helper if needed. Wire OBSERVABILITY_E2E_HOOKS_ENABLED only for e2e observability runs where appropriate.

Out of scope:
Making observability e2e mandatory in the default test gate.

Inspect these likely areas:
Makefile
docker-compose.observability.yml
.env.example if present
docs/plans/07-observability-e2e-metrics-alerting-plan.md

Avoid changing:
Existing contract-tests and docker-contract-tests behavior.

Respect these contracts:
Default compose URLs: service 8080, Prometheus 9090, Alertmanager 9093, RabbitMQ Management 15672.

Acceptance criteria:
Opt-in commands build and run the runner with documented defaults.

Verification:
go test ./...
promtool test rules configs/observability/prometheus/alert-rule-tests.yml
make docker-observability-e2e

Ready for integration when:
A single make target runs the full observability e2e suite against the compose stack.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
