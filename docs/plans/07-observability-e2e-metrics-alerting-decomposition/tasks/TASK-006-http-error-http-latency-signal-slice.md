# TASK-006: HTTP Error And HTTP Latency Signal Slice

Phase: Parallel
Owner: Backend
Owned surface: HTTP-layer hooks plus matching runner scenarios
Parallel-safe: Yes
Can start after: TASK-002

Goal: Support `HighHTTPErrorRate` and `HighResponseTimeP95` runtime signal checks.

Scope:
- Gated 5xx hook.
- Gated HTTP latency hook.
- Unit tests for disabled/enabled behavior.
- Runner scenarios querying `ALERTS` or runtime series as defined by the plan.

Out of scope:
- Upload latency.
- Dependency metrics.
- Worker metrics.

Likely touched areas:
- `internal/http`
- `cmd/observability-e2e/**`
- `configs/observability/prometheus/alerts.yml` for reference only.

High-conflict surfaces:
- Router if hook contract changes.

Dependencies:
- TASK-002.

Contract constraints:
- No hook availability without `OBSERVABILITY_E2E_HOOKS_ENABLED=true`.
- Production routes unchanged.

Acceptance criteria:
- 5xx increments `http_requests_total{status=~"5.."}`.
- Latency produces `http_request_duration_seconds_bucket`.

Verification:
- `go test ./internal/http ./cmd/observability-e2e`
- `go test ./...`

Ready for integration when:
- Scenarios can trigger and observe both HTTP signals.

Integration notes:
- Keep hook paths aligned with TASK-002 output.

## Worker Prompt

```text
Implement TASK-006: HTTP Error And HTTP Latency Signal Slice

Goal:
Support runtime signal checks for HighHTTPErrorRate and HighResponseTimeP95.

Owned surface:
HTTP-layer gated hooks and matching runner scenarios.

Scope:
Add controlled 5xx hook, controlled HTTP latency hook, tests for disabled/enabled behavior, and runner scenarios that observe Prometheus runtime series or ALERTS as defined by the plan.

Out of scope:
Upload latency, dependency metrics, worker/RabbitMQ metrics.

Inspect these likely areas:
internal/http/router.go
internal/observability/middleware.go
cmd/observability-e2e
configs/observability/prometheus/alerts.yml

Avoid changing:
Production route behavior, alert thresholds, public API docs.

Respect these contracts:
Hooks unavailable unless OBSERVABILITY_E2E_HOOKS_ENABLED=true.

Acceptance criteria:
5xx increments http_requests_total with 5xx status. Latency creates http_request_duration_seconds_bucket observations.

Verification:
go test ./internal/http ./cmd/observability-e2e
go test ./...

Ready for integration when:
Runner can generate and observe both HTTP signals.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
