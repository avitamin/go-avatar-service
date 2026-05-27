# TASK-004: Prometheus And Alertmanager Baseline Scenarios

Phase: Parallel
Owner: QA/Test
Owned surface: runner scenarios for readiness, scrape targets, loaded rules, Alertmanager readiness
Parallel-safe: Yes
Can start after: TASK-001

Goal: Verify runtime observability stack wiring without generating alert signals.

Scope:
- Prometheus `/api/v1/targets`.
- Prometheus `/api/v1/rules`.
- PromQL helper.
- Alertmanager `/api/v2/status`.
- Diagnostics.

Out of scope:
- Upload traffic.
- Controlled alert triggers.

Likely touched areas:
- `cmd/observability-e2e/**`
- `configs/observability/prometheus/prometheus.yml` for expected jobs.
- `configs/observability/prometheus/alerts.yml` for expected alerts.

High-conflict surfaces:
- Runner client interfaces, if not stabilized in TASK-001.

Dependencies:
- TASK-001.

Contract constraints:
- Expected jobs include `avatar-service-server`, `avatar-service-worker`, `postgres`, `rabbitmq`.
- Grafana is not required.

Acceptance criteria:
- Missing target/rule/status gives actionable failure.

Verification:
- `go test ./cmd/observability-e2e`
- Manual run against compose when available.

Ready for integration when:
- Baseline scenarios fail clearly against an unhealthy stack.

Integration notes:
- Do not add product traffic here; keep this slice about stack wiring.

## Worker Prompt

```text
Implement TASK-004: Prometheus And Alertmanager Baseline Scenarios

Goal:
Add baseline observability stack scenarios to the e2e runner.

Owned surface:
cmd/observability-e2e baseline Prometheus/Alertmanager clients and scenarios.

Scope:
Check Prometheus readiness, scrape targets, loaded alert rules, PromQL polling helper, Alertmanager readiness.

Out of scope:
Public API traffic, test-only hooks, alert signal generation.

Inspect these likely areas:
cmd/observability-e2e
configs/observability/prometheus/prometheus.yml
configs/observability/prometheus/alerts.yml
configs/observability/alertmanager/alertmanager.yml

Avoid changing:
internal packages, Makefile, compose files.

Respect these contracts:
Expected jobs: avatar-service-server, avatar-service-worker, postgres, rabbitmq. Grafana is not required.

Acceptance criteria:
Missing target/rule/readiness failures are diagnostic and include observed state.

Verification:
go test ./cmd/observability-e2e
go test ./...

Ready for integration when:
Baseline scenarios can run against compose and fail clearly when stack is unhealthy.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
