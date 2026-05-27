# TASK-001: Observability E2E Runner Foundation

Phase: Foundation
Owner: QA/Test
Owned surface: `cmd/observability-e2e` runner skeleton and runner-local test helpers
Parallel-safe: No
Can start after: None

Goal: Create the black-box runner contract used by all later scenarios.

Scope:
- CLI flags/env defaults.
- URL validation.
- Timeout context.
- Verbose diagnostics.
- Scenario interface/registry.
- Exit codes.
- Minimal unit tests.

Out of scope:
- Concrete Prometheus scenarios.
- Hooks.
- Makefile targets.

Likely touched areas:
- `cmd/observability-e2e/**`
- `cmd/avatar-contract-tests/**` as a style reference.

High-conflict surfaces:
- Scenario registry.
- CLI config.

Dependencies: None.

Contract constraints:
- Do not import `internal/`.
- Config errors exit `2`.
- Scenario failures exit `1`.

Acceptance criteria:
- Invalid/missing config exits `2`.
- Scenarios can be added in separate files with low contention.
- Unit tests cover URL normalization and exit-code mapping.

Verification:
- `go test ./cmd/observability-e2e`
- `go test ./...`

Ready for integration when:
- Runner builds and a no-op scenario can run.

Integration notes:
- Downstream tasks must not change CLI flag names without coordinating here.

## Worker Prompt

```text
Implement TASK-001: Observability E2E Runner Foundation

Goal:
Create the black-box runner skeleton for cmd/observability-e2e.

Owned surface:
cmd/observability-e2e only.

Scope:
Add CLI/env config, URL validation, timeout context, verbose mode, scenario interface/registry, exit codes 0/1/2, and unit tests.

Out of scope:
Concrete Prometheus/Alertmanager/RabbitMQ scenarios, hooks, Makefile targets.

Inspect these likely areas:
cmd/avatar-contract-tests for black-box runner style.
docs/plans/07-observability-e2e-metrics-alerting-plan.md.

Avoid changing:
internal packages, Makefile, docker-compose files.

Respect these contracts:
No internal imports. Config errors exit 2. Scenario failures exit 1.

Acceptance criteria:
Runner builds, invalid config is tested, and downstream scenarios can be added with low file contention.

Verification:
go test ./cmd/observability-e2e
go test ./...

Ready for integration when:
A no-op scenario can run and report success/failure consistently.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
