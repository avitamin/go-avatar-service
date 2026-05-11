# TASK-003: Promtool Alert Rule Fixtures

Phase: Parallel
Owner: QA/Test
Owned surface: Prometheus synthetic rule tests
Parallel-safe: Yes
Can start after: None

Goal: Cover all 7 alert expressions with fast synthetic rule tests.

Scope:
- Add `configs/observability/prometheus/alert-rule-tests.yml`.
- Inactive and firing cases for every alert rule.
- Expected labels and key annotations.

Out of scope:
- Changing `alerts.yml` thresholds.
- Changing production `for` windows.

Likely touched areas:
- `configs/observability/prometheus/alert-rule-tests.yml`
- `configs/observability/prometheus/alerts.yml` for reference only.

High-conflict surfaces:
- Prometheus alert schemas.

Dependencies:
- Current `configs/observability/prometheus/alerts.yml`.

Contract constraints:
- Fixtures must match existing alert names and labels.

Acceptance criteria:
- All 7 alerts have inactive and firing cases.

Verification:
- `promtool test rules configs/observability/prometheus/alert-rule-tests.yml`

Ready for integration when:
- Fixture passes with current rules.

Integration notes:
- If an expression appears wrong, report it rather than changing thresholds silently.

## Worker Prompt

```text
Implement TASK-003: Promtool Alert Rule Fixtures

Goal:
Add synthetic promtool tests for all current alert rules.

Owned surface:
configs/observability/prometheus/alert-rule-tests.yml.

Scope:
Cover inactive and firing cases for HighHTTPErrorRate, HighUploadErrorRate, HighResponseTimeP95, UploadLatencyCritical, DependencyOperationErrors, RabbitMQQueueBacklog, WorkerProcessingFailures. Check key labels and annotations.

Out of scope:
Changing alerts.yml thresholds, for windows, or production alert semantics.

Inspect these likely areas:
configs/observability/prometheus/alerts.yml
internal/observability/grafana_dashboards_test.go

Avoid changing:
Runtime Go code, Makefile unless explicitly needed after coordination.

Respect these contracts:
Fixtures must match existing alert names, severity/service/component labels where present.

Acceptance criteria:
Each rule has inactive and firing coverage.

Verification:
promtool test rules configs/observability/prometheus/alert-rule-tests.yml
go test ./...

Ready for integration when:
Fixtures pass against current alerts.yml.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
