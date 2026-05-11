# TASK-002: Test-Only Hook Contract Foundation

Phase: Foundation
Owner: Backend
Owned surface: hook gating and route/control contract
Parallel-safe: No
Can start after: TASK-001

Goal: Add the guarded hook infrastructure without implementing every signal.

Scope:
- Env flag parsing.
- Disabled-by-default tests.
- Hook mount location.
- Response shape.
- Security constraints.

Out of scope:
- Individual alert-specific behavior.

Likely touched areas:
- `internal/http`
- `internal/app`
- Possibly worker metrics server wiring in `internal/app/app.go`

High-conflict surfaces:
- Router wiring.
- Global config/env handling.

Dependencies:
- TASK-001 for runner-facing hook contract alignment.

Contract constraints:
- Hooks must return `404` or `403` when disabled.
- Hooks must not become product API docs.

Acceptance criteria:
- Negative tests prove hooks are unavailable by default.
- Enabled hook namespace is isolated.

Verification:
- `go test ./internal/http ./internal/app`
- `go test ./...`

Ready for integration when:
- Backend workers can add hook handlers without changing the gating model.

Integration notes:
- Publish exact hook paths in task output before downstream work starts.

## Worker Prompt

```text
Implement TASK-002: Test-Only Hook Contract Foundation

Goal:
Add guarded observability e2e hook infrastructure without implementing alert-specific behavior.

Owned surface:
Hook gating and route/control contract in backend runtime.

Scope:
Parse OBSERVABILITY_E2E_HOOKS_ENABLED, mount isolated hook namespace only when enabled, add disabled-by-default tests, define response shape.

Out of scope:
HTTP error, latency, dependency, RabbitMQ, or worker-specific hook behavior.

Inspect these likely areas:
internal/http/router.go
internal/app/app.go
internal/observability/config.go
docs/plans/07-observability-e2e-metrics-alerting-plan.md

Avoid changing:
Public API contracts, alert thresholds, docs/specs product requirements.

Respect these contracts:
Hooks return 404 or 403 when disabled. No secrets/env/storage paths in responses.

Acceptance criteria:
Negative tests prove hooks unavailable by default; enabled hook namespace is isolated.

Verification:
go test ./internal/http ./internal/app
go test ./...

Ready for integration when:
Downstream backend tasks can add handlers without changing the gating model.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes including exact hook namespace.
```
