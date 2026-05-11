# TASK-009: RabbitMQ Backlog And Worker Failure Slice

Phase: Parallel
Owner: Backend
Owned surface: RabbitMQ/worker control path and runner scenarios
Parallel-safe: Yes
Can start after: TASK-001, TASK-002

Goal: Support `RabbitMQQueueBacklog` and `WorkerProcessingFailures` runtime checks.

Scope:
- Deterministic backlog mechanism.
- Worker failure signal.
- Cleanup/resume behavior.
- RabbitMQ Management client in runner if needed.

Out of scope:
- Changing RabbitMQ topology.
- Changing production worker retry semantics.

Likely touched areas:
- `internal/app`
- `internal/worker`
- `internal/broker/rabbitmq`
- `cmd/observability-e2e/**`

High-conflict surfaces:
- Worker run loop.
- Broker topology.

Dependencies:
- TASK-001.
- TASK-002.

Contract constraints:
- Queues/routing keys remain `avatars.uploads`, `avatars.deletes`, `avatar.uploaded`, `avatar.delete_requested`.
- Stack must not remain paused.

Acceptance criteria:
- Queue depth metric observed.
- Worker error metric observed.
- Cleanup runs on failure.

Verification:
- `go test ./internal/worker ./internal/broker/rabbitmq ./cmd/observability-e2e`
- Compose smoke when available.

Ready for integration when:
- Scenarios leave RabbitMQ/worker in reusable state.

Integration notes:
- Any pause/resume hook must use `defer` cleanup in the runner scenario.

## Worker Prompt

```text
Implement TASK-009: RabbitMQ Backlog And Worker Failure Slice

Goal:
Support runtime checks for RabbitMQQueueBacklog and WorkerProcessingFailures.

Owned surface:
RabbitMQ/worker control path and runner scenarios.

Scope:
Create deterministic backlog generation, worker failure signal, cleanup/resume behavior, and RabbitMQ Management API helper if needed.

Out of scope:
Changing RabbitMQ topology, queue names, routing keys, or production retry semantics.

Inspect these likely areas:
internal/app/app.go
internal/worker/runner.go
internal/broker/rabbitmq/rabbitmq.go
cmd/observability-e2e
docker-compose.yml
docker-compose.observability.yml

Avoid changing:
Exchange/queue/routing key names and production worker behavior outside gated test-only paths.

Respect these contracts:
Queues avatars.uploads and avatars.deletes; routing keys avatar.uploaded and avatar.delete_requested. Stack must not remain paused or polluted.

Acceptance criteria:
rabbitmq_queue_messages is observable for backlog; avatar_worker_messages_total{status="error"} is observable for failure; cleanup runs on scenario failure.

Verification:
go test ./internal/worker ./internal/broker/rabbitmq ./cmd/observability-e2e
go test ./...
Manual compose smoke if available.

Ready for integration when:
Scenarios leave RabbitMQ and worker reusable.

Final response should include:
- changed surfaces,
- checks run,
- risks or blockers,
- integration notes.
```
