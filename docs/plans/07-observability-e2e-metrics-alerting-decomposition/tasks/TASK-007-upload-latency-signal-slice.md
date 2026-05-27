# TASK-007: Upload Latency Signal Slice

Phase: Parallel
Owner: Backend
Owned surface: upload-specific test-only delay and runner scenario
Parallel-safe: Yes
Can start after: TASK-002

Goal: Support `UploadLatencyCritical` runtime metric-path check.

Scope:
- Gated upload delay mechanism.
- Tests proving normal upload behavior is unchanged.
- Scenario observing `avatars_upload_duration_seconds_bucket`.

Out of scope:
- Upload error public scenario from TASK-005.

Likely touched areas:
- `internal/http`
- `internal/service`
- `cmd/observability-e2e/**`

High-conflict surfaces:
- Upload handler behavior.

Dependencies:
- TASK-002.

Contract constraints:
- Hook/delay must not alter confirmed public upload contract.

Acceptance criteria:
- Controlled observations hit upload duration histogram.
- Disabled runtime has no delay path.

Verification:
- `go test ./internal/http ./internal/service ./cmd/observability-e2e`
- `go test ./...`

Ready for integration when:
- Runtime upload latency metric can be generated deterministically.

Integration notes:
- Coordinate with TASK-005 if shared upload helpers are needed.

## Worker Prompt

```text
Implement TASK-007: Upload Latency Signal Slice

Goal:
Support runtime metric-path check for UploadLatencyCritical.

Owned surface:
Upload-specific gated delay behavior and runner scenario.

Scope:
Add a test-only way to delay upload processing, preserve normal upload behavior, and observe avatars_upload_duration_seconds_bucket from the runner.

Out of scope:
Invalid upload error-rate scenario; that belongs to public API metrics work.

Inspect these likely areas:
internal/http/router.go
internal/service/service.go
cmd/observability-e2e
configs/observability/prometheus/alerts.yml

Avoid changing:
Confirmed upload API behavior and validation rules.

Respect these contracts:
Delay must be unavailable without OBSERVABILITY_E2E_HOOKS_ENABLED=true.

Acceptance criteria:
Controlled upload duration observations are visible in Prometheus.

Verification:
go test ./internal/http ./internal/service ./cmd/observability-e2e
go test ./...

Ready for integration when:
Upload latency scenario is deterministic and does not affect normal uploads.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
