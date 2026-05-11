# TASK-005: Public API Metrics Scenarios

Phase: Parallel
Owner: QA/Test
Owned surface: runner scenarios using only public avatar-service API
Parallel-safe: Yes
Can start after: TASK-001

Goal: Prove real `/health` and upload traffic creates expected Prometheus series.

Scope:
- `/health` traffic.
- Valid upload.
- Invalid upload.
- Multipart helper.
- Polling for `http_requests_total` and `avatars_uploads_total`.

Out of scope:
- Test-only hooks.
- Alert firing.

Likely touched areas:
- `cmd/observability-e2e/**`
- `cmd/avatar-contract-tests/**` for black-box helper style.

High-conflict surfaces:
- Shared runner HTTP helper if TASK-001 did not isolate it.

Dependencies:
- TASK-001.

Contract constraints:
- No `internal/` imports.
- Route labels must use templates and not raw IDs.

Acceptance criteria:
- Success/error upload metrics observed.
- High-cardinality route/user/avatar leakage checked.

Verification:
- `go test ./cmd/observability-e2e`
- `go test ./...`
- Manual run against docker observability stack if available.

Ready for integration when:
- Public traffic scenarios are deterministic against compose.

Integration notes:
- Keep generated test IDs unique and clean up where practical.

## Worker Prompt

```text
Implement TASK-005: Public API Metrics Scenarios

Goal:
Add runner scenarios that generate metrics through public avatar-service API only.

Owned surface:
cmd/observability-e2e public API traffic helpers and metrics scenarios.

Scope:
Call /health, perform valid upload, perform invalid upload, poll Prometheus for http_requests_total and avatars_uploads_total.

Out of scope:
Test-only hooks and alert firing.

Inspect these likely areas:
cmd/avatar-contract-tests
cmd/observability-e2e
internal/http/observability_test.go for expected metric labels only; do not import internal code.

Avoid changing:
internal packages, alert configs, Makefile.

Respect these contracts:
No internal imports. Route labels must not leak raw avatar_id/user_id.

Acceptance criteria:
Success and error upload metrics are observed; route label cardinality checks exist.

Verification:
go test ./cmd/observability-e2e
go test ./...
Manual run against docker observability stack if available.

Ready for integration when:
Scenarios are deterministic against compose.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
