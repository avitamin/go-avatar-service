# TASK-011: Final Hardening And Documentation Update

Phase: Hardening
Owner: Integration
Owned surface: final verification notes and plan/doc updates
Parallel-safe: No
Can start after: TASK-010

Goal: Close gaps, remove drift, and record operational usage.

Scope:
- Full command run.
- Flake review.
- Cleanup checks.
- Concise docs update in the appropriate owner doc.

Out of scope:
- New features.
- New alert rules.

Likely touched areas:
- Docs owner chosen from `docs/repo-documentation-guide.md`.
- Possibly `README.md` or development docs if Make targets need discoverability.

High-conflict surfaces:
- Docs if other docs work is ongoing.

Dependencies:
- TASK-010.

Contract constraints:
- Do not promote hooks as product API.

Acceptance criteria:
- Full verification list has results.
- Docs mention opt-in nature and required stack state.

Verification:
- `go test ./...`
- `promtool test rules configs/observability/prometheus/alert-rule-tests.yml`
- `make docker-observability-up`
- `make docker-observability-e2e`

Ready for integration when:
- PR has clear test evidence and known residual risks.

Integration notes:
- Update indexes only if new long-lived documentation is added during implementation.

## Worker Prompt

```text
Implement TASK-011: Final Hardening And Documentation Update

Goal:
Perform final integration verification and document the opt-in workflow.

Owned surface:
Final docs/update notes and integration cleanup.

Scope:
Run full verification, identify flakes, ensure cleanup behavior, update the appropriate repo documentation with how to run observability e2e and promtool checks.

Out of scope:
New alert rules, new scenarios, or production threshold changes.

Inspect these likely areas:
docs/repo-documentation-guide.md
README.md
docs/development-workflow.md
docs/plans/07-observability-e2e-metrics-alerting-plan.md

Avoid changing:
Product requirements/spec unless an actual conflict is discovered and explicitly approved.

Respect these contracts:
Do not document hooks as public API. Emphasize opt-in nature.

Acceptance criteria:
Docs show commands and prerequisites; final verification evidence is available.

Verification:
go test ./...
promtool test rules configs/observability/prometheus/alert-rule-tests.yml
make docker-observability-up
make docker-observability-e2e

Ready for integration when:
The PR can be reviewed with clear checks, known risks, and no unresolved coordination notes.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
