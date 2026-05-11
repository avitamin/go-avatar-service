# TASK-008: Dependency Error Signal Slice

Phase: Parallel
Owner: Backend
Owned surface: dependency-operation test signal and runner scenario
Parallel-safe: Yes
Can start after: TASK-002

Goal: Support `DependencyOperationErrors` runtime metric-path check.

Scope:
- Gated dependency error hook or deterministic dependency-failure action.
- Scenario observing `avatar_dependency_operations_total{status="error"}`.

Out of scope:
- Real outage orchestration that destabilizes stack.

Likely touched areas:
- `internal/http` or `internal/app`
- `internal/observability`
- `cmd/observability-e2e/**`

High-conflict surfaces:
- Metrics names/labels.

Dependencies:
- TASK-002.

Contract constraints:
- Component label must stay within expected dependency components such as `postgres`, `minio`, `rabbitmq`.

Acceptance criteria:
- Error metric appears with expected component.
- No secrets in hook response.

Verification:
- `go test ./internal/...`
- `go test ./cmd/observability-e2e`

Ready for integration when:
- Dependency error signal is observable without breaking subsequent scenarios.

Integration notes:
- Prefer a synthetic metric path over breaking live dependencies.

## Worker Prompt

```text
Implement TASK-008: Dependency Error Signal Slice

Goal:
Support runtime metric-path check for DependencyOperationErrors.

Owned surface:
Gated dependency error signal and matching runner scenario.

Scope:
Add deterministic test-only dependency error signal or safe dependency-failure action; observe avatar_dependency_operations_total{status="error"} in Prometheus.

Out of scope:
Breaking real Postgres/MinIO/RabbitMQ availability for the stack.

Inspect these likely areas:
internal/app/app.go
internal/http/router.go
internal/observability/metrics.go
cmd/observability-e2e
configs/observability/prometheus/alerts.yml

Avoid changing:
Metric names/label names unless required and coordinated.

Respect these contracts:
Component label must be an expected dependency component. Hook response must not reveal secrets.

Acceptance criteria:
Error metric appears with expected component label and can be polled by runner.

Verification:
go test ./internal/...
go test ./cmd/observability-e2e
go test ./...

Ready for integration when:
Dependency error signal does not destabilize later scenarios.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
